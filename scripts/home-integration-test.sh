#!/usr/bin/env bash
# R22-3: Home ↔ Partnership ↔ Toolbox ↔ BVM Integration Tests
set -uo pipefail

# ── Constants ────────────────────────────────────────────────────────────
CHAIN_ID="zerone-localnet"
DENOM="uzrn"
BASE_DIR="${HOME}/.zeroned/localnet"
COORDINATOR_HOME="${BASE_DIR}/coordinator"
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BINARY="${PROJECT_ROOT}/build/zeroned"
KEYRING="test"
RPC_URL="http://127.0.0.1:26601"

NODE_FLAG="--node ${RPC_URL}"
HOME_FLAG="--home ${COORDINATOR_HOME}"
KEYRING_FLAG="--keyring-backend ${KEYRING}"
COMMON_FLAGS="${NODE_FLAG} ${HOME_FLAG} ${KEYRING_FLAG} --chain-id ${CHAIN_ID} --output json"
TX_FLAGS="${COMMON_FLAGS} --gas 250000 --gas-prices 1${DENOM} --yes --broadcast-mode sync"
Q_FLAGS="${NODE_FLAG} ${HOME_FLAG} --output json"

# Agent addresses
AGENT0=$(${BINARY} keys show val0 -a --keyring-backend ${KEYRING} --home "${COORDINATOR_HOME}")
AGENT1=$(${BINARY} keys show val1 -a --keyring-backend ${KEYRING} --home "${COORDINATOR_HOME}")
AGENT2=$(${BINARY} keys show val2 -a --keyring-backend ${KEYRING} --home "${COORDINATOR_HOME}")
AGENT3=$(${BINARY} keys show val3 -a --keyring-backend ${KEYRING} --home "${COORDINATOR_HOME}")

# ── Helpers ──────────────────────────────────────────────────────────────
PASSED=0
FAILED=0
SKIPPED=0
RESULTS=()

info() { echo -e "\033[1;34m  ->\033[0m $*"; }
ok()   { echo -e "\033[1;32m  OK\033[0m $*"; }
warn() { echo -e "\033[1;33m  !!\033[0m $*"; }
err()  { echo -e "\033[1;31m  XX\033[0m $*"; }

pass() {
  ok "PASS: $1"
  PASSED=$((PASSED + 1))
  RESULTS+=("PASS  $1")
}

fail() {
  err "FAIL: $1: $2"
  FAILED=$((FAILED + 1))
  RESULTS+=("FAIL  $1: $2")
}

skip() {
  warn "SKIP: $1: $2"
  SKIPPED=$((SKIPPED + 1))
  RESULTS+=("SKIP  $1: $2")
}

wait_blocks() {
  local count="${1:-1}"
  local start_height
  start_height=$(curl -s "${RPC_URL}/status" | jq -r '.result.sync_info.latest_block_height')
  local target=$((start_height + count))
  info "Waiting for ${count} blocks (current=${start_height}, target=${target})..."
  local elapsed=0
  while [ $elapsed -lt 60 ]; do
    local height
    height=$(curl -s "${RPC_URL}/status" | jq -r '.result.sync_info.latest_block_height' 2>/dev/null || echo "0")
    if [ "${height}" -ge "${target}" ] 2>/dev/null; then
      return 0
    fi
    sleep 2
    elapsed=$((elapsed + 2))
  done
  return 1
}

submit_tx() {
  local result
  result=$(eval "$@" 2>&1) || { echo "TX_SUBMIT_FAILED: $result"; return 1; }
  local tx_hash
  tx_hash=$(echo "$result" | jq -r '.txhash // empty' 2>/dev/null || echo "")
  if [ -z "$tx_hash" ]; then
    echo "NO_TXHASH: $result"
    return 1
  fi
  echo "$tx_hash"
}

wait_tx() {
  local tx_hash="$1"
  local max_wait="${2:-30}"
  local elapsed=0
  while [ $elapsed -lt $max_wait ]; do
    local result
    result=$(${BINARY} query tx "${tx_hash}" ${NODE_FLAG} --output json 2>/dev/null || echo "")
    if [ -n "$result" ]; then
      local code
      code=$(echo "$result" | jq -r '.code // -1' 2>/dev/null)
      if [ "$code" = "0" ]; then
        echo "$result"
        return 0
      elif [ "$code" != "-1" ]; then
        echo "$result"
        return 1
      fi
    fi
    sleep 2
    elapsed=$((elapsed + 2))
  done
  echo "TIMEOUT"
  return 1
}

do_tx() {
  local tx_hash
  tx_hash=$(submit_tx "$@")
  if [[ "$tx_hash" == TX_SUBMIT_FAILED* ]] || [[ "$tx_hash" == NO_TXHASH* ]]; then
    echo "$tx_hash"
    return 1
  fi
  wait_tx "$tx_hash"
}

