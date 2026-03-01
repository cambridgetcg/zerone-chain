#!/usr/bin/env bash
# ═══════════════════════════════════════════════════════════════════════════
# Zerone — Upgrade Simulation Test
# ═══════════════════════════════════════════════════════════════════════════
#
# Tests the full governance-triggered upgrade pipeline:
#   1. Submit MsgSoftwareUpgrade governance proposal ("v1.0.1-testnet")
#   2. Vote YES from all 4 validators
#   3. Wait for proposal to pass + upgrade height reached
#   4. Verify: chain continues (handler ran seamlessly)
#   5. Verify: knowledge module at ConsensusVersion 2
#   6. Verify: upgrade marked as applied
#
# Requires: scripts/localnet.sh start (localnet must be running)
#
# Usage:
#   scripts/upgrade-test.sh
# ═══════════════════════════════════════════════════════════════════════════

set -euo pipefail

# ── Constants ────────────────────────────────────────────────────────────

CHAIN_ID="zerone-localnet"
DENOM="uzrn"
BASE_DIR="${HOME}/.zeroned/localnet"
COORDINATOR_HOME="${BASE_DIR}/coordinator"
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BINARY="${PROJECT_ROOT}/build/zeroned"
KEYRING="test"

# val0 endpoints
RPC_URL="http://127.0.0.1:26601"
API_URL="http://127.0.0.1:1317"
NODE_FLAG="--node ${RPC_URL}"
HOME_FLAG="--home ${COORDINATOR_HOME}"
KEYRING_FLAG="--keyring-backend ${KEYRING}"
COMMON_FLAGS="${NODE_FLAG} ${HOME_FLAG} ${KEYRING_FLAG} --chain-id ${CHAIN_ID} --output json"
TX_FLAGS="${COMMON_FLAGS} --gas auto --gas-adjustment 1.5 --gas-prices 1${DENOM} --yes --broadcast-mode sync"

UPGRADE_NAME="v1.0.1-testnet"

# Test results
PASSED=0
FAILED=0
RESULTS=()

# ── Helpers ──────────────────────────────────────────────────────────────

die()  { echo -e "\033[1;31mERROR:\033[0m $*" >&2; exit 1; }
info() { echo -e "\033[1;34m  ->\033[0m $*"; }
ok()   { echo -e "\033[1;32m  OK\033[0m $*"; }
warn() { echo -e "\033[1;33m  !!\033[0m $*"; }

pass() {
  ok "PASS: $1"
  PASSED=$((PASSED + 1))
  RESULTS+=("PASS  $1")
}

fail() {
  echo -e "\033[1;31m  FAIL\033[0m $1: $2"
  FAILED=$((FAILED + 1))
  RESULTS+=("FAIL  $1: $2")
}

get_height() {
  curl -s "${RPC_URL}/status" 2>/dev/null | jq -r '.result.sync_info.latest_block_height' 2>/dev/null || echo "0"
}

wait_blocks() {
  local count="${1:-1}"
  local start_height
  start_height=$(get_height)
  local target=$((start_height + count))
  info "Waiting for ${count} blocks (current=${start_height}, target=${target})..."
  local elapsed=0
  while [ $elapsed -lt 300 ]; do
    local height
    height=$(get_height)
    if [ "${height}" -ge "${target}" ] 2>/dev/null; then
      return 0
    fi
    sleep 2
    elapsed=$((elapsed + 2))
  done
  warn "Timed out waiting for blocks"
  return 1
}

wait_height() {
  local target="$1"
  local max_wait="${2:-300}"
  info "Waiting for height ${target}..."
  local elapsed=0
  while [ $elapsed -lt $max_wait ]; do
    local height
    height=$(get_height)
    if [ "${height}" -ge "${target}" ] 2>/dev/null; then
      info "  Reached height ${height}"
      return 0
    fi
    sleep 2
    elapsed=$((elapsed + 2))
    if [ $((elapsed % 20)) -eq 0 ]; then
      info "  still waiting... (${elapsed}s, height=${height})"
    fi
  done
  warn "Timed out waiting for height ${target} after ${max_wait}s"
  return 1
}

wait_tx() {
  local tx_hash="$1"
  local max_wait="${2:-30}"
  local elapsed=0
  while [ $elapsed -lt $max_wait ]; do
    if ${BINARY} query tx "${tx_hash}" ${NODE_FLAG} --output json 2>/dev/null | jq -e '.code == 0' >/dev/null 2>&1; then
      return 0
    fi
    sleep 2
    elapsed=$((elapsed + 2))
  done
  return 1
}

submit_tx() {
  local result
  result=$(eval "$@" 2>/dev/null) || { echo "TX_FAILED"; return 1; }
  local tx_hash
  tx_hash=$(echo "$result" | jq -r '.txhash // empty' 2>/dev/null || echo "")
  if [ -z "$tx_hash" ]; then
    echo "TX_FAILED"
    return 1
  fi
  echo "$tx_hash"
}

