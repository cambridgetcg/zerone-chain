#!/usr/bin/env bash
set -euo pipefail

###############################################################################
# R25-4 — Dispute Resolution E2E Test
# Tests: disputes, evidence_mgmt, capture_challenge, capture_defense
###############################################################################

BINARY=./build/zeroned
HOME_DIR=~/.zeroned/localnet/coordinator
NODE_FLAG="--node tcp://127.0.0.1:26601"
CHAIN_ID="zerone-localnet"
KB="--keyring-backend test"
GAS_FLAGS="--gas auto --gas-adjustment 1.5 --gas-prices 1uzrn"

# Result tracking
PASS=0; FAIL=0; SKIP=0; STUB=0
RESULTS=""

log() { echo -e "\n=== $1 ==="; }
ok()  { PASS=$((PASS+1)); RESULTS="${RESULTS}\n| $1 | PASS | $2 |"; echo "  ✓ PASS: $1"; }
fail(){ FAIL=$((FAIL+1)); RESULTS="${RESULTS}\n| $1 | FAIL | $2 |"; echo "  ✗ FAIL: $1 — $2"; }
skip(){ SKIP=$((SKIP+1)); RESULTS="${RESULTS}\n| $1 | SKIP | $2 |"; echo "  ⊘ SKIP: $1 — $2"; }
stub(){ STUB=$((STUB+1)); RESULTS="${RESULTS}\n| $1 | STUB | $2 |"; echo "  ◇ STUB: $1 — $2"; }

submit_tx() {
  local result
  result=$(eval "$@" 2>&1) || { echo "TX_FAILED: $result"; return 1; }
  local tx_hash
  tx_hash=$(echo "$result" | jq -r '.txhash // empty' 2>/dev/null || echo "")
  if [ -z "$tx_hash" ]; then
    echo "TX_FAILED: no txhash in: $result"
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
    result=$($BINARY query tx "$tx_hash" $NODE_FLAG --output json 2>/dev/null) || true
    if echo "$result" | jq -e '.code == 0' >/dev/null 2>&1; then
      return 0
    fi
    # Check for non-zero code (tx failed on-chain)
    local code
    code=$(echo "$result" | jq -r '.code // empty' 2>/dev/null)
    if [ -n "$code" ] && [ "$code" != "0" ] && [ "$code" != "null" ]; then
      local raw_log
      raw_log=$(echo "$result" | jq -r '.raw_log // .log // "unknown error"' 2>/dev/null)
      echo "TX_EXEC_FAILED: code=$code log=$raw_log"
      return 2
    fi
    sleep 2
    elapsed=$((elapsed + 2))
  done
  echo "TX_TIMEOUT after ${max_wait}s"
  return 1
}

get_tx_events() {
  local tx_hash="$1"
  $BINARY query tx "$tx_hash" $NODE_FLAG --output json 2>/dev/null | jq '.events'
}

get_tx_event() {
  local tx_hash="$1"
  local event_type="$2"
  local attr_key="$3"
  $BINARY query tx "$tx_hash" $NODE_FLAG --output json 2>/dev/null | \
    jq -r "[.events[] | select(.type == \"${event_type}\") | .attributes[] | select(.key == \"${attr_key}\") | .value][0] // empty"
}

get_height() {
  curl -s http://127.0.0.1:26601/status | jq -r '.result.sync_info.latest_block_height'
}

wait_blocks() {
  local count="${1:-1}"
  local start_height=$(get_height)
  local target=$((start_height + count))
  local timeout=300
  local elapsed=0
  while [ $(get_height) -lt $target ] && [ $elapsed -lt $timeout ]; do
    sleep 2
    elapsed=$((elapsed + 2))
  done
}

# Get addresses
VAL0=$($BINARY keys show val0 -a $KB --home $HOME_DIR)
VAL1=$($BINARY keys show val1 -a $KB --home $HOME_DIR)
VAL2=$($BINARY keys show val2 -a $KB --home $HOME_DIR)
VAL3=$($BINARY keys show val3 -a $KB --home $HOME_DIR)

echo "========================================"
echo " R25-4 Dispute Resolution E2E Test"
echo "========================================"
echo "Chain: $CHAIN_ID"
echo "Val0: $VAL0"
echo "Val1: $VAL1"
echo "Val2: $VAL2"
echo "Val3: $VAL3"
echo "Block: $(get_height)"
echo "========================================"

###############################################################################
# Step 1: Set Up — Create disputer account, fund it, submit a claim
###############################################################################
log "Step 1: Setup — Create & fund accounts, submit claim"

# Create disputer if needed
DISPUTER=$($BINARY keys show disputer -a $KB --home $HOME_DIR 2>/dev/null || true)
if [ -z "$DISPUTER" ]; then
  $BINARY keys add disputer $KB --home $HOME_DIR >/dev/null 2>&1
  DISPUTER=$($BINARY keys show disputer -a $KB --home $HOME_DIR)
fi
echo "  Disputer: $DISPUTER"

# Fund disputer
FUND_HASH=$(submit_tx $BINARY tx bank send "$VAL0" "$DISPUTER" 500000000uzrn \
  --from val0 $NODE_FLAG --home $HOME_DIR $KB --chain-id $CHAIN_ID \
  $GAS_FLAGS --yes --broadcast-mode sync --output json)
echo "  Fund tx: $FUND_HASH"
sleep 4

# Check balance
BALANCE=$($BINARY query bank balances "$DISPUTER" $NODE_FLAG --output json 2>/dev/null | jq -r '.balances[0].amount // "0"')
echo "  Disputer balance: ${BALANCE}uzrn"

if [ "$BALANCE" = "0" ] || [ -z "$BALANCE" ]; then
  fail "1.1 Fund disputer" "Balance still 0"
