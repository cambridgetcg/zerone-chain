# R12-3 — Add MsgUpdateParams to 5 Missing Modules

## Context

The prototype had `MsgUpdateParams` (authority-gated governance parameter update) on every module. In Zerone, 5 modules are missing it:

1. `capture_challenge`
2. `capture_defense`
3. `claiming_pot`
4. `disputes`
5. `home`

Without `MsgUpdateParams`, these modules' parameters can only be changed via a binary upgrade, not by governance vote or the ParamRouter.

## Task

For each of the 5 modules:

### 1. Add proto definitions to `proto/zerone/<module>/v1/tx.proto`

Add to the `service Msg` block:
```protobuf
rpc UpdateParams(MsgUpdateParams) returns (MsgUpdateParamsResponse);
```

Add message definitions:
```protobuf
message MsgUpdateParams {
  option (cosmos.msg.v1.signer) = "authority";

  string authority                          = 1; // must be governance module address
  zerone.<module>.v1.Params params           = 2 [(gogoproto.nullable) = false];
}

message MsgUpdateParamsResponse {}
```

**Important:** Reference each module's existing `Params` message from its `types.proto` / `genesis.proto`. Do not redefine it.

Check each module's proto files to find where `Params` is already defined. If `Params` is not yet a proto message (hand-written Go struct only), you must create it in `proto/zerone/<module>/v1/genesis.proto` first.

Regenerate:
```bash
cd proto && buf generate
```

### 2. Implement handler in `x/<module>/keeper/msg_server.go`

Follow the prototype's pattern exactly:

```go
// UpdateParams handles MsgUpdateParams — governance-gated parameter update.
func (ms msgServer) UpdateParams(goCtx context.Context, msg *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	if ms.GetAuthority() != msg.Authority {
		return nil, fmt.Errorf("unauthorized: expected %s, got %s", ms.GetAuthority(), msg.Authority)
	}

	ctx := sdk.UnwrapSDKContext(goCtx)

	if err := msg.Params.Validate(); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	ms.SetParams(ctx, &msg.Params)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.<module>.params_updated",
			sdk.NewAttribute("authority", msg.Authority),
		),
	)

	return &types.MsgUpdateParamsResponse{}, nil
}
```

Replace `<module>` with the actual module name in the event type.

### 3. Ensure keeper has `GetAuthority()` method

Each keeper should already have this. Verify:
```go
func (k Keeper) GetAuthority() string {
	return k.authority
}
```

If `authority` is not stored on the keeper struct, add it to `NewKeeper` params and struct field.

### 4. Ensure `Params.Validate()` exists

Each module should already have a `Validate()` method on its `Params` type. If not, add basic validation.

### 5. Register codec

In `x/<module>/types/codec.go`, ensure `MsgUpdateParams` is registered:
```go
registry.RegisterImplementations((*sdk.Msg)(nil),
	// ... existing registrations ...
	&MsgUpdateParams{},
)
```

### 6. Add CLI command (optional but recommended)

In `x/<module>/client/cli/tx.go`, add a `CmdUpdateParams` that reads a JSON params file and submits the tx. Follow the pattern from modules that already have it (e.g., `x/billing/client/cli/tx.go`).

## Module-Specific Notes

### capture_challenge
- Params likely include: challenge period blocks, minimum stake, reward distribution
- Prototype: `legible_money/x/capture_challenge/keeper/msg_server.go`

### capture_defense
- Params likely include: defense period blocks, bond requirements
- Prototype: `legible_money/x/capture_defense/keeper/msg_server.go`

### claiming_pot
- Params likely include: claim window, distribution rules
- Prototype: `legible_money/x/claiming_pot/keeper/msg_server.go`

### disputes
- Params likely include: dispute period, evidence window, arbitration threshold
- Prototype: `legible_money/x/disputes/keeper/msg_server.go`

### home
- Params likely include: max guardians, patina decay rate, comfort thresholds
- Prototype: `legible_money/x/home/keeper/msg_server.go`

## Verification

```bash
# Must compile
go build ./...

# Test each module
go test ./x/capture_challenge/...
go test ./x/capture_defense/...
go test ./x/claiming_pot/...
go test ./x/disputes/...
go test ./x/home/...
```

Write a test for each module that:
1. Sets initial params via genesis
2. Calls `UpdateParams` with valid authority → succeeds
3. Calls `UpdateParams` with wrong authority → fails with "unauthorized"
4. Calls `UpdateParams` with invalid params → fails with validation error

## Reference

- Prototype pattern: `legible_money/x/capture_challenge/keeper/msg_server.go` (UpdateParams)
- Working Zerone example: `x/billing/keeper/msg_server.go` (UpdateParams — already implemented)
- Proto pattern: `proto/zerone/billing/v1/tx.proto` (MsgUpdateParams)
