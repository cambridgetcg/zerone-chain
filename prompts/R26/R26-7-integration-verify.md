# R26-7 — End-to-End Integration: The Full Truth-Seeking Loop

## Context

R26-1 through R26-6 wire the disconnected modules together. This session verifies the complete loop works end-to-end on a live localnet with all fixes active.

**This session depends on ALL previous R26 sessions being complete and merged.**

## The Loop

```
Register (human + agent) → Form Partnership → Qualify for Domain →
Submit Claim (through partnership) → Qualified Verifiers Selected →
Commit-Reveal Verification → Reward Distribution (through partnership split) →
Vesting → Block Rewards Fund Research → Submit Research → Auto-Resolve →
Bounty Created → Bounty Fulfilled
```

Every arrow in this chain was broken before R26. Now we test them all.

## Task

### 1. Fresh Localnet

```bash
cd ~/Desktop/zerone
scripts/localnet.sh stop
scripts/localnet.sh clean  # Fresh genesis
scripts/localnet.sh start
```

### 2. Register Accounts with Types

```bash
# Create and fund accounts
$BINARY keys add alice --keyring-backend test --home $HOME_DIR  # Human
$BINARY keys add sage1 --keyring-backend test --home $HOME_DIR  # Agent
$BINARY keys add rogue --keyring-backend test --home $HOME_DIR  # Agent (adversarial)
ALICE=$($BINARY keys show alice -a --keyring-backend test --home $HOME_DIR)
SAGE1=$($BINARY keys show sage1 -a --keyring-backend test --home $HOME_DIR)
ROGUE=$($BINARY keys show rogue -a --keyring-backend test --home $HOME_DIR)

# Fund all accounts
for addr in $ALICE $SAGE1 $ROGUE; do
    $BINARY tx bank send $VAL0_ADDR $addr 1000000000uzrn --from val0 $TX_FLAGS
done
sleep 6

# Register with account types
$BINARY tx zerone-auth register-account human --from alice $TX_FLAGS
$BINARY tx zerone-auth register-account agent --from sage1 $TX_FLAGS
$BINARY tx zerone-auth register-account agent --from rogue $TX_FLAGS
sleep 6
```

**Verify:**
- [ ] All accounts registered with correct types
- [ ] Capability flags match expected defaults

### 3. Test Capability Enforcement (R26-2)

```bash
# If contract accounts are tested: create one and verify it CAN'T submit claims
# For now, verify registered accounts CAN do their expected operations
$BINARY query zerone-auth account $ALICE $Q_FLAGS  # Check capabilities
$BINARY query zerone-auth account $SAGE1 $Q_FLAGS
```

### 4. Form Partnership (R26-4)

```bash
$BINARY tx partnerships propose-partnership $ALICE $SAGE1 50 50 --from alice $TX_FLAGS
sleep 3
# Get partnership ID from events
PARTNERSHIP_ID=<from_events>
$BINARY tx partnerships accept-partnership $PARTNERSHIP_ID --from sage1 $TX_FLAGS
sleep 3

$BINARY query partnerships partnership $PARTNERSHIP_ID $Q_FLAGS
```

**Verify:**
- [ ] Partnership active with 50/50 split
- [ ] Alice (human) and Sage1 (agent) are participants

### 5. Qualify for Domain (R26-3)

```bash
# Qualify val0 for "general" domain via stake
$BINARY tx qualification qualify-stake val0 general 100000000uzrn --from val0 $TX_FLAGS
# Qualify val1 too
$BINARY tx qualification qualify-stake val1 general 100000000uzrn --from val1 $TX_FLAGS
sleep 6

$BINARY query qualification qualified-validators general $Q_FLAGS
```

**Verify:**
- [ ] val0 and val1 qualified for "general"
- [ ] Other validators NOT qualified

### 6. Submit Claim Through Partnership (R26-4)

```bash
$BINARY tx knowledge submit-claim \
    "The speed of light in vacuum is approximately 299,792,458 metres per second" \
    general computational 1000000 \
    --partnership-id $PARTNERSHIP_ID \
    --from alice $TX_FLAGS
sleep 3
CLAIM_ID=<from_events>

# Verify claim stored with partnership_id
$BINARY query knowledge claim $CLAIM_ID $Q_FLAGS
```

**Verify:**
- [ ] Claim created with partnership_id set
- [ ] Verification round created
- [ ] Only qualified validators selected for the round

### 7. Verification Round (R26-3 gating)

