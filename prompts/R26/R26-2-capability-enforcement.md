# R26-2 — Capability Flag Enforcement in the AnteHandler

## Context

`ZeroneCapabilityDecorator` at `app/ante_zerone.go:734` currently only enforces capabilities for **session keys** — it checks if a session key has permission to submit claims, challenge, etc. But the underlying **account-level** capability flags (`CanSubmitClaims`, `CanChallenge`, `CanStake`, `CanVote`, `CanPartnership`, `CanResearch`, `CanDispute`, `CanTransfer`) set during registration are **never checked for primary keys**.

This means:
- Contract accounts have `CanSubmitClaims=false` set at registration but can submit claims freely
- System accounts are only blocked by the `Frozen` flag, not by individual capabilities
- The account_type distinction has zero enforcement

## Task

### 1. Extend ZeroneCapabilityDecorator for Primary Keys

Currently `AnteHandle` at line 761 only checks capabilities when the signer is using a session key. Add a parallel path:

```
For each message in the tx:
  1. Get the signer address
  2. Look up zerone_auth account (if registered)
  3. If registered AND signing with primary key (not session key):
     - Check account-level capabilities against message type
     - Reject if capability is false
  4. If NOT registered in zerone_auth:
     - Allow (standard Cosmos accounts bypass zerone restrictions)
     - OR require registration (design decision — document which you chose and why)
```

**The `checkCapability` method at line 808 already maps message types to capabilities.** Reuse this logic for primary keys — the same capability struct applies.

### 2. Map Account Types to Default Capabilities

Verify the registration defaults in `x/zerone_auth` match the intended restrictions:

| Account Type | CanSubmitClaims | CanChallenge | CanStake | CanVote | CanPartnership | CanResearch | CanDispute | CanTransfer |
|-------------|-----------------|--------------|----------|---------|----------------|-------------|------------|-------------|
| Human | true | true | true | true | true | true | true | true |
| Agent | true | true | true | true | true | true | true | true |
| Contract | **false** | **false** | false | false | true | false | false | true |
| System | **false** | **false** | **false** | **false** | false | false | false | false |

If the defaults in the registration handler don't match this table, fix them.

### 3. Handle Unregistered Accounts

Standard Cosmos accounts (those that haven't called `RegisterAccount` in zerone_auth) need a policy:

**Option A: Permissive** — Unregistered accounts bypass capability checks entirely. They can do anything. Registration adds restrictions, not permissions.

**Option B: Restrictive** — Unregistered accounts are treated as having minimal capabilities (transfer only). Registration unlocks capabilities.

**Recommend Option B** for testnet — it makes account_type meaningful and forces participants to declare whether they're human or agent. Document the choice.

### 4. Write Tests

Existing test structure is at `app/ante_test.go:549` and `app/ante_integration_test.go:351`.

**New tests needed:**
- Contract account tries MsgSubmitClaim → rejected (CanSubmitClaims=false)
- Contract account tries MsgInitiateChallenge → rejected (CanChallenge=false)
- Human account submits claim → allowed
- Agent account submits claim → allowed
- System account tries any tx → rejected (Frozen, already works — verify)
- Unregistered account policy (whichever option you chose)
- Capability check doesn't fire for standard bank/staking msgs (or does it? — design decision)

### 5. Verify No Regressions

```bash
cd ~/Desktop/zerone
go test ./app/... -v -run "TestAnte|TestCapability"
go test ./... -count=1 2>&1 | tail -20
```

All existing tests must pass. The change should be backwards-compatible for human/agent accounts (all caps = true).

## Files to Modify

- `app/ante_zerone.go` — Extend `ZeroneCapabilityDecorator.AnteHandle` for primary key capability checks
- `app/ante_test.go` — Add primary key capability enforcement tests
- `app/ante_integration_test.go` — Integration tests with real keeper
- Possibly `x/zerone_auth/keeper/msg_server.go` — If registration defaults need fixing

## Success Criteria

- [ ] Contract accounts blocked from submitting claims (CanSubmitClaims enforced)
- [ ] Contract accounts blocked from challenging (CanChallenge enforced)
- [ ] Human and agent accounts unaffected (all capabilities true)
- [ ] System accounts still blocked (Frozen enforcement unchanged)
- [ ] Unregistered account policy documented and tested
- [ ] All existing tests pass
- [ ] New tests cover each capability flag × account type combination
