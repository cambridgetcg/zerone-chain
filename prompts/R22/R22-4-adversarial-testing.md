# R22-4 — Adversarial Testing: Edge Cases, Abuse, and DoS Vectors

## Context

The happy path works. Now break it. This session attempts everything an adversary, a confused agent, or a malicious validator might try.

## Prerequisites

- R22-1 complete (know what "normal" looks like)
- Localnet running

## Test Categories

### A. Input Validation Attacks

#### A1. Name Injection

```bash
# Extremely long name
$BINARY tx home create-home --name "$(python3 -c 'print("A"*10000)')" --from val0 $TX_FLAGS

# Empty name
$BINARY tx home create-home --name "" --from val0 $TX_FLAGS

# Name with special characters / control chars / unicode
$BINARY tx home create-home --name "🏠💀<script>alert(1)</script>" --from val0 $TX_FLAGS
$BINARY tx home create-home --name $'\x00\x01\x02' --from val0 $TX_FLAGS
```

**Verify for each:**
- [ ] Long name: rejected or truncated? If accepted, how does it affect state size and queries?
- [ ] Empty name: accepted or rejected?
- [ ] Special chars: accepted or sanitised?
- [ ] Null bytes: definitely should be rejected — proto marshaling may panic

#### A2. Key Hash Abuse

```bash
# Very long key hash
$BINARY tx home register-key --home-id <HOME_ID> \
    --key-hash "$(python3 -c 'print("x"*10000)')" \
    --key-type ed25519 --role agent --permissions "transfer" --from val0 $TX_FLAGS

# Empty key hash
$BINARY tx home register-key --home-id <HOME_ID> \
    --key-hash "" --key-type ed25519 --role agent --permissions "transfer" --from val0 $TX_FLAGS
```

**Verify:**
- [ ] Long key hash rejected or creates store bloat?
- [ ] Empty key hash rejected?

#### A3. Memo/CID Abuse

```bash
# Very long CID
$BINARY tx home update-memory-cid --home-id <HOME_ID> \
    --cid "$(python3 -c 'print("Q"*100000)')" --from val0 $TX_FLAGS
```

**Verify:**
- [ ] Large CID: accepted? Gas cost proportional to size?
- [ ] Proto field is `string` — no length limit in proto, so the message handler or ante handler must enforce it

### B. Permission Escalation

#### B1. Session Requests Unpermitted Actions

```bash
# Register a key with limited permissions
$BINARY tx home register-key --home-id <HOME_ID> \
    --key-hash "limited-key" --key-type ed25519 --role session \
    --permissions "submit_claim" --from val0 $TX_FLAGS

# Start session requesting permissions the key doesn't have
$BINARY tx home start-session --home-id <HOME_ID> \
    --key-hash "limited-key" \
    --requested-permissions "transfer,stake,vote" --from val0 $TX_FLAGS
```

**Verify:**
- [ ] Session's granted permissions = intersection (only "submit_claim" if it's not in the requested set, or empty if no overlap)
- [ ] Does an empty intersection create a session with zero permissions? Is that useful?

#### B2. Expired Key Session Start

```bash
# Register a key that expires at block N
# Wait for block N+1
# Try to start session with expired key
```

**Verify:**
- [ ] Rejected with "key expired" error
- [ ] No partial state changes

#### B3. Non-Owner as Guardian

```bash
# Set Agent1 as guardian for Agent0's home
# Agent1 tries operations beyond alert acknowledgment:
$BINARY tx home update-home --home-id <AGENT0_HOME_ID> --status dormant --from val1 $TX_FLAGS
$BINARY tx home register-key --home-id <AGENT0_HOME_ID> ... --from val1 $TX_FLAGS
$BINARY tx home revoke-key --home-id <AGENT0_HOME_ID> ... --from val1 $TX_FLAGS
```

**Verify:**
- [ ] All should fail (guardian can only acknowledge alerts)
- [ ] Error messages don't reveal whether the guardian address is correct

### C. State Exhaustion / DoS

#### C1. Home Creation Spam

```bash
# Create homes in a loop until gas/balance runs out
for i in $(seq 1 50); do
    $BINARY tx home create-home --name "spam-$i" --from val0 $TX_FLAGS
    sleep 1
done
```

**Verify:**
- [ ] Creation fee acts as spam deterrent
- [ ] No performance degradation in queries/BeginBlocker after many homes
- [ ] Is there a max homes per account? Should there be?

#### C2. Alert Flooding

