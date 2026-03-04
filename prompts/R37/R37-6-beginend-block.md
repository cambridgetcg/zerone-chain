# R37-6 — BeginBlocker/EndBlocker: Block-Level Processing

## Objective

Implement the block-level lifecycle processing: quality round phase transitions, energy decay, pruning, fitness recalculation, and bounty maintenance.

## Tasks

### 1. BeginBlocker

```go
func (k Keeper) BeginBlocker(ctx context.Context) error {
    // 1. Transition quality rounds:
    //    - COMMIT → REVEAL (if commit deadline passed)
    //    - REVEAL → AGGREGATION (if reveal deadline passed)
    // 2. Process expired rounds (not enough validators participated)
    return nil
}
```

### 2. EndBlocker

```go
func (k Keeper) EndBlocker(ctx context.Context) error {
    sdkCtx := sdk.UnwrapSDKContext(ctx)
    blockHeight := uint64(sdkCtx.BlockHeight())
    params := k.GetParams(ctx)

    // 1. Aggregate completed quality rounds
    k.aggregateCompletedRounds(ctx, blockHeight)

    // 2. Process epoch boundary (if applicable):
    if isEpochBoundary(blockHeight, params) {
        // a. Decay energy for all active samples
        k.decayAllEnergy(ctx, params)

        // b. Recalculate fitness scores
        k.recalculateFitness(ctx, params)

        // c. Update niche rankings
        k.updateNicheRankings(ctx)

        // d. Prune at-risk samples past grace period
        k.pruneSamples(ctx, currentEpoch, params)

        // e. Expire old bounties
        k.expireBounties(ctx, blockHeight)

        // f. Reset epoch-level counters
        k.resetEpochCounters(ctx)
    }

    // 3. Expire patronage
    k.expirePatronage(ctx, blockHeight)

    // 4. Slash validators who missed reveals
    k.slashMissedReveals(ctx, blockHeight, params)

    return nil
}
```

### 3. Round Phase Transitions

Port from old knowledge module, adapting for quality rounds:

```go
func (k Keeper) transitionRounds(ctx context.Context, blockHeight uint64) {
    k.IterateActiveRounds(ctx, func(round *types.QualityRound) bool {
        switch round.Phase {
        case types.VERIFICATION_PHASE_COMMIT:
            if blockHeight >= round.CommitDeadline {
                if len(round.Commits) >= params.MinValidatorsPerRound {
                    round.Phase = types.VERIFICATION_PHASE_REVEAL
                } else {
                    // Not enough commits — expire round, return stakes
                    k.expireRound(ctx, round)
                }
            }
        case types.VERIFICATION_PHASE_REVEAL:
            if blockHeight >= round.RevealDeadline {
                round.Phase = types.VERIFICATION_PHASE_AGGREGATION
            }
        }
        return false
    })
}
```

### 4. Batch Energy Decay

```go
func (k Keeper) decayAllEnergy(ctx context.Context, params *types.Params) {
    k.IterateActiveSamples(ctx, func(sample *types.Sample) bool {
        // Skip sponsored samples
        if sample.PatronageExpiryBlock > currentBlock {
            return false
        }
        oldEnergy := sample.Energy
        k.decayEnergy(ctx, sample, params)
        if sample.Energy == 0 && oldEnergy > 0 {
            sample.AtRiskSinceEpoch = currentEpoch
        }
        k.SetSample(ctx, sample)
        return false
    })
}
```

### 5. Fitness Recalculation

Batch recalculate fitness for all active samples once per epoch. This is the same pattern as the old knowledge module but with training-data-specific fitness components.

### 6. Tests

- Round transition: COMMIT → REVEAL on deadline
- Round transition: REVEAL → AGGREGATION on deadline
- Round expiry: insufficient commits
- Missed reveal slashing
- Energy decay across epoch boundary
- Sponsored sample skips decay
- At-risk transition
- Pruning at grace period
- Fitness recalculation
- Niche ranking update
- Bounty expiry
- Patronage expiry
- Full lifecycle: submit → commit → reveal → aggregate → sample created → energy decay → access → energy restored

Target: ≥ 30 tests.

## Integration Test

Write one comprehensive integration test that exercises the full lifecycle across multiple blocks:

1. Block 1: Submit data
2. Block 2: Quality round starts, validators commit
3. Block N: Commit deadline → REVEAL phase
4. Block N+1: Validators reveal quality scores
5. Block M: Reveal deadline → AGGREGATION
6. Block M+1: Aggregation → Sample created (gold)
7. Block M+100: Energy decay
8. Block M+101: Consumer accesses sample → energy restored, revenue distributed
9. Block M+1000: Sample at risk (no accesses)
10. Block M+2000: Sample pruned

This proves the full flow works end-to-end within the block processing framework.
