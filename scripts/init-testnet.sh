#!/bin/bash
# Quick single-validator testnet initialization for Zerone.
# Usage: bash scripts/init-testnet.sh [binary] [chain-id]
set -euo pipefail

BINARY=${1:-"./build/zeroned"}
CHAIN_ID=${2:-"zerone-testnet-1"}
HOME_DIR=$(mktemp -d)

echo "=== Zerone Testnet Init ==="
echo "Binary:   $BINARY"
echo "Chain ID: $CHAIN_ID"
echo "Home:     $HOME_DIR"
echo ""

# Initialize node
$BINARY init validator-1 --chain-id "$CHAIN_ID" --home "$HOME_DIR" --default-denom uzrn 2>/dev/null

# Create validator key
$BINARY keys add validator --keyring-backend test --home "$HOME_DIR" 2>/dev/null
ADDR=$($BINARY keys show validator -a --keyring-backend test --home "$HOME_DIR")

# Fund validator account (1,000,000 ZRN = 1_000_000_000_000 uzrn)
$BINARY add-genesis-account "$ADDR" 1000000000000uzrn --home "$HOME_DIR"

# Create gentx (100,000 ZRN self-delegation)
$BINARY gentx validator 100000000000uzrn \
    --chain-id "$CHAIN_ID" \
    --keyring-backend test \
    --home "$HOME_DIR" 2>/dev/null

# Collect gentxs
$BINARY collect-gentxs --home "$HOME_DIR" 2>/dev/null

echo ""
echo "Genesis initialized at $HOME_DIR"
echo "Validator address: $ADDR"
echo ""
echo "Start with:"
echo "  $BINARY start --home $HOME_DIR --minimum-gas-prices 0uzrn"
