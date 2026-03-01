# R19-6 — Non-Refundable Review Fee (Option C Revenue Split)

## Context

The current claim stake model is refundable on acceptance and partially slashed on rejection. This creates weak incentives:

- **Submitting is nearly free if you're right** — verifiers do real work (read, evaluate, commit, reveal) but the submitter pays almost nothing for a successful claim (just gas).
- **Spam is cheap** — submit 100 garbage claims, get 78% back on each rejection. Net cost: ~0.42 ZRN per failed claim.
- **Verifier compensation is disconnected** — verification rewards come from protocol inflation, not from the submitter who benefits.

Changing the stake to a **non-refundable review fee** fixes all three. The fee pays for the review process regardless of outcome, like a patent filing fee or journal submission fee. If the claim is accepted, it generates economic value through vesting rewards. If rejected, the fee is lost — the market said your knowledge wasn't valuable.

The fee follows the **same 55/22/19.67/3.33 revenue split** as block rewards (Option C), keeping economics consistent across the protocol.

## Task

### 1. Rename: Stake → Review Fee

This is primarily a naming and behavioral change. The existing `stake` field on `MsgSubmitClaim` becomes a review fee.

In `proto/zerone/knowledge/v1/genesis.proto`, rename:

```protobuf
// Before:
string min_claim_stake = X;

// After:
string min_review_fee = X;  // Minimum non-refundable fee for claim submission (uzrn)
```

In `x/knowledge/types/genesis.go`, update defaults:

```go
MinReviewFee: "100000",  // 0.1 ZRN — non-refundable review fee
```

Keep `min_challenge_stake` as-is — challenges are a different mechanism (stake is still at-risk for the challenger).

### 2. Remove Claim Stake Refund Logic

In `x/knowledge/keeper/rounds.go`, `CompleteRound()`:

**Remove** the `returnClaimStake()` call from the ACCEPT case:

```go
// Before:
case types.Verdict_VERDICT_ACCEPT:
    if err := k.createFactFromClaim(ctx, claim, round, result.Confidence); err != nil {
        return err
    }
    if err := k.returnClaimStake(ctx, claim); err != nil { // ← DELETE THIS
        ...
    }

// After:
case types.Verdict_VERDICT_ACCEPT:
    if err := k.createFactFromClaim(ctx, claim, round, result.Confidence); err != nil {
        return err
    }
    // Review fee already distributed at submission time — no refund
```

**Remove** the `returnClaimStake()` call from the INCONCLUSIVE case:

```go
// Before:
case types.Verdict_VERDICT_INCONCLUSIVE:
    if err := k.returnClaimStake(ctx, claim); err != nil { // ← DELETE THIS
        ...
    }

// After:
case types.Verdict_VERDICT_INCONCLUSIVE:
    // Review fee is non-refundable — verifiers still did work even if inconclusive
```

**Replace** the `slashAndBurnClaimStake()` call in the REJECT case:

```go
// Before:
case types.Verdict_VERDICT_REJECT:
    if err := k.slashAndBurnClaimStake(ctx, claim, params.InvalidClaimSlashBps); err != nil {

// After:
case types.Verdict_VERDICT_REJECT:
    // Review fee already distributed at submission time — no additional slashing needed
    // The fee IS the cost of rejection
```

### 3. Distribute Fee at Submission Time

In `x/knowledge/keeper/msg_server.go`, `SubmitClaim()`, replace the current stake lock with immediate distribution:

```go
// Before: Lock stake in module account
if m.keeper.bankKeeper != nil {
    submitterAddr, _ := sdk.AccAddressFromBech32(msg.Submitter)
    coins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(stakeAmt)))
    if err := m.keeper.bankKeeper.SendCoinsFromAccountToModule(ctx, submitterAddr, types.ModuleName, coins); err != nil {
        return nil, fmt.Errorf("failed to lock stake: %w", err)
    }
}

// After: Distribute review fee immediately via revenue split
if m.keeper.bankKeeper != nil {
    submitterAddr, _ := sdk.AccAddressFromBech32(msg.Submitter)
    feeAmt := stakeAmt.Uint64()

    // Collect fee from submitter to module first
    feeCoins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(stakeAmt)))
    if err := m.keeper.bankKeeper.SendCoinsFromAccountToModule(ctx, submitterAddr, types.ModuleName, feeCoins); err != nil {
        return nil, fmt.Errorf("failed to collect review fee: %w", err)
    }

    // Distribute via revenue split: 55% contributor pool, 22% protocol, 19.67% development, 3.33% research
    if err := k.distributeReviewFee(ctx, feeAmt); err != nil {
        k.Logger(ctx).Error("failed to distribute review fee", "error", err)
    }
}
```

