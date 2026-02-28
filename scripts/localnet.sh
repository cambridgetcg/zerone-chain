#!/usr/bin/env bash
# ═══════════════════════════════════════════════════════════════════════════
# Zerone — 4-Validator Local Testnet
# ═══════════════════════════════════════════════════════════════════════════
#
# Manages a 4-validator local testnet for PoT consensus, tier progression,
# delegation, slashing, and knowledge verification round testing.
#
# Usage:
#   scripts/localnet.sh start    # Build, init, start all 4 validators
#   scripts/localnet.sh init     # Build + init only (for axiom injection before boot)
#   scripts/localnet.sh boot     # Start validators from initialized state
#   scripts/localnet.sh stop     # Stop all validators
#   scripts/localnet.sh status   # Query each validator's height + power
#   scripts/localnet.sh logs [N] # Tail validator N's logs (default: all)
#   scripts/localnet.sh clean    # Stop + remove all state
#
# Requires: go (1.24+), jq >= 1.6
# ═══════════════════════════════════════════════════════════════════════════

set -euo pipefail

# ── Constants ────────────────────────────────────────────────────────────

CHAIN_ID="zerone-localnet"
DENOM="uzrn"
NUM_VALIDATORS=4
BASE_DIR="${HOME}/.zeroned/localnet"
COORDINATOR_HOME="${BASE_DIR}/coordinator"
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BINARY="${PROJECT_ROOT}/build/zeroned"
KEYRING="test"

BASE_P2P_PORT=26600

# Balances in uzrn: 100K / 1M / 10M / 100M ZRN
VALIDATOR_BALANCES=(100000000000 1000000000000 10000000000000 100000000000000)
# Stakes in uzrn: 100 / 1,000 / 10,000 / 100,000 ZRN
VALIDATOR_STAKES=(100000000 1000000000 10000000000 100000000000)
# Validator monikers
VALIDATOR_NAMES=("val0" "val1" "val2" "val3")

# Bootstrap accounts
FAUCET_BALANCE="10000000000000"  # 10M ZRN
TEST_BALANCE="10000000000"       # 10K ZRN each

# ── Helpers ──────────────────────────────────────────────────────────────

die()  { echo -e "\033[1;31mERROR:\033[0m $*" >&2; exit 1; }
info() { echo -e "\033[1;34m  ->\033[0m $*"; }
ok()   { echo -e "\033[1;32m  OK\033[0m $*"; }
warn() { echo -e "\033[1;33m  !!\033[0m $*"; }

check_deps() {
  command -v jq >/dev/null 2>&1 || die "jq >= 1.6 required. Install: brew install jq"
  command -v go >/dev/null 2>&1 || die "go >= 1.24 required."
}

# Atomic jq patch on coordinator genesis
patch_genesis() {
  local genesis="${COORDINATOR_HOME}/config/genesis.json"
  jq "$1" "$genesis" > "${genesis}.tmp" && mv "${genesis}.tmp" "$genesis"
}

# Get port for validator i
p2p_port()  { echo $(( BASE_P2P_PORT + $1 * 10 )); }
rpc_port()  { echo $(( BASE_P2P_PORT + $1 * 10 + 1 )); }
grpc_port() { echo $(( 9090 + $1 )); }
api_port()  { echo $(( 1317 + $1 )); }

zeroned_coord() {
  "${BINARY}" "$@" --home "${COORDINATOR_HOME}"
}

# ── cmd_init ─────────────────────────────────────────────────────────────
# Initializes the localnet: builds binary, creates genesis, sets up all
# validators, validates genesis, configures networking. Does NOT start
# the validators. Use 'boot' after init (or 'start' for init+boot).

