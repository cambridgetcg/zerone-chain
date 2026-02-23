# R12-3: Add MsgUpdateParams to 4 Missing Modules — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add governance-gated `MsgUpdateParams` to `capture_challenge`, `capture_defense`, `disputes`, and `home` modules so their parameters can be changed via governance vote.

**Architecture:** Cookie-cutter pattern from billing reference. Each module gets: proto message definition, msg_server handler, codec registration, and tests. All 4 modules already have Params proto messages, Validate(), GetAuthority(), and SetParams() — only the wiring is missing.

**Tech Stack:** Protobuf, Cosmos SDK v0.50, Go 1.21+, buf for proto codegen.

---

### Task 1: Add proto definitions to all 4 modules

**Files:**
- Modify: `proto/zerone/capture_challenge/v1/tx.proto`
- Modify: `proto/zerone/capture_defense/v1/tx.proto`
- Modify: `proto/zerone/disputes/v1/tx.proto`
- Modify: `proto/zerone/home/v1/tx.proto`

**Step 1: Edit capture_challenge tx.proto**

Add `genesis.proto` import (Params lives there), add UpdateParams rpc to service, and add message definitions at the end:

```protobuf
import "zerone/capture_challenge/v1/genesis.proto";
```

Add to service Msg block:
```protobuf
  rpc UpdateParams(MsgUpdateParams) returns (MsgUpdateParamsResponse);
```

Add at end of file:
```protobuf
message MsgUpdateParams {
  option (cosmos.msg.v1.signer) = "authority";
  string authority = 1;
  Params params    = 2;
}
message MsgUpdateParamsResponse {}
```

**Step 2: Edit capture_defense tx.proto**

Add `genesis.proto` import (Params lives there), add UpdateParams rpc to service, and add message definitions at the end:

```protobuf
import "zerone/capture_defense/v1/genesis.proto";
```

Add to service Msg block:
```protobuf
  rpc UpdateParams(MsgUpdateParams) returns (MsgUpdateParamsResponse);
```

Add at end of file:
```protobuf
message MsgUpdateParams {
  option (cosmos.msg.v1.signer) = "authority";
  string authority = 1;
  Params params    = 2;
}
message MsgUpdateParamsResponse {}
```

**Step 3: Edit disputes tx.proto**

`genesis.proto` is already imported. Add UpdateParams rpc to service and add message definitions at the end:

Add to service Msg block:
```protobuf
  rpc UpdateParams(MsgUpdateParams) returns (MsgUpdateParamsResponse);
```

Add at end of file:
```protobuf
message MsgUpdateParams {
  option (cosmos.msg.v1.signer) = "authority";
  string authority = 1;
  Params params    = 2;
}
message MsgUpdateParamsResponse {}
```

**Step 4: Edit home tx.proto**

Add `genesis.proto` import (Params lives there), add UpdateParams rpc to service, and add message definitions at the end:

```protobuf
import "zerone/home/v1/genesis.proto";
```

Add to service Msg block:
```protobuf
  rpc UpdateParams(MsgUpdateParams) returns (MsgUpdateParamsResponse);
```

Add at end of file:
```protobuf
message MsgUpdateParams {
  option (cosmos.msg.v1.signer) = "authority";
  string authority = 1;
  Params params    = 2;
}
message MsgUpdateParamsResponse {}
```

**Step 5: Run buf generate**

Run: `cd /Users/yuai/Desktop/Zerone/proto && buf generate`
Expected: Clean generation, no errors.

**Step 6: Verify generated Go code exists**

Run: `ls -la /Users/yuai/Desktop/Zerone/x/{capture_challenge,capture_defense,disputes,home}/types/tx.pb.go`
Expected: All 4 files exist with recent timestamps.

**Step 7: Commit**

```bash
git add proto/zerone/capture_challenge/v1/tx.proto \
        proto/zerone/capture_defense/v1/tx.proto \
        proto/zerone/disputes/v1/tx.proto \
        proto/zerone/home/v1/tx.proto \
        x/capture_challenge/types/ \
        x/capture_defense/types/ \
        x/disputes/types/ \
        x/home/types/
git commit -m "proto(R12-3): add MsgUpdateParams to 4 missing modules"
```

---

### Task 2: Add UpdateParams handlers to all 4 msg_server.go files

