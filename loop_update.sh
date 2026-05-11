#!/bin/bash

# 每10s发送一次HTTP请求，value从1累加到2000后重置
URL="http://localhost:8999/api/points/16386"
value=1

while true; do
  echo "[$(date '+%H:%M:%S')] value=${value}"
  curl -s -X PUT "$URL" \
    -H 'Content-Type: application/json' \
    -d "{\"value\": ${value}}"
  echo ""

  value=$((value + 1))
  [ "$value" -gt 2000 ] && value=1

  sleep 10
done
