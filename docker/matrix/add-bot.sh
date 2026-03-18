#!/bin/bash

# ==============================================================================
# HotPlex Matrix: Interactive Bot Addition Script
# ==============================================================================
# Supports the hotplex-matrix-*-* named volume architecture.
# Convention over Configuration.
# ==============================================================================

# --- Colors & Styling ---
CYAN='\033[0;36m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
PURPLE='\033[0;35m'
BLUE='\033[0;34m'
BOLD='\033[1m'
NC='\033[0m' # No Color

# --- Initialization ---
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR" || exit 1

# --- Banner ---
printf "${BLUE}${BOLD}╭──────────────────────────────────────────────────────────────────╮${NC}\n"
printf "${BLUE}${BOLD}│${NC}  ${CYAN}${BOLD}🤖 HotPlex Matrix: Add New Bot Instance${NC}                     ${BLUE}${BOLD}│${NC}\n"
printf "${BLUE}${BOLD}╰──────────────────────────────────────────────────────────────────╯${NC}\n"

# --- 1. Auto-Discovery ---
printf "\n${BOLD}${BLUE}🔍 Step 1: Discovering Environment...${NC}\n"

# naming: hotplex-01, hotplex-02, ...
BOT_INDEX=1
while [ -f ".env-$(printf "%02d" $BOT_INDEX)" ]; do
    ((BOT_INDEX++))
done

BOT_PADDED_INDEX=$(printf "%02d" $BOT_INDEX)
SERVICE_NAME="hotplex-$BOT_PADDED_INDEX"
ENV_FILE=".env-$BOT_PADDED_INDEX"

# Role: first bot is primary, subsequent are secondary
if [ "$BOT_INDEX" -eq 1 ]; then
    BOT_ROLE="primary"
else
    BOT_ROLE="secondary"
fi

# Detect next available port (Start at 18080 for bot-01, 18081 for bot-02, ...)
BASE_PORT=18080
PORT=$((BASE_PORT + BOT_INDEX - 1))
# Ensure port is not already used in docker-compose.yml
while grep -q ":$PORT:8080" docker-compose.yml 2>/dev/null; do
    ((PORT++))
done

# Named volume for Claude state
CLAUDE_VOLUME="hotplex-matrix-claude-$BOT_PADDED_INDEX"
# Named volume for per-instance Go build cache (isolated)
BUILD_VOLUME="hotplex-matrix-go-build-$BOT_PADDED_INDEX"

# Inherit from .env-01
if [ -f ".env-01" ]; then
    DEFAULT_OWNER=$(grep "^HOTPLEX_SLACK_PRIMARY_OWNER=" .env-01 2>/dev/null | cut -d= -f2 | tr -d ' ' | tr -d '\r')
    DEFAULT_GITHUB_TOKEN=$(grep "^GITHUB_TOKEN=" .env-01 2>/dev/null | cut -d= -f2 | tr -d ' ' | tr -d '\r')
    DEFAULT_GITHUB_PAT=$(grep "^GITHUB_PERSONAL_ACCESS_TOKEN=" .env-01 2>/dev/null | cut -d= -f2 | tr -d ' ' | tr -d '\r')
    DEFAULT_IMAGE=$(grep "^HOTPLEX_IMAGE=" .env-01 2>/dev/null | cut -d= -f2 | tr -d ' ' | tr -d '\r')
fi

# Generate API Key
NEW_API_KEY=$(LC_ALL=C tr -dc 'a-f0-9' < /dev/urandom | head -c 64)

printf "  ${GREEN}✓${NC} Bot Index: ${BOLD}${BOT_INDEX}${NC}\n"
printf "  ${GREEN}✓${NC} Role: ${BOLD}${BOT_ROLE}${NC}\n"
printf "  ${GREEN}✓${NC} Target Port: ${BOLD}${PORT}${NC}\n"
printf "  ${GREEN}✓${NC} Claude Volume: ${BOLD}${CLAUDE_VOLUME}${NC}\n"
printf "  ${GREEN}✓${NC} Build Cache Volume: ${BOLD}${BUILD_VOLUME}${NC}\n"
printf "  ${GREEN}✓${NC} Generated API Key: ${BOLD}${NEW_API_KEY:0:8}...${NC}\n"

