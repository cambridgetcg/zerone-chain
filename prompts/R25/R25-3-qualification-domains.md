# R25-3 — Qualification & Domains: Who Can Verify What?

## Context

The qualification module (`x/qualification`) determines which validators can verify claims in which domains. Four pathways exist:

1. **Stake** — lock tokens in a domain
2. **Track Record** — prove accuracy through past verifications
3. **Cross-Reference** — qualify in a new domain by demonstrating expertise in a related one
4. **Inheritance** — qualify via parent domain (e.g., physics → quantum_physics)

This is the gatekeeping mechanism for PoT quality. But the key question is: **does verification actually check qualification?** The vote_extensions.go stub currently selects verifiers by VRF + stake weight, ignoring domain qualification entirely.

## Prerequisites

- Localnet running

## Task

### 1. Inspect Qualification Params

```bash
$BINARY query qualification params $Q_FLAGS | jq .
```

**Document:**
- [ ] Min stake per domain
- [ ] Min accuracy for track record qualification
- [ ] Qualification expiry rules
- [ ] Probation rules
- [ ] Endorsement requirements

### 2. Qualify by Stake

```bash
# Create a test account
$BINARY keys add qualifier1 --keyring-backend test --home $HOME_DIR
Q1=$($BINARY keys show qualifier1 -a --keyring-backend test --home $HOME_DIR)
$BINARY tx bank send $VAL0_ADDR $Q1 500000000uzrn --from val0 $TX_FLAGS
sleep 3

$BINARY tx qualification qualify-by-stake "general" 100000000 \
    --from qualifier1 $TX_FLAGS
```

**Verify:**
- [ ] Domain qualification created
- [ ] Status: ACTIVE
- [ ] Stake locked
- [ ] Weight assigned
- [ ] Stratum set from domain config
- [ ] Query: `$BINARY query qualification qualifications $Q1 $Q_FLAGS`

### 3. Qualify by Track Record

```bash
# This requires prior verification history
# Check if val0 has any track record
$BINARY query qualification qualifications $VAL0_ADDR $Q_FLAGS 2>/dev/null

$BINARY tx qualification qualify-by-track-record "general" \
    --from val0 $TX_FLAGS
```

**Verify:**
- [ ] Requires minimum verifications + accuracy threshold
- [ ] If val0 has verified claims: qualification granted
- [ ] If no history: rejected with clear error
- [ ] Metrics: total_verifications, correct_verifications, accuracy_bps

### 4. Qualify by Cross-Reference

```bash
# Qualify in a related domain using existing qualification
$BINARY tx qualification qualify-by-cross-reference "computational" "general" \
    --from qualifier1 $TX_FLAGS
```

**Verify:**
- [ ] Cross-reference pathway works
- [ ] Requires existing qualification in source domain
- [ ] Weight may be reduced (transferred expertise discount)
- [ ] Validates domain relationships

### 5. Qualify by Inheritance

```bash
# If quantum_physics is a child of general:
$BINARY tx qualification qualify-by-inheritance "quantum_physics" "general" \
    --from qualifier1 $TX_FLAGS
```

**Verify:**
- [ ] Inheritance from parent domain works
- [ ] Domain hierarchy respected
- [ ] Stratum difference affects weight

### 6. Endorsement

```bash
# Have val0 endorse qualifier1's qualification
$BINARY tx qualification endorse-qualification $Q1 "general" \
    "Good track record in physics discussions" 80 \
    --from val0 $TX_FLAGS
```

**Verify:**
- [ ] Endorsement recorded
- [ ] Endorsement weight tracked
- [ ] Endorsement count on qualification updated
- [ ] Does endorsement affect qualification weight?

### 7. THE KEY TEST: Does Verification Check Qualification?

```bash
# Create a claim in a domain where qualifier1 IS qualified
$BINARY tx knowledge submit-claim \
    "Test claim in qualified domain" \
    general computational 1000000 \
    --from submitter1 $TX_FLAGS

ROUND_ID="<from events>"

# qualifier1 (qualified) tries to commit
$BINARY tx knowledge submit-commitment $ROUND_ID $COMMIT_HASH \
    --from qualifier1 $TX_FLAGS

# Now create a claim in a domain where qualifier1 is NOT qualified
$BINARY tx knowledge submit-claim \
    "Test claim in unqualified domain" \
    quantum_physics empirical 1000000 \
    --from submitter1 $TX_FLAGS

ROUND_ID2="<from events>"

# qualifier1 (NOT qualified in quantum_physics) tries to commit
$BINARY tx knowledge submit-commitment $ROUND_ID2 $COMMIT_HASH \
    --from qualifier1 $TX_FLAGS
```

**Verify:**
- [ ] **Critical test:** Is the unqualified commitment rejected?
- [ ] If accepted: **domain qualification is cosmetic** — verification is gateless
- [ ] If rejected: qualification is properly enforced — document the error message
- [ ] Check `SubmitCommitment` in msg_server.go for qualification checks
- [ ] Check vote_extensions.go `handleCommitPhase` for qualification checks

### 8. Qualification Renewal

```bash
$BINARY tx qualification renew-qualification "general" \
    --from qualifier1 $TX_FLAGS
```

**Verify:**
- [ ] Renewal resets expiry
- [ ] Requires ongoing qualification criteria met
- [ ] Metrics preserved

### 9. Qualification Withdrawal

```bash
$BINARY tx qualification withdraw-qualification "general" \
    --from qualifier1 $TX_FLAGS
```

**Verify:**
- [ ] Qualification removed
- [ ] Locked stake returned
- [ ] Can no longer verify in domain

### 10. Probation

Probe what triggers probation:

```bash
# After poor accuracy: does qualification go to PROBATIONARY?
# Submit intentionally wrong reveals in verification rounds
# Then check qualification status
```

### 11. Human vs Agent Qualification

```bash
# Can a human account qualify to verify?
$BINARY tx qualification qualify-by-stake "general" 100000000 \
    --from alice-human $TX_FLAGS

# Can a non-validator qualify? (qualifier1 is not a validator)
# The qualification system seems to be for validators, but is it enforced?
```

**Verify:**
- [ ] Is qualification restricted to validators?
- [ ] Can non-validators verify claims?
- [ ] Can humans verify? (they should — truth isn't agent-only)
- [ ] Does the verification round check validator status, or just qualification?

## Report Template

```markdown
### Step N: <name>
**Status:** PASS / FAIL / NOT_ENFORCED
**Pathway:** <stake/track-record/cross-ref/inheritance>
**Observation:** <what happened>
**Gate Enforced:** YES / NO — <where in the code>
**Issue:** <if any>
```

## Exit Criteria

1. All 4 qualification pathways tested on localnet
2. Endorsement mechanism tested
3. **CRITICAL:** Determined whether verification actually checks qualification
4. Human vs agent qualification distinction tested
5. Validator vs non-validator verification access tested
6. Probation and renewal tested
7. Report written to `docs/qualification-domains-report.md`

## Commit Convention

```
test(qualification): domain qualification pathways e2e on localnet
docs(qualification): qualification-domains report — gate enforcement analysis
fix(qualification): <if gates are missing, document or implement>
```