cmd_init() {
  echo "═══════════════════════════════════════════════════════════════"
  echo "  Zerone — Local Testnet: INIT"
  echo "  Chain: ${CHAIN_ID} | Validators: ${NUM_VALIDATORS} | Block: 2521ms"
  echo "═══════════════════════════════════════════════════════════════"
  echo ""

  check_deps

  # ── Step 1: Build ────────────────────────────────────────────────────
  info "Building zeroned binary..."
  mkdir -p "${PROJECT_ROOT}/build"
  (cd "${PROJECT_ROOT}" && go build -ldflags "-X github.com/cosmos/cosmos-sdk/version.Name=zerone -X github.com/cosmos/cosmos-sdk/version.AppName=zeroned" -o build/zeroned ./cmd/zeroned) || die "Build failed"
  ok "Binary built: ${BINARY}"

  # ── Step 2: Clean previous state ─────────────────────────────────────
  if [ -d "${BASE_DIR}" ]; then
    warn "Removing previous localnet state at ${BASE_DIR}"
    rm -rf "${BASE_DIR}"
  fi
  mkdir -p "${BASE_DIR}"

  # ── Step 3: Init coordinator ─────────────────────────────────────────
  info "Initializing coordinator..."
  ${BINARY} init coordinator \
    --chain-id "${CHAIN_ID}" \
    --default-denom "${DENOM}" \
    --home "${COORDINATOR_HOME}" 2>/dev/null

  # ── Step 4: Patch genesis ────────────────────────────────────────────
  info "Patching genesis parameters..."

  # Consensus params — enable vote extensions at height 1
  patch_genesis '
    .consensus.params.block.max_gas = "33333333" |
    .consensus.params.block.max_bytes = "4194304" |
    .consensus.params.abci.vote_extensions_enable_height = "1"
  '

  # SDK module overrides
  patch_genesis '
    .app_state.staking.params.bond_denom = "uzrn" |
    .app_state.slashing.params.signed_blocks_window = "100" |
    .app_state.slashing.params.min_signed_per_window = "0.500000000000000000" |
    .app_state.slashing.params.downtime_jail_duration = "60s" |
    .app_state.slashing.params.slash_fraction_downtime = "0.010000000000000000" |
    .app_state.gov.params.voting_period = "60s" |
    .app_state.gov.params.expedited_voting_period = "30s"
  '

  # Zerone governance — fast params for local testing
  patch_genesis '
    .app_state.zerone_gov.params.voting_period_blocks = 10 |
    .app_state.zerone_gov.params.discussion_period_blocks = 5 |
    .app_state.zerone_gov.params.category_configs = [
      {"category":"parameter","required_stake_bps":"1000000","review_blocks":5},
      {"category":"upgrade","required_stake_bps":"1000000","review_blocks":5},
      {"category":"text","required_stake_bps":"1000000","review_blocks":5},
      {"category":"research_spend","required_stake_bps":"1000000","review_blocks":5},
      {"category":"research_seat_election","required_stake_bps":"1000000","review_blocks":5},
      {"category":"research_phase_transition","required_stake_bps":"1000000","review_blocks":5},
      {"category":"research_phase_rollback","required_stake_bps":"1000000","review_blocks":5}
    ] |
    .app_state.zerone_gov.params.research_discussion_blocks = 5 |
    .app_state.zerone_gov.params.research_voting_blocks = 10
  '

  # Knowledge module — fast params for local testing
  patch_genesis '
    .app_state.knowledge.params.min_verifiers = 2 |
    .app_state.knowledge.params.commit_phase_blocks = 10 |
    .app_state.knowledge.params.reveal_phase_blocks = 10 |
    .app_state.knowledge.params.aggregation_phase_blocks = 5 |
    .app_state.knowledge.params.adversarial_verification_enabled = false |
    .app_state.knowledge.params.min_claim_text_length = 20 |
    .app_state.knowledge.params.confidence_threshold = 770000 |
    .app_state.knowledge.params.quorum_threshold = 660000 |
    .app_state.knowledge.params.max_validators_per_round = 22 |
    .app_state.knowledge.params.verification_reward = "3000000"
  '

  # Research module — fast params for local testing
  patch_genesis '
    .app_state.research.params.review_period_blocks = 20 |
    .app_state.research.params.min_reviewer_count = 2
  '

  # Partnerships module — fast params for local testing
  patch_genesis '
    .app_state.partnerships.params.safety_freeze_duration_blocks = 10 |
    .app_state.partnerships.params.coercion_review_blocks = 15
  '

  # Vesting rewards — lower min_validators for local testing
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
  ok "Genesis patched"

  # ── Step 5: Add bootstrap accounts ──────────────────────────────────
  info "Creating bootstrap accounts..."

  # Faucet
  ${BINARY} keys add faucet --keyring-backend ${KEYRING} --home "${COORDINATOR_HOME}" 2>/dev/null
  FAUCET_ADDR=$(${BINARY} keys show faucet -a --keyring-backend ${KEYRING} --home "${COORDINATOR_HOME}")
  ${BINARY} add-genesis-account "${FAUCET_ADDR}" "${FAUCET_BALANCE}${DENOM}" --home "${COORDINATOR_HOME}"
  info "  Faucet: ${FAUCET_ADDR} (10M ZRN)"

  # Test accounts
  for i in 1 2 3; do
    ${BINARY} keys add "test${i}" --keyring-backend ${KEYRING} --home "${COORDINATOR_HOME}" 2>/dev/null
    TEST_ADDR=$(${BINARY} keys show "test${i}" -a --keyring-backend ${KEYRING} --home "${COORDINATOR_HOME}")
    ${BINARY} add-genesis-account "${TEST_ADDR}" "${TEST_BALANCE}${DENOM}" --home "${COORDINATOR_HOME}"
    info "  test${i}: ${TEST_ADDR} (10K ZRN)"
  done

  # ── Step 6: Setup validators ────────────────────────────────────────
  info "Setting up ${NUM_VALIDATORS} validators..."
  mkdir -p "${COORDINATOR_HOME}/config/gentx"

  for i in $(seq 0 $((NUM_VALIDATORS - 1))); do
    local_name="${VALIDATOR_NAMES[$i]}"
    local_home="${BASE_DIR}/${local_name}"
    local_balance="${VALIDATOR_BALANCES[$i]}"
    local_stake="${VALIDATOR_STAKES[$i]}"
    local_balance_zrn=$(( local_balance / 1000000 ))
    local_stake_zrn=$(( local_stake / 1000000 ))

    info "  ${local_name}: balance=${local_balance_zrn} ZRN, stake=${local_stake_zrn} ZRN"

    # Init separate home (unique consensus key)
    ${BINARY} init "${local_name}" \
      --chain-id "${CHAIN_ID}" \
      --home "${local_home}" \
      --overwrite 2>/dev/null

    # Copy coordinator genesis (has params + bootstrap accounts)
    cp "${COORDINATOR_HOME}/config/genesis.json" "${local_home}/config/genesis.json"

    # Create validator account key in coordinator keyring
    ${BINARY} keys add "${local_name}" \
      --keyring-backend ${KEYRING} \
      --home "${COORDINATOR_HOME}" 2>/dev/null

    # Get address
    local_addr=$(${BINARY} keys show "${local_name}" -a --keyring-backend ${KEYRING} --home "${COORDINATOR_HOME}")

    # Fund validator in coordinator genesis
    ${BINARY} add-genesis-account "${local_addr}" "${local_balance}${DENOM}" \
      --home "${COORDINATOR_HOME}"

    # Copy updated genesis + keyring to validator home
    cp "${COORDINATOR_HOME}/config/genesis.json" "${local_home}/config/genesis.json"
    cp -r "${COORDINATOR_HOME}/keyring-test" "${local_home}/"

    # Generate gentx
    ${BINARY} genesis gentx "${local_name}" "${local_stake}${DENOM}" \
      --chain-id "${CHAIN_ID}" \
      --keyring-backend ${KEYRING} \
      --home "${local_home}" \
      --moniker "${local_name}" \
      --commission-rate "0.10" \
      --commission-max-rate "0.20" \
      --commission-max-change-rate "0.01" \
      --output-document "${COORDINATOR_HOME}/config/gentx/gentx-${local_name}.json" 2>/dev/null
  done

  # ── Step 7: Collect gentxs ──────────────────────────────────────────
  info "Collecting gentxs..."
  ${BINARY} genesis collect-gentxs --home "${COORDINATOR_HOME}" 2>/dev/null
  ok "Gentxs collected"

  # ── Step 8: Validate genesis ────────────────────────────────────────
  info "Validating genesis..."
  if ${BINARY} genesis validate --home "${COORDINATOR_HOME}" 2>&1; then
    ok "Genesis valid"
  else
    die "Genesis validation FAILED"
  fi

  # ── Step 9: Distribute final genesis ────────────────────────────────
  info "Distributing final genesis to all validators..."
  for i in $(seq 0 $((NUM_VALIDATORS - 1))); do
    local_name="${VALIDATOR_NAMES[$i]}"
    local_home="${BASE_DIR}/${local_name}"
    cp "${COORDINATOR_HOME}/config/genesis.json" "${local_home}/config/genesis.json"
  done

  # ── Step 10: Collect node IDs and patch configs ─────────────────────
  info "Configuring validator networking..."

  # Collect all node IDs first
  declare -a NODE_IDS
  for i in $(seq 0 $((NUM_VALIDATORS - 1))); do
    local_name="${VALIDATOR_NAMES[$i]}"
    local_home="${BASE_DIR}/${local_name}"
    NODE_IDS[$i]=$(${BINARY} tendermint show-node-id --home "${local_home}" 2>/dev/null || \
                   ${BINARY} comet show-node-id --home "${local_home}" 2>/dev/null || \
                   echo "")
  done

  for i in $(seq 0 $((NUM_VALIDATORS - 1))); do
    local_name="${VALIDATOR_NAMES[$i]}"
    local_home="${BASE_DIR}/${local_name}"
    local_p2p=$(p2p_port $i)
    local_rpc=$(rpc_port $i)
    local_grpc=$(grpc_port $i)
    local_api=$(api_port $i)

    config_toml="${local_home}/config/config.toml"
    app_toml="${local_home}/config/app.toml"

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

    # Patch config.toml
    sed -i.bak "s/^laddr = \"tcp:\/\/0.0.0.0:26656\"/laddr = \"tcp:\/\/0.0.0.0:${local_p2p}\"/" "$config_toml"
    sed -i.bak "s/^laddr = \"tcp:\/\/127.0.0.1:26657\"/laddr = \"tcp:\/\/0.0.0.0:${local_rpc}\"/" "$config_toml"
    sed -i.bak "s/^persistent_peers = .*/persistent_peers = \"${peers}\"/" "$config_toml"
    sed -i.bak 's/cors_allowed_origins = \[\]/cors_allowed_origins = ["*"]/' "$config_toml"
    sed -i.bak 's/^unsafe = false/unsafe = true/' "$config_toml"
    sed -i.bak 's/^prometheus = false/prometheus = true/' "$config_toml"
    sed -i.bak 's/^timeout_commit = .*/timeout_commit = "2521ms"/' "$config_toml"
    sed -i.bak 's/^timeout_propose = .*/timeout_propose = "2000ms"/' "$config_toml"
    # Allow duplicate IPs for local testnet
    sed -i.bak 's/^allow_duplicate_ip = false/allow_duplicate_ip = true/' "$config_toml"
    # Disable addr-book strictness for localhost peers
    sed -i.bak 's/^addr_book_strict = true/addr_book_strict = false/' "$config_toml"

    # Patch app.toml — gRPC and API ports
    sed -i.bak "s/^address = \"localhost:9090\"/address = \"localhost:${local_grpc}\"/" "$app_toml"
    sed -i.bak "s/^address = \"0.0.0.0:9090\"/address = \"0.0.0.0:${local_grpc}\"/" "$app_toml"
    # API server address
    sed -i.bak "s|^address = \"tcp://localhost:1317\"|address = \"tcp://localhost:${local_api}\"|" "$app_toml"
    sed -i.bak "s|^address = \"tcp://0.0.0.0:1317\"|address = \"tcp://0.0.0.0:${local_api}\"|" "$app_toml"
    sed -i.bak 's/^enabled-unsafe-cors = false/enabled-unsafe-cors = true/' "$app_toml"
    # Min gas prices
    sed -i.bak "s/^minimum-gas-prices = .*/minimum-gas-prices = \"1${DENOM}\"/" "$app_toml"
    # Disable IAVL fast nodes (prevents "version does not exist" query errors)
    sed -i.bak 's/^iavl-disable-fastnode = false/iavl-disable-fastnode = true/' "$app_toml"
    # Disable inter-block cache for query reliability
    sed -i.bak 's/^inter-block-cache = true/inter-block-cache = false/' "$app_toml"
    # Enable mempool (default is no-op mempool which drops all transactions)
    sed -i.bak 's/^max-txs = -1/max-txs = 5000/' "$app_toml"

    # Cleanup sed backups
    rm -f "${config_toml}.bak" "${app_toml}.bak"

    info "  ${local_name}: P2P=${local_p2p} RPC=${local_rpc} gRPC=${local_grpc} API=${local_api}"
  done

  ok "Localnet initialized — ready for 'boot' or 'start'"
  echo ""
  echo "  Genesis:   ${COORDINATOR_HOME}/config/genesis.json"
  echo "  State dir: ${BASE_DIR}"
  echo ""
  echo "  Next steps:"
  echo "    scripts/localnet.sh boot    # Start validators from initialized state"
  echo "    scripts/localnet.sh start   # (or re-run start for init + boot)"
}