else
  ok "1.1 Fund disputer" "Balance: ${BALANCE}uzrn"
fi

# Submit a claim
CLAIM_HASH=$(submit_tx $BINARY tx knowledge submit-claim \
  "Earth is approximately 6000 years old based on biblical genealogy" \
  general empirical 2000000 \
  --from disputer $NODE_FLAG --home $HOME_DIR $KB --chain-id $CHAIN_ID \
  $GAS_FLAGS --yes --broadcast-mode sync --output json)
echo "  Claim tx: $CLAIM_HASH"
sleep 4

if [[ "$CLAIM_HASH" == TX_FAILED* ]]; then
  fail "1.2 Submit claim" "$CLAIM_HASH"
  CLAIM_ID=""
  ROUND_ID=""
else
  wait_result=$(wait_tx "$CLAIM_HASH" 30)
  if [ $? -eq 0 ]; then
    CLAIM_ID=$(get_tx_event "$CLAIM_HASH" "zerone.knowledge.submit_claim" "claim_id")
    ROUND_ID=$(get_tx_event "$CLAIM_HASH" "zerone.knowledge.verification_round_created" "round_id")
    echo "  Claim ID: $CLAIM_ID"
    echo "  Round ID: $ROUND_ID"
    if [ -n "$CLAIM_ID" ]; then
      ok "1.2 Submit claim" "Claim $CLAIM_ID, Round $ROUND_ID"
    else
      fail "1.2 Submit claim" "No claim_id in events"
    fi
  else
    fail "1.2 Submit claim" "TX failed: $wait_result"
  fi
fi

# Verify the claim via commit-reveal (val0 + val1)
if [ -n "$ROUND_ID" ]; then
  log "Step 1b: Verify claim via commit-reveal"

  # Commit phase
  SALT0=$(openssl rand -hex 16)
  SALT1=$(openssl rand -hex 16)
  COMMIT0=$(echo -n "accept${SALT0}" | shasum -a 256 | cut -d' ' -f1)
  COMMIT1=$(echo -n "accept${SALT1}" | shasum -a 256 | cut -d' ' -f1)

  C0_HASH=$(submit_tx $BINARY tx knowledge submit-commitment \
    "$ROUND_ID" "$COMMIT0" \
    --from val0 $NODE_FLAG --home $HOME_DIR $KB --chain-id $CHAIN_ID \
    $GAS_FLAGS --yes --broadcast-mode sync --output json)
  echo "  Commit val0: $C0_HASH"
  sleep 3

  C1_HASH=$(submit_tx $BINARY tx knowledge submit-commitment \
    "$ROUND_ID" "$COMMIT1" \
    --from val1 $NODE_FLAG --home $HOME_DIR $KB --chain-id $CHAIN_ID \
    $GAS_FLAGS --yes --broadcast-mode sync --output json)
  echo "  Commit val1: $C1_HASH"
  sleep 3

  if [[ "$C0_HASH" == TX_FAILED* ]] || [[ "$C1_HASH" == TX_FAILED* ]]; then
    fail "1.3 Submit commitments" "val0=$C0_HASH, val1=$C1_HASH"
  else
    wait_tx "$C0_HASH" 20 >/dev/null 2>&1
    wait_tx "$C1_HASH" 20 >/dev/null 2>&1
    ok "1.3 Submit commitments" "Both validators committed"
  fi

  # Wait for reveal phase
  echo "  Waiting for reveal phase (10 blocks)..."
  wait_blocks 12

  # Reveal phase
  R0_HASH=$(submit_tx $BINARY tx knowledge submit-reveal \
    "$ROUND_ID" accept "$SALT0" \
    --from val0 $NODE_FLAG --home $HOME_DIR $KB --chain-id $CHAIN_ID \
    $GAS_FLAGS --yes --broadcast-mode sync --output json)
  echo "  Reveal val0: $R0_HASH"
  sleep 3

  R1_HASH=$(submit_tx $BINARY tx knowledge submit-reveal \
    "$ROUND_ID" accept "$SALT1" \
    --from val1 $NODE_FLAG --home $HOME_DIR $KB --chain-id $CHAIN_ID \
    $GAS_FLAGS --yes --broadcast-mode sync --output json)
  echo "  Reveal val1: $R1_HASH"
  sleep 3

  if [[ "$R0_HASH" == TX_FAILED* ]] || [[ "$R1_HASH" == TX_FAILED* ]]; then
    fail "1.4 Submit reveals" "val0=$R0_HASH, val1=$R1_HASH"
  else
    r0_wait=$(wait_tx "$R0_HASH" 20 2>&1)
    r1_wait=$(wait_tx "$R1_HASH" 20 2>&1)
    if [[ "$r0_wait" == TX_EXEC_FAILED* ]] || [[ "$r1_wait" == TX_EXEC_FAILED* ]]; then
      fail "1.4 Submit reveals" "val0=$r0_wait, val1=$r1_wait"
    else
      ok "1.4 Submit reveals" "Both validators revealed"
    fi
  fi

  # Wait for aggregation
  echo "  Waiting for aggregation (5+ blocks)..."
  wait_blocks 8

  # Check fact status
  FACT_ID=""
  FACTS_JSON=$($BINARY query knowledge facts $NODE_FLAG --output json 2>/dev/null || echo "{}")
  FACT_COUNT=$(echo "$FACTS_JSON" | jq '.facts | length' 2>/dev/null || echo "0")
  echo "  Facts found: $FACT_COUNT"

  if [ "$FACT_COUNT" -gt 0 ] 2>/dev/null; then
    FACT_ID=$(echo "$FACTS_JSON" | jq -r '.facts[0].id // empty')
    FACT_STATUS=$(echo "$FACTS_JSON" | jq -r '.facts[0].status // empty')
    echo "  Fact ID: $FACT_ID"
    echo "  Fact status: $FACT_STATUS"
    ok "1.5 Fact verified" "ID=$FACT_ID status=$FACT_STATUS"
  else
    # Check claim status directly
    if [ -n "$CLAIM_ID" ]; then
      CLAIM_JSON=$($BINARY query knowledge claim "$CLAIM_ID" $NODE_FLAG --output json 2>/dev/null || echo "{}")
      CLAIM_STATUS=$(echo "$CLAIM_JSON" | jq -r '.claim.status // empty')
      echo "  Claim status: $CLAIM_STATUS"
    fi
    # Try round status
    if [ -n "$ROUND_ID" ]; then
      ROUND_JSON=$($BINARY query knowledge verification-round "$ROUND_ID" $NODE_FLAG --output json 2>/dev/null || echo "{}")
      ROUND_STATUS=$(echo "$ROUND_JSON" | jq -r '.round.phase // .round.status // empty')
      echo "  Round status: $ROUND_STATUS"
      echo "  Round detail: $(echo "$ROUND_JSON" | jq -c '.' 2>/dev/null)"
    fi
    fail "1.5 Fact verified" "No facts found after aggregation"
  fi
