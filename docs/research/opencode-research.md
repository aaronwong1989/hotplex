# OpenCode Comprehensive Research Report

**Research Date**: 2026-03-29
**OpenCode Version**: 1.3.3
**Installation Path**: `/Users/huangzhonghui/.opencode/bin/opencode`

---

## Executive Summary

OpenCode is a **terminal-based AI coding assistant** (NOT the OpenCode editor/IDE). It's a production-ready CLI tool similar to Claude Code, supporting both interactive TUI mode and programmatic access via CLI flags and server mode.

---

## 1. Architecture Overview

### What is OpenCode?

OpenCode is an **AI-powered coding agent** that runs in the terminal. It provides:
- Interactive TUI (Terminal User Interface) - default mode
- Programmatic CLI execution via `opencode run`
- Headless server mode via `opencode serve` (Web UI + API)
- Agent Client Protocol (ACP) server for IDE integration
- Model Context Protocol (MCP) support for tool extensions

### Core Components

```
OpenCode Architecture
├── CLI Entry Point
│   ├── opencode [project]        # TUI mode (default)
│   ├── opencode run              # Non-interactive execution
│   ├── opencode serve            # HTTP/WebSocket server
│   ├── opencode acp              # ACP server for IDEs
│   └── opencode mcp              # MCP server management
│
├── Session Management
│   ├── SQLite Database (~/.local/share/opencode/opencode.db)
│   ├── Session persistence & history
│   ├── Message threading
│   └── Todo tracking
│
├── Agent System (via oh-my-opencode plugin)
│   ├── Sisyphus (Orchestrator)
│   ├── Atlas (Conductor)
│   ├── Prometheus (Strategy)
│   ├── Oracle (Deep Reasoning)
│   ├── Metis (Risk Control)
│   ├── Momus (Quality Reviewer)
│   ├── Librarian (RAG/Retrieval)
│   ├── Looker (Vision)
│   ├── Writer (Content)
│   └── Explore (Codebase Scanner)
│
└── Provider System
    ├── Multi-provider support (Anthropic, OpenAI, Google, etc.)
    ├── Chinese providers (GLM, Qwen, MiniMax, Kimi)
    ├── Model hot-swapping
    └── Cost optimization routing
```

---

## 2. CLI Mode (`opencode run`)

### 2.1 Basic Usage

```bash
# Interactive message
opencode run "your message here"

# With specific model
opencode run "fix the bug" -m bailian-coding-plan/glm-5

# Continue last session
opencode run "continue" -c

# Resume specific session
opencode run "continue" -s ses_2c622e806ffeMffpTI5QjHGdo0

# Fork session (create new from existing)
opencode run "new direction" -s ses_xxx --fork

# Attach files
opencode run "analyze this" -f file1.py -f file2.go

# Set session title
opencode run "task" --title "My Custom Title"
```

### 2.2 Output Formats

**Critical Finding**: OpenCode supports **two output formats**:

#### Format 1: Default (Human-Readable)
```bash
opencode run "what is 2+2"
# Returns formatted text with colors and markdown
```

#### Format 2: JSON (Machine-Readable)
```bash
opencode run "what is 2+2" --format json
```

**JSON Event Structure**:
```json
{
  "type": "error|message|tool_call|step_start|step_finish",
  "timestamp": 1774792284283,
  "sessionID": "ses_2c622e806ffeMffpTI5QjHGdo0",
  "error": {
    "name": "UnknownError",
    "data": {
      "message": "Model not found"
    }
  }
}
```

**Event Types**:
- `error`: Error events
- `message`: Text responses
- `tool_call`: Tool invocation events
- `step_start`: Agent step begins
- `step_finish`: Agent step completes
- `reasoning`: Thinking blocks (with `--thinking` flag)

### 2.3 Complete CLI Flags

```bash
opencode run [message..] [options]

Positionals:
  message         Message to send (space-separated words)  [array] [default: []]

Options:
  --format        Output format: "default" (formatted) or "json" (raw JSON events)
                                                   [string] [choices: "default", "json"]
  -m, --model     Model in format provider/model   [string]
  -c, --continue  Continue the last session        [boolean]
  -s, --session   Session ID to continue           [string]
  --fork          Fork session when continuing     [boolean]
  --share         Share the session                [boolean]
  -f, --file      Files to attach to message       [array]
  --title         Title for the session            [string]
  --attach        Attach to running server (e.g., http://localhost:4096)  [string]
  -p, --password  Basic auth password (defaults to OPENCODE_SERVER_PASSWORD)  [string]
  --dir           Directory to run in              [string]
  --port          Port for local server            [number]
  --agent         Agent to use                     [string]
  --variant       Model variant (e.g., high, max, minimal)  [string]
  --thinking      Show thinking blocks             [boolean] [default: false]
  --command       The command to run               [string]
```

