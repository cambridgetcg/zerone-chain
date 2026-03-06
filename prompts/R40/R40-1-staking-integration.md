# R40-1 — Reviewer Staking Integration

## Objective

Wire `reviewer_staking.go` into the quality round commit/reveal/aggregate flow so that reviewer stakes are automatically escrowed and distributed as part of the existing lifecycle.

## Context

- `reviewer_staking.go` has: `EscrowReviewerStake()`, `DistributeReviewerStakes()`, `HandleDeepContested()`, `RecordContestedStrike()`
- `quality_round.go` has: `SubmitCommitment()`, `RevealScore()`, `AggregateQualityRound()`
- `phases.go` handles phase transitions in BeginBlocker/EndBlocker
- Reviewer staking params are in `params_reviewer.go` with `ReviewerStakeRatioBps`, etc.

## Tasks

1. **In `SubmitCommitment()`**: After validating the commitment, call `EscrowReviewerStake(ctx, round, reviewer)` to lock the reviewer's stake. If escrow fails (insufficient funds), reject the commitment.

2. **In `AggregateQualityRound()`**: After determining majority/minority:
   - Call `DistributeReviewerStakes(ctx, round, majorityReviewers, minorityReviewers, outcome)` 
   - For accept: submitter gets stake back + accept bonus from module account
   - For reject: submitter loses stake, rejectors split challenge bonus
   - For deep contested: call `HandleDeepContested()` — return all stakes
   - Call `RecordContestedStrike()` for permanently rejected content hashes

3. **Show-up rewards**: In aggregation, identify reviewers who participated (committed + revealed) and distribute show-up reward from minority pot only (no rewards on unanimous votes).

4. **Params integration**: Load `ReviewerStakingParams` from state in quality round functions. Don't hardcode ratios.

5. **Error handling**: If distribution fails mid-way, ensure atomicity — use `CacheContext` + `Write()` pattern.

## Tests

- Test: reviewer commits → stake escrowed → verify balance decreased
- Test: accept outcome → submitter gets accept bonus, majority reviewers get stake back + show-up
- Test: reject outcome → submitter loses stake, rejectors split bonus
- Test: deep contested → all stakes returned
- Test: insufficient funds → commitment rejected
- Test: unanimous vote → no show-up rewards
- Test: 3 strikes → permanent reject

## Key Files

- `x/knowledge/keeper/quality_round.go` — modify
- `x/knowledge/keeper/reviewer_staking.go` — call into
- `x/knowledge/keeper/state.go` — existing CRUD
- `x/knowledge/types/params_reviewer.go` — load params

## Constraints

- Do NOT modify proto files
- Use existing `bankKeeper.SendCoins` / `SendCoinsFromModuleToAccount` for transfers
- All amounts in uzrn, use `sdkmath.Int`
