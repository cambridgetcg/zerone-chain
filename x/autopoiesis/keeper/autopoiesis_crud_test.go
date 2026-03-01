package keeper_test

import (
	"testing"
	"time"

	"cosmossdk.io/log"
	"cosmossdk.io/store"
	storemetrics "cosmossdk.io/store/metrics"
	storetypes "cosmossdk.io/store/types"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/autopoiesis/keeper"
	"github.com/zerone-chain/zerone/x/autopoiesis/types"
)

// setupBareKeeper creates a keeper without InitGenesis for testing default/empty state.
func setupBareKeeper(t *testing.T) (keeper.Keeper, sdk.Context) {
	t.Helper()
	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewNopLogger(), storemetrics.NewNoOpMetrics())
	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	if err := stateStore.LoadLatestVersion(); err != nil {
		t.Fatalf("failed to load: %v", err)
	}
	ctx := sdk.NewContext(stateStore, cmtproto.Header{Height: 1}, false, log.NewNopLogger()).
		WithBlockTime(time.Now())
	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)
	k := keeper.NewKeeper(runtime.NewKVStoreService(storeKey), cdc, "authority", nil)
	return k, ctx
}

// ========== Params CRUD ==========

func TestSetGetParamsRoundTrip(t *testing.T) {
	k, _, _, _, ctx := setupKeeper(t)
	p := types.DefaultParams()
	p.EpochLengthBlocks = 200
	p.MaxChangePerEpochBps = 50_000
	k.SetParams(ctx, &p)

	got := k.GetParams(ctx)
	if got.EpochLengthBlocks != 200 {
		t.Errorf("EpochLengthBlocks: expected 200, got %d", got.EpochLengthBlocks)
	}
	if got.MaxChangePerEpochBps != 50_000 {
		t.Errorf("MaxChangePerEpochBps: expected 50000, got %d", got.MaxChangePerEpochBps)
	}
}