### 2.4 Session Management

```bash
# List sessions
opencode session list

# Delete session
opencode session delete <session-id>

# Export session to JSON
opencode export <session-id>

# Import session from JSON
opencode import <file.json>
```

### 2.5 Programmatic Integration Example

```bash
# Example: Non-interactive execution with JSON parsing
RESULT=$(opencode run "list all TODO comments" --format json -m minimax-cn-coding-plan/MiniMax-M2.5-highspeed)

# Parse JSON events
echo "$RESULT" | jq -r 'select(.type == "message") | .content'
```

---

## 3. Server Mode (`opencode serve`)

### 3.1 Basic Usage

```bash
# Start server with random port
opencode serve

# Start with specific port
opencode serve --port 4096

# Bind to all interfaces (for remote access)
opencode serve --hostname 0.0.0.0 --port 4096

# With authentication
OPENCODE_SERVER_PASSWORD="secret123" opencode serve --port 4096

# Enable mDNS discovery
opencode serve --mdns --mdns-domain "opencode.local"

# Add CORS domains
opencode serve --cors "https://example.com" --cors "https://app.example.com"

# With debug logging
opencode serve --print-logs --log-level DEBUG
```

### 3.2 Server Configuration

```bash
opencode serve [options]

Options:
  --port         Port to listen on                           [number] [default: 0 (random)]
  --hostname     Hostname to bind                            [string] [default: "127.0.0.1"]
  --mdns         Enable mDNS service discovery               [boolean] [default: false]
  --mdns-domain  Custom mDNS domain                          [string] [default: "opencode.local"]
  --cors         Additional CORS domains                     [array] [default: []]
  --print-logs   Print logs to stderr                        [boolean]
  --log-level    Log level (DEBUG, INFO, WARN, ERROR)        [string]
```

### 3.3 HTTP API Endpoints

**Critical Finding**: The HTTP server primarily serves a **Web UI** (SPA). All routes return HTML.

**Available Endpoints**:
```
GET /                  # Web UI (Single Page Application)
GET /health            # Health check (returns HTML, not JSON)
GET /api/*             # All return Web UI (SPA routing)
```

**Authentication**:
- If `OPENCODE_SERVER_PASSWORD` is set, requires Basic Auth
- Password passed via `-p` flag or environment variable

**Example**:
```bash
# Start server
opencode serve --port 4096 &

# Access requires browser (Web UI)
open http://localhost:4096

# With authentication
curl -u "user:secret123" http://localhost:4096/
```

### 3.4 WebSocket API

**Status**: Research incomplete - server uses WebSocket for real-time communication, but protocol details require source code inspection or reverse engineering.

**Observed Events** (from debug logs):
```
session.updated
message.updated
message.part.updated
session.diff
file.watcher.updated
```

### 3.5 mDNS Discovery

When `--mdns` is enabled:
- Service broadcasts as `_opencode._tcp.local`
- Accessible via `http://opencode.local:4096` (or custom domain)
- Defaults hostname to `0.0.0.0` (all interfaces)

---

## 4. ACP (Agent Client Protocol) Mode

### 4.1 What is ACP?

ACP is a **protocol for IDE integration**. It allows IDEs (like VSCode, Cursor) to communicate with AI agents.

### 4.2 Usage

```bash
# Start ACP server
opencode acp

# With options
opencode acp --port 5000 --hostname 127.0.0.1 --cwd /path/to/project
```

### 4.3 ACP Server Options

```bash
opencode acp [options]

Options:
  --port         Port to listen on         [number] [default: 0 (random)]
  --hostname     Hostname to bind          [string] [default: "127.0.0.1"]
  --mdns         Enable mDNS discovery     [boolean] [default: false]
  --mdns-domain  Custom mDNS domain        [string] [default: "opencode.local"]
  --cors         Additional CORS domains   [array] [default: []]
  --cwd          Working directory         [string] [default: current directory]
```

**Note**: ACP protocol specification is at https://spec.agentclientprotocol.org/ (site was unreachable during research)

---

## 5. MCP (Model Context Protocol) Support

### 5.1 What is MCP?

