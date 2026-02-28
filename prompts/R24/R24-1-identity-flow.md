# R24-1 — Agent Identity Flow: DID Registration → Sessions → Home

## Context

An agent arrives at ZERONE with nothing — no identity, no home, no keys. This session tests the entire identity bootstrapping sequence via CLI on a live localnet. The x/auth module has 12 RPCs, most never tested as a connected flow.

## Prerequisites

- Localnet running (`scripts/localnet.sh start`)

## Task

### 1. Create a Fresh Account

```bash
# Generate a new key (simulating an agent arriving fresh)
$BINARY keys add agent-alpha --keyring-backend test --home $HOME_DIR
AGENT=$($BINARY keys show agent-alpha -a --keyring-backend test --home $HOME_DIR)
echo "Fresh agent address: $AGENT"

# Fund it from val0 (simulating faucet)
$BINARY tx bank send $VAL0_ADDR $AGENT 100000000uzrn --from val0 $TX_FLAGS
sleep 3

# Verify balance
$BINARY query bank balances $AGENT $Q_FLAGS
```

### 2. Register Account with DID

```bash
$BINARY tx auth register-account \
    --did "did:zerone:agent-alpha-001" \
    --account-type "agent" \
    --metadata '{"model":"claude-4","version":"1.0"}' \
    --from agent-alpha $TX_FLAGS
```

**Verify:**
- [ ] Tx succeeds
- [ ] Query: `$BINARY query auth account $AGENT $Q_FLAGS` — DID mapped
- [ ] What does `account_type` accept? ("agent", "human", "validator"?)
- [ ] Is `public_key` required or auto-derived from the signing key?
- [ ] Is `operational_key_hash` needed? What is it?
- [ ] Can you register the same DID twice? (should fail)
- [ ] Can two accounts share the same DID? (should fail)

**Issues to look for:**
- Is there a DID format validation? (`did:zerone:...` or anything?)
- Is metadata validated? Size limits?
- Is there a registration fee?
- What does the auth module query surface look like?

### 3. Check DID Resolution

```bash
# Query DID → address resolution
$BINARY query auth did "did:zerone:agent-alpha-001" $Q_FLAGS 2>/dev/null

# Query address → DID resolution
$BINARY query auth account-did $AGENT $Q_FLAGS 2>/dev/null
```

**Verify:**
- [ ] Bidirectional resolution works (DID → address, address → DID)
- [ ] Querying a non-existent DID returns clear error
- [ ] This is what BVM's `AuthKeeper.GetAccountDID` calls under the hood

### 4. Create Session Key

```bash
# Generate a session key
$BINARY keys add agent-alpha-session --keyring-backend test --home $HOME_DIR
SESSION_KEY=$($BINARY keys show agent-alpha-session -a --keyring-backend test --home $HOME_DIR)

$BINARY tx auth create-session \
    --session-key $SESSION_KEY \
    --capabilities "transfer,submit_claim" \
    --duration-blocks 1000 \
    --from agent-alpha $TX_FLAGS
```

**Verify:**
- [ ] Session created
- [ ] Session key has restricted capabilities
- [ ] Session key can sign txs on behalf of the agent (with limited permissions)
- [ ] Session expires after duration_blocks

**Issues to look for:**
- The R23-3 report found a **proto parse error** when creating sessions. This is the first real test — does it work via CLI or reproduce the bug?
- What capability strings are valid?
- Can the session key send a bank transfer? (only if "transfer" capability)
- Can the session key submit a knowledge claim? (only if "submit_claim" capability)

### 5. Key Rotation

```bash
$BINARY tx auth rotate-key \
    --new-key <new_pubkey> \
    --authorization-signature <sig> \
    --from agent-alpha $TX_FLAGS
```

**Verify:**
- [ ] Key rotation succeeds
- [ ] Old key can no longer sign
- [ ] New key works immediately
- [ ] DID mapping preserved (same DID, different key)

**Issues to look for:**
- How is `authorization_signature` generated? Is there a helper?
- Is this too complex for an agent to do programmatically?

### 6. Create Home (Integration with Identity)

```bash
# Now that agent has DID, create a home
$BINARY tx home create-home --name "Alpha Station" --from agent-alpha $TX_FLAGS

# Verify home links to the DID
HOME_ID=$($BINARY query home homes-by-owner $AGENT $Q_FLAGS | jq -r '.home_ids[0]')
$BINARY query home home $HOME_ID $Q_FLAGS
```

**Verify:**
- [ ] Home created for DID-registered agent
- [ ] Does home creation require DID registration? (probably not, but should it?)
- [ ] Can an agent without DID create a home? (test this too)

### 7. Register Keys on Home

```bash
# Register the session key on the home
SESSION_KEY_HASH=$(echo -n "$SESSION_KEY" | sha256sum | cut -d' ' -f1)

$BINARY tx home register-key \
    --home-id $HOME_ID \
    --key-hash "$SESSION_KEY_HASH" \
    --key-type ed25519 \
    --role session \
    --permissions "submit_claim,memory_write" \
    --from agent-alpha $TX_FLAGS
```

**Verify:**
- [ ] Key registered on home
- [ ] Can start a home session with this key
- [ ] Is there a relationship between auth session keys and home keys? (probably separate systems — document)

### 8. Account Recovery Setup

```bash
$BINARY tx auth set-recovery-config \
    --recovery-addresses "$VAL0_ADDR,$VAL1_ADDR" \
    --threshold 2 \
    --from agent-alpha $TX_FLAGS
```

**Verify:**
- [ ] Recovery config stored
- [ ] Compare with home guardian recovery — are these separate systems?
- [ ] Is there duplication between x/auth recovery and x/home guardian recovery?

### 9. Account Freeze/Unfreeze

```bash
# Freeze the account (emergency lockdown)
$BINARY tx auth freeze-account --from agent-alpha $TX_FLAGS

# Verify: all txs from this account should fail
$BINARY tx bank send $AGENT $VAL0_ADDR 1000uzrn --from agent-alpha $TX_FLAGS
# Should fail

# Unfreeze
$BINARY tx auth unfreeze-account --from agent-alpha $TX_FLAGS
```

**Verify:**
- [ ] Frozen account can't send any txs
- [ ] Unfreeze restores normal operation
- [ ] Can a frozen account still receive funds?
- [ ] Who can unfreeze? Only the account owner? Recovery addresses?

### 10. Full Flow Timing

Run the entire sequence (steps 1-7) and time it:

```bash
time {
    # Create key, fund, register DID, create session, create home, register home key
    # Each step waits for tx inclusion
}
```

**Document:**
- [ ] Total time from zero to fully set up agent
- [ ] Number of transactions required (should be ~6)
- [ ] Total cost in uzrn (fees + home creation)
- [ ] Could this be automated into a single script?

## Report Template

```markdown
### Step N: <name>
**Status:** PASS / FAIL / BLOCKED
**Tx Hash:** <hash>
**Time:** <seconds>
**Cost:** <uzrn>
**Issue:** <if any>
**UX Note:** <complexity observation>
```

## Exit Criteria

1. Full identity flow completed on localnet (DID → session → home → keys)
2. Session key capability restriction tested (or bug reproduced)
3. Recovery config tested
4. Freeze/unfreeze tested
5. Total onboarding time and cost documented
6. Auth/home key system overlap documented
7. Report written to `docs/agent-identity-flow-report.md`

## Commit Convention

```
test(auth): agent identity bootstrapping e2e on localnet
docs(auth): agent identity flow report
fix(auth): <any issues found>
```