### 4. Implement distributeReviewFee

In `x/knowledge/keeper/rounds.go` (or a new `x/knowledge/keeper/fees.go`):

```go
// distributeReviewFee distributes a non-refundable review fee using the standard revenue split.
// 55% → verification reward pool (held in module, paid to verifiers on round completion)
// 22% → protocol treasury
// 19.67% → development fund  
// 3.33% → research fund
func (k Keeper) distributeReviewFee(ctx context.Context, feeAmount uint64) error {
    if k.bankKeeper == nil || feeAmount == 0 {
        return nil
    }

    // Use the vesting_rewards revenue split for consistency
    split := k.getRevenueSplit(ctx)

    verifierPool := safeMulDiv(feeAmount, split.ContributorBps, 1_000_000)   // 55%
    protocolAmt  := safeMulDiv(feeAmount, split.ProtocolBps, 1_000_000)      // 22%
    devAmt       := safeMulDiv(feeAmount, split.DevelopmentBps, 1_000_000)   // 19.67%
    researchAmt  := feeAmount - verifierPool - protocolAmt - devAmt          // 3.33% (remainder)

    // Verifier pool stays in knowledge module — distributed to verifiers when round completes
    // (Already there from the initial SendCoinsFromAccountToModule)

    // Send protocol share
    if protocolAmt > 0 {
        coins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(new(big.Int).SetUint64(protocolAmt))))
        _ = k.bankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, "protocol_treasury", coins)
    }

    // Send development share
    if devAmt > 0 {
        coins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(new(big.Int).SetUint64(devAmt))))
        _ = k.bankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, "development_fund", coins)
    }

    // Send research share
    if researchAmt > 0 {
        coins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(new(big.Int).SetUint64(researchAmt))))
        _ = k.bankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, "research_fund", coins)
    }

    // verifierPool (55%) remains in the knowledge module account,
    // to be distributed to verifiers when the round completes via distributeVerifierReward()

    return nil
}
```

### 5. Verifier Reward: Fund from Fee Pool Instead of Minting

In `x/knowledge/keeper/rounds.go`, `distributeVerifierReward()`:

The current `VerificationReward` param (3 ZRN per verifier) is funded from... nowhere clear (module account). Now it's explicitly funded by the review fee's 55% contributor share.

Update the reward calculation to distribute from the pool proportionally:

```go
// distributeVerifierReward distributes the verification reward pool among correct verifiers.
func (k Keeper) distributeVerifierRewardsFromPool(ctx context.Context, round *types.VerificationRound, result *VerificationResult) {
    if k.bankKeeper == nil {
        return
    }

    // Calculate how much was allocated to verifier pool (55% of the original fee)
    claim, found := k.GetClaim(ctx, round.ClaimId)
    if !found {
        return
    }
    feeAmt, _ := new(big.Int).SetString(claim.Stake, 10) // "stake" field still holds fee amount
    split := k.getRevenueSplit(ctx)
    poolAmount := safeMulDiv(feeAmt.Uint64(), split.ContributorBps, 1_000_000)

    // Divide pool equally among rewarded verifiers
    if len(result.Rewards) == 0 || poolAmount == 0 {
        return
    }
    perVerifier := poolAmount / uint64(len(result.Rewards))
    remainder := poolAmount - (perVerifier * uint64(len(result.Rewards)))

    for i, reward := range result.Rewards {
        amount := perVerifier
        if i == 0 {
            amount += remainder // first verifier gets dust
        }
        result.Rewards[i].Amount = amount
        k.distributeVerifierReward(ctx, reward.Verifier, amount)
    }
}
```

### 6. Update Events

Add `fee_distributed` event in `SubmitClaim`:

