# R11 — Stabilisation: Proto Compliance + Runtime Fix + Test Green

**Goal:** `zeroned` binary runs without panics. All tests pass. Every message type has proper protobuf definitions. Ready to share with other developers.

## The Problem

The binary builds but panics at runtime due to hand-written Go structs that implement `ProtoMessage()` without actual protobuf registration. The Cosmos SDK interface registry sees them all as typeURL `/` and panics on duplicate registration.

### Affected Modules

| Module | Hand-written types (no .proto) | File |
|--------|-------------------------------|------|
| x/gov | MsgSubmitResearchSpend, MsgVoteResearchSpend, MsgSetResearchVoters + responses | x/gov/types/types.go |
| x/bvm | MsgScheduleContract, MsgCancelSchedule, MsgUpdateContractState, MsgUpdateParams + response | x/bvm/types/types.go |
| x/ontology | IncompletenessAcknowledgment (data type, not Msg) | x/ontology/types/types.go |

### Root Cause

These types were added in later rounds (R3-R7) as hand-written Go structs with `ProtoMessage()` stubs but no corresponding `.proto` file definitions and no generated `.pb.go` code. Without protobuf reflection data, they resolve to typeURL `/` and collide during `RegisterImplementations`.

## Sessions (3)

| # | File | Scope |
|---|------|-------|
| R11-1 | R11-1-proto-gov-research.md | Add missing proto definitions for x/gov research spend messages, regenerate, fix codec |
| R11-2 | R11-2-proto-bvm-schedule.md | Add missing proto definitions for x/bvm schedule/params messages, regenerate, fix codec |
| R11-3 | R11-3-runtime-verify.md | Full runtime verification: `zeroned version`, `zeroned init`, `zeroned start`, all tests green |

## Run Order

- **Wave 1 (parallel):** R11-1, R11-2
- **Wave 2:** R11-3 (depends on R11-1 + R11-2)

## Exit Criteria

1. `zeroned version` prints version without panic
2. `zeroned init test-node --chain-id zerone-testnet-1` succeeds
3. `go test ./...` — all packages pass
4. No hand-written `ProtoMessage()` stubs outside `.pb.go` files (except pure data types like IncompletenessAcknowledgment)
5. `make proto-gen` regenerates cleanly
