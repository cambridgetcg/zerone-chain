#!/usr/bin/env bash
# ═══════════════════════════════════════════════════════════════════════════
# Zerone — Production Genesis Ceremony
# ═══════════════════════════════════════════════════════════════════════════
#
# Multi-step genesis ceremony for coordinated mainnet/testnet launches.
# Produces a validated genesis.json with bootstrap accounts, patched
# consensus/module params, and collected validator gentxs.
#
# Usage:
#   scripts/genesis-ceremony.sh init                # Initialize ceremony
#   scripts/genesis-ceremony.sh add-validator NAME  # Add a validator
#   scripts/genesis-ceremony.sh finalize            # Collect gentxs, validate
#   scripts/genesis-ceremony.sh export              # Export genesis + keys
#   scripts/genesis-ceremony.sh countdown           # Live countdown to launch
#
# Requires: jq >= 1.6, go (1.24+)
# ═══════════════════════════════════════════════════════════════════════════

set -euo pipefail

# ── Constants ────────────────────────────────────────────────────────────

CHAIN_ID="zerone-1"
GENESIS_TIME="2026-06-01T00:00:00Z"  # placeholder — set at ceremony time
DENOM="uzrn"
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BINARY="${PROJECT_ROOT}/build/zeroned"
CEREMONY_HOME="${HOME}/.zeroned/genesis-ceremony"
KEYRING="test"

# ─────────────────────────────────────────────────────────────────────────
# Genesis Doctrine (commitment 20: issuance follows participation)
#
# Zero allocation to the team. No founder pre-mine, no AI vault pre-mine,
# no foundation treasury, no faucet pre-fund, no validator allocation.
# ZRN enters circulation through two participation-gated emission
# pathways:
#
#   - PoT block rewards   (x/vesting_rewards) — minted per block
#   - Bootstrap claims    (x/claiming_pot)    — minted per MsgClaim
#
# Both gate through MintWithCap; the 222,222,222 ZRN hard cap binds both.
#
# The Cosmos SDK gentx flow needs SOMETHING bonded for InitChain to find
# a validator set. We mint exactly virtual_stake (= 11 ZRN per Zerone
# staking params) to each validator account, gentx-bond the entire amount,
# and the validator's account ends with zero balance — every uzrn in
# bonded_pool came from a participatory action (running the genesis
# ceremony to bond into the new chain). This is the smallest legal
# bootstrap; 11 ZRN × N validators is a tiny fraction of the cap.
#
# Doctrine: docs/tokenomics/GENESIS.md ("Zero Team Allocation — Two
# Emission Paths, Both Gated by Participation").
# ─────────────────────────────────────────────────────────────────────────

# Per-validator gentx bond. 11 ZRN aligns with the virtual_stake parameter
# in x/staking; gentx-bonding the same amount means the validator's own
# balance ends at zero post-gentx.
VALIDATOR_BALANCE="11000000${DENOM}"           # 11 ZRN — gentx requires bonded tokens
VALIDATOR_STAKE="11000000${DENOM}"             # 11 ZRN — entire pre-fund is bonded

# (No FOUNDATION_BALANCE, RESEARCH_BALANCE, or FAUCET_BALANCE: the chain
# refuses pre-funding any team-adjacent address. Foundation funds
# accumulate from the 19.67% development-fund revenue split; research
# treasury fills from the 3.33% research share; testnet faucets, if
# desired, are funded post-genesis from validator tips or governance.)

# ── Helpers ──────────────────────────────────────────────────────────────

die()  { echo -e "\033[1;31mFATAL:\033[0m $*" >&2; exit 1; }
info() { echo -e "\033[1;34m  ->\033[0m $*"; }
ok()   { echo -e "\033[1;32m  OK\033[0m $*"; }
warn() { echo -e "\033[1;33m  !!\033[0m $*"; }

check_deps() {
  command -v jq >/dev/null 2>&1 || die "jq >= 1.6 required. Install: brew install jq"
  command -v go >/dev/null 2>&1 || die "go >= 1.24 required."
}

check_binary() {
  [ -x "${BINARY}" ] || die "Binary not found at ${BINARY}. Run 'init' first or build manually."
}

