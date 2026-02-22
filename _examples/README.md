# HotPlex Examples

This directory contains examples of how to use the HotPlex SDK and Proxy Server.

## 📁 Examples Structure

### 1. [Claude Basic (Go)](./go_claude_basic)
A simple Go application demonstrating the basic usage of the `HotPlexClient` with Claude Code CLI.

### 2. [Claude Lifecycle (Go)](./go_claude_lifecycle)
A comprehensive Go demo showing the end-to-end lifecycle of a Claude session:
- **Cold Start**: Initializing a new persistent process.
- **Hot-Multiplexing**: Reusing an existing process for sub-second latency.
- **Process Recovery**: How HotPlex resumes sessions after a "crash" or restart using marker files.
- **Manual Termination**: Explicitly stopping a session.

### 3. [OpenCode Basic (Go)](./go_opencode_basic)
Demonstrates how to use HotPlex with the **OpenCode** CLI agent:
- **Provider Switching**: Seamlessly swapping the underlying AI agent.
- **Plan/Build Modes**: Configuring OpenCode-specific operational modes.
- **Model Configuration**: Overriding default models for the provider.

### 4. [Claude WebSocket Client (Node.js)](./node_claude_websocket)

| File                   | Description                                                                                                |
| :--------------------- | :--------------------------------------------------------------------------------------------------------- |
| `client.js`            | **Quick Start** - Minimal ~50 LOC for getting started in 30 seconds                                        |
| `enterprise_client.js` | **Enterprise** - Production-ready client with reconnection, error handling, metrics, and graceful shutdown |

**Enterprise Features:**
- Automatic reconnection with exponential backoff
- Comprehensive error handling and recovery
- Structured logging with configurable levels
- Connection health monitoring (heartbeat)
- Request timeout management
- Graceful shutdown support (SIGINT/SIGTERM)
- Metrics collection (latency, success rate, reconnect count)
- Progress callbacks for streaming events

---

## 🚀 How to Run

### Prerequisite: Claude Code CLI
Ensure you have the `claude` CLI installed and authenticated.

#### Recommended (Native):
```bash
# macOS / Linux / WSL
curl -fsSL https://claude.ai/install.sh | bash

# Windows (PowerShell)
irm https://claude.ai/install.ps1 | iex
```

#### Alternatives:
```bash
brew install claude-code
# OR
npm install -g @anthropic-ai/claude-code
```

Run authentication:
```bash
claude auth
```

### Running the Go Examples
```bash
# Claude Basic Demo
go run _examples/go_claude_basic/main.go

# Claude Lifecycle Demo
go run _examples/go_claude_lifecycle/main.go

# OpenCode Basic Demo
go run _examples/go_opencode_basic/main.go
```

### Running the WebSocket Examples

1. Start the HotPlex Proxy Server:
   ```bash
   go run cmd/hotplexd/main.go
   ```

2. Run the Node.js client (in another terminal):
   ```bash
   cd _examples/node_claude_websocket
   npm install

   # Quick Start
   node client.js

   # Enterprise Demo
   node enterprise_client.js
   ```

### Using Enterprise Client as a Module
```javascript
const { HotPlexClient } = require('./enterprise_client');

const client = new HotPlexClient({
  url: 'ws://localhost:8080/ws/v1/agent',
  sessionId: 'my-session',
  logLevel: 'info',
  reconnect: { enabled: true, maxAttempts: 5 }
});

await client.connect();

const result = await client.execute('List files in current directory', {
  systemPrompt: 'You are a helpful assistant.',
  onProgress: (event) => {
    if (event.type === 'answer') process.stdout.write(event.data);
  }
});

console.log(result);
await client.disconnect();
```

## 📡 Protocol Notes

### Request-Response Correlation
All WebSocket requests support an optional `request_id` field. The server echoes this ID in responses, enabling proper correlation when sending concurrent requests on the same connection.

```javascript
// Request with request_id
{ "request_id": "req-123", "cmd": "prompt", "data": "..." }

// Response includes the same request_id
{ "request_id": "req-123", "type": "answer", "data": "..." }
```

## ⚙️ Configuration Hints
- **`IDLE_TIMEOUT`**: Set this env var when running `hotplexd` to change how long idle processes stay alive (e.g., `IDLE_TIMEOUT=5m`).
- **`PORT`**: Change the default `8080` port.
