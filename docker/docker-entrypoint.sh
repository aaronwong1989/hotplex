#!/usr/bin/env bash
set -e

# ==============================================================================
# HotPlex Docker Entrypoint with OpenCode Sidecar Support
# Handles permission fixes, config env expansion, Git identity, PIP tools,
# OpenCode sidecar startup, and privilege drop.
# ==============================================================================

HOTPLEX_HOME="/home/hotplex"
CONFIG_DIR="${HOTPLEX_HOME}/.hotplex"

# Sidecar process tracking (global for trap access)
OPENCODE_SIDECAR_PID=""

# Helper: Check if a PID is running
is_process_alive() { [[ -n "${1}" ]] && kill -0 "${1}" 2>/dev/null; }

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
# Helper: Stop OpenCode sidecar gracefully (SIGTERM -> 10s grace -> SIGKILL)
# Safe to call multiple times; no-ops if PID is unset or process gone.
# ------------------------------------------------------------------------------
stop_opencode_sidecar() {
    if ! is_process_alive "${OPENCODE_SIDECAR_PID}"; then
        return 0
    fi
    echo "--> Stopping OpenCode sidecar (PID: ${OPENCODE_SIDECAR_PID})..."
    kill -TERM "${OPENCODE_SIDECAR_PID}" 2>/dev/null || true
    local i=0
    while [[ $i -lt 10 ]]; do
        is_process_alive "${OPENCODE_SIDECAR_PID}" || return 0
        sleep 1
        ((i++)) || true
    done
    kill -KILL "${OPENCODE_SIDECAR_PID}" 2>/dev/null || true
    echo "--> OpenCode sidecar force-killed (did not exit within 10s)"
}