# --- 2. Interactive Prompts ---
printf "\n${BOLD}${BLUE}⌨️  Step 2: Configuration (Mandatory)${NC}\n"

# Mandatory Fields
while [ -z "$HOTPLEX_BOT_ID" ]; do
    printf "  ${YELLOW}Input HOTPLEX_BOT_ID (Slack User ID):${NC} "
    read -r HOTPLEX_BOT_ID
done

while [ -z "$HOTPLEX_SLACK_BOT_TOKEN" ]; do
    printf "  ${YELLOW}Input HOTPLEX_SLACK_BOT_TOKEN (xoxb-...):${NC} "
    read -r HOTPLEX_SLACK_BOT_TOKEN
done

while [ -z "$HOTPLEX_SLACK_APP_TOKEN" ]; do
    printf "  ${YELLOW}Input HOTPLEX_SLACK_APP_TOKEN (xapp-...):${NC} "
    read -r HOTPLEX_SLACK_APP_TOKEN
done

# Optional / Default Fields
printf "\n${BOLD}${BLUE}⚙️  Step 3: Configuration (Defaults)${NC}\n"

printf "  ${CYAN}Primary Owner ID [${DEFAULT_OWNER}]:${NC} "
read -r INPUT_OWNER
HOTPLEX_SLACK_PRIMARY_OWNER=${INPUT_OWNER:-$DEFAULT_OWNER}

printf "  ${CYAN}GitHub Token [${DEFAULT_GITHUB_TOKEN:0:8}...]:${NC} "
read -r INPUT_GH
GITHUB_TOKEN=${INPUT_GH:-$DEFAULT_GITHUB_TOKEN}

printf "  ${CYAN}GitHub PAT [${DEFAULT_GITHUB_PAT:0:8}...]:${NC} "
read -r INPUT_GH_PAT
GITHUB_PERSONAL_ACCESS_TOKEN=${INPUT_GH_PAT:-$DEFAULT_GITHUB_PAT}

printf "  ${CYAN}Docker Image [${DEFAULT_IMAGE:-hotplex:go}]:${NC} "
read -r INPUT_IMG
HOTPLEX_IMAGE=${INPUT_IMG:-${DEFAULT_IMAGE:-hotplex:go}}

# Log Level selection
printf "\n  ${CYAN}Log Level [debug/info/warn/error, default: debug]:${NC} "
read -r INPUT_LOG_LEVEL
HOTPLEX_LOG_LEVEL=${INPUT_LOG_LEVEL:-debug}

# CORS Origins (Optional)
DEFAULT_ORIGINS="http://localhost:8080,http://127.0.0.1:8080,http://host.docker.internal:8080"
printf "  ${CYAN}CORS Origins (comma-separated) [${DEFAULT_ORIGINS}]:${NC} "
read -r INPUT_ORIGINS
HOTPLEX_ALLOWED_ORIGINS=${INPUT_ORIGINS:-$DEFAULT_ORIGINS}

# Proxy Configuration (Optional)
printf "  ${CYAN}HTTP Proxy [none]:${NC} "
read -r INPUT_HTTP_PROXY
HTTP_PROXY=${INPUT_HTTP_PROXY:-}

printf "  ${CYAN}HTTPS Proxy [none]:${NC} "
read -r INPUT_HTTPS_PROXY
HTTPS_PROXY=${INPUT_HTTPS_PROXY:-}

BOT_NAME="Bot $BOT_INDEX"
GIT_USER="HotPlex secondary-$BOT_INDEX"
GIT_EMAIL="bot$BOT_INDEX@hotplex.dev"

# --- 4. Generate .env File ---
printf "\n${BOLD}${BLUE}📝 Step 4: Writing Configuration Files...${NC}\n"

# Convention: Only bot-specific variables in .env-*
# All other defaults come from common.yml
cat > "$ENV_FILE" <<EOF
# ==============================================================================
# HotPlex Environment Configuration - Bot $BOT_PADDED_INDEX
# Generated by add-bot.sh on $(date)
# ==============================================================================
# Convention over Configuration:
# - Default values are defined in common.yml
# - Only override bot-specific variables here
# ==============================================================================

# --- Identity (Required - Bot-specific) ---
HOTPLEX_BOT_ID=$HOTPLEX_BOT_ID

