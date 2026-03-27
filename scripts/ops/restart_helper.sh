#!/bin/bash
BIN_PATH=$1
LOG_FILE=$2

if [ -z "$BIN_PATH" ] || [ -z "$LOG_FILE" ]; then
    echo "Usage: $0 <binary_path> <log_file>"
    exit 1
fi

OLD_PID=$(pgrep -f "$(basename "$BIN_PATH")" | head -1 || echo "")
if [ -n "$OLD_PID" ]; then
    echo "🛑 Stopping old daemon (PID: $OLD_PID)..."
    pkill -f "$(basename "$BIN_PATH")" || true
    echo "Waiting for process to exit and release ports..."

    count=0
    while [ $count -lt 5 ]; do
        if ! pgrep -f "$(basename "$BIN_PATH")" > /dev/null; then
            echo "✅ Old daemon stopped."
            break
        fi
        sleep 1
        count=$((count+1))
    done

    if pgrep -f "$(basename "$BIN_PATH")" > /dev/null; then
        echo "⚠️  Force killing old daemon..."
        pkill -9 -f "$(basename "$BIN_PATH")" || true
    fi
else
    echo "ℹ️  No running daemon found."
fi

mkdir -p "$(dirname "$LOG_FILE")"
> "$LOG_FILE"

echo "🔥 Starting NEW HotPlex Daemon..."

# Load .env file if exists (priority: ./.env, ~/.hotplex/.env)
ENV_FILE=""
if [ -f ".env" ]; then
    ENV_FILE=".env"
elif [ -f "$HOME/.hotplex/.env" ]; then
    ENV_FILE="$HOME/.hotplex/.env"
fi

if [ -n "$ENV_FILE" ]; then
    echo "📋 Loading environment from: $ENV_FILE"

    # Create temporary file with cleaned environment variables
    TEMP_ENV=$(mktemp /tmp/hotplex_env.XXXXXX)

    # Process .env file and create clean version without comments
    while IFS= read -r line || [ -n "$line" ]; do
        # Skip comments (lines starting with #) and empty lines
        [[ "$line" =~ ^[[:space:]]*# ]] && continue
        [[ -z "$line" ]] && continue

        # Only keep lines with = sign (actual variable assignments)
        if [[ "$line" == *"="* ]]; then
            echo "$line" >> "$TEMP_ENV"
        fi
    done < "$ENV_FILE"

    # Debug: Check if password was extracted
    if grep -q "HOTPLEX_OPEN_CODE_PASSWORD" "$TEMP_ENV"; then
        echo "🔑 Password found in env file"
    else
        echo "⚠️  Warning: HOTPLEX_OPEN_CODE_PASSWORD not found in $ENV_FILE"
    fi

    # Start daemon with environment variables from temp file
    # Export all variables to current shell first
    set -a
    source "$TEMP_ENV"
    set +a

    # Debug: Verify password is available
    echo "🔍 Debug: Password before daemon start: ${HOTPLEX_OPEN_CODE_PASSWORD:0:10}..." >> "$LOG_FILE"

    # Start daemon - it will inherit environment from current shell
    # Set HOTPLEX_CHATAPPS_CONFIG_DIR to point to synced config directory
    export HOTPLEX_CHATAPPS_CONFIG_DIR="$HOME/.hotplex/configs"

    # Debug: Verify password is available
    echo "🔍 Debug: Password before daemon start: ${HOTPLEX_OPEN_CODE_PASSWORD:0:10}..." >> "$LOG_FILE"

    nohup "$BIN_PATH" start >> "$LOG_FILE" 2>&1 &
    DAEMON_PID=$!
    disown

    echo "🚀 Daemon started with PID: $DAEMON_PID" >> "$LOG_FILE"

    # Clean up temp file
    rm -f "$TEMP_ENV"
else
    nohup "$BIN_PATH" start >> "$LOG_FILE" 2>&1 & disown
fi

sleep 2

NEW_PID=$(pgrep -f "$(basename "$BIN_PATH")" | head -1 || echo "")
if [ -z "$NEW_PID" ] || [ "$NEW_PID" = "$OLD_PID" ]; then
    echo "❌ Restart FAILED. Check $LOG_FILE for errors (e.g., Port already in use)."
    exit 1
fi

COMMIT=$("$BIN_PATH" --version 2>/dev/null | grep Commit || echo "unknown")
echo "✅ Successfully restarted!"
echo "   PID:     $NEW_PID"
echo "   Commit:  $COMMIT"
echo "   Binary:  $BIN_PATH"
echo "   Logs:    tail -100f .logs/daemon.log"
echo "   💡 Stop: make stop"
