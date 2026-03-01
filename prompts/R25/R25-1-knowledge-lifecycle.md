# R25-1 — Knowledge Lifecycle: Claim → Verify → Challenge → Patronise → Expire

## Context

The knowledge module has 21 tx RPCs and 28 queries — it's the largest module in ZERONE. The localnet test (test_pot_round) covers basic claim submission + commit-reveal but doesn't test the full lifecycle: challenges, patronage, contradictions, structured claims, domain proposals, demand signals, satisfaction feedback, or fact metabolism.

This session walks through every stage of a fact's life on a live chain.

## Prerequisites

- Localnet running (`scripts/localnet.sh start`)

## Task

### 1. Inspect Current Knowledge State

```bash
# Check genesis axioms
$BINARY query knowledge facts --limit 10 $Q_FLAGS
$BINARY query knowledge facts --limit 1 --offset 770 $Q_FLAGS

# How many domains exist?
$BINARY query knowledge domains $Q_FLAGS

# Params: verification windows, fees, metabolism
$BINARY query knowledge params $Q_FLAGS | jq .
```

**Document:**
- [ ] Number of genesis facts (expect 777 axioms)
- [ ] Active domains and their strata
- [ ] Key params: commit_phase_blocks, reveal_phase_blocks, min_review_fee, min_claim_stake
- [ ] Metabolism params: base_energy, decay_rate, energy_cap

### 2. Submit a Simple Claim

```bash
# Create a funded test account
$BINARY keys add submitter1 --keyring-backend test --home $HOME_DIR
SUBMITTER=$($BINARY keys show submitter1 -a --keyring-backend test --home $HOME_DIR)
$BINARY tx bank send $VAL0_ADDR $SUBMITTER 500000000uzrn --from val0 $TX_FLAGS
sleep 3

# Submit a basic assertion
$BINARY tx knowledge submit-claim \
    "Water boils at 100 degrees Celsius at standard atmospheric pressure" \
    general computational 1000000 \
    --from submitter1 $TX_FLAGS
```

**Verify:**
- [ ] Claim created with status PENDING
- [ ] Verification round auto-created
- [ ] Review fee deducted
- [ ] Claim ID in tx events

### 3. Submit a Structured Claim

```bash
$BINARY tx knowledge submit-claim \
    "The speed of light in vacuum is approximately 299,792,458 metres per second" \
    general formal_proof 2000000 \
    --claim-type CLAIM_TYPE_ASSERTION \
    --structure '{"subject":"speed of light in vacuum","predicate":"equals approximately","object":"299792458 m/s","scope":"special relativity","tags":["physics","constants","speed-of-light"]}' \
    --from submitter1 $TX_FLAGS
```

**Verify:**
- [ ] Structured claim accepted
- [ ] Structure fields stored (subject, predicate, object, scope, tags)
- [ ] canonical_form auto-derived
- [ ] canonical_hash computed
- [ ] Can query by subject: `$BINARY query knowledge facts-by-subject "speed of light" $Q_FLAGS`
- [ ] Can query by tag: `$BINARY query knowledge facts-by-tag "physics" $Q_FLAGS`

### 4. Commit-Reveal Verification

```bash
# Get the round ID from step 2
ROUND_ID="<from tx events>"

# Check round phase
$BINARY query knowledge verification-round $ROUND_ID $Q_FLAGS

# Phase should be COMMIT
# Generate commitment: SHA256(vote_bytes || salt_bytes)
SALT=$(openssl rand -hex 16)
COMMIT_HASH=$( (printf "accept"; printf '%s' "$SALT" | xxd -r -p) | shasum -a 256 | awk '{print $1}')

# Submit commitments from validators
$BINARY tx knowledge submit-commitment $ROUND_ID $COMMIT_HASH --from val0 $TX_FLAGS
$BINARY tx knowledge submit-commitment $ROUND_ID $COMMIT_HASH --from val1 $TX_FLAGS
sleep 5

# Wait for reveal phase
$BINARY query knowledge verification-round $ROUND_ID $Q_FLAGS | jq '.round.phase'

# Submit reveals
$BINARY tx knowledge submit-reveal $ROUND_ID accept $SALT --from val0 $TX_FLAGS
$BINARY tx knowledge submit-reveal $ROUND_ID accept $SALT --from val1 $TX_FLAGS
sleep 5

# Check round completion
$BINARY query knowledge verification-round $ROUND_ID $Q_FLAGS | jq '{phase: .round.phase, verdict: .round.verdict}'
```

**Verify:**
- [ ] Commit phase: commitments accepted only from qualified validators
- [ ] Reveal phase: reveals match commitments
- [ ] Aggregation: verdict computed (accept/reject/inconclusive)
- [ ] Fact created with status VERIFIED (if accepted)
- [ ] Submitter reputation updated
- [ ] Verifier reputation updated

### 5. Challenge a Fact

```bash
FACT_ID="<from verification>"

# Challenge with a reason and stake
$BINARY tx knowledge challenge-fact $FACT_ID 5000000 \
    "The boiling point varies with altitude — claim is incomplete without specifying pressure explicitly" \
    --from submitter1 $TX_FLAGS
```

