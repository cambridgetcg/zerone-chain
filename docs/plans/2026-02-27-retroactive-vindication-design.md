# Retroactive Vindication Design

## Problem

The current slashing mechanism (`WrongVerificationSlashBps`) punishes minority voters regardless of correctness. When a challenge later proves the minority was right, they receive nothing — already slashed. This creates a game-theoretic trap: the rational strategy is to vote with the expected majority, not to evaluate claims honestly.

## Solution: Challenge-Based Retroactive Vindication

When a verified fact is later disproven via challenge, minority voters who were slashed for voting against the (now-disproven) majority receive their stake back plus a bonus from the majority's vindication slash.

## Architecture Decision

Vindication logic lives in a dedicated `keeper/vindication.go` file within the knowledge module. Same module boundary, logically separated code. Not a separate module — vindication is deeply coupled to round completion and challenge resolution.

## Data Model

### Types (`x/knowledge/types/vindication.go`)

```go
type VindicationEntry struct {
    Verifier    string // validator address
    Vote        string // their minority vote
    SlashAmount uint64 // tokens slashed (escrowed)
    SlashBps    uint64 // BPS rate applied
    RoundId     string // verification round ID
    FactId      string // the verified fact
    Height      uint64 // block height when slashed
}

type VindicationRecord struct {
    Verifier     string // vindicated validator
    FactId       string // the disproven fact
    RefundAmount uint64 // escrowed tokens returned
    BonusAmount  uint64 // bonus from majority slash pool
    VindicatedAt uint64 // block height
    DisprovenBy  string // fact that disproved the original
    RoundId      string // original verification round
}
```

### Store Prefixes (`keys.go`)

- `0x50` — `VindicationPendingPrefix`: `{factId} → []VindicationEntry`
- `0x51` — `VindicationRecordPrefix`: `{factId}/{verifier} → VindicationRecord`

### Module Account

`vindication_escrow` — holds slashed minority tokens until vindication fires or the window expires.

## Parameters

| Param | Type | Default | Purpose |
|-------|------|---------|---------|
| `VindicationRefundEnabled` | bool | true | Master switch |
| `VindicationBonusBps` | uint64 | 2000 (20%) | % of majority slash pool as bonus to minority |
| `VindicationSlashBps` | uint64 | 500 (5%) | Slash rate for majority on disproven fact |
| `VindicationWindowBlocks` | uint64 | 100000 | Eligibility window for vindication |

## Slash Routing for Escrow

### New Interface Method

Add `SlashValidatorToModule(ctx, addr, slashBps, destModule) (sdkmath.Int, error)` to:
- `StakingKeeper` interface in `x/knowledge/types/expected_keepers.go`
- `StakingKeeperAdapter` in `x/staking/keeper/knowledge_adapters.go`

Same escalation/SSI logic as `SlashValidator`, parameterized destination, returns actual slashed amount.

### Routing Rules

- **Wrong-vote minority slashes** (vindication-eligible): `SlashValidatorToModule(ctx, addr, bps, "vindication_escrow")`
- **Missed-reveal / equivocation** (not vindication-eligible): existing `SlashValidator` → `development_fund`

## DISPROVEN Transition

When a challenge round completes with verdict ACCEPT:

1. Get challenged fact via challenge claim's `ProvisionalFactId`
2. Contradiction check: same domain AND `ProvisionalFactId` link exists
3. If pass: transition original fact → `DISPROVEN`, set `DisprovenBy` field
4. Fire vindication

V1 contradiction detection relies on the challenge mechanism's explicit link (ProvisionalFactId). Semantic contradiction detection deferred to oracle integration.

## Vindication Execution

Triggered when fact status transitions to DISPROVEN:

1. Get `VindicationPending` entries for the fact
2. If empty, done
3. Identify majority voters from the original round
4. Slash each majority voter at `VindicationSlashBps` via `SlashValidatorToModule` → tokens to knowledge module
5. Calculate `bonus_pool = total_majority_slash * VindicationBonusBps / 10000`
6. Send `remainder = total_majority_slash - bonus_pool` to protocol treasury
7. For each `VindicationEntry`:
   - Refund `SlashAmount` from `vindication_escrow` to validator
   - Distribute proportional share of `bonus_pool` (weighted by original stake)
   - Create `VindicationRecord`
8. Emit `vindication_executed` event
9. Delete `VindicationPending` for this fact

## Pruning

Every 1000 blocks in BeginBlocker:

1. Iterate `VindicationPending` entries
2. For entries where `(currentHeight - Height) > VindicationWindowBlocks`:
   - Transfer escrowed tokens from `vindication_escrow` to protocol treasury
   - Delete the entry

## CLI Queries

- `zeroned query knowledge vindication-pending [fact-id]`
- `zeroned query knowledge vindication-record [fact-id]`
- `zeroned query knowledge vindication-stats`

## Testing

1. Escrow on minority slash: round completes → minority slashed → tokens escrowed → pending entry exists
2. Vindication on challenge success: challenge accepted → fact DISPROVEN → minority refunded + bonus
3. No vindication on challenge failure: challenge rejected → no vindication
4. Majority slashed on vindication at VindicationSlashBps
5. Proportional bonus: multiple minority voters get bonus by original stake weight
6. Zero-sum accounting: refund + bonus = escrow + majority_slash (exact, no leakage)
7. Pruning: expired entries cleaned, tokens to treasury
8. Window boundary: entry at exactly window NOT pruned; at +1 IS pruned
9. Disabled param: VindicationRefundEnabled=false → no escrow, normal slash to development_fund

## Files to Create/Modify

### New Files
- `x/knowledge/types/vindication.go` — VindicationEntry, VindicationRecord types
- `x/knowledge/keeper/vindication.go` — store methods + ExecuteVindication + pruning
- `x/knowledge/keeper/vindication_test.go` — full test suite

### Modified Files
- `x/knowledge/types/keys.go` — new store prefixes 0x50, 0x51
- `x/knowledge/types/params.go` — four new params
- `x/knowledge/types/expected_keepers.go` — SlashValidatorToModule on StakingKeeper
- `x/staking/keeper/knowledge_adapters.go` — implement SlashValidatorToModule
- `x/knowledge/keeper/confidence.go` — route minority slashes to escrow
- `x/knowledge/keeper/rounds.go` — record VindicationPending on round completion, DISPROVEN transition on challenge success
- `x/knowledge/keeper/phases.go` — pruning in BeginBlocker
- `x/knowledge/client/cli/query.go` — three new query commands
- `x/knowledge/module.go` — register vindication_escrow module account (if needed)
