# R13 — Genesis Launch Pipeline: Ceremony, Configuration, Axiom Tools, Validation

**Goal:** Zerone has a complete, production-grade genesis launch pipeline — from axiom validation through coordinated multi-validator ceremony to node configuration presets. Every parameter explicitly set and documented. No reliance on `DefaultParams()` for production genesis.

## The Problem

The Legible Money prototype had a full 5-stage genesis pipeline:

1. **Axiom pipeline** — parse markdown → validate DAG → inject into genesis
2. **Genesis ceremony** — multi-step coordinated process for mainnet/testnet launch
3. **Testnet genesis** — every param for all 30+ modules explicitly configured with `jq`
4. **Node configuration** — mode presets (validator/fullnode/seed/archive) for production tuning
5. **Go-level validation** — DefaultGenesis + Validate per module, InitGenesis→ExportGenesis round-trip, 100-block production test

Zerone has:
- ✅ 777 axioms in JSON + basic `convert_axioms.go` parser
- ✅ `localnet.sh` (4-validator local testnet, partial param patching)
- ✅ `join-testnet.sh` (cosmovisor + systemd + state sync)
- ✅ Config templates (`config.toml.template`, `app.toml.template`)
- ✅ Partial genesis tests (`genesis_test.go`, `genesis_roundtrip_test.go`)
- ❌ No axiom-loader (validate DAG, inject into genesis, stats)
- ❌ No production genesis ceremony script
- ❌ No explicit param configuration for testnet/mainnet genesis (relies on defaults)
- ❌ No `testnet-genesis-config.json` parameter reference
- ❌ No `configure-node.sh` with mode presets
- ❌ No per-module genesis validation test (currently validates all-at-once only)

## Module Name Mapping (genesis keys)

| Directory | ModuleName (genesis key) |
|-----------|--------------------------|
| x/alignment | `alignment` |
| x/auth | `zerone_auth` |
| x/autopoiesis | `autopoiesis` |
| x/billing | `billing` |
| x/bvm | `bvm` |
| x/capture_challenge | `capture_challenge` |
| x/capture_defense | `capture_defense` |
| x/channels | `channels` |
| x/claiming_pot | `claiming_pot` |
| x/compute_pool | `compute_pool` |
| x/discovery | `discovery` |
| x/disputes | `disputes` |
| x/emergency | `emergency` |
| x/evidence_mgmt | `evidence_mgmt` |
| x/gov | `zerone_gov` |
| x/home | `home` |
| x/ibcratelimit | `ibcratelimit` |
| x/icaauth | `icaauth` |
| x/knowledge | `knowledge` |
| x/liquiditypool | `liquiditypool` |
| x/ontology | `ontology` |
| x/partnerships | `partnerships` |
| x/qualification | `qualification` |
| x/research | `research` |
| x/schedule | `schedule` |
| x/staking | `zerone_staking` |
| x/tokens | `tokens` |
| x/toolbox | `toolbox` |
| x/tree | `tree` |
| x/vesting_rewards | `vesting_rewards` |

## Sessions (6)

| # | File | Scope | Parallelism |
|---|------|-------|-------------|
| R13-1 | R13-1-axiom-loader.md | Port axiom-loader tool (validate, inject, stats) | Wave 1 |
| R13-2 | R13-2-genesis-ceremony.md | Production genesis ceremony script (init → add-validator → finalize → export → countdown) | Wave 1 |
| R13-3 | R13-3-testnet-genesis.md | Testnet genesis with ALL params explicitly configured + config reference JSON | Wave 1 |
| R13-4 | R13-4-configure-node.md | Node configuration script with mode presets (validator/fullnode/seed/archive) | Wave 1 |
| R13-5 | R13-5-genesis-validation.md | Per-module genesis validation tests + InitGenesis→ExportGenesis + block production test | Wave 2 (after R13-3) |
| R13-6 | R13-6-verify.md | End-to-end: ceremony → inject axioms → validate → start → produce blocks | Wave 3 (after all) |

## Run Order

- **Wave 1 (parallel):** R13-1, R13-2, R13-3, R13-4
- **Wave 2:** R13-5 (depends on R13-3 for param values reference)
- **Wave 3:** R13-6 (depends on all)

## Exit Criteria

1. `tools/axiom-loader` validates 777 axioms with zero DAG errors
2. `tools/axiom-loader inject` successfully embeds axioms into genesis.json
3. `scripts/genesis-ceremony.sh init → add-validator → finalize → export` produces valid genesis
4. `scripts/testnet-genesis.sh` explicitly sets every param for all 30+ modules
5. `scripts/testnet-genesis-config.json` documents every param with annotations
6. `scripts/configure-node.sh` supports validator/fullnode/seed/archive modes
7. Per-module DefaultGenesis + Validate tests pass for all 30 custom modules
8. InitGenesis → ExportGenesis round-trip preserves all state
9. 100-block production test passes from generated genesis
10. `go test ./...` — all pass
