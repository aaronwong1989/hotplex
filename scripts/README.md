# HotPlex Scripts

Standardized utility scripts and Git hooks for development, operations, and verification.

## Directory Structure

| Directory | Focus | Key Scripts |
|-----------|-------|-------------|
| [`git-hooks/`](./git-hooks) | Developer experience & quality | `setup_hooks.sh`, `pre-commit`, `commit-msg` |
| [`ops/`](./ops) | Deployment & service management | `service.sh`, `restart_helper.sh`, `setup_gitconfig.sh` |
| [`verify/`](./verify) | Automated validation & diagnostics | `verify_claude_stream_tokens.py`, `check_links.py` |
| [`tools/`](./tools) | Asset generation & GitHub management | `generate_assets.sh`, `manage-labels.py` |

---

## 🔗 Git Hooks (`git-hooks/`)

Ensure code quality and consistent commit messages.

- `setup_hooks.sh`: Links hooks from `scripts/git-hooks/` to `.git/hooks/`.
- `pre-commit`: Runs `go fmt` and dependency checks.
- `commit-msg`: Validates [Conventional Commits](https://www.conventionalcommits.org/) format.
- `pre-push`: Runs full test suite before pushing.

**Setup:** `make install-hooks` (or `bash scripts/git-hooks/setup_hooks.sh`)

---

## 🚀 Operations (`ops/`)

Deployment and runtime management.

- `service.sh`: Install/manage `hotplexd` as a system service (launchd/systemd).
- `restart_helper.sh`: Gracefully restart the daemon (used by `Makefile`).
- `setup_gitconfig.sh`: Generate isolated git identity for bot containers.

---

## 🧪 Verification (`verify/`)

Validation of tokens, links, and engine behavior.

- `verify_claude_stream_tokens.py`: Verify token usage data in Claude Code stream-json.
- `verify_slack_tokens.sh`: Check validity of Slack Bot/App tokens.
- `check_links.py`: Audit internal documentation for dead links.
- `analyze_model_usage_fields.py`: Analyze model usage data fields.

---

## 🛠️ Tools & Assets (`tools/`)

Asset generation and repository management.

- `generate_assets.sh`: Generate `favicon.ico`, OG images, and PNGs from SVGs.
- `svg2png.sh`: Convert SVG to high-resolution PNGs.
- `manage-labels.py`: Sync GitHub issue/PR labels with project standards.

---

## Usage Examples

```bash
# Install hooks
make install-hooks

# Manage service
make service-status
make service-restart

# Verify tokens
bash scripts/verify/verify_slack_tokens.sh

# Sync GitHub labels
python3 scripts/tools/manage-labels.py --apply
```