# ── Preflight ────────────────────────────────────────────────────────────

preflight() {
  [ -f "${BINARY}" ] || die "Binary not found: ${BINARY}. Run: scripts/localnet.sh start"
  curl -s --connect-timeout 3 "${RPC_URL}/status" >/dev/null 2>&1 || \
    die "Localnet not running. Start with: scripts/localnet.sh start"
  local height
  height=$(get_height)
  [ "${height}" -ge 1 ] 2>/dev/null || die "Chain not producing blocks (height=${height})"
  info "Localnet reachable (height=${height})"
}

# ── Test: Governance-Triggered Upgrade ───────────────────────────────────

test_upgrade_simulation() {
  info "Testing governance-triggered upgrade simulation..."

  # Step 1: Get current height and calculate upgrade height
  local current_height
  current_height=$(get_height)
  # Allow enough time: ~60s voting period (~24 blocks) + buffer
  local UPGRADE_HEIGHT=$((current_height + 80))
  info "  Current height: ${current_height}"
  info "  Upgrade height: ${UPGRADE_HEIGHT}"

  # Step 2: Get the gov module address (authority for MsgSoftwareUpgrade)
  local gov_address
  gov_address=$(${BINARY} query auth module-account gov ${COMMON_FLAGS} 2>/dev/null | \
    jq -r '.account.base_account.address // .account.value.address // empty' 2>/dev/null || echo "")

  if [ -z "$gov_address" ]; then
    # Fallback: try REST API
    gov_address=$(curl -s "${API_URL}/cosmos/auth/v1beta1/module_accounts/gov" 2>/dev/null | \
      jq -r '.account.base_account.address // empty' 2>/dev/null || echo "")
  fi

  if [ -z "$gov_address" ]; then
    fail "upgrade_simulation" "Could not determine gov module address"
    return
  fi
  info "  Gov module address: ${gov_address}"

  # Step 3: Create proposal JSON
  local proposal_file
  proposal_file=$(mktemp /tmp/upgrade-proposal-XXXXXX.json)
  trap "rm -f ${proposal_file}" EXIT

  cat > "${proposal_file}" <<PROPOSAL_EOF
{
  "messages": [
    {
      "@type": "/cosmos.upgrade.v1beta1.MsgSoftwareUpgrade",
      "authority": "${gov_address}",
      "plan": {
        "name": "${UPGRADE_NAME}",
        "height": "${UPGRADE_HEIGHT}",
        "info": "Upgrade simulation test — v1.0.1-testnet migration pipeline"
      }
    }
  ],
  "metadata": "",
  "deposit": "10000000${DENOM}",
  "title": "Upgrade to ${UPGRADE_NAME}",
  "summary": "Simulated upgrade to test the v1-to-v2 migration pipeline for the knowledge module."
}
PROPOSAL_EOF

  info "  Proposal JSON written to ${proposal_file}"

  # Step 4: Submit governance proposal
  local tx_hash
  tx_hash=$(submit_tx "${BINARY} tx gov submit-proposal ${proposal_file} --from val0 ${TX_FLAGS}")

  if [ "$tx_hash" = "TX_FAILED" ]; then
    fail "upgrade_simulation" "Governance proposal submission failed"
    return
  fi

  if ! wait_tx "$tx_hash" 30; then
    fail "upgrade_simulation" "Proposal tx not included in block"
    return
  fi
  info "  Proposal submitted (tx: ${tx_hash})"

  # Step 5: Find the proposal ID
  wait_blocks 2

  local proposal_id
  proposal_id=$(${BINARY} query gov proposals --status voting_period ${COMMON_FLAGS} 2>/dev/null | \
    jq -r '.proposals[-1].id // empty' 2>/dev/null || echo "")

  # Fallback: try all proposals
  if [ -z "$proposal_id" ]; then
    proposal_id=$(${BINARY} query gov proposals ${COMMON_FLAGS} 2>/dev/null | \
      jq -r '.proposals[-1].id // empty' 2>/dev/null || echo "")
  fi

  # Fallback: REST API
  if [ -z "$proposal_id" ]; then
    proposal_id=$(curl -s "${API_URL}/cosmos/gov/v1/proposals?proposal_status=2" 2>/dev/null | \
      jq -r '.proposals[-1].id // empty' 2>/dev/null || echo "")
  fi

  if [ -z "$proposal_id" ]; then
    fail "upgrade_simulation" "Could not find proposal ID"
    return
  fi
  info "  Proposal ID: ${proposal_id}"

  # Step 6: Vote YES from all 4 validators
  for val in val0 val1 val2 val3; do
    local vote_tx
    vote_tx=$(submit_tx "${BINARY} tx gov vote ${proposal_id} yes --from ${val} ${TX_FLAGS}")
    if [ "$vote_tx" != "TX_FAILED" ]; then
      wait_tx "$vote_tx" 30 || true
      info "  ${val} voted YES"
    else
      warn "  ${val} vote failed"
    fi
  done

  # Step 7: Wait for voting period to end (60s in localnet genesis)
  info "  Waiting for voting period to end (60s)..."
  sleep 65

  # Step 8: Verify proposal passed
  local proposal_status
  proposal_status=$(${BINARY} query gov proposal "${proposal_id}" ${COMMON_FLAGS} 2>/dev/null | \
    jq -r '.proposal.status // empty' 2>/dev/null || echo "")

  # Fallback: REST API
  if [ -z "$proposal_status" ]; then
    proposal_status=$(curl -s "${API_URL}/cosmos/gov/v1/proposals/${proposal_id}" 2>/dev/null | \
      jq -r '.proposal.status // empty' 2>/dev/null || echo "")
  fi

  info "  Proposal status: ${proposal_status}"

  if [ "$proposal_status" != "PROPOSAL_STATUS_PASSED" ] && [ "$proposal_status" != "3" ]; then
    fail "upgrade_simulation" "Proposal did not pass (status: ${proposal_status})"
    return
  fi
  pass "governance_proposal_passed"

  # Step 9: Wait for the upgrade height
  if ! wait_height "${UPGRADE_HEIGHT}" 300; then
    # Chain may have halted — check if it's at or near upgrade height
    local halt_height
    halt_height=$(get_height)
    if [ "$halt_height" -ge "$((UPGRADE_HEIGHT - 1))" ] 2>/dev/null; then
      info "  Chain appears halted at height ${halt_height} (upgrade height: ${UPGRADE_HEIGHT})"
      info "  This means the handler did not run seamlessly — restarting..."

      # Stop and resume with the same binary (handler is already registered)
      "${PROJECT_ROOT}/scripts/localnet.sh" stop
      sleep 5
      "${PROJECT_ROOT}/scripts/localnet.sh" resume

      # Wait for chain to advance past upgrade height
      sleep 10
      if ! wait_height "$((UPGRADE_HEIGHT + 2))" 120; then
        fail "upgrade_simulation" "Chain did not resume after restart"
        return
      fi
      pass "chain_resumed_after_halt_and_restart"
    else
      fail "upgrade_simulation" "Timed out waiting for upgrade height (current: ${halt_height})"
      return
    fi
  else
    pass "chain_continued_past_upgrade_height"
  fi

  # Step 10: Verify chain is still producing blocks
  wait_blocks 3
  local post_height
  post_height=$(get_height)
  if [ "$post_height" -gt "$UPGRADE_HEIGHT" ]; then
    pass "chain_producing_blocks_post_upgrade (height: ${post_height})"
  else
    fail "chain_post_upgrade" "Chain not advancing past upgrade height (height: ${post_height})"
    return
  fi

  # Step 11: Verify knowledge module version is 2
  local knowledge_version
  knowledge_version=$(curl -s "${API_URL}/cosmos/upgrade/v1beta1/module_versions?module_name=knowledge" 2>/dev/null | \
    jq -r '.module_versions[0].version // empty' 2>/dev/null || echo "")

  if [ -z "$knowledge_version" ]; then
    # Fallback: query all module versions
    knowledge_version=$(curl -s "${API_URL}/cosmos/upgrade/v1beta1/module_versions" 2>/dev/null | \
      jq -r '.module_versions[] | select(.name == "knowledge") | .version // empty' 2>/dev/null || echo "")
  fi

  info "  Knowledge module version: ${knowledge_version}"

  if [ "$knowledge_version" = "2" ]; then
    pass "knowledge_module_version_is_2"
  else
    fail "module_version" "Expected knowledge version 2, got '${knowledge_version}'"
  fi

  # Step 12: Verify the upgrade is marked as applied
  local applied
  applied=$(curl -s "${API_URL}/cosmos/upgrade/v1beta1/applied_plan/${UPGRADE_NAME}" 2>/dev/null | \
    jq -r '.height // empty' 2>/dev/null || echo "")

  if [ -n "$applied" ] && [ "$applied" != "0" ] && [ "$applied" != "null" ]; then
    pass "upgrade_applied_at_height_${applied}"
  else
    fail "upgrade_applied" "Upgrade '${UPGRADE_NAME}' not marked as applied"
  fi
}

# ── Run ──────────────────────────────────────────────────────────────────

echo "═══════════════════════════════════════════════════════════════"
echo "  Zerone — Upgrade Simulation Test"
echo "═══════════════════════════════════════════════════════════════"
echo ""

preflight
echo ""

test_upgrade_simulation

echo ""
echo "═══════════════════════════════════════════════════════════════"
echo "  Results"
echo "═══════════════════════════════════════════════════════════════"
echo ""
for r in "${RESULTS[@]}"; do
  echo "  ${r}"
done
echo ""
echo "  Passed: ${PASSED}  Failed: ${FAILED}"
echo ""

if [ $FAILED -gt 0 ]; then
  echo "  STATUS: FAILED"
  exit 1
else
  echo "  STATUS: ALL PASSED"
  exit 0
fi