func TestParamsValidation(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*types.Params)
		wantErr bool
	}{
		{"valid defaults", func(p *types.Params) {}, false},
		{"epoch length zero", func(p *types.Params) { p.EpochLengthBlocks = 0 }, true},
		{"max change exceeds BPS", func(p *types.Params) { p.MaxChangePerEpochBps = types.BPSScale + 1 }, true},
		{"slash min > max", func(p *types.Params) {
			p.SlashMultiplierMin = 2_000_001
			p.SlashMultiplierMax = 2_000_000
		}, true},
		{"critical > stressed", func(p *types.Params) {
			p.SsiCriticalThreshold = 500_001
			p.SsiStressedThreshold = 500_000
		}, true},
		{"stressed > healthy", func(p *types.Params) {
			p.SsiStressedThreshold = 750_001
			p.SsiHealthyThreshold = 750_000
		}, true},
		{"healthy > BPS scale", func(p *types.Params) {
			p.SsiHealthyThreshold = types.BPSScale + 1
		}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := types.DefaultParams()
			tc.modify(&p)
			err := p.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestGetParamsDefaultWhenUnset(t *testing.T) {
	k, ctx := setupBareKeeper(t)
	got := k.GetParams(ctx)
	def := types.DefaultParams()
	if got.EpochLengthBlocks != def.EpochLengthBlocks {
		t.Errorf("expected default EpochLengthBlocks %d, got %d", def.EpochLengthBlocks, got.EpochLengthBlocks)
	}
	if got.MaxChangePerEpochBps != def.MaxChangePerEpochBps {
		t.Errorf("expected default MaxChangePerEpochBps %d, got %d", def.MaxChangePerEpochBps, got.MaxChangePerEpochBps)
	}
}

// ========== State CRUD ==========

func TestSetGetState(t *testing.T) {
	k, _, _, _, ctx := setupKeeper(t)
	state := &types.AutopoiesisState{
		Activated:       true,
		CurrentEpoch:    5,
		LastEpochHeight: 500,
	}
	k.SetState(ctx, state)
	got := k.GetState(ctx)
	if got.Activated != true {
		t.Error("expected Activated=true")
	}
	if got.CurrentEpoch != 5 {
		t.Errorf("expected CurrentEpoch=5, got %d", got.CurrentEpoch)
	}
	if got.LastEpochHeight != 500 {
		t.Errorf("expected LastEpochHeight=500, got %d", got.LastEpochHeight)
	}
}

func TestGetStateDefaultNotActivated(t *testing.T) {
	k, ctx := setupBareKeeper(t)
	got := k.GetState(ctx)
	if got.Activated {
		t.Error("expected default state to be not activated")
	}
	if got.CurrentEpoch != 0 {
		t.Errorf("expected default CurrentEpoch=0, got %d", got.CurrentEpoch)
	}
}

func TestIsActive(t *testing.T) {
	k, _, _, _, ctx := setupKeeper(t)
	if !k.IsActive(ctx) {
		t.Error("expected IsActive=true after setupKeeper (genesis activated)")
	}
	state := k.GetState(ctx)
	state.Activated = false
	k.SetState(ctx, state)
	if k.IsActive(ctx) {
		t.Error("expected IsActive=false after deactivation")
	}
}

// ========== MultiplierState CRUD ==========

func TestSetGetMultiplierState(t *testing.T) {
	k, _, _, _, ctx := setupKeeper(t)
	ms := &types.MultiplierState{
		Path:       "test.path",
		CurrentBps: 800_000,
		TargetBps:  900_000,
		MinBps:     500_000,
		MaxBps:     1_500_000,
		Frozen:     false,
	}
	k.SetMultiplierState(ctx, ms)
	got, found := k.GetMultiplierState(ctx, "test.path")
	if !found {
		t.Fatal("expected to find test.path")
	}
	if got.CurrentBps != 800_000 {
		t.Errorf("CurrentBps: expected 800000, got %d", got.CurrentBps)
	}
	if got.TargetBps != 900_000 {
		t.Errorf("TargetBps: expected 900000, got %d", got.TargetBps)
	}
}

func TestGetMultiplierStateNotFound(t *testing.T) {
	k, _, _, _, ctx := setupKeeper(t)
	_, found := k.GetMultiplierState(ctx, "nonexistent.path")
	if found {
		t.Error("expected not found for nonexistent path")
	}
}

func TestSetMultiplierStateOverwrite(t *testing.T) {
	k, _, _, _, ctx := setupKeeper(t)
	ms := &types.MultiplierState{Path: "rewards.block", CurrentBps: 800_000, MinBps: 500_000, MaxBps: 2_000_000}
	k.SetMultiplierState(ctx, ms)
	got, _ := k.GetMultiplierState(ctx, "rewards.block")
	if got.CurrentBps != 800_000 {
		t.Errorf("expected overwritten CurrentBps=800000, got %d", got.CurrentBps)
	}
}

func TestGetAllMultipliersDefault(t *testing.T) {
	k, _, _, _, ctx := setupKeeper(t)
	all := k.GetAllMultipliers(ctx)
	if len(all) != 3 {
		t.Fatalf("expected 3 default multipliers, got %d", len(all))
	}
	paths := map[string]bool{}
	for _, m := range all {
		paths[m.Path] = true
	}
	for _, p := range []string{"rewards.block", "slashing.severity", "fees.base"} {
		if !paths[p] {
			t.Errorf("missing default multiplier path: %s", p)
		}
	}
}

func TestGetAllMultipliersEmpty(t *testing.T) {
	k, ctx := setupBareKeeper(t)
	all := k.GetAllMultipliers(ctx)
	if len(all) != 0 {
		t.Errorf("expected 0 multipliers on bare keeper, got %d", len(all))
	}
}

// ========== Frozen CRUD ==========

func TestSetGetMultiplierFrozen(t *testing.T) {
	k, _, _, _, ctx := setupKeeper(t)
	k.SetMultiplierFrozen(ctx, "rewards.block", true)
	if !k.IsMultiplierFrozen(ctx, "rewards.block") {
		t.Error("expected rewards.block to be frozen")
	}
}

func TestIsMultiplierFrozenDefault(t *testing.T) {
	k, _, _, _, ctx := setupKeeper(t)
	if k.IsMultiplierFrozen(ctx, "rewards.block") {
		t.Error("expected rewards.block to not be frozen by default")
	}
}

func TestFreezeThenUnfreeze(t *testing.T) {
	k, _, _, _, ctx := setupKeeper(t)
	k.SetMultiplierFrozen(ctx, "fees.base", true)
	if !k.IsMultiplierFrozen(ctx, "fees.base") {
		t.Fatal("expected frozen")
	}
	k.SetMultiplierFrozen(ctx, "fees.base", false)
	if k.IsMultiplierFrozen(ctx, "fees.base") {
		t.Error("expected unfrozen after toggle")
	}
}

// ========== EpochSnapshot CRUD ==========

func TestSetGetEpochSnapshot(t *testing.T) {
	k, _, _, _, ctx := setupKeeper(t)
	snap := &types.EpochSnapshot{
		Epoch:       1,
		BlockHeight: 101,
		SsiScore:    800_000,
		SsiCategory: types.SSIThriving,
	}
	k.SetEpochSnapshot(ctx, snap)
	got, found := k.GetEpochSnapshot(ctx, 1)
	if !found {
		t.Fatal("expected to find epoch 1 snapshot")
	}
	if got.SsiScore != 800_000 {
		t.Errorf("SsiScore: expected 800000, got %d", got.SsiScore)
	}
	if got.SsiCategory != types.SSIThriving {
		t.Errorf("SsiCategory: expected %s, got %s", types.SSIThriving, got.SsiCategory)
	}
}

func TestGetEpochSnapshotNotFound(t *testing.T) {
	k, _, _, _, ctx := setupKeeper(t)
	_, found := k.GetEpochSnapshot(ctx, 999)
	if found {
		t.Error("expected not found for nonexistent epoch")
	}
}

func TestGetAllSnapshotsMultiple(t *testing.T) {
	k, _, _, _, ctx := setupKeeper(t)
	for i := uint64(1); i <= 3; i++ {
		k.SetEpochSnapshot(ctx, &types.EpochSnapshot{Epoch: i, BlockHeight: i * 100, SsiScore: i * 100_000})
	}
	all := k.GetAllSnapshots(ctx)
	if len(all) != 3 {
		t.Fatalf("expected 3 snapshots, got %d", len(all))
	}
	// Verify big-endian ordering (epochs should be in order).
	for i, s := range all {
		if s.Epoch != uint64(i+1) {
			t.Errorf("snapshot[%d]: expected epoch %d, got %d", i, i+1, s.Epoch)
		}
	}
}

func TestGetAllSnapshotsEmpty(t *testing.T) {
	k, _, _, _, ctx := setupKeeper(t)
	all := k.GetAllSnapshots(ctx)
	if len(all) != 0 {
		t.Errorf("expected 0 snapshots initially, got %d", len(all))
	}
}

// ========== SSI CRUD ==========

func TestSetGetSSI(t *testing.T) {
	k, _, _, _, ctx := setupKeeper(t)
	k.SetSSI(ctx, 750_000)
	got := k.GetSSI(ctx)
	if got != 750_000 {
		t.Errorf("expected SSI=750000, got %d", got)
	}
}

func TestGetSSIDefaultZero(t *testing.T) {
	k, _, _, _, ctx := setupKeeper(t)
	got := k.GetSSI(ctx)
	if got != 0 {
		t.Errorf("expected default SSI=0, got %d", got)
	}
}

// ========== ComputeSSI Pure Function ==========

func TestComputeSSIExactValues(t *testing.T) {
	tests := []struct {
		name    string
		staking uint64
		verif   uint64
		halted  bool
		want    uint64
	}{
		{"all max not halted", 1_000_000, 1_000_000, false, 1_000_000},
		{"half signals not halted", 500_000, 500_000, false, 600_000},
		{"zero signals not halted", 0, 0, false, 200_000},
		{"zero signals halted", 0, 0, true, 0},
		{"all max halted", 1_000_000, 1_000_000, true, 800_000},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := types.ComputeSSI(tc.staking, tc.verif, tc.halted)
			if got != tc.want {
				t.Errorf("ComputeSSI(%d, %d, %v) = %d, want %d", tc.staking, tc.verif, tc.halted, got, tc.want)
			}
		})
	}
}

