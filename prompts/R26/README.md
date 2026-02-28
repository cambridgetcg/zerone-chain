# R26 — Cross-Module Wiring: Making ZERONE Actually Work

**Goal:** Wire the P0 connections identified in R25's collaboration assessment. After R26, the core truth-seeking loop (claim → qualify → verify → reward → vest) should function end-to-end with real enforcement.

## Why This Matters

R25 proved the modules are well-built islands. R26 connects them into a continent. Every session in this batch fixes a real enforcement gap — no new features, just making existing code actually talk to each other.

## What's Being Fixed

| Gap | Current State | After R26 |
|-----|--------------|-----------|
| Block rewards | `SetBlockTxCount` never called — 0 rewards minted | Called in PrepareProposal, economic loop alive |
| Account type enforcement | `CanSubmitClaims`/`CanChallenge` flags set but never checked | AnteHandler enforces all capability flags |
| Qualification gating | `IsQualified()` exists but never called from verification | SubmitCommitment + vote extensions check qualification |
| Partnership rewards | `DistributeReward()` exists but never called | Claims with valid partnership_id route through splits |
| Research resolution | Requires governance authority — lifecycle broken | Auto-resolve when review conditions met |
| Tree non-determinism | State divergence on val0 | Root cause found and fixed |

## Sessions (7)

| # | File | Scope | Parallelism |
|---|------|-------|-------------|
| R26-1 | R26-1-block-rewards.md | Wire SetBlockTxCount + verify economic loop end-to-end | Wave 1 |
| R26-2 | R26-2-capability-enforcement.md | Enforce CanSubmitClaims/CanChallenge/all flags in AnteHandler | Wave 1 |
| R26-3 | R26-3-qualification-gating.md | Wire DomainQualificationKeeper into verification flow | Wave 1 |
| R26-4 | R26-4-partnership-rewards.md | Add PartnershipKeeper to knowledge, route rewards through splits | Wave 2 |
| R26-5 | R26-5-research-resolution.md | Auto-resolve research + auto-fulfill bounties without governance | Wave 2 |
| R26-6 | R26-6-tree-determinism.md | Investigate and fix tree module non-determinism | Wave 2 |
| R26-7 | R26-7-integration-verify.md | End-to-end localnet test: full loop with all wiring active | Wave 3 |

## Run Order

- **Wave 1 (parallel):** R26-1, R26-2, R26-3
- **Wave 2 (parallel):** R26-4, R26-5, R26-6
- **Wave 3 (sequential):** R26-7 (depends on all previous)

## Key Principle

**No new features.** Every change in R26 connects existing code. The interfaces are defined, the functions are written, the keeper methods exist. We're adding calls, not inventing APIs.
