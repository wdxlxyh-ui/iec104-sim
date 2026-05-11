#!/bin/bash
export PATH="$HOME/go-toolchain/go/bin:$PATH"
SIM_BIN=/tmp/iec104-sim-inttest
PID_FILE=/tmp/iec104-sim.pid
PASS=0
FAIL=0

cleanup() {
    if [ -f "$PID_FILE" ]; then
        kill $(cat "$PID_FILE") 2>/dev/null
        rm -f "$PID_FILE"
    fi
}
trap cleanup EXIT

assert_eq() {
    local desc="$1" expected="$2" actual="$3"
    if [ "$expected" = "$actual" ]; then
        echo "  PASS: $desc"
        PASS=$((PASS+1))
    else
        echo "  FAIL: $desc (expected: $expected, got: $actual)"
        FAIL=$((FAIL+1))
    fi
}

assert_contains() {
    local desc="$1" haystack="$2" needle="$3"
    if echo "$haystack" | grep -q "$needle"; then
        echo "  PASS: $desc"
        PASS=$((PASS+1))
    else
        echo "  FAIL: $desc (expected to contain: $needle)"
        FAIL=$((FAIL+1))
    fi
}

echo "============================================"
echo " Integration Test: HTTP API End-to-End"
echo "============================================"
echo "Go: $(which go) $(go version 2>&1)"

# Build
echo "Building..."
cd ~/iec104-sim
go build -o "$SIM_BIN" . 2>&1
if [ ! -f "$SIM_BIN" ]; then
    echo "BUILD FAILED"
    exit 1
fi
echo "Build OK: $(ls -la $SIM_BIN | awk '{print $5}') bytes"

# Start simulator
echo "Starting simulator..."
$SIM_BIN -p 2404 -c /mnt/d/AI/Claw/iec104-sim/samples/point.xlsx -H :9090 -l error &
SIM_PID=$!
echo $SIM_PID > "$PID_FILE"
sleep 2

# Verify it's running
if ! kill -0 $SIM_PID 2>/dev/null; then
    echo "SIMULATOR FAILED TO START"
    exit 1
fi
echo "Simulator PID: $SIM_PID"

BASE="http://localhost:9090"

echo ""
echo "--- Test 1: GET /api/status ---"
STATUS=$(curl -s --max-time 3 $BASE/api/status)
assert_contains "status has total_points" "$STATUS" "total_points"
assert_contains "status has version" "$STATUS" "1.0.0"

echo ""
echo "--- Test 2: GET /api/points (list all) ---"
POINTS=$(curl -s --max-time 3 $BASE/api/points)
assert_contains "list returns points" "$POINTS" "points"
POINT_COUNT=$(echo "$POINTS" | python3 -c "import sys,json; print(len(json.load(sys.stdin)['points']))")
assert_eq "point count = 7" "7" "$POINT_COUNT"

echo ""
echo "--- Test 3: GET /api/points/5 (DI_01, IOA=5) ---"
PT=$(curl -s --max-time 3 $BASE/api/points/5)
assert_contains "DI_01 exists" "$PT" "DI_01"
assert_contains "DI_01 type=DI" "$PT" '"point_type":"DI"'

echo ""
echo "--- Test 4: GET /api/points/9999 (not found) ---"
NF=$(curl -s -o /dev/null -w "%{http_code}" --max-time 3 $BASE/api/points/9999)
assert_eq "not found returns 404" "404" "$NF"

echo ""
echo "--- Test 5: PUT /api/points/16385 (AI_01) update value ---"
R1=$(curl -s -X PUT --max-time 3 $BASE/api/points/16385 -H 'Content-Type: application/json' -d '{"value": 123.45}')
assert_contains "AI update success" "$R1" "true"
PT1=$(curl -s --max-time 3 $BASE/api/points/16385)
assert_contains "AI value updated" "$PT1" "123.45"

echo ""
echo "--- Test 6: PUT /api/points/5 (DI_01) update bool ---"
R2=$(curl -s -X PUT --max-time 3 $BASE/api/points/5 -H 'Content-Type: application/json' -d '{"bool_value": true}')
assert_contains "DI update success" "$R2" "true"
PT2=$(curl -s --max-time 3 $BASE/api/points/5)
assert_contains "DI bool_value=true" "$PT2" "true"

echo ""
echo "--- Test 7: POST /api/points (batch update) ---"
R3=$(curl -s -X POST --max-time 3 $BASE/api/points -H 'Content-Type: application/json' \
  -d '{"points":[{"ioa":16385,"value":999.99},{"ioa":5,"bool_value":false}]}')
assert_contains "batch updated=2" "$R3" '"updated":2'
PT3=$(curl -s --max-time 3 $BASE/api/points/16385)
assert_contains "batch AI=999.99" "$PT3" "999.99"

echo ""
echo "--- Test 8: PUT /api/points/16385/qds (quality descriptor) ---"
R4=$(curl -s -X PUT --max-time 3 $BASE/api/points/16385/qds -H 'Content-Type: application/json' \
  -d '{"invalid":true,"blocked":true}')
assert_contains "QDS update success" "$R4" "true"
PT4=$(curl -s --max-time 3 $BASE/api/points/16385)
assert_contains "QDS invalid=true" "$PT4" '"invalid":true'
assert_contains "QDS blocked=true" "$PT4" '"blocked":true'

echo ""
echo "--- Test 9: GET /api/status (verify counters) ---"
S2=$(curl -s --max-time 3 $BASE/api/status)
assert_contains "status has client_connected" "$S2" "client_connected"
assert_contains "status has point_counts" "$S2" "point_counts"

echo ""
echo "--- Test 10: PUT bool on AI point (type safety) ---"
R5=$(curl -s -X PUT --max-time 3 $BASE/api/points/16385 -H 'Content-Type: application/json' -d '{"bool_value":true}')
PT5=$(curl -s --max-time 3 $BASE/api/points/16385)
assert_contains "AI unchanged after bool set" "$PT5" "999.99"

echo ""
echo "============================================"
echo " Integration Test Results"
echo " Passed: $PASS"
echo " Failed: $FAIL"
echo "============================================"

echo "{\"pass\":$PASS,\"fail\":$FAIL}" > /tmp/inttest_results.json

cleanup
exit $FAIL
