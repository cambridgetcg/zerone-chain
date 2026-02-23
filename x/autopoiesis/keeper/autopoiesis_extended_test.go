package keeper_test

import (
	"math/big"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/autopoiesis/keeper"
	"github.com/zerone-chain/zerone/x/autopoiesis/types"
)

func testAddr(name string) string {
	addr := make([]byte, 20)
	copy(addr, name)
	return sdk.AccAddress(addr).String()
}

// ========== MsgServer — UpdateParams ==========

func TestMsgUpdateParamsSuccess(t *testing.T) {
	k, _, _, _, ctx := setupKeeper(t)
	msgServer := keeper.NewMsgServerImpl(k)
	p := types.DefaultParams()
	p.EpochLengthBlocks = 200
	_, err := msgServer.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: "authority",
		Params:    &p,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := k.GetParams(ctx)
	if got.EpochLengthBlocks != 200 {
		t.Errorf("expected 200, got %d", got.EpochLengthBlocks)
	}
}

func TestMsgUpdateParamsUnauthorized(t *testing.T) {
	k, _, _, _, ctx := setupKeeper(t)
	msgServer := keeper.NewMsgServerImpl(k)
	p := types.DefaultParams()
	_, err := msgServer.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: "not-authority",
		Params:    &p,
	})
	if err == nil {
		t.Fatal("expected unauthorized error")
	}
}

func TestMsgUpdateParamsNilParams(t *testing.T) {
	k, _, _, _, ctx := setupKeeper(t)
	msgServer := keeper.NewMsgServerImpl(k)
	_, err := msgServer.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: "authority",
		Params:    nil,
	})
	if err == nil {
		t.Fatal("expected error for nil params")
	}
}

func TestMsgUpdateParamsInvalidParams(t *testing.T) {
	k, _, _, _, ctx := setupKeeper(t)
	msgServer := keeper.NewMsgServerImpl(k)
	p := types.DefaultParams()
	p.EpochLengthBlocks = 0 // invalid
	_, err := msgServer.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: "authority",
		Params:    &p,
	})
	if err == nil {
		t.Fatal("expected error for invalid params")
	}
}

// ========== MsgServer — Activate ==========

