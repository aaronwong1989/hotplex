# Telemetry Package (`internal/telemetry`)

Observability and metrics collection for HotPlex.

## Overview

This package manages Prometheus metrics and internal tracing. It tracks session duration, token usage, and security blocks globally.

## Metrics Tracked

| Metric | Description |
|--------|-------------|
| `sessions_active` | Currently active sessions |
| `sessions_total` | Total sessions created |
| `sessions_errors` | Failed sessions |
| `tools_invoked` | Tool invocations count |
| `dangers_blocked` | Security blocks count |
| `slack_permission_*` | Slack permission decisions |

## Usage

```go
import "github.com/hrygo/hotplex/internal/telemetry"

// Initialize global metrics
telemetry.InitMetrics(logger)

// Get metrics instance
m := telemetry.GetMetrics()

// Record events
m.IncSessionsActive()
m.IncToolsInvoked()
m.IncDangersBlocked()

// Get snapshot
snapshot := m.Snapshot()
fmt.Printf("Active sessions: %d\n", snapshot.SessionsActive)
```

## Health Check

```go
// Health endpoint handler
handler := telemetry.NewHealthHandler(engine)
// GET /health returns {"status": "ok"}
```

## Files

| File | Purpose |
|------|---------|
| `metrics.go` | Metrics collection and snapshots |
| `tracer.go` | Distributed tracing support |
| `health.go` | Health check handler |