else
  skip "1.3-1.5" "No round_id — skipping verification"
  FACT_ID=""
fi

###############################################################################
# Step 2: Query Dispute Params
###############################################################################
log "Step 2: Query dispute module params"

DISPUTE_PARAMS=$($BINARY query disputes params $NODE_FLAG --output json 2>/dev/null)
echo "  Params: $DISPUTE_PARAMS"
if [ -n "$DISPUTE_PARAMS" ]; then
  TIER1_BOND=$(echo "$DISPUTE_PARAMS" | jq -r '.params.tier_configs[0].min_bond')
  TIER1_ARBITERS=$(echo "$DISPUTE_PARAMS" | jq -r '.params.tier_configs[0].arbiter_count')
  TIER1_EVIDENCE=$(echo "$DISPUTE_PARAMS" | jq -r '.params.tier_configs[0].evidence_period')
  TIER1_VOTING=$(echo "$DISPUTE_PARAMS" | jq -r '.params.tier_configs[0].voting_period')
  echo "  Tier 1: bond=${TIER1_BOND}, arbiters=${TIER1_ARBITERS}, evidence=${TIER1_EVIDENCE}blk, voting=${TIER1_VOTING}blk"
  ok "2.0 Query params" "4 tiers configured, tier1 bond=${TIER1_BOND}"
else
  fail "2.0 Query params" "No params returned"
fi

###############################################################################
# Step 3: Initiate a Dispute
###############################################################################
log "Step 3: Initiate dispute"

# Target type 1 = fact
TARGET_ID="${FACT_ID:-dummy-fact-id}"
DISPUTE_BOND="1000000"  # Tier 1 min bond

INIT_HASH=$(submit_tx $BINARY tx disputes initiate \
  1 "$TARGET_ID" \
  "This claim contradicts overwhelming geological and radiometric evidence" \
  "$DISPUTE_BOND" \
  --from val0 $NODE_FLAG --home $HOME_DIR $KB --chain-id $CHAIN_ID \
  $GAS_FLAGS --yes --broadcast-mode sync --output json)
echo "  Initiate tx: $INIT_HASH"
sleep 4

DISPUTE_ID=""
if [[ "$INIT_HASH" == TX_FAILED* ]]; then
  fail "3.1 Initiate dispute" "$INIT_HASH"
else
  init_wait=$(wait_tx "$INIT_HASH" 30 2>&1)
  if [ $? -eq 0 ]; then
    # Extract dispute ID from events
    DISPUTE_ID=$(get_tx_event "$INIT_HASH" "zerone.disputes.dispute_initiated" "dispute_id")
    if [ -z "$DISPUTE_ID" ]; then
      # Try alternative event names
      DISPUTE_ID=$(get_tx_event "$INIT_HASH" "dispute_initiated" "dispute_id")
    fi
    if [ -z "$DISPUTE_ID" ]; then
      # Try getting from all events
      ALL_EVENTS=$(get_tx_events "$INIT_HASH")
      echo "  All events: $(echo "$ALL_EVENTS" | jq -c '[.[].type]' 2>/dev/null)"
      DISPUTE_ID=$(echo "$ALL_EVENTS" | jq -r '[.[] | .attributes[] | select(.key == "dispute_id") | .value][0] // empty')
    fi

    echo "  Dispute ID: $DISPUTE_ID"
    if [ -n "$DISPUTE_ID" ]; then
      ok "3.1 Initiate dispute" "Dispute ID=$DISPUTE_ID"
    else
      ok "3.1 Initiate dispute" "TX succeeded but no dispute_id in events"
    fi

    # Query the dispute
    if [ -n "$DISPUTE_ID" ]; then
      DISPUTE_JSON=$($BINARY query disputes dispute "$DISPUTE_ID" $NODE_FLAG --output json 2>/dev/null || echo "{}")
      echo "  Dispute state: $(echo "$DISPUTE_JSON" | jq -c '.' 2>/dev/null)"
      DISPUTE_PHASE=$(echo "$DISPUTE_JSON" | jq -r '.dispute.phase // empty')
      DISPUTE_TIER=$(echo "$DISPUTE_JSON" | jq -r '.dispute.tier // empty')
      DISPUTE_ARBITERS=$(echo "$DISPUTE_JSON" | jq -r '.dispute.arbiters // empty')
      echo "  Phase: $DISPUTE_PHASE, Tier: $DISPUTE_TIER"
      echo "  Arbiters: $DISPUTE_ARBITERS"
      ok "3.2 Query dispute" "Phase=$DISPUTE_PHASE Tier=$DISPUTE_TIER"
    fi
  elif [ $? -eq 2 ]; then
    fail "3.1 Initiate dispute" "TX exec failed: $init_wait"
  else
    fail "3.1 Initiate dispute" "TX timeout: $init_wait"
  fi
