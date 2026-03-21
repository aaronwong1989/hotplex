# hotplexd

`hotplexd` is the main entry point for the HotPlex daemon. It initializes the HotPlex engine, starts the HTTP servers, and manages the overall lifecycle of the agent runtime.

## Files

- **`main.go`**: Contains the `main` function and the `runDaemon` logic. It handles flag parsing, environment loading, and server orchestration.
- **`main_test.go`**: Unit tests for the daemon startup and management logic.

## Daemon Responsibilities

1.  **Environment Loading**: Loads configuration from `.env` files and environment variables.
2.  **Configuration Management**: Resolves and loads the server configuration YAML.
3.  **Engine Initialization**: Sets up the HotPlex `Engine` with the designated provider (e.g., Claude Code).
4.  **HTTP Server Setup**:
    - **Main Server**: Handles WebSocket connections for agents and OpenCode compatibility APIs.
    - **Admin Server**: Provides internal management APIs on a separate port.
5.  **Observability**: Registers health checks and metrics endpoints.
6.  **ChatApps Integration**: Optionally initializes adapters for platforms like Slack or Feishu.

## Related Packages

- [cmd](./cmd): CLI command implementations.
- [internal/server](../../internal/server): HTTP and WebSocket handler logic.
- [internal/config](../../internal/config): Configuration loading and validation.
- [hotplex](../../): Core engine interfaces.