MCP is a protocol for **extending AI agents with external tools**. OpenCode can connect to MCP servers to access additional capabilities.

### 5.2 Usage

```bash
# List MCP servers
opencode mcp list

# Add MCP server
opencode mcp add

# Authenticate with OAuth-enabled MCP server
opencode mcp auth [name]

# Logout from MCP server
opencode mcp logout [name]

# Debug OAuth connection
opencode mcp debug <name>
```

---

## 6. Database & Storage

### 6.1 Database Location

```
~/.local/share/opencode/opencode.db
```

### 6.2 Database Schema

**Tables**:
- `session`: Session metadata
- `message`: Messages within sessions
- `part`: Message parts (text, tool calls, etc.)
- `project`: Project metadata
- `todo`: Todo items
- `permission`: Permission settings
- `session_share`: Shared session metadata
- `workspace`: Workspace configuration
- `account`: Account information
- `event`: Event log
- `event_sequence`: Event sequencing

**Key Fields**:
```sql
-- Session table
CREATE TABLE session (
  id TEXT PRIMARY KEY,
  project_id TEXT NOT NULL,
  parent_id TEXT,
  slug TEXT NOT NULL,
  directory TEXT NOT NULL,
  title TEXT NOT NULL,
  version TEXT NOT NULL,
  share_url TEXT,
  time_created INTEGER,
  time_updated INTEGER,
  ...
);

-- Message table
CREATE TABLE message (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL,
  time_created INTEGER,
  data TEXT NOT NULL,  -- JSON blob
  ...
);
```

### 6.3 Exported Session Format

```json
{
  "info": {
    "id": "ses_2d2801166ffen46ZhSG8jANSk7",
    "slug": "quiet-planet",
    "projectID": "f6f336a4921d9a99e1388cc11f3a1456c1f4fce1",
    "directory": "/Users/huangzhonghui/hotplex",
    "title": "Listing /tmp directory",
    "version": "1.3.2",
    "summary": {
      "additions": 0,
      "deletions": 0,
      "files": 0
    },
    "time": {
      "created": 1774584852121,
      "updated": 1774584871427
    }
  },
  "messages": [
    {
      "info": {
        "role": "user",
        "time": {"created": 1774584852149},
        "agent": "Sisyphus (Ultraworker)",
        "model": {
          "providerID": "minimax-cn-coding-plan",
          "modelID": "MiniMax-M2.5"
        },
        "id": "msg_d2d7feeb5001wmNjxXvZiIN0BR",
        "sessionID": "ses_2d2801166ffen46ZhSG8jANSk7"
      },
      "parts": [
        {
          "type": "text",
          "text": "List /tmp",
          "id": "prt_d2d7feeb5002ZcyX2zal2IYRI7",
          "sessionID": "ses_2d2801166ffen46ZhSG8jANSk7",
          "messageID": "msg_d2d7feeb5001wmNjxXvZiIN0BR"
        }
      ]
    },
    {
      "info": {
        "role": "assistant",
        "time": {
          "created": 1774584852151,
          "completed": 1774584857144
        },
        "parentID": "msg_d2d7feeb5001wmNjxXvZiIN0BR",
        "modelID": "MiniMax-M2.5",
        "providerID": "minimax-cn-coding-plan",
        "mode": "Sisyphus (Ultraworker)",
        "cost": 0,
        "tokens": {
          "total": 37078,
          "input": 35543,
          "output": 63,
          "reasoning": 0,
          "cache": {
            "read": 1472,
            "write": 0
          }
        },
        "finish": "tool-calls"
      },
      "parts": [
        {
          "type": "step-start",
          "snapshot": "a953388ec98c5aebace78dff2c7f639760af9b31"
        },
        {
          "type": "reasoning",
          "text": "The user wants to list files...",
          "metadata": {
            "anthropic": {
              "signature": "02e34d0535f61651507d8c0115c566db5a9cade2ec6bb72b2bcaf69c7081f1ac"
            }
          },
          "time": {
            "start": 1774584856339,
            "end": 1774584856568
          }
        },
        {
          "type": "tool",
          "callID": "call_function_knsddi0re5gd_1",
          "tool": "bash",
          "state": {
            "status": "completed",
            "input": {
              "command": "ls -la /tmp",
              "description": "List files in /tmp directory"
            },
            "output": "lrwxr-xr-x@ 1 root  wheel  11 Feb 25 11:41 /tmp -> private/tmp\n",
            "metadata": {
              "output": "lrwxr-xr-x@ 1 root  wheel  11 Feb 25 11:41 /tmp -> private/tmp\n",
              "exit": 0,
              "description": "List files in /tmp directory",
              "truncated": false
            },
            "time": {
              "start": 1774584857011,
              "end": 1774584857027
            }
          }
        },
        {
          "type": "step-finish",
          "reason": "tool-calls",
          "cost": 0,
          "tokens": {...}
        }
      ]
    }
  ]
}
```

