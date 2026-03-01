# R28-5 Citrinitas: Role Differentiation Design

## Decisions

- **Claim type mapping:** `OBSERVATION` = empirical (existing), `COMPUTATIONAL` = new type (7)
- **Bonus application point:** On Fact creation (not on Claim or in AnteHandler)
- **Architecture:** Module-local bonuses — each module queries ZeroneAuthKeeper and applies its own bonuses
- **Service stake discount:** Deferred — no stake requirement exists yet; param added for governance but not applied
- **BPS scale:** 1,000,000 = 100% (consistent with codebase); all bonus params use `*BonusBps` suffix to distinguish from thresholds

## Changes

### 1. Proto: New Claim Type

Add `CLAIM_TYPE_COMPUTATIONAL = 7` to `ClaimType` enum. Represents claims derived from computation, synthesis, or formal inference.

### 2. New Keeper Interface: ZeroneAuthKeeper

In `x/knowledge/types/expected_keepers.go`:

```go
type ZeroneAuthKeeper interface {
    GetAccount(ctx sdk.Context, address string) (*zeroneauthtypes.Account, bool)
}
```

Wired in `app/app.go` via `SetZeroneAuthKeeper()` (setter pattern, same as OntologyKeeper).

Also added to partnerships module for coercion signal handling.

### 3. Knowledge Module — New Params

| Param | Default | Meaning |
|-------|---------|---------|
| `HumanEmpiricalBonusBps` | 150,000 | +15% confidence for human empirical claims |
| `AgentComputationalBonusBps` | 150,000 | +15% confidence for agent computational claims |
| `AgentVerificationBonusBps` | 200,000 | +20% vote weight for agent verifiers |
| `HumanPatronageBonusBps` | 100,000 | +10% energy boost for human patrons |
| `DualValidationBonusBps` | 250,000 | +25% confidence for partnership claims |

### 4. Knowledge Module — Bonus Application Points

**4a. Claim Confidence Bonus** (`rounds.go:createFactFromClaim`):
- Lookup submitter account type via ZeroneAuthKeeper
- OBSERVATION + "human" → `confidence = confidence * (1M + HumanEmpiricalBonusBps) / 1M`
- COMPUTATIONAL + "agent" → `confidence = confidence * (1M + AgentComputationalBonusBps) / 1M`
- Still clamped by MaxConfidence and stratum ceiling after bonus

**4b. Vote Weight Bonus** (`confidence.go:AggregateVerificationResult`):
- Lookup voter account type
- "agent" → `stakeWeight = stakeWeight * (1M + AgentVerificationBonusBps) / 1M`

**4c. Patronage Energy Bonus** (`metabolism.go:ApplyPatronageEnergyBoost`):
- Lookup patron account type (from msg.Patron)
- "human" → `boost = boost * (1M + HumanPatronageBonusBps) / 1M`

**4d. Dual Validation Bonus** (`rounds.go:createFactFromClaim`):
- If claim has partnership ID → apply DualValidationBonusBps
- Stacks with claim type bonus (applied after)
- Clamped by MaxConfidence

### 5. Partnerships Module — Coercion Freeze Multiplier

New param: `HumanCoercionFreezeMultiplierBps` (default: 1,500,000 = 1.5x)

In `HandleCoercionSignal()`:
- Lookup signaler account type
- "human" → `freezeBlocks = freezeBlocks * HumanCoercionFreezeMultiplierBps / 1M`

Requires adding ZeroneAuthKeeper to partnerships keeper.

### 6. Service Stake Discount — Deferred

Param exists in governance but not applied until service deployment stake is introduced.

## Files Modified

- `x/knowledge/types/types.proto` — CLAIM_TYPE_COMPUTATIONAL
- `x/knowledge/types/expected_keepers.go` — ZeroneAuthKeeper interface
- `x/knowledge/types/genesis.pb.go` — New bonus params + defaults
- `x/knowledge/keeper/keeper.go` — SetZeroneAuthKeeper setter
- `x/knowledge/keeper/rounds.go` — Claim confidence + dual validation bonus
- `x/knowledge/keeper/confidence.go` — Agent vote weight bonus
- `x/knowledge/keeper/metabolism.go` — Human patronage bonus
- `x/knowledge/keeper/msg_server.go` — Validate COMPUTATIONAL claim type
- `x/partnerships/types/expected_keepers.go` — ZeroneAuthKeeper interface
- `x/partnerships/keeper/keeper.go` — SetZeroneAuthKeeper setter
- `x/partnerships/keeper/anti_coercion.go` — Human coercion freeze multiplier
- `app/app.go` — Wire ZeroneAuthKeeper to knowledge + partnerships

## Tests

- Human submits OBSERVATION claim → +15% confidence on resulting Fact
- Agent submits COMPUTATIONAL claim → +15% confidence on resulting Fact
- Human submits COMPUTATIONAL claim → no bonus (baseline)
- Agent submits OBSERVATION claim → no bonus (baseline)
- Agent verification vote → +20% weight in aggregation
- Human patronage → +10% energy boost
- Partnership claim → +25% dual validation bonus
- Bonuses clamped by MaxConfidence
- Bonuses configurable via params