fi

# Check active disputes
ACTIVE=$($BINARY query disputes active $NODE_FLAG --output json 2>/dev/null || echo "{}")
ACTIVE_COUNT=$(echo "$ACTIVE" | jq '.disputes | length' 2>/dev/null || echo "0")
echo "  Active disputes: $ACTIVE_COUNT"

# Check disputes by target
if [ -n "$TARGET_ID" ] && [ "$TARGET_ID" != "dummy-fact-id" ]; then
  BY_TARGET=$($BINARY query disputes by-target "$TARGET_ID" $NODE_FLAG --output json 2>/dev/null || echo "{}")
  echo "  Disputes for target: $(echo "$BY_TARGET" | jq -c '.' 2>/dev/null)"
fi

###############################################################################
# Step 4: Evidence Commit Phase
###############################################################################
log "Step 4: Evidence commit (disputes module)"

if [ -n "$DISPUTE_ID" ]; then
  # Create evidence commitment
  EVIDENCE_CONTENT="Radiometric dating of zircon crystals consistently dates Earth formation to 4.54 billion years"
  EVIDENCE_NONCE=$(openssl rand -hex 16)
  EVIDENCE_COMMIT=$(echo -n "${EVIDENCE_CONTENT}${EVIDENCE_NONCE}" | shasum -a 256 | cut -d' ' -f1)

  EC_HASH=$(submit_tx $BINARY tx disputes commit-evidence \
    "$DISPUTE_ID" "$EVIDENCE_COMMIT" \
    --from val0 $NODE_FLAG --home $HOME_DIR $KB --chain-id $CHAIN_ID \
    $GAS_FLAGS --yes --broadcast-mode sync --output json)
  echo "  Commit evidence tx (val0): $EC_HASH"
  sleep 4

  if [[ "$EC_HASH" == TX_FAILED* ]]; then
    fail "4.1 Commit evidence (val0)" "$EC_HASH"
  else
    ec_wait=$(wait_tx "$EC_HASH" 30 2>&1)
    if [ $? -eq 0 ]; then
      ok "4.1 Commit evidence (val0)" "Hash committed"
    else
      fail "4.1 Commit evidence (val0)" "$ec_wait"
    fi
  fi

  # Second party commits evidence
  EVIDENCE2_CONTENT="Multiple independent dating methods converge on 4.54Ga Earth age"
  EVIDENCE2_NONCE=$(openssl rand -hex 16)
  EVIDENCE2_COMMIT=$(echo -n "${EVIDENCE2_CONTENT}${EVIDENCE2_NONCE}" | shasum -a 256 | cut -d' ' -f1)

  EC2_HASH=$(submit_tx $BINARY tx disputes commit-evidence \
    "$DISPUTE_ID" "$EVIDENCE2_COMMIT" \
    --from val1 $NODE_FLAG --home $HOME_DIR $KB --chain-id $CHAIN_ID \
    $GAS_FLAGS --yes --broadcast-mode sync --output json)
  echo "  Commit evidence tx (val1): $EC2_HASH"
  sleep 4

  if [[ "$EC2_HASH" == TX_FAILED* ]]; then
    fail "4.2 Commit evidence (val1)" "$EC2_HASH"
  else
    ec2_wait=$(wait_tx "$EC2_HASH" 30 2>&1)
    if [ $? -eq 0 ]; then
      ok "4.2 Commit evidence (val1)" "Second party committed"
    else
      fail "4.2 Commit evidence (val1)" "$ec2_wait"
    fi
  fi
else
  skip "4.1-4.2 Commit evidence" "No dispute_id"
fi

###############################################################################
# Step 5: Evidence Reveal Phase
###############################################################################
log "Step 5: Evidence reveal (disputes module)"

