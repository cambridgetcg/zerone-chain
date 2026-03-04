# R37-6 Design: BeginBlocker/EndBlocker Block-Level Processing

## Current State

- **BeginBlocker** (`phases.go:11-51`): Handles round phase transitions (COMMIT→REVEAL, REVEAL→AGGREGATION/EXPIRED) + ecology epoch every 100 blocks
- **No EndBlocker** registered
- `RunEcologyEpoch` in `ecology.go:260` handles decay, fitness, at-risk, prune
- Missing: min-validators check, sponsored skip, niche rankings in epoch, bounty/patronage expiry, missed reveal slashing

## Design

### Architecture: Split BeginBlocker + EndBlocker

**BeginBlocker** (setup/transitions):
1. Phase transitions with min-validators check on COMMIT→REVEAL
2. Round expiry returns stakes to submitter

**EndBlocker** (settlement/consequences):
1. Aggregate completed rounds (already in BeginBlocker via `AggregateQualityRound` — move here)
2. Epoch boundary processing: decay (skip sponsored), fitness, niche rankings, prune, bounty expiry, counter reset
3. Patronage expiry (every block, not just epochs)
4. Missed reveal slashing

### New Keeper Methods

| Method | Description |
|--------|-------------|
| `EndBlocker(ctx)` | Main EndBlocker entry point |
| `expireRound(ctx, round)` | Mark round EXPIRED, return stake to submitter |
| `expireBounties(ctx, blockHeight)` | Mark unclaimed bounties past `expires_at_block` as expired |
| `expirePatronage(ctx, blockHeight)` | Clear `patronage_expiry_block` on samples past expiry |
| `slashMissedReveals(ctx, blockHeight)` | Emit slash events for validators who committed but didn't reveal |
| `updateNicheRankings(ctx)` | Iterate all niches and call `UpdateNicheLeader` |
| `resetEpochCounters(ctx)` | Placeholder for future epoch-level counter resets |

### Modified Methods

- `BeginBlocker`: Remove ecology epoch, add min-validators check, use `expireRound`
- `RunEcologyEpoch`: Add sponsored sample skip, niche rankings, bounty expiry, counter reset

### Bounty State CRUD

Add to `state.go`:
- `SetDataBounty(ctx, bounty)`, `GetDataBounty(ctx, id)`, `DeleteDataBounty(ctx, id)`
- `IterateDataBounties(ctx, callback)` — iterate all bounties for expiry check

### Patronage Expiry

The `patronage_expiry_block` field on Sample already exists. Expiry = iterate samples, clear the field when `blockHeight >= patronage_expiry_block`. No separate PatronageRecord proto needed.

### Missed Reveal Slashing

For completed rounds, check `commits` vs `reveals` — validators who committed but didn't reveal get a slash event emitted. Actual token slashing deferred to staking integration (R37-4+). For now: emit events.

### Module Wiring

Add `EndBlock(ctx) error` to `module.go` calling `keeper.EndBlocker(ctx)`.

### Test Coverage (≥30 tests)

See implementation plan for full test list.
