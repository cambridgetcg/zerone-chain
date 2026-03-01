# R27-3 — End-to-End Integration: The Full Truth-Seeking Loop

## Context

R26 wired cross-module connections but R26-7 (integration verify) never ran. This session runs the complete loop on a fresh localnet with all wiring active. It absorbs and expands the R26-7 scope.

**This is the single most important test before testnet.** If the loop works here, we can launch.

## The Loop

```
Register (human + agent)
  → Form Partnership
    → Qualify for Domain
      → Submit Claim (through partnership)
        → Qualified Verifiers Selected (R26-3)
          → Commit-Reveal Verification
            → Reward Distribution (through partnership split) (R26-4)
              → Block Rewards Fund Research (R26-1, already wired)
                → Submit Research → Auto-Resolve (R26-5)
                  → Bounty Created → Bounty Fulfilled (R26-5)
```

Plus negative tests for capability enforcement (R26-2, already wired) and determinism (R26-6).

## Task

### Phase 1: Setup (Fresh Chain)

```bash
cd ~/Desktop/zerone
scripts/localnet.sh stop
scripts/localnet.sh clean
scripts/localnet.sh start
sleep 10

# Verify chain is producing blocks
$BINARY status $Q_FLAGS | jq '.sync_info.latest_block_height'
```

Create a reusable test script at `scripts/e2e-full-loop.sh` that can be re-run on any localnet.

### Phase 2: Account Registration + Capabilities

```bash
# Create accounts
for name in alice sage1 rogue; do
    $BINARY keys add $name --keyring-backend test --home $HOME_DIR 2>/dev/null
done

ALICE=$($BINARY keys show alice -a --keyring-backend test --home $HOME_DIR)
SAGE1=$($BINARY keys show sage1 -a --keyring-backend test --home $HOME_DIR)
ROGUE=$($BINARY keys show rogue -a --keyring-backend test --home $HOME_DIR)

# Fund
for addr in $ALICE $SAGE1 $ROGUE; do
    $BINARY tx bank send $VAL0_ADDR $addr 1000000000uzrn --from val0 $TX_FLAGS
done
sleep 6

# Register with types
$BINARY tx zerone-auth register-account human --from alice $TX_FLAGS
$BINARY tx zerone-auth register-account agent --from sage1 $TX_FLAGS
$BINARY tx zerone-auth register-account agent --from rogue $TX_FLAGS
sleep 6

# Verify capabilities
for name in alice sage1 rogue; do
    addr=$($BINARY keys show $name -a --keyring-backend test --home $HOME_DIR)
    echo "=== $name ==="
    $BINARY query zerone-auth account $addr $Q_FLAGS
done
```

**Checkpoint 1:**
- [ ] All accounts registered with correct types
- [ ] Human and agent have full capabilities
- [ ] account_type stored correctly

### Phase 3: Block Rewards Flowing

```bash
# Submit a few txs to generate block rewards
for i in 1 2 3; do
    $BINARY tx bank send $VAL0_ADDR $ALICE 1uzrn --from val0 $TX_FLAGS
    sleep 6
done

# Check fund balances
echo "=== Protocol Treasury ==="
$BINARY query vesting-rewards protocol-treasury-balance $Q_FLAGS 2>/dev/null || \
    $BINARY query bank balances $($BINARY query auth module-account protocol_treasury -o json $Q_FLAGS | jq -r '.account.base_account.address') $Q_FLAGS

echo "=== Research Fund ==="
$BINARY query bank balances $($BINARY query auth module-account research_fund -o json $Q_FLAGS | jq -r '.account.base_account.address') $Q_FLAGS
```

**Checkpoint 2:**
- [ ] Fund balances non-zero after blocks with transactions
- [ ] Revenue split ratios approximately correct (55/22/19.67/3.33)

### Phase 4: Partnership Formation

```bash
$BINARY tx partnerships propose-partnership $SAGE1 50 50 --from alice $TX_FLAGS
sleep 6
# Get partnership ID from tx events
PARTNERSHIP_ID=$($BINARY query tx <txhash> -o json $Q_FLAGS | jq -r '.events[] | select(.type=="partnership_proposed") | .attributes[] | select(.key=="partnership_id") | .value')

$BINARY tx partnerships accept-partnership $PARTNERSHIP_ID --from sage1 $TX_FLAGS
sleep 6

$BINARY query partnerships partnership $PARTNERSHIP_ID $Q_FLAGS
```

**Checkpoint 3:**
- [ ] Partnership active
- [ ] Alice (human) + Sage1 (agent) linked
- [ ] 50/50 split configured

### Phase 5: Domain Qualification

```bash
# Qualify val0 and val1 for "general" domain
$BINARY tx qualification qualify-stake val0_addr general 100000000uzrn --from val0 $TX_FLAGS
$BINARY tx qualification qualify-stake val1_addr general 100000000uzrn --from val1 $TX_FLAGS
sleep 6

# Verify
$BINARY query qualification qualified-validators general $Q_FLAGS
```