# ── _boot_validators (internal) ──────────────────────────────────────────
# Starts all validator processes and waits for consensus.
# Called by cmd_start and cmd_boot.

_boot_validators() {
  # Trap EXIT for cleanup on failure
  trap 'echo ""; warn "Script interrupted — run: scripts/localnet.sh stop"' INT TERM

  # ── Start all validators ───────────────────────────────────────────
  info "Starting ${NUM_VALIDATORS} validators..."

  for i in $(seq 0 $((NUM_VALIDATORS - 1))); do
    local_name="${VALIDATOR_NAMES[$i]}"
    local_home="${BASE_DIR}/${local_name}"
    local_log="${BASE_DIR}/${local_name}.log"

    ${BINARY} start \
      --home "${local_home}" \
      --minimum-gas-prices "1${DENOM}" \
      > "${local_log}" 2>&1 &

    local_pid=$!
    echo "${local_pid}" > "${BASE_DIR}/${local_name}.pid"
    info "  ${local_name} started (PID=${local_pid})"
  done

  # ── Wait for consensus ─────────────────────────────────────────────
  info "Waiting for consensus (target: block 3)..."
  local rpc_url="http://127.0.0.1:$(rpc_port 0)"
  local max_wait=60
  local elapsed=0

  while [ $elapsed -lt $max_wait ]; do
    local height
    height=$(curl -s "${rpc_url}/status" 2>/dev/null | jq -r '.result.sync_info.latest_block_height' 2>/dev/null || echo "0")
    if [ "${height}" -ge 3 ] 2>/dev/null; then
      ok "Consensus reached at block ${height}"
      break
    fi
    sleep 2
    elapsed=$((elapsed + 2))
    if [ $((elapsed % 10)) -eq 0 ]; then
      info "  still waiting... (${elapsed}s, height=${height:-0})"
    fi
  done

  if [ $elapsed -ge $max_wait ]; then
    warn "Timed out waiting for block 3 after ${max_wait}s"
    warn "Check logs: scripts/localnet.sh logs"
  fi
}