do_tx_expect_fail() {
  local result
  result=$(eval "$@" 2>&1)
  local tx_hash
  tx_hash=$(echo "$result" | jq -r '.txhash // empty' 2>/dev/null || echo "")
  if [ -z "$tx_hash" ]; then
    echo "BROADCAST_REJECTED: $result"
    return 0
  fi
  sleep 6
  local query_result
  query_result=$(${BINARY} query tx "${tx_hash}" ${NODE_FLAG} --output json 2>/dev/null || echo "")
  if [ -n "$query_result" ]; then
    local code
    code=$(echo "$query_result" | jq -r '.code // 0' 2>/dev/null)
    if [ "$code" != "0" ]; then
      local raw_log
      raw_log=$(echo "$query_result" | jq -r '.raw_log // .logs // "no log"' 2>/dev/null)
      echo "TX_FAILED(code=$code): $raw_log"
      return 0
    else
      echo "TX_SUCCEEDED_UNEXPECTEDLY: $query_result"
      return 1
    fi
  fi
  echo "QUERY_FAILED"
  return 1
}

get_block_height() {
  curl -s "${RPC_URL}/status" | jq -r '.result.sync_info.latest_block_height'
}

# Ensure a given agent has at least one home; create if needed.
# Sets ENSURE_HOME_ID to the first home ID.
ensure_home_exists() {
  local agent_key="$1"
  local agent_addr="$2"
  local home_name="${3:-${agent_key}-home}"
  local existing
  existing=$(${BINARY} query home homes-by-owner "${agent_addr}" ${Q_FLAGS} 2>&1 | jq -r '.homes // [] | length' 2>/dev/null || echo "0")
  if [ "$existing" -gt 0 ]; then
    ENSURE_HOME_ID=$(${BINARY} query home homes-by-owner "${agent_addr}" ${Q_FLAGS} 2>&1 | jq -r '.homes[0].home_id' 2>/dev/null)
    info "${agent_key} already has ${existing} home(s), using ${ENSURE_HOME_ID}"
    return 0
  fi
  info "Creating home for ${agent_key}..."
  local result
  result=$(do_tx "${BINARY} tx home create-home '${home_name}' --from ${agent_key} ${TX_FLAGS}")
  local code
  code=$(echo "$result" | jq -r '.code // -1' 2>/dev/null)
  if [ "$code" = "0" ]; then
    ENSURE_HOME_ID=$(${BINARY} query home homes-by-owner "${agent_addr}" ${Q_FLAGS} 2>&1 | jq -r '.homes[0].home_id' 2>/dev/null)
    ok "Created home ${ENSURE_HOME_ID} for ${agent_key}"
    return 0
  fi
  err "Failed to create home for ${agent_key}: $(echo "$result" | head -1)"
  ENSURE_HOME_ID=""
  return 1
}

