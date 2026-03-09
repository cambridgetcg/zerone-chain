#!/bin/bash
# ═══════════════════════════════════════════════════════════════════════════
# Zerone — Devnet Initialization Script
# ═══════════════════════════════════════════════════════════════════════════
#
# Initializes a multi-validator devnet with agent wallets, seeded domains,
# and fast epoch parameters for development and integration testing.
#
# Usage:
#   scripts/devnet-init.sh [options]
#
# Options:
#   --chain-id ID          Chain ID (default: zerone-devnet-1)
#   --validators N         Number of validators (default: 3)
#   --denom DENOM          Token denomination (default: uzrn)
#   --base-dir DIR         State directory (default: ~/.zeroned/devnet)
#   --binary PATH          Path to zeroned binary (default: ./build/zeroned)
#   --build                Build binary before init
#   -h, --help             Show this help message
#
# Outputs:
#   - genesis.json at <base-dir>/coordinator/config/genesis.json
#   - persistent_peers to stdout
#   - Per-validator config in <base-dir>/val{0..N-1}/
#
# Requires: go (1.24+ if --build), jq >= 1.6
# ═══════════════════════════════════════════════════════════════════════════

set -euo pipefail

# ── Defaults ─────────────────────────────────────────────────────────────

CHAIN_ID="zerone-devnet-1"
NUM_VALIDATORS=3
DENOM="uzrn"
BASE_DIR="${HOME}/.zeroned/devnet"
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BINARY="${PROJECT_ROOT}/build/zeroned"
KEYRING="test"
DO_BUILD=false

# Validator economics
VALIDATOR_BALANCE="500000000"    # 500M uzrn = 500 ZRN
VALIDATOR_STAKE="100000000"      # 100M uzrn = 100 ZRN
AGENT_BALANCE="100000000"        # 100M uzrn = 100 ZRN each

# Port bases (same scheme as localnet)
BASE_P2P_PORT=26600

# Agent fleet
AGENT_NAMES=("SAGE" "MUSE" "SENTINEL" "SPROUT" "HERALD")

# 9 devnet knowledge domains
DEVNET_DOMAINS=("math" "science" "literature" "philosophy" "engineering" "medicine" "law" "art" "economics")

# ── Usage ────────────────────────────────────────────────────────────────

usage() {
  cat <<EOF
Usage: $(basename "$0") [options]

Initializes a Zerone devnet with validators, agent wallets, seeded domains,
and fast epoch parameters.

Options:
  --chain-id ID          Chain ID (default: ${CHAIN_ID})
  --validators N         Number of validators (default: ${NUM_VALIDATORS})
  --denom DENOM          Token denomination (default: ${DENOM})
  --base-dir DIR         State directory (default: ${BASE_DIR})
  --binary PATH          Path to zeroned binary (default: ./build/zeroned)
  --build                Build binary before init
  -h, --help             Show this help message

Validator economics:
  Each validator receives 500M ${DENOM} and self-delegates 100M ${DENOM}.
  5 agent wallets (SAGE, MUSE, SENTINEL, SPROUT, HERALD) each get 100M ${DENOM}.

Seeded domains:
  ${DEVNET_DOMAINS[*]}

Port layout (per validator):
  val0: RPC 26601, gRPC 9090, API 1317
  val1: RPC 26611, gRPC 9091, API 1318
  val2: RPC 26621, gRPC 9092, API 1319
  ...

EOF
  exit 0
}

# ── Parse arguments ──────────────────────────────────────────────────────

while [[ $# -gt 0 ]]; do
  case "$1" in
    --chain-id)    CHAIN_ID="$2"; shift 2 ;;
    --validators)  NUM_VALIDATORS="$2"; shift 2 ;;
    --denom)       DENOM="$2"; shift 2 ;;
    --base-dir)    BASE_DIR="$2"; shift 2 ;;
    --binary)      BINARY="$2"; shift 2 ;;
    --build)       DO_BUILD=true; shift ;;
    -h|--help)     usage ;;
    *)             echo "Unknown option: $1" >&2; usage ;;
  esac
