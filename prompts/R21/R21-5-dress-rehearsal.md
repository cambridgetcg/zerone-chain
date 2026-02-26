# R21-5 — Dress Rehearsal: Full Launch Pipeline

## Context

This is the go/no-go gate. Every component works individually. R21-1 through R21-4 verified the pieces in combination. This session runs the **exact launch-day sequence** as a single unbroken pipeline with zero manual intervention.

If this passes, you're ready for public testnet.

## Prerequisites

ALL R21-1 through R21-4 complete. All tests passing.

## Task

### The Pipeline

One script. One run. No stopping to fix things. If it breaks, fix it offline and re-run from scratch.

Create `scripts/dress-rehearsal.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

# ═══════════════════════════════════════════════════════════════
# ZERONE TESTNET DRESS REHEARSAL
# Expected runtime: ~10 minutes
# ═══════════════════════════════════════════════════════════════

STEP=0
total_steps=12
pass() { STEP=$((STEP+1)); echo -e "\n\033[1;32m[$STEP/$total_steps] PASS:\033[0m $1"; }
fail() { echo -e "\n\033[1;31m[$STEP/$total_steps] FAIL:\033[0m $1"; exit 1; }
```

### Phase 1: Clean Build (Step 1)

```bash
cd ~/Desktop/zerone
make clean && make build || fail "Build failed"
VERSION=$(./build/zeroned version)
pass "Clean build — zeroned $VERSION"
```

### Phase 2: Genesis Ceremony (Steps 2-4)

```bash
# Step 2: Initialize localnet
scripts/localnet.sh clean 2>/dev/null || true
scripts/localnet.sh init || fail "Localnet init failed"
pass "Localnet initialized (4 validators)"

# Step 3: Inject axiom seeds
GENESIS="${HOME}/.zeroned/localnet/coordinator/config/genesis.json"
go run tools/axiom-loader/main.go inject \
    --input seeds.txt \
    --genesis "$GENESIS" || fail "Axiom injection failed"
AXIOM_COUNT=$(jq '.app_state.knowledge.facts | length' "$GENESIS")
pass "Axiom seeds injected ($AXIOM_COUNT facts)"

# Step 4: Genesis invariant check
go run tools/genesis-check/main.go \
    --genesis "$GENESIS" \
    --profile testnet || fail "Genesis invariants failed"
pass "Genesis invariants validated"
```

**Note:** If `scripts/localnet.sh` doesn't have a separate `init` subcommand, adapt accordingly. The key phases are: (a) create genesis, (b) inject axioms, (c) validate, (d) start. If `localnet.sh start` does all of (a)+(d), you may need to inject axioms between init and start — add an `init` subcommand to localnet.sh if needed.

### Phase 3: Boot Chain (Step 5)

```bash
# Step 5: Start all 4 validators
scripts/localnet.sh start || fail "Localnet start failed"

# Wait for first block
for i in $(seq 1 30); do
    H=$(curl -s http://127.0.0.1:26601/status 2>/dev/null | jq -r '.result.sync_info.latest_block_height' 2>/dev/null || echo "0")
    [ "$H" -gt "0" ] && break
    sleep 1
done
[ "$H" -gt "0" ] || fail "No blocks produced after 30s"
pass "Chain booted — height $H"
```

### Phase 4: 100 Blocks (Step 6)

```bash
# Step 6: Wait for 100 blocks
for i in $(seq 1 120); do
    H=$(curl -s http://127.0.0.1:26601/status | jq -r '.result.sync_info.latest_block_height')
    [ "$H" -ge "100" ] && break
    sleep 2
done
[ "$H" -ge "100" ] || fail "Did not reach block 100 (stuck at $H)"
pass "100 blocks produced (height $H)"
```

### Phase 5: Smoke Tests (Steps 7-8)

