# Persistence Package (`internal/persistence`)

Session marker storage abstractions.

## Overview

This package provides session marker persistence, decoupling session management from filesystem operations. This improves testability and enables alternative storage backends.

## Purpose

Session markers track whether a CLI session can be resumed (e.g., Claude Code's `--resume` functionality).

## Interface

```go
type SessionMarkerStore interface {
    // Exists checks if a session marker exists
    Exists(sessionID string) bool

    // Create creates a session marker
    Create(sessionID string) error

    // Delete removes the session marker
    Delete(sessionID string) error

    // Dir returns the base directory
    Dir() string
}
```

## Usage

```go
import "github.com/hrygo/hotplex/internal/persistence"

// Create file-based marker store
store, err := persistence.NewFileMarkerStore("/var/lib/hotplex/markers")

// Check if session can be resumed
if store.Exists(sessionID) {
    // Resume session
}

// Create marker for new session
err := store.Create(sessionID)

// Clean up after session ends
err := store.Delete(sessionID)
```

## Implementation

| Type | Description |
|------|-------------|
| `FileMarkerStore` | Filesystem-based storage (default) |
| `SessionMarkerStore` | Interface for custom backends |

## Files

| File | Purpose |
|------|---------|
| `markers.go` | Marker store interface and file implementation |
