# R21 — Testnet Launch Gate

**Goal:** Verify everything works together. Every module is built, every test passes individually. R21 is about the *assembled machine* — does the chain boot, run PoT rounds across validators, handle governance, survive upgrades, and pass invariants?

## The Problem

Zerone has 32 modules, 290K LOC, 13,500+ tests, all passing. The binary builds and boots. But:

1. **Multi-validator PoT unverified** — `scripts/localnet.sh` exists, `scripts/localnet-test.sh` has 8 test scenarios including `test_pot_round`. Neither has been run against the current binary. PoT commit/reveal/verdict across 4 validators is the single highest-risk item.

2. **No genesis invariant checker** — Revenue splits, founder immutability, research fund multisig, bootstrap fund, demand tracking, axiom seeds — all wired individually. No tool validates they're assembled correctly in genesis.json.

3. **IBC untested on live chain** — IBC module tests exist in isolation. No localnet test verifies an actual cross-chain transfer works.

4. **Upgrade path untested** — `app/upgrades.go` has `v1.0.0-testnet` handler, migration framework exists (`x/*/migrations/v2`). But no test simulates a governance-triggered upgrade with state migration.

5. **No dress rehearsal** — The full pipeline (build → genesis ceremony → axiom injection → configure → boot → 100 blocks → PoT round → governance → bank transfer → shutdown) hasn't run as a single unbroken sequence.

## What Already Exists

| Component | Status |
|-----------|--------|
| `scripts/localnet.sh` | 563 lines, 4-validator setup, start/stop/status/logs/clean |
| `scripts/localnet-test.sh` | 665 lines, 8 tests (block_production, validator_set, delegation, tier_check, pot_round, slashing, recovery, governance) |
| `tools/axiom-loader/` | Genesis axiom injection tool |
| `seeds.txt` | Axiom seed data |
| `app/upgrades.go` | v1.0.0-testnet handler + store upgrades |
| `x/*/migrations/v2/` | Per-module v1→v2 migrations (alignment, auth, autopoiesis, billing+) |

## Sessions (5)

| # | File | Scope | Parallelism |
|---|------|-------|-------------|
| R21-1 | R21-1-localnet-verification.md | Run localnet.sh + localnet-test.sh, fix anything that breaks | Wave 1 |
| R21-2 | R21-2-genesis-invariants.md | Build genesis invariant checker tool | Wave 1 |
| R21-3 | R21-3-ibc-smoke-test.md | Add IBC transfer test to localnet | Wave 2 (after R21-1) |
| R21-4 | R21-4-upgrade-simulation.md | Simulate governance-triggered upgrade with migrations | Wave 2 (after R21-1) |
| R21-5 | R21-5-dress-rehearsal.md | Full launch pipeline end-to-end | Wave 3 (after all) |

## Run Order

- **Wave 1 (parallel):** R21-1, R21-2
- **Wave 2 (parallel, after R21-1):** R21-3, R21-4
- **Wave 3 (sequential, after all):** R21-5

## Exit Criteria

1. `scripts/localnet.sh start` boots 4 validators, all producing blocks
2. All 8 `localnet-test.sh` scenarios pass (including `test_pot_round`)
3. Genesis invariant checker passes on both localnet and production genesis configs
4. IBC transfer completes on localnet (send + receive + balance verified)
5. Governance-triggered upgrade completes with state migration verified
6. Dress rehearsal passes end-to-end (full pipeline, single run, no manual intervention)
7. `go test ./...` — zero failures
8. `make pr-check` clean

## After R21

Public testnet. External validators. Vault integration E2E. Documentation for operators.