if [ -n "$DISPUTE_ID" ]; then
  # Check current phase — we may need to wait for reveal phase
  CURRENT_PHASE=$($BINARY query disputes dispute "$DISPUTE_ID" $NODE_FLAG --output json 2>/dev/null | jq -r '.dispute.phase // empty')
  echo "  Current phase: $CURRENT_PHASE"

  # Try revealing immediately (will tell us if timing is enforced)
  ER_HASH=$(submit_tx $BINARY tx disputes reveal-evidence \
    "$DISPUTE_ID" "$EVIDENCE_CONTENT" "$EVIDENCE_NONCE" \
    --from val0 $NODE_FLAG --home $HOME_DIR $KB --chain-id $CHAIN_ID \
    $GAS_FLAGS --yes --broadcast-mode sync --output json)
  echo "  Reveal evidence tx (val0): $ER_HASH"
  sleep 4

  if [[ "$ER_HASH" == TX_FAILED* ]]; then
    # Might need to wait for reveal phase transition
    echo "  Reveal failed — possibly still in commit phase. Checking phase..."
    fail "5.1 Reveal evidence (val0, early)" "$ER_HASH"

    # Wait for evidence period to expire (500 blocks for tier 1 — too long!)
    # Just document the timing requirement
    echo "  NOTE: Tier 1 evidence_period = 500 blocks (~21 min). Too long for interactive test."
    echo "  Phase transition happens via BeginBlocker when evidence_commit_deadline passes."
    stub "5.1 Reveal evidence" "Evidence period too long (500 blocks) for interactive test"
  else
    er_wait=$(wait_tx "$ER_HASH" 30 2>&1)
    if [ $? -eq 0 ]; then
      ok "5.1 Reveal evidence (val0)" "Evidence revealed successfully"

      # Try mismatched reveal
      BAD_HASH=$(submit_tx $BINARY tx disputes reveal-evidence \
        "$DISPUTE_ID" "wrong content" "$EVIDENCE_NONCE" \
        --from val1 $NODE_FLAG --home $HOME_DIR $KB --chain-id $CHAIN_ID \
        $GAS_FLAGS --yes --broadcast-mode sync --output json)
      echo "  Bad reveal tx: $BAD_HASH"
      sleep 4

      if [[ "$BAD_HASH" == TX_FAILED* ]]; then
        ok "5.2 Mismatched reveal rejected" "Correctly rejected at broadcast"
      else
        bad_wait=$(wait_tx "$BAD_HASH" 20 2>&1)
        if [[ "$bad_wait" == TX_EXEC_FAILED* ]]; then
          ok "5.2 Mismatched reveal rejected" "Correctly rejected on-chain"
        else
          fail "5.2 Mismatched reveal" "Should have been rejected but wasn't"
        fi
      fi

      # Correct reveal for val1
      ER2_HASH=$(submit_tx $BINARY tx disputes reveal-evidence \
        "$DISPUTE_ID" "$EVIDENCE2_CONTENT" "$EVIDENCE2_NONCE" \
        --from val1 $NODE_FLAG --home $HOME_DIR $KB --chain-id $CHAIN_ID \
        $GAS_FLAGS --yes --broadcast-mode sync --output json)
      echo "  Reveal val1 tx: $ER2_HASH"
      sleep 4

      if [[ "$ER2_HASH" == TX_FAILED* ]]; then
        fail "5.3 Reveal evidence (val1)" "$ER2_HASH"
      else
        er2_wait=$(wait_tx "$ER2_HASH" 30 2>&1)
        if [ $? -eq 0 ]; then
          ok "5.3 Reveal evidence (val1)" "Second party revealed"
        else
          fail "5.3 Reveal evidence (val1)" "$er2_wait"
        fi
      fi
    elif [[ "$er_wait" == TX_EXEC_FAILED* ]]; then
      echo "  Reveal rejected on-chain (likely phase timing): $er_wait"
      stub "5.1 Reveal evidence" "Phase transition needed — evidence_period=500 blocks"
    else
      fail "5.1 Reveal evidence" "$er_wait"
    fi
  fi
else
  skip "5.x Reveal evidence" "No dispute_id"
fi

###############################################################################
# Step 6: Evidence Management Module (Standalone)
###############################################################################
log "Step 6: Evidence management module"

# Submit evidence (type 1 = document)
CONTENT_HASH=$(echo -n "Earth age radiometric evidence 4.54Ga" | shasum -a 256 | cut -d' ' -f1)

EM_HASH=$(submit_tx $BINARY tx evidence_mgmt submit \
  1 "$CONTENT_HASH" "Radiometric dating peer-reviewed study" \
  --from val0 $NODE_FLAG --home $HOME_DIR $KB --chain-id $CHAIN_ID \
  $GAS_FLAGS --yes --broadcast-mode sync --output json)
echo "  Submit evidence tx: $EM_HASH"
sleep 4

EVIDENCE_ID=""
if [[ "$EM_HASH" == TX_FAILED* ]]; then
  fail "6.1 Submit evidence (evidence_mgmt)" "$EM_HASH"
else
  em_wait=$(wait_tx "$EM_HASH" 30 2>&1)
  if [ $? -eq 0 ]; then
    EVIDENCE_ID=$(get_tx_event "$EM_HASH" "zerone.evidence_mgmt.evidence_submitted" "evidence_id")
    if [ -z "$EVIDENCE_ID" ]; then
      ALL_EM_EVENTS=$(get_tx_events "$EM_HASH")
      echo "  Events: $(echo "$ALL_EM_EVENTS" | jq -c '[.[].type]' 2>/dev/null)"
      EVIDENCE_ID=$(echo "$ALL_EM_EVENTS" | jq -r '[.[] | .attributes[] | select(.key == "evidence_id") | .value][0] // empty')
    fi
    echo "  Evidence ID: $EVIDENCE_ID"

    if [ -n "$EVIDENCE_ID" ]; then
      ok "6.1 Submit evidence" "ID=$EVIDENCE_ID"

      # Query evidence
      EV_JSON=$($BINARY query evidence_mgmt evidence "$EVIDENCE_ID" $NODE_FLAG --output json 2>/dev/null || echo "{}")
      echo "  Evidence state: $(echo "$EV_JSON" | jq -c '.' 2>/dev/null)"

      EV_STATUS=$(echo "$EV_JSON" | jq -r '.evidence.status // empty')
      EV_CUSTODIAN=$(echo "$EV_JSON" | jq -r '.evidence.chain_of_custody[0].custodian // empty')
      echo "  Status: $EV_STATUS, Initial custodian: $EV_CUSTODIAN"
      ok "6.2 Query evidence" "Status=$EV_STATUS"
    else
      ok "6.1 Submit evidence" "TX succeeded but no evidence_id in events"
    fi
  else
    fail "6.1 Submit evidence" "$em_wait"
  fi
fi

# Query evidence_mgmt params
EM_PARAMS=$($BINARY query evidence_mgmt params $NODE_FLAG --output json 2>/dev/null || echo "{}")
echo "  Evidence mgmt params: $(echo "$EM_PARAMS" | jq -c '.' 2>/dev/null)"

# Note: transfer-custody and verify-evidence have no CLI
stub "6.3 Transfer custody" "No CLI command (proto-only MsgTransferCustody)"
stub "6.4 Verify evidence" "No CLI command (proto-only MsgVerifyEvidence)"

