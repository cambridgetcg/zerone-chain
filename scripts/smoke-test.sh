#!/bin/bash
# Minimal smoke test: init, start, wait for block 5, stop.
# Proves the zeroned binary can produce blocks.
set -euo pipefail

BINARY="./build/zeroned"
HOME_DIR=$(mktemp -d)
CHAIN_ID="zerone-smoke"
PID=""

cleanup() {
    if [ -n "$PID" ]; then
        kill "$PID" 2>/dev/null || true
        wait "$PID" 2>/dev/null || true
    fi
    rm -rf "$HOME_DIR"
}
trap cleanup EXIT

echo "=== Zerone Smoke Test ==="
echo "Binary: $BINARY"
echo "Home:   $HOME_DIR"
echo ""

# Check binary exists
if [ ! -x "$BINARY" ]; then
    echo "Binary not found at $BINARY"
    echo "Build with: go build -o build/zeroned ./cmd/zeroned"
    exit 1
fi

# Init
echo "Initializing chain..."
$BINARY init smoke-node --chain-id "$CHAIN_ID" --home "$HOME_DIR" --default-denom uzrn 2>/dev/null
$BINARY keys add val --keyring-backend test --home "$HOME_DIR" 2>/dev/null
ADDR=$($BINARY keys show val -a --keyring-backend test --home "$HOME_DIR")
$BINARY add-genesis-account "$ADDR" 1000000000000uzrn --home "$HOME_DIR"
$BINARY gentx val 100000000000uzrn \
    --chain-id "$CHAIN_ID" \
    --keyring-backend test \
    --home "$HOME_DIR" 2>/dev/null
$BINARY collect-gentxs --home "$HOME_DIR" 2>/dev/null

# Start node in background
echo "Starting node..."
$BINARY start --home "$HOME_DIR" --minimum-gas-prices 0uzrn --log_level error &
PID=$!

# Wait for block production
echo "Waiting for block production..."
for i in $(seq 1 30); do
    if ! kill -0 "$PID" 2>/dev/null; then
        echo "Node process died unexpectedly"
        exit 1
    fi
    HEIGHT=$($BINARY status --home "$HOME_DIR" 2>/dev/null | jq -r '.sync_info.latest_block_height // "0"' 2>/dev/null || echo "0")
    if [ "$HEIGHT" -ge 5 ] 2>/dev/null; then
        echo "Block $HEIGHT reached. Smoke test passed!"
        exit 0
    fi
    sleep 1
done

echo "Failed to reach block 5 within 30 seconds"
exit 1