func TestComputeSSICapsInputs(t *testing.T) {
	// Inputs above BPSScale should be capped.
	got := types.ComputeSSI(2_000_000, 2_000_000, false)
	if got != 1_000_000 {
		t.Errorf("expected capped SSI=1000000, got %d", got)
	}
}

func TestComputeSSIAsymmetricSignals(t *testing.T) {
	// 100% staking, 0% verification, not halted.
	// (1M*40 + 0*40 + 1M*20) / 100 = 60M/100 = 600_000
	got := types.ComputeSSI(1_000_000, 0, false)
	if got != 600_000 {
		t.Errorf("expected 600000, got %d", got)
	}
}

// ========== ClassifySSI Pure Function ==========

func TestClassifySSICategories(t *testing.T) {
	params := types.DefaultParams()
	tests := []struct {
		name string
		ssi  uint64
		want string
	}{
		{"critical", 100_000, types.SSICritical},
		{"stressed", 400_000, types.SSIStressed},
		{"healthy", 600_000, types.SSIHealthy},
		{"thriving", 900_000, types.SSIThriving},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := types.ClassifySSI(tc.ssi, &params)
			if got != tc.want {
				t.Errorf("ClassifySSI(%d) = %s, want %s", tc.ssi, got, tc.want)
			}
		})
	}
}