func TestMsgActivateSuccess(t *testing.T) {
	k, _, _, _, ctx := setupKeeper(t)
	msgServer := keeper.NewMsgServerImpl(k)
	// Deactivate first.
	_, err := msgServer.Activate(ctx, &types.MsgActivateAutopoiesis{
		Authority: "authority",
		Activate:  false,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if k.IsActive(ctx) {
		t.Error("expected inactive after deactivation")
	}
	// Re-activate.
	_, err = msgServer.Activate(ctx, &types.MsgActivateAutopoiesis{
		Authority: "authority",
		Activate:  true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !k.IsActive(ctx) {
		t.Error("expected active after re-activation")
	}
}

func TestMsgActivateUnauthorized(t *testing.T) {
	k, _, _, _, ctx := setupKeeper(t)
	msgServer := keeper.NewMsgServerImpl(k)
	_, err := msgServer.Activate(ctx, &types.MsgActivateAutopoiesis{
		Authority: "not-authority",
		Activate:  true,
	})
	if err == nil {
		t.Fatal("expected unauthorized error")
	}
}

func TestMsgDeactivate(t *testing.T) {
	k, _, _, _, ctx := setupKeeper(t)
	msgServer := keeper.NewMsgServerImpl(k)
	_, err := msgServer.Activate(ctx, &types.MsgActivateAutopoiesis{
		Authority: "authority",
		Activate:  false,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if k.IsActive(ctx) {
		t.Error("expected inactive")
	}
}

func TestMsgActivateSetsBaselineHeight(t *testing.T) {
	k, ctx := setupBareKeeper(t)
	// Set up state as not activated with LastEpochHeight=0.
	k.SetState(ctx, &types.AutopoiesisState{Activated: false})
	msgServer := keeper.NewMsgServerImpl(k)
	_, err := msgServer.Activate(ctx, &types.MsgActivateAutopoiesis{
		Authority: "authority",
		Activate:  true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	state := k.GetState(ctx)
	if state.LastEpochHeight != 1 { // block height is 1 in setupBareKeeper
		t.Errorf("expected LastEpochHeight=1, got %d", state.LastEpochHeight)
	}
}

// ========== MsgServer — OverrideMultiplier ==========

func TestMsgOverrideMultiplierUnauthorized(t *testing.T) {
	k, _, _, _, ctx := setupKeeper(t)
	msgServer := keeper.NewMsgServerImpl(k)
	_, err := msgServer.OverrideMultiplier(ctx, &types.MsgOverrideMultiplier{
		Authority: "not-authority",
		Path:      "rewards.block",
		Value:     750_000,
	})
	if err == nil {
		t.Fatal("expected unauthorized error")
	}
}

func TestMsgOverrideMultiplierInvalidPath(t *testing.T) {
	k, _, _, _, ctx := setupKeeper(t)
	msgServer := keeper.NewMsgServerImpl(k)
	_, err := msgServer.OverrideMultiplier(ctx, &types.MsgOverrideMultiplier{
		Authority: "authority",
		Path:      "invalid.path",
		Value:     750_000,
	})
	if err == nil {
		t.Fatal("expected error for invalid path")
	}
}

func TestMsgOverrideMultiplierSetsLastUpdated(t *testing.T) {
	k, _, _, _, ctx := setupKeeper(t)
	msgServer := keeper.NewMsgServerImpl(k)
	_, err := msgServer.OverrideMultiplier(ctx, &types.MsgOverrideMultiplier{
		Authority: "authority",
		Path:      "fees.base",
		Value:     1_200_000,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ms, found := k.GetMultiplierState(ctx, "fees.base")
	if !found {
		t.Fatal("fees.base not found")
	}
	if ms.LastUpdated != 1 { // block height is 1 in setupKeeper
		t.Errorf("expected LastUpdated=1, got %d", ms.LastUpdated)
	}
	if ms.CurrentBps != 1_200_000 || ms.TargetBps != 1_200_000 {
		t.Errorf("expected Current=Target=1200000, got %d/%d", ms.CurrentBps, ms.TargetBps)
	}
}

// ========== MsgServer — FreezeMultiplier ==========

func TestMsgFreezeMultiplierSuccess(t *testing.T) {
	k, _, _, _, ctx := setupKeeper(t)
	msgServer := keeper.NewMsgServerImpl(k)
	_, err := msgServer.FreezeMultiplier(ctx, &types.MsgFreezeMultiplier{
		Authority: "authority",
		Path:      "rewards.block",
		Frozen:    true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !k.IsMultiplierFrozen(ctx, "rewards.block") {
		t.Error("expected rewards.block to be frozen")
	}
	ms, _ := k.GetMultiplierState(ctx, "rewards.block")
	if !ms.Frozen {
		t.Error("expected multiplier state Frozen=true")
	}
}

func TestMsgFreezeMultiplierUnauthorized(t *testing.T) {
	k, _, _, _, ctx := setupKeeper(t)
	msgServer := keeper.NewMsgServerImpl(k)
	_, err := msgServer.FreezeMultiplier(ctx, &types.MsgFreezeMultiplier{
		Authority: "not-authority",
		Path:      "rewards.block",
		Frozen:    true,
	})
	if err == nil {
		t.Fatal("expected unauthorized error")
	}
}

func TestMsgFreezeMultiplierInvalidPath(t *testing.T) {
	k, _, _, _, ctx := setupKeeper(t)
	msgServer := keeper.NewMsgServerImpl(k)
	_, err := msgServer.FreezeMultiplier(ctx, &types.MsgFreezeMultiplier{
		Authority: "authority",
		Path:      "invalid.path",
		Frozen:    true,
	})
	if err == nil {
		t.Fatal("expected error for invalid path")
	}
}

func TestMsgFreezeMultiplierUnfreeze(t *testing.T) {
	k, _, _, _, ctx := setupKeeper(t)
	msgServer := keeper.NewMsgServerImpl(k)
	// Freeze.
	_, _ = msgServer.FreezeMultiplier(ctx, &types.MsgFreezeMultiplier{
		Authority: "authority", Path: "slashing.severity", Frozen: true,
	})
	// Unfreeze.
	_, err := msgServer.FreezeMultiplier(ctx, &types.MsgFreezeMultiplier{
		Authority: "authority", Path: "slashing.severity", Frozen: false,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if k.IsMultiplierFrozen(ctx, "slashing.severity") {
		t.Error("expected unfrozen")
	}
}

// ========== QueryServer ==========

func TestQueryParamsReturnsParams(t *testing.T) {
	k, _, _, _, ctx := setupKeeper(t)
	qServer := keeper.NewQueryServerImpl(k)
	resp, err := qServer.Params(ctx, &types.QueryParamsRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Params == nil {
		t.Fatal("expected non-nil params")
	}
	if resp.Params.EpochLengthBlocks != 100 {
		t.Errorf("expected default EpochLengthBlocks=100, got %d", resp.Params.EpochLengthBlocks)
	}
}

func TestQueryMultiplierFound(t *testing.T) {
	k, _, _, _, ctx := setupKeeper(t)
	qServer := keeper.NewQueryServerImpl(k)
	resp, err := qServer.Multiplier(ctx, &types.QueryMultiplierRequest{Path: "rewards.block"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Multiplier == nil {
		t.Fatal("expected non-nil multiplier")
	}
	if resp.Multiplier.Path != "rewards.block" {
		t.Errorf("expected path rewards.block, got %s", resp.Multiplier.Path)
	}
}

func TestQueryMultiplierNotFound(t *testing.T) {
	k, _, _, _, ctx := setupKeeper(t)
	qServer := keeper.NewQueryServerImpl(k)
	resp, err := qServer.Multiplier(ctx, &types.QueryMultiplierRequest{Path: "nonexistent"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Multiplier != nil {
		t.Error("expected nil multiplier for nonexistent path")
	}
}

func TestQueryAllMultipliersCount(t *testing.T) {
	k, _, _, _, ctx := setupKeeper(t)
	qServer := keeper.NewQueryServerImpl(k)
	resp, err := qServer.AllMultipliers(ctx, &types.QueryAllMultipliersRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Multipliers) != 3 {
		t.Errorf("expected 3 multipliers, got %d", len(resp.Multipliers))
	}
}

func TestQueryEpochSnapshotFound(t *testing.T) {
	k, _, _, _, ctx := setupKeeper(t)
	k.SetEpochSnapshot(ctx, &types.EpochSnapshot{Epoch: 1, BlockHeight: 101, SsiScore: 800_000})
	qServer := keeper.NewQueryServerImpl(k)
	resp, err := qServer.EpochSnapshot(ctx, &types.QueryEpochSnapshotRequest{Epoch: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if resp.Snapshot.SsiScore != 800_000 {
		t.Errorf("expected SsiScore=800000, got %d", resp.Snapshot.SsiScore)
	}
}

func TestQueryEpochSnapshotNotFound(t *testing.T) {
	k, _, _, _, ctx := setupKeeper(t)
	qServer := keeper.NewQueryServerImpl(k)
	resp, err := qServer.EpochSnapshot(ctx, &types.QueryEpochSnapshotRequest{Epoch: 999})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Snapshot != nil {
		t.Error("expected nil snapshot for nonexistent epoch")
	}
}

func TestQuerySSIReturnsScoreAndCategory(t *testing.T) {
	k, _, _, _, ctx := setupKeeper(t)
	k.SetSSI(ctx, 800_000)
	qServer := keeper.NewQueryServerImpl(k)
	resp, err := qServer.SSI(ctx, &types.QuerySSIRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.SsiScore != 800_000 {
		t.Errorf("expected SsiScore=800000, got %d", resp.SsiScore)
	}
	if resp.SsiCategory != types.SSIThriving {
		t.Errorf("expected category %s, got %s", types.SSIThriving, resp.SsiCategory)
	}
}

// ========== CollectAndAdapt Integration ==========

func TestCollectAndAdaptInactive(t *testing.T) {
	k, _, _, _, ctx := setupKeeper(t)
	state := k.GetState(ctx)
	state.Activated = false
	k.SetState(ctx, state)

	initial, _ := k.GetMultiplierState(ctx, "rewards.block")
	initialBps := initial.CurrentBps

	k.CollectAndAdapt(ctx)
	ctx = advanceBlocks(ctx, 200)
	k.CollectAndAdapt(ctx)

	after, _ := k.GetMultiplierState(ctx, "rewards.block")
	if after.CurrentBps != initialBps {
		t.Errorf("multiplier changed when inactive: %d → %d", initialBps, after.CurrentBps)
	}
}

func TestCollectAndAdaptDisabledParams(t *testing.T) {
	k, _, _, _, ctx := setupKeeper(t)
	p := types.DefaultParams()
	p.Enabled = false
	k.SetParams(ctx, &p)

	initial, _ := k.GetMultiplierState(ctx, "rewards.block")
	initialBps := initial.CurrentBps

	k.CollectAndAdapt(ctx)
	ctx = advanceBlocks(ctx, 200)
	k.CollectAndAdapt(ctx)

	after, _ := k.GetMultiplierState(ctx, "rewards.block")
	if after.CurrentBps != initialBps {
		t.Errorf("multiplier changed when disabled: %d → %d", initialBps, after.CurrentBps)
	}
}

func TestCollectAndAdaptSetsBaseline(t *testing.T) {
	k, _, _, _, ctx := setupKeeper(t)
	// Reset state to simulate first activation.
	k.SetState(ctx, &types.AutopoiesisState{Activated: true, LastEpochHeight: 0})

	k.CollectAndAdapt(ctx)
	state := k.GetState(ctx)
	if state.LastEpochHeight != 1 { // block 1 from setupKeeper
		t.Errorf("expected LastEpochHeight=1 after baseline, got %d", state.LastEpochHeight)
	}
	// Epoch should not have advanced.
	if state.CurrentEpoch != 0 {
		t.Errorf("expected no epoch advance on baseline, got %d", state.CurrentEpoch)
	}
}

func TestCollectAndAdaptBeforeEpoch(t *testing.T) {
	k, mockSK, mockKK, _, ctx := setupKeeper(t)
	mockSK.activeValCount = 10
	mockKK.verificationRate = 500_000

	// Set baseline.
	k.CollectAndAdapt(ctx)
	initial, _ := k.GetMultiplierState(ctx, "rewards.block")

	// Advance only 50 blocks (less than epoch length 100).
	ctx = advanceBlocks(ctx, 50)
	k.CollectAndAdapt(ctx)

	after, _ := k.GetMultiplierState(ctx, "rewards.block")
	if after.CurrentBps != initial.CurrentBps {
		t.Error("multiplier should not change before epoch boundary")
	}
}

func TestCollectAndAdaptMultipleEpochs(t *testing.T) {
	k, mockSK, mockKK, _, ctx := setupKeeper(t)
	mockSK.activeValCount = 10
	mockKK.verificationRate = 500_000

	k.CollectAndAdapt(ctx) // baseline
	for i := 0; i < 3; i++ {
		ctx = advanceBlocks(ctx, 101)
		k.CollectAndAdapt(ctx)
	}

	state := k.GetState(ctx)
	if state.CurrentEpoch != 3 {
		t.Errorf("expected 3 epochs, got %d", state.CurrentEpoch)
	}
	// Verify snapshots were created for each epoch.
	for ep := uint64(1); ep <= 3; ep++ {
		_, found := k.GetEpochSnapshot(ctx, ep)
		if !found {
			t.Errorf("expected snapshot for epoch %d", ep)
		}
	}
}

func TestCollectAndAdaptSSIStored(t *testing.T) {
	k, _, _, _, ctx := setupKeeper(t)
	k.CollectAndAdapt(ctx) // baseline
	ctx = advanceBlocks(ctx, 101)
	k.CollectAndAdapt(ctx)

	ssi := k.GetSSI(ctx)
	if ssi == 0 {
		t.Error("expected non-zero SSI after epoch")
	}
}

func TestCollectAndAdaptSnapshotContent(t *testing.T) {
	k, mockSK, mockKK, _, ctx := setupKeeper(t)
	mockSK.activeValCount = 21
	mockKK.verificationRate = 1_000_000

	k.CollectAndAdapt(ctx) // baseline
	ctx = advanceBlocks(ctx, 101)
	k.CollectAndAdapt(ctx)

	snap, found := k.GetEpochSnapshot(ctx, 1)
	if !found {
		t.Fatal("expected epoch 1 snapshot")
	}
	if snap.SsiScore == 0 {
		t.Error("expected non-zero SSI in snapshot")
	}
	if snap.SsiCategory == "" {
		t.Error("expected non-empty SSI category")
	}
	if len(snap.Multipliers) != 3 {
		t.Errorf("expected 3 multipliers in snapshot, got %d", len(snap.Multipliers))
	}
}

// ========== Adversarial / Security ==========

func TestMultipleEpochsConvergeToTarget(t *testing.T) {
	k, mockSK, mockKK, _, ctx := setupKeeper(t)
	// Perfect signals → SSI = 1M → rewards target = 1.5M.
	mockSK.activeValCount = 21
	mockKK.verificationRate = 1_000_000

	k.CollectAndAdapt(ctx) // baseline
	for i := 0; i < 60; i++ {
		ctx = advanceBlocks(ctx, 101)
		k.CollectAndAdapt(ctx)
	}

	ms, _ := k.GetMultiplierState(ctx, "rewards.block")
	target := types.ComputeTarget(1_000_000, "rewards.block") // 1_500_000
	if ms.CurrentBps != target {
		t.Errorf("expected convergence to %d after 60 epochs, got %d", target, ms.CurrentBps)
	}
}

func TestFrozenSurvivesMultipleEpochs(t *testing.T) {
	k, mockSK, mockKK, _, ctx := setupKeeper(t)
	mockSK.activeValCount = 5
	mockKK.verificationRate = 200_000
	k.SetMultiplierFrozen(ctx, "fees.base", true)

	initial, _ := k.GetMultiplierState(ctx, "fees.base")

	k.CollectAndAdapt(ctx) // baseline
	for i := 0; i < 5; i++ {
		ctx = advanceBlocks(ctx, 101)
		k.CollectAndAdapt(ctx)
	}

	after, _ := k.GetMultiplierState(ctx, "fees.base")
	if after.CurrentBps != initial.CurrentBps {
		t.Errorf("frozen multiplier changed over 5 epochs: %d → %d", initial.CurrentBps, after.CurrentBps)
	}
}

func TestOverrideThenEpochAdjusts(t *testing.T) {
	k, mockSK, mockKK, _, ctx := setupKeeper(t)
	mockSK.activeValCount = 21
	mockKK.verificationRate = 1_000_000

	// Override to 800_000.
	msgServer := keeper.NewMsgServerImpl(k)
	_, _ = msgServer.OverrideMultiplier(ctx, &types.MsgOverrideMultiplier{
		Authority: "authority", Path: "rewards.block", Value: 800_000,
	})

	k.CollectAndAdapt(ctx) // baseline
	ctx = advanceBlocks(ctx, 101)
	k.CollectAndAdapt(ctx)

	ms, _ := k.GetMultiplierState(ctx, "rewards.block")
	// With SSI=1M, target is 1.5M. From 800k, should increase by max 10k.
	if ms.CurrentBps != 810_000 {
		t.Errorf("expected 810000 (800k + 10k max change), got %d", ms.CurrentBps)
	}
}

func TestActivateDeactivateReturnsDefault(t *testing.T) {
	k, _, _, _, ctx := setupKeeper(t)
	// Override to custom value.
	msgServer := keeper.NewMsgServerImpl(k)
	_, _ = msgServer.OverrideMultiplier(ctx, &types.MsgOverrideMultiplier{
		Authority: "authority", Path: "rewards.block", Value: 750_000,
	})
	// Deactivate.
	_, _ = msgServer.Activate(ctx, &types.MsgActivateAutopoiesis{
		Authority: "authority", Activate: false,
	})
	// GetMultiplier should return BPSScale when inactive.
	val, _ := k.GetMultiplier(ctx, "rewards.block")
	if val != types.BPSScale {
		t.Errorf("expected BPSScale when deactivated, got %d", val)
	}
}

func TestCollectAndAdaptNilStakingKeeper(t *testing.T) {
	k, ctx := setupBareKeeper(t)
	// Init genesis with activated=true.
	genState := types.DefaultGenesis()
	genState.Activated = true
	k.InitGenesis(ctx, genState)

	k.CollectAndAdapt(ctx) // baseline
	ctx = advanceBlocks(ctx, 101)
	k.CollectAndAdapt(ctx)

	// With nil staking/knowledge keepers, both assume BPSScale (healthy).
	// SSI = ComputeSSI(1M, 1M, false) = 1M.
	ssi := k.GetSSI(ctx)
	if ssi != 1_000_000 {
		t.Errorf("expected SSI=1000000 with nil keepers, got %d", ssi)
	}
}

func TestCollectAndAdaptZeroBondedStake(t *testing.T) {
	k, mockSK, mockKK, _, ctx := setupKeeper(t)
	mockSK.totalBonded = big.NewInt(0)
	mockSK.activeValCount = 0
	mockKK.verificationRate = 0

	k.CollectAndAdapt(ctx) // baseline
	ctx = advanceBlocks(ctx, 101)
	k.CollectAndAdapt(ctx)

	// SSI = (0 + 0 + 1M*20) / 100 = 200_000 (critical).
	ssi := k.GetSSI(ctx)
	if ssi != 200_000 {
		t.Errorf("expected SSI=200000 with zero signals, got %d", ssi)
	}
	snap, _ := k.GetEpochSnapshot(ctx, 1)
	if snap.SsiCategory != types.SSICritical {
		t.Errorf("expected critical category, got %s", snap.SsiCategory)
	}
}

func TestSlashingIncreasesUnderStress(t *testing.T) {
	k, mockSK, mockKK, _, ctx := setupKeeper(t)
	mockSK.activeValCount = 0
	mockKK.verificationRate = 0

	k.CollectAndAdapt(ctx) // baseline
	ctx = advanceBlocks(ctx, 101)
	k.CollectAndAdapt(ctx)

	ms, _ := k.GetMultiplierState(ctx, "slashing.severity")
	if ms.CurrentBps <= types.BPSScale {
		t.Errorf("expected slashing to increase under stress, got %d", ms.CurrentBps)
	}
}

func TestRewardsDecreaseUnderStress(t *testing.T) {
	k, mockSK, mockKK, _, ctx := setupKeeper(t)
	mockSK.activeValCount = 0
	mockKK.verificationRate = 0

	k.CollectAndAdapt(ctx) // baseline
	ctx = advanceBlocks(ctx, 101)
	k.CollectAndAdapt(ctx)

	ms, _ := k.GetMultiplierState(ctx, "rewards.block")
	if ms.CurrentBps >= types.BPSScale {
		t.Errorf("expected rewards to decrease under stress, got %d", ms.CurrentBps)
	}
}

func TestFeesIncreaseUnderStress(t *testing.T) {
	k, mockSK, mockKK, _, ctx := setupKeeper(t)
	mockSK.activeValCount = 0
	mockKK.verificationRate = 0

	k.CollectAndAdapt(ctx) // baseline
	ctx = advanceBlocks(ctx, 101)
	k.CollectAndAdapt(ctx)

	ms, _ := k.GetMultiplierState(ctx, "fees.base")
	if ms.CurrentBps <= types.BPSScale {
		t.Errorf("expected fees to increase under stress, got %d", ms.CurrentBps)
	}
}

// ========== ValidateBasic ==========

func TestMsgUpdateParamsValidateBasic(t *testing.T) {
	validAddr := testAddr("authority")
	tests := []struct {
		name    string
		makeMsg func() *types.MsgUpdateParams
		wantErr bool
	}{
		{"valid", func() *types.MsgUpdateParams {
			p := types.DefaultParams()
			return &types.MsgUpdateParams{Authority: validAddr, Params: &p}
		}, false},
		{"empty authority", func() *types.MsgUpdateParams {
			p := types.DefaultParams()
			return &types.MsgUpdateParams{Authority: "", Params: &p}
		}, true},
		{"invalid authority", func() *types.MsgUpdateParams {
			p := types.DefaultParams()
			return &types.MsgUpdateParams{Authority: "bad", Params: &p}
		}, true},
		{"nil params", func() *types.MsgUpdateParams {
			return &types.MsgUpdateParams{Authority: validAddr, Params: nil}
		}, true},
		{"invalid params", func() *types.MsgUpdateParams {
			p := types.DefaultParams()
			p.EpochLengthBlocks = 0
			return &types.MsgUpdateParams{Authority: validAddr, Params: &p}
		}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.makeMsg().ValidateBasic()
			if (err != nil) != tc.wantErr {
				t.Errorf("ValidateBasic() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestMsgActivateValidateBasic(t *testing.T) {
	validAddr := testAddr("authority")
	tests := []struct {
		name    string
		makeMsg func() *types.MsgActivateAutopoiesis
		wantErr bool
	}{
		{"valid", func() *types.MsgActivateAutopoiesis {
			return &types.MsgActivateAutopoiesis{Authority: validAddr, Activate: true}
		}, false},
		{"empty authority", func() *types.MsgActivateAutopoiesis {
			return &types.MsgActivateAutopoiesis{Authority: "", Activate: true}
		}, true},
		{"invalid authority", func() *types.MsgActivateAutopoiesis {
			return &types.MsgActivateAutopoiesis{Authority: "bad", Activate: true}
		}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.makeMsg().ValidateBasic()
			if (err != nil) != tc.wantErr {
				t.Errorf("ValidateBasic() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestMsgOverrideMultiplierValidateBasic(t *testing.T) {
	validAddr := testAddr("authority")
	tests := []struct {
		name    string
		makeMsg func() *types.MsgOverrideMultiplier
		wantErr bool
	}{
		{"valid", func() *types.MsgOverrideMultiplier {
			return &types.MsgOverrideMultiplier{Authority: validAddr, Path: "rewards.block", Value: 750_000}
		}, false},
		{"empty authority", func() *types.MsgOverrideMultiplier {
			return &types.MsgOverrideMultiplier{Authority: "", Path: "rewards.block", Value: 750_000}
		}, true},
		{"invalid authority", func() *types.MsgOverrideMultiplier {
			return &types.MsgOverrideMultiplier{Authority: "bad", Path: "rewards.block", Value: 750_000}
		}, true},
		{"invalid path", func() *types.MsgOverrideMultiplier {
			return &types.MsgOverrideMultiplier{Authority: validAddr, Path: "bad.path", Value: 750_000}
		}, true},
		{"zero value", func() *types.MsgOverrideMultiplier {
			return &types.MsgOverrideMultiplier{Authority: validAddr, Path: "rewards.block", Value: 0}
		}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.makeMsg().ValidateBasic()
			if (err != nil) != tc.wantErr {
				t.Errorf("ValidateBasic() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestMsgFreezeMultiplierValidateBasic(t *testing.T) {
	validAddr := testAddr("authority")
	tests := []struct {
		name    string
		makeMsg func() *types.MsgFreezeMultiplier
		wantErr bool
	}{
		{"valid", func() *types.MsgFreezeMultiplier {
			return &types.MsgFreezeMultiplier{Authority: validAddr, Path: "rewards.block", Frozen: true}
		}, false},
		{"empty authority", func() *types.MsgFreezeMultiplier {
			return &types.MsgFreezeMultiplier{Authority: "", Path: "rewards.block", Frozen: true}
		}, true},
		{"invalid authority", func() *types.MsgFreezeMultiplier {
			return &types.MsgFreezeMultiplier{Authority: "bad", Path: "rewards.block", Frozen: true}
		}, true},
		{"invalid path", func() *types.MsgFreezeMultiplier {
			return &types.MsgFreezeMultiplier{Authority: validAddr, Path: "bad.path", Frozen: true}
		}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.makeMsg().ValidateBasic()
			if (err != nil) != tc.wantErr {
				t.Errorf("ValidateBasic() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

// ========== Full Lifecycle ==========

func TestFullAutopoiesisLifecycle(t *testing.T) {
	k, mockSK, mockKK, mockEK, ctx := setupKeeper(t)
	msgServer := keeper.NewMsgServerImpl(k)

	// 1. Start with healthy signals — run 3 epochs.
	mockSK.activeValCount = 21
	mockKK.verificationRate = 1_000_000
	k.CollectAndAdapt(ctx) // baseline
	for i := 0; i < 3; i++ {
		ctx = advanceBlocks(ctx, 101)
		k.CollectAndAdapt(ctx)
	}
	ms, _ := k.GetMultiplierState(ctx, "rewards.block")
	if ms.CurrentBps <= types.BPSScale {
		t.Error("expected rewards to increase with healthy signals")
	}

	// 2. Override slashing.
	_, err := msgServer.OverrideMultiplier(ctx, &types.MsgOverrideMultiplier{
		Authority: "authority", Path: "slashing.severity", Value: 1_500_000,
	})
	if err != nil {
		t.Fatalf("override failed: %v", err)
	}

	// 3. Freeze fees.
	_, err = msgServer.FreezeMultiplier(ctx, &types.MsgFreezeMultiplier{
		Authority: "authority", Path: "fees.base", Frozen: true,
	})
	if err != nil {
		t.Fatalf("freeze failed: %v", err)
	}
	feesBefore, _ := k.GetMultiplierState(ctx, "fees.base")

	// 4. Degrade signals and run more epochs.
	mockSK.activeValCount = 5
	mockKK.verificationRate = 200_000
	for i := 0; i < 3; i++ {
		ctx = advanceBlocks(ctx, 101)
		k.CollectAndAdapt(ctx)
	}

	// Fees should not change (frozen).
	feesAfter, _ := k.GetMultiplierState(ctx, "fees.base")
	if feesAfter.CurrentBps != feesBefore.CurrentBps {
		t.Errorf("frozen fees changed: %d → %d", feesBefore.CurrentBps, feesAfter.CurrentBps)
	}

	// Slashing should adjust from the override value.
	slashMs, _ := k.GetMultiplierState(ctx, "slashing.severity")
	if slashMs.CurrentBps == 1_500_000 {
		t.Error("expected slashing to adjust from override value")
	}

	// 5. Emergency halt stops adjustment.
	mockEK.halted = true
	rewardsBefore, _ := k.GetMultiplierState(ctx, "rewards.block")
	ctx = advanceBlocks(ctx, 101)
	k.CollectAndAdapt(ctx)
	rewardsAfter, _ := k.GetMultiplierState(ctx, "rewards.block")
	if rewardsAfter.CurrentBps != rewardsBefore.CurrentBps {
		t.Error("expected no change during emergency halt")
	}

	// 6. Export and validate.
	exported := k.ExportGenesis(ctx)
	if err := exported.Validate(); err != nil {
		t.Fatalf("exported genesis invalid: %v", err)
	}
	if len(exported.Snapshots) < 6 {
		t.Errorf("expected at least 6 snapshots, got %d", len(exported.Snapshots))
	}
}