# ── cmd_boot ─────────────────────────────────────────────────────────────
# Starts validators from an already-initialized localnet state.
# Use after 'init' (e.g., to inject axioms between init and boot).

cmd_boot() {
  echo "═══════════════════════════════════════════════════════════════"
  echo "  Zerone — Local Testnet: BOOT (from initialized state)"
  echo "═══════════════════════════════════════════════════════════════"
  echo ""

  [ -d "${BASE_DIR}/val0" ] || die "No initialized localnet found. Run 'init' first."
  [ -f "${BINARY}" ] || die "Binary not found: ${BINARY}. Run 'init' or 'make build' first."

  _boot_validators

  _print_summary
}

# ── cmd_start ────────────────────────────────────────────────────────────
# Full pipeline: init + boot.

cmd_start() {
  cmd_init

  _boot_validators

  _print_summary
}

# ── _print_summary (internal) ───────────────────────────────────────────

_print_summary() {
  echo ""
  echo "═══════════════════════════════════════════════════════════════"
  echo "  Zerone — Local Testnet RUNNING"
  echo "═══════════════════════════════════════════════════════════════"
  echo ""
  echo "  Chain ID:     ${CHAIN_ID}"
  echo "  Validators:   ${NUM_VALIDATORS}"
  echo "  Block time:   2521ms"
  echo "  State dir:    ${BASE_DIR}"
  echo ""
  echo "  Validator Endpoints:"
  for i in $(seq 0 $((NUM_VALIDATORS - 1))); do
    local_name="${VALIDATOR_NAMES[$i]}"
    local_stake_zrn=$(( VALIDATOR_STAKES[$i] / 1000000 ))
    echo "    ${local_name} (stake: ${local_stake_zrn} ZRN):"
    echo "      RPC:  http://127.0.0.1:$(rpc_port $i)"
    echo "      gRPC: 127.0.0.1:$(grpc_port $i)"
    echo "      API:  http://127.0.0.1:$(api_port $i)"
  done
  echo ""
  echo "  Key Accounts (in coordinator keyring):"
  for name in faucet test1 test2 test3; do
    addr=$(${BINARY} keys show "${name}" -a --keyring-backend ${KEYRING} --home "${COORDINATOR_HOME}" 2>/dev/null || echo "?")
    echo "    ${name}: ${addr}"
  done
  echo ""
  echo "  Knowledge Params (for PoT testing):"
  echo "    min_verifiers:             2"
  echo "    commit_phase_blocks:       10"
  echo "    reveal_phase_blocks:       10"
  echo "    aggregation_phase_blocks:  5"
  echo "    adversarial_verification:  false"
  echo ""
  echo "  Commands:"
  echo "    Status:    scripts/localnet.sh status"
  echo "    Logs:      scripts/localnet.sh logs [0-3]"
  echo "    Stop:      scripts/localnet.sh stop"
  echo "    Clean:     scripts/localnet.sh clean"
  echo "    Test:      scripts/localnet-test.sh"
  echo "═══════════════════════════════════════════════════════════════"
}

