# R14 — Chain Binary + Test Parity: Make It Run

**Goal:** Zerone boots, produces blocks, and has test coverage parity with the prototype for all critical modules. After R14, `zeroned start` works and the chain is testnet-ready.

## The Problem

R1-R13 built the entire protocol — 32 modules, 1992 tests, full app wiring, genesis pipeline. But:

1. **No binary** — `cmd/zeroned/` doesn't exist. `make build` fails. The chain cannot start.
2. **~1,200 test gap** — zerone has 1,992 tests vs legible-money's 3,206. Critical modules under-tested:
   - `bvm` — 72 vs 438 (gap: 366) — the VM is the highest-risk module
   - `knowledge` — 304 vs 482 (gap: 178) — PoT core
   - `toolbox` — 128 vs 268 (gap: 140) — platform module
   - `autopoiesis` — 7 vs 121 (gap: 114) — self-regulation barely tested
3. **IBC tests hollow** — 4 test files, 705 lines, but only 1 actual `func Test`. Tests are structured but the test functions aren't wired.
4. **E2E verify blocked** — R13-6 can't run without the binary.

## Sessions (6)

| # | File | Scope | Parallelism |
|---|------|-------|-------------|
| R14-1 | R14-1-cmd-zeroned.md | Create `cmd/zeroned/` — root command, config, genesis helpers, CLI | Wave 1 |
| R14-2 | R14-2-bvm-tests.md | Port BVM test coverage from prototype (366 test gap) | Wave 1 |
| R14-3 | R14-3-knowledge-toolbox-tests.md | Port knowledge (178 gap) + toolbox (140 gap) tests | Wave 1 |
| R14-4 | R14-4-autopoiesis-alignment-tests.md | Port autopoiesis (114 gap) + alignment (39 gap) + evidence_mgmt (58 gap) tests | Wave 1 |
| R14-5 | R14-5-ibc-integration-tests.md | Wire IBC test functions + port remaining module test gaps (channels, disputes, staking, etc.) | Wave 2 (after Wave 1) |
| R14-6 | R14-6-boot-verify.md | End-to-end: build binary → init → start → produce blocks → smoke test → full verify | Wave 3 (after all) |

## Run Order

- **Wave 1 (parallel):** R14-1, R14-2, R14-3, R14-4
- **Wave 2:** R14-5 (can start in parallel but merges after Wave 1)
- **Wave 3:** R14-6 (depends on all — this is the go/no-go gate)

## Exit Criteria

1. `make build` succeeds — produces `build/zeroned` binary
2. `zeroned init` works — generates valid genesis
3. `zeroned start` boots a single-validator chain and produces blocks
4. `go test ./...` — all pass, zero failures
5. Test count ≥ 2,800 (closing ~80% of the gap)
6. BVM tests ≥ 350 (critical: VM is highest risk)
7. Knowledge tests ≥ 420
8. Autopoiesis tests ≥ 80
9. IBC tests have real `func Test` entries (transfer, ratelimit, timeout)
10. `make pr-check` (lint + test + build) passes clean

## Design Standards

- `cmd/zeroned/` follows Cosmos SDK v0.50 patterns (see simapp or any v0.50 chain)
- Test ports adapt to zerone naming (uzrn, zeroned, zerone-testnet-1) — not copy-paste
- New tests follow existing patterns in each module's `keeper/*_test.go`
- All tests use the unified 1,000,000 BPS scale
- Conventional commits per session
