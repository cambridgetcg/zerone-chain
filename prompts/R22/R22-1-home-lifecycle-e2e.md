# R22-1 — Agent Home Lifecycle: End-to-End on Localnet

## Context

Exercise the full lifecycle of an Agent Home on a live devnet. Every tx, every query, every state transition — via the CLI, as an agent would.

## Prerequisites

- Localnet running (`scripts/localnet.sh start`)
- Binary built (`make build`)

## Setup

```bash
BINARY="./build/zeroned"
NODE="--node http://127.0.0.1:26601"
HOME_DIR="--home ${HOME}/.zeroned/localnet/coordinator"
KEYRING="--keyring-backend test"
CHAIN="--chain-id zerone-localnet"
GAS="--gas auto --gas-adjustment 1.5 --gas-prices 0.025uzrn"
TX_FLAGS="$NODE $HOME_DIR $KEYRING $CHAIN $GAS --yes --output json"
Q_FLAGS="$NODE $HOME_DIR --output json"

# Agent address (use val0 as the agent for testing)
AGENT=$($BINARY keys show val0 -a $KEYRING $HOME_DIR)
echo "Agent address: $AGENT"
```

## Test Scenarios

### 1. Create Home

```bash
$BINARY tx home create-home --name "AI-Alpha" --from val0 $TX_FLAGS
```

**Verify:**
- [ ] Tx succeeds, returns `home_id`
- [ ] Query: `$BINARY query home home <home_id> $Q_FLAGS`
  - Status = "active"
  - Owner = agent address
  - Name = "AI-Alpha"
  - Comfort score = 50 (default)
  - Treasury reserved_balance = "0"
  - Guardian defaults (defense_strategy = "moderate", auto_defend = false)
  - Created at block = current height
- [ ] Fee deducted from agent balance (check with `query bank balances`)
- [ ] Owner index: `$BINARY query home homes-by-owner $AGENT $Q_FLAGS` lists the home

**Issues to look for:**
- Is the home creation fee reasonable? What's the default? Can an agent with minimum balance create one?
- What happens if name is empty? Very long? Contains special characters?
- Is the home_id format human-readable? ("home-1" is fine, a 64-char hex would be hostile)

### 2. Register Keys

Register three keys with different roles and permissions:

```bash
# Primary agent key — full access
$BINARY tx home register-key \
    --home-id <HOME_ID> \
    --key-hash "sha256-primary-agent-key-001" \
    --key-type ed25519 \
    --role agent \
    --permissions "transfer,stake,submit_claim,vote,memory_write" \
    --from val0 $TX_FLAGS

# Session key — limited, time-bound
$BINARY tx home register-key \
    --home-id <HOME_ID> \
    --key-hash "sha256-session-key-001" \
    --key-type ed25519 \
    --role session \
    --permissions "submit_claim,memory_write" \
    --expires-at $(($(curl -s http://127.0.0.1:26601/status | jq -r '.result.sync_info.latest_block_height') + 100)) \
    --from val0 $TX_FLAGS

# Guardian key — defense only
$BINARY tx home register-key \
    --home-id <HOME_ID> \
    --key-hash "sha256-guardian-key-001" \
    --key-type ed25519 \
    --role guardian \
    --permissions "acknowledge_alert,defend" \
    --from val0 $TX_FLAGS
```

**Verify:**
- [ ] Three keys registered: `$BINARY query home keys <HOME_ID> $Q_FLAGS`
- [ ] Each has correct role, permissions, key_type
- [ ] Session key has expires_at set
- [ ] Guardian key has limited permissions
- [ ] Registering a 4th key (if max_keys_per_home allows) or hitting the limit

**Issues to look for:**
- Are permission names documented anywhere? What are the valid values?
- What happens with an unknown permission name? Silently accepted or rejected?
- Is key_hash just any string, or does it validate format?
- Can you register the same key_hash twice? (should fail)

### 3. Start and End Sessions

```bash
# Start session with primary key
$BINARY tx home start-session \
    --home-id <HOME_ID> \
    --key-hash "sha256-primary-agent-key-001" \
    --requested-permissions "transfer,submit_claim" \
    --from val0 $TX_FLAGS
```

**Verify:**
- [ ] Session created, returns session_id
- [ ] Query: `$BINARY query home sessions <HOME_ID> $Q_FLAGS`
  - Session has correct home_id, key_hash
  - Permissions = intersection of key perms and requested perms
  - Started_at = current block
  - Expires_at = started_at + session_timeout_blocks
- [ ] Key's last_used_at updated
- [ ] Home's last_active_block updated

```bash
# End session
$BINARY tx home end-session \
    --home-id <HOME_ID> \
    --session-id <SESSION_ID> \
    --from val0 $TX_FLAGS
```

**Verify:**
- [ ] Session removed from state
- [ ] Ending a non-existent session returns clear error

**Issues to look for:**
- Can anyone end anyone else's session? (should only owner/signer)
- What does "requested-permissions" look like in CLI? Comma-separated? JSON array?
- Session ID format — is it predictable? Does it matter?

### 4. Update Memory CID

```bash
$BINARY tx home update-memory-cid \
    --home-id <HOME_ID> \
    --cid "bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi" \
    --from val0 $TX_FLAGS
```

**Verify:**
- [ ] Home's memory_cid updated
- [ ] Last_active_block updated
- [ ] CID accepts any string (no IPFS validation on-chain — by design?)
- [ ] Overwriting with a new CID works

