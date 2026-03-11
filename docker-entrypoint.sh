#!/usr/bin/env bash
set -e

# ==============================================================================
# HotPlex Docker Entrypoint
# Handles permission fixes, config seeding, Git identity, and privilege drop
# ==============================================================================

HOTPLEX_HOME="/home/hotplex"
CONFIG_DIR="$HOTPLEX_HOME/.hotplex"

# ------------------------------------------------------------------------------
# Helper: Run commands as the hotplex user if currently root
# ------------------------------------------------------------------------------
run_as_hotplex() {
    if [ "$(id -u)" = "0" ]; then
        runuser -u hotplex -m -- "$@"
    else
        "$@"
    fi
}

# ------------------------------------------------------------------------------
# 1. Fix Permissions (if running as root)
#    Solves EACCES issues with host-mounted volumes
# ------------------------------------------------------------------------------
if [ "$(id -u)" = "0" ]; then
    echo "--> Fixing permissions for mounted volumes..."
    chown -R hotplex:hotplex "$CONFIG_DIR" 2>/dev/null || true
    chown -R hotplex:hotplex "$HOTPLEX_HOME/.claude" 2>/dev/null || true
    chown -R hotplex:hotplex "$HOTPLEX_HOME/projects" 2>/dev/null || true
fi

# ------------------------------------------------------------------------------
# 2. Claude Code Configuration - create if not exists
# ------------------------------------------------------------------------------
CLAUDE_CONFIG="$HOTPLEX_HOME/.claude.json"
if [ ! -f "$CLAUDE_CONFIG" ]; then
    run_as_hotplex sh -c "echo '{}' > '$CLAUDE_CONFIG'"
    echo "[entrypoint] Created $CLAUDE_CONFIG"
fi

# ------------------------------------------------------------------------------
# 3. Git Identity Injection (from environment variables)
#    Allows configuring Git identity via .env without host .gitconfig dependency
# ------------------------------------------------------------------------------
if [ -n "${GIT_USER_NAME:-}" ]; then
    echo "--> Setting Git identity: $GIT_USER_NAME"
    run_as_hotplex git config --global user.name "$GIT_USER_NAME"
fi
if [ -n "${GIT_USER_EMAIL:-}" ]; then
    run_as_hotplex git config --global user.email "$GIT_USER_EMAIL"
fi

# ------------------------------------------------------------------------------
# 4. Execute CMD (drop privileges if root)
#    Ensures all files created by the app belong to 'hotplex' user
# ------------------------------------------------------------------------------
echo "==> Starting HotPlex..."
if [ "$(id -u)" = "0" ]; then
    exec runuser -u hotplex -- "$@"
else
    exec "$@"
fi
