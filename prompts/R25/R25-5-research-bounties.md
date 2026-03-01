# R25-5 — Research & Bounties: Incentivised Truth Discovery

## Context

The research module (`x/research`) enables formal research submissions with peer review, while bounties incentivise investigation into specific questions. The tree module (`x/tree`) handles projects, tasks, and services — the "productive economy" layer. The vesting_rewards module links all of this back to truth-dependent token release.

This session tests the economic incentive layer: can agents and humans earn real rewards for discovering and validating truth?

## Prerequisites

- Localnet running

## Task

### 1. Submit Research

```bash
# Create researcher account
$BINARY keys add researcher1 --keyring-backend test --home $HOME_DIR
RESEARCHER=$($BINARY keys show researcher1 -a --keyring-backend test --home $HOME_DIR)
$BINARY tx bank send $VAL0_ADDR $RESEARCHER 500000000uzrn --from val0 $TX_FLAGS
sleep 3

$BINARY tx research submit-research \
    "Replication study of water boiling point claim" \
    "Independent measurement of water boiling point under standard conditions" \
    "general" \
    "replication" \
    $FACT_ID \
    10000000 \
    --from researcher1 $TX_FLAGS
```

**Verify:**
- [ ] Research submission created
- [ ] Status: "submitted"
- [ ] Target fact linked
- [ ] Stake escrowed
- [ ] Research types accepted: replication, fraud_investigation, methodology_audit, data_validation

**Document:**
- [ ] Min stake for research submission
- [ ] How does research relate to claims? (research validates existing facts vs claims create new ones)

### 2. Peer Review

```bash
RESEARCH_ID="<from events>"

# Validator reviews the research
$BINARY tx research review-research $RESEARCH_ID \
    "approve" 85 \
    "Methodology is sound, measurements are within expected range" \
    --from val0 $TX_FLAGS

# Second review
$BINARY tx research review-research $RESEARCH_ID \
    "approve" 90 \
    "Replicated successfully, consistent with established knowledge" \
    --from val1 $TX_FLAGS
```

**Verify:**
- [ ] Reviews recorded
- [ ] Verdicts: approve, reject, revise
- [ ] Score (0-100) tracked
- [ ] Aggregate score computed
- [ ] Min reviews for resolution?
- [ ] Who can review? (any validator? any qualified account?)

### 3. Resolve Research

```bash
$BINARY tx research resolve-research $RESEARCH_ID \
    --from val0 $TX_FLAGS
```

**Verify:**
- [ ] Research resolved based on reviews
- [ ] Status → "accepted" or "rejected"
- [ ] If accepted: target fact confidence boosted?
- [ ] Researcher gets reward?
- [ ] Reviewers get reward?

### 4. Challenge Research

```bash
# Submit controversial research
$BINARY tx research submit-research \
    "Questioning thermodynamics" \
    "Investigation suggesting perpetual motion is possible" \
    "general" \
    "methodology_audit" \
    $FACT_ID \
    10000000 \
    --from researcher1 $TX_FLAGS

R2_ID="<from events>"

# Challenge it
$BINARY tx research challenge-research $R2_ID \
    "This contradicts well-established physics" \
    10000000 \
    --from val0 $TX_FLAGS
```

**Verify:**
- [ ] Challenge recorded
- [ ] Research enters review
- [ ] Challenge stake locked
- [ ] Resolution includes challenge evidence

### 5. Create a Bounty

```bash
$BINARY tx research create-bounty \
    "Measure water boiling point at 3000m altitude" \
    "Empirical measurement needed for altitude-adjusted boiling point claims" \
    "general" \
    50000000 \
    --deadline-blocks 10000 \
    --from val0 $TX_FLAGS
```

**Verify:**
- [ ] Bounty created with reward escrowed
- [ ] Status: "open"
- [ ] Deadline set
- [ ] Anyone can see and claim it
- [ ] Reward amount locked

### 6. Claim and Fulfil a Bounty

```bash
BOUNTY_ID="<from events>"

# Claim the bounty (signal intent to work on it)
$BINARY tx research claim-bounty $BOUNTY_ID \
    --from researcher1 $TX_FLAGS

# Fulfil (submit deliverable)
$BINARY tx research fulfil-bounty $BOUNTY_ID \
    "Measured boiling point at 3000m: 90.0°C (±0.5°C), consistent with atmospheric pressure reduction" \
    --from researcher1 $TX_FLAGS
```