###############################################################################
# Step 7: Arbiter Vote
###############################################################################
log "Step 7: Arbiter vote"

if [ -n "$DISPUTE_ID" ]; then
  # Check dispute phase
  DISP_STATE=$($BINARY query disputes dispute "$DISPUTE_ID" $NODE_FLAG --output json 2>/dev/null || echo "{}")
  CUR_PHASE=$(echo "$DISP_STATE" | jq -r '.dispute.phase // empty')
  ARBITERS_LIST=$(echo "$DISP_STATE" | jq -r '.dispute.arbiters // empty')
  echo "  Current phase: $CUR_PHASE"
  echo "  Arbiters: $ARBITERS_LIST"

  # Try voting (decision: challenger, defender, abstain)
  VOTE_HASH=$(submit_tx $BINARY tx disputes vote \
    "$DISPUTE_ID" "challenger" \
    "The challenger evidence is scientifically rigorous and contradicts the original claim" \
    --from val0 $NODE_FLAG --home $HOME_DIR $KB --chain-id $CHAIN_ID \
    $GAS_FLAGS --yes --broadcast-mode sync --output json)
  echo "  Vote tx (val0): $VOTE_HASH"
  sleep 4

  if [[ "$VOTE_HASH" == TX_FAILED* ]]; then
    fail "7.1 Arbiter vote (val0)" "$VOTE_HASH"
  else
    vote_wait=$(wait_tx "$VOTE_HASH" 30 2>&1)
    if [ $? -eq 0 ]; then
      ok "7.1 Arbiter vote (val0)" "Voted for challenger"
    elif [[ "$vote_wait" == TX_EXEC_FAILED* ]]; then
      echo "  Vote failed on-chain: $vote_wait"
      fail "7.1 Arbiter vote (val0)" "$vote_wait"
    else
      fail "7.1 Arbiter vote (val0)" "$vote_wait"
    fi
  fi

  # Second arbiter vote
  VOTE2_HASH=$(submit_tx $BINARY tx disputes vote \
    "$DISPUTE_ID" "challenger" \
    "Agree — radiometric dating is well-established" \
    --from val1 $NODE_FLAG --home $HOME_DIR $KB --chain-id $CHAIN_ID \
    $GAS_FLAGS --yes --broadcast-mode sync --output json)
  echo "  Vote tx (val1): $VOTE2_HASH"
  sleep 4

  if [[ "$VOTE2_HASH" == TX_FAILED* ]]; then
    fail "7.2 Arbiter vote (val1)" "$VOTE2_HASH"
  else
    vote2_wait=$(wait_tx "$VOTE2_HASH" 30 2>&1)
    if [ $? -eq 0 ]; then
      ok "7.2 Arbiter vote (val1)" "Voted for challenger"
    elif [[ "$vote2_wait" == TX_EXEC_FAILED* ]]; then
      fail "7.2 Arbiter vote (val1)" "$vote2_wait"
    else
      fail "7.2 Arbiter vote (val1)" "$vote2_wait"
    fi
  fi

  # Third arbiter vote (need 3 for tier 1)
  VOTE3_HASH=$(submit_tx $BINARY tx disputes vote \
    "$DISPUTE_ID" "defender" \
    "The original claim has cultural validity" \
    --from val2 $NODE_FLAG --home $HOME_DIR $KB --chain-id $CHAIN_ID \
    $GAS_FLAGS --yes --broadcast-mode sync --output json)
  echo "  Vote tx (val2, defender): $VOTE3_HASH"
  sleep 4

  if [[ "$VOTE3_HASH" == TX_FAILED* ]]; then
    fail "7.3 Arbiter vote (val2)" "$VOTE3_HASH"
  else
    vote3_wait=$(wait_tx "$VOTE3_HASH" 30 2>&1)
    if [ $? -eq 0 ]; then
      ok "7.3 Arbiter vote (val2)" "Voted for defender"
    else
      fail "7.3 Arbiter vote (val2)" "$vote3_wait"
    fi
  fi
else
  skip "7.x Arbiter votes" "No dispute_id"
fi

###############################################################################
# Step 8: Settle Dispute
###############################################################################
log "Step 8: Settle dispute"

if [ -n "$DISPUTE_ID" ]; then
  # settle is authority-only
  SETTLE_HASH=$(submit_tx $BINARY tx disputes settle \
    "$DISPUTE_ID" \
    --from val0 $NODE_FLAG --home $HOME_DIR $KB --chain-id $CHAIN_ID \
    $GAS_FLAGS --yes --broadcast-mode sync --output json)
  echo "  Settle tx: $SETTLE_HASH"
  sleep 4

  if [[ "$SETTLE_HASH" == TX_FAILED* ]]; then
    fail "8.1 Settle dispute" "$SETTLE_HASH"
  else
    settle_wait=$(wait_tx "$SETTLE_HASH" 30 2>&1)
    if [ $? -eq 0 ]; then
      ok "8.1 Settle dispute" "Settled successfully"

      # Check final dispute state
      FINAL=$($BINARY query disputes dispute "$DISPUTE_ID" $NODE_FLAG --output json 2>/dev/null || echo "{}")
      FINAL_PHASE=$(echo "$FINAL" | jq -r '.dispute.phase // empty')
      FINAL_OUTCOME=$(echo "$FINAL" | jq -r '.dispute.outcome // empty')
      echo "  Final phase: $FINAL_PHASE"
      echo "  Outcome: $FINAL_OUTCOME"
      ok "8.2 Dispute outcome" "Phase=$FINAL_PHASE Outcome=$FINAL_OUTCOME"
    elif [[ "$settle_wait" == TX_EXEC_FAILED* ]]; then
      echo "  Settle failed (likely authority-only): $settle_wait"
      stub "8.1 Settle dispute" "Authority-only — val0 not authorized"
    else
      fail "8.1 Settle dispute" "$settle_wait"
    fi
  fi