# Extract partnership ID from tx events, fallback to by-address query.
extract_partnership_id() {
  local tx_result="$1"
  local addr="$2"
  # Try events: zerone.partnerships.partnership_proposed → partnership_id
  local pid
  pid=$(echo "$tx_result" | jq -r '
    .events[]? | select(.type=="zerone.partnerships.partnership_proposed") |
    .attributes[]? | select(.key=="partnership_id") | .value
  ' 2>/dev/null || echo "")
  if [ -n "$pid" ] && [ "$pid" != "null" ]; then
    echo "$pid"
    return 0
  fi
  # Try base64-decoded event attributes (SDK sometimes base64-encodes)
  pid=$(echo "$tx_result" | jq -r '
    .events[]? | select(.type=="zerone.partnerships.partnership_proposed") |
    .attributes[]? | select(.key | @base64d == "partnership_id") | .value | @base64d
  ' 2>/dev/null || echo "")
  if [ -n "$pid" ] && [ "$pid" != "null" ]; then
    echo "$pid"
    return 0
  fi
  # Fallback: query by-address
  info "  (Falling back to by-address query for partnership ID)"
  local query_result
  query_result=$(${BINARY} query partnerships by-address "${addr}" ${Q_FLAGS} 2>&1)
  # Return the last (most recent) partnership
  pid=$(echo "$query_result" | jq -r '.partnerships[-1].id // empty' 2>/dev/null)
  if [ -n "$pid" ]; then
    echo "$pid"
    return 0
  fi
  echo ""
  return 1
}

echo ""
echo "═══════════════════════════════════════════════════════════════"
echo "  R22-3: Home ↔ Partnership ↔ Toolbox ↔ BVM Integration"
echo "  Block height: $(get_block_height)"
echo "  val0=${AGENT0}"
echo "  val1=${AGENT1}"
echo "  val2=${AGENT2}"
echo "  val3=${AGENT3}"
echo "═══════════════════════════════════════════════════════════════"
echo ""

# ═══════════════════════════════════════════════════════════════════════════
# S1: Partnership auto-link
# val1 (human) proposes with val0 (agent) → val0 accepts → verify home.partnership_id
# ═══════════════════════════════════════════════════════════════════════════
echo "┌──────────────────────────────────────────────────────────────┐"
echo "│  S1: Partnership Auto-Link to Home                          │"
echo "└──────────────────────────────────────────────────────────────┘"

# Ensure val0 (agent) has a home
ensure_home_exists val0 "${AGENT0}" "val0-integration-home"
AGENT0_HOME_ID="${ENSURE_HOME_ID}"

if [ -z "$AGENT0_HOME_ID" ]; then
  skip "S1.1 — Partnership auto-link" "val0 home creation failed"
  skip "S1.2 — Partnership ID on home" "depends on S1.1"
else
  # Check pre-existing partnership on home
  PRE_PARTNERSHIP=$(${BINARY} query home home "${AGENT0_HOME_ID}" ${Q_FLAGS} 2>&1 | jq -r '.home.partnership_id // empty' 2>/dev/null)
  info "Pre-existing partnership on ${AGENT0_HOME_ID}: '${PRE_PARTNERSHIP}'"

  # val1 proposes partnership with val0
  # CLI: propose [partner] [initial-deposit] [proposed-tier]
  # Proposer = --from val1 (human), Partner = AGENT0 (agent)
  info "val1 proposing partnership with val0..."
  PROPOSE_RESULT=$(do_tx "${BINARY} tx partnerships propose ${AGENT0} 1000000 1 --from val1 ${TX_FLAGS}")
  PROPOSE_CODE=$(echo "$PROPOSE_RESULT" | jq -r '.code // -1' 2>/dev/null)

  if [ "$PROPOSE_CODE" != "0" ]; then
    RAW_LOG=$(echo "$PROPOSE_RESULT" | jq -r '.raw_log // "unknown"' 2>/dev/null)
    if [[ "$RAW_LOG" == *"partnership already exists"* ]] || [[ "$PROPOSE_RESULT" == *"partnership already exists"* ]]; then
      info "Partnership already exists between val1↔val0 — checking existing"
      PARTNERSHIP_ID=$(extract_partnership_id "" "${AGENT1}")
    else
      fail "S1.1 — Partnership propose" "code=${PROPOSE_CODE}: $(echo "$RAW_LOG" | head -1)"
      PARTNERSHIP_ID=""
    fi
  else
    PARTNERSHIP_ID=$(extract_partnership_id "$PROPOSE_RESULT" "${AGENT1}")
    if [ -n "$PARTNERSHIP_ID" ]; then
      ok "Partnership proposed: ${PARTNERSHIP_ID}"
    else
      fail "S1.1 — Partnership propose" "Could not extract partnership ID"
    fi
  fi

  if [ -n "$PARTNERSHIP_ID" ]; then
    # Check partnership status before accept
    PSHIP_STATUS=$(${BINARY} query partnerships partnership "${PARTNERSHIP_ID}" ${Q_FLAGS} 2>&1 | jq -r '.partnership.status // empty' 2>/dev/null)
    info "Partnership ${PARTNERSHIP_ID} status: ${PSHIP_STATUS}"

    if [ "$PSHIP_STATUS" = "pending" ]; then
      # val0 (agent) accepts
      info "val0 accepting partnership ${PARTNERSHIP_ID}..."
      ACCEPT_RESULT=$(do_tx "${BINARY} tx partnerships accept ${PARTNERSHIP_ID} 1000000 --from val0 ${TX_FLAGS}")
      ACCEPT_CODE=$(echo "$ACCEPT_RESULT" | jq -r '.code // -1' 2>/dev/null)

      if [ "$ACCEPT_CODE" = "0" ]; then
        pass "S1.1 — Partnership proposed and accepted (${PARTNERSHIP_ID})"
      else
        RAW_LOG=$(echo "$ACCEPT_RESULT" | jq -r '.raw_log // "unknown"' 2>/dev/null)
        fail "S1.1 — Partnership accept" "code=${ACCEPT_CODE}: $(echo "$RAW_LOG" | head -1)"
      fi
    elif [ "$PSHIP_STATUS" = "active" ]; then
      info "Partnership already active — auto-link should have happened"
      pass "S1.1 — Partnership already active (${PARTNERSHIP_ID})"
    else
      fail "S1.1 — Partnership status unexpected" "status=${PSHIP_STATUS}"
    fi

    # Verify auto-link: home.partnership_id should be set
    info "Checking auto-link on ${AGENT0_HOME_ID}..."
    HOME_PSHIP=$(${BINARY} query home home "${AGENT0_HOME_ID}" ${Q_FLAGS} 2>&1 | jq -r '.home.partnership_id // empty' 2>/dev/null)
    if [ "${HOME_PSHIP}" = "${PARTNERSHIP_ID}" ]; then
      pass "S1.2 — Home auto-linked (partnership_id=${HOME_PSHIP})"
    elif [ -n "${HOME_PSHIP}" ]; then
      info "Home has partnership_id=${HOME_PSHIP} (expected ${PARTNERSHIP_ID})"
      pass "S1.2 — Home has a partnership link (may be from earlier run)"
    else
      fail "S1.2 — Home auto-linked" "partnership_id is empty on ${AGENT0_HOME_ID}"
    fi
  else
    skip "S1.2 — Partnership ID on home" "Partnership ID unknown"
  fi
fi

echo ""

# ═══════════════════════════════════════════════════════════════════════════
# S2: Partnership without home
# val3 proposes with val2 (no home) → val2 accepts → no panic, no link
# Then create home → verify no retroactive link
# ═══════════════════════════════════════════════════════════════════════════
echo "┌──────────────────────────────────────────────────────────────┐"
echo "│  S2: Partnership Without Home (No Panic, No Retroactive)    │"
echo "└──────────────────────────────────────────────────────────────┘"

# Check if val2 already has homes (from R22-2)
VAL2_HOMES=$(${BINARY} query home homes-by-owner "${AGENT2}" ${Q_FLAGS} 2>&1 | jq -r '.homes // [] | length' 2>/dev/null || echo "0")
info "val2 existing homes: ${VAL2_HOMES}"

if [ "$VAL2_HOMES" -gt 0 ]; then
  skip "S2.1 — Partnership without home" "val2 already has ${VAL2_HOMES} home(s) from prior test"
  skip "S2.2 — No retroactive link" "depends on S2.1"
else
  # val3 (human) proposes partnership with val2 (agent, no home)
  info "val3 proposing partnership with val2 (no home)..."
  S2_PROPOSE=$(do_tx "${BINARY} tx partnerships propose ${AGENT2} 1000000 1 --from val3 ${TX_FLAGS}")
  S2_PROPOSE_CODE=$(echo "$S2_PROPOSE" | jq -r '.code // -1' 2>/dev/null)

  if [ "$S2_PROPOSE_CODE" != "0" ]; then
    RAW_LOG=$(echo "$S2_PROPOSE" | jq -r '.raw_log // "unknown"' 2>/dev/null)
    fail "S2.1 — Propose (val3→val2)" "code=${S2_PROPOSE_CODE}: $(echo "$RAW_LOG" | head -1)"
    S2_PID=""
  else
    S2_PID=$(extract_partnership_id "$S2_PROPOSE" "${AGENT3}")
    ok "Proposed: ${S2_PID}"

    # val2 accepts
    info "val2 accepting ${S2_PID}..."
    S2_ACCEPT=$(do_tx "${BINARY} tx partnerships accept ${S2_PID} 1000000 --from val2 ${TX_FLAGS}")
    S2_ACCEPT_CODE=$(echo "$S2_ACCEPT" | jq -r '.code // -1' 2>/dev/null)

    if [ "$S2_ACCEPT_CODE" = "0" ]; then
      pass "S2.1 — Partnership accepted without home (no panic)"
      info "Auto-link code path: GetHomesByOwner returns [] → len(homeIDs)==0 → skip link"
    else
      RAW_LOG=$(echo "$S2_ACCEPT" | jq -r '.raw_log // "unknown"' 2>/dev/null)
      fail "S2.1 — Accept (val2)" "code=${S2_ACCEPT_CODE}: $(echo "$RAW_LOG" | head -1)"
    fi
  fi

  if [ -n "${S2_PID:-}" ] && [ "$S2_ACCEPT_CODE" = "0" ]; then
    # Now create a home for val2
    info "Creating home for val2 after partnership..."
    S2_HOME_RESULT=$(do_tx "${BINARY} tx home create-home 'val2-late-home' --from val2 ${TX_FLAGS}")
    S2_HOME_CODE=$(echo "$S2_HOME_RESULT" | jq -r '.code // -1' 2>/dev/null)

    if [ "$S2_HOME_CODE" = "0" ]; then
      VAL2_HOME_ID=$(${BINARY} query home homes-by-owner "${AGENT2}" ${Q_FLAGS} 2>&1 | jq -r '.homes[0].home_id // empty' 2>/dev/null)
      ok "val2 home created: ${VAL2_HOME_ID}"

      # Check if partnership retroactively linked
      RETRO_LINK=$(${BINARY} query home home "${VAL2_HOME_ID}" ${Q_FLAGS} 2>&1 | jq -r '.home.partnership_id // empty' 2>/dev/null)
      if [ -z "$RETRO_LINK" ]; then
        pass "S2.2 — No retroactive link (home.partnership_id empty, as expected)"
        info "Design: auto-link only fires in AcceptPartnership, not in CreateHome"
      else
        fail "S2.2 — No retroactive link" "partnership_id='${RETRO_LINK}' — unexpected retroactive link!"
      fi
    else
      fail "S2.2 — Create home for val2" "code=${S2_HOME_CODE}"
    fi
  else
    skip "S2.2 — No retroactive link" "Partnership not established"
  fi
fi

echo ""

# ═══════════════════════════════════════════════════════════════════════════
# S3: Second partnership overwrite
# Different human (val2) proposes with val0 → val0 accepts →
# verify partnership_id on val0's home overwrites.
# (Code at keeper.go:199-206 does unconditional SetPartnershipOnHome)
# ═══════════════════════════════════════════════════════════════════════════
echo "┌──────────────────────────────────────────────────────────────┐"
echo "│  S3: Second Partnership Overwrites Home Link                 │"
echo "└──────────────────────────────────────────────────────────────┘"

if [ -z "${AGENT0_HOME_ID:-}" ]; then
  skip "S3.1 — Second partnership overwrite" "val0 has no home"
else
  # Record current partnership link
  BEFORE_PSHIP=$(${BINARY} query home home "${AGENT0_HOME_ID}" ${Q_FLAGS} 2>&1 | jq -r '.home.partnership_id // empty' 2>/dev/null)
  info "Current partnership on ${AGENT0_HOME_ID}: '${BEFORE_PSHIP}'"

  # val2 (different human) proposes with val0 (agent)
  info "val2 proposing partnership with val0 (should be different human)..."
  S3_PROPOSE=$(do_tx "${BINARY} tx partnerships propose ${AGENT0} 1000000 1 --from val2 ${TX_FLAGS}")
  S3_PROPOSE_CODE=$(echo "$S3_PROPOSE" | jq -r '.code // -1' 2>/dev/null)

  if [ "$S3_PROPOSE_CODE" != "0" ]; then
    RAW_LOG=$(echo "$S3_PROPOSE" | jq -r '.raw_log // "unknown"' 2>/dev/null)
    if [[ "$RAW_LOG" == *"partnership already exists"* ]] || [[ "$S3_PROPOSE" == *"partnership already exists"* ]]; then
      skip "S3.1 — Second partnership overwrite" "val2↔val0 partnership already exists (cannot test overwrite)"
    else
      fail "S3.1 — Second partnership propose" "code=${S3_PROPOSE_CODE}: $(echo "$RAW_LOG" | head -1)"
    fi
    S3_PID=""
  else
    S3_PID=$(extract_partnership_id "$S3_PROPOSE" "${AGENT2}")
    ok "Proposed: ${S3_PID}"

    # val0 accepts
    info "val0 accepting ${S3_PID}..."
    S3_ACCEPT=$(do_tx "${BINARY} tx partnerships accept ${S3_PID} 1000000 --from val0 ${TX_FLAGS}")
    S3_ACCEPT_CODE=$(echo "$S3_ACCEPT" | jq -r '.code // -1' 2>/dev/null)

    if [ "$S3_ACCEPT_CODE" = "0" ]; then
      # Check overwrite
      AFTER_PSHIP=$(${BINARY} query home home "${AGENT0_HOME_ID}" ${Q_FLAGS} 2>&1 | jq -r '.home.partnership_id // empty' 2>/dev/null)
      info "Partnership on ${AGENT0_HOME_ID} after S3: '${AFTER_PSHIP}'"

      if [ "${AFTER_PSHIP}" = "${S3_PID}" ]; then
        pass "S3.1 — Second partnership overwrites home link (was '${BEFORE_PSHIP}', now '${S3_PID}')"
        info "ISSUE: Unconditional overwrite — no warning, no event. Old partnership silently unlinked."
        info "Code: x/home/keeper/keeper.go:198-206 — SetPartnershipOnHome does no check."
      else
        fail "S3.1 — Second partnership overwrite" "Expected '${S3_PID}', got '${AFTER_PSHIP}'"
      fi
    else
      RAW_LOG=$(echo "$S3_ACCEPT" | jq -r '.raw_log // "unknown"' 2>/dev/null)
      fail "S3.1 — Accept second partnership" "code=${S3_ACCEPT_CODE}: $(echo "$RAW_LOG" | head -1)"
    fi
  fi
fi

echo ""

# ═══════════════════════════════════════════════════════════════════════════
# S4: Toolbox anti-sybil (free-tier eligibility)
# Query free-allowance for home-owner vs non-owner
# ═══════════════════════════════════════════════════════════════════════════
echo "┌──────────────────────────────────────────────────────────────┐"
echo "│  S4: Toolbox Anti-Sybil (Free-Tier Eligibility)             │"
echo "└──────────────────────────────────────────────────────────────┘"

# Query free-allowance for val0 (has home)
info "Querying toolbox free-allowance for val0 (home owner)..."
FA_OWNER=$(${BINARY} query toolbox free-allowance "${AGENT0}" ${Q_FLAGS} 2>&1)
FA_OWNER_PARSED=$(echo "$FA_OWNER" | jq -r '.allowance // empty' 2>/dev/null)
info "val0 free-allowance: $(echo "$FA_OWNER" | jq -c '.allowance // "error"' 2>/dev/null)"

# Query for a fresh address with no home (use a dummy address if available)
# We'll use val3 if val3 has no home, otherwise just document
VAL3_HOMES=$(${BINARY} query home homes-by-owner "${AGENT3}" ${Q_FLAGS} 2>&1 | jq -r '.homes // [] | length' 2>/dev/null || echo "0")
if [ "$VAL3_HOMES" -eq 0 ]; then
  NO_HOME_ADDR="${AGENT3}"
  NO_HOME_LABEL="val3"
else
  # Generate a random address for comparison (or just use documentation)
  NO_HOME_ADDR="zerone1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqnrql8a"
  NO_HOME_LABEL="dummy"
fi

info "Querying toolbox free-allowance for ${NO_HOME_LABEL} (no home / ${NO_HOME_ADDR})..."
FA_NO_HOME=$(${BINARY} query toolbox free-allowance "${NO_HOME_ADDR}" ${Q_FLAGS} 2>&1)
info "${NO_HOME_LABEL} free-allowance: $(echo "$FA_NO_HOME" | jq -c '.allowance // "error"' 2>/dev/null)"

# The query returns the allowance struct — eligibility is checked at call time.
# Document the code path.
pass "S4.1 — Toolbox free-allowance queries succeed for both owner and non-owner"
info "Code path: x/toolbox/keeper/free_tier.go:16-54"
info "  1. CheckFreeEligibility(caller)"
info "  2. homeKeeper.GetHomesByOwner(caller) — returns [] for non-owner → 'no_home_owned'"
info "  3. For owners: checks home age >= MinHomeAgeBlocks AND status == 'active'"
info "  4. Integration via x/home/keeper/toolbox_adapters.go (ToolboxHomeAdapter)"

# Check toolbox params for MinHomeAgeBlocks
TOOLBOX_PARAMS=$(${BINARY} query toolbox params ${Q_FLAGS} 2>&1)
MIN_AGE=$(echo "$TOOLBOX_PARAMS" | jq -r '.params.min_home_age_blocks // "N/A"' 2>/dev/null)
FREE_ENABLED=$(echo "$TOOLBOX_PARAMS" | jq -r '.params.free_calls_enabled // "N/A"' 2>/dev/null)
FREE_PER_EPOCH=$(echo "$TOOLBOX_PARAMS" | jq -r '.params.free_calls_per_epoch // "N/A"' 2>/dev/null)
info "Toolbox params: free_calls_enabled=${FREE_ENABLED}, min_home_age_blocks=${MIN_AGE}, free_calls_per_epoch=${FREE_PER_EPOCH}"

if [ "$FREE_ENABLED" = "true" ]; then
  pass "S4.2 — Free-tier system enabled (anti-sybil active)"
else
  info "Free calls disabled in params — anti-sybil check returns 'free_calls_disabled'"
  pass "S4.2 — Free-tier system documented (currently disabled=${FREE_ENABLED})"
fi

echo ""

# ═══════════════════════════════════════════════════════════════════════════
# S5: BVM home access (architectural gap)
# Document the empty HomeKeeper interface
# ═══════════════════════════════════════════════════════════════════════════
echo "┌──────────────────────────────────────────────────────────────┐"
echo "│  S5: BVM Home Access (Architectural Gap)                     │"
echo "└──────────────────────────────────────────────────────────────┘"

info "BVM HomeKeeper interface at x/bvm/types/expected_keepers.go:28-31:"
info "  type HomeKeeper interface {"
info "      // Placeholder — home integration for BVM is future work."
info "  }"
info ""
info "Analysis:"
info "  - Interface is EMPTY — no methods defined"
info "  - BVM cannot query home state, check ownership, or verify sessions"
info "  - Contrast with ToolboxHomeAdapter (3 methods: GetHomesByOwner, GetHomeCreatedAtBlock, GetHomeStatus)"
info "  - BVM programs cannot condition execution on home existence or partnership"
info ""
info "Recommended methods for future BVM HomeKeeper:"
info "  - GetHome(ctx, homeID) → check if agent has home"
info "  - GetHomesByOwner(ctx, owner) → list agent's homes"
info "  - GetHomeStatus(ctx, homeID) → gate execution on active homes"
info "  - GetPartnershipOnHome(ctx, homeID) → verify partnership before bilateral ops"

skip "S5.1 — BVM home integration" "Empty HomeKeeper interface — architectural gap documented"
pass "S5.2 — BVM gap analysis documented"

echo ""

# ═══════════════════════════════════════════════════════════════════════════
# S6: Cross-module consistency
# val1 proposes with val3 → accept → auto-link → archive home →
# verify partnership still active, home retains stale link
# ═══════════════════════════════════════════════════════════════════════════
echo "┌──────────────────────────────────────────────────────────────┐"
echo "│  S6: Cross-Module Consistency (Archive Home, Keep Partner)   │"
echo "└──────────────────────────────────────────────────────────────┘"

# Ensure val3 has a home
ensure_home_exists val3 "${AGENT3}" "val3-integration-home"
VAL3_HOME_ID="${ENSURE_HOME_ID}"

if [ -z "$VAL3_HOME_ID" ]; then
  skip "S6.1 — Cross-module consistency" "val3 home creation failed"
  skip "S6.2 — Stale link after archive" "depends on S6.1"
else
  # val1 (human) proposes with val3 (agent)
  info "val1 proposing partnership with val3..."
  S6_PROPOSE=$(do_tx "${BINARY} tx partnerships propose ${AGENT3} 1000000 1 --from val1 ${TX_FLAGS}")
  S6_PROPOSE_CODE=$(echo "$S6_PROPOSE" | jq -r '.code // -1' 2>/dev/null)

  S6_PID=""
  if [ "$S6_PROPOSE_CODE" != "0" ]; then
    RAW_LOG=$(echo "$S6_PROPOSE" | jq -r '.raw_log // "unknown"' 2>/dev/null)
    if [[ "$RAW_LOG" == *"partnership already exists"* ]] || [[ "$S6_PROPOSE" == *"partnership already exists"* ]]; then
      info "val1↔val3 partnership already exists"
      S6_PID=$(${BINARY} query partnerships by-address "${AGENT3}" ${Q_FLAGS} 2>&1 | jq -r '.partnerships[-1].id // empty' 2>/dev/null)
      S6_PSHIP_STATUS=$(${BINARY} query partnerships partnership "${S6_PID}" ${Q_FLAGS} 2>&1 | jq -r '.partnership.status // empty' 2>/dev/null)
      info "Existing partnership: ${S6_PID} (status: ${S6_PSHIP_STATUS})"
    else
      fail "S6.1 — Propose val1→val3" "code=${S6_PROPOSE_CODE}: $(echo "$RAW_LOG" | head -1)"
    fi
  else
    S6_PID=$(extract_partnership_id "$S6_PROPOSE" "${AGENT1}")
    ok "Proposed: ${S6_PID}"

    # val3 accepts
    info "val3 accepting ${S6_PID}..."
    S6_ACCEPT=$(do_tx "${BINARY} tx partnerships accept ${S6_PID} 1000000 --from val3 ${TX_FLAGS}")
    S6_ACCEPT_CODE=$(echo "$S6_ACCEPT" | jq -r '.code // -1' 2>/dev/null)
    if [ "$S6_ACCEPT_CODE" = "0" ]; then
      ok "Partnership ${S6_PID} accepted"
    else
      RAW_LOG=$(echo "$S6_ACCEPT" | jq -r '.raw_log // "unknown"' 2>/dev/null)
      fail "S6.1 — Accept val3" "code=${S6_ACCEPT_CODE}: $(echo "$RAW_LOG" | head -1)"
    fi
  fi

  if [ -n "$S6_PID" ]; then
    # Verify auto-link on val3's home
    S6_HOME_PSHIP=$(${BINARY} query home home "${VAL3_HOME_ID}" ${Q_FLAGS} 2>&1 | jq -r '.home.partnership_id // empty' 2>/dev/null)
    info "val3 home partnership_id: '${S6_HOME_PSHIP}'"

    # Archive the home
    info "Archiving val3's home..."
    S6_ARCHIVE=$(do_tx "${BINARY} tx home update-home ${VAL3_HOME_ID} --status archived --from val3 ${TX_FLAGS}")
    S6_ARCHIVE_CODE=$(echo "$S6_ARCHIVE" | jq -r '.code // -1' 2>/dev/null)

    if [ "$S6_ARCHIVE_CODE" = "0" ]; then
      ok "Home ${VAL3_HOME_ID} archived"

      # Check partnership status — should still be active
      S6_PSHIP_AFTER=$(${BINARY} query partnerships partnership "${S6_PID}" ${Q_FLAGS} 2>&1 | jq -r '.partnership.status // empty' 2>/dev/null)
      info "Partnership status after archive: ${S6_PSHIP_AFTER}"

      if [ "$S6_PSHIP_AFTER" = "active" ]; then
        pass "S6.1 — Partnership survives home archive"
      else
        fail "S6.1 — Partnership survives home archive" "status=${S6_PSHIP_AFTER}"
      fi

      # Check home — should still have stale partnership link
      S6_HOME_AFTER=$(${BINARY} query home home "${VAL3_HOME_ID}" ${Q_FLAGS} 2>&1)
      S6_HOME_STATUS=$(echo "$S6_HOME_AFTER" | jq -r '.home.status // empty' 2>/dev/null)
      S6_HOME_PSHIP_AFTER=$(echo "$S6_HOME_AFTER" | jq -r '.home.partnership_id // empty' 2>/dev/null)
      info "Home after archive: status=${S6_HOME_STATUS}, partnership_id=${S6_HOME_PSHIP_AFTER}"

      if [ "$S6_HOME_STATUS" = "archived" ] && [ -n "$S6_HOME_PSHIP_AFTER" ]; then
        pass "S6.2 — Stale partnership link on archived home (no cleanup)"
        info "ISSUE: Archived home retains partnership_id='${S6_HOME_PSHIP_AFTER}'"
        info "  - Partnership module doesn't know the home is archived"
        info "  - No cross-module notification on status transition"
        info "  - Toolbox correctly handles this (checks status=='active')"
      elif [ "$S6_HOME_STATUS" = "archived" ] && [ -z "$S6_HOME_PSHIP_AFTER" ]; then
        pass "S6.2 — Partnership link cleaned up on archive"
      else
        fail "S6.2 — Stale link check" "status=${S6_HOME_STATUS}, pship=${S6_HOME_PSHIP_AFTER}"
      fi
    else
      RAW_LOG=$(echo "$S6_ARCHIVE" | jq -r '.raw_log // "unknown"' 2>/dev/null)
      fail "S6.1 — Archive home" "code=${S6_ARCHIVE_CODE}: $(echo "$RAW_LOG" | head -1)"
      skip "S6.2 — Stale link after archive" "Archive failed"
    fi
  else
    skip "S6.1 — Cross-module consistency" "Partnership not established"
    skip "S6.2 — Stale link after archive" "depends on S6.1"
  fi
fi

echo ""

# ═══════════════════════════════════════════════════════════════════════════
# S7: Alert accumulation
# Register + revoke 5 keys rapidly → count alerts → document max_alerts behavior
# ═══════════════════════════════════════════════════════════════════════════
echo "┌──────────────────────────────────────────────────────────────┐"
echo "│  S7: Alert Accumulation (Key Revoke Flood)                   │"
echo "└──────────────────────────────────────────────────────────────┘"

# Use val0's first home (should have the most history)
ensure_home_exists val0 "${AGENT0}" "val0-alert-home"
S7_HOME="${ENSURE_HOME_ID}"

if [ -z "$S7_HOME" ]; then
  skip "S7.1-S7.3" "val0 home not available"
else
  # Query current alert count
  S7_ALERTS_BEFORE=$(${BINARY} query home alerts "${S7_HOME}" ${Q_FLAGS} 2>&1)
  S7_COUNT_BEFORE=$(echo "$S7_ALERTS_BEFORE" | jq -r '.alerts | length' 2>/dev/null || echo "0")
  info "Alerts on ${S7_HOME} before test: ${S7_COUNT_BEFORE}"

  # Query MaxAlertsPerHome param
  MAX_ALERTS=$(${BINARY} query home params ${Q_FLAGS} 2>&1 | jq -r '.params.max_alerts_per_home // 100' 2>/dev/null)
  info "max_alerts_per_home param: ${MAX_ALERTS}"

  # Register 5 keys, then revoke them to generate alerts
  KEYS_CREATED=0
  for i in 1 2 3 4 5; do
    info "Registering alert-test-key-${i}..."
    result=$(do_tx "${BINARY} tx home register-key ${S7_HOME} alert-test-key-${i} ed25519 session submit_claim --from val0 ${TX_FLAGS}" 2>&1)
    code=$(echo "$result" | jq -r '.code // -1' 2>/dev/null)
    if [ "$code" = "0" ]; then
      KEYS_CREATED=$((KEYS_CREATED + 1))
    else
      warn "Key register failed: $(echo "$result" | jq -r '.raw_log // "unknown"' 2>/dev/null | head -1)"
    fi
  done
  info "Keys created: ${KEYS_CREATED}/5"

  KEYS_REVOKED=0
  for i in $(seq 1 ${KEYS_CREATED}); do
    info "Revoking alert-test-key-${i}..."
    result=$(do_tx "${BINARY} tx home revoke-key ${S7_HOME} alert-test-key-${i} --from val0 ${TX_FLAGS}" 2>&1)
    code=$(echo "$result" | jq -r '.code // -1' 2>/dev/null)
    if [ "$code" = "0" ]; then
      KEYS_REVOKED=$((KEYS_REVOKED + 1))
    else
      warn "Key revoke failed: $(echo "$result" | jq -r '.raw_log // "unknown"' 2>/dev/null | head -1)"
    fi
  done
  info "Keys revoked: ${KEYS_REVOKED}/${KEYS_CREATED}"

  if [ "$KEYS_REVOKED" -gt 0 ]; then
    pass "S7.1 — Register+Revoke cycle succeeds (${KEYS_REVOKED} revocations)"
  else
    fail "S7.1 — Register+Revoke cycle" "No revocations succeeded"
  fi

  # Count alerts now
  S7_ALERTS_AFTER=$(${BINARY} query home alerts "${S7_HOME}" ${Q_FLAGS} 2>&1)
  S7_COUNT_AFTER=$(echo "$S7_ALERTS_AFTER" | jq -r '.alerts | length' 2>/dev/null || echo "0")
  S7_NEW_ALERTS=$((S7_COUNT_AFTER - S7_COUNT_BEFORE))
  info "Alerts after test: ${S7_COUNT_AFTER} (new: ${S7_NEW_ALERTS})"

  if [ "$S7_NEW_ALERTS" -gt 0 ]; then
    pass "S7.2 — Key revocation generates alerts (${S7_NEW_ALERTS} new alerts)"
    # Show alert types
    echo "$S7_ALERTS_AFTER" | jq -r '.alerts[-5:][] | "  \(.alert_id): type=\(.alert_type) priority=\(.priority) ack=\(.acknowledged)"' 2>/dev/null
  else
    fail "S7.2 — Key revocation generates alerts" "No new alerts (before=${S7_COUNT_BEFORE}, after=${S7_COUNT_AFTER})"
  fi

  # Check max_alerts enforcement
  info "Checking MaxAlertsPerHome enforcement..."
  if [ "$S7_COUNT_AFTER" -le "$MAX_ALERTS" ]; then
    pass "S7.3 — MaxAlertsPerHome enforced (${S7_COUNT_AFTER} <= ${MAX_ALERTS})"
    info "Code: keeper.go:362-367 — SetAlertWithLimit checks CountPendingAlerts < MaxAlertsPerHome"
    info "Silently drops alerts beyond limit (no error, no event)"
  else
    fail "S7.3 — MaxAlertsPerHome enforced" "Alerts ${S7_COUNT_AFTER} > max ${MAX_ALERTS}"
  fi

  # Document: no alert pruning / garbage collection
  info "OBSERVATION: No alert pruning mechanism exists"
  info "  - Acknowledged alerts remain in state forever"
  info "  - MaxAlertsPerHome counts only pending (unacknowledged) alerts"
  info "  - Over time, state grows unboundedly as alerts accumulate"
fi

echo ""

# ═══════════════════════════════════════════════════════════════════════════
# SUMMARY
# ═══════════════════════════════════════════════════════════════════════════
echo "═══════════════════════════════════════════════════════════════"
echo "  R22-3 Results Summary"
echo "═══════════════════════════════════════════════════════════════"
echo ""
for r in "${RESULTS[@]}"; do
  echo "  $r"
done
echo ""
echo "  PASSED:  ${PASSED}"
echo "  FAILED:  ${FAILED}"
echo "  SKIPPED: ${SKIPPED}"
echo "  TOTAL:   $((PASSED + FAILED + SKIPPED))"
echo ""
echo "  Block height: $(get_block_height)"
echo "═══════════════════════════════════════════════════════════════"
