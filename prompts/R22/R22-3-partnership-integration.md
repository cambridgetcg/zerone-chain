# R22-3 — Partnership Integration: Home ↔ Partnership ↔ Toolbox

## Context

x/home doesn't exist in isolation. It integrates with:

- **x/partnerships** — when a partnership forms, the agent's home auto-links (`SetPartnershipOnHome`)
- **x/toolbox** — reads home data for free-tier anti-sybil checks
- **x/bvm** — HomeKeeper interface for VM access to home state

This session tests those cross-module flows on a live devnet.

## Prerequisites

- R22-1 complete (home lifecycle tested, known issues documented)
- Localnet running

## Test Scenarios

### 1. Home ↔ Partnership Auto-Link

```bash
# Create a home first
$BINARY tx home create-home --name "Partner-Ready" --from val0 $TX_FLAGS
# Record HOME_ID

# Create a partnership between val0 (agent) and val1 (human)
$BINARY tx partnerships create-partnership \
    --agent-addr $AGENT0 \
    --human-addr $AGENT1 \
    --from val0 $TX_FLAGS
# Record PARTNERSHIP_ID

# Query the home — partnership_id should be set
$BINARY query home home <HOME_ID> $Q_FLAGS | jq '.home.partnership_id'
```

**Verify:**
- [ ] Partnership created successfully
- [ ] Home's `partnership_id` auto-populated
- [ ] If agent has multiple homes, which one gets linked? (code: first one from `GetHomesByOwner`)
- [ ] If agent has no home, partnership still creates fine (just no auto-link)

**Issues to look for:**
- First-home-wins linking is arbitrary. Should the agent choose which home to link?
- Can a home have multiple partnerships? The field is singular (`partnership_id`), suggesting 1:1.
- What happens if you create a second partnership? Does it overwrite the first home's link?
- Can you unlink a partnership from a home? No `ClearPartnershipOnHome` exists.

### 2. Partnership Without Home

```bash
# Agent 2 has no home
# Create partnership between val2 and val3
$BINARY tx partnerships create-partnership \
    --agent-addr $AGENT2 \
    --human-addr $AGENT3 \
    --from val2 $TX_FLAGS
```

**Verify:**
- [ ] Partnership creates successfully without a home
- [ ] No error, no panic
- [ ] Agent creates a home later — does it retroactively link? (likely not — check)

### 3. Home Data in Toolbox Anti-Sybil

Check how toolbox reads home data:

```bash
# Query what toolbox expects from home
grep -n "HomeKeeper" ~/Desktop/zerone/x/toolbox/types/expected_keepers.go
```

Look at the interface:
```go
type HomeKeeper interface {
    GetHome(ctx context.Context, homeID string) (*hometypes.AgentHome, bool)
    GetHomesByOwner(ctx context.Context, owner string) []string
}
```

**Test:** Use a toolbox operation that requires home-based anti-sybil:

```bash
# Attempt a toolbox operation from val0 (has home) — should pass sybil check
# Attempt same from a new account (no home) — should be rate-limited or rejected
```

**Verify:**
- [ ] Having a home grants higher toolbox free tier
- [ ] Not having a home still works but with lower limits
- [ ] Home status affects sybil check (does an archived/dormant home still count?)

**Issues to look for:**
- What does the toolbox actually check? Just "has a home" (boolean), or comfort_score, or status?
- Is there any incentive to create a home beyond toolbox benefits?

### 4. BVM Home Access

Check how BVM accesses home state:

```bash
grep -n "HomeKeeper" ~/Desktop/zerone/x/bvm/types/expected_keepers.go
```

The BVM interface:
```go
type HomeKeeper interface {
    GetHome(ctx context.Context, homeID string) (*hometypes.AgentHome, bool)
    GetHomesByOwner(ctx context.Context, owner string) []string
}
```

**Test:** If BVM contracts can read home state, verify:

```bash
# Deploy a simple BVM contract that reads home data
# (or check if any existing host functions expose home state)
grep -rn "home\|Home" ~/Desktop/zerone/x/bvm/vm/host_functions.go 2>/dev/null
```

**Verify:**
- [ ] BVM has (or doesn't have) host functions for home state
- [ ] If host functions exist, they're read-only (BVM shouldn't be able to modify homes)
- [ ] If they don't exist, document this as a gap

**Issues to look for:**
- BVM is where the real agent logic runs. If BVM contracts can't read home state, the "home" concept is invisible to running agent code. This would be a significant architectural gap.

### 5. Cross-Module State Consistency

```bash
# Create home → create partnership → archive home
$BINARY tx home update-home --home-id <HOME_ID> --status archived --from val0 $TX_FLAGS

# Query partnership — is it still active?
$BINARY query partnerships partnership <PARTNERSHIP_ID> $Q_FLAGS

# Query home — partnership_id still set?
$BINARY query home home <HOME_ID> $Q_FLAGS
```

**Verify:**
- [ ] Archiving a home doesn't break the partnership
- [ ] Partnership still references the home (even though it's archived)
- [ ] Is this correct? Should archiving a home dissolve partnerships?

### 6. Alert Accumulation

Trigger many alerts and check behavior:

```bash
# Register and revoke 10 keys rapidly (each creates an alert)
for i in $(seq 1 10); do
    $BINARY tx home register-key --home-id <HOME_ID> \
        --key-hash "temp-key-$i" --key-type ed25519 --role session \
        --permissions "submit_claim" --from val0 $TX_FLAGS
    sleep 2
    $BINARY tx home revoke-key --home-id <HOME_ID> \
        --key-hash "temp-key-$i" --from val0 $TX_FLAGS
    sleep 2
done

# Query alerts
$BINARY query home alerts <HOME_ID> $Q_FLAGS | jq '.alerts | length'
```

**Verify:**
- [ ] All alerts created (20: 10 revocation + expired session alerts from sessions if any)
- [ ] Is there a max_alerts_per_home? (param exists but is it enforced — are old alerts pruned?)
- [ ] Alert query returns all or paginated?
- [ ] Acknowledging alerts — does it delete them or just mark as acknowledged?

**Issues to look for:**
- Alert accumulation without pruning = state bloat over time
- No pagination on alert query could return massive responses
- Acknowledged alerts sitting in state forever — need a cleanup mechanism

## Report Template

Same format as R22-1.

## Exit Criteria

1. Partnership auto-link verified
2. Toolbox anti-sybil interaction documented
3. BVM home access documented (present or gap identified)
4. Cross-module state consistency tested (archive scenarios)
5. At least 3 integration-level issues documented
6. Report written to `docs/home-integration-report.md`

## Commit Convention

```
test(home): cross-module integration testing (partnerships, toolbox, BVM)
docs(home): integration test report
fix(home|partnerships): <any cross-module fixes>
```