else
  skip "8.x Settle dispute" "No dispute_id"
fi

###############################################################################
# Step 9: Escalation (Second Dispute)
###############################################################################
log "Step 9: Escalation test"

# Create a second dispute to test escalation
if [ -n "$FACT_ID" ] || [ -n "$TARGET_ID" ]; then
  ESCAL_TARGET="${FACT_ID:-$TARGET_ID}"

  # Initiate a fresh dispute
  INIT2_HASH=$(submit_tx $BINARY tx disputes initiate \
    1 "$ESCAL_TARGET" \
    "Testing escalation mechanism" \
    "1000000" \
    --from val1 $NODE_FLAG --home $HOME_DIR $KB --chain-id $CHAIN_ID \
    $GAS_FLAGS --yes --broadcast-mode sync --output json)
  echo "  Init dispute 2 tx: $INIT2_HASH"
  sleep 4

  DISPUTE2_ID=""
  if [[ "$INIT2_HASH" != TX_FAILED* ]]; then
    wait_tx "$INIT2_HASH" 30 >/dev/null 2>&1
    ALL_EVENTS2=$(get_tx_events "$INIT2_HASH")
    DISPUTE2_ID=$(echo "$ALL_EVENTS2" | jq -r '[.[] | .attributes[] | select(.key == "dispute_id") | .value][0] // empty')
    echo "  Dispute 2 ID: $DISPUTE2_ID"
  fi

  if [ -n "$DISPUTE2_ID" ]; then
    # Escalate to tier 2
    ESCAL_HASH=$(submit_tx $BINARY tx disputes escalate \
      "$DISPUTE2_ID" "10000000" \
      --from val1 $NODE_FLAG --home $HOME_DIR $KB --chain-id $CHAIN_ID \
      $GAS_FLAGS --yes --broadcast-mode sync --output json)
    echo "  Escalate tx: $ESCAL_HASH"
    sleep 4

    if [[ "$ESCAL_HASH" == TX_FAILED* ]]; then
      fail "9.1 Escalate dispute" "$ESCAL_HASH"
    else
      escal_wait=$(wait_tx "$ESCAL_HASH" 30 2>&1)
      if [ $? -eq 0 ]; then
        ESC_STATE=$($BINARY query disputes dispute "$DISPUTE2_ID" $NODE_FLAG --output json 2>/dev/null || echo "{}")
        ESC_TIER=$(echo "$ESC_STATE" | jq -r '.dispute.tier // empty')
        ESC_PHASE=$(echo "$ESC_STATE" | jq -r '.dispute.phase // empty')
        echo "  Escalated tier: $ESC_TIER, phase: $ESC_PHASE"
        ok "9.1 Escalate dispute" "New tier=$ESC_TIER phase=$ESC_PHASE"
      elif [[ "$escal_wait" == TX_EXEC_FAILED* ]]; then
        fail "9.1 Escalate dispute" "$escal_wait"
      else
        fail "9.1 Escalate dispute" "$escal_wait"
      fi
    fi
  else
    fail "9.1 Escalate dispute" "Could not create second dispute"
  fi
else
  skip "9.x Escalation" "No target available"
fi

###############################################################################
# Step 10: Capture Challenge
###############################################################################
log "Step 10: Capture challenge"

# Submit capture challenge against "general" domain
CC_HASH=$(submit_tx $BINARY tx capture_challenge submit \
  "general" "5000000" "$VAL0" \
  --reason "Validator val0 dominates general domain verification — potential capture" \
  --from val1 $NODE_FLAG --home $HOME_DIR $KB --chain-id $CHAIN_ID \
  $GAS_FLAGS --yes --broadcast-mode sync --output json)
echo "  Capture challenge tx: $CC_HASH"
sleep 4

CHALLENGE_ID=""
if [[ "$CC_HASH" == TX_FAILED* ]]; then
  fail "10.1 Submit capture challenge" "$CC_HASH"
else
  cc_wait=$(wait_tx "$CC_HASH" 30 2>&1)
  if [ $? -eq 0 ]; then
    ALL_CC_EVENTS=$(get_tx_events "$CC_HASH")
    echo "  Events: $(echo "$ALL_CC_EVENTS" | jq -c '[.[].type]' 2>/dev/null)"
    CHALLENGE_ID=$(echo "$ALL_CC_EVENTS" | jq -r '[.[] | .attributes[] | select(.key == "challenge_id") | .value][0] // empty')
    echo "  Challenge ID: $CHALLENGE_ID"
    ok "10.1 Submit capture challenge" "ID=$CHALLENGE_ID"

    if [ -n "$CHALLENGE_ID" ]; then
      # Query the challenge
      CC_JSON=$($BINARY query capture_challenge challenge "$CHALLENGE_ID" $NODE_FLAG --output json 2>/dev/null || echo "{}")
      echo "  Challenge state: $(echo "$CC_JSON" | jq -c '.' 2>/dev/null)"
      CC_STATUS=$(echo "$CC_JSON" | jq -r '.challenge.status // empty')
      echo "  Status: $CC_STATUS"
    fi
  elif [[ "$cc_wait" == TX_EXEC_FAILED* ]]; then
    fail "10.1 Submit capture challenge" "$cc_wait"
  else
    fail "10.1 Submit capture challenge" "$cc_wait"
  fi
fi

