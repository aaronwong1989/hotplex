# Docker Matrix Instance Configurations

This directory contains bot-specific configuration files for Docker Matrix deployment.

## Directory Structure

```
configs/
├── bot-01/
│   ├── base/               # Base templates (synced from ./configs/base/)
│   │   ├── server.yaml
│   │   ├── slack.yaml
│   │   ├── slack_capabilities.yaml
│   │   └── feishu.yaml
│   ├── slack.yaml          # Bot-specific Slack config
│   └── server.yaml         # Bot-specific server config
├── bot-02/
│   ├── base/
│   ├── slack.yaml
│   └── server.yaml
└── bot-03/
    ├── base/
    ├── slack.yaml
    └── server.yaml
```

## Configuration Sync

Base templates in `bot-*/base/` are synced from `configs/base/` directory. Use these commands:

```bash
# Sync to admin bot (~/.hotplex/)
make sync

# Sync to all Docker instances
make docker-sync
```

### Sync Behavior

- `make sync`: Copies `configs/base/*` to `~/.hotplex/seed/` and `~/.hotplex/configs/base/`
- `make docker-sync`: Copies `configs/base/*` to each bot instance's `base/` directory

## Warning: Do Not Edit base/ Directory

**The `bot-*/base/` directories are auto-generated and will be overwritten on every sync.**

- Do NOT edit files in `bot-*/base/` directly
- Edit source templates in `configs/base/` instead
- Run `make docker-sync` to propagate changes

## Configuration Format

Bot-specific configs (`slack.yaml`, `server.yaml`) are **complete configurations**, not partial overrides.

### Full Configuration Example (slack.yaml)

```yaml
# =============================================================================
# HotPlex Slack Adapter Configuration
# =============================================================================

# 1. PLATFORM & CONNECTION
platform: slack
mode: socket                       # "socket" (Recommended) or "http"
server_addr: :8080                 # Used for health checks or HTTP mode

# 2. AI PROVIDER & ENGINE
provider:
  type: claude-code                # claude-code | opencode
  default_model: sonnet
  default_permission_mode: bypassPermissions
  dangerously_skip_permissions: true

engine:
  work_dir: ${HOTPLEX_PROJECTS_DIR}/hotplex
  timeout: 30m
  idle_timeout: 1h

# 3. SECURITY & ACCESS
security:
  verify_signature: true
  owner:
    primary: ${HOTPLEX_SLACK_PRIMARY_OWNER}
    policy: trusted                # owner_only | trusted | public
    trusted_users: []

  permission:
    dm_policy: allow
    group_policy: multibot         # allow | mention | multibot | block
    bot_user_id: ${HOTPLEX_SLACK_BOT_USER_ID}
    thread_ownership:
      enabled: true

# 4. STORAGE
message_store:
  enabled: true
  type: sqlite
  sqlite:
    path: ${HOTPLEX_MESSAGE_STORE_SQLITE_PATH}

# 5. AI IDENTITY & BEHAVIOR
system_prompt: |
  You are HotPlex, an expert software engineer in a Slack conversation.

  ## Environment
  - Running under HotPlex engine (stdin/stdout)
  - Headless mode - cannot prompt for user input

  ## Slack Context
  - Replies go to thread automatically
  - Keep answers concise - user expects quick responses

# 6. APP HOME CAPABILITY CENTER
apphome:
  enabled: true
  capabilities_path: ""
```

### Full Configuration Example (server.yaml)

```yaml
# HotPlex Server Configuration

# Engine Configuration
engine:
  timeout: 30m
  idle_timeout: 1h
  work_dir: /tmp/hotplex_sandbox
  system_prompt: "You are an expert AI software engineer running on HotPlex."

# Server Configuration
server:
  port: 8080
  log_level: info
  log_format: text

# Security Configuration
security:
  api_key: "${HOTPLEX_API_KEY}"
  permission_mode: bypassPermissions
```

## Creating New Bots

Use the interactive script:

```bash
make add-bot
# or
./docker/matrix/add-bot.sh
```

## Sensitive Information

Sensitive credentials (tokens, API keys) are stored in `.env` files, not in YAML configs.
The `.env` files are located at `~/.hotplex/instances/$BOT_ID/.env` and are never committed.

## Do Not Sync to Upstream

This directory contains your bot configurations. **Do not sync to upstream repository.**

The `.gitignore` pattern `docker/matrix/configs/bot-*/` excludes these files.