func TestClassifySSIExactBoundaries(t *testing.T) {
	params := types.DefaultParams() // critical=250k, stressed=500k, healthy=750k
	tests := []struct {
		ssi  uint64
		want string
	}{
		{0, types.SSICritical},
		{249_999, types.SSICritical},
		{250_000, types.SSIStressed},
		{499_999, types.SSIStressed},
		{500_000, types.SSIHealthy},
		{749_999, types.SSIHealthy},
		{750_000, types.SSIThriving},
		{1_000_000, types.SSIThriving},
	}
	for _, tc := range tests {
		got := types.ClassifySSI(tc.ssi, &params)
		if got != tc.want {
			t.Errorf("ClassifySSI(%d) = %s, want %s", tc.ssi, got, tc.want)
		}
	}
}

// ========== ComputeTarget Pure Function ==========

func TestComputeTargetRewardsBlock(t *testing.T) {
	// rewards.block: 500_000 + SSI
	if got := types.ComputeTarget(0, "rewards.block"); got != 500_000 {
		t.Errorf("SSI=0: expected 500000, got %d", got)
	}
	if got := types.ComputeTarget(1_000_000, "rewards.block"); got != 1_500_000 {
		t.Errorf("SSI=1M: expected 1500000, got %d", got)
	}
	if got := types.ComputeTarget(500_000, "rewards.block"); got != 1_000_000 {
		t.Errorf("SSI=500k: expected 1000000, got %d", got)
	}
}

func TestComputeTargetSlashingSeverity(t *testing.T) {
	// slashing.severity: 2_000_000 - (SSI * 3 / 2)
	if got := types.ComputeTarget(0, "slashing.severity"); got != 2_000_000 {
		t.Errorf("SSI=0: expected 2000000, got %d", got)
	}
	if got := types.ComputeTarget(1_000_000, "slashing.severity"); got != 500_000 {
		t.Errorf("SSI=1M: expected 500000, got %d", got)
	}
}