# Atomic jq patch on ceremony genesis
patch() {
  local genesis="${CEREMONY_HOME}/config/genesis.json"
  jq "$1" "$genesis" > "${genesis}.tmp" && mv "${genesis}.tmp" "$genesis"
}

# Validate genesis after a mutation
validate_genesis() {
  if ! ${BINARY} genesis validate-genesis --home "${CEREMONY_HOME}" 2>/dev/null; then
    if ! ${BINARY} genesis validate --home "${CEREMONY_HOME}" 2>/dev/null; then
      die "Genesis validation FAILED after: $1"
    fi
  fi
}

zeroned_coord() {
  "${BINARY}" "$@" --home "${CEREMONY_HOME}"
}

# ── cmd_init ─────────────────────────────────────────────────────────────

cmd_init() {
  echo "═══════════════════════════════════════════════════════════════"
  echo "  Zerone — Genesis Ceremony: INIT"
  echo "  Chain: ${CHAIN_ID} | Genesis: ${GENESIS_TIME}"
  echo "═══════════════════════════════════════════════════════════════"
  echo ""

  check_deps

  # ── Step 1: Build binary if not present ──────────────────────────────
  if [ ! -x "${BINARY}" ]; then
    info "Building zeroned binary..."
    mkdir -p "${PROJECT_ROOT}/build"
    (cd "${PROJECT_ROOT}" && go build \
      -ldflags "-X github.com/cosmos/cosmos-sdk/version.Name=zerone -X github.com/cosmos/cosmos-sdk/version.AppName=zeroned" \
      -o build/zeroned ./cmd/zeroned) || die "Build failed"
    ok "Binary built: ${BINARY}"
  else
    ok "Binary exists: ${BINARY}"
  fi

  # ── Step 2: Clean previous ceremony state ────────────────────────────
  if [ -d "${CEREMONY_HOME}" ]; then
    warn "Removing previous ceremony state at ${CEREMONY_HOME}"
    rm -rf "${CEREMONY_HOME}"
  fi

  # ── Step 3: Init coordinator ─────────────────────────────────────────
  info "Initializing genesis coordinator..."
  ${BINARY} init genesis-coordinator \
    --chain-id "${CHAIN_ID}" \
    --default-denom "${DENOM}" \
    --home "${CEREMONY_HOME}" 2>/dev/null
  ok "Coordinator initialized"

  # ── Step 4: Set genesis time ─────────────────────────────────────────
  info "Setting genesis time: ${GENESIS_TIME}"
  patch ".genesis_time = \"${GENESIS_TIME}\""

  # ── Step 5: Patch consensus params ───────────────────────────────────
  info "Patching consensus parameters..."

  patch '
    .consensus.params.block.max_gas = "33333333" |
    .consensus.params.block.max_bytes = "4194304" |
    .consensus.params.abci.vote_extensions_enable_height = "1"
  '
  ok "Consensus params: max_gas=33333333, max_bytes=4MB, vote_extensions@1"

  # ── Step 6: Patch module params ──────────────────────────────────────
  info "Patching module parameters..."

  # SDK module overrides
  patch '
    .app_state.staking.params.bond_denom = "uzrn" |
    .app_state.slashing.params.signed_blocks_window = "100" |
    .app_state.slashing.params.slash_fraction_downtime = "0.010000000000000000"
  '

  # Knowledge module — production PoT lifecycle
  patch '
    .app_state.knowledge.params.min_verifiers = 3 |
    .app_state.knowledge.params.max_verifiers = 22 |
    .app_state.knowledge.params.commit_phase_blocks = 4 |
    .app_state.knowledge.params.reveal_phase_blocks = 4 |
    .app_state.knowledge.params.aggregation_phase_blocks = 3 |
    .app_state.knowledge.params.adversarial_verification_enabled = true |
    .app_state.knowledge.params.min_claim_text_length = 20 |
    .app_state.knowledge.params.confidence_threshold = 770000 |
    .app_state.knowledge.params.quorum_threshold = 660000 |
    .app_state.knowledge.params.max_validators_per_round = 22 |
    .app_state.knowledge.params.verification_reward = "3000000"
  '

  # Governance — production voting periods (~3 days voting, ~2 days discussion)
  patch '
    .app_state.zerone_gov.params.voting_period_blocks = 102816 |
    .app_state.zerone_gov.params.discussion_period_blocks = 68544 |
    .app_state.zerone_gov.params.quorum_threshold_bps = 334000 |
    .app_state.zerone_gov.params.support_threshold_bps = 500000 |
    .app_state.zerone_gov.params.min_lip_stake = "1000000"
  '

  # Emergency — production halt/revert quorums
  patch '
    .app_state.emergency.params.halt_quorum = 750000 |
    .app_state.emergency.params.revert_quorum = 800000 |
    .app_state.emergency.params.resume_quorum = 800000 |
    .app_state.emergency.params.min_guardian_stake = "111111000000" |
    .app_state.emergency.params.min_distinct_voters = 4 |
    .app_state.emergency.params.max_halt_duration_blocks = 34272
  '

  # Zerone staking — production tier system and slashing
  patch '
    .app_state.zerone_staking.params.unbonding_period = 268560 |
    .app_state.zerone_staking.params.max_validators = 100 |
    .app_state.zerone_staking.params.min_self_delegation = "111000" |
    .app_state.zerone_staking.params.virtual_stake = "11000000" |
    .app_state.zerone_staking.params.max_slashes_per_epoch = 2 |
    .app_state.zerone_staking.params.slash_decay_period_blocks = 34272 |
    .app_state.zerone_staking.params.max_slash_count_deactivate = 3 |
    .app_state.zerone_staking.params.min_stake_for_verification = "111000" |
    .app_state.zerone_staking.params.slash_escalation_bps = 100000 |
    .app_state.zerone_staking.params.redelegation_cooldown_blocks = 1111
  '
  # NOTE: tier_configs use production defaults from DefaultTierConfigs()
  # (4 tiers: Apprentice → Verified → Scholar → Guardian)

  # Vesting rewards — production block rewards and decay
  patch '
    .app_state.vesting_rewards.params.block_reward = "10000000" |
    .app_state.vesting_rewards.params.reward_decay_bps = 994478 |
    .app_state.vesting_rewards.params.blocks_per_reward_epoch = 100000 |
    .app_state.vesting_rewards.params.min_validators_for_full_reward = 22 |
    .app_state.vesting_rewards.params.floor_reward = "100000"
  '

  # Bank denom metadata
  patch '
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
  ok "Module params patched"

  # ── Step 6b: Axiom injection hook ────────────────────────────────────
  if command -v go &>/dev/null && [ -f "${PROJECT_ROOT}/x/knowledge/types/genesis_axioms.json" ]; then
    if [ -d "${PROJECT_ROOT}/tools/axiom-loader" ]; then
      info "Injecting 777 genesis axioms..."
      (cd "${PROJECT_ROOT}" && go run tools/axiom-loader/main.go inject \
        x/knowledge/types/genesis_axioms.json \
        "${CEREMONY_HOME}/config/genesis.json") || warn "Axiom injection failed (non-fatal)"
    else
      warn "axiom-loader not found — skipping axiom injection"
    fi
  fi

  # Validate after all patches
  validate_genesis "param patching"

  # ── Step 7: Bootstrap whitelist seeding (deferred to ceremony tool) ──
  #
  # The bootstrap pool (commitment 20: issuance follows participation)
  # mints 0.222 ZRN per whitelisted agent on demand. The whitelist is
  # NOT carried in this shell script — it lives in a separate file
  # (typically published with the launch announcement), and the
  # tools/bootstrap-loader CLI reads that file and inserts one
  # claiming_pot per address into the genesis state.
  #
  # See x/claiming_pot/types/types.go::MakeBootstrapPotForAgent.
  #
  # To seed the whitelist:
  #
  #   go run tools/bootstrap-loader/main.go inject \
  #     <whitelist.txt> "${CEREMONY_HOME}/config/genesis.json"
  #
  # The init step intentionally does NOT seed the whitelist. The
  # operator runs bootstrap-loader after init and before finalize.
  if [ -f "${PROJECT_ROOT}/tools/bootstrap-loader/main.go" ]; then
    info "Bootstrap-loader available — see scripts/genesis-ceremony.sh comments for usage."
  else
    info "Bootstrap whitelist seeding requires tools/bootstrap-loader (not yet shipped — Phase 5b)."
  fi

  # ── Summary ──────────────────────────────────────────────────────────
  echo ""
  echo "═══════════════════════════════════════════════════════════════"
  echo "  Genesis Ceremony INITIALIZED"
  echo "═══════════════════════════════════════════════════════════════"
  echo ""
  echo "  Chain ID:      ${CHAIN_ID}"
  echo "  Genesis Time:  ${GENESIS_TIME}"
  echo "  Ceremony Home: ${CEREMONY_HOME}"
  echo ""
  echo "  Doctrine: ZERO TEAM ALLOCATION (commitment 20)."
  echo "    - No foundation/research/faucet pre-funding."
  echo "    - Validators bond exactly virtual_stake (11 ZRN) at gentx."
  echo "    - Bootstrap pool mints 0.222 ZRN per whitelisted agent."
  echo ""
  echo "  Next:"
  echo "    ./scripts/genesis-ceremony.sh add-validator <name>"
  echo "    (then) seed bootstrap whitelist via tools/bootstrap-loader"
  echo "═══════════════════════════════════════════════════════════════"
}

