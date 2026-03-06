# R40 — 結 (Musubi): The Knot — ToK Feature Integration

**Goal:** Wire the four ToK spec features (reviewer staking, fitness decay, dataset sharding, reputation decay) into the R37-R39 keeper flow. After R40, these features are not standalone — they're part of the living system.

結 (Musubi) means "knot" or "connection" — tying separate threads into one fabric.

## Context

Four features were built from `docs/tok-spec.md` (sections 4.5-8.2) as standalone keeper methods:
- `reviewer_staking.go` — escrow/distribution on quality round resolution
- `fitness.go` — TDU lifecycle scoring and longevity rewards
- `sharding.go` — deterministic shard assignment for validators
- `reputation.go` — domain-level reputation decay for agents

These need to be wired into:
- `quality_round.go` / `phases.go` — reviewer staking escrow on commit, distribution on aggregate
- `ecology.go` / BeginBlocker — fitness decay per epoch, reputation decay per interval
- `submission.go` — reputation check on submission weight
- `sharding.go` ↔ `genesis.go` — shard state in genesis export/import
- `msg_server.go` — new Msg handlers for fitness updates and shard attestations

## Sessions (5)

| # | File | Scope |
|---|------|-------|
| R40-1 | R40-1-staking-integration.md | Wire reviewer staking into quality round commit/reveal/aggregate flow. Escrow on MsgCommitScore, distribute on aggregation. |
| R40-2 | R40-2-fitness-hooks.md | Wire fitness decay into BeginBlocker epoch processing. Score updates from quality round outcomes. Longevity reward distribution per epoch. |
| R40-3 | R40-3-sharding-lifecycle.md | Wire sharding into BeginBlocker (reshuffle on snapshot interval), genesis export/import, validator set change hooks. |
| R40-4 | R40-4-reputation-wiring.md | Wire reputation decay into BeginBlocker. Reset timer on successful submission/review. Use reputation as vote weight in quality rounds. |
| R40-5 | R40-5-integration-tests.md | End-to-end integration tests: full submission → commit → reveal → aggregate → fitness update → shard reshuffle → reputation adjustment cycle. Adversarial scenarios. |

## Run Order

Sequential: R40-1 → R40-2 → R40-3 → R40-4 → R40-5

## Exit Criteria

1. Reviewer staking escrow/distribution fires automatically during quality rounds
2. Fitness scores update after each quality round resolution
3. BeginBlocker runs fitness decay, reputation decay, and shard reshuffling at correct intervals
4. Reputation weights influence quality round vote aggregation
5. Genesis export/import preserves all new state (fitness records, shard assignments, reputation records)
6. Integration tests cover full lifecycle with ≥ 20 new tests
7. `go test ./x/knowledge/...` — all pass