---

## 7. CLI vs Server Mode Comparison

| Aspect | CLI Mode (`opencode run`) | Server Mode (`opencode serve`) |
|--------|---------------------------|--------------------------------|
| **Primary Use** | Programmatic execution, CI/CD, scripts | Web UI access, remote usage |
| **Output Format** | JSON (`--format json`) or formatted text | WebSocket events (UI-only) |
| **Session Access** | Direct via CLI flags | Via Web UI only |
| **Authentication** | Not required | Optional (Basic Auth) |
| **Concurrency** | Single session per invocation | Multiple sessions via UI |
| **Performance** | Low overhead (direct execution) | Higher overhead (HTTP + WebSocket) |
| **Feature Parity** | Full feature access | Limited to Web UI features |
| **Programmatic Access** | ✅ Excellent (JSON output) | ❌ Poor (UI-only, WebSocket protocol undocumented) |
| **Production Use** | ✅ Recommended | ⚠️ For human users only |
| **IDE Integration** | ❌ Not applicable | ✅ Via ACP (`opencode acp`) |

### Recommendation for HotPlex

**For HotPlex integration**: Use **CLI mode** (`opencode run --format json`)

**Reasons**:
1. ✅ Structured JSON output for parsing
2. ✅ Session management via CLI flags
3. ✅ Stateless execution (easier to manage)
4. ✅ No authentication overhead
5. ✅ Direct access to all features
6. ❌ Server mode WebSocket protocol is undocumented

---

## 8. Session Export Format (Detailed)

### 8.1 Message Part Types

```typescript
type PartType =
  | "text"           // User/assistant text
  | "reasoning"      // Thinking blocks
  | "tool"           // Tool calls
  | "step-start"     // Agent step begins
  | "step-finish"    // Agent step ends
  | "image"          // Image content
```

### 8.2 Token Tracking

```json
{
  "tokens": {
    "total": 37078,
    "input": 35543,
    "output": 63,
    "reasoning": 0,
    "cache": {
      "read": 1472,
      "write": 0
    }
  }
}
```

### 8.3 Tool Call Structure

```json
{
  "type": "tool",
  "callID": "call_function_xxx",
  "tool": "bash|read|write|edit|glob|grep|...",
  "state": {
    "status": "completed|running|error",
    "input": { /* tool-specific input */ },
    "output": "string result",
    "metadata": {
      "exit": 0,
      "truncated": false
    },
    "time": {
      "start": timestamp,
      "end": timestamp
    }
  }
}
```

---

## 9. Configuration System

### 9.1 Configuration Paths

```
~/.config/opencode/
├── opencode.json              # Main config (providers, models, API keys)
├── opencode.jsonc             # JSON with comments (alternative)
├── oh-my-opencode.jsonc       # Agent configuration
├── antigravity-accounts.json  # Authentication tokens
└── plans/                     # Configuration presets

~/.local/share/opencode/
├── opencode.db                # SQLite database
└── log/                       # Log files

~/.cache/opencode/
└── bin/                       # Cached binaries (node modules, etc.)
```

### 9.2 Provider Configuration Example

```json
{
  "$schema": "https://opencode.ai/config.json",
  "provider": {
    "bailian-coding-plan": {
      "npm": "@ai-sdk/anthropic",
      "name": "Model Studio Coding Plan",
      "options": {
        "apiKey": "sk-xxx",
        "baseURL": "https://coding.dashscope.aliyuncs.com/apps/anthropic/v1"
      },
      "models": {
        "qwen3.5-plus": {
          "name": "Qwen3.5 Plus",
          "modalities": {
            "input": ["text", "image"],
            "output": ["text"]
          },
          "options": {
            "thinking": {
              "type": "enabled",
              "budgetTokens": 1024
            }
          }
        }
      }
    }
  },
  "model": "bailian-coding-plan/qwen3.5-plus"
}
```

---

## 10. Other Useful Commands

### 10.1 Statistics & Monitoring

