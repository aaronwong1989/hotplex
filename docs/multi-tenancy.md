# Multi-Tenancy Implementation

## Overview

This document describes the multi-tenancy implementation in Hotplex, providing workspace isolation, resource quotas, and audit logging per workspace.

## Components

### 1. WorkspaceManager (`internal/engine/workspace.go`)

The `WorkspaceManager` interface defines the contract for managing isolated workspaces:

```go
type WorkspaceManager interface {
    CreateWorkspace(ctx context.Context, cfg WorkspaceConfig) (*Workspace, error)
    GetWorkspace(ctx context.Context, workspaceID string) (*Workspace, bool)
    DeleteWorkspace(ctx context.Context, workspaceID string) error
    ListWorkspaces(ctx context.Context) []*Workspace
    UpdateQuota(ctx context.Context, workspaceID string, quota ResourceQuota) error
    GetUsage(ctx context.Context, workspaceID string) (WorkspaceUsage, error)
    ValidatePath(ctx context.Context, workspaceID string, path string) (bool, error)
    AuditLog(ctx context.Context, workspaceID string) (security.AuditStore, error)
    RegisterSession(ctx context.Context, workspaceID string, session *Session) error
    UnregisterSession(ctx context.Context, workspaceID string, sessionID string) error
    Shutdown()
}
```

#### Key Types

- **WorkspaceConfig**: Configuration for creating a workspace
- **Workspace**: Represents an isolated execution environment
- **ResourceQuota**: Resource limits (memory, CPU, processes, disk I/O)
- **WorkspaceUsage**: Current resource usage statistics

#### Default Quotas

```go
ResourceQuota{
    MemoryLimit:       2GB,
    CPUPercent:        80%,
    MaxProcesses:      50,
    DiskIOBytesPerSec: 100MB/s,
    MaxSessions:       10,
    MaxWorkspaceSize:  10GB,
}
```

### 2. Pool Integration (`internal/engine/pool.go`)

The `SessionPool` has been extended to support workspace integration:

```go
// Set workspace manager for multi-tenant isolation
sm.SetWorkspaceManager(wm, "default-workspace")

// Validate workspace paths
valid, err := sm.ValidateWorkspacePath(ctx, workspaceID, workDir)

// Get workspace usage
usage, err := sm.GetWorkspaceUsage(ctx, workspaceID)

// Check quota before creating session
err := sm.EnforceWorkspaceQuota(ctx, workspaceID)
```

### 3. Workspace-Level Audit (`internal/security/audit/workspace_audit.go`)

The `WorkspaceAuditStore` provides per-workspace and per-session audit logging:

```go
// Create workspace audit store
ws, _ := NewWorkspaceAuditStore("workspace-1", "")

// Save workspace-level event
ws.Save(ctx, event)

// Save session-level event
ws.SaveSessionEvent(ctx, "session-1", event)

// Query workspace events
events, _ := ws.Query(ctx, filter)

// Query specific session events
events, _ := ws.QuerySession(ctx, "session-1", filter)

// Query across all sessions
events, _ := ws.QueryAllSessions(ctx, filter, 100)
```

## Security Features

### Path Traversal Protection

The `ValidatePath` method prevents path traversal attacks:

```go
valid, err := wm.ValidatePath(ctx, workspaceID, "/path/to/file")
// Returns false if path attempts to escape workspace boundaries
```

### Directory Validation

- Validates workspace root path is absolute
- Checks for path traversal attempts (`../`)
- Verifies paths are within allowed base directories
- Tests write permission before workspace creation

## Usage Examples

### Creating a Workspace

```go
wm := NewWorkspaceManager(logger, "", DefaultResourceQuota())
defer wm.Shutdown()

cfg := WorkspaceConfig{
    ID:        "team-a-workspace",
    Name:      "Team A Workspace",
    RootPath:  "/home/user/.hotplex/workspaces/team-a",
    CreatedBy: "admin",
    Quota:     DefaultResourceQuota(),
}

ws, err := wm.CreateWorkspace(context.Background(), cfg)
```

### Using with SessionPool

```go
pool := NewSessionPool(logger, timeout, opts, cliPath, provider)

// Set workspace manager
pool.SetWorkspaceManager(wm, "default-workspace")

// Before creating session, validate path
valid, err := pool.ValidateWorkspacePath(ctx, workspaceID, sessionConfig.WorkDir)
if !valid {
    return fmt.Errorf("path validation failed: %w", err)
}

// Check quota
if err := pool.EnforceWorkspaceQuota(ctx, workspaceID); err != nil {
    return err
}

// Create session
session, created, err := pool.GetOrCreateSession(ctx, sessionID, cfg, prompt)
```

### Querying Audit Logs

```go
auditStore, _ := wm.AuditLog(ctx, workspaceID)
filter := security.AuditFilter{
    StartTime: time.Now().Add(-24 * time.Hour),
    Limit:     100,
}
events, _ := auditStore.Query(ctx, filter)
```

## Testing

Run workspace tests:

```bash
go test ./internal/engine/... -v -run TestWorkspace
```

## File Structure

```
internal/
├── engine/
│   ├── workspace.go          # WorkspaceManager implementation
│   ├── workspace_test.go     # Unit tests
│   ├── pool.go               # Extended with workspace integration
│   └── session.go
└── security/
    └── audit/
        ├── file_store.go
        ├── memory_store.go
        └── workspace_audit.go  # Per-workspace audit support
```