```bash
# Step 7: API health
NETWORK=$(curl -s http://127.0.0.1:1317/cosmos/base/tendermint/v1beta1/node_info | jq -r '.default_node_info.network')
[ "$NETWORK" = "zerone-localnet" ] || fail "Wrong network: $NETWORK"

VAL_COUNT=$(curl -s http://127.0.0.1:26601/validators | jq -r '.result.total')
[ "$VAL_COUNT" = "4" ] || fail "Expected 4 validators, got $VAL_COUNT"
pass "API healthy — 4 validators on $NETWORK"

# Step 8: Axioms queryable
# Query knowledge facts — verify axioms loaded
FACT_COUNT=$(./build/zeroned query knowledge facts --limit 1000 \
    --node http://127.0.0.1:26601 --output json 2>/dev/null | jq '.facts | length')
[ "$FACT_COUNT" -gt "0" ] || fail "No facts in knowledge module"
pass "Knowledge module has $FACT_COUNT facts"
```

### Phase 6: PoT Round (Step 9)

```bash
# Step 9: Submit a claim and watch PoT round complete
COORD_HOME="${HOME}/.zeroned/localnet/coordinator"

./build/zeroned tx knowledge submit-claim \
    --domain "test-domain" \
    --claim-type assertion \
    --subject "dress rehearsal test" \
    --predicate "passes all checks" \
    --from val0 \
    --home "$COORD_HOME" \
    --keyring-backend test \
    --node http://127.0.0.1:26601 \
    --chain-id zerone-localnet \
    --gas auto --gas-adjustment 1.5 --gas-prices 0.025uzrn \
    --yes --output json 2>/dev/null || fail "Claim submission failed"

# Wait for round to complete (up to 60s)
ROUND_COMPLETE=false
for i in $(seq 1 30); do
    ROUNDS=$(./build/zeroned query knowledge rounds \
        --node http://127.0.0.1:26601 --output json 2>/dev/null | jq '.rounds | length' 2>/dev/null || echo "0")
    # Check for completed rounds or created facts
    FACTS=$(./build/zeroned query knowledge facts --domain test-domain \
        --node http://127.0.0.1:26601 --output json 2>/dev/null | jq '.facts | length' 2>/dev/null || echo "0")
    [ "$FACTS" -gt "0" ] && ROUND_COMPLETE=true && break
    sleep 2
done
$ROUND_COMPLETE || fail "PoT round did not complete within 60s"
pass "PoT round completed — fact created in test-domain"
```

**Note:** The exact CLI flags for `submit-claim` may differ. Adapt to match the actual CLI interface. If the round needs more time (depending on round length params), increase the timeout. The key assertion: a claim goes in, a fact comes out.

### Phase 7: Governance (Step 10)

```bash
# Step 10: Submit and pass a governance proposal
./build/zeroned tx gov submit-proposal \
    --title "Dress Rehearsal Test" \
    --description "Testing governance pipeline" \
    --type text \
    --deposit 10000000uzrn \
    --from val0 \
    --home "$COORD_HOME" \
    --keyring-backend test \
    --node http://127.0.0.1:26601 \
    --chain-id zerone-localnet \
    --gas auto --gas-adjustment 1.5 --gas-prices 0.025uzrn \
    --yes --output json 2>/dev/null || fail "Proposal submission failed"

sleep 3

# Get proposal ID
PROP_ID=$(./build/zeroned query gov proposals \
    --node http://127.0.0.1:26601 --output json 2>/dev/null | jq -r '.proposals[-1].id')

# Vote from all 4 validators
for v in val0 val1 val2 val3; do
    ./build/zeroned tx gov vote "$PROP_ID" yes \
        --from $v \
        --home "$COORD_HOME" \
        --keyring-backend test \
        --node http://127.0.0.1:26601 \
        --chain-id zerone-localnet \
        --gas auto --gas-adjustment 1.5 --gas-prices 0.025uzrn \
        --yes 2>/dev/null || warn "Vote from $v failed (non-fatal)"
done

# Wait for proposal to pass (depends on voting period — should be short on localnet)
PROP_PASSED=false
for i in $(seq 1 30); do
    STATUS=$(./build/zeroned query gov proposal "$PROP_ID" \
        --node http://127.0.0.1:26601 --output json 2>/dev/null | jq -r '.proposal.status' 2>/dev/null || echo "unknown")
    [ "$STATUS" = "PROPOSAL_STATUS_PASSED" ] && PROP_PASSED=true && break
    sleep 2
done
$PROP_PASSED || fail "Governance proposal did not pass (status: $STATUS)"
pass "Governance proposal $PROP_ID passed"
```