# --- Core Server ---
HOTPLEX_PORT=8080
HOTPLEX_API_KEY=$NEW_API_KEY
HOTPLEX_LOG_LEVEL=$HOTPLEX_LOG_LEVEL

# --- CORS (Optional - for Web UI access) ---
HOTPLEX_ALLOWED_ORIGINS=$HOTPLEX_ALLOWED_ORIGINS

# --- Proxy (Optional) ---
HTTP_PROXY=$HTTP_PROXY
HTTPS_PROXY=$HTTPS_PROXY

# --- Provider (Optional override) ---
# HOTPLEX_PROVIDER_MODEL=sonnet

# --- Slack Bot (Required - Bot-specific) ---
HOTPLEX_SLACK_BOT_USER_ID=$HOTPLEX_BOT_ID
HOTPLEX_SLACK_BOT_TOKEN=$HOTPLEX_SLACK_BOT_TOKEN
HOTPLEX_SLACK_APP_TOKEN=$HOTPLEX_SLACK_APP_TOKEN
HOTPLEX_SLACK_PRIMARY_OWNER=$HOTPLEX_SLACK_PRIMARY_OWNER

# --- GitHub (Required for Git operations) ---
GITHUB_TOKEN=$GITHUB_TOKEN
GITHUB_PERSONAL_ACCESS_TOKEN=$GITHUB_PERSONAL_ACCESS_TOKEN

# --- Git Identity (Optional) ---
GIT_USER_NAME="$GIT_USER"
GIT_USER_EMAIL=$GIT_EMAIL

# --- Orchestration (Auto-detected) ---
COMPOSE_FILE=docker-compose.yml
HOTPLEX_IMAGE=$HOTPLEX_IMAGE
HOST_UID=$(id -u)
EOF

printf "  ${GREEN}✓${NC} Created ${BOLD}$ENV_FILE${NC}\n"

# --- 5. Update docker-compose.yml ---

# 5a. Add named volume to top-level volumes: section (macOS-compatible)
if grep -q "^  $CLAUDE_VOLUME:" docker-compose.yml 2>/dev/null; then
    printf "  ${YELLOW}⚠${NC} Volume ${BOLD}$CLAUDE_VOLUME${NC} already exists in compose file.\n"
else
    if grep -q "^services:" docker-compose.yml; then
        # Use awk to insert before "services:" (macOS sed incompatible with \n in replacement)
        awk -v claude="$CLAUDE_VOLUME" -v build="$BUILD_VOLUME" '
            /^services:/ {
                print "  # Per-instance Claude state (auto-added by add-bot.sh)"
                print "  " claude ": { name: " claude " }"
                print "  # Per-instance Go build cache (auto-added by add-bot.sh)"
                print "  " build ": { name: " build " }"
                print ""
            }
            { print }
        ' docker-compose.yml > docker-compose.yml.tmp && mv docker-compose.yml.tmp docker-compose.yml
    else
        # No services section yet, append at end
        printf "\nvolumes:\n  $CLAUDE_VOLUME: { name: $CLAUDE_VOLUME }\n  $BUILD_VOLUME: { name: $BUILD_VOLUME }\n" >> docker-compose.yml
    fi
    printf "  ${GREEN}✓${NC} Added volumes ${BOLD}$CLAUDE_VOLUME${NC} and ${BOLD}$BUILD_VOLUME${NC} to top-level volumes.\n"
fi

