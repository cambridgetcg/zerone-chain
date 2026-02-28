# R30-2 — Parameter Governance Safety

## Problem

R28 and R29 added ~30 new governance-mutable parameters across knowledge, alignment, capture_defense, and partnerships. Each parameter can be changed via `MsgUpdateParams` governance proposals. But:

1. **No bounds checking on parameter interactions.** Individual params validate in isolation, but combinations can be dangerous — e.g., setting `OvercrowdingDecayMultiplierBps` to 10M while `MetabolismInitialEnergy` is 1 would cause instant fact death.

2. **No migration defaults.** When a chain upgrades and new params appear, the proto default (zero) is used unless an explicit migration sets them. Zero triggers validation failures, as we saw with R29-4's correction confidence params.

3. **Missing `RequiredStakeBps` rename.** The governance module has a known misnomer (TODO in code): `RequiredStakeBps` actually stores raw `uzrn`, not BPS.

## Objective

1. Add cross-parameter validation to modules with interdependent params
2. Create a migration template that sets sensible defaults for new params
3. Fix the governance `RequiredStakeBps` misnomer
4. Add a cross-stack test that governance-updates every param to boundary values

## Tasks

### Task 1: Cross-Parameter Validation

In `knowledge/types/genesis.go` `Validate()`, add relational checks:

```go
// Carrying capacity: decay multiplier must not cause instant death
if p.OvercrowdingDecayMultiplierBps > 0 && p.MetabolismInitialEnergy > 0 {
    // At 2x capacity, one decay cycle should not drain more than 50% of initial energy
    maxDecayPerCycle := p.MetabolismInitialEnergy * p.OvercrowdingDecayMultiplierBps / BPS
    if maxDecayPerCycle > p.MetabolismInitialEnergy / 2 {
        return fmt.Errorf("overcrowding_decay_multiplier too aggressive: would drain %d of %d initial energy", 
            maxDecayPerCycle, p.MetabolismInitialEnergy)
    }
}

// Epistemic temperature: cold cap must be < default cap
if p.EpistemicColdConfidenceCapBps >= p.MaxSurvivalConfidence {
    return fmt.Errorf("epistemic_cold_confidence_cap (%d) must be < max_survival_confidence (%d)",
        p.EpistemicColdConfidenceCapBps, p.MaxSurvivalConfidence)
}

// Role elasticity: min < max (already checked), but also min * max_bonus shouldn't exceed BPS
if p.RoleElasticityMaxMultiplierBps * p.AgentVerificationVoteWeightBonusBps / BPS > BPS {
    return fmt.Errorf("role_elasticity_max_multiplier * agent_bonus would exceed 100%% vote weight")
}
```

In `alignment/types/genesis.go`:
```go
// Correction bounds: min multiplier applied to max_magnitude shouldn't exceed BPS
if p.CorrectionBoundsMaxMultiplierBps * p.MaxAutoApplyMagnitudeBps / BPS > BPS {
    return fmt.Errorf("max correction bounds multiplier * max_magnitude would exceed 100%%")
}
```

### Task 2: Migration Template

Create `x/knowledge/migrations/v3/migrate.go` (template for future use):

```go
func Migrate(ctx sdk.Context, keeper knowledge.Keeper) error {
    params := keeper.GetParams(ctx)
    
    // Set defaults for new params that would be zero from proto
    if params.DomainBaseCapacity == 0 {
        params.DomainBaseCapacity = 1000
    }
    if params.EpistemicTemperatureDecayBps == 0 {
        params.EpistemicTemperatureDecayBps = 995_000
    }
    // ... etc for all R29 params
    
    return keeper.SetParams(ctx, &params)
}
```

Register in module.go's `RegisterMigration`.

### Task 3: Fix Governance Misnomer

Rename `RequiredStakeBps` → `RequiredStakeUzrn` in governance params:
- Update proto field name (keep same field number for wire compat)
- Update genesis.go defaults and validation
- Update all references in keeper
- Add migration from v1 → v2 for the field rename

### Task 4: Boundary Governance Test

Create `tests/cross_stack/param_boundaries_test.go`:

```go
func TestGovernanceParamBoundaries(t *testing.T) {
    // For each module's params, attempt to set every uint64 field to:
    // - 0 (should fail validation for most)
    // - max uint64 (should fail validation for all BPS fields)
    // - 1 (edge case)
    // - BPS (1,000,000 — at-limit for BPS fields)
    // Verify that Validate() catches all dangerous values
}

func TestGovernanceParamInteractions(t *testing.T) {
    // Set valid but adversarial combinations:
    // - Max decay + min initial energy
    // - Max multipliers on all bonuses simultaneously
    // - Zero thresholds with non-zero intervals
    // Verify cross-param validation catches these
}
```

## Tests

1. Cross-parameter validation catches dangerous combinations
2. Migration template correctly sets defaults for zero-valued new params
3. Governance misnomer rename is backward-compatible (same field number)
4. Boundary tests cover every param in knowledge, alignment, capture_defense, partnerships

## Success Criteria

- No parameter combination that passes `Validate()` can crash the chain or cause instant state corruption
- Chain upgrade from pre-R29 → post-R29 correctly initialises all new params
- `RequiredStakeBps` → `RequiredStakeUzrn` rename complete
