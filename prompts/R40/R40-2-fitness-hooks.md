# R40-2 — Fitness Decay Hooks

## Objective

Wire `fitness.go` into the BeginBlocker epoch cycle and quality round resolution so TDU fitness scores update automatically based on curation outcomes and decay over time.

## Context

- `fitness.go` has: `UpdateFitnessScore()`, `DecayUnscored()`, `ComputeLongevityReward()`, `DistributeLongevityRewards()`
- `ecology.go` runs epoch processing in BeginBlocker
- `quality_round.go` resolves submissions → samples
- Fitness signals come from: quality round outcomes (50% weight), usage/access patterns (30%), redundancy detection (20%)

## Tasks

1. **On quality round resolution** (`AggregateQualityRound()`):
   - When a submission is ACCEPTED → create `FitnessRecord` with initial score 0.5 (Active)
   - Generate a `FitnessSignal` with type `TrainingInfluence`, weight based on reviewer consensus strength (unanimous = 1.0, supermajority = 0.8, bare majority = 0.6)
   - Call `UpdateFitnessScore()` with the signal

2. **On sample access** (`AccessSample()` in `access.go`):
   - Generate `FitnessSignal` with type `UsageCorrelation`, positive weight
   - Call `UpdateFitnessScore()` to reinforce accessed TDUs

3. **In BeginBlocker epoch processing** (every `fitness_epoch_blocks`):
   - Call `DecayUnscored(ctx, currentHeight)` to decay TDUs with no signal for N cycles
   - Call `DistributeLongevityRewards(ctx)` — Core TDUs earn 0.01×stake/cycle, Active earn 0.005×
   - Prune TDUs with fitness < 0.1 (mark as Pruned, exclude from training datasets)

4. **Lifecycle transitions**:
   - When score crosses threshold (0.7 ↑ Core, 0.3 ↓ Dormant, 0.1 ↓ Pruned), emit events
   - Dormant TDUs excluded from dataset access queries (but retained in archive)
   - Pruned TDUs: on-chain record preserved, data removed from active shards

5. **Add `FitnessEpochBlocks` to params** (default 100 blocks, ~6 min on testnet)

## Tests

- Test: accepted submission creates FitnessRecord at 0.5
- Test: repeated access increases fitness toward Core
- Test: N epochs with no signal → gradual decay
- Test: fitness drops below 0.3 → status changes to Dormant
- Test: fitness drops below 0.1 → status changes to Pruned
- Test: Core TDU earns longevity reward per epoch
- Test: Pruned TDU earns nothing
- Test: lifecycle event emitted on status transition

## Key Files

- `x/knowledge/keeper/fitness.go` — call into
- `x/knowledge/keeper/ecology.go` — add fitness epoch processing
- `x/knowledge/keeper/quality_round.go` — create FitnessRecord on accept
- `x/knowledge/keeper/access.go` — signal on access
- `x/knowledge/types/fitness.go` — types

## Constraints

- Do NOT modify proto files
- Longevity rewards come from module account (same as accept bonus)
- Fitness epoch should be independent of ecology epoch (can differ)
