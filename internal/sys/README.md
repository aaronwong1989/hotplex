# Sys Package (`internal/sys`)

Cross-platform process management utilities.

## Overview

This package handles low-level OS-specific operations for process group management, signal handling, and process lifecycle control. It abstracts the differences between Unix (PGID-based) and Windows (taskkill-based) process termination strategies.

## Primary Purpose

Ensure proper cleanup of CLI processes and their children, preventing zombie processes and resource leaks.

## Usage

```go
import "github.com/hrygo/hotplex/internal/sys"

// Kill process group (Unix: PGID, Windows: taskkill)
err := sys.KillProcessGroup(pid)

// Find process by name
pids, err := sys.FindProcessByName("claude")
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
| `proc_unix.go` | Unix process management |
| `proc_windows.go` | Windows process management |
| `path.go` | Path utilities |
