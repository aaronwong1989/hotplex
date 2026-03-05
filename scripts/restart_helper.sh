#!/bin/bash
BIN_PATH=$1
LOG_FILE=$2

if [ -z "$BIN_PATH" ] || [ -z "$LOG_FILE" ]; then
    echo "Usage: ./scripts/restart_helper.sh <binary_path> <log_file>"
    exit 1
fi

OLD_PID=$(pgrep -f $(basename $BIN_PATH) | head -1 || echo "")
if [ -n "$OLD_PID" ]; then
    echo "🛑 Stopping old daemon (PID: $OLD_PID)..."
    pkill -f $(basename $BIN_PATH) || true
    echo "Waiting for process to exit and release ports..."
    for i in {1..5}; do
        if ! pgrep -f $(basename $BIN_PATH) > /dev/null; then
            echo "✅ Old daemon stopped."
            break
        fi
        sleep 1
    done
    if pgrep -f $(basename $BIN_PATH) > /dev/null; then
        echo "⚠️  Force killing old daemon..."
        pkill -9 -f $(basename $BIN_PATH) || true
    fi
else
    echo "ℹ️  No running daemon found."
fi

mkdir -p $(dirname $LOG_FILE)
truncate -s 0 $LOG_FILE

echo "🔥 Starting NEW HotPlex Daemon..."
nohup $BIN_PATH > $LOG_FILE 2>&1 & disown

sleep 2

NEW_PID=$(pgrep -f $(basename $BIN_PATH) | head -1 || echo "")
if [ -z "$NEW_PID" ] || [ "$NEW_PID" = "$OLD_PID" ]; then
    echo "❌ Restart FAILED. Check $LOG_FILE for errors (e.g., Port already in use)."
    exit 1
fi

COMMIT=$($BIN_PATH --version 2>/dev/null | grep Commit || echo "unknown")
echo "✅ Successfully restarted!"
echo "   PID:     $NEW_PID"
echo "   Commit:  $COMMIT"
echo "   Binary:  $BIN_PATH"
echo "   Logs:    tail -f $LOG_FILE"
echo "   💡 Stop: make stop"
