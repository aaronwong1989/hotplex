# ChatApps Reference

The `chatapps` package provides the bridge between HotPlex's core engine and various chat platforms. It normalizes platform-specific events and messages into a unified "Chat Language".

## 🔄 End-to-End Bidirectional Flow

<div class="architecture-diagram" style="margin: 2rem 0; border-radius: 16px; overflow: hidden; background: #0F172A; box-shadow: 0 10px 30px -10px rgba(0,0,0,0.5);">
  <img src="/images/chatapps_flow.svg" alt="HotPlex Architecture Flow" style="display: block; width: 100%; height: auto;" />
</div>

## 🏛 Architecture Overview

HotPlex uses an **Adapter-based Pipeline** architecture.

### Data Normalization

The `chatapps` layer normalizes raw provider events into standard UI components.

| Provider Event       | `base.MessageType`             | UI Presentation     |
| :------------------- | :----------------------------- | :------------------ |
| `thinking`           | `MessageTypeThinking`          | Thinking bubbles    |
| `tool_use`           | `MessageTypeToolUse`           | Tool info block     |
| `tool_result`        | `MessageTypeToolResult`        | Collapsible output  |
| `answer`             | `MessageTypeAnswer`            | Markdown text       |
| `permission_request` | `MessageTypePermissionRequest` | Interactive buttons |

### Key Architectural Concepts

-   **`ChatAdapter`**: The platform-specific connector logic.
-   **`AdapterManager`**: Singleton for managing active connections.
-   **`ProcessorChain`**: Middleware-style pipeline for message styling and filtering.
-   **`AdapterPluginRegistry`**: Global singleton that tracks all registered `AdapterFactory` instances by platform name. New adapters register themselves via `init()` — no central wiring required.

---

## 🛠 Developer Guide

### 1. Implementing a New Platform Adapter

HotPlex uses a **self-registration** pattern: adapters register themselves at startup via `init()`, and `setup.go` discovers them from the global registry — no central wiring is required.

#### Step 1 — Implement `AdapterFactory`

Implement `base.AdapterFactory` in your platform package:

```go
// myplatform/factory.go
package myplatform

import (
    "context"
    "log/slog"
    "os"

    "github.com/hrygo/hotplex/chatapps/base"
)

type AdapterFactory struct{}

var _ base.AdapterFactory = (*AdapterFactory)(nil)

// Platform returns the platform identifier.
func (f *AdapterFactory) Platform() string { return "myplatform" }

// RequiredEnvVars declares environment variables that must be set for this
// platform to start. If any are missing, the platform is silently skipped.
func (f *AdapterFactory) RequiredEnvVars() []string {
    return []string{"HOTPLEX_MYPLATFORM_BOT_TOKEN"}
}

// New creates a new adapter instance from the given platform config.
// Return nil if credentials are missing or the adapter cannot be constructed.
func (f *AdapterFactory) New(pc *base.PlatformConfig) any {
    token := os.Getenv("HOTPLEX_MYPLATFORM_BOT_TOKEN")
    if token == "" {
        return nil
    }

    cfg := &Config{BotToken: token}
    if pc != nil {
        cfg.BotUserID   = pc.Security.Permission.BotUserID
        cfg.SystemPrompt = pc.SystemPrompt
        // ... map other fields
    }
    return NewAdapter(cfg, slog.Default())
}

// PostSetup performs any post-initialization wiring that requires the
// full adapter context (e.g., registering webhooks, publishing AppHome).
// It is called after the adapter is registered with AdapterManager.
func (f *AdapterFactory) PostSetup(_ context.Context, adapter, setupCtx any) {
    ctx, ok := setupCtx.(*base.SetupContext)
    if !ok || ctx == nil || ctx.Logger == nil {
        return
    }
    myAdapter, ok := adapter.(*Adapter)
    if !ok {
        return
    }
    // Example: initialize a capability that needs the Slack client
    ctx.Logger.Info("PostSetup done", "platform", "myplatform")
}

func init() {
    base.GlobalAdapterRegistry().Register(&AdapterFactory{})
}
```

#### Step 2 — Register the Package

In `chatapps/setup.go` (or a `chatapps/extra.go` imported by it), add an import for your package so its `init()` runs:

```go
import (
    _ "github.com/hrygo/hotplex/chatapps/myplatform"
)
```

The import only needs to be present once — the `init()` inside your `factory.go` handles registration automatically.

#### Step 3 — Add Configuration (Optional)

Place a YAML config in your config directory:

```yaml
# configs/chatapps/myplatform.yaml
platform: myplatform
system_prompt: "You are a helpful assistant."
engine:
  timeout: 30m
  idle_timeout: 30m
  work_dir: ${MYPLATFORM_WORK_DIR}
security:
  permission:
    bot_user_id: ${HOTPLEX_MYPLATFORM_BOT_USER_ID}
    dm_policy: allow
    group_policy: allow
features:
  markdown:
    enabled: true
  chunking:
    enabled: true
    max_chars: 4000
```

#### How the Discovery Flow Works

```
setup.go:Setup()
  └─▶ GlobalAdapterRegistry().List()
        → ["slack", "myplatform", ...]

  for each platform:
      factory := Get(platform)       // AdapterFactory for that platform
      adapter := factory.New(pc)     // create adapter instance
      manager.Register(adapter)      // register with AdapterManager
      factory.PostSetup(ctx, ...)    // post-init hook (AppHome, webhooks, etc.)
```

#### Key Design Points

| Concern | How it is handled |
|---------|-------------------|
| Missing credentials | `New()` returns `nil` → platform skipped silently |
| Missing env vars | `RequiredEnvVars()` checked before `New()` |
| Platform wiring | `EngineSupport` interface auto-detected via type assertion in `setup.go` |
| Order of operations | `Register` → `SetHandler` → `PostSetup` |
| Thread safety | `AdapterPluginRegistry` uses `sync.RWMutex` |

---

## 🏗️ Connect More Platforms

<div class="audience-section">
  <div class="audience-card" style="padding: 24px; min-width: 200px;">
    <h3>Slack Guide</h3>
    <p>Step-by-step Slack bot creation and Block Kit setup.</p>
    <a href="/hotplex/guide/chatapps-slack.html" class="audience-btn">View Slack</a>
  </div>
  <div class="audience-card" style="padding: 24px; min-width: 200px;">
    <h3>Engine Manual</h3>
    <p>Understand how messages are processed by the core.</p>
    <a href="/hotplex/reference/engine.html" class="audience-btn">View Engine</a>
  </div>
</div>

> "Interfaces are the grammar of software architecture." — The HotPlex Core Team
