# Engine Package (`internal/engine`)

Core session management and process pool implementation for HotPlex.

## Overview

This package implements the **Hot-Multiplexing** pattern, maintaining persistent CLI agent processes that can be reused across multiple execution turns. This eliminates the cold-start latency of spawning heavy Node.js processes for each request.

## Key Components

| Component | Description |
|-----------|-------------|
| `SessionPool` | Thread-safe process pool with idle GC |
| `Session` | Individual CLI process wrapper with full-duplex I/O |
| `SessionManager` | Interface for process lifecycle management |

## Architecture

```
SessionPool
    ├── sessions map[string]*Session  (active sessions)
    ├── mu sync.RWMutex               (thread-safe access)
    ├── markerStore                   (session persistence)
    └── cleanupLoop()                 (idle session GC)
```

## Usage

```go
import "github.com/hrygo/hotplex/internal/engine"

// Create session pool
pool := engine.NewSessionPool(
    logger,
    30*time.Minute,  // idle timeout
    opts,            // engine options
    cliPath,         // CLI binary path
    provider,        // CLI provider
)

// Get or create session
session, created, err := pool.GetOrCreateSession(ctx, sessionID, cfg, prompt)

// Execute command
err := session.Execute(ctx, prompt, callback)

// Shutdown pool (graceful)
pool.Shutdown(ctx)
```

## Design Principles

- **PGID Isolation**: Each session runs in its own process group for clean termination
- **Idle GC**: Inactive sessions are garbage collected after timeout
- **Thread Safety**: All operations are protected by `sync.RWMutex`
- **Graceful Shutdown**: Respects context cancellation

## Files

| File | Purpose |
|------|---------|
| `pool.go` | SessionPool implementation |
| `session.go` | Session lifecycle and I/O handling |
| `types.go` | Type definitions and interfaces |
