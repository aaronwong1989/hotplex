# HotPlex Commands (cmd)

This directory contains the entry points for the HotPlex AI Agent Runtime Engine.

## [hotplexd](./hotplexd)

`hotplexd` is the main daemon and command-line interface (CLI) for HotPlex. It provides long-lived sessions for AI tools like Claude Code and OpenCode, and manages various chat platform integrations.

### Core Components

- **`main.go`**: The entry point for the daemon and the CLI.
- **`cmd/`**: Implementation of the Cobra-based CLI commands.
- **`cmd/session/`**: Commands for managing active agent sessions.

### Usage

#### Starting the Daemon

To run the HotPlex daemon, use the `start` command:

```bash
hotplexd start [flags]
```

**Flags:**
- `--config <path>`: Path to the server configuration YAML file.
- `--env-file <path>`: Path to a `.env` file for environment variables.
- `--admin-port <port>`: Port for the Admin API server (default: 8081).

#### CLI Commands

`hotplexd` also includes several utility commands:

- `hotplexd session`: Manage active sessions (list, kill, logs).
- `hotplexd status`: Check the status of the daemon and active sessions.
- `hotplexd config`: View or modify configuration settings.
- `hotplexd doctor`: Run diagnostic checks on the system environment.
- `hotplexd version`: Display version and build information.

### Configuration

`hotplexd` can be configured via:
1.  **Command-line flags**: e.g., `--config`.
2.  **Environment variables**: e.g., `HOTPLEX_API_KEY`, `HOTPLEX_LOG_LEVEL`.
3.  **Config file**: Usually a `config.yaml` file.
4.  **`.env` file**: Key-value pairs for environment variables.

For detailed configuration options, refer to the [internal/config](../internal/config) package or run `hotplexd config --help`.
