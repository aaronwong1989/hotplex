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

    # Fix permissions for project subdirectories (e.g., .agent, .claude created by CLI tools)
    # These directories may be owned by root if created during container runtime
    if [[ -d "${HOTPLEX_HOME}/projects" ]]; then
        find "${HOTPLEX_HOME}/projects" -type d \( -name ".agent" -o -name ".claude" \) -exec chown -R hotplex:hotplex {} \; 2>/dev/null || true
    fi

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
# 1.5. Initialize plugins directory
#      The seed plugins/ is mounted read-only as .seed,
#      writable layer is at plugins/ (named volume hotplex-matrix-plugins)
#      Merge seed plugins into writable layer
# ------------------------------------------------------------------------------
CLAUDE_DIR="${HOTPLEX_HOME}/.claude"
PLUGINS_SEED="${CLAUDE_DIR}/plugins.seed"
PLUGINS_WRITABLE="${CLAUDE_DIR}/plugins"

# Ensure writable plugins directory exists (from named volume)
run_as_hotplex mkdir -p "${PLUGINS_WRITABLE}"

# Copy seed plugins to writable layer if writable layer is empty
if [[ -d "${PLUGINS_SEED}" ]] && [[ -z "$(ls -A "${PLUGINS_WRITABLE}" 2>/dev/null)" ]]; then
    echo "--> Copying seed plugins to writable layer..."
    cp -r "${PLUGINS_SEED}"/* "${PLUGINS_WRITABLE}/" 2>/dev/null || true
fi
STATUSLINE_SEED="${CLAUDE_DIR}/statusline.sh.seed"
STATUSLINE_TARGET="${CLAUDE_DIR}/statusline.sh"

if [[ -f "${STATUSLINE_SEED}" ]]; then
    echo "--> Generating statusline.sh from seed..."
    cp "${STATUSLINE_SEED}" "${STATUSLINE_TARGET}"

    if [[ "$(id -u)" = "0" ]]; then
        chown hotplex:hotplex "${STATUSLINE_TARGET}" 2>/dev/null || true
        chmod +x "${STATUSLINE_TARGET}" 2>/dev/null || true
    fi
    echo "    - statusline.sh generated successfully"
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
CREDENTIALS_JSON="${CLAUDE_DIR}/credentials.json"

# Ensure container-private .claude directory exists (named volume auto-creates)
run_as_hotplex mkdir -p "${CLAUDE_DIR}"

# ------------------------------------------------------------------------------
# 4.5. Initialize Claude Configuration from Seed
# ------------------------------------------------------------------------------
initialize_claude_config() {
    local claude_json="${HOTPLEX_HOME}/.claude.json"
    local seed_claude_json="${HOTPLEX_HOME}/.claude/.claude.json"

    # Skip if seed file not available
    [[ ! -f "${seed_claude_json}" ]] && return 0

    # Skip if container already has config (preserve user runtime state)
    [[ -f "${claude_json}" ]] && return 0

    # Copy seed config to container home
    if cp "${seed_claude_json}" "${claude_json}" 2>/dev/null; then
        [[ "$(id -u)" = "0" ]] && chown hotplex:hotplex "${claude_json}" 2>/dev/null || true
    fi
}

# Run initialization
initialize_claude_config

# ------------------------------------------------------------------------------
# 5. Cleanup Orphaned Claude userID (prevent OAuth prompts in containers)
#    Matches internal/engine/maintenance.go:clearClaudeJSONUserID logic
# ------------------------------------------------------------------------------
# Background: Claude Code 2.1.81+ uses userID to trigger OAuth authentication.
# In containerized environments, OAuth cannot complete (no browser), causing
# "Not logged in · Please run /login" errors. This cleanup removes orphaned
# userID entries and marks onboarding as completed to prevent prompts.
#
# Controlled by HOTPLEX_CLAUDE_CLEAR_USERID=true (default) or false to disable.
# This mirrors the Go maintenance task that runs every 10 minutes.
# ------------------------------------------------------------------------------
if [[ "${HOTPLEX_CLAUDE_CLEAR_USERID:-true}" != "false" ]]; then
    if [[ -f "${CLAUDE_JSON}" ]]; then
        # Check if userID exists in config
        if grep -q '"userID"' "${CLAUDE_JSON}" 2>/dev/null; then
            # Check if credentials.json exists (valid OAuth setup)
            if [[ ! -f "${CREDENTIALS_JSON}" ]]; then
                echo "--> Cleaning up orphaned Claude userID (no OAuth credentials found)..."

                # Use jq to process JSON if available
                if command -v jq >/dev/null 2>&1; then
                    # Remove userID and add hasCompletedOnboarding
                    tmp_file="${CLAUDE_JSON}.tmp"
                    if jq 'del(.userID) | .hasCompletedOnboarding = true' "${CLAUDE_JSON}" > "${tmp_file}" 2>/dev/null; then
                        # Preserve permissions
                        if [[ "$(id -u)" = "0" ]]; then
                            chown hotplex:hotplex "${tmp_file}" 2>/dev/null || true
                        fi
                        mv "${tmp_file}" "${CLAUDE_JSON}"
                        echo "--> Cleared orphaned userID and set hasCompletedOnboarding=true"
                    else
                        echo "--> Warning: Failed to process ${CLAUDE_JSON}, skipping cleanup"
                        rm -f "${tmp_file}" 2>/dev/null || true
                    fi
                else
                    echo "--> Warning: jq not available, skipping userID cleanup (install jq for automatic cleanup)"
                fi
            else
                echo "--> Valid OAuth setup detected (credentials.json exists), preserving userID"
            fi
        fi
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
# 7.5. Inject Environment Variables to .bashrc
#      Exports HOTPLEX, GITHUB, GIT_USER, ANTHROPIC variables to .bashrc
#      for interactive shell sessions and debugging
# ------------------------------------------------------------------------------
inject_env_to_bashrc() {
    local bashrc="${HOTPLEX_HOME}/.bashrc"
    local start_marker="# === HotPlex Environment Variables (Auto-generated) ==="
    local end_marker="# === End HotPlex Environment Variables ==="

    # Remove old auto-generated section if exists
    if grep -qF "${start_marker}" "${bashrc}" 2>/dev/null; then
        # Use sed to delete the block between markers (inclusive)
        # Need to escape special chars for sed pattern
        local start_escaped
        start_escaped=$(printf '%s\n' "${start_marker}" | sed 's/[[\.*^$()+?{|\\]/\\&/g')
        local end_escaped
        end_escaped=$(printf '%s\n' "${end_marker}" | sed 's/[[\.*^$()+?{|\\]/\\&/g')

        sed -i "/^${start_escaped}$/,/^${end_escaped}$/d" "${bashrc}" 2>/dev/null || true
    fi

    # Append new environment variables
    {
        echo ""
        echo "${start_marker}"
        echo "# This section is auto-generated by docker-entrypoint.sh"
        echo "# Do not edit manually - changes will be overwritten on container restart"
        echo ""

        # Export relevant environment variables
        # Using printenv to get all environment variables and filter
        while IFS='=' read -r key value; do
            # Skip if key is empty
            [[ -z "${key}" ]] && continue

            # Escape single quotes in value for bash export
            # This handles values with spaces, special chars, etc.
            local escaped_value
            escaped_value=$(printf '%s\n' "${value}" | sed "s/'/'\\\\''/g")
            echo "export ${key}='${escaped_value}'"
        done < <(printenv | grep -E "^(HOTPLEX|GITHUB|GIT_USER|ANTHROPIC)_" | sort)

        echo ""
        echo "${end_marker}"
    } >> "${bashrc}"

    # Set correct ownership
    if [[ "$(id -u)" = "0" ]]; then
        chown hotplex:hotplex "${bashrc}" 2>/dev/null || true
    fi

    echo "--> Injected environment variables to .bashrc"
}

# Only inject when running as root (avoid duplicate injections)
if [[ "$(id -u)" = "0" ]]; then
    inject_env_to_bashrc
fi

# ------------------------------------------------------------------------------
# 8. Execute CMD (drop privileges if root)
#    Ensures all files created by the app belong to 'hotplex' user
# ------------------------------------------------------------------------------
echo "==> Starting HotPlex Engine..."
if [[ "$(id -u)" = "0" ]]; then
    # Ensure HOME is correctly set for hotplex before execution
    # Note: -m is NOT used because it preserves PID 1's HOME=/root from the base image,
    # which would make ClaudeCodeExtractor read ~/.claude/settings.json from /root instead
    # of /home/hotplex/.claude/ where the named volume is mounted.
    export HOME="${HOTPLEX_HOME}"
    exec runuser -u hotplex -- "$@"
else
    exec "$@"
fi