**Files:**
- Modify: `x/capture_challenge/keeper/msg_server.go`
- Modify: `x/capture_defense/keeper/msg_server.go`
- Modify: `x/disputes/keeper/msg_server.go`
- Modify: `x/home/keeper/msg_server.go`

**Step 1: Add UpdateParams to capture_challenge msg_server.go**

Append at end of file. Note: `context`, `fmt`, and `sdk` are already imported. The receiver is `msgServer` (not `Keeper`). SetParams takes `*types.Params`.

```go
// UpdateParams handles MsgUpdateParams — governance-gated parameter update.
func (ms msgServer) UpdateParams(goCtx context.Context, msg *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	if ms.GetAuthority() != msg.Authority {
		return nil, fmt.Errorf("unauthorized: expected %s, got %s", ms.GetAuthority(), msg.Authority)
	}

	ctx := sdk.UnwrapSDKContext(goCtx)
	if msg.Params == nil {
		return nil, fmt.Errorf("params cannot be nil")
	}
	if err := msg.Params.Validate(); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	ms.SetParams(ctx, msg.Params)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.capture_challenge.params_updated",
			sdk.NewAttribute("authority", msg.Authority),
		),
	)

	return &types.MsgUpdateParamsResponse{}, nil
}
```

**Step 2: Add UpdateParams to capture_defense msg_server.go**

Same pattern. Receiver is `k msgServer`. Append at end.

```go
// UpdateParams handles MsgUpdateParams — governance-gated parameter update.
func (k msgServer) UpdateParams(goCtx context.Context, msg *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	if k.GetAuthority() != msg.Authority {
		return nil, fmt.Errorf("unauthorized: expected %s, got %s", k.GetAuthority(), msg.Authority)
	}

	ctx := sdk.UnwrapSDKContext(goCtx)
	if msg.Params == nil {
		return nil, fmt.Errorf("params cannot be nil")
	}
	if err := msg.Params.Validate(); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	k.SetParams(ctx, msg.Params)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.capture_defense.params_updated",
			sdk.NewAttribute("authority", msg.Authority),
		),
	)

	return &types.MsgUpdateParamsResponse{}, nil
}
```

**Step 3: Add UpdateParams to disputes msg_server.go**

Same pattern. Receiver is `k msgServer`. Append at end.

```go
// UpdateParams handles MsgUpdateParams — governance-gated parameter update.
func (k msgServer) UpdateParams(goCtx context.Context, msg *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	if k.GetAuthority() != msg.Authority {
		return nil, fmt.Errorf("unauthorized: expected %s, got %s", k.GetAuthority(), msg.Authority)
	}

	ctx := sdk.UnwrapSDKContext(goCtx)
	if msg.Params == nil {
		return nil, fmt.Errorf("params cannot be nil")
	}
	if err := msg.Params.Validate(); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	k.SetParams(ctx, msg.Params)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.disputes.params_updated",
			sdk.NewAttribute("authority", msg.Authority),
		),
	)

	return &types.MsgUpdateParamsResponse{}, nil
}
```

**Step 4: Add UpdateParams to home msg_server.go**

Same pattern. Receiver is `k msgServer`. Append at end.

```go
// UpdateParams handles MsgUpdateParams — governance-gated parameter update.
func (k msgServer) UpdateParams(goCtx context.Context, msg *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	if k.GetAuthority() != msg.Authority {
		return nil, fmt.Errorf("unauthorized: expected %s, got %s", k.GetAuthority(), msg.Authority)
	}

	ctx := sdk.UnwrapSDKContext(goCtx)
	if msg.Params == nil {
		return nil, fmt.Errorf("params cannot be nil")
	}
	if err := msg.Params.Validate(); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	k.SetParams(ctx, msg.Params)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.home.params_updated",
			sdk.NewAttribute("authority", msg.Authority),
		),
	)

	return &types.MsgUpdateParamsResponse{}, nil
}
```

**Step 5: Commit**

```bash
git add x/capture_challenge/keeper/msg_server.go \
        x/capture_defense/keeper/msg_server.go \
        x/disputes/keeper/msg_server.go \
        x/home/keeper/msg_server.go
git commit -m "feat(R12-3): add UpdateParams handler to 4 modules"
```

---

### Task 3: Register MsgUpdateParams in codec for all 4 modules

**Files:**
- Modify: `x/capture_challenge/types/codec.go`
- Modify: `x/capture_defense/types/codec.go`
- Modify: `x/disputes/types/codec.go`
- Modify: `x/home/types/codec.go`

