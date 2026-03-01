# R22-2 — Multi-Agent Scenarios: Isolation, Sharing, and Boundaries

## Context

A single agent creating a single home is the happy path. Real usage means multiple agents, multiple homes, shared keys, permission conflicts, and cross-home isolation. This session stress-tests those boundaries.

## Prerequisites

- Localnet running (`scripts/localnet.sh start`)
- Use val0-val3 as four different "agents"

## Setup

```bash
# Same TX_FLAGS/Q_FLAGS as R22-1
AGENT0=$($BINARY keys show val0 -a $KEYRING $HOME_DIR)
AGENT1=$($BINARY keys show val1 -a $KEYRING $HOME_DIR)
AGENT2=$($BINARY keys show val2 -a $KEYRING $HOME_DIR)
AGENT3=$($BINARY keys show val3 -a $KEYRING $HOME_DIR)
```

## Test Scenarios

### 1. Multiple Homes Per Agent

```bash
# Agent 0 creates 3 homes
$BINARY tx home create-home --name "Workshop" --from val0 $TX_FLAGS
$BINARY tx home create-home --name "Archive" --from val0 $TX_FLAGS
$BINARY tx home create-home --name "Lab" --from val0 $TX_FLAGS
```

**Verify:**
- [ ] Three homes created with sequential IDs
- [ ] `homes-by-owner` returns all three
- [ ] Each home has independent state (status, guardian, keys)
- [ ] Is there a limit on homes per agent? Should there be?

### 2. Cross-Agent Isolation

```bash
# Agent 1 creates a home
$BINARY tx home create-home --name "Agent1-Home" --from val1 $TX_FLAGS

# Agent 0 tries to modify Agent 1's home
$BINARY tx home update-home --home-id <AGENT1_HOME_ID> --name "Hijacked" --from val0 $TX_FLAGS
# Should fail: "not the home owner"

# Agent 0 tries to register a key on Agent 1's home
$BINARY tx home register-key --home-id <AGENT1_HOME_ID> \
    --key-hash "attacker-key" --key-type ed25519 --role agent \
    --permissions "transfer" --from val0 $TX_FLAGS
# Should fail

# Agent 0 tries to revoke Agent 1's key
$BINARY tx home revoke-key --home-id <AGENT1_HOME_ID> \
    --key-hash <AGENT1_KEY> --from val0 $TX_FLAGS
# Should fail
```

**Verify:**
- [ ] All cross-agent modifications fail with "unauthorized" or "not the home owner"
- [ ] Error messages are clear and don't leak internal state
- [ ] Agent 0 can still read Agent 1's home (queries are public) — is this desired?

### 3. Shared Guardian

Agent 0 sets Agent 1 as their guardian:

```bash
$BINARY tx home configure-guardian --home-id <AGENT0_HOME_ID> \
    --guardian-address $AGENT1 \
    --defense-strategy moderate \
    --from val0 $TX_FLAGS
```

Then Agent 1 acknowledges alerts on Agent 0's home:

```bash
# Trigger an alert (e.g., revoke a key to generate one)
$BINARY tx home revoke-key --home-id <AGENT0_HOME_ID> \
    --key-hash "some-key" --from val0 $TX_FLAGS

# Agent 1 (guardian) acknowledges the alert
$BINARY tx home acknowledge-alert --home-id <AGENT0_HOME_ID> \
    --alert-id <ALERT_ID> --from val1 $TX_FLAGS
```

**Verify:**
- [ ] Guardian can acknowledge alerts
- [ ] Non-guardian, non-owner cannot acknowledge
- [ ] What else can a guardian do? Currently only alert acknowledgment — is that enough?

**Issues to look for:**
- Guardian role feels thin. In the current code, guardian_address only grants alert acknowledgment. An agent's guardian should arguably be able to:
  - Trigger emergency status transitions (active → guarded)
  - Revoke compromised keys
  - Execute recovery if the owner goes silent
- These are design questions, not bugs. Document them for R22-5.

### 4. Session Limit Exhaustion

