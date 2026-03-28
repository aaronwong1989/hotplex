#!/bin/bash

# ==============================================================================
# HotPlex: Switch Provider for Bot Instances
# ==============================================================================
# Switches AI provider for Admin bot (host) and/or Docker matrix bots.
# Updates both .env files and YAML config files atomically.
#
# Usage:
#   ./scripts/switch-provider.sh              # Interactive mode
#   ./scripts/switch-provider.sh --help       # Show help
#
# Location: ./scripts/switch-provider.sh (project root relative)
# ==============================================================================

set -euo pipefail

# --- Colors & Styling ---
CYAN='\033[0;36m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
BOLD='\033[1m'
DIM='\033[2m'
NC='\033[0m'

# --- Path Resolution (absolute, no cd) ---
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
MATRIX_DIR="$PROJECT_ROOT/docker/matrix"

# Cleanup temp files on exit (after PROJECT_ROOT is defined)
cleanup() { find "$PROJECT_ROOT" -name "*.tmp" -delete 2>/dev/null || true; }
trap cleanup EXIT

# --- Constants ---
PROVIDER_TYPES=("claude-code" "opencode" "opencode-server" "pi")
VALID_MODELS=("opus" "sonnet" "haiku")

# Env key names (standardized across host + docker matrix)
ENV_KEY_TYPE="HOTPLEX_PROVIDER_TYPE"
ENV_KEY_MODEL="HOTPLEX_PROVIDER_MODEL"

# --- Early Flags ---
if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
    cat <<'USAGE'
Usage: ./scripts/switch-provider.sh

Interactive script to switch AI provider for HotPlex bot instances.

Targets:
  admin       Host-mode admin bot (.env + configs/admin/slack.yaml)
  bot-NN      Docker matrix bot (docker/matrix/.env-NN + configs/bot-NN/base/slack.yaml)
  all         All bots (admin + all docker matrix bots)

Providers:  claude-code | opencode | opencode-server | pi
Models:     opus | sonnet | haiku
USAGE
    exit 0
fi

# --- Banner ---
printf "${BLUE}${BOLD}╭──────────────────────────────────────────────────────────────────╮${NC}\n"
printf "${BLUE}${BOLD}│${NC}  ${CYAN}${BOLD}🔄 HotPlex: Switch Bot Provider${NC}                               ${BLUE}${BOLD}│${NC}\n"
printf "${BLUE}${BOLD}╰──────────────────────────────────────────────────────────────────╯${NC}\n"

# ============================================================================
# Helper Functions
# ============================================================================

# Read a value from .env file (first match, strip surrounding quotes)
env_get() {
    local file="$1" key="$2"
    grep "^${key}=" "$file" 2>/dev/null | head -1 | sed "s/^${key}=//" | tr -d '"' | tr -d "'"
}

# Set or update a value in .env file (single-pass awk, macOS-compatible)
env_set() {
    local file="$1" key="$2" value="$3"
    awk -v k="$key" -v v="$value" '
        $0 ~ "^" k "=" { print k "=" v; found=1; next }
        { print }
        END { if (!found) print k "=" v }
    ' "$file" > "${file}.tmp" && mv "${file}.tmp" "$file"
}

