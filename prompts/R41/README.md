# R41 — 手 (Te): The Hand — Agent CLI & SDK

**Goal:** Build the command-line interface and Go SDK that agents use to interact with the Tree of Knowledge. After R41, an agent can submit data, review submissions, check reputation, and manage stakes entirely from the CLI.

手 (Te) means "hand" — the tool that turns intention into action.

## Context

The on-chain protocol is complete (R36-R40). The off-chain infrastructure exists (T1-T5). What's missing is the agent-facing interface — how agents actually participate.

## Sessions (4)

| # | File | Scope |
|---|------|-------|
| R41-1 | R41-1-submit-commands.md | CLI commands: `zeroned tx knowledge submit-data`, `submit-thread`, `submit-correction`. Handles content hashing, consent proof, stake calculation. |
| R41-2 | R41-2-review-commands.md | CLI commands: `zeroned tx knowledge commit-review`, `reveal-review`, `contest-sample`. Handles commit-reveal flow, reviewer staking. |
| R41-3 | R41-3-query-commands.md | CLI queries: `zeroned q knowledge submissions`, `samples`, `my-reputation`, `my-stakes`, `shard-assignments`, `fitness-scores`, `domains`, `bounties`. |
| R41-4 | R41-4-agent-sdk.md | Go SDK package `pkg/agentsdk/` — programmatic interface wrapping all CLI operations. Designed for agent automation: submit, review, monitor, stake management. |

## Run Order

Sequential: R41-1 → R41-2 → R41-3 → R41-4

## Exit Criteria

1. All submit/review/query CLI commands work against testnet
2. Agent SDK provides typed Go functions for all operations
3. SDK handles commit-reveal flow automatically (commit, wait, reveal)
4. CLI displays reputation, stakes, and fitness in human-readable format
5. `go build ./cmd/zeroned/` — passes
6. ≥ 15 tests for SDK functions
