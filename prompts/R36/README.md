# R36 — 種 (Tane): The Seed — Proto Pivot to Training Data

**Goal:** Rewrite the `knowledge` module's protobuf types from a fact-claim verification system to a training data protocol. After R36, the new data model compiles, genesis works, and the type system reflects the pivot.

種 (Tane) means "seed" — we're planting a new kind of tree. The old knowledge tree stored verified facts. The new one grows from organic discourse — the conversations, debates, and explanations that make AI models actually useful.

## Context

Read `docs/DESIGN-training-data-protocol.md` for the full design rationale.

**The pivot:** Instead of `Claim → Verification → Fact`, we now have `Submission → Quality Validation → Sample`. The commit-reveal mechanism stays identical — what changes is *what* is being validated (quality/novelty/consent instead of truth).

## Sessions (5)

| # | File | Scope |
|---|------|-------|
| R36-1 | R36-1-proto-types.md | Rewrite `proto/zerone/knowledge/v1/types.proto` — new enums, messages, Sample, Submission, ConsentProof |
| R36-2 | R36-2-proto-tx-query.md | Rewrite `tx.proto` and `query.proto` — new Msg service (SubmitData, ScoreQuality, etc.) and query endpoints |
| R36-3 | R36-3-proto-genesis.md | Rewrite `genesis.proto` — new genesis state, seed dataset instead of 777 axioms |
| R36-4 | R36-4-codegen-types.md | Regenerate Go code, update `types/` package — keys, codec, params, validation |
| R36-5 | R36-5-migration.md | Write v4 migration from old knowledge state to new training data state, update module.go |

## Run Order

Sequential: R36-1 → R36-2 → R36-3 → R36-4 → R36-5

## Exit Criteria

1. All proto files compile with `make proto-gen`
2. `x/knowledge/types/` has updated Go types matching new protos
3. Genesis with seed dataset validates and round-trips
4. Migration from v3 → v4 state compiles (handler may be no-op for testnet)
5. `go build ./...` passes
6. All existing tests that reference old types are updated or marked TODO