# Read provider type from YAML config (first `type:` under `provider:`)
yaml_get_provider() {
    local file="$1"
    # Match lines like "  type: opencode-server" (indented, under provider:)
    # Stop at first blank line or non-indented line to avoid message_store.type
    awk '
        /^[[:space:]]*type:/ && !found {
            gsub(/^[[:space:]]*type:[[:space:]]*/, "")
            gsub(/#.*/, "")
            gsub(/[[:space:]]*$/, "")
            print
            found = 1
        }
    ' "$file" 2>/dev/null
}

# Set provider type in YAML config (only the first `type:` under `provider:`)
yaml_set_provider() {
    local file="$1" value="$2"
    local comment="# enum: claude-code | opencode | opencode-server | pi"
    awk -v v="$value" -v c="$comment" '
        /^[[:space:]]*type:/ && !done {
            match($0, /^[[:space:]]*/);
            indent = substr($0, RSTART, RLENGTH);
            $0 = indent "type: " v "  " c
            done = 1
        }
        { print }
    ' "$file" > "${file}.tmp" && mv "${file}.tmp" "$file"
}

# Ensure or remove opencode: block in YAML provider section
# $1=file  $2=add|remove
yaml_manage_opencode() {
    local file="$1" action="$2"
    [[ -f "$file" ]] || return 1

    case "$action" in
        add)
            # Already has opencode block with password? Skip
            if awk '/^[[:space:]]*opencode:/{f=1} f && /password:/{exit 0} END{exit !f}' "$file"; then
                return 0
            fi
            # Find anchor: prefer last provider field (break on first match, ordered last→first)
            local anchor=""
            for a in "dangerously_skip_permissions:" "default_permission_mode:" "default_model:" "type:"; do
                if grep -q "^[[:space:]]*${a}" "$file" 2>/dev/null; then
                    anchor="$a"; break
                fi
            done
            [[ -z "$anchor" ]] && return 1
            awk -v a="$anchor" '
                $0 ~ "^[[:space:]]*" a {
                    print; print "  opencode:"
                    print "    server_url: http://127.0.0.1:4096"
                    print "    password: \${OPENCODE_SERVER_PASSWORD}"; next
                }
                { print }
            ' "$file" > "${file}.tmp" && mv "${file}.tmp" "$file"
            ;;
        remove)
            # Delete opencode: block + preceding blank line (buffered look-back)
            awk '
                /^[[:space:]]*opencode:[[:space:]]*$/ {
                    if (prev !~ /^[[:space:]]*$/) print prev
                    skip = 1; had_prev = 0; next
                }
                skip && /^[[:space:]]{4}/ { next }
                skip { skip = 0 }
                { if (had_prev) print prev; prev = $0; had_prev = 1 }
                END { if (had_prev) print prev }
            ' "$file" > "${file}.tmp" && mv "${file}.tmp" "$file"
            ;;
    esac
}

# Get instance home directory for a docker bot index
# Parses docker-compose.yml to find HOTPLEX_BOT_ID, then constructs path
instance_home_for_bot() {
    local idx="$1"
    local compose="$MATRIX_DIR/docker-compose.yml"
    [[ -f "$compose" ]] || return 1
    local bot_id
    bot_id=$(grep -A20 "hotplex-${idx}:" "$compose" 2>/dev/null \
        | grep "HOTPLEX_BOT_ID:" | sed 's/.*HOTPLEX_BOT_ID:[[:space:]]*//')
    [[ -n "$bot_id" && -d "$HOME/.hotplex/instances/$bot_id" ]] || return 1
    echo "$HOME/.hotplex/instances/$bot_id"
}

# Validate provider type
validate_provider() {
    case "$1" in
        claude-code|opencode|opencode-server|pi) return 0 ;;
    esac
    return 1
}

# Provider display name with icon
provider_display() {
    case "$1" in
        claude-code)      printf "🟠 claude-code" ;;
        opencode)         printf "🔵 opencode" ;;
        opencode-server)  printf "🟢 opencode-server" ;;
        pi)               printf "🟣 pi" ;;
        *)                printf "❓ $1" ;;
    esac
}

# Provider color code for table formatting
provider_color() {
    case "$1" in
        claude-code)      printf "%s" "$YELLOW" ;;
        opencode)         printf "%s" "$BLUE" ;;
        opencode-server)  printf "%s" "$GREEN" ;;
        pi)               printf "%s" "$RED" ;;
        *)                printf "%s" "$DIM" ;;
    esac
}

# ============================================================================
# Step 1: Scan & Display Current Status
# ============================================================================

printf "\n${BOLD}${BLUE}📊 Step 1: Current Provider Status${NC}\n"
printf "${DIM}──────────────────────────────────────────────────────────────────${NC}\n"
printf "  ${BOLD}%-6s %-12s %-20s %-10s %-12s${NC}\n" "Bot" "Mode" "Provider" "Model" "Source"
printf "${DIM}  ──────────────────────────────────────────────────────────────${NC}\n"

# --- Admin Bot (Host Mode) ---
ADMIN_ENV="$PROJECT_ROOT/.env"
ADMIN_YAML="$PROJECT_ROOT/configs/admin/slack.yaml"
ADMIN_BASE_YAML="$PROJECT_ROOT/configs/base/slack.yaml"