# ------------------------------------------------------------------------------
# Helper: Start OpenCode Server as sidecar (background process)
# Sets OPENCODE_SIDECAR_PID on success.
# Logs to stdout/stderr for docker logs aggregation.
# ------------------------------------------------------------------------------
start_opencode_sidecar() {
    local enabled="${OPENCODE_SERVER_ENABLED:-true}"
    local port="${OPENCODE_SERVER_PORT:-4096}"
    local password="${OPENCODE_SERVER_PASSWORD:-}"

    if [[ "${enabled}" != "true" ]]; then
        echo "--> OpenCode sidecar disabled, skipping..."
        return 0
    fi

    if ! command -v opencode >/dev/null 2>&1; then
        echo "--> WARNING: opencode binary not found, skipping sidecar startup"
        return 0
    fi

    echo "==> Starting OpenCode Server sidecar on port ${port}..."

    local args=( "serve" "--port" "${port}" "--hostname" "127.0.0.1" )

    if [[ -n "${password}" ]]; then
        export OPENCODE_SERVER_PASSWORD="${password}"
    fi

    # Start opencode in background; stdout/stderr flow to container logs
    run_as_hotplex opencode "${args[@]}" &
    OPENCODE_SIDECAR_PID=$!
    # Wait for server to become ready (max 30s)
    local i=0
    while [[ $i -lt 30 ]]; do
        if ! is_process_alive "${OPENCODE_SIDECAR_PID}"; then
            echo "--> ERROR: OpenCode sidecar exited prematurely" >&2
            OPENCODE_SIDECAR_PID=""
            return 1
        fi
        if netstat -tuln 2>/dev/null | grep -q ":${port}" || ss -tuln 2>/dev/null | grep -q ":${port}"; then
            echo "--> OpenCode sidecar ready (PID: ${OPENCODE_SIDECAR_PID}, port: ${port})"
            return 0
        fi
        sleep 1
        ((i++)) || true
    done

    echo "--> WARNING: OpenCode sidecar not ready within 30s" >&2
    kill "${OPENCODE_SIDECAR_PID}" 2>/dev/null || true
    OPENCODE_SIDECAR_PID=""
    return 1
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

    # Fix .claude.json permissions (if existing as file or symlink)
    if [[ -e "${HOTPLEX_HOME}/.claude.json" ]]; then
        chown -h hotplex:hotplex "${HOTPLEX_HOME}/.claude.json" 2>/dev/null || true
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
        [[ -f "$yaml" ]] || continue
        # Use .tmp extension to avoid partial writes on crash
        if envsubst "${VARS}" < "$yaml" > "${yaml}.tmp"; then
            mv "${yaml}.tmp" "${yaml}"
        else
            echo "    Warning: Failed to expand variables in $(basename "$yaml")"
            rm -f "${yaml}.tmp"
        fi
    done
fi

# ------------------------------------------------------------------------------
# 4. Claude Code Runtime Files (named volume persists across restarts)
# ------------------------------------------------------------------------------
CLAUDE_DIR="${HOTPLEX_HOME}/.claude"
CLAUDE_JSON_PERSISTENT="${CLAUDE_DIR}/.claude.json"
CLAUDE_JSON_HOME="${HOTPLEX_HOME}/.claude.json"
CLAUDE_JSON_SEED="${HOTPLEX_HOME}/.claude.json.seed"

# Ensure container-private .claude directory exists (named volume auto-creates)
run_as_hotplex mkdir -p "${CLAUDE_DIR}"

# Seed & Clone strategy for .claude.json
# Prevents race conditions by isolating state in per-instance volumes
if [[ ! -f "${CLAUDE_JSON_PERSISTENT}" ]]; then
    if [[ -f "${CLAUDE_JSON_SEED}" ]]; then
        echo "--> Cloning .claude.json from seed source..."
        run_as_hotplex cp "${CLAUDE_JSON_SEED}" "${CLAUDE_JSON_PERSISTENT}"
    else
        echo "--> Creating empty .claude.json configuration file..."
        run_as_hotplex sh -c "echo '{}' > '${CLAUDE_JSON_PERSISTENT}'"
    fi
    
    # Ensure correct permissions for the new file
    [[ "$(id -u)" = "0" ]] && chown hotplex:hotplex "${CLAUDE_JSON_PERSISTENT}" 2>/dev/null || true
fi

# Ensure symlink for system & CLI compatibility
if [[ ! -L "${CLAUDE_JSON_HOME}" ]]; then
    echo "--> Linking persistent .claude.json to home directory..."
    rm -f "${CLAUDE_JSON_HOME}" 2>/dev/null || true
    run_as_hotplex ln -sf "${CLAUDE_JSON_PERSISTENT}" "${CLAUDE_JSON_HOME}"
fi

# ------------------------------------------------------------------------------
# 5. Git Identity Injection (from environment variables)
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
# 6. Auto-install pip tools (reinstalled on rebuild via entrypoint)
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
# 7. Start OpenCode Sidecar (if enabled)
# ------------------------------------------------------------------------------
start_opencode_sidecar || true

# ------------------------------------------------------------------------------
# 8. Execute CMD with lifecycle management
#    - No sidecar: exec for optimal PID 1 signal handling (via tini)
#    - Sidecar active: trap-based lifecycle with watchdog and graceful shutdown
# ------------------------------------------------------------------------------
if [[ -z "${OPENCODE_SIDECAR_PID}" ]]; then
    # No sidecar → exec replaces shell with main process (optimal signal path)
    echo "==> Starting HotPlex Engine..."
    if [[ "$(id -u)" = "0" ]]; then
        export HOME="${HOTPLEX_HOME}"
        exec runuser -u hotplex -m -- "$@"
    else
        exec "$@"
    fi
fi

# --- Sidecar lifecycle mode ---
echo "==> Starting HotPlex Engine (with sidecar lifecycle)..."

MAIN_PID=""
SIDECAR_RESTARTS=0
SIDECAR_MAX_RESTARTS="${OPENCODE_SIDECAR_MAX_RESTARTS:-3}"

# Graceful shutdown: stop sidecar then terminate main process
shutdown() {
    echo "==> Received shutdown signal, cleaning up..."
    stop_opencode_sidecar
    if is_process_alive "${MAIN_PID}"; then
        kill -TERM "${MAIN_PID}" 2>/dev/null || true
    fi
}
trap shutdown SIGTERM SIGINT

# Start main process in background
if [[ "$(id -u)" = "0" ]]; then
    export HOME="${HOTPLEX_HOME}"
    runuser -u hotplex -m -- "$@" &
else
    "$@" &
fi
MAIN_PID=$!

# Monitor loop: restart sidecar on crash, exit when main process dies
while true; do
    wait -n 2>/dev/null || true

    # Main process exited → clean up sidecar and exit
    if ! is_process_alive "${MAIN_PID}"; then
        echo "==> HotPlex Engine exited"
        stop_opencode_sidecar
        wait "${MAIN_PID}" 2>/dev/null
        MAIN_EXIT=$?
        exit ${MAIN_EXIT}
    fi

    # Sidecar crashed → attempt restart with backoff limit
    if [[ -n "${OPENCODE_SIDECAR_PID}" ]] && ! is_process_alive "${OPENCODE_SIDECAR_PID}"; then
        SIDECAR_RESTARTS=$((SIDECAR_RESTARTS + 1))
        if [[ ${SIDECAR_RESTARTS} -ge ${SIDECAR_MAX_RESTARTS} ]]; then
            echo "--> ERROR: OpenCode sidecar exceeded max restarts (${SIDECAR_MAX_RESTARTS}), giving up" >&2
            OPENCODE_SIDECAR_PID=""
        else
            echo "==> OpenCode sidecar crashed (restart #${SIDECAR_RESTARTS}/${SIDECAR_MAX_RESTARTS}), restarting..."
            sleep 2
            start_opencode_sidecar || echo "--> WARNING: Sidecar restart failed" >&2
        fi
    fi
done