# ── cmd_stop ─────────────────────────────────────────────────────────────

cmd_stop() {
  echo "Stopping local testnet..."

  local stopped=0
  for i in $(seq 0 $((NUM_VALIDATORS - 1))); do
    local_name="${VALIDATOR_NAMES[$i]}"
    local_pid_file="${BASE_DIR}/${local_name}.pid"

    if [ -f "${local_pid_file}" ]; then
      local_pid=$(cat "${local_pid_file}")
      if kill -0 "${local_pid}" 2>/dev/null; then
        kill "${local_pid}" 2>/dev/null || true
        info "Stopped ${local_name} (PID=${local_pid})"
        stopped=$((stopped + 1))
      else
        info "${local_name} already stopped"
      fi
      rm -f "${local_pid_file}"
    fi
  done

  # Fallback: kill any remaining zeroned processes for this localnet
  if pgrep -f "zeroned.*start.*localnet" >/dev/null 2>&1; then
    pkill -f "zeroned.*start.*localnet" 2>/dev/null || true
    info "Killed remaining zeroned localnet processes"
  fi

  if [ $stopped -gt 0 ]; then
    ok "Stopped ${stopped} validators"
  else
    info "No validators were running"
  fi
}

# ── cmd_status ───────────────────────────────────────────────────────────