**Step 1: capture_challenge codec.go**

Add to RegisterCodec (after FundBountyPool line):
```go
	cdc.RegisterConcrete(&MsgUpdateParams{}, "zerone_capture_challenge/UpdateParams", nil)
```

Add to RegisterInterfaces (after &MsgFundBountyPool{} line):
```go
		&MsgUpdateParams{},
```

**Step 2: capture_defense codec.go**

Add to RegisterCodec (after AnalyzeDomain line):
```go
	cdc.RegisterConcrete(&MsgUpdateParams{}, "zerone_capture_defense/UpdateParams", nil)
```

Add to RegisterInterfaces (after &MsgAnalyzeDomain{} line):
```go
		&MsgUpdateParams{},
```

**Step 3: disputes codec.go**

Add to RegisterCodec (after SettleDispute line):
```go
	cdc.RegisterConcrete(&MsgUpdateParams{}, "zerone_disputes/UpdateParams", nil)
```

Add to RegisterInterfaces (after &MsgSettleDispute{} line):
```go
		&MsgUpdateParams{},
```

**Step 4: home codec.go**

Add to RegisterCodec (after SetSpendingLimit line):
```go
	cdc.RegisterConcrete(&MsgUpdateParams{}, "zerone_home/UpdateParams", nil)
```

Add to RegisterInterfaces (after &MsgSetSpendingLimit{} line):
```go
		&MsgUpdateParams{},
```

**Step 5: Build check**

Run: `cd /Users/yuai/Desktop/Zerone && go build ./...`
Expected: Clean build, no errors.

**Step 6: Commit**

```bash
git add x/capture_challenge/types/codec.go \
        x/capture_defense/types/codec.go \
        x/disputes/types/codec.go \
        x/home/types/codec.go
git commit -m "feat(R12-3): register MsgUpdateParams in codec for 4 modules"
```

---

### Task 4: Write tests for capture_challenge UpdateParams

**Files:**
- Modify: `x/capture_challenge/keeper/keeper_test.go`

**Step 1: Write tests**

Add after the existing `TestResolveChallengeUnauthorized` test (before the unused import guard). The test uses the existing `setupKeeper` which returns `(keeper.Keeper, sdk.Context, *mockBankKeeper)`.

```go
// -----------------------------------------------------------------------
// Tests: UpdateParams
// -----------------------------------------------------------------------

func TestUpdateParams(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	authority := k.GetAuthority()

	srv := keeper.NewMsgServerImpl(k)
	newParams := types.DefaultParams()
	newParams.MinChallengeStake = "50000000"
	newParams.EvidencePeriodBlocks = 10000

	_, err := srv.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: authority,
		Params:    newParams,
	})
	if err != nil {
		t.Fatalf("UpdateParams failed: %v", err)
	}

	got := k.GetParams(ctx)
	if got.MinChallengeStake != "50000000" {
		t.Errorf("expected MinChallengeStake 50000000, got %s", got.MinChallengeStake)
	}
	if got.EvidencePeriodBlocks != 10000 {
		t.Errorf("expected EvidencePeriodBlocks 10000, got %d", got.EvidencePeriodBlocks)
	}
}

func TestUpdateParamsUnauthorized(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	srv := keeper.NewMsgServerImpl(k)
	_, err := srv.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: testAddr(99),
		Params:    types.DefaultParams(),
	})
	if err == nil {
		t.Fatal("expected unauthorized error")
	}
}

func TestUpdateParamsNilParams(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	authority := k.GetAuthority()
	srv := keeper.NewMsgServerImpl(k)
	_, err := srv.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: authority,
		Params:    nil,
	})
	if err == nil {
		t.Fatal("expected nil params error")
	}
}

func TestUpdateParamsInvalid(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	authority := k.GetAuthority()
	srv := keeper.NewMsgServerImpl(k)

	badParams := types.DefaultParams()
	badParams.MinChallengeStake = "0" // invalid: must be > 0

	_, err := srv.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: authority,
		Params:    badParams,
	})
	if err == nil {
		t.Fatal("expected validation error for invalid params")
	}
}
```

**Step 2: Run tests**

Run: `cd /Users/yuai/Desktop/Zerone && go test ./x/capture_challenge/...`
Expected: All tests PASS.

**Step 3: Commit**

