# R28-7 Rubedo Alignment Design

## Context

The alignment module is structurally complete — sensors, scoring, corrections, health categorization, EndBlocker, CLI queries all exist. Keepers are wired via adapters. But several pieces are inert:

1. Knowledge adapter `GetVerificationRate()` returns hardcoded 500,000 (neutral)
2. Corrections flow through autopoiesis but have no magnitude bounds
3. Health transitions emit no events and trigger no behavioral changes
4. No history query for health over time

## Design

### 1. Fix Stub Adapters

Knowledge adapter `GetVerificationRate()` currently returns hardcoded neutral. Replace with real computation from claim data: `verified_claims / total_claims * BPS`. If total is 0, return NeutralBPS.

### 2. Bounded Correction Application

Add `MaxAutoApplyMagnitudeBps` to alignment params (default: 500 = 5%).

In `ApplyCorrections`, before calling `autopoiesisKeeper.SuggestAdjustment()`:
- If `correction.Magnitude <= MaxAutoApplyMagnitudeBps` → proceed with SuggestAdjustment as today
- If `correction.Magnitude > MaxAutoApplyMagnitudeBps` → emit `correction_governance_required` event, set `Applied=false`, skip autopoiesis call

This layers bounds checking on the existing autopoiesis delegation without replacing it.

### 3. Health Transition Responses (Conservative)

Add `DegradedFrequencyActive bool` and `PreviousCategory string` to `AlignmentState`.

In EndBlocker, after computing health category, compare with `PreviousCategory`:

- **Healthy -> Degraded**: Set `DegradedFrequencyActive = true`, emit `network_health_degraded`
- **Any -> Critical**: Emit `network_health_critical` (no halt, no bounds override)
- **Degraded/Critical -> Healthy**: Set `DegradedFrequencyActive = false`, emit `network_health_recovered`

Effective observation interval = `params.ObservationIntervalBlocks / 2` when `DegradedFrequencyActive` is true, otherwise `params.ObservationIntervalBlocks`. This means param changes via governance auto-propagate correctly — no stale absolute overrides.

### 4. History Query

Add `query alignment history [limit]` — iterates health index store keys in reverse order, returns up to `limit` entries (default 20, max 100). Internal iteration capped at 10,000 keys to prevent store walks.

### 5. Tests

- **Sensor tests**: Each sensor returns real values with wired keeper, NeutralBPS with nil keeper
- **Bounded corrections**: Small magnitude -> auto-applied via autopoiesis. Large magnitude -> governance event, not applied
- **Health transitions**: Degraded -> frequency doubles. Recovery -> frequency normal. Critical -> event emitted
- **Integration**: EndBlocker produces real observations when keepers are wired

## Files to Modify

- `x/knowledge/keeper/alignment_adapters.go` — Real GetVerificationRate
- `x/alignment/types/genesis.go` — Add MaxAutoApplyMagnitudeBps param
- `x/alignment/keeper/corrections.go` — Bounds check in ApplyCorrections
- `x/alignment/module.go` — Health transitions, frequency override in EndBlocker
- `x/alignment/keeper/state.go` — DegradedFrequencyActive and PreviousCategory in state
- `x/alignment/client/cli/query.go` — History query
- `x/alignment/keeper/grpc_query.go` — History query handler
- Proto files if state/params structures need updating
- Test files for all changes