cmd_status() {
  echo ""
  echo "═══ Zerone Local Testnet Status ═══"
  echo ""

  local all_ok=true
  for i in $(seq 0 $((NUM_VALIDATORS - 1))); do
    local_name="${VALIDATOR_NAMES[$i]}"
    local_rpc="http://127.0.0.1:$(rpc_port $i)"
    local_pid_file="${BASE_DIR}/${local_name}.pid"

    # Check PID
    local running="no"
    if [ -f "${local_pid_file}" ]; then
      local_pid=$(cat "${local_pid_file}")
      if kill -0 "${local_pid}" 2>/dev/null; then
        running="yes (PID=${local_pid})"
      fi
    fi

    # Query RPC
    local height="?"
    local voting_power="?"
    if curl -s --connect-timeout 2 "${local_rpc}/status" >/dev/null 2>&1; then
      local status_json
      status_json=$(curl -s "${local_rpc}/status")
      height=$(echo "$status_json" | jq -r '.result.sync_info.latest_block_height' 2>/dev/null || echo "?")
      voting_power=$(echo "$status_json" | jq -r '.result.validator_info.voting_power' 2>/dev/null || echo "?")
    else
      all_ok=false
    fi

    echo "  ${local_name}:"
    echo "    Running:      ${running}"
    echo "    RPC:          ${local_rpc}"
    echo "    Block Height: ${height}"
    echo "    Voting Power: ${voting_power}"
    echo ""
  done

  if [ "$all_ok" = false ]; then
    warn "Some validators are not reachable"
  fi
}

