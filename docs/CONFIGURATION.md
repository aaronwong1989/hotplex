# HotPlex Configuration Reference

> Complete reference for all configuration options.
> For quick start, see [development.md](development.md).
>
> **[中文版](configuration_zh.md)** | **English**

## Table of Contents

- [Configuration Layers](#configuration-layers)
- [Environment Variables](#environment-variables)
- [YAML Configuration](#yaml-configuration)
- [Configuration Inheritance](#configuration-inheritance)
- [Role Templates](#role-templates)
- [Admin Bot Configuration](#admin-bot-configuration)
- [Platform-Specific Configs](#platform-specific-configs)
- [Examples](#examples)

---

## Configuration Layers

HotPlex uses a layered configuration system with the following priority (highest to lowest):

```
1. Command-line flags     (--config, --env-file, --admin-port)
2. Environment variables  (HOTPLEX_*)
3. YAML config files      (configs/base/*.yaml, configs/admin/*.yaml)
4. Default values         (built-in defaults)
```

### Layer Purposes

| Layer          | Contents                                                        |
| -------------- | ---------------------------------------------------------------|
| **`.env`**     | Global parameters, bot credentials, secrets, persistence config |
| **YAML files** | Platform behavior, permissions, features, AI prompts            |

### Configuration Directory Structure (v0.33.0+)

```
configs/
├── base/                    # SSOT: Base configuration templates
│   ├── server.yaml          # Core server configuration
│   ├── slack.yaml           # Slack adapter configuration
│   ├── feishu.yaml          # Feishu adapter configuration
│   └── README.md            # Base config documentation
├── admin/                   # Admin bot configuration
│   ├── slack.yaml           # inherits: ../base/slack.yaml
│   └── server.yaml          # inherits: ../base/server.yaml
└── templates/               # Role templates for new instances
    └── roles/              # Role-specific system prompts
        ├── go.yaml         # Go Backend Engineer template
        ├── frontend.yaml   # React/Next.js Frontend Engineer
        ├── devops.yaml     # Docker/K8s DevOps Engineer
        └── custom.yaml     # User-defined template
```

> **Note**: The `configs/chatapps/` directory has been deprecated. Use `configs/base/` for all base configurations.

---

## Environment Variables

### Core Server

| Variable             | Default      | Description                         |
| -------------------- | ------------ | ----------------------------------- |
| `HOTPLEX_PORT`       | `8080`       | Server listen port                  |
| `HOTPLEX_LOG_LEVEL`  | `INFO`       | Log level: DEBUG, INFO, WARN, ERROR |
| `HOTPLEX_LOG_FORMAT` | `json`       | Log format: json, text              |
| `HOTPLEX_API_KEY`    | *(required)* | API security token                  |
| `HOTPLEX_API_KEYS`   | *(optional)* | Multiple API keys (comma-separated) |

### Engine

| Variable                    | Default | Description                |
| --------------------------- | ------- | -------------------------- |
| `HOTPLEX_EXECUTION_TIMEOUT` | `30m`   | Max wait for AI response   |
| `HOTPLEX_IDLE_TIMEOUT`      | `1h`    | Session inactivity timeout |

### Provider

| Variable                                        | Default         | Description                            |
| ----------------------------------------------- | --------------- | -------------------------------------- |
| `HOTPLEX_PROVIDER_TYPE`                         | `claude-code`   | Provider: claude-code, opencode        |
| `HOTPLEX_PROVIDER_MODEL`                        | `sonnet`        | Default model: sonnet, haiku, opus     |
| `HOTPLEX_PROVIDER_BINARY`                       | *(auto-detect)* | Path to CLI binary                     |
| `HOTPLEX_PROVIDER_DANGEROUSLY_SKIP_PERMISSIONS` | `false`         | Skip all permission checks             |
| `HOTPLEX_OPENCODE_COMPAT_ENABLED`               | `true`          | Enable OpenCode HTTP API compatibility |

### Projects (Docker)

| Variable               | Description                         |
| ---------------------- | ----------------------------------- |
| `HOTPLEX_PROJECTS_DIR` | Project workspace directory         |
| `HOTPLEX_GITCONFIG`    | Path to git config for bot identity |

### Native Brain (Optional)

| Variable                          | Default       | Description                             |
| --------------------------------- | ------------- | --------------------------------------- |
| `HOTPLEX_BRAIN_API_KEY`           | *(unset)*     | Brain API key (enables brain when set)  |
| `HOTPLEX_BRAIN_PROVIDER`          | `openai`      | Brain provider: openai, anthropic, etc. |
| `HOTPLEX_BRAIN_MODEL`             | `gpt-4o-mini` | Brain model                             |
| `HOTPLEX_BRAIN_ENDPOINT`          | *(optional)*  | Custom API endpoint                     |
| `HOTPLEX_BRAIN_TIMEOUT_S`         | `10`          | Request timeout in seconds              |
| `HOTPLEX_BRAIN_CACHE_SIZE`        | `1000`        | Cache size                              |
| `HOTPLEX_BRAIN_MAX_RETRIES`       | `3`           | Max retry attempts                      |
| `HOTPLEX_BRAIN_RETRY_MIN_WAIT_MS` | `100`         | Min retry wait                          |
| `HOTPLEX_BRAIN_RETRY_MAX_WAIT_MS` | `5000`        | Max retry wait                          |

### Message Store

| Variable                                   | Default                          | Description                       |
| ------------------------------------------ | -------------------------------- | --------------------------------- |
| `HOTPLEX_MESSAGE_STORE_ENABLED`            | `true`                           | Enable message persistence        |
| `HOTPLEX_MESSAGE_STORE_TYPE`               | `sqlite`                         | Storage: sqlite, postgres, memory |
| `HOTPLEX_MESSAGE_STORE_SQLITE_PATH`        | `~/.config/hotplex/chatapp_messages.db` | SQLite database path              |
| `HOTPLEX_MESSAGE_STORE_SQLITE_MAX_SIZE_MB` | `1024`                           | Max database size                 |
| `HOTPLEX_MESSAGE_STORE_STREAMING_ENABLED`  | `true`                           | Enable streaming storage          |
| `HOTPLEX_MESSAGE_STORE_STREAMING_TIMEOUT`  | `5m`                             | Streaming timeout                 |

### CORS

| Variable                  | Description                       |
| ------------------------- | --------------------------------- |
| `HOTPLEX_ALLOWED_ORIGINS` | Allowed origins (comma-separated) |

---

## Platform Credentials

### Slack

| Variable                       | Required    | Description                     |
| ------------------------------ | ----------- | ------------------------------- |
| `HOTPLEX_SLACK_PRIMARY_OWNER` | **Yes**     | Slack User ID of the primary owner |
| `HOTPLEX_SLACK_BOT_USER_ID`    | **Yes**     | Bot User ID (UXXXXXXXXXX)       |
| `HOTPLEX_SLACK_BOT_TOKEN`      | **Yes**     | Bot Token (xoxb-...)            |
| `HOTPLEX_SLACK_APP_TOKEN`      | Socket Mode | App Token (xapp-...)            |
| `HOTPLEX_SLACK_SIGNING_SECRET` | HTTP Mode   | Signing secret for verification |

### Feishu

| Variable                            | Description        |
| ----------------------------------- | ------------------ |
| `HOTPLEX_FEISHU_APP_ID`             | App ID             |
| `HOTPLEX_FEISHU_APP_SECRET`         | App secret         |
| `HOTPLEX_FEISHU_VERIFICATION_TOKEN` | Verification token |
| `HOTPLEX_FEISHU_ENCRYPT_KEY`        | Encryption key     |

---

## YAML Configuration

### Structure

```yaml
# configs/base/slack.yaml

# [Required] Inherit from base configuration (v0.33.0+)
inherits: null  # or path to parent config

# [Required] Platform identifier
platform: slack

# Provider settings
provider:
  type: claude-code
  enabled: true
  default_model: sonnet
  default_permission_mode: bypassPermissions

# Engine settings
engine:
  work_dir: ~/projects/myproject
  timeout: 30m
  idle_timeout: 1h

# Session lifecycle
session:
  timeout: 1h
  cleanup_interval: 5m

# Connection mode
mode: socket  # or "http"
server_addr: :8080

# AI behavior
system_prompt: |
  You are a helpful assistant...

# Feature toggles
features:
  chunking:
    enabled: true
    max_chars: 4000
  threading:
    enabled: true

# Security
security:
  verify_signature: true
  permission:
    dm_policy: allow
    group_policy: mention
    bot_user_id: ${HOTPLEX_SLACK_BOT_USER_ID}
```

### Environment Variable Expansion

**IMPORTANT**: Go's `os.ExpandEnv` only supports basic variable substitution:

```yaml
# Supported - simple variable expansion
bot_user_id: ${HOTPLEX_SLACK_BOT_USER_ID}
api_key: ${HOTPLEX_API_KEY}

# NOT supported (shell-style defaults)
bot_user_id: ${HOTPLEX_SLACK_BOT_USER_ID:-}  # Will NOT work!
api_key: ${HOTPLEX_API_KEY:-default}        # Will NOT work!
```

If an environment variable is not set, it will be replaced with an empty string. Ensure all required variables are defined in your `.env` file.

### Provider Section

| Field                          | Description                                                                        |
| ------------------------------ | ---------------------------------------------------------------------------------- |
| `type`                         | Provider type: `claude-code`, `opencode`                                           |
| `enabled`                      | Enable/disable provider                                                            |
| `default_model`                | Default model ID                                                                   |
| `default_permission_mode`      | Permission mode: `bypassPermissions`, `acceptEdits`, `default`, `dontAsk`, `plan` |
| `dangerously_skip_permissions` | Skip all permission checks (Docker/CI)                                             |
| `binary_path`                  | Custom binary path                                                                 |
| `allowed_tools`                | Tool whitelist                                                                     |
| `disallowed_tools`             | Tool blacklist                                                                     |

### Engine Section

| Field              | Description                 |
| ------------------ | --------------------------- |
| `work_dir`         | Agent's working directory   |
| `timeout`          | Max execution time          |
| `idle_timeout`     | Session idle timeout        |
| `allowed_tools`    | Engine-level tool whitelist |
| `disallowed_tools` | Engine-level tool blacklist |

### Features Section

| Feature                    | Description                         |
| -------------------------- | ----------------------------------- |
| `chunking.enabled`         | Split long messages                 |
| `chunking.max_chars`       | Max chars per chunk (Slack: 4000)   |
| `threading.enabled`        | Reply in threads                    |
| `rate_limit.enabled`       | Enable rate limit handling          |
| `rate_limit.max_attempts`  | Max retry attempts                  |
| `rate_limit.base_delay_ms` | Initial retry delay                 |
| `rate_limit.max_delay_ms`  | Max retry delay                     |
| `markdown.enabled`         | Convert Markdown to platform format |

### Security Section

| Field                                 | Description                                           |
| ------------------------------------- | ----------------------------------------------------- |
| `verify_signature`                    | Verify platform signatures (HTTP mode)                |
| `permission.dm_policy`                | DM policy: `allow`, `pairing`, `block`                |
| `permission.group_policy`             | Group policy: `allow`, `mention`, `multibot`, `block` |
| `permission.bot_user_id`              | Bot's User ID (required)                              |
| `permission.allowed_users`            | User whitelist                                        |
| `permission.blocked_users`            | User blacklist                                        |
| `permission.slash_command_rate_limit` | Rate limit per user                                   |

---

## Configuration Inheritance

HotPlex v0.33.0+ introduces a unified configuration system with inheritance support. This allows you to extend base configurations while customizing only what you need.

### The `inherits` Field

Use the `inherits` field to specify a parent configuration file:

```yaml
# configs/instances/my-bot/slack.yaml
inherits: ../../base/slack.yaml

# Only override what you need
ai:
  system_prompt: |
    Your custom system prompt here...
```

### Relative Path Resolution

The `inherits` field supports relative path resolution from the config file's location:

| Current File | Inherits Value | Resolved Path |
|--------------|----------------|---------------|
| `configs/admin/slack.yaml` | `./base/slack.yaml` | `configs/admin/base/slack.yaml` |
| `configs/admin/slack.yaml` | `../base/slack.yaml` | `configs/base/slack.yaml` |
| `configs/instances/bot-01/slack.yaml` | `../../base/slack.yaml` | `configs/base/slack.yaml` |

### Deep Merge

Configuration inheritance performs a **deep merge** for nested objects. Child values override parent values:

```yaml
# configs/base/slack.yaml (parent)
provider:
  type: claude-code
  default_model: sonnet
  enabled: true

security:
  permission:
    dm_policy: allow
    group_policy: mention

# configs/instances/my-bot/slack.yaml (child)
inherits: ../../base/slack.yaml

# Override only specific fields
provider:
  default_model: opus  # Override nested value

security:
  permission:
    group_policy: multibot  # Override nested value
    bot_user_id: U12345    # Add new field
```

Result after merge:
```yaml
provider:
  type: claude-code      # from parent
  default_model: opus    # overridden
  enabled: true          # from parent

security:
  permission:
    dm_policy: allow     # from parent
    group_policy: multibot  # overridden
    bot_user_id: U12345 # from child
```

### Circular Inheritance Detection

HotPlex automatically detects circular inheritance and provides informative errors:

```
Error: circular inheritance detected: a.yaml -> b.yaml -> c.yaml -> a.yaml
```

### Instance Isolation

For multi-bot deployments, each bot should have its own configuration directory:

```
configs/
├── base/                    # SSOT - do not modify for instances
│   ├── slack.yaml
│   └── server.yaml
├── admin/                   # Admin bot
│   ├── slack.yaml           # inherits: ../base/slack.yaml
│   └── server.yaml
└── instances/               # Bot instances
    ├── bot-01/
    │   ├── slack.yaml       # inherits: ../../base/slack.yaml
    │   └── server.yaml      # inherits: ../../base/server.yaml
    └── bot-02/
        ├── slack.yaml       # inherits: ../../base/slack.yaml
        └── server.yaml
```

---

## Role Templates

HotPlex provides role-based system prompt templates in `configs/templates/roles/`. These templates define AI behavior for different development specializations.

### Available Roles

| Role | Description |
|------|-------------|
| `go` | Go Backend Engineer - REST APIs, database, concurrency |
| `frontend` | React/Next.js Frontend Engineer - UI components, state |
| `devops` | Docker/K8s DevOps Engineer - containers, CI/CD |
| `custom` | User-defined template for custom roles |

### Using Role Templates

#### Method 1: Copy to Bot Config

```bash
# Copy role template to your bot config
cp configs/templates/roles/go.yaml configs/instances/my-bot/system-prompt.yaml

# Reference in your slack.yaml
inherits: ../../base/slack.yaml

ai:
  system_prompt_file: ./system-prompt.yaml
```

#### Method 2: Inline Reference

```yaml
# configs/instances/my-bot/slack.yaml
inherits: ../../base/slack.yaml

# Use role template directly
system_prompt: |
  {{ include "configs/templates/roles/go.yaml" | toJson }}

# Or copy the content and customize
system_prompt: |
  You are HotPlex, a senior Go backend engineer in a Slack conversation.

  ## Environment
  - Running under HotPlex engine (stdin/stdout)
  - Headless mode - cannot prompt for user input
  ...
```

### Customizing Role Templates

For custom roles:

1. Copy `configs/templates/roles/custom.yaml` to a new file
2. Edit the `system_prompt` content
3. Reference it in your bot configuration

---

## Admin Bot Configuration

The admin bot in `configs/admin/` provides operational capabilities for the HotPlex multi-bot system.

### Configuration Structure

```yaml
# configs/admin/slack.yaml
inherits: ../base/slack.yaml

system_prompt: |
  You are HotPlex Admin Bot, a DevOps specialist...
```

### Admin Bot Capabilities

The admin bot has specialized skills for operational tasks:

| Skill | Trigger Phrases | Purpose |
|-------|-----------------|---------|
| `docker-container-ops` | restart bot, start container, check status | Container lifecycle management |
| `hotplex-diagnostics` | diagnose, check health, view logs, debug | Health monitoring & debugging |
| `hotplex-data-mgmt` | clean sessions, markers, export data | Session & data management |
| `hotplex-release` | release, create tag, bump version | Version releases |

### Key Constraints

The admin bot is designed for **operations only**:
- **NEVER modifies code directly** - no editing source files
- **ALL problems become Issues** - creates issues for code problems
- **OBSERVE, ANALYZE, REPORT** - other bots implement fixes

### Deployment

```bash
# Sync admin config to home directory
make sync

# Or manually
cp -r configs/admin/* ~/.hotplex/configs/
cp -r configs/base ~/.hotplex/configs/base/
```

---

## Platform-Specific Configs

### Slack (socket mode)

```yaml
platform: slack
mode: socket  # Recommended for development

provider:
  type: claude-code

security:
  permission:
    bot_user_id: ${HOTPLEX_SLACK_BOT_USER_ID}
    group_policy: multibot

features:
  chunking:
    enabled: true
    max_chars: 4000
  threading:
    enabled: true
```

### Slack (HTTP/webhook mode)

```yaml
platform: slack
mode: http
server_addr: :8080

security:
  verify_signature: true
```

---

## Examples

### Configuration Inheritance Chain

#### Base Configuration (configs/base/slack.yaml)

```yaml
# SSOT base configuration - DO NOT modify directly for instances
platform: slack
mode: socket

provider:
  type: claude-code
  default_model: sonnet

engine:
  work_dir: ~/projects
  timeout: 30m
  idle_timeout: 1h

security:
  permission:
    dm_policy: allow
    group_policy: mention
```

#### Instance Configuration (configs/instances/bot-01/slack.yaml)

```yaml
# Inherit from base, override specific values
inherits: ../../base/slack.yaml

# Custom settings for this bot
security:
  permission:
    bot_user_id: ${HOTPLEX_SLACK_BOT_USER_ID}
    group_policy: multibot

# Custom system prompt
system_prompt: |
  You are Bot-01, a senior Go backend engineer...

features:
  chunking:
    enabled: true
    max_chars: 4000
  threading:
    enabled: true
```

#### Admin Configuration (configs/admin/slack.yaml)

```yaml
# Admin bot inherits from base
inherits: ../base/slack.yaml

# Admin-specific overrides
system_prompt: |
  You are HotPlex Admin Bot, a DevOps specialist...

security:
  permission:
    bot_user_id: ${HOTPLEX_SLACK_BOT_USER_ID}
```

### Minimal Slack Config

```yaml
platform: slack
mode: socket

provider:
  type: claude-code

engine:
  work_dir: ~/projects/myproject

security:
  permission:
    bot_user_id: ${HOTPLEX_SLACK_BOT_USER_ID}
```

### Multi-Bot Setup

```yaml
platform: slack
mode: socket

security:
  permission:
    bot_user_id: ${HOTPLEX_SLACK_BOT_USER_ID}
    group_policy: multibot  # Key setting for multi-bot
    broadcast_response: |
      Please @mention me if you'd like help.
```

### Docker Production Config

```yaml
platform: slack

provider:
  type: claude-code
  dangerously_skip_permissions: true  # For containerized environments

engine:
  work_dir: /app/workspace
  timeout: 30m
  idle_timeout: 2h

features:
  chunking:
    enabled: true
  rate_limit:
    enabled: true
    max_attempts: 5
```

---

## Related Documentation

- [development.md](development.md) - Development guide
- [ARCHITECTURE.md](ARCHITECTURE.md) - Architecture overview
- [docker-deployment.md](docker-deployment.md) - Docker deployment
- [chatapps/slack-setup-beginner.md](chatapps/slack-setup-beginner.md) - Slack setup guide
- [configs/README.md](../configs/README.md) - Configuration directory overview
- [configs/base/README.md](../configs/base/README.md) - Base configuration details