```bash
# Find max_sessions_per_home from params
$BINARY query home params $Q_FLAGS | jq '.params.max_sessions_per_home'

# Register enough keys, then start sessions up to the limit
for i in $(seq 1 $MAX_SESSIONS); do
    $BINARY tx home register-key --home-id <HOME_ID> \
        --key-hash "key-$i" --key-type ed25519 --role session \
        --permissions "submit_claim" --from val0 $TX_FLAGS
    
    $BINARY tx home start-session --home-id <HOME_ID> \
        --key-hash "key-$i" --from val0 $TX_FLAGS
done

# One more should fail
$BINARY tx home register-key --home-id <HOME_ID> \
    --key-hash "key-overflow" --key-type ed25519 --role session \
    --permissions "submit_claim" --from val0 $TX_FLAGS
$BINARY tx home start-session --home-id <HOME_ID> \
    --key-hash "key-overflow" --from val0 $TX_FLAGS
```

**Verify:**
- [ ] Sessions up to limit succeed
- [ ] Session beyond limit fails with clear error
- [ ] After ending one session, a new one can be created (limit is current count, not lifetime)

### 5. Key Limit Exhaustion

Same pattern as sessions but for `max_keys_per_home`.

**Verify:**
- [ ] Keys up to limit succeed
- [ ] Key beyond limit fails
- [ ] Revoking a key frees the slot? Or do revoked keys count against the limit?

**Issues to look for:**
- If revoked keys count against the limit, an agent can eventually get locked out of registering new keys. This would be a significant UX problem.

### 6. Four Agents, Four Homes, Cross-Queries

All four agents create homes. Query everything from each agent's perspective:

```bash
for agent in val0 val1 val2 val3; do
    $BINARY tx home create-home --name "${agent}-home" --from $agent $TX_FLAGS
done

# Each agent queries all homes
for id in home-1 home-2 home-3 home-4; do
    $BINARY query home home $id $Q_FLAGS
done
```

**Verify:**
- [ ] All queries succeed (home state is public)
- [ ] Owner information is visible (is this a privacy concern?)
- [ ] Keys and sessions are visible (permissions, key hashes visible to anyone who queries)

**Issues to look for:**
- Privacy: should key_hash, permissions, and session details be queryable by anyone? In a public chain they're visible anyway, but the query API makes it easy. Document for R22-5.

### 7. Session Expiry Under Load

Start multiple sessions, advance blocks past expiry, verify cleanup:

```bash
# Start 3 sessions
# Note the expires_at block
# Advance past it (wait or use localnet block production)
# Query sessions — should be empty (BeginBlocker cleans up)
# Check for session_expired alerts
```

**Verify:**
- [ ] Expired sessions are cleaned up automatically
- [ ] Alerts generated for each expired session
- [ ] Home with many expired sessions doesn't cause BeginBlocker slowdown (note timing)

### 8. Concurrent Home Operations

Submit multiple transactions in rapid succession from the same agent:

```bash
# Rapid-fire: create home, register key, start session, update memory — all in ~2 blocks
$BINARY tx home create-home --name "Speed-Test" --from val0 $TX_FLAGS &
sleep 1
# Get home ID from events, then immediately:
$BINARY tx home register-key --home-id <HOME_ID> ... --from val0 $TX_FLAGS &
$BINARY tx home update-home --home-id <HOME_ID> --name "Renamed" --from val0 $TX_FLAGS &
```

**Verify:**
- [ ] No sequence number collisions (Cosmos SDK handles this, but verify)
- [ ] State is consistent after all txs settle
- [ ] No partial states visible between blocks

## Report Template

Same as R22-1 — for each scenario: Status, Tx Hash, Observation, Issue, Suggestion.

## Exit Criteria

1. All 8 scenarios attempted
2. Cross-agent isolation verified (no unauthorised modifications)
3. Limit enforcement tested (sessions + keys)
4. At least 3 boundary/design issues documented
5. Report written to `docs/home-multiagent-report.md`

## Commit Convention

```
test(home): multi-agent isolation and boundary testing
docs(home): multi-agent test report
fix(home): <any isolation/boundary fixes>
```
