# Telemetry Package (`internal/telemetry`)

Observability and metrics collection for HotPlex.

## Overview

This package manages Prometheus metrics and internal tracing. It tracks session duration, token usage, and security blocks globally.

## Metrics Tracked

| Metric | Description |
|--------|-------------|
| `sessions_active` | Currently active sessions (Gauge) |
| `sessions_total` | Total sessions created (Counter) |
| `sessions_errors` | Failed sessions (Counter) |
| `tools_invoked` | Tool invocations count (Counter) |
| `dangers_blocked` | Security blocks count (Counter) |
| `request_duration` | Latest request duration (ms) |
| `slack_permission_*` | Slack permission decisions (Allowed, BlockedUser, DM, Mention) |

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
m.RecordDuration(150 * time.Millisecond)

// Get snapshot
snapshot := m.Snapshot()
fmt.Printf("Active sessions: %d\n", snapshot.SessionsActive)
fmt.Printf("Last duration: %v\n", snapshot.RequestDuration)
```

## Health Check

HotPlex uses a two-tier health system:
1. **`internal/telemetry.HealthChecker`**: Logic-only checker for registering and running probes.
2. **`internal/server.HealthHandler`**: HTTP wrapper that exposes the status as JSON.

```go
// Register a custom check
checker := telemetry.GetHealthChecker()
checker.RegisterCheck("engine", func() bool { return engine.IsReady() })

// Checks are exposed via /health, /health/ready, and /health/live
```

## Files

| File | Purpose |
|------|---------|
| `metrics.go` | Metrics collection and snapshots |
| `tracer.go` | Distributed tracing support |
| `health.go` | Health check handler |
