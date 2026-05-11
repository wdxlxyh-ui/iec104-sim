#!/bin/bash
# IEC104 Simulator - 重启脚本
set -e

DIR="$(cd "$(dirname "$0")/.." && pwd)"

echo "Restarting IEC104 Sim..."
"$DIR/scripts/stop.sh"
sleep 1
"$DIR/scripts/start.sh"
echo "Restart completed"
