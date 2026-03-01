# R14-5 — Wire IBC Tests + Port Remaining Module Test Gaps

## Context

### IBC Tests — Structured but Empty

The `tests/ibc/` directory has 4 test files totalling 705 lines, but only 1 actual `func Test`:

| File | Lines | Test Funcs |
|------|-------|------------|
| setup_test.go | 90 | 1 |
| transfer_test.go | 166 | 0 |
| ratelimit_test.go | 310 | 0 |
| timeout_test.go | 139 | 0 |

The test infrastructure is built — helpers, mock chains, relay setup — but the actual test functions aren't wired. This is likely because the tests were scaffolded waiting for a working binary.

### Remaining Module Gaps (10-50 each)

| Module | Gap | Priority |
|--------|-----|----------|
| ibcratelimit | 49 | High — IBC security |
| channels | 51 | High — payment channels |
| staking | 42 | High — validator lifecycle |
| research | 37 | Medium — treasury management |
| disputes | 36 | Medium — resolution logic |
| partnerships | 33 | Medium — collaboration |
| tree | 32 | Medium — service registry |
| claiming_pot | 28 | Low |
| gov | 26 | Medium — LIP lifecycle |
| emergency | 23 | Medium — kill switch |
| discovery | 17 | Low |
| icaauth | 15 | Low |

## Task

### Part 1: Wire IBC Test Functions

Complete the existing test infrastructure in `tests/ibc/`:

1. **transfer_test.go** — wrap existing test logic in `func TestIBCTransfer`, `func TestIBCTransferDenomTrace`, etc.
2. **ratelimit_test.go** — wire rate limit tests: `func TestRateLimitExceeded`, `func TestRateLimitReset`, `func TestRateLimitPerChannel`
3. **timeout_test.go** — wire timeout tests: `func TestIBCTimeout`, `func TestTimeoutRefund`, `func TestTimeoutOnClose`

### Part 2: Port High-Priority Module Tests

Focus on modules with gaps > 30 that are security-critical:

1. **ibcratelimit** (49 gap) — rate calculations, per-channel limits, epoch resets, quota overflow
2. **channels** (51 gap) — payment channel open/close, state updates, dispute resolution, cooperative close
3. **staking** (42 gap) — tier transitions (Apprentice→Verified→Bonded→Guardian), delegation, slashing, reputation
4. **disputes** (36 gap) — dispute creation, evidence submission, voting, resolution, tier-specific configs

### Part 3: Port Medium-Priority Module Tests (if time)

5. **research** (37 gap) — proposals, funding, treasury management
6. **partnerships** (33 gap) — matching, anti-coercion, deliberation
7. **tree** (32 gap) — service registry, revenue routing, evidence tax
8. **gov** (26 gap) — LIP lifecycle edge cases, param changes, upgrade proposals
9. **emergency** (23 gap) — halt/resume, guardian council, cascade effects

## Target

- IBC: ≥ 10 real test functions across transfer/ratelimit/timeout
- ibcratelimit module: ≥ 50 tests
- channels: ≥ 75 tests
- staking: ≥ 95 tests
- disputes: ≥ 65 tests
- Overall test count contribution: ~200+ new tests

## Verification

```bash
go test ./tests/ibc/... -count=1 -v
go test ./x/ibcratelimit/... -count=1 -v
go test ./x/channels/... -count=1 -v
go test ./x/staking/... -count=1 -v
go test ./x/disputes/... -count=1 -v
go test ./... 2>&1 | grep "^FAIL"  # should be empty
```

## Commit Convention

```
test(R14-5): wire IBC transfer + ratelimit + timeout tests
test(R14-5): port ibcratelimit + channels test coverage
test(R14-5): port staking tier transition + slashing tests
test(R14-5): port disputes + remaining module gaps
```
