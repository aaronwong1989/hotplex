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

## Usage

```go
import "github.com/hrygo/hotplex/internal/server"

// Create execution controller
ctrl := server.NewExecutionController(engine, logger)

// Execute request
err := ctrl.Execute(ctx, server.ExecutionRequest{
    SessionID:    "session-123",
    Prompt:       "Write a hello world program",
    WorkDir:      "/tmp/sandbox",
    Timeout:      15 * time.Minute,
}, callback)
```

## Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/ws` | WebSocket | Native HotPlex WebSocket |
| `/v1/execute` | POST | OpenCode-compatible execution |
| `/health` | GET | Health check |
| `/metrics` | GET | Prometheus metrics |

## Security

- **Path Validation**: Prevents directory traversal attacks
- **Timeout Enforcement**: All requests have configurable timeouts
- **Input Sanitization**: WorkDir paths are validated

## Files

| File | Purpose |
|------|---------|
| `controller.go` | Execution orchestration |
| `hotplex_ws.go` | WebSocket handler |
| `opencode_http.go` | OpenCode HTTP compatibility |
| `observability.go` | Metrics and health |
| `security.go` | Security utilities |
