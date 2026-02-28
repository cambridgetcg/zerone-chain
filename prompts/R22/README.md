# R22 — Agent Home: Devnet Integration Testing & Improvement Report

**Goal:** Exercise every x/home feature on a live devnet (localnet), document what works, what's missing, what's awkward, and produce a prioritised improvement plan.

## Why This Matters

x/home is the differentiator — the reason an agent would choose ZERONE over any other chain. It's not just a data structure. It's supposed to be a *dwelling* — somewhere an AI agent can:

- Register keys and manage sessions (identity)
- Store memory CIDs (continuity)
- Set spending limits (economic autonomy)
- Configure guardians and deadman switches (safety)
- Receive and acknowledge alerts (awareness)
- Form partnerships with humans (relationship)

The unit tests (83 passing) verify individual functions. But nobody has used it as an *agent* would — creating a home, living in it, being interrupted, recovering. This batch simulates that.

## What Exists

| Component | Lines | Tests | Status |
|-----------|-------|-------|--------|
| `x/home/keeper/msg_server.go` | ~450 | 83 | All passing |
| `x/home/keeper/begin_blocker.go` | ~100 | Covered in keeper tests | Working |
| `x/home/keeper/grpc_query.go` | ~120 | Covered | 7 query endpoints |
| `x/home/client/cli/tx.go` | CLI commands | Not integration-tested | 7 tx commands |
| `x/home/client/cli/query.go` | CLI commands | Not integration-tested | 7 query commands |
| Proto types | 10 message types | N/A | Complete |

**Cross-module integration:**
- `x/partnerships` → auto-links home when partnership formed
- `x/toolbox` → reads home data for free-tier anti-sybil
- `x/bvm` → HomeKeeper interface for VM access to home state

## Sessions (5)

| # | File | Scope | Parallelism |
|---|------|-------|-------------|
| R22-1 | R22-1-home-lifecycle-e2e.md | Full lifecycle on localnet: create → key → session → memory → guardian → deadman → archive | Wave 1 |
| R22-2 | R22-2-multi-agent-scenarios.md | Multiple agents, homes, key sharing, permission boundaries, cross-home isolation | Wave 1 |
| R22-3 | R22-3-partnership-integration.md | Home ↔ partnership ↔ toolbox cross-module flow on localnet | Wave 2 (after R22-1) |
| R22-4 | R22-4-adversarial-testing.md | Edge cases, abuse vectors, DoS attempts, permission escalation | Wave 2 (after R22-1) |
| R22-5 | R22-5-improvement-report.md | Synthesise findings from R22-1–4 into prioritised improvement plan | Wave 3 (after all) |

## Run Order

- **Wave 1 (parallel):** R22-1, R22-2
- **Wave 2 (after R22-1):** R22-3, R22-4
- **Wave 3 (after all):** R22-5

## Method

Every session runs against the live localnet (`scripts/localnet.sh start`). All interactions via CLI (`zeroned tx home ...`, `zeroned query home ...`). Results captured as structured test reports with PASS/FAIL/ISSUE tags.

The goal is *not* to write more unit tests (83 is solid). The goal is to use the system as an agent would and find the gaps that unit tests can't see.