**Verify:**
- [ ] Challenge created
- [ ] Fact status → CHALLENGED
- [ ] Challenge stake escrowed
- [ ] New verification round created for challenge?
- [ ] If challenge upheld: fact → DISPROVEN, challenger gets reward
- [ ] If challenge rejected: fact stays VERIFIED, challenger loses stake

### 6. Submit a Contradiction

```bash
# Submit a fact that explicitly contradicts another
$BINARY tx knowledge submit-contradiction \
    $FACT_ID \
    "Water boils at different temperatures depending on atmospheric pressure, not fixed at 100C" \
    general computational 3000000 \
    --from submitter1 $TX_FLAGS
```

**Verify:**
- [ ] Contradiction submitted
- [ ] Target fact status → CONTESTED
- [ ] Both facts enter verification
- [ ] Only one can survive (or both if they're compatible)

### 7. Patronise a Fact

```bash
# Fund a fact to keep it alive (metabolism)
$BINARY tx knowledge patronize-fact $FACT_ID 10000000 100 \
    --from submitter1 $TX_FLAGS
```

**Verify:**
- [ ] Patronage recorded
- [ ] Fact energy increased
- [ ] Patronage expiry set (blocks)
- [ ] Patronage amount stored on fact

### 8. Propose a New Domain

```bash
$BINARY tx knowledge propose-domain \
    "quantum_physics" \
    "Quantum mechanics and quantum field theory" \
    3 \
    --from submitter1 $TX_FLAGS
```

**Verify:**
- [ ] Domain proposal created
- [ ] Status: PROPOSED
- [ ] Requires endorsements to become ACTIVE
- [ ] Stratum level set (1 = most foundational, higher = more specialized)

### 9. Endorse and Activate Domain

```bash
$BINARY tx knowledge endorse-domain-proposal "quantum_physics" \
    --from val0 $TX_FLAGS
$BINARY tx knowledge endorse-domain-proposal "quantum_physics" \
    --from val1 $TX_FLAGS

# Check if activated
$BINARY query knowledge domain "quantum_physics" $Q_FLAGS
```

### 10. Report Demand

```bash
$BINARY tx knowledge report-demand "quantum_physics" 100 \
    --from submitter1 $TX_FLAGS
```

**Verify:**
- [ ] Demand signal recorded
- [ ] Affects fact fitness calculations for domain

### 11. Rate a Fact (Satisfaction Feedback)

```bash
$BINARY tx knowledge rate-fact $FACT_ID true --from submitter1 $TX_FLAGS
```

**Verify:**
- [ ] Satisfaction counts updated (up/down)
- [ ] Satisfaction affects fitness score
- [ ] Can only rate once per account per fact? Or unlimited?

### 12. Add Common Knowledge

```bash
$BINARY tx knowledge add-common-knowledge "water" \
    --from val0 $TX_FLAGS  # (requires authority?)
```

**Verify:**
- [ ] Common knowledge entry created
- [ ] Claims with matching subject get `common_knowledge_match = true`
- [ ] Novelty score reduced for common knowledge

### 13. Fact Metabolism (Energy Decay)

Wait N blocks and observe:

```bash
# Check energy before
$BINARY query knowledge fact $FACT_ID $Q_FLAGS | jq '{energy: .fact.energy, energy_cap: .fact.energy_cap}'

# Wait 50+ blocks
sleep 300

# Check energy after
$BINARY query knowledge fact $FACT_ID $Q_FLAGS | jq '{energy: .fact.energy, energy_cap: .fact.energy_cap}'
```

**Verify:**
- [ ] Energy decays over time (BeginBlocker)
- [ ] At energy = 0: fact → AT_RISK
- [ ] After AT_RISK grace period: fact → PRUNED
- [ ] Patronage slows decay

### 14. Query the Knowledge Graph

```bash
# Fact relations
$BINARY query knowledge fact-relations $FACT_ID $Q_FLAGS

# Facts by submitter
$BINARY query knowledge facts-by-submitter $SUBMITTER $Q_FLAGS

# Facts by domain
$BINARY query knowledge facts-by-domain "general" $Q_FLAGS

# Confidence
$BINARY query knowledge fact-confidence $FACT_ID $Q_FLAGS
```

## Report Template

```markdown
### Step N: <name>
**Status:** PASS / FAIL / STUB / BLOCKED
**Tx Hash:** <hash>
**Result:** <what happened>
**Issue:** <if any>
**Design Question:** <if the behaviour raises a design question>
```

## Exit Criteria

1. Full claim lifecycle tested: submit → verify → challenge → patronise
2. Structured claims tested with subject/predicate/object
3. Contradiction submission tested
4. Domain proposal + endorsement tested
5. Satisfaction feedback tested
6. Metabolism (energy decay) observed over time
7. Knowledge graph queries tested (relations, by-domain, by-submitter, by-subject, by-tag)
8. Every knowledge RPC attempted and documented
9. Report written to `docs/knowledge-lifecycle-report.md`

## Commit Convention

```
test(knowledge): full lifecycle e2e — claim, verify, challenge, patronise, metabolism
docs(knowledge): knowledge lifecycle report from R25-1 testing
fix(knowledge): <any issues>
```
