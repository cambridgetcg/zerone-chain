# R30-4 — Cross-Stack Integration Coverage

## Problem

R28 and R29 added complex cross-module interactions:
- Knowledge → capture_defense (verification history feed)
- Capture_challenge → knowledge (challenge resolution effects)
- Capture_defense → partnerships (structural immunity)
- Alignment → all modules (global pacing)
- Knowledge internal (carrying capacity, epistemic temperature, role elasticity)

Each feature has unit tests and per-feature integration tests. But the **combined behavior** — what happens when carrying capacity pressure, epistemic temperature, role elasticity, pacing changes, and capture defense all fire in the same block — is untested.

## Objective

Add cross-stack integration tests that exercise multi-feature interactions within a single test app, verifying emergent behavior when R28/R29 features interact.

## Tasks

### Task 1: Multi-Feature Stress Test

`tests/cross_stack/r29_integration_test.go`:

```go
func TestR29_FullEcosystemCycle(t *testing.T) {
    // Boot a test app with all R28/R29 features active
    // 1. Populate a domain past carrying capacity (R29-1)
    // 2. Verify epistemic temperature starts neutral (R29-2)
    // 3. Run verification rounds with conformity → temperature cools (R29-2)
    // 4. Execute a vindication → temperature heats (R29-2)
    // 5. Check role elasticity updated from vindication (R29-3)
    // 6. Verify alignment observes and generates corrections (R28-7, R29-4)
    // 7. Apply corrections → record outcomes → check confidence (R29-4)
    // 8. Degrade health → verify pacing changes across modules (R29-6)
    // 9. Flag domain for capture → verify partnership formation bonus (R29-5)
    // 10. Recovery → verify pacing normalises
    //
    // This is the Tàijí breathing: the whole system responds to each perturbation
}
```

### Task 2: Adversarial Interaction Test

```go
func TestR29_AdversarialInteractions(t *testing.T) {
    // Test pathological interactions:
    // - Domain at max capacity + cold epistemic temperature + low role elasticity
    //   → Facts should decay fast, confidence should grow slowly, bonuses should be minimal
    //   → System should NOT panic or produce invalid state
    
    // - All corrections fail + health critical + pacing at max defensive
    //   → Correction confidence drops to zero → governance lockout
    //   → All modules at max cooldown → system is slow but stable
    
    // - Vindication in overcrowded domain with capture flag
    //   → Temperature heats (R29-2) + role record updates (R29-3)
    //   → Carrying capacity may still force decay (R29-1)
    //   → Capture defense may reduce HHI from partnerships (R29-5)
}
```

### Task 3: BeginBlocker/EndBlocker Ordering Test

```go
func TestBlockerOrdering_NoPanic(t *testing.T) {
    // Verify that the module execution order in app.go
    // (BeginBlocker and EndBlocker sequences) doesn't cause
    // nil pointer dereferences when modules read state that
    // another module hasn't yet written in this block.
    //
    // Run 1000 blocks with randomised initial state.
    // Assert no panics.
}
```

### Task 4: Genesis Export/Import Round-Trip

```go
func TestGenesisExportImport_WithR29State(t *testing.T) {
    // 1. Boot app, run 100 blocks to accumulate R29 state
    //    (domain stats, epistemic states, role records, correction outcomes)
    // 2. Export genesis
    // 3. Import into fresh app
    // 4. Verify all R29 state survived the round-trip
    // 5. Run 10 more blocks, verify behaviour is identical
}
```

## Tests

Each task IS a test. Success = all pass, no panics, no invalid state after multi-feature interactions.

## Success Criteria

- Full ecosystem cycle test exercises all six R29 polarities in sequence
- Adversarial combinations don't cause panics or state corruption
- BeginBlocker/EndBlocker ordering is safe for 1000 blocks
- Genesis export/import preserves all R29 state
