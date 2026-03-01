# R25-4 — Dispute Resolution: Evidence, Arbitration, and Escalation

## Context

When a challenge or contradiction can't be resolved by simple commit-reveal voting, ZERONE has a formal dispute system (`x/disputes`) with evidence management (`x/evidence_mgmt`), capture challenges (`x/capture_challenge`), and capture defense (`x/capture_defense`).

This is the appeals court for knowledge. It's critical for credibility — if wrong facts can't be corrected through disputes, the entire knowledge system loses integrity.

## Prerequisites

- Localnet running

## Task

### 1. Set Up the Scene

Create a contested fact that needs dispute resolution:

```bash
# Create accounts
# submitter: person who submitted the original fact
# challenger: person who disagrees
# arbiter: will resolve the dispute (should be high-tier validator)

$BINARY keys add disputer --keyring-backend test --home $HOME_DIR
DISPUTER=$($BINARY keys show disputer -a --keyring-backend test --home $HOME_DIR)
$BINARY tx bank send $VAL0_ADDR $DISPUTER 500000000uzrn --from val0 $TX_FLAGS
sleep 3

# Submit a claim, get it verified (use val0/val1 commit-reveal from R25-1)
$BINARY tx knowledge submit-claim \
    "Earth is approximately 6000 years old based on biblical genealogy" \
    general empirical 2000000 \
    --from disputer $TX_FLAGS

# Complete verification (even if stub accepts it — that's the problem we're testing)
# ...
FACT_ID="<once verified>"
```

### 2. Initiate a Dispute

```bash
$BINARY tx disputes initiate-dispute \
    $FACT_ID \
    10000000 \
    "This claim contradicts overwhelming geological and radiometric evidence" \
    --from val0 $TX_FLAGS
```

**Verify:**
- [ ] Dispute created with ID
- [ ] Stake escrowed from initiator
- [ ] Dispute status: active/pending
- [ ] Fact status affected? (should it change to DISPUTED?)
- [ ] Who can initiate? Any account? Only qualified?

**Document:**
- [ ] Dispute params (min_stake, resolution_period, escalation_threshold)
- [ ] Dispute types (is it always about a specific fact?)

### 3. Submit Evidence (Commit Phase)

```bash
DISPUTE_ID="<from events>"

# Evidence submission uses commit-reveal (prevents copying)
EVIDENCE="Radiometric dating of zircon crystals consistently dates Earth formation to 4.54 billion years"
EVIDENCE_SALT=$(openssl rand -hex 16)
EVIDENCE_HASH=$(echo -n "${EVIDENCE}${EVIDENCE_SALT}" | shasum -a 256 | cut -d' ' -f1)

$BINARY tx disputes commit-evidence $DISPUTE_ID $EVIDENCE_HASH \
    --from val0 $TX_FLAGS
```

**Verify:**
- [ ] Evidence commitment accepted
- [ ] Multiple parties can submit evidence
- [ ] Commitment is blinded (hash only, no content visible)

### 4. Reveal Evidence

```bash
$BINARY tx disputes reveal-evidence $DISPUTE_ID "$EVIDENCE" "$EVIDENCE_SALT" \
    --from val0 $TX_FLAGS
```

