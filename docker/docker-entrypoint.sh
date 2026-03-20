#!/usr/bin/env bash
set -e

# ==============================================================================
# HotPlex Docker Entrypoint
# Handles permission fixes, config env expansion, Git identity, PIP tools, and
# privilege drop. Inspired by OpenClaw DevKit patterns.
# ==============================================================================

HOTPLEX_HOME="/home/hotplex"
CONFIG_DIR="${HOTPLEX_HOME}/.hotplex"

# ------------------------------------------------------------------------------
# Helper: Run commands as the hotplex user if currently root
# Uses env to explicitly set HOME (runuser --setenv not available on Debian 12)
# ------------------------------------------------------------------------------
run_as_hotplex() {
    if [[ "$(id -u)" = "0" ]]; then
        runuser -u hotplex -- env HOME="${HOTPLEX_HOME}" "$@"
    else
        "$@"
    fi
}

# ------------------------------------------------------------------------------
# Helper: Validate package name (alphanumeric, hyphens, underscores only)
# Prevents command injection via PIP_TOOLS
# ------------------------------------------------------------------------------
validate_pkg_name() {
    local name="$1"
    # Allow: letters, numbers, hyphens, underscores, dots (for version specs)
    if [[ ! "$name" =~ ^[a-zA-Z0-9._-]+$ ]]; then
        echo "ERROR: Invalid package name: $name" >&2
        return 1
    fi
    return 0
}

# ------------------------------------------------------------------------------
# 0. Cleanup stale temporary files from previous runs
# ------------------------------------------------------------------------------
find "${CONFIG_DIR}" -name "*.tmp" -type f -delete 2>/dev/null || true
find "${HOTPLEX_HOME}/configs/chatapps" -name "*.tmp" -type f -delete 2>/dev/null || true

# ------------------------------------------------------------------------------
# 1. Fix Permissions & Create Directories (if running as root)
#    Solves EACCES issues with host-mounted volumes and ensures paths exist
# ------------------------------------------------------------------------------
if [[ "$(id -u)" = "0" ]]; then
    echo "--> Optimizing file access policy for ${CONFIG_DIR}..."
    mkdir -p "${CONFIG_DIR}" "${HOTPLEX_HOME}/.claude" "${HOTPLEX_HOME}/projects"

    chown -R hotplex:hotplex "${CONFIG_DIR}" 2>/dev/null || true
    chown -R hotplex:hotplex "${HOTPLEX_HOME}/.claude" 2>/dev/null || true
    chown -R hotplex:hotplex "${HOTPLEX_HOME}/projects" 2>/dev/null || true

    # Fix backup files created by CLI (may be owned by root if CLI runs during entrypoint)
    # These are .claude.json.backup.* files in home directory
    find "${HOTPLEX_HOME}" -maxdepth 1 -name ".claude.json.backup.*" -type f -exec chown hotplex:hotplex {} \; 2>/dev/null || true

    # Fix Go module cache permissions (Docker volume may be owned by root)
    if [[ -d "${HOTPLEX_HOME}/go" ]]; then
        echo "--> Fixing Go module cache permissions..."
        chown -R hotplex:hotplex "${HOTPLEX_HOME}/go" 2>/dev/null || true
    fi

    # Fix pip packages directory permissions (for PIP_TOOLS persistence)
    if [[ -d "${HOTPLEX_HOME}/.local" ]]; then
        echo "--> Fixing pip packages permissions..."
        chown -R hotplex:hotplex "${HOTPLEX_HOME}/.local" 2>/dev/null || true
    fi

    # Fix Go build cache
    if [[ -d "${HOTPLEX_HOME}/.cache/go-build" ]]; then
        echo "--> Fixing Go build cache permissions..."
        chown -R hotplex:hotplex "${HOTPLEX_HOME}/.cache/go-build" 2>/dev/null || true
    fi
fi

# ------------------------------------------------------------------------------
# 2. Bot Identity & Logging
# ------------------------------------------------------------------------------
echo "==> HotPlex Bot Instance: ${HOTPLEX_BOT_ID:-unknown}"

