# HotPlex Commands (cmd)

This directory contains the entry points for the HotPlex AI Agent Runtime Engine.

## [hotplexd](./hotplexd)

`hotplexd` is the main daemon and command-line interface (CLI) for HotPlex. It provides long-lived sessions for AI tools like Claude Code and OpenCode, and manages various chat platform integrations.

### Core Components

- **`main.go`**: The entry point for the daemon and the CLI.
- **`cmd/`**: Implementation of the Cobra-based CLI commands.
- **`cmd/session/`**: Commands for managing active agent sessions.
- **`cmd/cron/`**: Commands for background task scheduling.
- **`cmd/relay/`**: Commands for bot-to-bot cross-platform relaying.

### Usage

#### Starting the Daemon

To run the HotPlex daemon, use the `start` command:

```bash
hotplexd start [flags]
```

**Flags:**
- `--config <path>`: Path to the server configuration YAML file.
- `--env-file <path>`: Path to a `.env` file for environment variables.
- `--admin-port <port>`: Port for the Admin API server (default: 9080).

#### CLI Commands

`hotplexd` also includes several utility commands:

- `hotplexd session`: Manage active sessions (list, kill, logs).
- `hotplexd cron`: Schedule background tasks with AI (add, list, history).
- `hotplexd relay`: Manage cross-platform bot communication bindings.
- `hotplexd status`: Check real-time daemon metrics (CPU, Memory, Sessions).
- `hotplexd config`: Validate configuration files locally or remotely.
- `hotplexd doctor`: Run in-depth diagnostics (Binary, Network, API Health).
- `hotplexd version`: Display version and build information.

### Configuration

`hotplexd` can be configured via:
1.  **Command-line flags**: e.g., `--config`.
2.  **Environment variables**: e.g., `HOTPLEX_API_KEY`, `HOTPLEX_LOG_LEVEL`.
3.  **Config file**: Usually a `config.yaml` file.
4.  **`.env` file**: Key-value pairs for environment variables.

For detailed configuration options, refer to the [internal/config](../internal/config) package or run `hotplexd config --help`.

---
> [!NOTE]
> **Dual Management Modes**: Many `hotplexd` subcommands (session, cron, status, etc.) support both direct local access and remote communication via the **Admin API** (default port 9080). This ensures flexibility for both local development and remote orchestration.
