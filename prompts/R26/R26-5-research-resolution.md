# R26-5 — Auto-Resolution for Research and Bounties

## Context

`MsgResolveResearch` and bounty fulfillment currently require governance authority (the `authority` field must be the governance module address). This means:

- Research submissions can never complete their lifecycle without a governance proposal
- Bounties can never be fulfilled without governance
- The research fund accumulates tokens with no way to distribute them
- The entire research → review → resolve → reward flow is broken

## Task

### 1. Add Auto-Resolution in EndBlocker

In `x/research/module.go` (or wherever the EndBlocker is), add logic to auto-resolve research when conditions are met:

**Resolution conditions:**
- Research status is REVIEWING (past submission, reviews collected)
- `review_period_blocks` has elapsed since first review
- `min_reviewer_count` reviews have been submitted
- Aggregate score meets acceptance threshold (check params)

```go
func (k Keeper) AutoResolveResearch(ctx sdk.Context) {
    // Iterate research with status REVIEWING
    // For each: check if review_period elapsed AND min reviews met
    // If yes: calculate aggregate score, transition to ACCEPTED or REJECTED
    // If ACCEPTED: release escrowed stake back to researcher
    // If REJECTED: slash stake per params
    // Emit resolution event
}
```

### 2. Add Auto-Fulfillment for Bounties

Similarly, bounties should auto-fulfill when:
- A deliverable has been submitted (claimed)
- The review period has elapsed
- Reviewers have approved (or no objections within window)

```go
func (k Keeper) AutoFulfillBounties(ctx sdk.Context) {
    // Iterate bounties with status CLAIMED (deliverable submitted)
    // For each: check if fulfillment_period elapsed
    // If accepted deliverable: transfer bounty reward to claimer
    // If rejected: return bounty to creator, release claimer
    // Emit fulfillment event
}
```

### 3. Keep Governance Path as Override

Don't remove `MsgResolveResearch` — governance should still be able to force-resolve disputed research. But make auto-resolution the default path.

### 4. Check Research Fund Wiring

Now that R26-1 activates block rewards, the research fund will actually have tokens. Verify:
- Research bounties can draw from the research fund
- Research rewards are paid from the correct module account
- The fund balance decreases correctly after payouts

### 5. Test

**Unit tests:**
- Research with enough reviews + elapsed period → auto-resolved
- Research with insufficient reviews → not resolved (waits)
- Research within review period → not resolved (waits)
- Bounty with accepted deliverable + elapsed period → auto-fulfilled
- Governance can still force-resolve (override path)
- Escrowed stake returned on acceptance, slashed on rejection

**Integration test:**
```bash
# Submit research
$BINARY tx research submit-research "Test Research" "Abstract" "QmHash" general 10000000uzrn --from researcher $TX_FLAGS

# Submit reviews (need min_reviewer_count)
$BINARY tx research review-research <research_id> 8 "Good work" --from reviewer1 $TX_FLAGS
$BINARY tx research review-research <research_id> 7 "Solid" --from reviewer2 $TX_FLAGS

# Wait for review_period_blocks
sleep <appropriate_time>

# Check auto-resolution
$BINARY query research research <research_id> $Q_FLAGS  # Should be ACCEPTED
```

## Files to Modify

- `x/research/keeper/` — Add AutoResolveResearch, AutoFulfillBounties
- `x/research/module.go` — Call auto-resolve/fulfill in EndBlocker
- `x/research/types/params.go` — Verify review_period_blocks and min_reviewer_count params exist
- Tests in `x/research/keeper/`

## Success Criteria

- [ ] Research auto-resolves when review conditions met
- [ ] Bounties auto-fulfill when deliverable accepted + period elapsed
- [ ] Governance override still works
- [ ] Escrowed stakes handled correctly (return on accept, slash on reject)
- [ ] Research fund balance decreases after payouts
- [ ] All existing tests pass
- [ ] New tests cover auto-resolution lifecycle
