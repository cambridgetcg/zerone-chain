# R24-2 — Validator Lifecycle: Register → Delegate → Tier → Slash → Unjail

## Context

Zerone's staking has a 4-tier system (Apprentice → Verified → Scholar → Guardian) driven by stake, reputation, and verification accuracy. The localnet tests cover PoT rounds and basic delegation. This session tests the full validator lifecycle as an external operator would experience it.

## Prerequisites

- Localnet running

## Task

### 1. Inspect Staking Params & Tiers

```bash
$BINARY query zerone-staking params $Q_FLAGS | jq .

# Specifically: tier configs
$BINARY query zerone-staking params $Q_FLAGS | jq '.params.tier_configs'
```

**Document:**
- [ ] 4 tier configs with: name, min_stake, min_reputation, min_verifications, min_accuracy
- [ ] Reward multipliers per tier
- [ ] Selection weights (VRF weighting for PoT rounds)
- [ ] Slash multipliers per tier
- [ ] Unbonding period
- [ ] Max validator count

### 2. Register a New Validator

```bash
# Create a new key for our validator
$BINARY keys add new-validator --keyring-backend test --home $HOME_DIR
VAL_ADDR=$($BINARY keys show new-validator -a --keyring-backend test --home $HOME_DIR)

# Fund it generously
$BINARY tx bank send $VAL0_ADDR $VAL_ADDR 1000000000000uzrn --from val0 $TX_FLAGS
sleep 3

# Get the validator consensus pubkey
# (On a real node this comes from zeroned tendermint show-validator)
# On localnet we need to generate one
VAL_PUBKEY=$($BINARY keys show new-validator --keyring-backend test --home $HOME_DIR -p)

$BINARY tx zerone-staking register-validator \
    --moniker "Test-Validator-5" \
    --did "did:zerone:validator-5" \
    --consensus-pubkey "$VAL_PUBKEY" \
    --self-delegation 100000000uzrn \
    --commission-bps 500 \
    --website "https://example.com" \
    --details "Test validator for R24" \
    --from new-validator $TX_FLAGS
```

**Verify:**
- [ ] Validator registered
- [ ] Query: `$BINARY query zerone-staking validator $VAL_ADDR $Q_FLAGS`
  - Tier = APPRENTICE (initial)
  - Self delegation matches
  - Reputation = 0
  - is_active = true (or false if below active set?)
  - Commission set correctly
- [ ] Validator appears in validator list
- [ ] What happens if consensus pubkey is wrong format?
- [ ] What's the minimum self-delegation for registration?

**Issues to look for:**
- Consensus pubkey format — CometBFT requires a specific encoding. Does the CLI handle this?
- Does registration require the node to be running and synced? Or can you register before starting?
- Is DID required for validator registration? Or optional?

### 3. Delegation

```bash
# Delegate from a separate account to our new validator
$BINARY tx zerone-staking delegate $VAL_ADDR 500000000000uzrn \
    --from val0 $TX_FLAGS

# Query delegations
$BINARY query zerone-staking delegations $VAL_ADDR $Q_FLAGS
```

**Verify:**
- [ ] Delegation recorded
- [ ] Validator's total_stake increased
- [ ] Delegator's balance decreased
- [ ] Can delegate to a jailed validator? (should probably fail)

### 4. Tier Progression

Check what triggers tier promotion:

```bash
# Current tier after registration + delegation
$BINARY query zerone-staking validator $VAL_ADDR $Q_FLAGS | jq '.validator.tier'

# What are the tier boundaries?
# Apprentice → Verified: needs min_stake + min_reputation + min_verifications
```

**Verify:**
- [ ] New validator starts at Apprentice
- [ ] After meeting Verified requirements, tier updates (at epoch boundary? block-by-block?)
- [ ] Tier affects reward multiplier and VRF selection weight
- [ ] Does tier auto-promote or require a tx?