done

COORDINATOR_HOME="${BASE_DIR}/coordinator"

# ── Helpers ──────────────────────────────────────────────────────────────

die()  { echo -e "\033[1;31mERROR:\033[0m $*" >&2; exit 1; }
info() { echo -e "\033[1;34m  ->\033[0m $*"; }
ok()   { echo -e "\033[1;32m  OK\033[0m $*"; }
warn() { echo -e "\033[1;33m  !!\033[0m $*"; }

check_deps() {
  command -v jq >/dev/null 2>&1 || die "jq >= 1.6 required. Install: brew install jq"
  if [ "${DO_BUILD}" = true ]; then
    command -v go >/dev/null 2>&1 || die "go >= 1.24 required (needed for --build)."
  fi
}

# Atomic jq patch on coordinator genesis
patch_genesis() {
  local genesis="${COORDINATOR_HOME}/config/genesis.json"
  jq "$1" "$genesis" > "${genesis}.tmp" && mv "${genesis}.tmp" "$genesis"
}

# Port helpers (same layout as localnet)
p2p_port()  { echo $(( BASE_P2P_PORT + $1 * 10 )); }
rpc_port()  { echo $(( BASE_P2P_PORT + $1 * 10 + 1 )); }
grpc_port() { echo $(( 9090 + $1 )); }
api_port()  { echo $(( 1317 + $1 )); }

# ── Main ─────────────────────────────────────────────────────────────────

echo "═══════════════════════════════════════════════════════════════"
echo "  Zerone — Devnet Init"
echo "  Chain: ${CHAIN_ID} | Validators: ${NUM_VALIDATORS} | Denom: ${DENOM}"
echo "═══════════════════════════════════════════════════════════════"
echo ""

check_deps

# ── Step 1: Build (optional) ────────────────────────────────────────────

if [ "${DO_BUILD}" = true ]; then
  info "Building zeroned binary..."
  mkdir -p "${PROJECT_ROOT}/build"
  (cd "${PROJECT_ROOT}" && go build \
    -ldflags "-X github.com/cosmos/cosmos-sdk/version.Name=zerone -X github.com/cosmos/cosmos-sdk/version.AppName=zeroned" \
    -o build/zeroned ./cmd/zeroned) || die "Build failed"
  ok "Binary built: ${BINARY}"
fi

# Verify binary exists
[ -x "${BINARY}" ] || die "Binary not found or not executable: ${BINARY}. Use --build or 'make build' first."

# ── Step 2: Clean previous state (idempotent) ───────────────────────────

if [ -d "${BASE_DIR}" ]; then
  warn "Removing previous devnet state at ${BASE_DIR}"
  rm -rf "${BASE_DIR}"
fi
mkdir -p "${BASE_DIR}"

# ── Step 3: Init coordinator node ───────────────────────────────────────

info "Initializing coordinator node..."
${BINARY} init coordinator \
  --chain-id "${CHAIN_ID}" \
  --default-denom "${DENOM}" \
  --home "${COORDINATOR_HOME}" 2>/dev/null

# ── Step 4: Patch genesis — consensus params ────────────────────────────

info "Patching genesis parameters..."

# Consensus: vote extensions from block 1, 2s block time target
patch_genesis '
  .consensus.params.block.max_gas = "33333333" |
  .consensus.params.block.max_bytes = "4194304" |
  .consensus.params.abci.vote_extensions_enable_height = "1"
'

# ── Step 5: Patch genesis — SDK module params ───────────────────────────

# Staking + slashing + gov
patch_genesis '
  .app_state.staking.params.bond_denom = "uzrn" |
  .app_state.slashing.params.signed_blocks_window = "100" |
  .app_state.slashing.params.min_signed_per_window = "0.500000000000000000" |
  .app_state.slashing.params.downtime_jail_duration = "60s" |
  .app_state.slashing.params.slash_fraction_downtime = "0.010000000000000000" |
  .app_state.gov.params.voting_period = "60s" |
  .app_state.gov.params.expedited_voting_period = "30s"