func TestComputeTargetFeesBase(t *testing.T) {
	// fees.base: same formula as slashing.severity
	if got := types.ComputeTarget(0, "fees.base"); got != 2_000_000 {
		t.Errorf("SSI=0: expected 2000000, got %d", got)
	}
	if got := types.ComputeTarget(1_000_000, "fees.base"); got != 500_000 {
		t.Errorf("SSI=1M: expected 500000, got %d", got)
	}
}

func TestComputeTargetUnknownPath(t *testing.T) {
	got := types.ComputeTarget(500_000, "unknown.path")
	if got != types.BPSScale {
		t.Errorf("expected BPSScale for unknown path, got %d", got)
	}
}

// ========== GetMultiplier Keeper ==========

func TestGetMultiplierWhenInactive(t *testing.T) {
	k, _, _, _, ctx := setupKeeper(t)
	state := k.GetState(ctx)
	state.Activated = false
	k.SetState(ctx, state)

	val, err := k.GetMultiplier(ctx, "rewards.block")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != types.BPSScale {
		t.Errorf("expected BPSScale when inactive, got %d", val)
	}
}

func TestGetMultiplierPathNotFound(t *testing.T) {
	k, _, _, _, ctx := setupKeeper(t)
	val, err := k.GetMultiplier(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != types.BPSScale {
		t.Errorf("expected BPSScale for missing path, got %d", val)
	}
}

func TestGetMultiplierReturnsCurrentBps(t *testing.T) {
	k, _, _, _, ctx := setupKeeper(t)
	ms := &types.MultiplierState{Path: "rewards.block", CurrentBps: 750_000, MinBps: 500_000, MaxBps: 2_000_000}
	k.SetMultiplierState(ctx, ms)

	val, err := k.GetMultiplier(ctx, "rewards.block")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != 750_000 {
		t.Errorf("expected 750000, got %d", val)
	}
}

// ========== SuggestAdjustment ==========

func TestSuggestAdjustmentReturnsNil(t *testing.T) {
	k, _, _, _, ctx := setupKeeper(t)
	err := k.SuggestAdjustment(ctx, "rewards.block", "increase", 10_000)
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

// ========== Genesis Validation ==========

func TestDefaultGenesisValid(t *testing.T) {
	gs := types.DefaultGenesis()
	if err := gs.Validate(); err != nil {
		t.Errorf("DefaultGenesis should be valid: %v", err)
	}
}

func TestDefaultMultipliersCount(t *testing.T) {
	mults := types.DefaultMultipliers()
	if len(mults) != 3 {
		t.Fatalf("expected 3 default multipliers, got %d", len(mults))
	}
	for _, m := range mults {
		if m.CurrentBps != types.BPSScale {
			t.Errorf("path %s: expected CurrentBps=%d, got %d", m.Path, types.BPSScale, m.CurrentBps)
		}
	}
}

func TestGenesisValidateNilParams(t *testing.T) {
	gs := types.DefaultGenesis()
	gs.Params = nil
	if err := gs.Validate(); err == nil {
		t.Error("expected error for nil params")
	}
}

func TestGenesisValidateEmptyPath(t *testing.T) {
	gs := types.DefaultGenesis()
	gs.Multipliers = append(gs.Multipliers, &types.MultiplierState{
		Path: "", CurrentBps: types.BPSScale, MinBps: 500_000, MaxBps: 2_000_000,
	})
	if err := gs.Validate(); err == nil {
		t.Error("expected error for empty path")
	}
}

func TestGenesisValidateDuplicatePath(t *testing.T) {
	gs := types.DefaultGenesis()
	gs.Multipliers = append(gs.Multipliers, &types.MultiplierState{
		Path: "rewards.block", CurrentBps: types.BPSScale, MinBps: 500_000, MaxBps: 2_000_000,
	})
	if err := gs.Validate(); err == nil {
		t.Error("expected error for duplicate path")
	}
}

func TestGenesisValidateMinGtMax(t *testing.T) {
	gs := &types.GenesisState{
		Params: func() *types.Params { p := types.DefaultParams(); return &p }(),
		Multipliers: []*types.MultiplierState{
			{Path: "test", CurrentBps: 1_000_000, MinBps: 2_000_000, MaxBps: 500_000},
		},
	}
	if err := gs.Validate(); err == nil {
		t.Error("expected error for min > max")
	}
}

func TestGenesisValidateCurrentOutOfBounds(t *testing.T) {
	gs := &types.GenesisState{
		Params: func() *types.Params { p := types.DefaultParams(); return &p }(),
		Multipliers: []*types.MultiplierState{
			{Path: "test", CurrentBps: 100_000, MinBps: 500_000, MaxBps: 2_000_000},
		},
	}
	if err := gs.Validate(); err == nil {
		t.Error("expected error for current_bps below min_bps")
	}
}

func TestGenesisValidateCurrentAboveMax(t *testing.T) {
	gs := &types.GenesisState{
		Params: func() *types.Params { p := types.DefaultParams(); return &p }(),
		Multipliers: []*types.MultiplierState{
			{Path: "test", CurrentBps: 3_000_000, MinBps: 500_000, MaxBps: 2_000_000},
		},
	}
	if err := gs.Validate(); err == nil {
		t.Error("expected error for current_bps above max_bps")
	}
}

// ========== Params Stored After Update ==========

func TestParamsRoundTripAllFields(t *testing.T) {
	k, _, _, _, ctx := setupKeeper(t)
	p := &types.Params{
		EpochLengthBlocks:    50,
		MaxChangePerEpochBps: 20_000,
		SlashMultiplierMin:   300_000,
		SlashMultiplierMax:   1_800_000,
		SsiCriticalThreshold: 200_000,
		SsiStressedThreshold: 400_000,
		SsiHealthyThreshold:  600_000,
		Enabled:              false,
	}
	k.SetParams(ctx, p)
	got := k.GetParams(ctx)
	if got.EpochLengthBlocks != 50 {
		t.Errorf("EpochLengthBlocks: got %d", got.EpochLengthBlocks)
	}
	if got.SlashMultiplierMin != 300_000 {
		t.Errorf("SlashMultiplierMin: got %d", got.SlashMultiplierMin)
	}
	if got.Enabled != false {
		t.Error("expected Enabled=false")
	}
}

// ========== Snapshot With Multipliers ==========

func TestEpochSnapshotWithMultipliers(t *testing.T) {
	k, _, _, _, ctx := setupKeeper(t)
	snap := &types.EpochSnapshot{
		Epoch:       2,
		BlockHeight: 201,
		Multipliers: []*types.MultiplierState{
			{Path: "rewards.block", CurrentBps: 1_010_000},
			{Path: "slashing.severity", CurrentBps: 990_000},
		},
		SsiScore:    900_000,
		SsiCategory: types.SSIThriving,
	}
	k.SetEpochSnapshot(ctx, snap)
	got, found := k.GetEpochSnapshot(ctx, 2)
	if !found {
		t.Fatal("expected to find epoch 2 snapshot")
	}
	if len(got.Multipliers) != 2 {
		t.Fatalf("expected 2 multipliers in snapshot, got %d", len(got.Multipliers))
	}
	if got.Multipliers[0].CurrentBps != 1_010_000 {
		t.Errorf("expected first multiplier 1010000, got %d", got.Multipliers[0].CurrentBps)
	}
}

// ========== SSI Overwrite ==========

func TestSSIOverwrite(t *testing.T) {
	k, _, _, _, ctx := setupKeeper(t)
	k.SetSSI(ctx, 500_000)
	k.SetSSI(ctx, 750_000)
	got := k.GetSSI(ctx)
	if got != 750_000 {
		t.Errorf("expected SSI=750000 after overwrite, got %d", got)
	}
}
