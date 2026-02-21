# R7-7 — Adaptive Layer Integration Tests

## Goal

Comprehensive integration tests for all R7 modules, verifying cross-module wiring works
end-to-end. Also verify the IBC proto fix from R7-1 holds under full app boot.

## Working Directory

`/Users/yuai/Desktop/Zerone`

## Prerequisites

All R7-1 through R7-6 must be complete and merged. All individual module tests passing.

## Test Scenarios

### 1. Autopoiesis ↔ Vesting Rewards
- Autopoiesis multiplier for `rewards.block` adjusts → vesting rewards module reads
  the multiplier and adjusts block reward distribution accordingly
- Test: set multiplier to 200000 (20%), verify block rewards reduced proportionally

### 2. Alignment → Autopoiesis Corrections
- Alignment detects low knowledge verification rate → generates correction signal →
  autopoiesis adjusts `rewards.block` multiplier upward
- Test: mock low verification rate, run alignment EndBlocker, verify autopoiesis
  received adjustment suggestion

### 3. Research → Knowledge Integration
- Submit research targeting a specific fact → research accepted → fact confidence increases
- Test: create fact, submit research, reviews approve, verify fact updated

### 4. Tree → Toolbox Revenue Flow
- Tool registered via toolbox → tool is backed by a tree project → revenue from tool call
  flows through tree revenue routing to contributors
- Test: create project with 2 contributors, register tool, simulate tool call revenue,
  verify split reaches both contributors

### 5. Evidence → Disputes Chain
- Submit evidence → challenge evidence → dispute created → dispute resolved →
  evidence status updated
- Test: full lifecycle from evidence submission to dispute resolution

### 6. Claiming Pot → Staking Eligibility
- Create claiming pot with tier-2 requirement → tier-1 agent cannot claim →
  tier-2 agent claims successfully
- Test: two agents at different tiers, verify eligibility enforcement

### 7. Full Adaptive Loop
- Start with low knowledge verification → alignment detects → correction signal →
  autopoiesis increases reward multiplier → vesting rewards increase →
  (simulate) more verification activity → alignment detects improvement →
  correction signal reduces → system stabilizes
- Test: multi-epoch simulation (3+ epochs) showing the feedback loop

### 8. App Boot Smoke Test
- Full app instantiation with ALL modules (including fixed IBC modules) →
  genesis init → export → reimport → verify no panics
- This validates the R7-1 proto fix under real app conditions

### 9. Cross-Stack Genesis Round-Trip
- All 33+ modules: DefaultGenesis → ValidateGenesis → InitGenesis → ExportGenesis → reimport
- Verify exported genesis matches imported genesis for every module

### 10. Emergency Halt Stops Adaptive Layer
- Emergency halt triggered → autopoiesis stops adjusting → alignment stops observing →
  research submissions rejected → resume → everything resumes
- Test: halt, verify freeze, resume, verify unfreeze

## Implementation

### Test Harness
Extend the existing `tests/cross_stack/harness_test.go` with new keepers:
- Add autopoiesis, alignment, research, tree, evidence_mgmt, claiming_pot keepers
- Wire all cross-module interfaces
- The harness should instantiate the full app (like `app_test.go`) to catch registration issues

### File Structure
```
tests/cross_stack/
├── harness_test.go          (extend with new keepers)
├── adaptive_test.go         (scenarios 1, 2, 7, 10)
├── research_evidence_test.go (scenarios 3, 5)
├── tree_revenue_test.go     (scenario 4)
├── claiming_test.go         (scenario 6)
└── genesis_roundtrip_test.go (scenarios 8, 9)
```

## Exit Criteria

- All 10 scenarios pass
- `go test ./...` — ALL packages pass (0 failures)
- `go test ./app/...` — passes (no proto panics)
- `go test ./tests/cross_stack/...` — passes
- Total test packages: 33+ (up from 27)

## Constraints

- Use the same test harness pattern as existing cross_stack tests
- Mock external keepers where direct wiring would create circular deps
- Each test must be self-contained (no shared mutable state between tests)
- Tests must complete in <30s total