'

# ── Step 6: Patch genesis — knowledge module (fast quality rounds) ──────

patch_genesis '
  .app_state.knowledge.params.commit_period_blocks = 4 |
  .app_state.knowledge.params.reveal_period_blocks = 4 |
  .app_state.knowledge.params.min_validators_per_round = 2 |
  .app_state.knowledge.params.max_validators_per_round = 22 |
  .app_state.knowledge.params.adversarial_verification_enabled = false |
  .app_state.knowledge.params.min_claim_text_length = 20 |
  .app_state.knowledge.params.confidence_threshold = 770000 |
  .app_state.knowledge.params.quorum_threshold = 660000 |
  .app_state.knowledge.params.verification_reward = "3000000" |
  .app_state.knowledge.params.energy_decay_rate = 100000
'

# ── Step 7: Seed 9 devnet domains ───────────────────────────────────────

info "Seeding ${#DEVNET_DOMAINS[@]} knowledge domains..."

# Build the domains JSON array
DOMAINS_JSON="["
for idx in "${!DEVNET_DOMAINS[@]}"; do
  domain="${DEVNET_DOMAINS[$idx]}"
  if [ "$idx" -gt 0 ]; then
    DOMAINS_JSON="${DOMAINS_JSON},"
  fi
  DOMAINS_JSON="${DOMAINS_JSON}{\"name\":\"${domain}\",\"status\":1}"
done
DOMAINS_JSON="${DOMAINS_JSON}]"

patch_genesis ".app_state.knowledge.domains = ${DOMAINS_JSON}"

# ── Step 8: Devnet epoch params (1000 blocks, faster decay) ─────────────

info "Setting devnet epoch params (1000 blocks, faster decay)..."

# Alignment module — epoch = 1000 blocks
patch_genesis '
  .app_state.alignment.params.observation_interval_blocks = 1000
'

# Zerone governance — fast params
patch_genesis '
  .app_state.zerone_gov.params.voting_period_blocks = 10 |
  .app_state.zerone_gov.params.discussion_period_blocks = 5
'

# Research module — fast review
patch_genesis '
  .app_state.research.params.review_period_blocks = 20 |
  .app_state.research.params.min_reviewer_count = 2
'

# Vesting rewards — low validator threshold for devnet
patch_genesis '
  .app_state.vesting_rewards.params.min_validators_for_full_reward = 2
'

# Bank denom metadata
patch_genesis '
  .app_state.bank.denom_metadata = [{
    "description": "Zerone - the currency of verified truth",
    "denom_units": [
      {"denom": "uzrn",  "exponent": 0, "aliases": ["microzerone"]},
      {"denom": "mzrn",  "exponent": 3, "aliases": ["millizerone"]},
      {"denom": "zrn",   "exponent": 6, "aliases": ["zerone"]}
    ],
    "base": "uzrn",
    "display": "zrn",
    "name": "Zerone",
    "symbol": "ZRN"
  }]
'

ok "Genesis params patched"

# ── Step 9: Create agent wallets ────────────────────────────────────────

info "Creating ${#AGENT_NAMES[@]} agent wallets..."

for agent_name in "${AGENT_NAMES[@]}"; do
  agent_key=$(echo "${agent_name}" | tr '[:upper:]' '[:lower:]')
  ${BINARY} keys add "${agent_key}" \
    --keyring-backend ${KEYRING} \
    --home "${COORDINATOR_HOME}" 2>/dev/null

  agent_addr=$(${BINARY} keys show "${agent_key}" -a \
    --keyring-backend ${KEYRING} \
    --home "${COORDINATOR_HOME}")

  ${BINARY} add-genesis-account "${agent_addr}" "${AGENT_BALANCE}${DENOM}" \
    --home "${COORDINATOR_HOME}"

  info "  ${agent_name}: ${agent_addr} ($(( AGENT_BALANCE / 1000000 )) ZRN)"