if [[ -f "$ADMIN_ENV" ]]; then
    ADMIN_PROVIDER=$(env_get "$ADMIN_ENV" "$ENV_KEY_TYPE")
    if [[ -z "$ADMIN_PROVIDER" && -f "$ADMIN_YAML" ]]; then
        ADMIN_PROVIDER=$(yaml_get_provider "$ADMIN_YAML")
    fi
    if [[ -z "$ADMIN_PROVIDER" && -f "$ADMIN_BASE_YAML" ]]; then
        ADMIN_PROVIDER=$(yaml_get_provider "$ADMIN_BASE_YAML")
    fi
    ADMIN_MODEL=$(env_get "$ADMIN_ENV" "$ENV_KEY_MODEL")
    [[ -z "$ADMIN_MODEL" ]] && ADMIN_MODEL="-"

    p_color=$(provider_color "${ADMIN_PROVIDER:-?}")
    printf "  ${CYAN}%-6s${NC} %-12s ${p_color}%-20s${NC} %-10s %-12s\n" \
        "admin" "host" "${ADMIN_PROVIDER:-?}" "$ADMIN_MODEL" ".env"
fi

# --- Docker Matrix Bots ---
BOT_COUNT=0
for env_file in "$MATRIX_DIR"/.env-[0-9][0-9]; do
    [[ -f "$env_file" ]] || continue
    ((BOT_COUNT++)) || true
    BOT_IDX="${env_file##*/.env-}"

    BOT_PROVIDER=$(env_get "$env_file" "$ENV_KEY_TYPE")
    BOT_MODEL=$(env_get "$env_file" "$ENV_KEY_MODEL")
    [[ -z "$BOT_MODEL" ]] && BOT_MODEL="-"

    p_color=$(provider_color "${BOT_PROVIDER:-?}")
    printf "  ${CYAN}%-6s${NC} %-12s ${p_color}%-20s${NC} %-10s %-12s\n" \
        "bot-$BOT_IDX" "docker" "${BOT_PROVIDER:-?}" "$BOT_MODEL" ".env-$BOT_IDX"
done

if [[ $BOT_COUNT -eq 0 ]]; then
    printf "  ${DIM}(no docker matrix bots found)${NC}\n"
fi

printf "${DIM}  ────────────────────────────────────────────────────────────${NC}\n"

# ============================================================================
# Step 2: Select Target Bot(s)
# ============================================================================

printf "\n${BOLD}${BLUE}🎯 Step 2: Select Target${NC}\n"
printf "  Available: "
printf "${CYAN}admin${NC} (host)  "
for env_file in "$MATRIX_DIR"/.env-[0-9][0-9]; do
    [[ -f "$env_file" ]] || continue
    BOT_IDX="${env_file##*/.env-}"
    printf "${CYAN}bot-%s${NC}  " "$BOT_IDX"
done
printf "${CYAN}all${NC}\n"

printf "\n  ${YELLOW}Select target [admin / bot-NN / all]:${NC} "
read -r TARGET

# Validate target and resolve to absolute paths
TARGETS=()
case "$TARGET" in
    admin)
        if [[ ! -f "$ADMIN_ENV" ]]; then
            printf "  ${RED}✗ Admin .env not found at $ADMIN_ENV${NC}\n"
            exit 1
        fi
        TARGETS=("admin")
        ;;
    bot-*)
        BOT_IDX="${TARGET#bot-}"
        ENV_FILE="$MATRIX_DIR/.env-${BOT_IDX}"
        if [[ ! -f "$ENV_FILE" ]]; then
            printf "  ${RED}✗ $ENV_FILE not found${NC}\n"
            exit 1
        fi
        TARGETS=("bot-${BOT_IDX}")
        ;;
    all)
        [[ -f "$ADMIN_ENV" ]] && TARGETS+=("admin")
        for env_file in "$MATRIX_DIR"/.env-[0-9][0-9]; do
            [[ -f "$env_file" ]] || continue
            BOT_IDX="${env_file##*/.env-}"
            TARGETS+=("bot-${BOT_IDX}")
        done
        ;;
    *)
        printf "  ${RED}✗ Invalid target: $TARGET${NC}\n"
        exit 1
        ;;