**Issues to look for:**
- No CID validation — is that intentional? An agent could store garbage here
- Should there be a CID history? Currently only latest is stored
- Is there any size limit on the CID string?

### 5. Configure Guardian

```bash
$BINARY tx home configure-guardian \
    --home-id <HOME_ID> \
    --defense-strategy aggressive \
    --auto-defend true \
    --deadman-enabled true \
    --deadman-inactivity-threshold 50 \
    --deadman-action "lock" \
    --deadman-beneficiary <SOME_ADDRESS> \
    --recovery-addresses "<ADDR1>,<ADDR2>" \
    --recovery-threshold 1 \
    --guardian-address <GUARDIAN_ADDR> \
    --from val0 $TX_FLAGS
```

**Verify:**
- [ ] Guardian config updated in home state
- [ ] Defense strategy = "aggressive"
- [ ] Auto defend = true
- [ ] Deadman switch enabled with correct threshold
- [ ] Recovery addresses stored
- [ ] Guardian address set

**Issues to look for:**
- What are the valid defense strategies? ("aggressive", "moderate", "conservative", "diplomatic" — are these documented?)
- Deadman action "lock" — is this enforced anywhere? What actions are valid?
- Is recovery_threshold validated against number of recovery_addresses?
- What does the guardian_address actually *do* beyond acknowledging alerts?

### 6. Trigger Deadman Switch

Wait for the inactivity threshold to pass without any home activity:

```bash
# Don't interact with the home for 50+ blocks
# Then check alerts
$BINARY query home alerts <HOME_ID> $Q_FLAGS
```

**Verify:**
- [ ] "deadman_triggered" alert created with priority "critical"
- [ ] Home status changed to "guarded"
- [ ] Alert message includes the inactivity threshold and action

**Issues to look for:**
- How long does this actually take on localnet? (depends on block time)
- The deadman trigger sets status to "guarded" — but does it actually execute the `action` ("lock")? Looking at the code, it only creates an alert. The action field is cosmetic.
- Can the agent reactivate after deadman triggers? (guarded → active is a valid transition)

### 7. Spending Limits

```bash
$BINARY tx home set-spending-limit \
    --home-id <HOME_ID> \
    --key-type ed25519 \
    --max-amount 1000000 \
    --period-blocks 100 \
    --from val0 $TX_FLAGS
```

**Verify:**
- [ ] Spending limit stored
- [ ] Query: `$BINARY query home spending-limits <HOME_ID> $Q_FLAGS`

**Issues to look for:**
- Spending limits are stored but are they *enforced* anywhere? Look for `CheckSpendingLimit` or similar in tx handlers. If not enforced, this is a UX promise without teeth.
- Period reset logic — does `spent_in_period` actually reset after `period_blocks`?

### 8. Key Revocation

```bash
$BINARY tx home revoke-key \
    --home-id <HOME_ID> \
    --key-hash "sha256-session-key-001" \
    --from val0 $TX_FLAGS
```

**Verify:**
- [ ] Key marked as revoked
- [ ] All sessions using this key are terminated
- [ ] Alert created for key revocation
- [ ] Starting a new session with the revoked key fails

### 9. Status Transitions

```bash
# Active → dormant
$BINARY tx home update-home --home-id <HOME_ID> --status dormant --from val0 $TX_FLAGS

# Dormant → active
$BINARY tx home update-home --home-id <HOME_ID> --status active --from val0 $TX_FLAGS

# Active → archived (terminal)
$BINARY tx home update-home --home-id <HOME_ID> --status archived --from val0 $TX_FLAGS

# Archived → anything (should fail)
$BINARY tx home update-home --home-id <HOME_ID> --status active --from val0 $TX_FLAGS
```

**Verify:**
- [ ] Valid transitions work
- [ ] Invalid transitions fail with clear error message
- [ ] Archived is terminal
- [ ] BeginBlocker skips archived homes (no deadman checks, no session cleanup)

### 10. Query Everything

Run all query commands and verify output structure:

```bash
$BINARY query home params $Q_FLAGS
$BINARY query home home <HOME_ID> $Q_FLAGS
$BINARY query home homes-by-owner $AGENT $Q_FLAGS
$BINARY query home keys <HOME_ID> $Q_FLAGS
$BINARY query home sessions <HOME_ID> $Q_FLAGS
$BINARY query home alerts <HOME_ID> $Q_FLAGS
$BINARY query home spending-limits <HOME_ID> $Q_FLAGS
```

**Verify:**
- [ ] All queries return valid JSON
- [ ] Empty results return empty arrays, not errors
- [ ] Params query shows all 8 params with values

## Report Template

For each scenario, record:

```markdown
### Scenario N: <name>
**Status:** PASS / FAIL / ISSUE
**Tx Hash:** <hash>
**Observation:** <what happened>
**Issue:** <if any — describe the problem>
**Suggestion:** <improvement idea>
```

## Exit Criteria

1. All 10 scenarios attempted on live localnet
2. Each scenario has PASS/FAIL/ISSUE status documented
3. Every CLI command tested (7 tx + 7 query)
4. At least 5 improvement suggestions identified
5. Report written to `docs/home-e2e-report.md`

## Commit Convention

```
test(home): e2e lifecycle testing on localnet
docs(home): e2e test report with improvement findings
fix(home): <any fixes discovered during testing>
```
