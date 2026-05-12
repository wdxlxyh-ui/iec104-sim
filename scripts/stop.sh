#!/bin/bash
# IEC104 Simulator - 停止脚本
set -e

DIR="$(cd "$(dirname "$0")/.." && pwd)"
PID_FILE="$DIR/logs/pid"

if [ ! -f "$PID_FILE" ]; then
    echo "PID file not found. Is the simulator running?"
    exit 0
fi

PID=$(cat "$PID_FILE")
if kill -0 "$PID" 2>/dev/null; then
    echo "Stopping IEC104 Sim (PID: $PID)..."
    kill "$PID"
    # Wait for graceful shutdown
    for i in $(seq 1 10); do
        if ! kill -0 "$PID" 2>/dev/null; then
            break
        fi
        sleep 1
    done
    # Force kill if still running
    if kill -0 "$PID" 2>/dev/null; then
        echo "Force stopping..."
        kill -9 "$PID" 2>/dev/null || true
    fi
    echo "IEC104 Sim stopped"
else
    echo "Process $PID not found"
fi

rm -f "$PID_FILE"
