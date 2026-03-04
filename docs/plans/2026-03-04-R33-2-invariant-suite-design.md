# R33-2 Module Invariant Suite — Design

## Objective

Implement invariants for every key custom module using the SDK's InvariantRegistry interface. Invariants are mathematical properties that must always hold — if broken, the chain has a critical bug.

## Architecture

- Use existing `sdk.InvariantRegistry` + `sdk.Invariant` from Cosmos SDK v0.50
- Each target module gets `keeper/invariants.go` following the `x/auth/keeper/invariants.go` pattern
- Crisis module was removed in SDK v0.50 — create a CLI command for on-demand invariant checking
- Register invariants in each module's `RegisterInvariants(ir)` method

## Modules & Invariants

### Economic (vesting_rewards, zerone_staking)

**vesting_rewards:**
- `params-valid` — Stored params pass validation
- `schedule-consistency` — All active vesting schedules have non-zero amounts and valid addresses

**zerone_staking:**
- `params-valid`
- `delegation-validator-exists` — Every delegation references an existing validator
- `unbonding-consistency` — All unbonding entries have completion height > current height

### Knowledge

- `params-valid`
- `domain-count-consistency` — Domain.ActiveCount matches actual count of active facts
- `no-self-citation` — No fact cites itself via relations
- `round-integrity` — No claim ACCEPTED without completed verification round

### Governance (zerone_gov, emergency)

**zerone_gov:**
- `params-valid`
- `proposal-status-consistency` — No LIP with PASSED status that has zero votes

**emergency:**
- `params-valid`
- `ceremony-consistency` — Active ceremonies have valid state

### Partnerships

- `params-valid`
- `active-partnership-members` — Every active partnership has exactly 2 members
- `no-duplicate-partnerships` — No two partnerships share the same member pair

### Defense (capture_defense)

- `params-valid`
- `metric-bounds` — HerfindahlIndex ∈ [0, 1M], RiskScore ∈ [0, 1M]

### Alignment

- `params-valid`
- `sensor-bounds` — All sensor readings ∈ [0, 1M] BPS range

## CLI Command

`zeroned query invariants check` — Iterates all registered invariants, runs against current state, reports pass/fail with diagnostic output.

## Testing

Each invariant gets unit tests:
1. Valid state → invariant passes
2. Broken state → invariant detects violation

## Out of Scope

- Graph acyclicity (cycle detection) — expensive, deferred to simulation
- Carrying capacity checks — mathematical relationship already tested in unit tests
- Per-block invariant execution — no crisis module, CLI-only for now
- Remaining ~20 modules without spec-listed invariants