# ── cmd_add_validator ────────────────────────────────────────────────────

cmd_add_validator() {
  local name="${1:-}"
  [ -n "${name}" ] || die "Usage: $0 add-validator <NAME>"

  echo "═══════════════════════════════════════════════════════════════"
  echo "  Zerone — Genesis Ceremony: ADD VALIDATOR '${name}'"
  echo "═══════════════════════════════════════════════════════════════"
  echo ""

  check_binary

  # ── Step 1: Validate ceremony home exists ────────────────────────────
  [ -d "${CEREMONY_HOME}" ] || die "Ceremony not initialized. Run: $0 init"
  [ -f "${CEREMONY_HOME}/config/genesis.json" ] || die "No genesis.json found. Run: $0 init"

  local val_home="${CEREMONY_HOME}/validators/${name}"

  if [ -d "${val_home}" ]; then
    die "Validator '${name}' already exists at ${val_home}"
  fi

  # ── Step 2: Generate unique consensus key in per-validator home ──────
  info "Initializing validator home..."
  ${BINARY} init "${name}" \
    --chain-id "${CHAIN_ID}" \
    --home "${val_home}" \
    --overwrite 2>/dev/null

  # ── Step 3: Copy coordinator genesis ─────────────────────────────────
  cp "${CEREMONY_HOME}/config/genesis.json" "${val_home}/config/genesis.json"

  # ── Step 4: Create validator account key in coordinator keyring ──────
  info "Creating validator key..."
  ${BINARY} keys add "${name}" \
    --keyring-backend ${KEYRING} \
    --home "${CEREMONY_HOME}" 2>/dev/null

  local val_addr
  val_addr=$(${BINARY} keys show "${name}" -a --keyring-backend ${KEYRING} --home "${CEREMONY_HOME}")

  # ── Step 5: Pre-fund validator with virtual_stake (gentx-bonded) ─────
  #
  # Commitment 20 doctrine: zero team allocation. The pre-fund here is
  # exactly virtual_stake (11 ZRN), which is the entire amount that gets
  # gentx-bonded into bonded_pool in the next step. The validator's
  # account balance ends at zero post-gentx — no privileged starting
  # balance held by any individual.
  info "Pre-funding validator with gentx bond: ${val_addr} (${VALIDATOR_BALANCE})"
  ${BINARY} add-genesis-account "${val_addr}" "${VALIDATOR_BALANCE}" \
    --home "${CEREMONY_HOME}"

  # ── Step 6: Copy updated genesis + keyring to validator home ─────────
  cp "${CEREMONY_HOME}/config/genesis.json" "${val_home}/config/genesis.json"
  cp -r "${CEREMONY_HOME}/keyring-test" "${val_home}/"

  # ── Step 7: Generate gentx ───────────────────────────────────────────
  info "Generating gentx (bond: ${VALIDATOR_STAKE} — entire pre-fund)..."
  mkdir -p "${CEREMONY_HOME}/config/gentx"

  ${BINARY} genesis gentx "${name}" "${VALIDATOR_STAKE}" \
    --chain-id "${CHAIN_ID}" \
    --keyring-backend ${KEYRING} \
    --home "${val_home}" \
    --moniker "${name}" \
    --commission-rate "0.10" \
    --commission-max-rate "0.20" \
    --commission-max-change-rate "0.01" \
    --output-document "${CEREMONY_HOME}/config/gentx/gentx-${name}.json" 2>/dev/null

  ok "Gentx generated"

  # ── Step 8: Save consensus key + node key for distribution ──────────
  local keys_dir="${CEREMONY_HOME}/validator-keys/${name}"
  mkdir -p "${keys_dir}"

  cp "${val_home}/config/priv_validator_key.json" "${keys_dir}/"
  cp "${val_home}/config/node_key.json" "${keys_dir}/"

  # Extract node ID
  local node_id
  node_id=$(${BINARY} tendermint show-node-id --home "${val_home}" 2>/dev/null || \
            ${BINARY} comet show-node-id --home "${val_home}" 2>/dev/null || \
            echo "unknown")
  echo "${node_id}" > "${keys_dir}/node_id.txt"

  # ── Summary ──────────────────────────────────────────────────────────
  echo ""
  echo "═══════════════════════════════════════════════════════════════"
  echo "  Validator '${name}' Added"
  echo "═══════════════════════════════════════════════════════════════"
  echo ""
  echo "  Address:       ${val_addr}"
  echo "  Pre-fund:      11 ZRN (virtual_stake)"
  echo "  Gentx bonded:  11 ZRN (entire pre-fund — account ends at zero)"
  echo "  Node ID:       ${node_id}"
  echo ""
  echo "  Key Locations:"
  echo "    priv_validator_key.json: ${keys_dir}/priv_validator_key.json"
  echo "    node_key.json:           ${keys_dir}/node_key.json"
  echo "    gentx:                   ${CEREMONY_HOME}/config/gentx/gentx-${name}.json"
  echo ""
  echo "  Next: add more validators or run: $0 finalize"
  echo "═══════════════════════════════════════════════════════════════"
}

