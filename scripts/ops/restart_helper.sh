#!/bin/bash
BIN_PATH=$1
LOG_FILE=$2
START_TIME=$SECONDS

if [ -z "$BIN_PATH" ] || [ -z "$LOG_FILE" ]; then
    echo "Usage: $0 <binary_path> <log_file>"
    exit 1
fi

BIN_NAME=$(basename "$BIN_PATH")

# --- Stop old daemon via PGID (aligned with Makefile `stop` target) ---
OLD_PID=$(pgrep -f "$BIN_NAME" | head -1 || echo "")
if [ -n "$OLD_PID" ]; then
    echo "🛑 Stopping old daemon (PID: $OLD_PID)..."
    PGID=$(ps -o pgid= -p "$OLD_PID" | tr -d ' ')
    if [ -n "$PGID" ] && [ "$PGID" != "1" ]; then
        kill -- -"$PGID" 2>/dev/null || kill "$OLD_PID" 2>/dev/null
    else
        kill "$OLD_PID" 2>/dev/null
    fi

    # Wait for process to exit (poll every 0.2s, max 3s)
    count=0
    while [ $count -lt 15 ]; do
        if ! pgrep -f "$BIN_NAME" > /dev/null 2>&1; then
            echo "✅ Old daemon stopped."
            break
        fi
        sleep 0.2
        count=$((count+1))
    done

    if pgrep -f "$BIN_NAME" > /dev/null 2>&1; then
        echo "⚠️  Force killing old daemon..."
        kill -9 -- -"$PGID" 2>/dev/null || kill -9 "$OLD_PID" 2>/dev/null || true
        sleep 0.3
    fi
else
    echo "ℹ️  No running daemon found."
fi

# --- Prepare log ---
mkdir -p "$(dirname "$LOG_FILE")"
> "$LOG_FILE"

# --- Load environment ---
ENV_FILE=""
if [ -f ".env" ]; then
    ENV_FILE=".env"
elif [ -f "$HOME/.hotplex/.env" ]; then
    ENV_FILE="$HOME/.hotplex/.env"
fi

if [ -n "$ENV_FILE" ]; then
    echo "📋 Loading environment from: $ENV_FILE"

    # Export env vars (skip comments and empty lines)
    set -a
    while IFS= read -r line || [ -n "$line" ]; do
        [[ "$line" =~ ^[[:space:]]*# ]] && continue
        [[ -z "$line" ]] && continue
        [[ "$line" == *"="* ]] && export "$line"
    done < "$ENV_FILE"
    set +a

    if [ -n "$HOTPLEX_OPEN_CODE_PASSWORD" ]; then
        echo "🔑 Password found in env file"
    else
        echo "⚠️  Warning: HOTPLEX_OPEN_CODE_PASSWORD not found in $ENV_FILE"
    fi
fi

# --- Start new daemon ---
export HOTPLEX_CHATAPPS_CONFIG_DIR="$HOME/.hotplex/configs"

echo "🔥 Starting NEW HotPlex Daemon..."
nohup "$BIN_PATH" start >> "$LOG_FILE" 2>&1 &
DAEMON_PID=$!
disown

echo "🚀 Daemon started with PID: $DAEMON_PID" >> "$LOG_FILE"

# --- Verify startup (poll every 0.5s, max 3s) ---
VERIFIED=false
count=0
while [ $count -lt 6 ]; do
    NEW_PID=$(pgrep -f "$BIN_NAME" | head -1 || echo "")
    if [ -n "$NEW_PID" ] && [ "$NEW_PID" != "$OLD_PID" ]; then
        VERIFIED=true
        break
    fi
    sleep 0.5
    count=$((count+1))
done

ELAPSED=$(( SECONDS - START_TIME ))

if [ "$VERIFIED" = true ]; then
    COMMIT=$("$BIN_PATH" --version 2>/dev/null | grep Commit || echo "unknown")
    echo "✅ Successfully restarted! (${ELAPSED}s)"
    echo "   PID:     $NEW_PID"
    echo "   Commit:  $COMMIT"
    echo "   Binary:  $BIN_PATH"
    echo "   Logs:    tail -100f .logs/daemon.log"
    echo "   💡 Stop: make stop"
else
    echo "❌ Restart FAILED (${ELAPSED}s). Check $LOG_FILE for errors."
    exit 1
fi