esac

printf "  ${GREEN}✓${NC} Target: ${BOLD}${TARGETS[*]}${NC}\n"

# ============================================================================
# Step 3: Select New Provider Type
# ============================================================================

printf "\n${BOLD}${BLUE}🔀 Step 3: Select Provider${NC}\n"
printf "  Available:\n"
printf "    ${CYAN}1)${NC} claude-code      ${DIM}(Anthropic Claude Code CLI)${NC}\n"
printf "    ${CYAN}2)${NC} opencode          ${DIM}(OpenCode CLI)${NC}\n"
printf "    ${CYAN}3)${NC} opencode-server   ${DIM}(OpenCode Server - HTTP/SSE)${NC}\n"
printf "    ${CYAN}4)${NC} pi                ${DIM}(Pi provider)${NC}\n"

printf "\n  ${YELLOW}Select provider [1-4]:${NC} "
read -r PROVIDER_CHOICE

case "$PROVIDER_CHOICE" in
    1) NEW_PROVIDER="claude-code" ;;
    2) NEW_PROVIDER="opencode" ;;
    3) NEW_PROVIDER="opencode-server" ;;
    4) NEW_PROVIDER="pi" ;;
    *)
        if validate_provider "$PROVIDER_CHOICE"; then
            NEW_PROVIDER="$PROVIDER_CHOICE"
        else
            printf "  ${RED}✗ Invalid provider: $PROVIDER_CHOICE${NC}\n"
            exit 1
        fi
        ;;
esac

printf "  ${GREEN}✓${NC} Provider: ${BOLD}"
provider_display "$NEW_PROVIDER"
printf "${NC}\n"

# ============================================================================
# Step 4: Select Model
# ============================================================================

printf "\n${BOLD}${BLUE}🧠 Step 4: Select Model${NC}\n"
printf "  Available: ${CYAN}opus${NC} (most capable) | ${CYAN}sonnet${NC} (balanced) | ${CYAN}haiku${NC} (fast)\n"

# Get current default from first target
CURRENT_MODEL=""
for t in "${TARGETS[@]}"; do
    case "$t" in
        admin)
            CURRENT_MODEL=$(env_get "$ADMIN_ENV" "$ENV_KEY_MODEL")
            ;;
        bot-*)
            BOT_IDX="${t#bot-}"
            CURRENT_MODEL=$(env_get "$MATRIX_DIR/.env-${BOT_IDX}" "$ENV_KEY_MODEL")
            ;;
    esac
    [[ -n "$CURRENT_MODEL" ]] && break
done
[[ -z "$CURRENT_MODEL" ]] && CURRENT_MODEL="opus"

printf "  ${YELLOW}Model [${CURRENT_MODEL}]:${NC} "
read -r INPUT_MODEL
NEW_MODEL="${INPUT_MODEL:-$CURRENT_MODEL}"

# Validate model
case "$NEW_MODEL" in
    opus|sonnet|haiku) ;;
    *) printf "  ${YELLOW}⚠ Unknown model '%s' — expected: opus | sonnet | haiku${NC}\n" "$NEW_MODEL" ;;
esac

printf "  ${GREEN}✓${NC} Model: ${BOLD}${NEW_MODEL}${NC}\n"

# ============================================================================
# Step 5: Confirm & Apply
# ============================================================================

printf "\n${BOLD}${BLUE}✅ Step 5: Confirm Changes${NC}\n"
printf "  ${BOLD}Targets:${NC}   ${TARGETS[*]}\n"
printf "  ${BOLD}Provider:${NC}  "
provider_display "$NEW_PROVIDER"
printf "\n"
printf "  ${BOLD}Model:${NC}     ${NEW_MODEL}\n"

printf "\n  ${YELLOW}Apply changes? [y/N]:${NC} "
read -r CONFIRM
[[ "$CONFIRM" != "y" && "$CONFIRM" != "Y" ]] && { printf "  ${DIM}Cancelled.${NC}\n"; exit 0; }

# --- Apply ---
CHANGES=0

