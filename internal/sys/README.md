# Sys Package (`internal/sys`)

Cross-platform process management utilities.

## Overview

This package handles low-level OS-specific operations for process group management, signal handling, and process lifecycle control. It abstracts the differences between Unix (PGID-based) and Windows (taskkill-based) process termination strategies.

## Primary Purpose

Ensure proper cleanup of CLI processes and their children, preventing zombie processes and resource leaks.

## Usage

```go
import "github.com/hrygo/hotplex/internal/sys"

// Kill process group (Unix: PGID, Windows: Job Object)
sys.KillProcessGroup(cmd, jobHandle)

// Check if process is alive
if sys.IsProcessAlive(process) {
    // ...
}

// Diagnostics
result := sys.CheckCliAvailable()
if result.Available {
    fmt.Printf("Claude version: %s\n", result.Version)
}
```

## Platform Differences

| Platform | Kill Strategy |
|----------|---------------|
| Unix/Linux | `kill(-pgid, SIGKILL)` |
| Windows | `taskkill /F /T /PID` |

## Files

| File | Purpose |
|------|---------|
| `doc.go` | Package documentation |
| `proc_unix.go` | Unix process management (PGID-based) |
| `proc_windows.go` | Windows process management (Job Object-based) |
| `diagnostics.go` | CLI and database health checks |
| `path.go` | Path utilities |