```go
sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
    "zerone.knowledge.review_fee_distributed",
    sdk.NewAttribute("claim_id", claimID),
    sdk.NewAttribute("fee_amount", msg.Stake),
    sdk.NewAttribute("verifier_pool", fmt.Sprintf("%d", verifierPool)),
    sdk.NewAttribute("protocol", fmt.Sprintf("%d", protocolAmt)),
    sdk.NewAttribute("development", fmt.Sprintf("%d", devAmt)),
    sdk.NewAttribute("research", fmt.Sprintf("%d", researchAmt)),
))
```

### 7. Remove Dead Code

- Delete or deprecate `returnClaimStake()` — no longer called for claims
- Delete or deprecate `slashAndBurnClaimStake()` — fee is already distributed, no stake to slash
- Remove `InvalidClaimSlashBps` param (no longer applicable — there's no stake to slash on rejection)
- Keep `MalformedClaimSlashBps` from R19-1 — BUT reframe it: malformed claims could have a *higher* review fee multiplier rather than a slash on a refundable stake. Or simply: the fee is the fee, malformed or not. Verifiers still did work.

### 8. Update Params

In genesis defaults:

```go
// Remove:
InvalidClaimSlashBps: 220_000,  // no longer needed

// Rename:
MinReviewFee: "100000",  // 0.1 ZRN non-refundable review fee (was MinClaimStake)
```

Add validation:

```go
if p.MinReviewFee == "" || p.MinReviewFee == "0" {
    return fmt.Errorf("min_review_fee must be > 0")
}
```

### 9. CLI Update

Rename the positional arg from `stake` to `review-fee`:

```
zeroned tx knowledge submit-claim [fact-content] [domain] [category] [review-fee] [flags]
```

### 10. Tests

1. **TestSubmitClaim_FeeDistributed** — fee is split 55/22/19.67/3.33 on submission
2. **TestCompleteRound_Accept_NoRefund** — accepted claim does NOT return fee
3. **TestCompleteRound_Reject_NoRefund** — rejected claim does NOT return fee
4. **TestCompleteRound_Inconclusive_NoRefund** — inconclusive does NOT return fee
5. **TestVerifierRewards_FundedFromPool** — verifier rewards come from the 55% pool
6. **TestVerifierRewards_SplitEvenly** — pool divided among correct verifiers
7. **TestReviewFee_BelowMinimum** — fee below MinReviewFee rejected
8. **TestReviewFee_AboveMinimum** — higher fee = larger verifier pool (incentivizes faster/better review)

## Design Notes

- **Fee is distributed at submission, not completion** — verifiers are guaranteed compensation regardless of outcome. This is critical: if fees were held until completion, verifiers bear the time-value risk. Immediate distribution = immediate compensation commitment.
- **The `stake` field on Claim is kept** (proto backward compat) but now represents the fee amount paid, not a refundable deposit. The field name in proto stays `stake` to avoid migration; the semantic change is in the code.
- **Higher fees = better reviews** — submitting 10 ZRN instead of 1 ZRN means the verifier pool is 5.5 ZRN instead of 0.55 ZRN. This naturally attracts more/better verifiers to higher-fee claims. Market pricing for review quality.
- **Challenge stakes remain refundable** — challenges are adversarial, not applications. The challenger puts skin in the game; if the challenge succeeds, they get rewarded. Different mechanism, different economics.
- **Malformed claims (R19-1)** — with non-refundable fees, malformed doesn't need a separate slash rate. The fee is already lost. The malformed verdict still matters for verifier rewards (malformed-voters rewarded, accept-voters slashed) but the submitter economics are the same as rejection.

## Files Modified

- `proto/zerone/knowledge/v1/genesis.proto` — rename min_claim_stake → min_review_fee, remove InvalidClaimSlashBps
- `x/knowledge/types/genesis.go` — updated defaults + validation
- `x/knowledge/types/*.pb.go` — regenerated
- `x/knowledge/keeper/msg_server.go` — distribute fee at submission
- `x/knowledge/keeper/rounds.go` — remove refund logic, add distributeReviewFee, update verifier reward funding
- `x/knowledge/keeper/fees.go` — **NEW**: fee distribution logic
- CLI: rename stake → review-fee
- Tests: 8 new tests

## Commit

Single commit: `feat(knowledge): non-refundable review fee with revenue split distribution`
