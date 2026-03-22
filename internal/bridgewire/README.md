# Bridge Wire Protocol

`internal/bridgewire` defines the shared types and constants for the **Bridge Wire Protocol**: a bidirectional JSON envelope used by BridgeServer and BridgeClients to communicate over WebSocket.

Both parties must agree on the exact field names and JSON tags documented here. This package is intentionally minimal — it contains no I/O, no business logic, and no dependencies beyond `encoding/json`.

---

## Architecture

```
External Adapter (DingTalk) ──WebSocket──> BridgeServer
  (bridge-client SDK)          internal/server/bridge.go
                                 │
                                 ▼
                        internal/bridgewire  (shared types)
                                 │
                                 ▼
                        HotPlex Engine / ChatApps
```

`internal/server/bridge.go` and `cmd/bridge-client` both import this package. The protocol is symmetric: both sides send and receive the same `WireMessage` envelope, but only relevant fields are populated for each message type.

---

## Message Types

`MsgType*` constants are the valid values for the `type` field in a `WireMessage`.

| Constant | Value | Direction | Description |
|----------|-------|----------|-------------|
| `MsgTypeRegister` | `"register"` | Client → Server | Initial handshake. Adapter announces its `platform` name and supported `capabilities`. Server responds with `MsgTypeReply`. |
| `MsgTypeMessage` | `"message"` | Both | Primary message exchange. Carries `content`, `session_key`, and `metadata`. |
| `MsgTypeReply` | `"reply"` | Both | Response to a `message`. Used by the adapter to deliver AI replies back to the platform. |
| `MsgTypeEvent` | `"event"` | Both | Asynchronous signals such as typing indicators (`"typing"`) or stream events (`"stream_start"`, `"stream_chunk"`, `"stream_end"`). |
| `MsgTypeError` | `"error"` | Both | Error response. Contains a numeric `code` and human-readable `message`. |

---

## Capabilities

`Capability` constants are declared by an adapter during the `register` handshake via the `capabilities` field. They inform HotPlex which message features the external platform supports.

| Constant | Value | Description |
|----------|-------|-------------|
| `CapText` | `"text"` | Plain and markdown text messages. |
| `CapImage` | `"image"` | Image/file attachments. |
| `CapCard` | `"card"` | Structured card layouts. |
| `CapButtons` | `"buttons"` | Interactive button elements. |
| `CapTyping` | `"typing"` | Typing indicator events. |
| `CapEdit` | `"edit"` | Message editing after sending. |
| `CapDelete` | `"delete"` | Message deletion. |
| `CapReact` | `"react"` | Emoji reaction events. |
| `CapThread` | `"thread"` | Threaded message replies. |

`AllCapabilities` exports the full set as a `[]string` slice for convenience.

---

## Structs

### WireMessage

The bidirectional JSON envelope. All fields use `snake_case` JSON tags matching the wire format.

| Field | Type | JSON tag | Present in | Description |
|-------|------|----------|-----------|-------------|
| `Type` | `string` | `type` | All | Message type identifier (`MsgType*`). |
| `Platform` | `string` | `platform` | `register`, `reply`, `message` | Platform name, e.g. `"dingtalk"`. |
| `Token` | `string` | `token` | `register` | Authentication token. |
| `SessionKey` | `string` | `session_key` | `message`, `reply`, `event` | Unique session identifier for routing. |
| `Content` | `string` | `content` | `message`, `reply` | Message text content. |
| `Metadata` | `json.RawMessage` | `metadata` | `message`, `reply` | `WireMetadata` encoded as JSON. |
| `Event` | `string` | `event` | `event` | Event name, e.g. `"typing"`. |
| `Data` | `json.RawMessage` | `data` | `event` | Arbitrary JSON payload for event data. |
| `Code` | `int` | `code` | `error` | Numeric error code. |
| `Message` | `string` | `message` | `error` | Human-readable error description. |
| `Capabilities` | `[]string` | `capabilities` | `register` | List of supported capabilities. |

### WireMetadata

Carried inside `WireMessage.Metadata`. Describes the source room, thread, and user identity.

| Field | Type | JSON tag | Description |
|-------|------|----------|-------------|
| `UserID` | `string` | `user_id` | User identifier on the external platform. |
| `RoomID` | `string` | `room_id` | Room or channel identifier. |
| `ThreadID` | `string` | `thread_id` | Thread identifier (for threaded platforms). |
| `Platform` | `string` | `platform` | Platform name that originated the metadata. |

---

## Consumers

### internal/server/bridge.go

`BridgeServer` acts as the WebSocket server side. It:
- Accepts WebSocket connections at `GET /bridge/v1/{platform}`.
- Authenticates via `Authorization: Bearer <token>` header (deprecated query-param fallback).
- Dispatches inbound `WireMessage`s to `BridgePlatform`, which translates them into `base.ChatMessage` and forwards to the HotPlex engine.
- Delivers outbound AI replies back to the adapter via the same WebSocket.

`BridgePlatform` implements `base.ChatAdapter`, making it indistinguishable from built-in adapters as far as the engine is concerned.

### cmd/bridge-client

`bridge-client` is the reference Go SDK for building external adapters. It:
- Connects to `BridgeServer` as a WebSocket client.
- Sends a `MsgTypeRegister` handshake on connect.
- Receives `MsgTypeMessage` (AI replies) and `MsgTypeEvent` (stream signals) via `OnMessage` / `OnEvent` handlers.
- Sends inbound user messages via `SendMessage` and typing indicators via `Typing`.

See `cmd/bridge-client/example/main.go` for a complete integration example.

---

## Protocol Flow

```
Adapter                          BridgeServer
  │                                    │
  │──── WebSocket Connect ───────────▶│
  │                                    │
  │──── register {platform, caps} ────▶│  validate + register platform
  │◀─── reply {type:"reply"} ──────────│
  │                                    │
  │                                    │  (user sends DM on DingTalk)
  │◀─── (routing via AdapterManager) ──│
  │──── message {content, session_key, metadata} ────▶│  WAF check → ChatMessage → engine
  │                                    │
  │◀─── message {content} ────────────│  AI reply from engine
  │──── reply {content, session_key} ──▶│
  │                                    │
  │  (stream events delivered via event type)
```