**Verify:**
- [ ] Bounty claimed (exclusive? or multiple claimants?)
- [ ] Fulfilment submitted
- [ ] Review process for fulfilment?
- [ ] Reward distributed on approval
- [ ] Deadline enforcement (expired bounties)

### 7. Fund Research

```bash
$BINARY tx research fund-research $RESEARCH_ID 25000000 \
    --from val0 $TX_FLAGS
```

**Verify:**
- [ ] External funding for research
- [ ] Funds escrowed
- [ ] Released to researcher on successful resolution

### 8. Vesting Rewards Deep Dive

```bash
# Check all vesting schedules
$BINARY query vesting-rewards params $Q_FLAGS | jq .

# Check category configs
$BINARY query vesting-rewards category-configs $Q_FLAGS 2>/dev/null

# Check specific vesting
$BINARY query vesting-rewards vesting-schedule $VESTING_ID $Q_FLAGS 2>/dev/null
```

**Verify:**
- [ ] Vesting schedules linked to accepted claims
- [ ] Half-life release curve per category (axiomatic, empirical, formal_proof, etc.)
- [ ] Cliff period before any release
- [ ] Reserve amount permanently locked (challenge insurance)
- [ ] Acceleration on: defense wins, replications, corroborations, citations
- [ ] Clawback on falsification (fact → DISPROVEN)

**Vesting Economics Questions:**
- [ ] What's the actual reward for a verified claim? (in uzrn)
- [ ] How long until first payout?
- [ ] What triggers clawback? (challenge success? or explicit falsification?)
- [ ] Does the reserve ever get released? (or burned?)

### 9. Block Reward Distribution

```bash
# Check how block rewards are distributed
$BINARY query vesting-rewards block-distribution $Q_FLAGS 2>/dev/null
```

**Verify:**
- [ ] Block producer gets reward
- [ ] Research fund gets 3.33%
- [ ] Development fund gets its share
- [ ] Protocol share distributed (citations, verification, treasury)
- [ ] Founder share (Yu: 7% of research = 0.23% of total)
- [ ] AI operations share (if implemented)

### 10. Tree Module: Project Creation

```bash
$BINARY tx tree create-project \
    "Altitude Boiling Point Database" \
    "Comprehensive database of water boiling points at various altitudes" \
    "general" \
    50000000 \
    --from val0 $TX_FLAGS

PROJECT_ID="<from events>"

# Add a task
$BINARY tx tree add-task $PROJECT_ID \
    "Collect measurements from 0-5000m" \
    "Systematic measurements every 500m" \
    "data_collection" \
    25000000 \
    --from val0 $TX_FLAGS
```

**Verify:**
- [ ] Project created with phase tracking
- [ ] Task added with bounty
- [ ] Budget tracked
- [ ] Can assign tasks to specific agents/humans

### 11. Service Deployment (Tree Leaf)

```bash
# After project completion, deploy a service
$BINARY tx tree deploy-service $PROJECT_ID \
    "Boiling Point API" \
    "Query altitude-adjusted boiling points" \
    "data_api" \
    --price-per-call 1000 \
    --from val0 $TX_FLAGS
```

**Verify:**
- [ ] Service deployed from project
- [ ] Price set
- [ ] Callable by other agents
- [ ] Revenue tracked
- [ ] Integration with BVM? (can BVM call a service?)

### 12. Opportunity Detection (Tree Seed)

```bash
$BINARY tx tree detect-opportunity "general" \
    --from val0 $TX_FLAGS

$BINARY tx tree begin-seeding \
    --from val0 $TX_FLAGS
```

**Verify:**
- [ ] Opportunity seeds detected from knowledge gaps
- [ ] Seeding creates project proposals
- [ ] Links back to demand signals from knowledge module

## Report Template

```markdown
### Step N: <name>
**Status:** PASS / FAIL / BLOCKED
**Economics:** <reward amount, timing, distribution>
**Observation:** <what happened>
**Issue:** <if any>
```

## Exit Criteria

1. Research submission + peer review + resolution tested
2. Bounty creation + claim + fulfilment tested
3. Research funding tested
4. Vesting reward economics documented (amounts, timing, categories)
5. Block reward distribution documented
6. Tree project + task lifecycle tested
7. Service deployment tested
8. Opportunity detection tested
9. Full economic loop documented: claim → verify → vest → claim rewards
10. Report written to `docs/research-bounties-report.md`

## Commit Convention

```
test(research): research submission, review, and bounty lifecycle e2e
test(tree): project and service deployment on localnet
docs(economics): research-bounties report — incentive layer analysis
fix(research): <any issues>
```