```bash
# Token usage and cost statistics
opencode stats

# Output:
# ┌────────────────────────────────────────────────────────┐
# │                       OVERVIEW                         │
# ├────────────────────────────────────────────────────────┤
# │Sessions                                            148 │
# │Messages                                          5,501 │
# │Days                                                 31 │
# └────────────────────────────────────────────────────────┘
```

### 10.2 Model Management

```bash
# List all available models
opencode models

# List models for specific provider
opencode models bailian-coding-plan

# Manage providers and credentials
opencode providers
opencode providers add
```

### 10.3 Agent Management

```bash
# Manage agents
opencode agent

# Debug agent configuration
opencode debug agent "Sisyphus (Ultraworker)"
```

### 10.4 Debugging & Diagnostics

```bash
# Show resolved configuration
opencode debug config

# Show global paths
opencode debug paths
# Output:
# home       /Users/huangzhonghui
# data       /Users/huangzhonghui/.local/share/opencode
# bin        /Users/huangzhonghui/.cache/opencode/bin
# log        /Users/huangzhonghui/.local/share/opencode/log
# cache      /Users/huangzhonghui/.cache/opencode
# config     /Users/huangzhonghui/.config/opencode
# state      /Users/huangzhonghui/.local/state/opencode

# LSP debugging
opencode debug lsp

# Ripgrep debugging
opencode debug rg

# File system debugging
opencode debug file

# List all known projects
opencode debug scrap

# List all available skills
opencode debug skill
```

### 10.5 GitHub Integration

```bash
# Manage GitHub agent
opencode github

# Fetch PR and run opencode
opencode pr <number>
```

---

## 11. Integration Recommendations for HotPlex

### 11.1 Recommended Approach: CLI Mode

```go
// Pseudocode for HotPlex integration
type OpenCodeProvider struct {
    BinaryPath string
    Model      string
}

func (p *OpenCodeProvider) Execute(ctx context.Context, input string) (*Session, error) {
    // 1. Create temp file for session ID (if resuming)
    sessionFile := "/tmp/opencode-session.txt"

    // 2. Build command
    cmd := exec.CommandContext(ctx,
        p.BinaryPath, "run",
        "--format", "json",
        "--model", p.Model,
        "--session", sessionID, // optional
        input,
    )

    // 3. Capture output
    output, err := cmd.Output()
    if err != nil {
        return nil, err
    }

    // 4. Parse JSON events
    var events []Event
    for _, line := range strings.Split(string(output), "\n") {
        if line == "" {
            continue
        }
        var event Event
        if err := json.Unmarshal([]byte(line), &event); err != nil {
            continue
        }
        events = append(events, event)
    }

    // 5. Extract session ID, messages, tool calls
    return parseEvents(events)
}
```

### 11.2 Session Management

```go
// Resume session
func (p *OpenCodeProvider) Resume(sessionID string, input string) (*Session, error) {
    cmd := exec.Command(
        p.BinaryPath, "run",
        "--format", "json",
        "-s", sessionID,
        input,
    )
    // ...
}

// Fork session
func (p *OpenCodeProvider) Fork(sessionID string, input string) (*Session, error) {
    cmd := exec.Command(
        p.BinaryPath, "run",
        "--format", "json",
        "-s", sessionID,
        "--fork",
        input,
    )
    // ...
}
```

### 11.3 Streaming Support

**Challenge**: `opencode run` does NOT support streaming output. It returns all events at once after completion.

**Workaround**: For streaming, you would need to:
1. Use `opencode serve` and reverse-engineer WebSocket protocol, OR
2. Poll session database directly, OR
3. Implement chunking (split large requests into smaller ones)

### 11.4 Error Handling

```go
type Event struct {
    Type      string          `json:"type"`
    Timestamp int64           `json:"timestamp"`
    SessionID string          `json:"sessionID"`
    Error     *ErrorData      `json:"error,omitempty"`
    Message   *MessageData    `json:"message,omitempty"`
    ToolCall  *ToolCallData   `json:"tool_call,omitempty"`
}

type ErrorData struct {
    Name string                 `json:"name"`
    Data map[string]interface{} `json:"data"`
}

func parseEvents(events []Event) (*Session, error) {
    for _, event := range events {
        if event.Type == "error" {
            return nil, fmt.Errorf("opencode error: %s - %v",
                event.Error.Name,
                event.Error.Data["message"],
            )
        }
    }
    // ...
}
```

---

## 12. Known Limitations

