# BridgeClient SDK

Go SDK for connecting external platform adapters to HotPlex via the BridgeServer WebSocket gateway.

## What is this?

HotPlex's **BridgeServer** (`internal/server/bridge.go`) is a WebSocket gateway that allows external platform adapters (DingTalk, WeChat, Feishu, etc.) to connect to HotPlex without being compiled into the same binary. **BridgeClient** is the SDK that adapter developers use to connect.

```
External Adapter (your DingTalk bot) ──BridgeClient──WebSocket──> BridgeServer
                                                                          │
                                                                          ▼
                                                                  HotPlex Engine
```

## Quick Start

```go
package main

import (
    "context"
    "log"
    "os"

    "github.com/hrygo/hotplex/cmd/bridge-client"
)

func main() {
    client, err := bridgeclient.New(
        bridgeclient.URL("wss://hotplex.internal/bridge"),
        bridgeclient.Platform("dingtalk"),
        bridgeclient.AuthToken(os.Getenv("HOTPLEX_BRIDGE_TOKEN")),
        bridgeclient.Capabilities(
            bridgeclient.CapText,
            bridgeclient.CapCard,
            bridgeclient.CapButtons,
            bridgeclient.CapTyping,
        ),
    )
    if err != nil {
        log.Fatal(err)
    }

    client.OnMessage(func(msg *bridgeclient.Message) *bridgeclient.Reply {
        log.Printf("session=%s user=%s: %s",
            msg.SessionKey, msg.Metadata.UserID, msg.Content)

        return &bridgeclient.Reply{
            Content:    "Hello from your DingTalk bot!",
            SessionKey: msg.SessionKey,
        }
    })

    ctx := context.Background()
    if err := client.Connect(ctx); err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    <-ctx.Done()
}
```

## Configuration

| Option | Required | Description |
|--------|----------|-------------|
| `URL` | Yes | BridgeServer WebSocket URL (e.g., `ws://localhost:8080/bridge`) |
| `Platform` | Yes | Platform identifier (e.g., `dingtalk`, `wechat`, `lark`) |
| `AuthToken` | Yes | Token matching `bridge_token` in HotPlex config |
| `Capabilities` | No | Defaults to `[CapText]` if not set |

## Capabilities

Declare what your adapter supports when connecting:

| Constant | Description |
|----------|-------------|
| `CapText` | Plain text messages |
| `CapImage` | Image uploads |
| `CapCard` | Rich card messages |
| `CapButtons` | Interactive buttons |
| `CapTyping` | Typing indicators |
| `CapEdit` | Edit sent messages |
| `CapDelete` | Delete sent messages |
| `CapReact` | Emoji reactions |
| `CapThread` | Thread/reply support |

## Message Flow

### Inbound (HotPlex → Your Adapter)

HotPlex sends user messages to your adapter via the `OnMessage` handler:

```go
client.OnMessage(func(msg *bridgeclient.Message) *bridgeclient.Reply {
    // msg.SessionKey  — Session identifier (pass back in Reply)
    // msg.Content     — Message text
    // msg.Metadata.UserID   — User identity
    // msg.Metadata.RoomID   — Room/channel identity
    // msg.Metadata.ThreadID — Thread/thread_ts identity
    // msg.Metadata.Platform — Always "hotplex"

    reply := yourAI.Process(msg.Content)
    return &bridgeclient.Reply{
        Content:    reply.Text,
        SessionKey: msg.SessionKey,
    }
})
```

### Events (HotPlex → Your Adapter)

HotPlex can also send events (e.g., `typing`, `stream_start`, `stream_end`):

```go
client.OnEvent(func(evt *bridgeclient.Event) {
    switch evt.Event {
    case "typing":
        // Show typing indicator in DingTalk UI
    case "stream_end":
        // Final message chunk received
    }
})
```

### Outbound (Your Adapter → HotPlex)

If HotPlex needs to receive messages initiated by your adapter (e.g., a DingTalk
outbound webhook triggered by an external event):

```go
err := client.SendMessage(ctx, &bridgeclient.Message{
    SessionKey: "session-from-dingtalk",
    Content:    "Outbound DingTalk message received",
    Metadata: bridgeclient.Metadata{
        UserID: "openid-xxx",
        RoomID: "chatid-yyy",
    },
})
```

## Running the Example

```bash
# 1. Start HotPlex with bridge enabled
# In your config.yaml or .env:
#   bridge_port=8080
#   bridge_token=secret

# 2. Run the example DingTalk adapter
cd _examples/go_bridge_client
go run main.go
```

## Bridge Wire Protocol

The SDK implements the Bridge Wire Protocol — a JSON envelope over WebSocket:

| Direction | Type | Description |
|-----------|------|-------------|
| Client → Server | `register` | Handshake: platform name + capabilities |
| Server → Client | `message` | Inbound message from HotPlex engine |
| Client → Server | `reply` | Response to a message |
| Server → Client | `event` | Engine events (typing, stream, etc.) |
| Either | `error` | Protocol errors |

See `internal/server/bridge.go` for the full protocol specification.

## HotPlex Configuration

Enable BridgeServer in your `config.yaml`:

```yaml
server:
  bridge_port: "8080"    # Separate HTTP server for bridge adapters
  bridge_token: "secret" # Token adapters must present
```

Or via environment variables:

```bash
HOTPLEX_BRIDGE_PORT=8080
HOTPLEX_BRIDGE_TOKEN=secret
```
