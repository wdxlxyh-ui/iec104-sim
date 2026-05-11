#!/bin/bash
export PATH="$HOME/go-toolchain/go/bin:$PATH"
cd ~/iec104-sim
TIMESTAMP=$(date '+%Y-%m-%d %H:%M:%S')
echo "==================================="
echo " IEC104 Simulator - Full Test Suite"
echo " Timestamp: $TIMESTAMP"
echo "==================================="

echo ""
echo "--- Step 1: go vet ---"
go vet ./... 2>&1
echo "Exit: $?"

echo ""
echo "--- Step 2: Unit Tests (verbose) ---"
go test -v -count=1 -timeout 30s ./... 2>&1
echo "Unit Tests Exit: $?"

echo ""
echo "--- Step 3: Race Detection ---"
go test -race -count=1 -timeout 30s ./pkg/... ./cmd/... 2>&1
echo "Race Test Exit: $?"

echo ""
echo "--- Step 4: Build Check ---"
go build -o /tmp/iec104-sim-test ./cmd/iec104-sim/ 2>&1
echo "Build Exit: $?"
file /tmp/iec104-sim-test
rm /tmp/iec104-sim-test