done

ok "Agent wallets created"

# ── Step 10: Setup validators ───────────────────────────────────────────

info "Setting up ${NUM_VALIDATORS} validators..."
mkdir -p "${COORDINATOR_HOME}/config/gentx"

declare -a VALIDATOR_NAMES
for i in $(seq 0 $((NUM_VALIDATORS - 1))); do
  VALIDATOR_NAMES[$i]="val${i}"
done

for i in $(seq 0 $((NUM_VALIDATORS - 1))); do
  val_name="${VALIDATOR_NAMES[$i]}"
  val_home="${BASE_DIR}/${val_name}"

  info "  ${val_name}: balance=$(( VALIDATOR_BALANCE / 1000000 )) ZRN, stake=$(( VALIDATOR_STAKE / 1000000 )) ZRN"

  # Init separate home (unique consensus key)
  ${BINARY} init "${val_name}" \
    --chain-id "${CHAIN_ID}" \
    --home "${val_home}" \
    --overwrite 2>/dev/null

  # Copy coordinator genesis (has params + agent accounts)
  cp "${COORDINATOR_HOME}/config/genesis.json" "${val_home}/config/genesis.json"

  # Create validator account key in coordinator keyring
  ${BINARY} keys add "${val_name}" \
    --keyring-backend ${KEYRING} \
    --home "${COORDINATOR_HOME}" 2>/dev/null

  # Get address
  val_addr=$(${BINARY} keys show "${val_name}" -a \
    --keyring-backend ${KEYRING} \
    --home "${COORDINATOR_HOME}")

  # Fund validator in coordinator genesis
  ${BINARY} add-genesis-account "${val_addr}" "${VALIDATOR_BALANCE}${DENOM}" \
    --home "${COORDINATOR_HOME}"

  # Copy updated genesis + keyring to validator home
  cp "${COORDINATOR_HOME}/config/genesis.json" "${val_home}/config/genesis.json"
  cp -r "${COORDINATOR_HOME}/keyring-test" "${val_home}/"

  # Generate gentx (self-delegation)
  ${BINARY} genesis gentx "${val_name}" "${VALIDATOR_STAKE}${DENOM}" \
    --chain-id "${CHAIN_ID}" \
    --keyring-backend ${KEYRING} \
    --home "${val_home}" \
    --moniker "${val_name}" \
    --commission-rate "0.10" \
    --commission-max-rate "0.20" \
    --commission-max-change-rate "0.01" \
    --output-document "${COORDINATOR_HOME}/config/gentx/gentx-${val_name}.json" 2>/dev/null
done

# ── Step 11: Collect gentxs ─────────────────────────────────────────────

info "Collecting gentxs..."
${BINARY} genesis collect-gentxs --home "${COORDINATOR_HOME}" 2>/dev/null
ok "Gentxs collected"

# ── Step 12: Validate genesis ───────────────────────────────────────────

info "Validating genesis..."
if ${BINARY} genesis validate --home "${COORDINATOR_HOME}" 2>&1; then
  ok "Genesis valid"
else
  die "Genesis validation FAILED"
fi

# ── Step 13: Distribute final genesis ───────────────────────────────────

info "Distributing final genesis to all validators..."
for i in $(seq 0 $((NUM_VALIDATORS - 1))); do
  val_name="${VALIDATOR_NAMES[$i]}"
  val_home="${BASE_DIR}/${val_name}"
  cp "${COORDINATOR_HOME}/config/genesis.json" "${val_home}/config/genesis.json"
done

# ── Step 14: Collect node IDs and configure networking ──────────────────

info "Configuring validator networking..."