1. **No Streaming in CLI Mode**: `opencode run` returns all output at once
2. **Undocumented WebSocket Protocol**: Server mode WebSocket API is not publicly documented
3. **HTML-Only HTTP API**: All HTTP endpoints return HTML (Web UI), not JSON
4. **No Built-in REST API**: Server mode is UI-focused, not API-focused
5. **Session ID Required for Resume**: Cannot list sessions programmatically (must use `opencode session list` CLI)
6. **Provider-Specific Quirks**: Different providers may have different behaviors

---

## 13. Future Research Needed

1. **WebSocket Protocol**: Reverse-engineer server mode WebSocket events
2. **ACP Protocol**: Obtain specification from https://spec.agentclientprotocol.org/
3. **Streaming Workarounds**: Investigate database polling for streaming
4. **Performance Benchmarks**: Measure latency and throughput vs Claude Code
5. **Error Recovery**: Test error handling and retry mechanisms

---

## 14. Summary for HotPlex Team

### Recommended Integration Path

**Use CLI mode (`opencode run --format json`)**

**Pros**:
- ✅ Clean JSON output
- ✅ Session persistence
- ✅ Multi-provider support
- ✅ Well-documented flags
- ✅ No authentication overhead

**Cons**:
- ❌ No streaming (batch output only)
- ❌ No HTTP API (must spawn process)
- ❌ Session listing requires separate command

### Architecture Decision

```
HotPlex Architecture with OpenCode
┌─────────────────────────────────────────────────┐
│                 HotPlex Engine                   │
│  ┌────────────────────────────────────────────┐ │
│  │         Provider Interface                  │ │
│  │  ┌──────────────┐  ┌──────────────────┐   │ │
│  │  │ Claude Code  │  │    OpenCode      │   │ │
│  │  │   Provider   │  │    Provider      │   │ │
│  │  └──────────────┘  └──────────────────┘   │ │
│  │         │                    │             │ │
│  │         │                    │             │ │
│  │         ▼                    ▼             │ │
│  │    CLI Binary          CLI Binary          │ │
│  │  (claude-code)        (opencode)           │ │
│  └────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────┘
```

### Implementation Checklist

- [ ] Implement `OpenCodeProvider` struct
- [ ] Add `opencode` binary path detection
- [ ] Parse JSON event stream
- [ ] Map OpenCode events to HotPlex events
- [ ] Handle session persistence
- [ ] Test error scenarios
- [ ] Benchmark performance
- [ ] Document provider-specific quirks

---

## Appendix A: Command Reference

### Core Commands

```bash
opencode [project]                    # Start TUI (default)
opencode run [message..]              # Non-interactive execution
opencode serve                        # HTTP/WebSocket server
opencode acp                          # ACP server for IDEs
opencode web                          # Server + open browser
opencode attach <url>                 # Attach to remote server
```

### Session Management

```bash
opencode session list                 # List sessions
opencode session delete <id>          # Delete session
opencode export [sessionID]           # Export to JSON
opencode import <file>                # Import from JSON
```

### Configuration

```bash
opencode providers                    # Manage providers
opencode models [provider]            # List models
opencode agent                        # Manage agents
opencode mcp                          # Manage MCP servers
```

### Diagnostics

```bash
opencode stats                        # Token/cost stats
opencode debug config                 # Show config
opencode debug paths                  # Show paths
opencode db [query]                   # Database shell
opencode db path                      # Database path
opencode db migrate                   # Migrate JSON to SQLite
```

### System

```bash
opencode upgrade [target]             # Upgrade binary
opencode uninstall                    # Uninstall
opencode --version                    # Show version
opencode --help                       # Show help
```

---

## Appendix B: Environment Variables

```bash
OPENCODE_SERVER_PASSWORD    # Server authentication password
OPENCODE_CONFIG_PATH        # Custom config path
OPENCODE_DATA_PATH          # Custom data path
OPENCODE_LOG_LEVEL          # Log level (DEBUG, INFO, WARN, ERROR)
```

---

## Appendix C: File Paths

```
~/.config/opencode/         # Configuration directory
~/.local/share/opencode/    # Data directory (database, logs)
~/.cache/opencode/          # Cache directory (binaries)
~/.local/state/opencode/    # State directory
~/.opencode/bin/            # Binary installation
```

---

**Report Compiled By**: Claude Code Agent
**Research Method**: Direct binary testing, database inspection, log analysis
**Confidence Level**: High (based on hands-on testing)
**Gaps**: WebSocket protocol, ACP specification