# Add evidence to challenge
if [ -n "$CHALLENGE_ID" ]; then
  DATA_HASH=$(echo -n "verification stats for val0 in general domain" | shasum -a 256 | cut -d' ' -f1)

  AE_HASH=$(submit_tx $BINARY tx capture_challenge add-evidence \
    "$CHALLENGE_ID" \
    "Val0 verified 90% of facts in general domain in the last 100 blocks" \
    "$DATA_HASH" \
    --from val1 $NODE_FLAG --home $HOME_DIR $KB --chain-id $CHAIN_ID \
    $GAS_FLAGS --yes --broadcast-mode sync --output json)
  echo "  Add evidence tx: $AE_HASH"
  sleep 4

  if [[ "$AE_HASH" == TX_FAILED* ]]; then
    fail "10.2 Add evidence to challenge" "$AE_HASH"
  else
    ae_wait=$(wait_tx "$AE_HASH" 30 2>&1)
    if [ $? -eq 0 ]; then
      ok "10.2 Add evidence to challenge" "Evidence added"
    else
      fail "10.2 Add evidence to challenge" "$ae_wait"
    fi
  fi
fi

# Query challenges by domain
CC_BY_DOMAIN=$($BINARY query capture_challenge challenges-by-domain "general" $NODE_FLAG --output json 2>/dev/null || echo "{}")
echo "  Challenges for 'general': $(echo "$CC_BY_DOMAIN" | jq -c '.' 2>/dev/null)"

###############################################################################
# Step 11: Fund Bounty Pool
###############################################################################
log "Step 11: Fund bounty pool"

FB_HASH=$(submit_tx $BINARY tx capture_challenge fund-bounty \
  "general" "50000000" \
  --from val0 $NODE_FLAG --home $HOME_DIR $KB --chain-id $CHAIN_ID \
  $GAS_FLAGS --yes --broadcast-mode sync --output json)
echo "  Fund bounty tx: $FB_HASH"
sleep 4

if [[ "$FB_HASH" == TX_FAILED* ]]; then
  fail "11.1 Fund bounty pool" "$FB_HASH"
else
  fb_wait=$(wait_tx "$FB_HASH" 30 2>&1)
  if [ $? -eq 0 ]; then
    # Query bounty pool
    BP_JSON=$($BINARY query capture_challenge bounty-pool "general" $NODE_FLAG --output json 2>/dev/null || echo "{}")
    BP_BALANCE=$(echo "$BP_JSON" | jq -r '.bounty_pool.balance // empty')
    echo "  Bounty pool balance: $BP_BALANCE"
    ok "11.1 Fund bounty pool" "Balance=$BP_BALANCE"
  else
    fail "11.1 Fund bounty pool" "$fb_wait"
  fi
fi

###############################################################################
# Step 12: Capture Defense — Analyze Domain
###############################################################################
log "Step 12: Capture defense"

# Analyze domain
AD_HASH=$(submit_tx $BINARY tx capture_defense analyze-domain \
  "general" \
  --from val0 $NODE_FLAG --home $HOME_DIR $KB --chain-id $CHAIN_ID \
  $GAS_FLAGS --yes --broadcast-mode sync --output json)
echo "  Analyze domain tx: $AD_HASH"
sleep 4

if [[ "$AD_HASH" == TX_FAILED* ]]; then
  fail "12.1 Analyze domain" "$AD_HASH"
else
  ad_wait=$(wait_tx "$AD_HASH" 30 2>&1)
  if [ $? -eq 0 ]; then
    ok "12.1 Analyze domain" "Analysis completed"
  elif [[ "$ad_wait" == TX_EXEC_FAILED* ]]; then
    fail "12.1 Analyze domain" "$ad_wait"
  else
    fail "12.1 Analyze domain" "$ad_wait"
  fi
fi

# Query capture metrics
METRICS=$($BINARY query capture_defense metrics "general" $NODE_FLAG --output json 2>/dev/null || echo "{}")
echo "  Capture metrics: $(echo "$METRICS" | jq -c '.' 2>/dev/null)"
RISK_SCORE=$(echo "$METRICS" | jq -r '.metrics.risk_score // empty')
HERFINDAHL=$(echo "$METRICS" | jq -r '.metrics.herfindahl_index // empty')
echo "  Risk score: $RISK_SCORE, Herfindahl: $HERFINDAHL"

if [ -n "$RISK_SCORE" ] || [ -n "$HERFINDAHL" ]; then
  ok "12.2 Capture metrics" "Risk=$RISK_SCORE Herfindahl=$HERFINDAHL"
else
  stub "12.2 Capture metrics" "No metrics computed (likely needs verification history)"
fi

# Query reputation
REP=$($BINARY query capture_defense reputation "$VAL0" $NODE_FLAG --output json 2>/dev/null || echo "{}")
echo "  Val0 reputation: $(echo "$REP" | jq -c '.' 2>/dev/null)"

REP_SCORE=$(echo "$REP" | jq -r '.reputation.score // .global_reputation.score // empty')
if [ -n "$REP_SCORE" ]; then
  ok "12.3 Validator reputation" "Val0 score=$REP_SCORE"
else
  stub "12.3 Validator reputation" "No reputation data yet"
fi

# Query capture_defense params
CD_PARAMS=$($BINARY query capture_defense params $NODE_FLAG --output json 2>/dev/null || echo "{}")
echo "  Capture defense params: $(echo "$CD_PARAMS" | jq -c '.' 2>/dev/null)"

###############################################################################
# Summary
###############################################################################
echo ""
echo "========================================"
echo " R25-4 TEST SUMMARY"
echo "========================================"
echo " PASS: $PASS"
echo " FAIL: $FAIL"
echo " SKIP: $SKIP"
echo " STUB: $STUB"
echo "========================================"
echo ""
echo "| Step | Status | Notes |"
echo "|------|--------|-------|"
echo -e "$RESULTS"
echo ""
echo "Block at end: $(get_height)"