# Collect all node IDs first
declare -a NODE_IDS
for i in $(seq 0 $((NUM_VALIDATORS - 1))); do
  val_name="${VALIDATOR_NAMES[$i]}"
  val_home="${BASE_DIR}/${val_name}"
  NODE_IDS[$i]=$(${BINARY} tendermint show-node-id --home "${val_home}" 2>/dev/null || \
                 ${BINARY} comet show-node-id --home "${val_home}" 2>/dev/null || \
                 echo "")
done

# Build full persistent_peers string (all validators)
PERSISTENT_PEERS=""
for i in $(seq 0 $((NUM_VALIDATORS - 1))); do
  if [ -n "${NODE_IDS[$i]}" ]; then
    peer_p2p=$(p2p_port $i)
    if [ -n "${PERSISTENT_PEERS}" ]; then
      PERSISTENT_PEERS="${PERSISTENT_PEERS},"
    fi
    PERSISTENT_PEERS="${PERSISTENT_PEERS}${NODE_IDS[$i]}@127.0.0.1:${peer_p2p}"
  fi
done

for i in $(seq 0 $((NUM_VALIDATORS - 1))); do
  val_name="${VALIDATOR_NAMES[$i]}"
  val_home="${BASE_DIR}/${val_name}"
  val_p2p=$(p2p_port $i)
  val_rpc=$(rpc_port $i)
  val_grpc=$(grpc_port $i)
  val_api=$(api_port $i)

  config_toml="${val_home}/config/config.toml"
  app_toml="${val_home}/config/app.toml"

  # Build persistent peers (all other validators)
  peers=""
  for j in $(seq 0 $((NUM_VALIDATORS - 1))); do
    if [ $j -ne $i ] && [ -n "${NODE_IDS[$j]}" ]; then
      peer_p2p=$(p2p_port $j)
      if [ -n "$peers" ]; then
        peers="${peers},"
      fi
      peers="${peers}${NODE_IDS[$j]}@127.0.0.1:${peer_p2p}"
    fi
  done

  # ── config.toml ──────────────────────────────────────────────────────

  # P2P listen address
  sed -i.bak "s/^laddr = \"tcp:\/\/0.0.0.0:26656\"/laddr = \"tcp:\/\/0.0.0.0:${val_p2p}\"/" "$config_toml"
  # RPC listen address
  sed -i.bak "s/^laddr = \"tcp:\/\/127.0.0.1:26657\"/laddr = \"tcp:\/\/0.0.0.0:${val_rpc}\"/" "$config_toml"
  # Persistent peers
  sed -i.bak "s/^persistent_peers = .*/persistent_peers = \"${peers}\"/" "$config_toml"
  # CORS
  sed -i.bak 's/cors_allowed_origins = \[\]/cors_allowed_origins = ["*"]/' "$config_toml"
  # Unsafe RPC (for devnet debugging)
  sed -i.bak 's/^unsafe = false/unsafe = true/' "$config_toml"
  # Prometheus metrics
  sed -i.bak 's/^prometheus = false/prometheus = true/' "$config_toml"
  # 2s block time for devnet
  sed -i.bak 's/^timeout_commit = .*/timeout_commit = "2s"/' "$config_toml"
  sed -i.bak 's/^timeout_propose = .*/timeout_propose = "2000ms"/' "$config_toml"
  # Allow duplicate IPs (all validators on localhost)
  sed -i.bak 's/^allow_duplicate_ip = false/allow_duplicate_ip = true/' "$config_toml"
  # Disable addr-book strictness for localhost peers
  sed -i.bak 's/^addr_book_strict = true/addr_book_strict = false/' "$config_toml"

  # ── app.toml ─────────────────────────────────────────────────────────

  # gRPC address
  sed -i.bak "s/^address = \"localhost:9090\"/address = \"localhost:${val_grpc}\"/" "$app_toml"
  sed -i.bak "s/^address = \"0.0.0.0:9090\"/address = \"0.0.0.0:${val_grpc}\"/" "$app_toml"
  # API server address
  sed -i.bak "s|^address = \"tcp://localhost:1317\"|address = \"tcp://localhost:${val_api}\"|" "$app_toml"
  sed -i.bak "s|^address = \"tcp://0.0.0.0:1317\"|address = \"tcp://0.0.0.0:${val_api}\"|" "$app_toml"
  # Enable API server
  sed -i.bak '/^\[api\]/,/^\[/{s/^enable = false/enable = true/}' "$app_toml"
  # Enable unsafe CORS for devnet
  sed -i.bak 's/^enabled-unsafe-cors = false/enabled-unsafe-cors = true/' "$app_toml"
  # Min gas prices
  sed -i.bak "s/^minimum-gas-prices = .*/minimum-gas-prices = \"1${DENOM}\"/" "$app_toml"
  # Disable IAVL fast nodes (prevents "version does not exist" query errors)
  sed -i.bak 's/^iavl-disable-fastnode = false/iavl-disable-fastnode = true/' "$app_toml"
  # Disable inter-block cache for query reliability
  sed -i.bak 's/^inter-block-cache = true/inter-block-cache = false/' "$app_toml"
  # Enable mempool (SDK v0.50 defaults to no-op mempool with max-txs = -1)
  sed -i.bak 's/^max-txs = -1/max-txs = 5000/' "$app_toml"

  # Cleanup sed backups
  rm -f "${config_toml}.bak" "${app_toml}.bak"

  info "  ${val_name}: P2P=${val_p2p} RPC=${val_rpc} gRPC=${val_grpc} API=${val_api}"