# ── cmd_finalize ─────────────────────────────────────────────────────────

cmd_finalize() {
  echo "═══════════════════════════════════════════════════════════════"
  echo "  Zerone — Genesis Ceremony: FINALIZE"
  echo "═══════════════════════════════════════════════════════════════"
  echo ""

  check_binary

  [ -d "${CEREMONY_HOME}" ] || die "Ceremony not initialized. Run: $0 init"

  local gentx_dir="${CEREMONY_HOME}/config/gentx"
  [ -d "${gentx_dir}" ] || die "No gentx directory found. Add validators first."

  local gentx_count
  gentx_count=$(ls -1 "${gentx_dir}"/gentx-*.json 2>/dev/null | wc -l | tr -d ' ')
  [ "${gentx_count}" -gt 0 ] || die "No gentx files found. Add validators first."

  # ── Step 1: Collect all gentxs ───────────────────────────────────────
  info "Collecting ${gentx_count} gentxs..."
  ${BINARY} genesis collect-gentxs --home "${CEREMONY_HOME}" 2>/dev/null
  ok "Gentxs collected"

  # ── Step 2: Validate genesis ─────────────────────────────────────────
  info "Validating genesis..."
  validate_genesis "finalization"
  ok "Genesis validated"

  # ── Step 3: Print summary ────────────────────────────────────────────
  local genesis="${CEREMONY_HOME}/config/genesis.json"
  local genesis_hash
  genesis_hash=$(shasum -a 256 "${genesis}" | cut -d' ' -f1)
  local chain_id
  chain_id=$(jq -r '.chain_id' "${genesis}")
  local gen_time
  gen_time=$(jq -r '.genesis_time' "${genesis}")

  echo ""
  echo "═══════════════════════════════════════════════════════════════"
  echo "  Genesis Ceremony FINALIZED"
  echo "═══════════════════════════════════════════════════════════════"
  echo ""
  echo "  Chain ID:     ${chain_id}"
  echo "  Genesis Time: ${gen_time}"
  echo "  Validators:   ${gentx_count}"
  echo "  Genesis Hash: ${genesis_hash}"
  echo ""
  echo "  Next: ./scripts/genesis-ceremony.sh export"
  echo "═══════════════════════════════════════════════════════════════"
}