# ── cmd_logs ─────────────────────────────────────────────────────────────

cmd_logs() {
  local val_num="${1:-}"

  if [ -n "${val_num}" ]; then
    # Specific validator
    local local_name="${VALIDATOR_NAMES[$val_num]:-}"
    if [ -z "${local_name}" ]; then
      die "Invalid validator number: ${val_num} (valid: 0-$((NUM_VALIDATORS - 1)))"
    fi
    local log_file="${BASE_DIR}/${local_name}.log"
    if [ ! -f "${log_file}" ]; then
      die "Log file not found: ${log_file}"
    fi
    tail -f "${log_file}"
  else
    # All validators (interleaved)
    local log_files=()
    for i in $(seq 0 $((NUM_VALIDATORS - 1))); do
      local_name="${VALIDATOR_NAMES[$i]}"
      local log_file="${BASE_DIR}/${local_name}.log"
      if [ -f "${log_file}" ]; then
        log_files+=("${log_file}")
      fi
    done
    if [ ${#log_files[@]} -eq 0 ]; then
      die "No log files found in ${BASE_DIR}"
    fi
    tail -f "${log_files[@]}"
  fi
}

# ── cmd_clean ────────────────────────────────────────────────────────────

cmd_clean() {
  cmd_stop
  echo ""
  if [ -d "${BASE_DIR}" ]; then
    info "Removing all localnet state at ${BASE_DIR}..."
    rm -rf "${BASE_DIR}"
    ok "Localnet state removed"
  else
    info "No localnet state found at ${BASE_DIR}"
  fi
}

# ── cmd_resume ──────────────────────────────────────────────────────────

cmd_resume() {
  echo "═══════════════════════════════════════════════════════════════"
  echo "  Zerone — Local Testnet: RESUME (from existing state)"
  echo "═══════════════════════════════════════════════════════════════"
  echo ""

  check_deps

  # Verify state exists
  if [ ! -d "${BASE_DIR}/val0" ]; then
    die "No existing localnet state found at ${BASE_DIR}. Use 'start' first."
  fi

  # Build fresh binary
  info "Building zeroned binary..."
  mkdir -p "${PROJECT_ROOT}/build"
  (cd "${PROJECT_ROOT}" && go build -ldflags "-X github.com/cosmos/cosmos-sdk/version.Name=zerone -X github.com/cosmos/cosmos-sdk/version.AppName=zeroned" -o build/zeroned ./cmd/zeroned) || die "Build failed"
  ok "Binary built: ${BINARY}"

  # Stop any running validators first
  cmd_stop 2>/dev/null || true

  # Start all validators from existing state
  info "Starting ${NUM_VALIDATORS} validators from existing state..."

  for i in $(seq 0 $((NUM_VALIDATORS - 1))); do
    local_name="${VALIDATOR_NAMES[$i]}"
    local_home="${BASE_DIR}/${local_name}"
    local_log="${BASE_DIR}/${local_name}.log"

    if [ ! -d "${local_home}" ]; then
      warn "Validator home ${local_home} not found, skipping"
      continue
    fi

    ${BINARY} start \
      --home "${local_home}" \
      --minimum-gas-prices "1${DENOM}" \
      >> "${local_log}" 2>&1 &

    local_pid=$!
    echo "${local_pid}" > "${BASE_DIR}/${local_name}.pid"
    info "  ${local_name} started (PID=${local_pid})"
  done

  # Wait for chain to resume producing blocks
  info "Waiting for chain to resume..."
  local rpc_url="http://127.0.0.1:$(rpc_port 0)"
  local max_wait=90
  local elapsed=0

  while [ $elapsed -lt $max_wait ]; do
    local height
    height=$(curl -s "${rpc_url}/status" 2>/dev/null | jq -r '.result.sync_info.latest_block_height' 2>/dev/null || echo "0")
    if [ "${height}" -ge 1 ] 2>/dev/null; then
      ok "Chain resumed at block ${height}"
      break
    fi
    sleep 2
    elapsed=$((elapsed + 2))
    if [ $((elapsed % 10)) -eq 0 ]; then
      info "  still waiting... (${elapsed}s)"
    fi
  done

  if [ $elapsed -ge $max_wait ]; then
    warn "Timed out waiting for chain to resume after ${max_wait}s"
    warn "Check logs: scripts/localnet.sh logs"
  fi
}

# ── Main ─────────────────────────────────────────────────────────────────

case "${1:-help}" in
  start)   cmd_start ;;
  init)    cmd_init ;;
  boot)    cmd_boot ;;
  stop)    cmd_stop ;;
  resume)  cmd_resume ;;
  status)  cmd_status ;;
  logs)    cmd_logs "${2:-}" ;;
  clean)   cmd_clean ;;
  help|--help|-h)
    echo "Usage: $0 {start|init|boot|stop|resume|status|logs [N]|clean}"
    echo ""
    echo "Commands:"
    echo "  start       Build, init, and start 4-validator local testnet"
    echo "  init        Build and init only (no start — for axiom injection)"
    echo "  boot        Start validators from already-initialized state"
    echo "  stop        Stop all validators"
    echo "  resume      Rebuild and restart validators from existing state"
    echo "  status      Query each validator's block height and voting power"
    echo "  logs [N]    Tail logs (all validators, or validator N = 0-3)"
    echo "  clean       Stop and remove all localnet state"
    echo ""
    echo "State directory: ${BASE_DIR}"
    echo "Chain ID:        ${CHAIN_ID}"
    ;;
  *)
    die "Unknown command: $1. Use --help for usage."
    ;;
esac
