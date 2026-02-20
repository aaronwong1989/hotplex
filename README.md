# 🔥 HotPlex (Hot-Multiplexer)

*Read this in other languages: [English](README.md), [简体中文](README_zh.md).*

**HotPlex** is a high-performance **Process Multiplexer** designed specifically for running heavy, local AI CLI Agents (like Claude Code, OpenCode, Aider) in long-lived server or web environments. 

It solves the "Cold Start" problem by keeping the underlying heavy Node.js or Python CLI processes alive and routing concurrent request streams (Hot-Multiplexing) into their Stdin/Stdout pipes.

## 🚀 Why HotPlex?

Running local CLI agents from a backend service (like a Go API) usually means spawning a new OS process for *every single interaction*. 

*   **The Problem:** Tools like `claude` (Claude Code) are heavy Node.js applications. Firing up `npx @anthropic-ai/claude-code` takes **3-5 seconds** just to boot up the V8 engine, read the filesystem context, and authenticate. For a real-time web UI, this latency makes the agent feel incredibly slow and unresponsive.
*   **The Solution:** HotPlex boots the CLI process *once* per user/session, keeps it alive in the background (within a secure `pgid`), and establishes a persistent pipeline. When the user sends a new message, HotPlex instantly injects it via `Stdin` and streams the JSON responses back via `Stdout`. Latency drops from **5000ms to < 200ms**.

## 💡 Vision & Application Scenarios

The original driving force behind HotPlex is to **empower AI applications to effortlessly integrate powerful CLI agents** (like Claude Code) as their external "muscles." Instead of reinventing the wheel to build coding, execution, and file-manipulation capabilities from scratch, your AI app can instantly borrow the immense capabilities of these mature CLI tools.

Key Application Scenarios include:

- **Web-based AI Agents**: Build a fully functional Web version of Claude Code. Users interact via a sleek browser UI while HotPlex reliably manages the persistent Claude CLI process in a sandboxed backend environment.
- **DevOps Toolchains**: Integrate AI directly into your DevOps workflows. Have an agent autonomously execute shell scripts, read Kubernetes logs, and troubleshoot infrastructure issues over a persistent HotPlex session.
- **CI/CD Pipelines**: Embed intelligent code review, automated testing, and dynamic bug fixing right into your Jenkins, GitLab, or GitHub Actions pipelines without the latency overhead of spinning up heavy Node.js tools repeatedly.
- **Intelligent Operations (AIOps)**: Create intelligent ops-bots that continuously monitor systems, analyze incident reports, and autonomously execute safe remediation commands via a controlled, hot-multiplexed terminal session.

## 🛠 Features

- **Blazing Fast Hot-Starts:** Instant response times after the initial boot.
- **Session Pooling (GC):** Automatically tracks idle processes and terminates them after a configurable timeout (default 30m) to save RAM.
- **WebSocket Gateway:** Includes a standalone batteries-included server (`hotplexd`) that exposes the multiplexer natively over WebSockets for consumption by React/Vue frontends or remote Python/Node scripts.
- **Native Go SDK:** Import `github.com/hrygo/hotplex/pkg/hotplex` to embed the engine directly into your Go backend.
- **Regex Security Firewall:** Built-in `danger.go` pre-flight interceptor blocks destructive commands (`rm -rf /`, fork bombs, reverse shells) before they even reach the agent.
- **Context Isolation:** Uses UUID v5 deterministic namespaces to guarantee sandboxed session isolation (e.g., separating user workspaces).

## 📦 Architecture

HotPlex is designed with a two-tier architecture:

1.  **Core SDK (`pkg/hotplex`)**: The engine itself. It provides the `Engine` Singleton, `SessionPool`, and `Detector` (Security Firewall). It expects JSON streams from the CLI and emits strongly-typed Go events.
2.  **Standalone Server (`cmd/hotplexd`)**: A lightweight wrapper around the SDK that exposes it over standard WebSockets.

*Note: The current MVP is deeply optimized for **Claude Code's** (`--output-format stream-json`) protocol but is designed with a future `Provider` interface abstraction in mind to support OpenCode and Aider.*

## ⚡ Quick Start

### 1. Running the Standalone WebSocket Server

If you just want to run the server and connect to it from a frontend or Python script:

```bash
# Ensure Claude Code is installed globally
npm install -g @anthropic-ai/claude-code

# Build and run the daemon
cd cmd/hotplexd
go build -o hotplexd main.go
./hotplexd
```
Server runs on `ws://localhost:8080/ws/v1/agent`. Check `_examples/websocket_client/client.js` for an integration demo.

### 2. Using the Go SDK Native Integration

See `_examples/basic_sdk/main.go` for a full example.

```go
import "github.com/hrygo/hotplex/pkg/hotplex"

opts := hotplex.EngineOptions{
    Timeout: 5 * time.Minute,
    Logger:  logger,
    // InputCostPerMillion: 3.0, // Configure token pricing
}

engine, _ := hotplex.NewEngine(opts)
defer engine.Close()

cfg := &hotplex.Config{
    Mode:      "MVP",
    WorkDir:   "/tmp",
    SessionID: "user_123_session", // Deterministic Hot-Multiplexing ID
}

ctx := context.Background()

// 1. Send Prompt & handle streaming callback
err := engine.Execute(ctx, cfg, "Calculate 20*5 silently", func(eventType string, data any) error {
    if eventType == "assistant" {
         fmt.Println("Agent generated text...")
    }
    return nil
})
```

## 🔒 Security Posture

HotPlex executes LLM-generated shell code on your machine. **Use with caution.**

We mitigate risks via:
1.  **Context WorkDirs:** The agent is sandboxed to the `WorkDir` provided in the Config.
2.  **Danger Detector:** A regex-based "WAF" (Web Application Firewall) intercepts and blocks destructive patterns (e.g., `mkfs`, `dd`, `rm -rf /*`, `chmod 000 /`) before they reach the OS.
3.  **Process Groups (PGID):** When a session is terminated, HotPlex sends `SIGKILL` to the entire negative Process Group ID (`-pgid`), guaranteeing that the CLI, any detached bash scripts, and grandchild processes are instantly eradicated without leaving zombies.

## Roadmap
- [ ] Provider interface extraction (support for `OpenCode`)
- [ ] Remote Docker sandbox execution (replacing local OS execution)
- [ ] REST API endpoints for session introspection
