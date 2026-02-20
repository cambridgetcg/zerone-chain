# R2-5 — Knowledge Tests: Full Test Suite

## Goal

Port all 309 knowledge module tests from the draft and add new tests for
security fixes. This is the most critical test suite in the chain — PoT
consensus correctness depends on it.

## Dependencies

- R2-2 (keeper) and R2-3 (ABCI) must be complete

## Working Directory

`/Users/yuai/Desktop/Zerone`

## Reference

- `/Users/yuai/Desktop/legible_money/x/knowledge/keeper/keeper_test.go` — main test file
- `/Users/yuai/Desktop/legible_money/x/knowledge/keeper/` — all test files (16 files)
- `/Users/yuai/Desktop/legible_money/x/knowledge/types/` — type tests
- `/Users/yuai/Desktop/legible_money/reports/batch-22/B22-3-audit-fixes.md` — slash param fixes

## Test Categories to Port

### Fact Lifecycle (~50 tests)
- Submit claim → verification → accept/reject
- Fact updates, contestation, citation
- Domain validation, category validation
- Reference integrity (can't cite non-existent facts)
- Confidence scoring after verification

### Verification Rounds (~80 tests)
- VRF verifier selection (deterministic, weighted by tier)
- Commit phase: valid commits, duplicate commits, wrong verifier
- Reveal phase: valid reveals, hash mismatch, late reveals
- Tally: majority accept, majority reject, split vote
- Round timeout: BeginBlocker advances stuck rounds
- Missing reveals: slashed for missed_reveal

### Slashing (~30 tests)
- Wrong vote slashing (non-zero, from params)
- Missed reveal slashing
- Equivocation slashing (double commit with different hashes)
- Slash amount matches params exactly
- Slash updates validator stake correctly

### Confidence Scoring (~40 tests)
- Initial confidence from category
- Boost per successful verification
- Fundamentality calculation (citation graph depth)
- Cross-reference bonus
- Confidence decay for old unverified facts
- Confidence threshold for promotion

### Domain Management (~30 tests)
- Create, update, deprecate domains
- Domain hierarchy (parent/child)
- Facts in deprecated domains
- Domain fact count tracking

### Crypto + VRF (~25 tests)
- VRF proof generation and verification
- Deterministic output (same input → same selection)
- Verifier exclusion (submitter can't verify own claim)
- Weighted selection by validator tier

### Extended Params (~20 tests)
- All 72+ extended params validate correctly
- Governance can update each param
- Invalid param values rejected

### ABCI Integration (~30 tests)
- Vote extension round-trip
- Extension aggregation in PrepareProposal
- Tamper detection in ProcessProposal
- Empty extensions for non-verifiers
- Multiple concurrent rounds

## New Tests (security fixes baked in)

| Test | Validates |
|------|-----------|
| `TestSlashParams_NonZeroInDefault` | DefaultParams has non-zero slash values |
| `TestSlashParams_CannotSetToZero` | UpdateParams rejects zero slash values |
| `TestProposerCannotTamperVerdict` | ProcessProposal rejects mismatched tally |
| `TestEquivocationDetection` | Different commits from same verifier → slash |
| `TestConcurrentRounds_NoInterference` | Multiple rounds don't corrupt each other |
| `TestVRF_SubmitterExcluded` | Claim submitter never selected as verifier |
| `TestConfidence_CategoryBaseline` | Axiomatic facts start higher than empirical |
| `TestReward_MatchesParamDecay` | Verification reward decays per epoch correctly |

## Test Organization

```
x/knowledge/keeper/
├── fact_test.go          # Fact lifecycle tests
├── round_test.go         # Verification round lifecycle
├── slash_test.go         # Slashing tests
├── confidence_test.go    # Confidence scoring
├── domain_test.go        # Domain management
├── vrf_test.go           # VRF + crypto tests
├── params_test.go        # Parameter validation
├── abci_test.go          # ABCI integration
├── query_test.go         # gRPC query tests
└── helpers_test.go       # Test helpers, mock setup
```

## Test Helpers

Create comprehensive helpers that will be reused by later modules:

```go
func setupKnowledgeTest(t *testing.T) (*Keeper, sdk.Context) {
    // Full keeper with mock dependencies
}

func submitAndVerifyClaim(t *testing.T, k *Keeper, ctx sdk.Context, content string) *types.Fact {
    // Helper: submit claim → start round → commit → reveal → tally → return fact
}

func advanceBlocks(ctx sdk.Context, n int) sdk.Context {
    // Advance block height by n
}
```

## Verification

```bash
go test ./x/knowledge/... -count=1 -v
go test ./x/knowledge/... -count=1 -race  # race detector
```

Target: **309+ tests, all passing.**

## Commit

```
test(knowledge): full test suite — 309+ tests covering PoT consensus, slashing, confidence
```

## Do NOT

- Skip the slashing tests (these were the P0 from B22-3)
- Use approximate assertions for economic values (exact match only)
- Skip the race detector run
- Create tests that depend on wall clock time (use block heights)
- Skip ABCI tests (proposer tampering was a P0)
