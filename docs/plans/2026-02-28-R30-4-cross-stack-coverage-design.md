# R30-4: Cross-Stack Integration Coverage Design

## Overview

Add cross-stack integration tests that exercise multi-feature interactions when R28/R29 features fire together in the same block sequence.

## Approach

**Direct state injection** via keeper APIs, matching the existing `adaptive_test.go` pattern. No message routing needed — we seed state directly, advance blocks through BeginBlocker/EndBlocker, and verify cross-module effects.

## Changes Required

### 1. Extend TestHarness (`tests/cross_stack/harness_test.go`)

Add fields + imports for:
- `CaptureDefenseKeeper` (zeronecdkeeper.Keeper)
- `PartnershipsKeeper` (zeronepartnershipskeeper.Keeper)
- `DiscoveryKeeper` (zeronediscoverykeeper.Keeper)

Wire from `app.CaptureDefenseKeeper`, `app.PartnershipsKeeper`, `app.DiscoveryKeeper`.

### 2. New test file: `tests/cross_stack/r29_integration_test.go`

**Test 1: TestR29_FullEcosystemCycle** — 10-step sequence:
1. Populate domain past carrying capacity via SetDomainStats
2. Verify epistemic temperature starts neutral via GetOrInitDomainEpistemicState
3. Set conformity diversity records → run UpdateEpistemicTemperature → verify cooling
4. Inject vindication records → run UpdateEpistemicTemperature → verify heating
5. Set DomainRoleRecord with vindication impact → verify GetRoleElasticity changes
6. Enable alignment, force low scores → verify corrections generated
7. Apply corrections, record outcomes → verify confidence tracking
8. Force critical health → verify pacing multipliers change (500k/2M)
9. Flag domain for capture → verify partnership formation bonus set
10. Recover health → verify pacing normalises to 1M/1M

**Test 2: TestR29_AdversarialInteractions** — Two sub-tests:
- Pathological cold: max capacity + cold temp + zero role data → no panics, slow confidence growth
- Total failure: all corrections fail + critical pacing → restricted confidence, stable but slow

**Test 3: TestBlockerOrdering_NoPanic** — 1000 blocks with randomized state:
- Random domain stats, epistemic temperatures, role records, capture metrics
- Assert no panics throughout

**Test 4: TestGenesisExportImport_WithR29State** — Round-trip:
- Boot, inject R29 state, advance 100 blocks, export, reimport, verify preserved, advance 10 more

## Key Module APIs Used

| Module | Key APIs |
|--------|----------|
| Knowledge | SetDomainStats, SetDomainEpistemicState, SetDomainRoleRecord, UpdateEpistemicTemperature, GetRoleElasticity |
| Alignment | SetState, SetParams, ObserveAll, ComputeScores, GenerateCorrections, ApplyCorrections, GetCorrectionConfidence, GetGlobalPacingMultiplier |
| CaptureDefense | SetCaptureMetrics, OnDomainFlagged, CalculateAdjustedHHI, IsDomainFlagged |
| Partnerships | GetDomainFormationBonus, GetDomainPartnershipDensity |
| Autopoiesis | SetState, SetMultiplierState, GetMultiplier |

## Success Criteria

- All four tests pass
- No panics across 1000-block stress test
- Genesis round-trip preserves all R29 state
- Combined R29 features produce coherent emergent behavior