# ── cmd_export ───────────────────────────────────────────────────────────

cmd_export() {
  echo "═══════════════════════════════════════════════════════════════"
  echo "  Zerone — Genesis Ceremony: EXPORT"
  echo "═══════════════════════════════════════════════════════════════"
  echo ""

  check_binary

  local genesis="${CEREMONY_HOME}/config/genesis.json"
  [ -f "${genesis}" ] || die "No genesis.json found. Run init + finalize first."

  # ── Step 1: Copy genesis.json to project root ────────────────────────
  info "Exporting genesis.json to project root..."
  cp "${genesis}" "${PROJECT_ROOT}/genesis.json"
  ok "Exported: ${PROJECT_ROOT}/genesis.json"

  # ── Step 2: Print chain info ─────────────────────────────────────────
  local chain_id gen_time account_count genesis_hash
  chain_id=$(jq -r '.chain_id' "${genesis}")
  gen_time=$(jq -r '.genesis_time' "${genesis}")
  account_count=$(jq '.app_state.auth.accounts | length' "${genesis}")
  genesis_hash=$(shasum -a 256 "${genesis}" | cut -d' ' -f1)

  echo ""
  echo "  Chain ID:     ${chain_id}"
  echo "  Genesis Time: ${gen_time}"
  echo "  Accounts:     ${account_count}"
  echo "  Genesis Hash: ${genesis_hash}"

  # ── Step 3: Extract validator node IDs ───────────────────────────────
  local keys_dir="${CEREMONY_HOME}/validator-keys"
  if [ -d "${keys_dir}" ]; then
    echo ""
    echo "  Validator Node IDs:"
    for val_dir in "${keys_dir}"/*/; do
      local val_name
      val_name=$(basename "${val_dir}")
      local node_id_file="${val_dir}/node_id.txt"
      if [ -f "${node_id_file}" ]; then
        local nid
        nid=$(cat "${node_id_file}")
        echo "    ${val_name}: ${nid}"
      fi
    done
  fi

  # ── Step 4: Print distribution instructions ──────────────────────────
  echo ""
  echo "═══════════════════════════════════════════════════════════════"
  echo "  Distribution Instructions"
  echo "═══════════════════════════════════════════════════════════════"
  echo ""
  echo "  Each validator operator must receive and install:"
  echo ""
  echo "  1. genesis.json → ~/.zeroned/config/genesis.json"
  echo "     cp genesis.json ~/.zeroned/config/genesis.json"
  echo ""
  echo "  2. priv_validator_key.json → ~/.zeroned/config/priv_validator_key.json"
  echo "     (Distribute securely — NEVER share publicly)"
  echo ""
  echo "  3. node_key.json → ~/.zeroned/config/node_key.json"
  echo "     (Distribute securely)"
  echo ""
  echo "  4. Configure persistent_peers in ~/.zeroned/config/config.toml:"
  echo ""

  # Build persistent_peers string
  if [ -d "${keys_dir}" ]; then
    local peers=""
    for val_dir in "${keys_dir}"/*/; do
      local val_name
      val_name=$(basename "${val_dir}")
      local node_id_file="${val_dir}/node_id.txt"
      if [ -f "${node_id_file}" ]; then
        local nid
        nid=$(cat "${node_id_file}")
        if [ -n "${peers}" ]; then
          peers="${peers},"
        fi
        peers="${peers}${nid}@<${val_name}-ip>:26656"
      fi
    done
    echo "     persistent_peers = \"${peers}\""
    echo ""
    echo "     Replace <name-ip> with each validator's public IP address."
  fi

  echo ""
  echo "  5. Start the node:"
  echo "     zeroned start --minimum-gas-prices 0.025uzrn"
  echo ""
  echo "  Next: ./scripts/genesis-ceremony.sh countdown"
  echo "═══════════════════════════════════════════════════════════════"
}