```bash
# Wait for commit phase
ROUND_ID=<from_claim>
$BINARY query knowledge round $ROUND_ID $Q_FLAGS

# Qualified validator commits
$BINARY tx knowledge submit-commitment $ROUND_ID <commitment_hash> --from val0 $TX_FLAGS

# Unqualified validator tries to commit — should FAIL
$BINARY tx knowledge submit-commitment $ROUND_ID <commitment_hash> --from val2 $TX_FLAGS
# Expect error: unqualified verifier

# Reveal phase
$BINARY tx knowledge submit-reveal $ROUND_ID <reveal_data> --from val0 $TX_FLAGS
sleep 6
```

**Verify:**
- [ ] Qualified validator's commitment accepted
- [ ] Unqualified validator's commitment rejected (if qualified validators exist)
- [ ] Round completes, claim verified
- [ ] RecordVerificationOutcome called (check qualification metrics)

### 8. Reward Distribution Through Partnership (R26-1 + R26-4)

```bash
# Check block rewards are being minted (R26-1)
$BINARY query bank balances <protocol_treasury_addr> $Q_FLAGS
$BINARY query bank balances <research_fund_addr> $Q_FLAGS

# Check partnership received rewards (R26-4)
$BINARY query partnerships partnership $PARTNERSHIP_ID $Q_FLAGS
# Common pot should have reward tokens
# Alice and Sage1 should have their split portions
```

**Verify:**
- [ ] Block rewards minting (non-zero fund balances)
- [ ] Claim rewards routed through partnership split
- [ ] Alice gets her 50%, Sage1 gets 50%
- [ ] Revenue split ratios correct (55/22/19.67/3.33)

### 9. Research + Auto-Resolution (R26-5)

```bash
# Submit research (funded by research fund)
$BINARY tx research submit-research \
    "Replication of Light Speed Measurement" \
    "Abstract describing methodology" \
    "QmEvidenceHash" \
    general \
    10000000uzrn \
    --from sage1 $TX_FLAGS
sleep 3
RESEARCH_ID=<from_events>

# Submit reviews
$BINARY tx research review-research $RESEARCH_ID 8 "Rigorous methodology" --from alice $TX_FLAGS
$BINARY tx research review-research $RESEARCH_ID 9 "Excellent" --from val0 $TX_FLAGS
sleep 3

# Wait for review period
# ... wait appropriate blocks ...

# Check auto-resolution
$BINARY query research research $RESEARCH_ID $Q_FLAGS
```

**Verify:**
- [ ] Research auto-resolved after conditions met
- [ ] Escrowed stake returned to researcher
- [ ] No governance proposal needed

### 10. Tree Module Determinism (R26-6)

```bash
# Create a project and verify state consistency
$BINARY tx tree create-project "Integration Test Project" "Testing cross-validator state" --from alice $TX_FLAGS
sleep 6

# Query on all validators
for port in 9090 9091 9092 9093; do
    $BINARY query tree projects --grpc-addr localhost:$port 2>/dev/null | grep -c "project_id"
done
```

**Verify:**
- [ ] Same project count on all validators
- [ ] app_hash matches across validators

### 11. Negative Tests

```bash
# Claim with non-existent partnership → rejected
$BINARY tx knowledge submit-claim "bad claim" general computational 1000000 \
    --partnership-id "fake-partnership" --from alice $TX_FLAGS
# Expect: error

# Claim with frozen partnership → rejected
$BINARY tx partnerships raise-coercion $PARTNERSHIP_ID --from sage1 $TX_FLAGS
sleep 3
$BINARY tx knowledge submit-claim "frozen claim" general computational 1000000 \
    --partnership-id $PARTNERSHIP_ID --from alice $TX_FLAGS
# Expect: error (partnership frozen)
```

### 12. Summary Report

Document:
- [ ] Which connections work end-to-end
- [ ] Any remaining gaps or unexpected failures
- [ ] Performance observations (block times with new logic)
- [ ] Testnet readiness assessment: can we launch?

## Success Criteria

- [ ] Full loop completes: register → partner → qualify → claim → verify → reward → vest
- [ ] Block rewards funding research and development
- [ ] Partnership rewards routing correctly
- [ ] Qualification gating verification
- [ ] Research auto-resolving
- [ ] Tree state consistent across validators
- [ ] Negative tests catching invalid operations
- [ ] **Testnet readiness: YES or list of remaining blockers**