**Note:** If Zerone uses LIPs instead of standard cosmos-sdk gov proposals, adapt the commands. The point is: submit → vote → pass.

### Phase 8: Bank Transfer (Step 11)

```bash
# Step 11: Token transfer between accounts
ADDR0=$(./build/zeroned keys show val0 -a --keyring-backend test --home "$COORD_HOME")
ADDR1=$(./build/zeroned keys show val1 -a --keyring-backend test --home "$COORD_HOME")

BAL_BEFORE=$(./build/zeroned query bank balances "$ADDR1" \
    --node http://127.0.0.1:26601 --output json 2>/dev/null | jq -r '.balances[] | select(.denom=="uzrn") | .amount' 2>/dev/null || echo "0")

./build/zeroned tx bank send "$ADDR0" "$ADDR1" 1000000uzrn \
    --from val0 \
    --home "$COORD_HOME" \
    --keyring-backend test \
    --node http://127.0.0.1:26601 \
    --chain-id zerone-localnet \
    --gas auto --gas-adjustment 1.5 --gas-prices 0.025uzrn \
    --yes 2>/dev/null || fail "Bank transfer failed"

sleep 3

BAL_AFTER=$(./build/zeroned query bank balances "$ADDR1" \
    --node http://127.0.0.1:26601 --output json 2>/dev/null | jq -r '.balances[] | select(.denom=="uzrn") | .amount' 2>/dev/null || echo "0")

# Verify balance increased
[ "$BAL_AFTER" -gt "$BAL_BEFORE" ] || fail "Balance did not increase ($BAL_BEFORE → $BAL_AFTER)"
pass "Bank transfer verified ($ADDR0 → $ADDR1, +1 ZRN)"
```

### Phase 9: Shutdown & Final (Step 12)

```bash
# Step 12: Clean shutdown + test suite
scripts/localnet.sh stop
sleep 3

# Verify all processes stopped
REMAINING=$(pgrep -f "zeroned start" | wc -l)
[ "$REMAINING" -eq "0" ] || fail "$REMAINING zeroned processes still running"

# Run full test suite
go test ./... -count=1 -timeout 600s || fail "Test suite has failures"

pass "Clean shutdown + test suite green"

# ═══════════════════════════════════════════════════════════════
echo ""
echo "═══════════════════════════════════════════════════════════"
echo "  DRESS REHEARSAL: $STEP/$total_steps PASSED"
if [ "$STEP" -eq "$total_steps" ]; then
    echo "  ✅ VERDICT: READY FOR PUBLIC TESTNET"
else
    echo "  ❌ VERDICT: BLOCKED ($((total_steps - STEP)) steps failed)"
fi
echo "═══════════════════════════════════════════════════════════"
```

## Adaptation Notes

The exact CLI commands above are best-effort guesses based on the codebase. The session runner should:

1. Check `./build/zeroned tx --help` and `./build/zeroned query --help` for actual subcommand names
2. Adapt flag names (e.g., `--claim-type` vs `--type`, `--subject` vs `--content`)
3. If `scripts/localnet.sh` doesn't support `init` separately from `start`, restructure the script to allow axiom injection between the two
4. Adjust timeouts based on actual block time and governance period params

The structure matters more than the exact flags. If a command is wrong, fix the command — don't skip the test.

## Exit Criteria

1. `scripts/dress-rehearsal.sh` exists and is executable
2. Running it produces 12/12 PASS with zero manual intervention
3. PoT round completes (claim → fact)
4. Governance proposal passes (submit → vote → pass)
5. Bank transfer verified (balance change)
6. `go test ./...` green at the end
7. Total runtime < 15 minutes

## Commit Convention

```
test(e2e): dress rehearsal — full testnet launch pipeline
scripts: dress-rehearsal.sh end-to-end launch verification
```