for t in "${TARGETS[@]}"; do
    case "$t" in
        admin)
            TARGET_ENV="$ADMIN_ENV"
            TARGET_YAML="$ADMIN_YAML"
            ;;
        bot-*)
            BOT_IDX="${t#bot-}"
            TARGET_ENV="$MATRIX_DIR/.env-${BOT_IDX}"
            TARGET_YAML="$MATRIX_DIR/configs/bot-${BOT_IDX}/base/slack.yaml"
            ;;
    esac

    # Update .env file
    if [[ -f "$TARGET_ENV" ]]; then
        env_set "$TARGET_ENV" "$ENV_KEY_TYPE" "$NEW_PROVIDER"
        env_set "$TARGET_ENV" "$ENV_KEY_MODEL" "$NEW_MODEL"

        printf "  ${GREEN}✓${NC} ${BOLD}$(basename "$TARGET_ENV")${NC}: "
        provider_display "$NEW_PROVIDER"
        printf " / %s\n" "$NEW_MODEL"
        ((CHANGES++)) || true
    else
        printf "  ${YELLOW}⚠${NC} ${BOLD}$(basename "$TARGET_ENV")${NC} not found, skipping .env update\n"
    fi

    # Update YAML config (template)
    if [[ -f "$TARGET_YAML" ]]; then
        yaml_set_provider "$TARGET_YAML" "$NEW_PROVIDER"
        [[ "$NEW_PROVIDER" == "opencode-server" ]] && yaml_manage_opencode "$TARGET_YAML" add
        [[ "$NEW_PROVIDER" != "opencode-server" ]] && yaml_manage_opencode "$TARGET_YAML" remove
        printf "  ${GREEN}✓${NC} ${BOLD}${TARGET_YAML#$PROJECT_ROOT/}${NC}: type: ${NEW_PROVIDER}\n"
        ((CHANGES++)) || true
    else
        printf "  ${YELLOW}⚠${NC} YAML not found at ${TARGET_YAML#$PROJECT_ROOT/}, skipping YAML update\n"
    fi

    # Update instance configs (running Docker configs, hot-reloaded)
    if [[ "$t" == bot-* ]]; then
        INSTANCE_HOME=$(instance_home_for_bot "$BOT_IDX") || true
        if [[ -n "$INSTANCE_HOME" ]]; then
            for yfile in "$INSTANCE_HOME"/configs/base/{slack,feishu}.yaml; do
                [[ -f "$yfile" ]] || continue
                yaml_set_provider "$yfile" "$NEW_PROVIDER"
                [[ "$NEW_PROVIDER" == "opencode-server" ]] && yaml_manage_opencode "$yfile" add
                [[ "$NEW_PROVIDER" != "opencode-server" ]] && yaml_manage_opencode "$yfile" remove
                printf "  ${GREEN}✓${NC} ${BOLD}${yfile#$HOME/}${NC}: synced\n"
                ((CHANGES++)) || true
            done
        fi
    fi
done

# ============================================================================
# Step 6: Summary
# ============================================================================

printf "\n${GREEN}${BOLD}✨ Done! ${CHANGES} file(s) updated.${NC}\n"
printf "──────────────────────────────────────────────────────────────────\n"

if [[ "$NEW_PROVIDER" == "opencode-server" ]]; then
    printf "\n${YELLOW}⚠ OpenCode Server notes:${NC}\n"
    printf "  - Ensure OPENCODE_SERVER_PASSWORD is set in .env-* files\n"
    printf "  - Docker sidecar must be enabled (OPENCODE_SERVER_ENABLED=true)\n"
    printf "  - Restart bots: ${BOLD}cd docker/matrix && make docker-restart${NC}\n"
elif [[ "$NEW_PROVIDER" == "claude-code" ]]; then
    printf "\n${YELLOW}⚠ Claude Code notes:${NC}\n"
    printf "  - Requires claude CLI installed and authenticated\n"
    printf "  - Ensure ANTHROPIC_API_KEY or claude login is configured\n"
fi

printf "\n${DIM}To restart host admin bot: hotplexd restart${NC}\n"
printf "${DIM}To restart docker bots: cd docker/matrix && make docker-restart${NC}\n\n"
