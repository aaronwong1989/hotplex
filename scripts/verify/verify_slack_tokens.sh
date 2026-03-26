#!/bin/bash
# =============================================================================
# Slack Token Verification Script
# =============================================================================
# Verifies Slack Bot Token (xoxb-) and App Token (xapp-) validity
# Usage: ./scripts/verify_slack_tokens.sh [env-file]
# =============================================================================

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
MATRIX_DIR="$PROJECT_ROOT/docker/matrix"

# Check for curl
if ! command -v curl &> /dev/null; then
    echo -e "${RED}Error: curl is required but not installed${NC}"
    exit 1
fi

# Function to verify Bot Token via auth.test API
verify_bot_token() {
    local token="$1"
    local name="$2"

    echo -e "${BLUE}Testing Bot Token ($name)...${NC}"

    # Mask token for display
    local masked_token="${token:0:20}...${token: -10}"
    echo "  Token: $masked_token"

    # Call Slack auth.test API
    local response
    response=$(curl -s -X POST "https://slack.com/api/auth.test" \
        -H "Authorization: Bearer $token" \
        -H "Content-Type: application/x-www-form-urlencoded")

    # Parse response
    local ok=$(echo "$response" | grep -o '"ok":[^,}]*' | cut -d':' -f2)

    if [ "$ok" = "true" ]; then
        local user=$(echo "$response" | grep -o '"user":"[^"]*"' | cut -d'"' -f4)
        local team=$(echo "$response" | grep -o '"team":"[^"]*"' | cut -d'"' -f4)
        local user_id=$(echo "$response" | grep -o '"user_id":"[^"]*"' | cut -d'"' -f4)
        local bot_id=$(echo "$response" | grep -o '"bot_id":"[^"]*"' | cut -d'"' -f4)

        echo -e "  ${GREEN}✓ Valid${NC}"
        echo "  User: $user ($user_id)"
        echo "  Team: $team"
        echo "  Bot ID: $bot_id"
        return 0
    else
        local error=$(echo "$response" | grep -o '"error":"[^"]*"' | cut -d'"' -f4)
        echo -e "  ${RED}✗ Invalid${NC}"
        echo "  Error: $error"
        return 1
    fi
}

# Function to verify App Token via apps.connections.open API
verify_app_token() {
    local token="$1"
    local name="$2"

    echo -e "${BLUE}Testing App Token ($name)...${NC}"

    # Mask token for display
    local masked_token="${token:0:20}...${token: -10}"
    echo "  Token: $masked_token"

    # Call Slack apps.connections.open API (Socket Mode)
    local response
    response=$(curl -s -X POST "https://slack.com/api/apps.connections.open" \
        -H "Authorization: Bearer $token" \
        -H "Content-Type: application/x-www-form-urlencoded")

    # Parse response
    local ok=$(echo "$response" | grep -o '"ok":[^,}]*' | cut -d':' -f2)

    if [ "$ok" = "true" ]; then
        echo -e "  ${GREEN}✓ Valid${NC}"
        echo "  Socket Mode: Ready"
        return 0
    else
        local error=$(echo "$response" | grep -o '"error":"[^"]*"' | cut -d'"' -f4)
        echo -e "  ${RED}✗ Invalid${NC}"
        echo "  Error: $error"
        return 1
    fi
}

# Function to verify tokens from an env file
verify_env_file() {
    local env_file="$1"
    local name=$(basename "$env_file")

    echo ""
    echo -e "${YELLOW}========================================${NC}"
    echo -e "${YELLOW}Verifying: $name${NC}"
    echo -e "${YELLOW}========================================${NC}"

    if [ ! -f "$env_file" ]; then
        echo -e "${RED}File not found: $env_file${NC}"
        return 1
    fi

    # Extract tokens
    local bot_token=$(grep "^HOTPLEX_SLACK_BOT_TOKEN=" "$env_file" | cut -d'=' -f2- | tr -d '"' | tr -d "'")
    local app_token=$(grep "^HOTPLEX_SLACK_APP_TOKEN=" "$env_file" | cut -d'=' -f2- | tr -d '"' | tr -d "'")
    local bot_id=$(grep "^HOTPLEX_SLACK_BOT_USER_ID=" "$env_file" | cut -d'=' -f2- | tr -d '"' | tr -d "'")

    echo "Bot User ID: $bot_id"

    local bot_valid=0
    local app_valid=0

    if [ -n "$bot_token" ]; then
        verify_bot_token "$bot_token" "$name" && bot_valid=1 || bot_valid=0
    else
        echo -e "${RED}  BOT_TOKEN not found in $name${NC}"
    fi

    echo ""

    if [ -n "$app_token" ]; then
        verify_app_token "$app_token" "$name" && app_valid=1 || app_valid=0
    else
        echo -e "${RED}  APP_TOKEN not found in $name${NC}"
    fi

    # Summary
    echo ""
    if [ $bot_valid -eq 1 ] && [ $app_valid -eq 1 ]; then
        echo -e "${GREEN}✓ $name: All tokens valid${NC}"
        return 0
    else
        echo -e "${RED}✗ $name: Some tokens invalid${NC}"
        return 1
    fi
}

# Main
main() {
    echo -e "${BLUE}============================================${NC}"
    echo -e "${BLUE}  Slack Token Verification Script${NC}"
    echo -e "${BLUE}============================================${NC}"

    if [ "$1" = "--help" ] || [ "$1" = "-h" ]; then
        echo "Usage: $0 [env-file]"
        echo "If no env-file is provided, verifies all .env-* files in $MATRIX_DIR"
        return 0
    fi

    local exit_code=0

    if [ -n "$1" ]; then
        # Verify specific file
        verify_env_file "$1" || exit_code=1
    else
        # Verify all .env-* files in matrix directory
        for env_file in "$MATRIX_DIR"/.env-0{1,2,3}; do
            verify_env_file "$env_file" || exit_code=1
        done
    fi

    echo ""
    echo -e "${BLUE}============================================${NC}"
    if [ $exit_code -eq 0 ]; then
        echo -e "${GREEN}All tokens verified successfully!${NC}"
    else
        echo -e "${RED}Some tokens failed verification${NC}"
    fi
    echo -e "${BLUE}============================================${NC}"

    return $exit_code
}

main "$@"
