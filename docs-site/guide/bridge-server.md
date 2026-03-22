# BridgeServer Integration Guide

## Bridging the Gap: Connecting External Platforms to HotPlex

HotPlex's **BridgeServer** is a WebSocket gateway that enables external platform adapters — such as DingTalk, WeChat, Feishu, or any custom chat platform — to connect to HotPlex without being compiled into the same binary. BridgeServer acts as a bidirectional translator between the native Bridge Wire Protocol and HotPlex's internal session model.

```
External Adapter (your DingTalk bot)
        │
        │ BridgeClient SDK
        ▼
   WebSocket
        │
        ▼
 BridgeServer
        │
        ▼
 HotPlex Engine
        │
        ▼
 SessionPool (AI Brain)
```

Once connected, a bridge adapter is indistinguishable from a built-in platform: it registers as a `base.ChatAdapter`, receives messages through the standard `MessageHandler` pipeline, and delivers replies back over the same WebSocket channel.

---

## Configuration

### Environment Variables

| Variable | Required | Description |
| :-------- | :------: | :---------- |
| `HOTPLEX_BRIDGE_PORT` | Yes | TCP port for the BridgeServer listener (e.g., `8080`). Empty disables BridgeServer. |
| `HOTPLEX_BRIDGE_TOKEN` | Yes | Shared secret token. All adapter connections must present this token. |

### config.yaml

```yaml
server:
  bridge_port: "8080"
  bridge_token: "your-secret-token"
```

The BridgeServer runs on its own HTTP port — it is **not** multiplexed with the main WebSocket gateway. This ensures fault isolation: a misbehaving external adapter does not affect the primary gateway.

---

## Quick Start

### 1. Enable BridgeServer

Start HotPlex with BridgeServer enabled:

```bash
hotplexd start --config=config.yaml
```

Or set environment variables directly:

```bash
HOTPLEX_BRIDGE_PORT=8080 HOTPLEX_BRIDGE_TOKEN=secret hotplexd start
```

HotPlex will log:

```
BridgeServer listening  addr=:8080
```

### 2. Run the Example Adapter

```bash
cd cmd/bridge-client/example
go run main.go
```

The example adapter connects to BridgeServer, registers as platform `dingtalk`, and handles incoming messages with a hardcoded reply. Watch the logs on both sides to observe the registration handshake and message round-trip.

---

## BridgeClient SDK

The **BridgeClient SDK** (`cmd/bridge-client`) is the Go library adapter developers use to connect to BridgeServer. It wraps the raw WebSocket connection and handles the Bridge Wire Protocol.

### Connection Options

| Option | Required | Description |
| :------ | :------: | :---------- |
| `URL` | Yes | BridgeServer WebSocket URL, e.g. `ws://localhost:8080/bridge` |
| `Platform` | Yes | Platform identifier (`[a-z0-9_-]{1,32}`). Cannot be: `slack`, `feishu`, `dingtalk`, `wechat`, `telegram`, `discord` |
| `AuthToken` | Yes | Must match `bridge_token` in HotPlex config |
| `Capabilities` | No | Defaults to `[CapText]` if omitted |

### Declared Capabilities

When connecting, declare what your adapter supports:

| Constant | Description |
| :------- | :---------- |
| `CapText` | Plain text messages |
| `CapImage` | Image uploads |
| `CapCard` | Rich card messages |
| `CapButtons` | Interactive buttons |
| `CapTyping` | Typing indicators |
| `CapEdit` | Edit sent messages |
| `CapDelete` | Delete sent messages |
| `CapReact` | Emoji reactions |
| `CapThread` | Thread / reply support |

### Message Flow

#### Inbound: HotPlex Engine → Your Adapter

Register a handler to receive messages:

```go
client.OnMessage(func(msg *bridgeclient.Message) *bridgeclient.Reply {
    log.Printf("session=%s user=%s: %s",
        msg.SessionKey, msg.Metadata.UserID, msg.Content)

    reply := yourAI.Process(msg.Content)
    return &bridgeclient.Reply{
        Content:    reply.Text,
        SessionKey: msg.SessionKey,
    }
})
```

#### Events: HotPlex Engine → Your Adapter

HotPlex sends ephemeral events (not user messages) separately:

```go
client.OnEvent(func(evt *bridgeclient.Event) {
    switch evt.Event {
    case "typing":
        // Show typing indicator in the platform UI
    case "stream_end":
        // Final chunk of a streamed response received
    }
})
```

#### Outbound: Your Adapter → HotPlex Engine

For webhooks or outbound-initiated messages:

```go
err := client.SendMessage(ctx, &bridgeclient.Message{
    SessionKey: "session-from-dingtalk",
    Content:    "Outbound message received",
    Metadata: bridgeclient.Metadata{
        UserID: "openid-xxx",
        RoomID: "chatid-yyy",
    },
})
```

---

## Bridge Wire Protocol

The wire protocol is a bidirectional JSON envelope over WebSocket. Both parties share the same `WireMessage` struct; only relevant fields are populated per message type.

### Message Types

| Direction | Type | Description |
| :-------- | :--- | :---------- |
| Client → Server | `register` | Handshake: declares platform name and capabilities |
| Server → Client | `reply` | Acknowledgment of successful registration |
| Server → Client | `message` | Inbound message from HotPlex engine |
| Client → Server | `reply` | Response to a `message` |
| Server → Client | `event` | Engine events (`typing`, `stream_end`, etc.) |
| Either | `error` | Protocol errors |

### WireMessage Schema

```json
{
  "type": "register",
  "platform": "dingtalk",
  "token": "your-secret-token",
  "capabilities": ["text", "card", "buttons"]
}
```

| Field | Type | Description |
| :----- | :--- | :---------- |
| `type` | string | Message type: `register`, `message`, `reply`, `event`, `error` |
| `platform` | string | Platform identifier (set by client on register, `"hotplex"` on replies from server) |
| `session_key` | string | Session identifier; must be echoed back in `reply` |
| `content` | string | Message text payload |
| `metadata` | object | Container for `user_id`, `room_id`, `thread_id`, `platform` |
| `event` | string | Event name (used with type `event`) |
| `code` | int | Error code (used with type `error`) |
| `message` | string | Error description (used with type `error`) |
| `capabilities` | string[] | Declared capabilities (used with type `register`) |

See `internal/bridgewire/wire.go` for the canonical schema definition.

---

## Security

### Token Authentication

All BridgeClient connections must present the shared `bridge_token`:

- **Preferred**: `Authorization: Bearer <token>` header
- **Deprecated**: `?token=<token>` query parameter (BridgeServer logs a warning; will be removed)

If `bridge_token` is empty in config, BridgeServer accepts connections without a token (development mode only).

### Platform Name Guard

BridgeServer validates platform names against `^[a-z0-9_-]{1,32}$` and rejects names reserved for built-in platforms (`slack`, `feishu`, `dingtalk`, `wechat`, `telegram`, `discord`). This prevents adapter name collisions and path injection.

### Input Validation (WAF)

All inbound message content is passed through HotPlex's Regex WAF (`internal/security/detector.go`) before reaching the engine. Dangerous commands (e.g., `rm -rf`) are flagged and blocked with a `400` error returned to the adapter.

### Connection Lifecycle

- BridgeServer runs on a **separate HTTP server** from the main WebSocket gateway, providing fault isolation.
- When a WebSocket connection closes, the platform is unregistered from the AdapterManager and the session pipeline, preventing orphaned routing.
- Graceful shutdown via `Shutdown(ctx)` closes all platform connections cleanly.
