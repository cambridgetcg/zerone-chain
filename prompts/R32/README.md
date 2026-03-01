# R32 — 鍛 (Tan): Forging — Interchaintest E2E Framework

**Goal:** Integrate [interchaintest](https://github.com/strangelove-ventures/interchaintest) (Strangelove, v8/SDK v0.50 compatible) as the E2E testing backbone. Build a reusable harness that spins up real `zeroned` chains in Docker, exercises full transaction lifecycles, and validates state transitions end-to-end.

interchaintest is the Cosmos ecosystem standard for E2E testing. It orchestrates Docker containers running real chain binaries — no mocks, no simulated consensus. Every major Cosmos chain (Osmosis, Stride, Celestia, dYdX) uses it for mainnet qualification.

## Why interchaintest

| What we have | What we need |
|---|---|
| 493 unit tests (mock keepers) | Real ABCI commit cycles |
| 15 cross-stack tests (single-process) | Multi-validator consensus |
| 3 simulation tests (economic invariants) | Full tx lifecycle (sign → broadcast → confirm → query) |
| Shell scripts (boot-test, smoke-test) | Reproducible, CI-parallelizable Go tests |
| No IBC tests with real relayers | IBC transfer + channel tests |

interchaintest gives us:
- **Real consensus**: CometBFT with 1-4 validators
- **Real transactions**: sign, broadcast, wait for inclusion
- **Real queries**: gRPC/REST against running nodes
- **Docker isolation**: each test gets a fresh chain
- **IBC**: built-in relayer support (Hermes/rly)
- **CI-native**: Go test binary, parallelizable, GitHub Actions compatible

## Sessions (6)

| # | File | Scope |
|---|------|-------|
| R32-1 | R32-1-harness-scaffold.md | interchaintest dependency, chain config, base harness, single-validator smoke |
| R32-2 | R32-2-genesis-lifecycle.md | Custom genesis with all 32 modules, param validation, export/import round-trip |
| R32-3 | R32-3-knowledge-e2e.md | Full knowledge lifecycle: claim → commitment → reveal → round → fact |
| R32-4 | R32-4-governance-e2e.md | Proposal lifecycle, LIP submission, emergency halt, param changes |
| R32-5 | R32-5-economic-e2e.md | Vesting rewards, staking, fee distribution, research fund flow |
| R32-6 | R32-6-multi-validator.md | 4-validator network, validator set changes, slashing, upgrade proposal |

## Run Order

- **R32-1** first (harness scaffold)
- **R32-2, R32-3, R32-4, R32-5** parallel (all use the harness)
- **R32-6** last (depends on harness + governance tests)

## Architecture

```
tests/
├── e2e/                          ← NEW (interchaintest)
│   ├── chain_config.go           ← zeroned Docker image + genesis config
│   ├── harness.go                ← shared setup/teardown, account funding
│   ├── helpers.go                ← tx broadcast, query, wait helpers
│   ├── genesis_test.go           ← R32-2
│   ├── knowledge_test.go         ← R32-3
│   ├── governance_test.go        ← R32-4
│   ├── economic_test.go          ← R32-5
│   └── multivalidator_test.go    ← R32-6
├── cross_stack/                  ← existing (keep)
├── integration/                  ← existing (keep)
└── simulation/                   ← existing (keep)
```

## Design Principles

1. **One chain per test function** — isolation over speed
2. **No shell scripts** — everything in Go, reproducible
3. **Assert on-chain state, not logs** — query the node, not parse stdout
4. **Parallel-safe** — each test uses unique chain-id and ports
5. **CI budget: 15 minutes** — fast enough for every PR

## The Deeper Pattern

| Framework | R-batch | What it does |
|-----------|---------|-------------|
| Prima Materia | R1-R15 | Raw modules created |
| Jungian Alchemy (R28) | Nigredo → Rubedo | Modules become self-aware |
| Tàijí (R29) | Yin-Yang | Pairs learn balance |
| 掃除 Sōji (R30) | Sweeping | Foundation cleaned |
| 五行 Wu Xing (R31) | Five Circulations | System circulates energy |
| **鍛 Tan (R32)** | **Forging** | **System tested by fire — real consensus, real transactions** |

The forge doesn't create new metal — it proves what's already there can hold. R32 subjects ZERONE to real-world conditions for the first time.
