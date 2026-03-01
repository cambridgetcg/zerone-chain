#!/usr/bin/env bash
# ═══════════════════════════════════════════════════════════════════════════
# Zerone — Node Configuration
# ═══════════════════════════════════════════════════════════════════════════
#
# Applies recommended production settings to config.toml and app.toml
# based on the node's intended role.
#
# Usage:
#   scripts/configure-node.sh [OPTIONS]
#
# Options:
#   --home <dir>           Node home directory (default: $HOME/.zeroned)
#   --mode <mode>          Preset: validator|fullnode|seed|archive (default: validator)
#   --gas-prices <prices>  Minimum gas prices (default: 0.025uzrn)
#   --enable-api           Enable REST API on port 1317
#   --enable-grpc          Enable gRPC on port 9090
#   --prometheus           Enable Prometheus metrics on port 26660
#   --external-address     External P2P address (ip:port)
#   --moniker <name>       Set node moniker
#   --help                 Show this help message
#
# Supports: Ubuntu 22.04+, macOS 13+
# ═══════════════════════════════════════════════════════════════════════════

set -euo pipefail

# ── Defaults ─────────────────────────────────────────────────────────────

ZERONED_HOME="${HOME}/.zeroned"
MODE="validator"
GAS_PRICES="0.025uzrn"
ENABLE_API=""
ENABLE_GRPC=""
ENABLE_PROMETHEUS=""
EXTERNAL_ADDRESS=""
MONIKER=""

# ── Helpers ──────────────────────────────────────────────────────────────

die()  { echo -e "\033[1;31mERROR:\033[0m $*" >&2; exit 1; }
info() { echo -e "\033[1;34m  ->\033[0m $*"; }
ok()   { echo -e "\033[1;32m  ✓\033[0m $*"; }
warn() { echo -e "\033[1;33m  !!\033[0m $*"; }

usage() {
  head -24 "$0" | tail -17
  exit 0
}

# Portable in-place sed: macOS requires -i '' while GNU sed uses -i
sedi() {
  if [[ "$(uname -s)" == "Darwin" ]]; then
    sed -i '' "$@"
  else
    sed -i "$@"
  fi
}

# ── Parse arguments ─────────────────────────────────────────────────────

while [[ $# -gt 0 ]]; do
  case "$1" in
    --home)             ZERONED_HOME="$2"; shift 2 ;;
    --mode)             MODE="$2"; shift 2 ;;
    --gas-prices)       GAS_PRICES="$2"; shift 2 ;;
    --enable-api)       ENABLE_API="true"; shift ;;
    --enable-grpc)      ENABLE_GRPC="true"; shift ;;
    --prometheus)       ENABLE_PROMETHEUS="true"; shift ;;
    --external-address) EXTERNAL_ADDRESS="$2"; shift 2 ;;
    --moniker)          MONIKER="$2"; shift 2 ;;
    --help|-h)          usage ;;
    *) die "Unknown option: $1. Run with --help for usage." ;;
  esac
done

# ── Validate mode ───────────────────────────────────────────────────────

case "${MODE}" in
  validator|fullnode|seed|archive) ;;
  *) die "Unknown mode: ${MODE}. Valid modes: validator, fullnode, seed, archive" ;;
esac

# ── Check config files exist ─────────────────────────────────────────────

CONFIG_DIR="${ZERONED_HOME}/config"
CONFIG_TOML="${CONFIG_DIR}/config.toml"
APP_TOML="${CONFIG_DIR}/app.toml"

[[ -f "${CONFIG_TOML}" ]] || die "config.toml not found at ${CONFIG_TOML}. Initialize node first: zeroned init <moniker> --chain-id <chain-id>"
[[ -f "${APP_TOML}" ]]    || die "app.toml not found at ${APP_TOML}. Initialize node first: zeroned init <moniker> --chain-id <chain-id>"

# ── Create backups ──────────────────────────────────────────────────────

cp "${CONFIG_TOML}" "${CONFIG_TOML}.backup"
cp "${APP_TOML}" "${APP_TOML}.backup"

# ── Apply mode defaults ────────────────────────────────────────────────
#
# Mode presets set defaults for API, gRPC, Prometheus, pruning, etc.
# Explicit flags (--enable-api, --enable-grpc, --prometheus) override
# the mode defaults.