# ── cmd_countdown ────────────────────────────────────────────────────────

cmd_countdown() {
  echo "═══════════════════════════════════════════════════════════════"
  echo "  Zerone — Genesis Ceremony: COUNTDOWN"
  echo "═══════════════════════════════════════════════════════════════"
  echo ""

  local genesis="${CEREMONY_HOME}/config/genesis.json"
  if [ ! -f "${genesis}" ]; then
    genesis="${PROJECT_ROOT}/genesis.json"
  fi
  [ -f "${genesis}" ] || die "No genesis.json found."

  # ── Step 1: Parse genesis time ───────────────────────────────────────
  local gen_time
  gen_time=$(jq -r '.genesis_time' "${genesis}")
  [ "${gen_time}" != "null" ] || die "Could not parse genesis_time from genesis.json"

  local chain_id
  chain_id=$(jq -r '.chain_id' "${genesis}")

  echo "  Chain:        ${chain_id}"
  echo "  Genesis Time: ${gen_time}"
  echo ""

  # Convert genesis time to epoch seconds
  local genesis_epoch
  if date --version >/dev/null 2>&1; then
    # GNU date
    genesis_epoch=$(date -d "${gen_time}" +%s)
  else
    # macOS/BSD date
    genesis_epoch=$(date -jf "%Y-%m-%dT%H:%M:%SZ" "${gen_time}" +%s 2>/dev/null || \
                    date -jf "%Y-%m-%dT%H:%M:%S" "${gen_time}" +%s 2>/dev/null || \
                    die "Could not parse genesis time: ${gen_time}")
  fi

  # ── Step 2: Instructions ─────────────────────────────────────────────
  echo "  ╔═══════════════════════════════════════════════════════════╗"
  echo "  ║  All validators should start their nodes NOW.            ║"
  echo "  ║                                                          ║"
  echo "  ║  zeroned start --minimum-gas-prices 0.025uzrn            ║"
  echo "  ║                                                          ║"
  echo "  ║  The chain will begin producing blocks at genesis time.  ║"
  echo "  ╚═══════════════════════════════════════════════════════════╝"
  echo ""

  # ── Step 3: Live countdown ───────────────────────────────────────────
  while true; do
    local now
    now=$(date +%s)
    local remaining=$((genesis_epoch - now))

    if [ ${remaining} -le 0 ]; then
      break
    fi

    local days=$((remaining / 86400))
    local hours=$(( (remaining % 86400) / 3600 ))
    local mins=$(( (remaining % 3600) / 60 ))
    local secs=$((remaining % 60))

    printf "\r  Countdown: %dd %02dh %02dm %02ds  " "${days}" "${hours}" "${mins}" "${secs}"
    sleep 1
  done

  # ── Step 4: Genesis reached ──────────────────────────────────────────
  echo ""
  echo ""
  echo "  ╔═══════════════════════════════════════════════════════════╗"
  echo "  ║                                                          ║"
  echo "  ║               GENESIS TIME REACHED!                      ║"
  echo "  ║                                                          ║"
  echo "  ╚═══════════════════════════════════════════════════════════╝"
  echo ""
  echo "  Verify block production:"
  echo "    zeroned status | jq '.sync_info.latest_block_height'"
  echo ""
  echo "  Expected: blocks should start appearing within seconds."
  echo "═══════════════════════════════════════════════════════════════"
}

