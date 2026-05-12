#!/bin/bash
# IEC104 Simulator - 启动脚本
set -e

DIR="$(cd "$(dirname "$0")/.." && pwd)"
PID_FILE="$DIR/logs/pid"
LOG_DIR="$DIR/logs"
CONFIG_DIR="$DIR/config"

# Ensure directories exist
mkdir -p "$LOG_DIR" "$CONFIG_DIR"

# Check if already running
if [ -f "$PID_FILE" ]; then
    PID=$(cat "$PID_FILE")
    if kill -0 "$PID" 2>/dev/null; then
        echo "IEC104 Sim is already running (PID: $PID)"
        exit 1
    else
        rm -f "$PID_FILE"
    fi
fi

# Switch to package root so web/dist resolves correctly
cd "$DIR"

# Start the simulator
nohup "$DIR/bin/iec104-sim" serve \
    --http ":8080" \
    --config-dir "$CONFIG_DIR" \
    --log-dir "$LOG_DIR" \
    --log info \
    >> "$LOG_DIR/output.log" 2>&1 &

SIM_PID=$!
echo $SIM_PID > "$PID_FILE"
echo "IEC104 Sim started (PID: $SIM_PID)"
echo "Web UI: http://localhost:8080"