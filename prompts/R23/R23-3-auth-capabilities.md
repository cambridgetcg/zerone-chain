# R23-3 — Auth & Capabilities: DID-Gated BVM Execution

## Context

R15-1 wired auth/DID into BVM:

- `CallerDID` resolved via `AuthKeeper.GetAccountDID` before execution
- `SessionCapabilities` populated via `AuthKeeper.GetSessionCapabilities`
- Anonymous callers (no DID) get `nil` capabilities → all agent ops denied (C-1 secure default)
- Scheduled execution inherits the scheduler's capabilities

This session tests these on a live chain — not just unit tests, but actual CLI-driven execution with real DID-registered accounts vs anonymous accounts.

## Prerequisites

- R23-1 complete (know how to deploy and call contracts)
- Localnet running
- Understanding of x/auth DID registration (check `zeroned tx auth --help`)

## Task

### 1. Check Auth Module DID Support

```bash
# Is DID registration available?
$BINARY tx auth --help 2>&1 | grep -i "did\|register\|identity"

# Check if val0 has a DID
$BINARY query auth did $AGENT0 $Q_FLAGS 2>/dev/null || echo "No DID query endpoint"
```

**Document:**
- [ ] Is DID registration a separate tx (`MsgRegisterDID`) or auto-assigned?
- [ ] What does a DID look like? (`did:zerone:...`?)
- [ ] Are validators auto-registered with DIDs at genesis?

### 2. Test Authenticated vs Anonymous Calls

Deploy a contract that uses KVERIFY (which checks CallerDID):

The current KVERIFY stub always returns false, but the *capability check* happens before the host function is called. If capabilities are nil (anonymous), the interpreter should deny the operation.

Check the interpreter code for capability gates:

```bash
grep -n "Capabilities\|caps\|CanSubmitClaims\|CanVote" ~/Desktop/zerone/x/bvm/vm/interpreter.go | head -10
```

**If capability gates exist on KVERIFY/KCITE:**

```bash
# Call from val0 (may have DID) — should reach the host function (even if stub returns false)
$BINARY tx bvm call-contract --contract-address <ADDR> --from val0 $TX_FLAGS

# Create a fresh account with no DID
$BINARY keys add anon-agent --keyring-backend test --home $HOME_DIR
# Fund it
$BINARY tx bank send $AGENT0 $ANON_ADDR 10000000uzrn --from val0 $TX_FLAGS

# Call from anon — should be denied at capability check
$BINARY tx bvm call-contract --contract-address <ADDR> --from anon-agent $TX_FLAGS
```

**Verify:**
- [ ] Authenticated caller: KVERIFY reaches host function (returns false from stub, but no capability error)
- [ ] Anonymous caller: execution fails at capability check before reaching host function
- [ ] Error message distinguishes "no DID" from "operation denied"

**If no capability gates in interpreter** (agent opcodes not gated by capabilities yet):

Document this as a gap. The CallerDID and Capabilities are in ExecutionContext but may not be checked before KQUERY/KVERIFY/KCITE execute.

### 3. Session Key Capabilities

Test the capability restriction chain:

```bash
# Check if x/auth has session key support
$BINARY tx auth --help 2>&1 | grep -i "session"

# If session keys exist:
# Register a session key for val0 with limited capabilities (e.g., only CanSubmitClaims)
# Call a contract that does KVERIFY (requires CanVote) — should be denied
# Call a contract that does KQUERY (read-only, no capability needed) — should work
```

**Verify:**
- [ ] Session key with `CanSubmitClaims=true, CanVote=false` can KQUERY but not KVERIFY
- [ ] Full-access key can do both
- [ ] Capability check happens at the correct level (BVM opcode execution, not tx ante)

### 4. Scheduled Execution Capability Inheritance

From the code (`msg_server.go:540-570`), scheduled execution inherits the scheduler's capabilities:

```bash
# Deploy a contract with a scheduled execution
$BINARY tx bvm schedule-execution \
    --contract-address <ADDR> \
    --execute-at-block $((CURRENT_HEIGHT + 20)) \
    --gas-limit 100000 \
    --from val0 $TX_FLAGS

# Wait for the scheduled block
# Check: did the scheduled execution use val0's capabilities?
```

**Verify:**
- [ ] Scheduled execution runs at the correct block
- [ ] Capabilities inherited from scheduler (not from the block proposer)
- [ ] If scheduler's DID is revoked between scheduling and execution — what happens?

**Issues to look for:**
- Time-of-check vs time-of-use: capabilities are resolved at schedule creation time? Or at execution time?
- If at creation time: stale capabilities could be used
- If at execution time: the scheduler might no longer have permissions

### 5. Contract-to-Contract Calls with Capabilities

```bash
# Deploy contract A that CREATEs or CALLs contract B
# Does contract B inherit A's capabilities?
# Or does B get the original caller's capabilities?
```

Check the interpreter's CALL handling for capability propagation:

```bash
grep -n "CALL\|callDepth\|Capabilities" ~/Desktop/zerone/x/bvm/vm/interpreter.go | head -20
```

**Verify:**
- [ ] Internal calls: capabilities propagate from original caller (like `msg.sender` vs `tx.origin`)
- [ ] DELEGATECALL: capabilities from caller context
- [ ] STATICCALL: no state changes allowed (regardless of capabilities)

### 6. Capability Denial Events

When a capability check fails:

- [ ] Is an event emitted? (for audit trail)
- [ ] Is the error message descriptive? ("anonymous caller denied" vs generic "execution failed")
- [ ] Is gas consumed up to the denial point? Or fully consumed?

### 7. Edge Cases

```bash
# What if AuthKeeper returns a DID but GetSessionCapabilities returns (_, false)?
# From code: "Identity/operational key with no session key → full access"
# This means: any registered DID without a specific session key gets ALL capabilities.
# Is this the intended behavior? Document the security implications.
```

- [ ] DID exists, no session key → full access (intended?)
- [ ] DID exists, session key expired → what happens?
- [ ] DID revoked between contract deploy and call → capabilities?

## Report Template

```markdown
### Test N: <name>
**Status:** PASS / FAIL / GAP / NOT_TESTABLE
**Observation:** <what happened>
**Security Note:** <if relevant>
**Recommendation:** <if gap found>
```

## Exit Criteria

1. Authenticated vs anonymous BVM execution tested
2. Capability gating on agent opcodes verified (or gap documented)
3. Session key capability restriction tested (or gap documented if no session key CLI)
4. Scheduled execution capability inheritance documented
5. Cross-call capability propagation documented
6. Security implications of "DID + no session key = full access" documented
7. Report written to `docs/bvm-auth-capabilities-report.md`

## Commit Convention

```
test(bvm): auth/DID capability gating e2e
docs(bvm): auth capabilities report — gating status, security analysis
fix(bvm): <any capability enforcement fixes>
```