# 5b. Append service definition (variables expanded via printf then passed to cat)
ROLE_DISPLAY="$(echo "$BOT_ROLE" | awk '{print toupper(substr($0,1,1)) tolower(substr($0,2))}')"
SERVICE_BLOCK=$(printf "\
  # ============================================================================
  # Bot %s: %s Instance (Auto-added by add-bot.sh)
  # ============================================================================
  %s:
    extends:
      file: common.yml
      service: hotplex-base
    container_name: %s
    ports: [ \"127.0.0.1:%s:8080\" ]
    env_file: [ %s ]
    volumes:
      # Claude state (named Docker volume for isolation)
      - %s:/home/hotplex/.claude:rw
      # Per-instance Go build cache (isolated)
      - %s:/home/hotplex/.cache/go-build:rw
      # Bot instance data (host path)
      - ~/.hotplex/instances/%s:/home/hotplex/.hotplex:rw
      # Project workspaces (host path)
      - ~/.hotplex/instances/%s/projects:/home/hotplex/projects:rw
    labels:
      - \"hotplex.bot.id=%s\"
      - \"hotplex.bot.role=%s\"
" "$BOT_PADDED_INDEX" "$ROLE_DISPLAY" "$SERVICE_NAME" "$SERVICE_NAME" \
  "$PORT" "$ENV_FILE" "$CLAUDE_VOLUME" "$BUILD_VOLUME" \
  "$HOTPLEX_BOT_ID" "$HOTPLEX_BOT_ID" "$HOTPLEX_BOT_ID" "$BOT_ROLE")

printf "\n%s\n" "$SERVICE_BLOCK" >> docker-compose.yml

printf "  ${GREEN}✓${NC} Service ${BOLD}$SERVICE_NAME${NC} added to compose file.\n"

# --- 6. Create bot config directory structure ---
printf "  ${GREEN}⟳${NC} Creating ${BOLD}configs/bot-$BOT_PADDED_INDEX/${NC} directory...\n"

BOT_CONFIG_DIR="configs/bot-$BOT_PADDED_INDEX"
mkdir -p "$BOT_CONFIG_DIR/base"

# Copy base templates from ../../configs/base/
if [ -d "../../configs/base" ]; then
    cp -r ../../configs/base/* "$BOT_CONFIG_DIR/base/" 2>/dev/null
    printf "  ${GREEN}✓${NC} Copied base templates to ${BOLD}$BOT_CONFIG_DIR/base/${NC}\n"
else
    printf "  ${YELLOW}⚠${NC} Base templates not found at ../../configs/base, skipping.\n"
fi

# Generate slack.yaml with inherits
cat > "$BOT_CONFIG_DIR/slack.yaml" <<EOF
# =============================================================================
# HotPlex Slack Adapter Configuration - Bot $BOT_PADDED_INDEX
# Generated by add-bot.sh on $(date)
# =============================================================================
inherits: ./base/slack.yaml

security:
  permission:
    bot_user_id: \${HOTPLEX_SLACK_BOT_USER_ID}

EOF
printf "  ${GREEN}✓${NC} Created ${BOLD}$BOT_CONFIG_DIR/slack.yaml${NC} with inherits\n"

# Generate server.yaml with inherits
cat > "$BOT_CONFIG_DIR/server.yaml" <<EOF
# =============================================================================
# HotPlex Server Configuration - Bot $BOT_PADDED_INDEX
# Generated by add-bot.sh on $(date)
# =============================================================================
inherits: ./base/server.yaml

EOF
printf "  ${GREEN}✓${NC} Created ${BOLD}$BOT_CONFIG_DIR/server.yaml${NC} with inherits\n"

# --- 7. Summary & Next Steps ---
printf "\n${GREEN}${BOLD}✨ Success! Bot $BOT_INDEX is ready to roll.${NC}\n"
printf "──────────────────────────────────────────────────────────────────\n"
printf "  ${BOLD}Bot ID:${NC}        $HOTPLEX_BOT_ID\n"
printf "  ${BOLD}Role:${NC}          $BOT_ROLE\n"
printf "  ${BOLD}Port:${NC}          $PORT\n"
printf "  ${BOLD}Claude Volume:${NC}      $CLAUDE_VOLUME\n"
printf "  ${BOLD}Build Cache Volume:${NC} $BUILD_VOLUME\n"
printf "  ${BOLD}Env File:${NC}          $ENV_FILE\n"
printf "──────────────────────────────────────────────────────────────────\n"
printf "\n${YELLOW}Next Steps:${NC}\n"
printf "  1. Create named Docker volumes (if not auto-created by compose):\n"
printf "     ${BOLD}docker volume create $CLAUDE_VOLUME${NC}\n"
printf "     ${BOLD}docker volume create $BUILD_VOLUME${NC}\n"
printf "  2. Prepare host directory:\n"
printf "     ${BOLD}mkdir -p ~/.hotplex/instances/$HOTPLEX_BOT_ID/projects${NC}\n"
printf "  3. Start the bot:\n"
printf "     ${BOLD}make docker-up${NC}\n"
printf "\n"
