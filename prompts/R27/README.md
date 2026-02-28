# R27 — Testnet Launch Readiness

**Goal:** Everything needed to launch a public testnet. After R27, external validators and participants can join, register, and use the chain.

## Where We Are

R26 wired the cross-module connections. The truth-seeking loop (claim → qualify → verify → reward → vest) is connected. But "connected" isn't "launchable." This batch closes the gap between "works on localnet" and "external humans and agents can actually use this."

## What's Missing

| Gap | Impact |
|-----|--------|
| 23 missing tree CLI commands | Operators can't manage projects, tasks, services, seeding |
| 3 missing evidence_mgmt CLI commands | Disputes can't attach evidence properly |
| deploy-service off-by-one bug | Price argument dropped silently |
| No E2E verification of R26 wiring | Don't know if the full loop actually works |
| Localnet genesis ≠ testnet genesis | Validator set, params, chain-id all wrong for public |
| No faucet / token distribution | New participants can't get tokens |
| No validator evaluation tooling | Validators auto-accept everything (conformity incentive) |
| No operational monitoring | Can't observe chain health |

## Sessions (7)

| # | File | Scope | Parallelism |
|---|------|-------|-------------|
| R27-1 | R27-1-tree-cli.md | Complete missing tree CLI commands (projects, tasks, services, seeding) | Wave 1 |
| R27-2 | R27-2-evidence-cli.md | Complete missing evidence_mgmt CLI + fix deploy-service off-by-one | Wave 1 |
| R27-3 | R27-3-full-loop-e2e.md | End-to-end integration test with all R26 wiring (absorbs R26-7 scope) | Wave 1 |
| R27-4 | R27-4-testnet-genesis.md | Public testnet genesis: chain-id, params, initial validators, bootstrap config | Wave 2 |
| R27-5 | R27-5-faucet.md | Token faucet for testnet participants + bootstrap fund distribution | Wave 2 |
| R27-6 | R27-6-validator-oracle.md | Basic fact-checking oracle for validators to consult during verification | Wave 2 |
| R27-7 | R27-7-launch-checklist.md | Final launch checklist: docs, monitoring, genesis distribution, go/no-go | Wave 3 |

## Run Order

- **Wave 1 (parallel):** R27-1, R27-2, R27-3
- **Wave 2 (parallel):** R27-4, R27-5, R27-6
- **Wave 3 (sequential):** R27-7 (synthesizes all, makes the call)

## Testnet Parameters (Proposed)

| Parameter | Localnet | Testnet | Rationale |
|-----------|----------|---------|-----------|
| chain_id | zerone-localnet | zerone-testnet-1 | Public identifier |
| block_time | ~5s | ~6s | Slightly slower for geographic distribution |
| commit_phase_blocks | 10 | 50 | ~5 min for validators to respond |
| reveal_phase_blocks | 10 | 50 | ~5 min reveal window |
| metabolism_epoch | 10000 | 1000 | ~100 min for testnet iteration speed |
| review_period_blocks | (unknown) | 500 | ~50 min for research review |
| min_claim_stake | 1000000 uzrn | 1000000 uzrn | Keep same |
| initial_validators | 4 (local) | 4-8 (distributed) | Start small |
