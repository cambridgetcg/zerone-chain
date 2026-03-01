# R26-3 — Wire Domain Qualification into Verification Flow

## Context

Both sides of this interface already exist:

**Knowledge module (consumer):**
- `x/knowledge/types/expected_keepers.go:51` — `DomainQualificationKeeper` interface defined with `IsQualified(ctx, validatorAddr, domain) (bool, error)`
- `x/knowledge/keeper/keeper.go:27` — `domainQualificationKeeper` field on keeper (nil)
- `x/knowledge/keeper/keeper.go:68` — `SetDomainQualificationKeeper(dk)` setter exists
- `x/knowledge/keeper/validator_selection.go:11` — Comment: "If DomainQualificationKeeper is available (R6-5), it filters by domain qualification"
- `x/knowledge/keeper/validator_selection.go:22` — Comment: "DomainQualificationKeeper is nil until R6-5 — return all active validators"

**Qualification module (provider):**
- `x/qualification/keeper/` — `IsQualified()`, `GetQualificationWeight()`, `GetQualifiedValidators()` all implemented
- Qualification statuses: ACTIVE, PROBATIONARY, SUSPENDED, REVOKED
- Four pathways: stake, track record, cross-reference, inheritance

**The code exists on both sides. Nobody called `SetDomainQualificationKeeper` in app wiring.**

## Task

### 1. Wire the Keeper in App Setup

In `app/app.go`, after both keepers are initialized, call:
```go
app.KnowledgeKeeper.SetDomainQualificationKeeper(app.QualificationKeeper)
```

Find the appropriate location — likely near other post-init keeper wiring (search for `Set.*Keeper` patterns in app.go).

### 2. Implement Qualification Filtering in Validator Selection

`x/knowledge/keeper/validator_selection.go` currently returns all active validators. Update it to:

1. Get the claim's domain from the verification round
2. Call `domainQualificationKeeper.IsQualified(ctx, validator, domain)` for each candidate
3. Filter out unqualified validators
4. **Fallback:** If fewer than `MinVerifiers` (from params) are qualified, log a warning and allow unqualified validators with a flag. Don't block verification entirely — this handles the bootstrapping problem

```go
// Pseudocode for the gating logic:
qualified := []Validator{}
for _, v := range activeValidators {
    if dk == nil {
        qualified = append(qualified, v)  // no qualification keeper = allow all
        continue
    }
    ok, err := dk.IsQualified(ctx, v.Address, claim.Domain)
    if err != nil {
        logger.Warn("qualification check failed", "validator", v.Address, "err", err)
        continue
    }
    if ok {
        qualified = append(qualified, v)
    }
}
if len(qualified) < minVerifiers {
    logger.Warn("insufficient qualified verifiers, falling back to all",
        "qualified", len(qualified), "min", minVerifiers, "domain", claim.Domain)
    qualified = activeValidators  // fallback
}
```

### 3. Gate SubmitCommitment

In `x/knowledge/keeper/msg_server.go` around line 239 (MsgSubmitCommitment handler):

After the existing checks (round exists, commit phase, min balance, no duplicates), add:
```go
// Check domain qualification
if k.domainQualificationKeeper != nil {
    claim, err := k.GetClaim(ctx, round.ClaimId)
    // ... error handling
    qualified, err := k.domainQualificationKeeper.IsQualified(ctx, msg.ValidatorAddress, claim.Domain)
    if !qualified {
        return nil, sdkerrors.Wrapf(types.ErrUnqualifiedVerifier,
            "validator %s is not qualified for domain %s", msg.ValidatorAddress, claim.Domain)
    }
}
```

**Design decision:** Should unqualified validators be hard-rejected or soft-warned? For testnet, recommend **hard reject with the fallback in validator selection** — if nobody's qualified, selection lets everyone through, but if some are qualified, only they can commit.

### 4. Wire RecordVerificationOutcome

This is a bonus connection that enables the track record pathway:

After a verification round completes (in the round finalization logic), call:
```go
qualificationKeeper.RecordVerificationOutcome(ctx, validatorAddr, domain, wasCorrect)
```

Where `wasCorrect` = whether the validator voted with the final consensus. This populates the metrics that the track record qualification pathway needs.

**Find the round completion code** — likely in an EndBlocker or in the reveal processing logic. Search for where claim status transitions from PENDING to VERIFIED/REJECTED.

### 5. Test

**Unit tests:**
- Qualified validator submits commitment → accepted
- Unqualified validator submits commitment → rejected
- No qualified validators → fallback allows all (with warning event)
- Validator selection filters by domain
- RecordVerificationOutcome updates qualification metrics

**Integration test on localnet:**
```bash
# Qualify a validator for "general" domain via stake pathway
$BINARY tx qualification qualify-stake val0 general 100000000uzrn --from val0 $TX_FLAGS

# Submit a claim in "general" domain
$BINARY tx knowledge submit-claim "test claim" general computational 1000000 --from submitter1 $TX_FLAGS

# Verify that only qualified validators participate in the round
$BINARY query knowledge round <round_id> $Q_FLAGS
```

## Files to Modify

- `app/app.go` — Add `SetDomainQualificationKeeper` call
- `x/knowledge/keeper/validator_selection.go` — Implement qualification filtering with fallback
- `x/knowledge/keeper/msg_server.go` — Add qualification check in SubmitCommitment
- `x/knowledge/keeper/rounds.go` (or wherever rounds finalize) — Call RecordVerificationOutcome
- Tests in `x/knowledge/keeper/` — New qualification gating tests

## Success Criteria

- [ ] `SetDomainQualificationKeeper` called in app wiring
- [ ] Validator selection filters by domain qualification
- [ ] Fallback to all validators when insufficient qualified ones
- [ ] SubmitCommitment rejects unqualified validators (when qualified ones exist)
- [ ] RecordVerificationOutcome called after round completion
- [ ] All existing tests pass (nil keeper path unchanged)
- [ ] New tests cover qualified/unqualified/fallback scenarios
