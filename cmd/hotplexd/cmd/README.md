# HotPlex CLI Commands

This package contains the implementation of the `hotplexd` command-line interface using the Cobra library. It provides subcommands for managing the daemon, sessions, and configuration.

## Commands

- **`root.go`**: Defines the `hotplexd` root command and global flags.
- **`config.go`**: Implements configuration-related commands, such as `config validate`.
- **`doctor.go`**: Implements the `doctor` command for environment and dependency diagnostics.
- **`http.go`**: Provides helper functions for interacting with the main daemon's Admin API.
- **`status.go`**: Implements the `status` command to show runtime statistics (uptime, memory, active sessions).
- **`version.go`**: Implements the `version` command to display build information.

## Sub-Packages

- [session](./session): Commands specifically for managing active agent sessions (list, kill, logs).

## Admin API Utility

The `DoAdminAPI` function in `http.go` is a shared utility used by most subcommands to communicate with the `hotplexd` daemon's administrative endpoint. It handles:
- Authentication via `HOTPLEX_ADMIN_TOKEN`.
- Sending HTTP requests to the daemon.
- Basic error handling for connection issues.
