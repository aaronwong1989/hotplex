# HotPlex Scripts

Utility scripts and Git hooks for development, deployment, and asset generation.

## Git Hooks

Ensure code quality and consistent commit messages.

| Script | Description |
|--------|-------------|
| `setup_hooks.sh` | Links hooks from `scripts/` to `.git/hooks/` |
| `pre-commit` | Runs `go fmt` and dependency checks before commit |
| `commit-msg` | Validates Conventional Commits format |
| `pre-push` | Final checks (e.g., full test suite) before push |

Setup: `bash scripts/setup_hooks.sh`

## Documentation

| Script | Description |
|--------|-------------|
| `check_links.py` | Audits internal docs links to prevent dead links |

## Asset Generation

| Script | Description |
|--------|-------------|
| `generate_assets.sh` | Generates `favicon.ico`, OG images, and PNGs from SVG sources |
| `svg2png.sh` | Converts SVG files to high-resolution PNGs with customizable zoom/colors |

## GitHub Management

| Script | Description |
|--------|-------------|
| `manage-labels.py` | Manages GitHub issue/PR labels — 7 categories, 34 labels total |

```bash
python3 scripts/manage-labels.py --dry-run  # Preview changes
python3 scripts/manage-labels.py --apply    # Apply changes
```

See [docs/github-labels.md](../docs/github-labels.md) for the full label system guide.

## Verification & Diagnostics

| Script | Description |
|--------|-------------|
| `verify_claude_stream_tokens.py` | Verify Claude Code CLI returns token data in stream-json output format |
| `verify_slack_tokens.sh` | Verify Slack Bot Token (xoxb-) and App Token (xapp-) validity |

### verify_claude_stream_tokens.py

Validates that Claude Code CLI provides token usage data (including cache tokens) in stream-json mode.

**Usage:**
```bash
python3 scripts/verify_claude_stream_tokens.py
```

**Requirements:**
- Claude Code CLI installed (`~/.local/bin/claude`)
- `ANTHROPIC_AUTH_TOKEN` environment variable set

**Output:**
- Event summary (all event types in the stream)
- Token data extraction (input/output/cache tokens)
- Verification status (passed/failed)

**Token Fields Verified:**
- `input_tokens`: Total input tokens consumed
- `output_tokens`: Total output tokens generated
- `cache_creation_input_tokens`: Tokens written to cache
- `cache_read_input_tokens`: Tokens read from cache (cache hits)

**Related Issues:** #350, #351

## Deployment & DevOps

| Script | Description |
|--------|-------------|
| `service.sh` | Install/manage hotplexd as a system service (launchd/systemd) |
| `setup_gitconfig.sh` | Generate isolated git identity for each bot container |
| `verify_slack_tokens.sh` | Verify Slack Bot Token (xoxb-) and App Token (xapp-) validity |
| `restart_helper.sh` | Gracefully restart the hotplexd daemon (used by Makefile) |

```bash
# Service management
bash scripts/service.sh install    # Install as system service
bash scripts/service.sh start      # Start service
bash scripts/service.sh status     # Check status

# Verify Slack tokens
bash scripts/verify_slack_tokens.sh

# Setup bot git identities
bash scripts/setup_gitconfig.sh
```