# ── Main ─────────────────────────────────────────────────────────────────

case "${1:-help}" in
  init)
    cmd_init
    ;;
  add-validator)
    cmd_add_validator "${2:-}"
    ;;
  finalize)
    cmd_finalize
    ;;
  export)
    cmd_export
    ;;
  countdown)
    cmd_countdown
    ;;
  help|--help|-h)
    echo "Usage: $0 {init|add-validator <NAME>|finalize|export|countdown}"
    echo ""
    echo "Commands:"
    echo "  init                Initialize ceremony (build, patch params, create accounts)"
    echo "  add-validator NAME  Add a validator (generate keys, fund, create gentx)"
    echo "  finalize            Collect gentxs and validate genesis"
    echo "  export              Export genesis.json and distribution instructions"
    echo "  countdown           Live countdown to genesis time"
    echo ""
    echo "Full ceremony flow:"
    echo "  $0 init"
    echo "  $0 add-validator val1"
    echo "  $0 add-validator val2"
    echo "  $0 add-validator val3"
    echo "  $0 finalize"
    echo "  $0 export"
    echo "  $0 countdown"
    echo ""
    echo "State directory: ${CEREMONY_HOME}"
    echo "Chain ID:        ${CHAIN_ID}"
    ;;
  *)
    die "Unknown command: $1. Use --help for usage."
    ;;
esac