PRUNING="default"
API_ENABLED="false"
GRPC_ENABLED="false"
PROMETHEUS_ENABLED="true"
SEED_MODE="false"
TX_INDEXER="kv"
CORS_ENABLED="false"
SNAPSHOT_INTERVAL="0"

case "${MODE}" in
  validator)
    PRUNING="default"
    API_ENABLED="false"
    GRPC_ENABLED="false"
    PROMETHEUS_ENABLED="true"
    ;;
  fullnode)
    PRUNING="default"
    API_ENABLED="true"
    GRPC_ENABLED="true"
    PROMETHEUS_ENABLED="true"
    CORS_ENABLED="true"
    ;;
  seed)
    PRUNING="everything"
    API_ENABLED="false"
    GRPC_ENABLED="false"
    PROMETHEUS_ENABLED="true"
    SEED_MODE="true"
    TX_INDEXER="null"
    ;;
  archive)
    PRUNING="nothing"
    API_ENABLED="true"
    GRPC_ENABLED="true"
    PROMETHEUS_ENABLED="true"
    SNAPSHOT_INTERVAL="1000"
    ;;
esac

# Override with explicit flags
[[ -n "${ENABLE_API}" ]]        && API_ENABLED="true"
[[ -n "${ENABLE_GRPC}" ]]       && GRPC_ENABLED="true"
[[ -n "${ENABLE_PROMETHEUS}" ]] && PROMETHEUS_ENABLED="true"

# ── Apply config.toml patches ──────────────────────────────────────────

# Moniker
if [[ -n "${MONIKER}" ]]; then
  sedi "s/^moniker = .*/moniker = \"${MONIKER}\"/" "${CONFIG_TOML}"
fi

# Consensus timeouts — Zerone targets 2521ms blocks
sedi "s/^timeout_commit = .*/timeout_commit = \"2521ms\"/" "${CONFIG_TOML}"
sedi "s/^timeout_propose = .*/timeout_propose = \"2000ms\"/" "${CONFIG_TOML}"

# Empty blocks (keep chain alive for PoT rounds)
sedi "s/^create_empty_blocks = .*/create_empty_blocks = true/" "${CONFIG_TOML}"
sedi "s/^create_empty_blocks_interval = .*/create_empty_blocks_interval = \"0s\"/" "${CONFIG_TOML}"

# Seed mode
sedi "s/^seed_mode = .*/seed_mode = ${SEED_MODE}/" "${CONFIG_TOML}"

# Prometheus instrumentation
sedi "s/^prometheus = .*/prometheus = ${PROMETHEUS_ENABLED}/" "${CONFIG_TOML}"

# TX indexer
sedi "s/^indexer = .*/indexer = \"${TX_INDEXER}\"/" "${CONFIG_TOML}"

# External address
if [[ -n "${EXTERNAL_ADDRESS}" ]]; then
  sedi "s/^external_address = .*/external_address = \"${EXTERNAL_ADDRESS}\"/" "${CONFIG_TOML}"
fi

# Seed mode: relax peer settings
if [[ "${SEED_MODE}" == "true" ]]; then
  sedi "s/^addr_book_strict = .*/addr_book_strict = false/" "${CONFIG_TOML}"
  sedi "s/^pex = .*/pex = true/" "${CONFIG_TOML}"
  sedi "s/^max_num_inbound_peers = .*/max_num_inbound_peers = 100/" "${CONFIG_TOML}"
  sedi "s/^max_num_outbound_peers = .*/max_num_outbound_peers = 40/" "${CONFIG_TOML}"
fi

# CORS for fullnode mode
if [[ "${CORS_ENABLED}" == "true" ]]; then
  sedi 's/^cors_allowed_origins = .*/cors_allowed_origins = ["*"]/' "${CONFIG_TOML}"
fi

# ── Apply app.toml patches ─────────────────────────────────────────────

# Minimum gas prices
sedi "s/^minimum-gas-prices = .*/minimum-gas-prices = \"${GAS_PRICES}\"/" "${APP_TOML}"

