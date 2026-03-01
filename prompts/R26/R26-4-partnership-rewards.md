# R26-4 — Wire Partnership Reward Routing into Knowledge Module

## Context

Two disconnected systems:

**Knowledge module** (`x/knowledge/keeper/rounds.go:120-147`):
- `distributeVerifierRewardsFromPool` pays verifiers directly, bypassing partnerships
- No `PartnershipKeeper` interface in `expected_keepers.go`
- `partnership_id` on claims is stored but never validated or used

**Partnerships module** (`x/partnerships/keeper/rewards.go:20-70`):
- `DistributeReward()` fully implements split logic: lock multiplier, common pot share, human/agent split
- Never called from any other module

**Goal:** When a claim has a valid `partnership_id`, route rewards through the partnership split instead of paying the submitter directly.

## Task

### 1. Define PartnershipKeeper Interface

Add to `x/knowledge/types/expected_keepers.go`:
```go
// PartnershipKeeper defines the expected partnership keeper interface.
type PartnershipKeeper interface {
    // GetPartnership returns a partnership by ID.
    GetPartnership(ctx context.Context, partnershipId string) (*partnershiptypes.Partnership, error)
    // IsParticipant checks if an address is a participant in a partnership.
    IsParticipant(ctx context.Context, partnershipId string, address string) (bool, error)
    // IsActive checks if a partnership is active (not frozen, dissolved, etc).
    IsActive(ctx context.Context, partnershipId string) (bool, error)
    // DistributeReward distributes a reward through the partnership split.
    DistributeReward(ctx context.Context, partnershipId string, amount sdk.Coins, source string) error
}
```

Check `x/partnerships/keeper/` for existing methods that match — you may need to add thin wrappers.

### 2. Add PartnershipKeeper to Knowledge Keeper

Mirror the pattern used for `DomainQualificationKeeper`:
- Add field to keeper struct
- Add setter method `SetPartnershipKeeper`
- Call setter in `app/app.go`

### 3. Validate partnership_id on Claim Submission

In `x/knowledge/keeper/msg_server.go` (MsgSubmitClaim handler, ~line 171):

Currently `PartnershipId` is copied blindly. Add validation:
```go
if msg.PartnershipId != "" {
    if k.partnershipKeeper == nil {
        return nil, sdkerrors.Wrap(types.ErrInvalidPartnership, "partnership module not available")
    }
    // Check partnership exists and is active
    active, err := k.partnershipKeeper.IsActive(ctx, msg.PartnershipId)
    if err != nil || !active {
        return nil, sdkerrors.Wrapf(types.ErrInvalidPartnership,
            "partnership %s is not active", msg.PartnershipId)
    }
    // Check submitter is a participant
    isParticipant, err := k.partnershipKeeper.IsParticipant(ctx, msg.PartnershipId, msg.Creator)
    if err != nil || !isParticipant {
        return nil, sdkerrors.Wrapf(types.ErrInvalidPartnership,
            "%s is not a participant in partnership %s", msg.Creator, msg.PartnershipId)
    }
}
```

### 4. Route Rewards Through Partnership Split

In `x/knowledge/keeper/rounds.go` (reward distribution):

When distributing rewards for a claim that has a valid `partnership_id`:
```go
if claim.PartnershipId != "" && k.partnershipKeeper != nil {
    err := k.partnershipKeeper.DistributeReward(ctx, claim.PartnershipId, rewardCoins, "knowledge_verification")
    if err != nil {
        // Fallback to direct payment on partnership error
        logger.Error("partnership reward routing failed, paying directly", "err", err)
        // ... direct payment fallback
    }
} else {
    // Direct payment (existing logic)
}
```

### 5. Reject Claims Through Frozen Partnerships

If a partnership has an active coercion freeze, claims should not be submittable through it:

```go
// In claim submission, after checking IsActive:
partnership, _ := k.partnershipKeeper.GetPartnership(ctx, msg.PartnershipId)
if partnership.Status == "suspended" {
    return nil, sdkerrors.Wrapf(types.ErrPartnershipFrozen,
        "partnership %s is frozen due to coercion signal", msg.PartnershipId)
}
```

### 6. Test

**Unit tests:**
- Claim with valid partnership_id → accepted, partnership validated
- Claim with non-existent partnership_id → rejected
- Claim with partnership_id where submitter isn't participant → rejected
- Claim with frozen partnership → rejected
- Claim with empty partnership_id → accepted (solo claim, existing behavior)
- Reward distribution routes through partnership split when partnership_id set
- Reward distribution falls back to direct when partnership_id empty
- Partnership keeper nil → partnership_id validation skipped (backward compat)

**Integration test:**
```bash
# Form a partnership
$BINARY tx partnerships propose-partnership $HUMAN_ADDR $AGENT_ADDR 50 50 --from human $TX_FLAGS
$BINARY tx partnerships accept-partnership <partnership_id> --from agent $TX_FLAGS

# Submit a claim through the partnership
$BINARY tx knowledge submit-claim "partnered claim" general computational 1000000 \
    --partnership-id <partnership_id> --from human $TX_FLAGS

# After verification, check reward went through partnership split
$BINARY query partnerships partnership <partnership_id> $Q_FLAGS  # Check common pot
```

## Files to Modify

- `x/knowledge/types/expected_keepers.go` — Add PartnershipKeeper interface
- `x/knowledge/keeper/keeper.go` — Add field + setter
- `x/knowledge/keeper/msg_server.go` — Validate partnership_id on submission
- `x/knowledge/keeper/rounds.go` — Route rewards through partnership
- `x/partnerships/keeper/` — Add any missing methods to satisfy interface
- `app/app.go` — Wire SetPartnershipKeeper
- Tests in `x/knowledge/keeper/` and `x/partnerships/keeper/`

## Success Criteria

- [ ] partnership_id validated on claim submission (exists, active, submitter is participant)
- [ ] Frozen partnerships block claim submission
- [ ] Rewards route through DistributeReward when partnership_id is set
- [ ] Solo claims (no partnership_id) work unchanged
- [ ] Partnership keeper nil = backward compatible (no validation)
- [ ] All existing tests pass
- [ ] New tests cover validation + routing + fallback
