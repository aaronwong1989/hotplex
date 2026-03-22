# Server Package (`internal/server`)

HTTP/WebSocket transport layer for HotPlex.

## Overview

This package provides the core HTTP and WebSocket handlers that bridge web clients to the HotPlex Engine. It implements OpenCode-compatible HTTP endpoints and native WebSocket protocol.

## Key Components

| Component | Description |
|-----------|-------------|
| `ExecutionController` | Main execution orchestration |
| `HotPlexWebSocket` | Native WebSocket handler |
| `OpenCodeHTTP` | OpenCode-compatible HTTP endpoints |
| `Observability` | Metrics and health endpoints |
| `BridgeServer` | WebSocket gateway for external platform adapters |

## Usage

```go
import "github.com/hrygo/hotplex/internal/server"

// Create execution controller
ctrl := server.NewExecutionController(engine, logger)

// Execute request
err := ctrl.Execute(ctx, server.ExecutionRequest{
    SessionID:    "session-123",
    Prompt:       "Write a hello world program",
    Instructions: "Use Python 3.14",
    SystemPrompt: "You are a senior engineer",
    WorkDir:      "/tmp/sandbox",
    Timeout:      15 * time.Minute,
}, callback)
```

## Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/ws/v1/agent` | WebSocket | Native HotPlex Agent WebSocket |
| `/session` | POST | Create OpenCode-compatible session |
| `/session/{id}/message` | POST | Send prompt (OpenCode compatible) |
| `/global/event` | GET/SSE | Global event stream (SSE) |
| `/api/v1/admin/` | ANY | Enhanced Admin API (Sessions, Stats, Doctor) |
| `/health[/ready|/live]` | GET | Health, Readiness, and Liveness checks |
| `/metrics` | GET | Prometheus metrics |
| `/bridge/v1/{platform}` | WebSocket | External platform adapter gateway (BridgeServer) |

## Security

- **Path Validation**: Prevents directory traversal attacks
- **Timeout Enforcement**: All requests have configurable timeouts
- **Input Sanitization**: WorkDir paths are validated
- **BridgeServer Token Auth**: Bridge connections authenticated via `Authorization: Bearer <token>` header (or deprecated query param `?token=`). Configured via `bridge_token` in server YAML or `HOTPLEX_BRIDGE_TOKEN` env var.

## Files

| File | Purpose |
|------|---------|
| `controller.go` | Execution orchestration |
| `hotplex_ws.go` | WebSocket handler |
| `opencode_http.go` | OpenCode HTTP compatibility |
| `observability.go` | Metrics and health |
| `security.go` | Security utilities |
| `bridge.go` | BridgeServer WebSocket gateway for external platform adapters |

## BridgeServer and External Adapters

BridgeServer exposes `/bridge/v1/{platform}` as a WebSocket endpoint. External platform adapters (e.g., DingTalk, WeChat) connect as WebSocket clients and communicate via the Bridge Wire Protocol defined in `internal/bridgewire/`. See [BridgeClient](../cmd/bridge-client/README.md) for the corresponding client-side implementation and protocol reference.
