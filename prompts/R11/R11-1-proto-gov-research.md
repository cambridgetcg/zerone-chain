# R11-1 — Proto Definitions for x/gov Research Spend Messages

## Context

The x/gov module has hand-written Go struct message types that implement `ProtoMessage()` without actual protobuf definitions. This causes a runtime panic because the Cosmos SDK interface registry resolves them all to typeURL `/` and detects duplicates.

**Panic message:**
```
panic: concrete type *types.MsgSubmitResearchSpend has already been registered under typeURL /,
cannot register *types.MsgVoteResearchSpend under same typeURL
```

## Affected Types

All in `x/gov/types/types.go` (lines ~244-340):

1. `MsgSubmitResearchSpend` — proposer, title, description, recipient, amount, justification
2. `MsgSubmitResearchSpendResponse` — proposal_id (uint64)
3. `MsgVoteResearchSpend` — voter, proposal_id, vote ("yes"/"no"), reasoning
4. `MsgVoteResearchSpendResponse` — empty
5. `MsgSetResearchVoters` — authority, voters (ResearchFundVoters — already in types.pb.go)
6. `MsgSetResearchVotersResponse` — empty

## Task

### 1. Add proto definitions to `proto/zerone/gov/v1/tx.proto`

Add to the `service Msg` block:
```protobuf
rpc SubmitResearchSpend(MsgSubmitResearchSpend) returns (MsgSubmitResearchSpendResponse);
rpc VoteResearchSpend(MsgVoteResearchSpend) returns (MsgVoteResearchSpendResponse);
rpc SetResearchVoters(MsgSetResearchVoters) returns (MsgSetResearchVotersResponse);
```

Add message definitions matching the existing Go struct fields. Use `cosmos.msg.v1.signer` annotations consistent with the existing messages in the same file (e.g. `MsgSubmitLIP`).

**Important:** `ResearchFundVoters` is already defined in `proto/zerone/gov/v1/types.proto` and generated in `types.pb.go` — reference it, don't redefine it.

### 2. Regenerate protobuf code

```bash
cd proto && buf generate
```

### 3. Remove hand-written types from `x/gov/types/types.go`

Delete the hand-written struct definitions, `ProtoMessage()`, `Reset()`, `String()` stubs for all 6 types listed above. Keep the `ValidateBasic()` and `GetSigners()` methods — they should work with the generated types.

**Note:** The generated types will use proto field names (snake_case in proto → Go CamelCase). Make sure field names match:
- `proposal_id` → `ProposalId` (uint64)
- `proposer` → `Proposer` (string)
- etc.

### 4. Update gRPC server registration

Check `x/gov/types/research_grpc.go` — this has a hand-written gRPC service registration for `ResearchMsgServer`. After proto-gen, the gRPC service handlers will be generated in `tx_grpc.pb.go`. Remove the hand-written gRPC code and use the generated server interface instead.

### 5. Update keeper msg_server.go

The keeper methods `SubmitResearchSpend` and `VoteResearchSpend` in `x/gov/keeper/msg_server.go` should implement the generated `MsgServer` interface. Verify the method signatures match.

### 6. Verify

```bash
go build ./...
go test ./x/gov/...
```

## Reference

- Existing proto pattern: `proto/zerone/gov/v1/tx.proto` (see MsgSubmitLIP for style)
- Existing generated code: `x/gov/types/tx.pb.go`
- ResearchFundVoters proto: `proto/zerone/gov/v1/types.proto`
- Hand-written gRPC: `x/gov/types/research_grpc.go`
- Keeper implementations: `x/gov/keeper/msg_server.go`, `x/gov/keeper/research_spend.go`
