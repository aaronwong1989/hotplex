# Session Management Commands

This package implements subcommands for managing active agent sessions. These commands are subcommands of `hotplexd session`.

## Commands

- **`session.go`**: Defines the `session` parent command.
- **`list.go`**: Implements `hotplexd session list`, which lists all active sessions with their IDs, status, and last active time.
- **`kill.go`**: Implements `hotplexd session kill <session-id>`, which terminates a specific session.
- **`logs.go`**: Implements `hotplexd session logs <session-id>`, which displays session log information and can optionally stream the log content.

## Integration

These commands interact with the daemon's Admin API via the shared `DoAdminAPI` utility in the [parent package](../).