```bash
git add x/capture_challenge/keeper/keeper_test.go
git commit -m "test(R12-3): add UpdateParams tests for capture_challenge"
```

---

### Task 5: Write tests for capture_defense UpdateParams

**Files:**
- Modify: `x/capture_defense/keeper/keeper_test.go`

**Step 1: Write tests**

The setupKeeper returns `(keeper.Keeper, sdk.Context)` — no mocks returned. Add at end of file:

```go
// -----------------------------------------------------------------------
// Tests: UpdateParams
// -----------------------------------------------------------------------

func TestUpdateParams(t *testing.T) {
	k, ctx := setupKeeper(t)
	authority := k.GetAuthority()

	srv := keeper.NewMsgServerImpl(k)
	newParams := types.DefaultParams()
	newParams.DecayEpochBlocks = 20000
	newParams.HhiThreshold = 300000

	_, err := srv.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: authority,
		Params:    newParams,
	})
	if err != nil {
		t.Fatalf("UpdateParams failed: %v", err)
	}

	got := k.GetParams(ctx)
	if got.DecayEpochBlocks != 20000 {
		t.Errorf("expected DecayEpochBlocks 20000, got %d", got.DecayEpochBlocks)
	}
	if got.HhiThreshold != 300000 {
		t.Errorf("expected HhiThreshold 300000, got %d", got.HhiThreshold)
	}
}

func TestUpdateParamsUnauthorized(t *testing.T) {
	k, ctx := setupKeeper(t)
	srv := keeper.NewMsgServerImpl(k)

	addr := sdk.AccAddress([]byte(fmt.Sprintf("test-addr-%010d", 99))).String()
	_, err := srv.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: addr,
		Params:    types.DefaultParams(),
	})
	if err == nil {
		t.Fatal("expected unauthorized error")
	}
}

func TestUpdateParamsNilParams(t *testing.T) {
	k, ctx := setupKeeper(t)
	authority := k.GetAuthority()
	srv := keeper.NewMsgServerImpl(k)
	_, err := srv.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: authority,
		Params:    nil,
	})
	if err == nil {
		t.Fatal("expected nil params error")
	}
}

func TestUpdateParamsInvalid(t *testing.T) {
	k, ctx := setupKeeper(t)
	authority := k.GetAuthority()
	srv := keeper.NewMsgServerImpl(k)

	badParams := types.DefaultParams()
	badParams.DecayEpochBlocks = 0 // invalid: must be > 0

	_, err := srv.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: authority,
		Params:    badParams,
	})
	if err == nil {
		t.Fatal("expected validation error for invalid params")
	}
}
```

**Step 2: Run tests**

Run: `cd /Users/yuai/Desktop/Zerone && go test ./x/capture_defense/...`
Expected: All tests PASS.

**Step 3: Commit**

```bash
git add x/capture_defense/keeper/keeper_test.go
git commit -m "test(R12-3): add UpdateParams tests for capture_defense"
```

---

### Task 6: Write tests for disputes UpdateParams

**Files:**
- Modify: `x/disputes/keeper/keeper_test.go`

**Step 1: Write tests**

The setupKeeper returns `(keeper.Keeper, sdk.Context, *mockBankKeeper, *mockStakingKeeper, *mockKnowledgeKeeper)`. Add at end of file:

```go
// -----------------------------------------------------------------------
// Tests: UpdateParams
// -----------------------------------------------------------------------

func TestUpdateParams(t *testing.T) {
	k, ctx, _, _, _ := setupKeeper(t)
	authority := k.GetAuthority()

	srv := keeper.NewMsgServerImpl(k)
	newParams := types.DefaultParams()
	newParams.MaxActiveDisputes = 200
	newParams.EscalationDelay = 1000

	_, err := srv.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: authority,
		Params:    newParams,
	})
	if err != nil {
		t.Fatalf("UpdateParams failed: %v", err)
	}

	got := k.GetParams(ctx)
	if got.MaxActiveDisputes != 200 {
		t.Errorf("expected MaxActiveDisputes 200, got %d", got.MaxActiveDisputes)
	}
	if got.EscalationDelay != 1000 {
		t.Errorf("expected EscalationDelay 1000, got %d", got.EscalationDelay)
	}
}

func TestUpdateParamsUnauthorized(t *testing.T) {
	k, ctx, _, _, _ := setupKeeper(t)
	srv := keeper.NewMsgServerImpl(k)
	_, err := srv.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: testAddr(99),
		Params:    types.DefaultParams(),
	})
	if err == nil {
		t.Fatal("expected unauthorized error")
	}
}

func TestUpdateParamsNilParams(t *testing.T) {
	k, ctx, _, _, _ := setupKeeper(t)
	authority := k.GetAuthority()
	srv := keeper.NewMsgServerImpl(k)
	_, err := srv.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: authority,
		Params:    nil,
	})
	if err == nil {
		t.Fatal("expected nil params error")
	}
}

func TestUpdateParamsInvalid(t *testing.T) {
	k, ctx, _, _, _ := setupKeeper(t)
	authority := k.GetAuthority()
	srv := keeper.NewMsgServerImpl(k)

	badParams := types.DefaultParams()
	badParams.TierConfigs = nil // invalid: at least one tier required

	_, err := srv.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: authority,
		Params:    badParams,
	})
	if err == nil {
		t.Fatal("expected validation error for invalid params")
	}
}
```