**Checkpoint 4:**
- [ ] val0 and val1 qualified for "general"
- [ ] val2 and val3 NOT qualified

### Phase 6: Claim Through Partnership + Qualified Verification

```bash
# Submit claim through partnership
$BINARY tx knowledge submit-claim \
    "The speed of light in vacuum is 299792458 m/s" \
    general computational 1000000 \
    --partnership-id $PARTNERSHIP_ID \
    --from alice $TX_FLAGS
sleep 6

CLAIM_ID=<from_events>
ROUND_ID=<from_claim_query>

# Check verification round — only qualified validators should be selected
$BINARY query knowledge verification-round $ROUND_ID $Q_FLAGS

# Qualified validator commits
COMMIT_HASH=$(echo -n "accept|salt123" | sha256sum | cut -d' ' -f1)
$BINARY tx knowledge submit-commitment $ROUND_ID $COMMIT_HASH --from val0 $TX_FLAGS
sleep 3

# Unqualified validator tries — should FAIL
$BINARY tx knowledge submit-commitment $ROUND_ID $COMMIT_HASH --from val2 $TX_FLAGS
# Expect error

# Reveal
$BINARY tx knowledge submit-reveal $ROUND_ID "accept" "salt123" --from val0 $TX_FLAGS
sleep 6

# Check result
$BINARY query knowledge claim $CLAIM_ID $Q_FLAGS
```

**Checkpoint 5:**
- [ ] Claim created with partnership_id
- [ ] Only qualified validators in round
- [ ] Unqualified validator rejected
- [ ] Round completes, claim verified
- [ ] Rewards routed through partnership (check partnership common pot)

### Phase 7: Research Auto-Resolution

```bash
# Submit research
$BINARY tx research submit-research \
    "Speed of Light Measurement Methodology" \
    "Replication study using interferometry" \
    "QmEvidenceHash123" \
    general \
    10000000uzrn \
    --from sage1 $TX_FLAGS
sleep 6
RESEARCH_ID=<from_events>

# Submit reviews (need min_reviewer_count)
$BINARY tx research review-research $RESEARCH_ID 8 "Solid methodology" --from alice $TX_FLAGS
$BINARY tx research review-research $RESEARCH_ID 9 "Excellent work" --from val0 $TX_FLAGS
sleep 6

# Check status — should be "under_review"
$BINARY query research research $RESEARCH_ID $Q_FLAGS

# Wait for review_period_blocks...
# (on localnet this may need param adjustment for speed)
# Check again after period
$BINARY query research research $RESEARCH_ID $Q_FLAGS
```

**Checkpoint 6:**
- [ ] Research submitted with escrowed stake
- [ ] Reviews recorded
- [ ] Auto-resolves after period + min reviews (or document if period too long for test)

### Phase 8: Negative Tests

```bash
# 1. Claim with fake partnership → rejected
$BINARY tx knowledge submit-claim "bad" general computational 1000000 \
    --partnership-id "nonexistent" --from alice $TX_FLAGS
# Expect: error

# 2. Coercion freeze → claim blocked
$BINARY tx partnerships raise-coercion $PARTNERSHIP_ID --from sage1 $TX_FLAGS
sleep 3
$BINARY tx knowledge submit-claim "frozen" general computational 1000000 \
    --partnership-id $PARTNERSHIP_ID --from alice $TX_FLAGS
# Expect: error (partnership frozen)

# 3. Verify coercion auto-expires
# Wait for CoercionReviewBlocks...
$BINARY query partnerships partnership $PARTNERSHIP_ID $Q_FLAGS
# Should return to active after expiry

# 4. Tree determinism — create project, verify all validators agree
$BINARY tx tree create-project "E2E Test Project" "Determinism check" --from alice $TX_FLAGS
sleep 6
for port in 9090 9091 9092 9093; do
    echo "=== gRPC $port ==="
    $BINARY query tree projects --grpc-addr localhost:$port 2>/dev/null | grep -c "project_id"
done
```

**Checkpoint 7:**
- [ ] Fake partnership rejected
- [ ] Frozen partnership blocks claims
- [ ] Coercion auto-expires
- [ ] Tree state consistent across validators

### Phase 9: Report

Write results to `docs/e2e-full-loop-report.md`:

1. Checkpoint summary (pass/fail for each)
2. Any failures — root cause + severity
3. Block reward accumulation over the test period
4. Timing observations (how long each phase took)
5. **Testnet readiness verdict: LAUNCH / NOT YET (with blockers)**

## Files to Create

- `scripts/e2e-full-loop.sh` — Reusable E2E test script
- `docs/e2e-full-loop-report.md` — Results report

## Success Criteria

- [ ] All 7 checkpoints pass
- [ ] Script is reusable (can re-run on fresh localnet)
- [ ] Report includes testnet readiness verdict
- [ ] No consensus-breaking issues discovered
