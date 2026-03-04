# R37 — 流 (Nagare): The Flow — Keeper Pivot to Quality Validation

**Goal:** Rewrite the `knowledge` keeper from truth verification to quality validation. After R37, submissions flow through quality rounds, samples are created, and the ecological dynamics apply to training data.

流 (Nagare) means "flow" — data flows in as submissions, flows through quality validation, and flows out as curated training samples.

## Context

- R36 completed the type system pivot (protos, codegen, migration)
- The commit-reveal mechanism is structurally identical — only what's being scored changes
- The ecological dynamics (energy, fitness, niche competition) transfer directly

## Sessions (6)

| # | File | Scope |
|---|------|-------|
| R37-1 | R37-1-submission-lifecycle.md | Submission handling: SubmitData, SubmitThread, content hashing, duplicate detection, consent validation |
| R37-2 | R37-2-quality-rounds.md | Quality round lifecycle: VRF selection, commit, reveal, aggregation → Sample creation |
| R37-3 | R37-3-sample-ecology.md | Sample fitness, energy metabolism, niche dynamics, pruning (adapted from fact ecology) |
| R37-4 | R37-4-contest-sponsor.md | ContestSample, SponsorSample, dispute flow, re-validation |
| R37-5 | R37-5-domain-demand.md | Domain management, TrainingDemand tracking, DataBounty auto-generation |
| R37-6 | R37-6-beginend-block.md | BeginBlocker/EndBlocker: round phase transitions, energy decay, pruning, bounty matching |

## Run Order

Sequential: R37-1 → R37-2 → R37-3 → R37-4 → R37-5 → R37-6

## Exit Criteria

1. `go test ./x/knowledge/...` — all pass
2. Full submission → quality round → sample creation flow works in tests
3. Ecological dynamics (energy, fitness, pruning) work for samples
4. Contest and sponsor flows work
5. BeginBlocker/EndBlocker handle all phase transitions
6. Test coverage ≥ 300 tests for knowledge module