**Step 2: Run tests**

Run: `cd /Users/yuai/Desktop/Zerone && go test ./x/disputes/...`
Expected: All tests PASS.

**Step 3: Commit**

```bash
git add x/disputes/keeper/keeper_test.go
git commit -m "test(R12-3): add UpdateParams tests for disputes"
```

---

### Task 7: Write tests for home UpdateParams

**Files:**
- Modify: `x/home/keeper/keeper_test.go`

**Step 1: Write tests**

The setupKeeper returns `(keeper.Keeper, sdk.Context, *mockBankKeeper)`. Note: `testAddr` here takes a string not an int. Add at end of file:

```go
// -----------------------------------------------------------------------
// Tests: UpdateParams
// -----------------------------------------------------------------------

func TestUpdateParams(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	authority := k.GetAuthority()

	srv := keeper.NewMsgServerImpl(k)
	newParams := types.DefaultParams()
	newParams.MaxKeysPerHome = 50
	newParams.SessionTimeoutBlocks = 2000

	_, err := srv.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: authority,
		Params:    newParams,
	})
	if err != nil {
		t.Fatalf("UpdateParams failed: %v", err)
	}

	got := k.GetParams(ctx)
	if got.MaxKeysPerHome != 50 {
		t.Errorf("expected MaxKeysPerHome 50, got %d", got.MaxKeysPerHome)
	}
	if got.SessionTimeoutBlocks != 2000 {
		t.Errorf("expected SessionTimeoutBlocks 2000, got %d", got.SessionTimeoutBlocks)
	}
}

func TestUpdateParamsUnauthorized(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	srv := keeper.NewMsgServerImpl(k)
	_, err := srv.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: testAddr("wrongauthority"),
		Params:    types.DefaultParams(),
	})
	if err == nil {
		t.Fatal("expected unauthorized error")
	}
}

func TestUpdateParamsNilParams(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	authority := k.GetAuthority()
	srv := keeper.NewMsgServerImpl(k)
	_, err := srv.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: authority,
		Params:    nil,
	})
	if err == nil {
		t.Fatal("expected nil params error")
	}
}

func TestUpdateParamsInvalid(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	authority := k.GetAuthority()
	srv := keeper.NewMsgServerImpl(k)

	badParams := types.DefaultParams()
	badParams.MaxKeysPerHome = 0 // invalid: must be > 0

	_, err := srv.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: authority,
		Params:    badParams,
	})
	if err == nil {
		t.Fatal("expected validation error for invalid params")
	}
}
```

**Step 2: Run tests**

Run: `cd /Users/yuai/Desktop/Zerone && go test ./x/home/...`
Expected: All tests PASS.

**Step 3: Commit**

```bash
git add x/home/keeper/keeper_test.go
git commit -m "test(R12-3): add UpdateParams tests for home"
```

---

### Task 8: Final verification

**Step 1: Full build**

Run: `cd /Users/yuai/Desktop/Zerone && go build ./...`
Expected: Clean build.

**Step 2: Run all 4 module test suites**

Run: `cd /Users/yuai/Desktop/Zerone && go test ./x/capture_challenge/... ./x/capture_defense/... ./x/disputes/... ./x/home/...`
Expected: All PASS.

**Step 3: Final commit (if any fixups needed)**

No new commit if everything passes. Otherwise fix and commit.