```bash
# Create/revoke keys rapidly to flood alerts
for i in $(seq 1 100); do
    $BINARY tx home register-key --home-id <HOME_ID> \
        --key-hash "flood-$i" --key-type ed25519 --role session \
        --permissions "submit_claim" --from val0 $TX_FLAGS
    $BINARY tx home revoke-key --home-id <HOME_ID> \
        --key-hash "flood-$i" --from val0 $TX_FLAGS
done
```

**Verify:**
- [ ] max_alerts_per_home enforced? If not, state grows unboundedly
- [ ] Alert query with 100+ alerts — response time?
- [ ] BeginBlocker iterates all homes every block — with 50 homes and 100 alerts each, is this slow?

#### C3. BeginBlocker Scalability

```bash
# Estimate: BeginBlocker does two passes:
# 1. CheckDeadmanSwitches: iterates ALL homes, checks guardian.deadman
# 2. CleanupExpiredSessions: iterates ALL homes, then iterates ALL sessions per home

# With 50 homes, each having 10 sessions:
# = 50 home iterations + 50*10 session iterations = 550 iterations per block
# Is this acceptable? Time it.
```

**Verify:**
- [ ] Measure block time with 5 homes vs 50 homes
- [ ] Any noticeable slowdown?
- [ ] At what scale does BeginBlocker become a bottleneck?

### D. Recovery Mechanism Testing

#### D1. Recovery Address Validation

```bash
# Set recovery with invalid addresses
$BINARY tx home configure-guardian --home-id <HOME_ID> \
    --recovery-addresses "not-an-address,also-not-valid" \
    --recovery-threshold 1 --from val0 $TX_FLAGS
```

**Verify:**
- [ ] Invalid bech32 addresses rejected?
- [ ] Or silently accepted? (check the code — `ConfigureGuardian` doesn't validate address format)

#### D2. Recovery Threshold Edge Cases

```bash
# threshold > number of addresses
$BINARY tx home configure-guardian --home-id <HOME_ID> \
    --recovery-addresses "$AGENT1" \
    --recovery-threshold 5 --from val0 $TX_FLAGS

# threshold = 0
$BINARY tx home configure-guardian --home-id <HOME_ID> \
    --recovery-addresses "$AGENT1" \
    --recovery-threshold 0 --from val0 $TX_FLAGS
```

**Verify:**
- [ ] threshold > addresses count: rejected (code has this check)
- [ ] threshold = 0: accepted? What does zero-threshold recovery mean?

#### D3. Recovery Execution

```bash
# The big question: is there an actual recovery mechanism?
# Recovery addresses are stored, but is there a MsgRecoverHome or similar?
grep -rn "Recover\|recover" ~/Desktop/zerone/x/home/ --include="*.go" | grep -v test
```

**Verify:**
- [ ] Does a recovery transaction exist? If not, recovery_addresses are decorative.
- [ ] Document the gap if missing.

### E. Concurrent/Race Conditions

#### E1. Simultaneous Key Revoke + Session Start

```bash
# In rapid succession:
$BINARY tx home revoke-key --home-id <HOME_ID> --key-hash "key1" --from val0 $TX_FLAGS &
$BINARY tx home start-session --home-id <HOME_ID> --key-hash "key1" --from val0 $TX_FLAGS &
wait
```

**Verify:**
- [ ] Both can't be in the same block (sequence number conflict), but if they're in consecutive blocks — does the session start get the revoked key?
- [ ] Cosmos SDK's sequential tx execution should prevent this, but verify the ordering

## Report Template

For each attack vector:

```markdown
### Vector <ID>: <name>
**Category:** Input / Permission / DoS / Recovery / Race
**Status:** BLOCKED / VULNERABLE / ACCEPTED_RISK
**Severity:** Critical / High / Medium / Low / Informational
**Observation:** <what happened>
**Recommendation:** <fix or acceptance rationale>
```

## Exit Criteria

1. All 5 categories tested (Input, Permission, DoS, Recovery, Race)
2. Each vector has BLOCKED/VULNERABLE/ACCEPTED_RISK status
3. At least 2 critical or high-severity findings documented
4. BeginBlocker scalability measured
5. Recovery mechanism gap documented (if exists)
6. Report written to `docs/home-adversarial-report.md`

## Commit Convention

```
test(home): adversarial testing — input validation, DoS, permission escalation
docs(home): adversarial test report with severity ratings
fix(home): <any security fixes — these should be separate, focused commits>
```