**Issues to look for:**
- How long does it take for tier progression on localnet?
- Is there a `promote-tier` tx or is it automatic?
- What if reputation is high enough but verifications are not?

### 5. Undelegation

```bash
$BINARY tx zerone-staking undelegate $VAL_ADDR 100000000000uzrn \
    --from val0 $TX_FLAGS

# Check unbonding
$BINARY query zerone-staking unbonding-delegations $VAL0_ADDR $Q_FLAGS
```

**Verify:**
- [ ] Unbonding entry created with correct amount
- [ ] Completion height set (current + unbonding period)
- [ ] Funds returned after unbonding period
- [ ] Validator's stake reduced immediately? Or after unbonding?

### 6. Redelegation

```bash
$BINARY tx zerone-staking redelegate $VAL_ADDR $VAL1_ADDR 50000000000uzrn \
    --from val0 $TX_FLAGS
```

**Verify:**
- [ ] Delegation moved from one validator to another
- [ ] No unbonding period for redelegation (or is there?)
- [ ] Can you redelegate during unbonding? (usually not in Cosmos)

### 7. Slashing (Simulate)

```bash
# If localnet test already covers this, reference it
# Otherwise: stop the new validator's node and wait for downtime slash

# Check slashing state
$BINARY query zerone-staking validator $VAL_ADDR $Q_FLAGS | jq '{
    jailed: .validator.jailed,
    jail_reason: .validator.jail_reason,
    slash_count: .validator.slash_count,
    unjail_after_block: .validator.unjail_after_block
}'
```

**Verify:**
- [ ] Validator jailed after sufficient downtime
- [ ] jail_reason populated
- [ ] slash_count incremented
- [ ] Stake slashed by the correct amount (per tier's slash_multiplier)
- [ ] Tier demotion on slash? (if slash_count exceeds tier's max_slash_count)

### 8. Unjail

```bash
# Wait for unjail_after_block
$BINARY tx slashing unjail --from new-validator $TX_FLAGS
# Or: $BINARY tx zerone-staking unjail --from new-validator $TX_FLAGS
```

**Verify:**
- [ ] Unjail succeeds after cooldown
- [ ] Unjail before cooldown fails
- [ ] Validator rejoins active set
- [ ] Reputation affected?

**Issues to look for:**
- Is unjail via standard Cosmos `x/slashing` or Zerone's custom staking?
- What's the unjail cooldown on localnet?

### 9. Commission Update

```bash
$BINARY tx zerone-staking update-stake \
    --commission-bps 1000 \
    --from new-validator $TX_FLAGS
```

**Verify:**
- [ ] Commission updated
- [ ] Is there a max commission rate?
- [ ] Is there a max commission change per period?
- [ ] Does commission affect delegation attractiveness?

### 10. Validator Deregistration

Is there a way to gracefully leave?

```bash
# Check if there's a deregister command
$BINARY tx zerone-staking --help 2>&1 | grep -i "deregister\|leave\|exit\|remove"
```

**Verify:**
- [ ] Graceful exit mechanism exists (or document the gap)
- [ ] If no explicit deregister: is the path to undelegate everything + wait for unbonding?
- [ ] What happens to delegators when a validator leaves?

## Report Template

```markdown
### Step N: <name>
**Status:** PASS / FAIL / BLOCKED
**Observation:** <what happened>
**Tier State:** <before → after>
**Issue:** <if any>
```

## Exit Criteria

1. Full validator lifecycle tested (register → delegate → tier check → undelegate → redelegate)
2. Slashing and unjail behaviour documented
3. Tier progression mechanism documented (auto vs manual, timing)
4. Commission update tested
5. Validator exit path documented
6. Minimum requirements to become each tier documented
7. Report written to `docs/validator-lifecycle-report.md`

## Commit Convention

```
test(staking): validator lifecycle e2e on localnet
docs(staking): validator lifecycle report — tier progression, slashing, unjail
fix(staking): <any issues>
```