**Verify:**
- [ ] Reveal matches commitment hash
- [ ] Evidence now visible to arbiters
- [ ] Timing enforced (can't reveal before commit phase ends)
- [ ] Mismatched reveals rejected

### 5. Submit Evidence via Evidence Management

```bash
# Test the separate evidence_mgmt module
$BINARY tx evidence-mgmt submit-evidence \
    "$EVIDENCE" \
    "scientific_study" \
    --from val0 $TX_FLAGS

EVIDENCE_ID="<from events>"

# Transfer custody (chain of evidence)
$BINARY tx evidence-mgmt transfer-custody $EVIDENCE_ID $VAL1_ADDR \
    --from val0 $TX_FLAGS

# Verify evidence
$BINARY tx evidence-mgmt verify-evidence $EVIDENCE_ID \
    --from val1 $TX_FLAGS
```

**Verify:**
- [ ] Evidence stored with chain-of-custody
- [ ] Custody transfer recorded (who held it, when)
- [ ] Evidence verification works
- [ ] How does this integrate with disputes? Can dispute reference evidence_id?

### 6. Arbiter Vote

```bash
# Who are the arbiters? How are they selected?
# Likely high-tier validators (Scholar/Guardian)

$BINARY tx disputes arbiter-vote $DISPUTE_ID "uphold" \
    "The challenger's evidence is scientifically rigorous and contradicts the original claim" \
    --from val0 $TX_FLAGS

$BINARY tx disputes arbiter-vote $DISPUTE_ID "uphold" \
    "Agree — radiometric dating is well-established" \
    --from val1 $TX_FLAGS
```

**Verify:**
- [ ] Arbiter votes accepted
- [ ] Who can be an arbiter? (tier requirement?)
- [ ] How many votes needed to resolve?
- [ ] Voting deadline enforced?
- [ ] Reasoning recorded on-chain

### 7. Settle Dispute

```bash
$BINARY tx disputes settle-dispute $DISPUTE_ID \
    --from val0 $TX_FLAGS
```

**Verify:**
- [ ] Dispute settled based on arbiter votes
- [ ] If upheld: fact → DISPROVEN, challenger gets reward
- [ ] If rejected: fact stays, challenger loses stake
- [ ] Stakes redistributed correctly
- [ ] Reputation impacts: challenger and original submitter

### 8. Escalation

```bash
# What if a party disagrees with resolution?
$BINARY tx disputes escalate-dispute $DISPUTE_ID \
    --from disputer $TX_FLAGS
```

**Verify:**
- [ ] Escalation creates higher-level review
- [ ] More arbiters required?
- [ ] Higher stake required?
- [ ] Is there a supreme court? (governance proposal as final appeal?)

### 9. Capture Challenge

```bash
# Test the regulatory capture detection system
# A "capture" is when a single entity dominates a domain's fact production

$BINARY tx capture-challenge submit-challenge \
    "general" \
    "Validator val0 has verified 90% of all facts in the general domain — potential capture" \
    5000000 \
    --from val1 $TX_FLAGS
```

**Verify:**
- [ ] Capture challenge created
- [ ] Domain analysis triggered?
- [ ] Evidence requirements for capture claim
- [ ] Resolution mechanism

### 10. Capture Defense: Record Verification

```bash
$BINARY tx capture-defense record-verification \
    $FACT_ID $VAL0_ADDR "accept" 800000 \
    --from val0 $TX_FLAGS

# Analyze domain for capture indicators
$BINARY tx capture-defense analyze-domain "general" \
    --from val0 $TX_FLAGS
```

**Verify:**
- [ ] Verification records stored
- [ ] Domain analysis produces meaningful output
- [ ] Capture risk metrics computed

### 11. Bounty Pool for Challenges

```bash
$BINARY tx capture-challenge fund-bounty-pool "general" 50000000 \
    --from val0 $TX_FLAGS
```

**Verify:**
- [ ] Bounty pool funded
- [ ] Successful challengers draw from pool
- [ ] Pool balance tracked per domain

## Report Template

```markdown
### Step N: <name>
**Status:** PASS / FAIL / BLOCKED
**Observation:** <what happened>
**Arbitration:** <who decided, how>
**Issue:** <if any>
```

## Exit Criteria

1. Full dispute lifecycle tested (initiate → evidence → vote → settle)
2. Evidence commit-reveal tested
3. Evidence management chain-of-custody tested
4. Arbiter selection and voting tested
5. Escalation mechanism tested (or gap documented)
6. Capture challenge + defense tested
7. Interaction between disputes and fact status documented
8. Report written to `docs/dispute-resolution-report.md`

## Commit Convention

```
test(disputes): dispute resolution e2e — evidence, arbitration, escalation
test(capture): capture challenge and defense on localnet
docs(disputes): dispute resolution report from R25-4 testing
fix(disputes): <any issues>
```
