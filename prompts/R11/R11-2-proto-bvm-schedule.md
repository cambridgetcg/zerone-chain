# R11-2 — Proto Definitions for x/bvm Schedule & Params Messages

## Context

The x/bvm module has hand-written Go struct message types with `ProtoMessage()` stubs. Unlike x/gov, these DO have `XXX_MessageName()` methods providing unique typeURLs, so they may not panic immediately — but they violate the proto-first principle and will break any client that tries to use protobuf serialization.

## Affected Types

All in `x/bvm/types/types.go` (lines ~80-210):

1. `MsgScheduleContract` — caller, contract_address, method, payload, execute_at_block, max_gas
2. `MsgScheduleContractResponse` — schedule_id (string)
3. `MsgCancelSchedule` — caller, schedule_id
4. `MsgCancelScheduleResponse` — empty
5. `MsgUpdateContractState` — authority, contract_address, key, value
6. `MsgUpdateContractStateResponse` — empty
7. `MsgUpdateParams` — authority, params (Params — check if already in proto)
8. `MsgUpdateParamsResponse` — empty

## Task

### 1. Add proto definitions to `proto/zerone/bvm/v1/tx.proto`

Add to the `service Msg` block:
```protobuf
rpc ScheduleContract(MsgScheduleContract) returns (MsgScheduleContractResponse);
rpc CancelSchedule(MsgCancelSchedule) returns (MsgCancelScheduleResponse);
rpc UpdateContractState(MsgUpdateContractState) returns (MsgUpdateContractStateResponse);
rpc UpdateParams(MsgUpdateParams) returns (MsgUpdateParamsResponse);
```

**Note:** There's already a `CancelSchedule` in `proto/zerone/schedule/v1/tx.proto` — that's a different module (x/schedule). The bvm one is specifically for BVM contract schedules. Use distinct proto message names or the existing naming is fine since they're in different packages (`zerone.bvm.v1` vs `zerone.schedule.v1`).

Add message definitions matching the Go struct fields. Use `cosmos.msg.v1.signer` annotations.

Check if `Params` type is already defined in `proto/zerone/bvm/v1/types.proto` or `genesis.proto`. If yes, import it. If not, add it.

### 2. Regenerate protobuf code

```bash
cd proto && buf generate
```

### 3. Remove hand-written types from `x/bvm/types/types.go`

Delete the hand-written struct definitions and all their stubs (`ProtoMessage`, `Reset`, `String`, `XXX_MessageName`). Keep `ValidateBasic()` and `GetSigners()`.

### 4. Update ExtendedMsgServer

The file has an `ExtendedMsgServer` interface that combines proto-generated `MsgServer` with hand-written handlers. After proto-gen, all handlers should be in the generated `MsgServer` interface. Remove `ExtendedMsgServer` and update `x/bvm/keeper/msg_server.go` to implement the unified generated interface.

### 5. Update codec registration

In `x/bvm/types/codec.go`, the `RegisterInterfaces` already registers these types. After proto-gen, verify the generated `RegisterInterfaces` is correct or that the manual one still works with the generated types.

### 6. Verify

```bash
go build ./...
go test ./x/bvm/...
```

## Reference

- Existing proto: `proto/zerone/bvm/v1/tx.proto` (see MsgDeployContract for style)
- Existing generated code: `x/bvm/types/tx.pb.go`
- Hand-written types: `x/bvm/types/types.go` lines ~80-210
- Keeper: `x/bvm/keeper/msg_server.go`