# Pruning
sedi "s/^pruning = .*/pruning = \"${PRUNING}\"/" "${APP_TOML}"

# API server
sedi "/^\[api\]/,/^\[/{s/^enable = .*/enable = ${API_ENABLED}/}" "${APP_TOML}"

# CORS for fullnode mode
if [[ "${CORS_ENABLED}" == "true" ]]; then
  sedi "s/^enabled-unsafe-cors = .*/enabled-unsafe-cors = true/" "${APP_TOML}"
fi

# gRPC server
sedi "/^\[grpc\]/,/^\[/{s/^enable = .*/enable = ${GRPC_ENABLED}/}" "${APP_TOML}"

# Telemetry — enable Prometheus retention
if [[ "${PROMETHEUS_ENABLED}" == "true" ]]; then
  sedi "s/^prometheus-retention-time = .*/prometheus-retention-time = 60/" "${APP_TOML}"
fi

# Snapshot interval for archive mode (state sync serving)
if [[ "${SNAPSHOT_INTERVAL}" != "0" ]]; then
  sedi "s/^snapshot-interval = .*/snapshot-interval = ${SNAPSHOT_INTERVAL}/" "${APP_TOML}"
fi

# Mempool — SDK v0.50 defaults max-txs=-1 (NoOpMempool), which drops all txs
sedi "s/^max-txs = -1/max-txs = 5000/" "${APP_TOML}"

# IAVL — disable fast nodes to prevent "version does not exist" query errors
sedi "s/^iavl-disable-fastnode = false/iavl-disable-fastnode = true/" "${APP_TOML}"

# ── Moniker warning ─────────────────────────────────────────────────────

CURRENT_MONIKER="${MONIKER}"
if [[ -z "${CURRENT_MONIKER}" ]]; then
  CURRENT_MONIKER=$(grep '^moniker = ' "${CONFIG_TOML}" | sed 's/moniker = "\(.*\)"/\1/')
fi

if [[ "${CURRENT_MONIKER}" == "my-zerone-node" ]] || [[ "${CURRENT_MONIKER}" == "my-node" ]]; then
  warn "Moniker is still set to the default (\"${CURRENT_MONIKER}\")."
  warn "Set a unique moniker with: --moniker <your-node-name>"
fi

# ── Determine display values ────────────────────────────────────────────

api_display="disabled"
[[ "${API_ENABLED}" == "true" ]] && api_display="enabled (port 1317)"

grpc_display="disabled"
[[ "${GRPC_ENABLED}" == "true" ]] && grpc_display="enabled (port 9090)"

prometheus_display="disabled"
[[ "${PROMETHEUS_ENABLED}" == "true" ]] && prometheus_display="enabled (port 26660)"

moniker_display="${CURRENT_MONIKER:-$(grep '^moniker = ' "${CONFIG_TOML}" | sed 's/moniker = "\(.*\)"/\1/')}"

# ── Print summary ──────────────────────────────────────────────────────

echo ""
echo "═══ Zerone Node Configuration ═══"
echo ""
echo "  Mode:      ${MODE}"
echo "  Home:      ${ZERONED_HOME}"
echo "  Moniker:   ${moniker_display}"
echo ""
echo "  Changes Applied:"
ok "Pruning:         ${PRUNING}"
ok "API:             ${api_display}"
ok "gRPC:            ${grpc_display}"
ok "Prometheus:      ${prometheus_display}"
ok "Min gas prices:  ${GAS_PRICES}"
ok "Block time:      2521ms commit, 2000ms propose"
if [[ "${SEED_MODE}" == "true" ]]; then
  ok "Seed mode:       enabled"
  ok "TX indexer:      null"
fi
if [[ "${SNAPSHOT_INTERVAL}" != "0" ]]; then
  ok "Snapshots:       every ${SNAPSHOT_INTERVAL} blocks"
fi
if [[ -n "${EXTERNAL_ADDRESS}" ]]; then
  ok "External addr:   ${EXTERNAL_ADDRESS}"
fi
echo ""
echo "  Backups:"
echo "    config.toml → config.toml.backup"
echo "    app.toml    → app.toml.backup"
echo ""
echo "  Start with:"
echo "    zeroned start --home ${ZERONED_HOME}"
echo ""