done

ok "Networking configured"

# ── Summary ──────────────────────────────────────────────────────────────

echo ""
echo "═══════════════════════════════════════════════════════════════"
echo "  Zerone — Devnet Initialized"
echo "═══════════════════════════════════════════════════════════════"
echo ""
echo "  Chain ID:     ${CHAIN_ID}"
echo "  Validators:   ${NUM_VALIDATORS}"
echo "  Block time:   2s"
echo "  Epoch:        1000 blocks"
echo "  State dir:    ${BASE_DIR}"
echo ""
echo "  Genesis:      ${COORDINATOR_HOME}/config/genesis.json"
echo ""
echo "  Validator Endpoints:"
for i in $(seq 0 $((NUM_VALIDATORS - 1))); do
  val_name="${VALIDATOR_NAMES[$i]}"
  echo "    ${val_name}:"
  echo "      RPC:  http://127.0.0.1:$(rpc_port $i)"
  echo "      gRPC: 127.0.0.1:$(grpc_port $i)"
  echo "      API:  http://127.0.0.1:$(api_port $i)"
done
echo ""
echo "  Agent Wallets:"
for agent_name in "${AGENT_NAMES[@]}"; do
  agent_key=$(echo "${agent_name}" | tr '[:upper:]' '[:lower:]')
  agent_addr=$(${BINARY} keys show "${agent_key}" -a \
    --keyring-backend ${KEYRING} \
    --home "${COORDINATOR_HOME}" 2>/dev/null || echo "?")
  echo "    ${agent_name}: ${agent_addr} ($(( AGENT_BALANCE / 1000000 )) ZRN)"
done
echo ""
echo "  Knowledge Domains:"
echo "    ${DEVNET_DOMAINS[*]}"
echo ""
echo "  Devnet Params:"
echo "    commit_period_blocks:  4"
echo "    reveal_period_blocks:  4"
echo "    epoch_blocks:          1000"
echo "    energy_decay_rate:     100000 (10%)"
echo ""
echo "  persistent_peers:"
echo "    ${PERSISTENT_PEERS}"
echo ""
echo "  Start validators with:"
for i in $(seq 0 $((NUM_VALIDATORS - 1))); do
  val_name="${VALIDATOR_NAMES[$i]}"
  val_home="${BASE_DIR}/${val_name}"
  echo "    ${BINARY} start --home ${val_home} --minimum-gas-prices 1${DENOM}"
done
echo ""
echo "═══════════════════════════════════════════════════════════════"