# ------------------------------------------------------------------------------
# 3. Expand Environment Variables in YAML Config Files
#    Required for ${HOTPLEX_*} variables in slack.yaml, feishu.yaml, etc.
# ------------------------------------------------------------------------------
CONFIG_CHATAPPS_DIR="${HOTPLEX_HOME}/configs/chatapps"
if [[ -d "${CONFIG_CHATAPPS_DIR}" ]]; then
    echo "--> Expanding environment variables in config files..."

    # Generate variable list for envsubst (only HOTPLEX, GIT, GITHUB variables)
    # This prevents envsubst from clearing out non-environment placeholders like ${issue_id}
    VARS=$(compgen -A export | grep -E "^(HOTPLEX_|GIT_|GITHUB_|HOST_)" | sed 's/^/$/' | tr '\n' ' ')

    for yaml in "${CONFIG_CHATAPPS_DIR}"/*.yaml; do
        if [[ -f "${yaml}" ]]; then
            # Create a temporary file to avoid partial write issues
            tmp_yaml="${yaml}.tmp"
            if [[ -n "${VARS}" ]]; then
                envsubst "${VARS}" < "${yaml}" > "${tmp_yaml}"
                mv "${tmp_yaml}" "${yaml}"
                echo "    - Processed $(basename "${yaml}")"
            else
                echo "    - Skipping $(basename "${yaml}") (No relevant variables exported)"
            fi
        fi
    done
fi

# ------------------------------------------------------------------------------
# 4. Claude Code Runtime Files (named volume persists across restarts)
# ------------------------------------------------------------------------------
CLAUDE_DIR="${HOTPLEX_HOME}/.claude"
CLAUDE_JSON="${HOTPLEX_HOME}/.claude.json"

# Ensure container-private .claude directory exists (named volume auto-creates)
run_as_hotplex mkdir -p "${CLAUDE_DIR}"

# Ensure .claude.json exists (Claude Code CLI requires this file)
# Create empty JSON object if missing to prevent "configuration file not found" warnings
# This file is a runtime state file containing userID, project configs, MCP servers, etc.
# Reference: https://code.claude.com/docs/en/settings
if [[ ! -f "${CLAUDE_JSON}" ]]; then
    echo "--> Creating empty .claude.json configuration file..."
    run_as_hotplex sh -c "echo '{}' > '${CLAUDE_JSON}'"
    # Ensure correct permissions
    if [[ "$(id -u)" = "0" ]]; then
        chown hotplex:hotplex "${CLAUDE_JSON}" 2>/dev/null || true
    fi
fi

# ------------------------------------------------------------------------------
# 6. Git Identity Injection (from environment variables)
#    Allows configuring Git identity via .env without host .gitconfig dependency
# ------------------------------------------------------------------------------
if [[ -n "${GIT_USER_NAME:-}" ]]; then
    echo "--> Setting Git identity: ${GIT_USER_NAME}"
    run_as_hotplex git config --global user.name "${GIT_USER_NAME}" || echo "    Warning: Failed to set git user.name"
fi
if [[ -n "${GIT_USER_EMAIL:-}" ]]; then
    run_as_hotplex git config --global user.email "${GIT_USER_EMAIL}" || echo "    Warning: Failed to set git user.email"
fi

# Auto-configure safe.directory for mounted project volumes
if [[ -d "${HOTPLEX_HOME}/projects" ]]; then
    run_as_hotplex git config --global --add safe.directory "${HOTPLEX_HOME}/projects" || true
    # Also add all first-level subdirectories (cloned repos)
    for d in "${HOTPLEX_HOME}/projects"/*/; do
        [[ -d "${d}.git" ]] && run_as_hotplex git config --global --add safe.directory "${d}" || true
    done
fi

# ------------------------------------------------------------------------------
# 7. Auto-install pip tools (reinstalled on rebuild via entrypoint)
# Set PIP_TOOLS env var to install additional packages, e.g., PIP_TOOLS="notebooklm pandas"
# Inspired by OpenClaw DevKit patterns.
# ------------------------------------------------------------------------------
if [[ -n "${PIP_TOOLS:-}" ]]; then
    echo "--> Checking pip tools: ${PIP_TOOLS}"

    for tool in ${PIP_TOOLS}; do
        # Extract package name (before :) and binary name (after :) if specified
        # Example: "notebooklm-py:notebooklm" installs pkg "notebooklm-py", checks binary "notebooklm"
        pkg_name="${tool%%:*}"
        bin_name="${tool#*:}"

        # Security: Validate package name to prevent command injection
        if ! validate_pkg_name "${pkg_name}"; then
            echo "--> ERROR: Skipping invalid package name: ${pkg_name}"
            continue
        fi

        # Check if binary exists
        if ! command -v "${bin_name}" >/dev/null 2>&1; then
            echo "--> Installing ${pkg_name} (binary: ${bin_name})..."
            # Use uv for fast installation (available in DevKit images)
            if command -v uv >/dev/null 2>&1; then
                if run_as_hotplex uv pip install --system --break-system-packages --no-cache "${pkg_name}" 2>&1; then
                    echo "--> Successfully installed ${pkg_name}"
                else
                    echo "--> Warning: Failed to install ${pkg_name} via uv"
                fi
            # Fallback to pip if uv is not available
            elif command -v pip3 >/dev/null 2>&1; then
                if run_as_hotplex pip3 install --break-system-packages --no-cache-dir "${pkg_name}" 2>&1; then
                    echo "--> Successfully installed ${pkg_name}"
                else
                    echo "--> Warning: Failed to install ${pkg_name} via pip3"
                fi
            else
                echo "--> Warning: neither uv nor pip3 available, skipping ${pkg_name}"
            fi
        else
            echo "--> ${bin_name} already installed, skipping."
        fi
    done
fi

# ------------------------------------------------------------------------------
# 8. Execute CMD (drop privileges if root)
#    Ensures all files created by the app belong to 'hotplex' user
# ------------------------------------------------------------------------------
echo "==> Starting HotPlex Engine..."
if [[ "$(id -u)" = "0" ]]; then
    # Ensure HOME is correctly set for hotplex before execution
    export HOME="${HOTPLEX_HOME}"
    exec runuser -u hotplex -m -- "$@"
else
    exec "$@"
fi
