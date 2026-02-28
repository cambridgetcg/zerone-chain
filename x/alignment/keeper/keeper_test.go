package keeper_test

import (
	"context"
	"math/big"
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

	alignment "github.com/zerone-chain/zerone/x/alignment"
	"github.com/zerone-chain/zerone/x/alignment/keeper"
	"github.com/zerone-chain/zerone/x/alignment/types"
)

// --- Mock Keepers ---

type mockKnowledgeKeeper struct {
	verificationRate    uint64
	totalFacts          uint64
	consensusDiversity  uint64
}

func (m *mockKnowledgeKeeper) GetVerificationRate(_ context.Context) uint64 {
	return m.verificationRate
}

func (m *mockKnowledgeKeeper) GetTotalFacts(_ context.Context) uint64 {
	return m.totalFacts
}

func (m *mockKnowledgeKeeper) GetConsensusDiversity(_ context.Context) uint64 {
	return m.consensusDiversity
}

type mockStakingKeeper struct {
	totalStaked         *big.Int
	activeValidators    uint64
	targetValidators    uint64
}

func (m *mockStakingKeeper) GetTotalStaked(_ context.Context) *big.Int {
	if m.totalStaked == nil {
		return new(big.Int)
	}
	return m.totalStaked
}

func (m *mockStakingKeeper) GetActiveValidatorCount(_ context.Context) uint64 {
	return m.activeValidators
}

func (m *mockStakingKeeper) GetTargetValidatorCount(_ context.Context) uint64 {
	return m.targetValidators
}

type mockOntologyKeeper struct {
	domainCount uint64
}

func (m *mockOntologyKeeper) GetDomainCount(_ context.Context) uint64 {
	return m.domainCount
}

type mockEmergencyKeeper struct {
	halted bool
}

func (m *mockEmergencyKeeper) IsHalted(_ context.Context) bool {
	return m.halted
}

type mockVestingRewardsKeeper struct {
	totalSupply *big.Int
}

func (m *mockVestingRewardsKeeper) GetTotalSupply(_ context.Context) *big.Int {
	if m.totalSupply == nil {
		return new(big.Int)
	}
	return m.totalSupply
}

type mockAutopoiesisKeeper struct {
	adjustments []adjustmentRecord
}

type adjustmentRecord struct {
	parameter string
	direction string
	magnitude uint64
}

func (m *mockAutopoiesisKeeper) SuggestAdjustment(_ context.Context, parameter, direction string, magnitude uint64) error {
	m.adjustments = append(m.adjustments, adjustmentRecord{parameter, direction, magnitude})
	return nil
}

// --- Test Setup ---

type testKeepers struct {
	knowledge      *mockKnowledgeKeeper
	staking        *mockStakingKeeper
	ontology       *mockOntologyKeeper
	emergency      *mockEmergencyKeeper
	vestingRewards *mockVestingRewardsKeeper
	autopoiesis    *mockAutopoiesisKeeper
}

func setupKeeper(t *testing.T) (keeper.Keeper, testKeepers, sdk.Context) {
	t.Helper()

	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewNopLogger(), storemetrics.NewNoOpMetrics())
	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	if err := stateStore.LoadLatestVersion(); err != nil {
		t.Fatalf("failed to load latest version: %v", err)
	}

	ctx := sdk.NewContext(stateStore, cmtproto.Header{Height: 100}, false, log.NewNopLogger()).
		WithBlockTime(time.Now())

	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)

	mocks := testKeepers{
		knowledge: &mockKnowledgeKeeper{
			verificationRate:   700_000, // 70%
			totalFacts:         1000,
			consensusDiversity: 500_000, // 50% — neutral default
		},
		staking: &mockStakingKeeper{
			totalStaked:      big.NewInt(600_000_000_000), // 600k ZRN
			activeValidators: 80,
			targetValidators: 111,
		},
		ontology: &mockOntologyKeeper{
			domainCount: 50,
		},
		emergency: &mockEmergencyKeeper{
			halted: false,
		},
		vestingRewards: &mockVestingRewardsKeeper{
			totalSupply: big.NewInt(1_000_000_000_000), // 1M ZRN
		},
		autopoiesis: nil, // nil by default (not wired)
	}

	k := keeper.NewKeeper(
		runtime.NewKVStoreService(storeKey),
		cdc,
		"authority",
		mocks.knowledge,
		mocks.staking,
		mocks.ontology,
		mocks.emergency,
		mocks.vestingRewards,
	)

	// Set default params.
	params := types.DefaultParams()
	k.SetParams(ctx, &params)

	// Set default state.
	k.SetState(ctx, &types.AlignmentState{Enabled: true})

	return k, mocks, ctx
}

// --- Test 1: Sensor readings from mock keepers produce correct observations ---

func TestSensorReadings(t *testing.T) {
	k, _, ctx := setupKeeper(t)

	obs := k.ObserveAll(ctx)

	// Knowledge: (700k*6 + 500k*4) / 10 = 620k (60% verification rate + 40% diversity)
	if obs.KnowledgeQuality != 620_000 {
		t.Fatalf("expected knowledge=620000, got %d", obs.KnowledgeQuality)
	}
	// Economic: staked/supply = 600k/1M = 60% = 600,000 BPS
	if obs.EconomicStability != 600_000 {
		t.Fatalf("expected economic=600000, got %d", obs.EconomicStability)
	}
	// Governance: 50 domains / 100 target = 50% = 500,000 BPS
	if obs.GovernanceParticipation != 500_000 {
		t.Fatalf("expected governance=500000, got %d", obs.GovernanceParticipation)
	}
	// Security: 80 active / 111 target ≈ 72% ≈ 720,720 BPS
	expectedSecurity := uint64(80) * types.BPS / uint64(111)
	if obs.NetworkSecurity != expectedSecurity {
		t.Fatalf("expected security=%d, got %d", expectedSecurity, obs.NetworkSecurity)
	}
	// Staking: same as economic = 600,000 BPS
	if obs.StakingRatio != 600_000 {
		t.Fatalf("expected staking=600000, got %d", obs.StakingRatio)
	}
}

// --- Test 2: Weighted scoring produces correct composite AHI ---

func TestWeightedScoring(t *testing.T) {
	k, _, ctx := setupKeeper(t)

	obs := k.ObserveAll(ctx)
	scores := k.ComputeScores(ctx, obs)

	// Weighted composite: (dim * weight) / BPS, summed over all 5 dims
	// With equal weights (200k each):
	// composite = (700k*200k + 600k*200k + 500k*200k + ~720k*200k + 600k*200k) / 1M
	// = (140B + 120B + 100B + ~144B + 120B) / 1M ≈ 624,144
	securityBPS := uint64(80) * types.BPS / 111
	expected := (obs.KnowledgeQuality*200_000 +
		obs.EconomicStability*200_000 +
		obs.GovernanceParticipation*200_000 +
		securityBPS*200_000 +
		obs.StakingRatio*200_000) / types.BPS

	if scores.Composite != expected {
		t.Fatalf("expected composite=%d, got %d", expected, scores.Composite)
	}

	// Category should be healthy (composite > 700k threshold? Actually 624k < 700k = degraded)
	category := k.CategorizeHealth(ctx, scores.Composite)
	if category != types.CategoryDegraded {
		t.Fatalf("expected degraded category, got %s (composite=%d)", category, scores.Composite)
	}
}

// --- Test 3: Corrections generated when dimensions below degraded threshold ---

func TestCorrectionsGenerated(t *testing.T) {
	k, mocks, ctx := setupKeeper(t)

	// Set low dimension values to trigger corrections.
	mocks.knowledge.verificationRate = 300_000 // below 400k degraded threshold
	mocks.staking.totalStaked = big.NewInt(200_000_000_000) // 200k/1M = 20% below degraded
	mocks.staking.activeValidators = 30                      // 30/111 ≈ 27% below degraded

	obs := k.ObserveAll(ctx)
	scores := k.ComputeScores(ctx, obs)
	corrections := k.GenerateCorrections(ctx, scores)

	if len(corrections) == 0 {
		t.Fatal("expected corrections for low dimensions")
	}

	// Should have corrections for knowledge, economic, security, staking (4).
	// Governance is log-only — no correction generated.
	foundDimensions := make(map[string]bool)
	for _, c := range corrections {
		foundDimensions[c.Dimension] = true
		if c.Applied {
			t.Fatalf("correction should not be applied (autopoiesis is nil): %s", c.Dimension)
		}
	}

	if !foundDimensions[types.DimKnowledgeQuality] {
		t.Error("expected correction for knowledge_quality")
	}
	if !foundDimensions[types.DimNetworkSecurity] {
		t.Error("expected correction for network_security")
	}
	if !foundDimensions[types.DimStakingRatio] {
		t.Error("expected correction for staking_ratio")
	}
}

// --- Test 4: No corrections when all dimensions healthy ---

func TestNoCorrectionsWhenHealthy(t *testing.T) {
	k, mocks, ctx := setupKeeper(t)

	// Set all dimensions to healthy values.
	mocks.knowledge.verificationRate = 800_000
	mocks.staking.totalStaked = big.NewInt(800_000_000_000)
	mocks.staking.activeValidators = 111
	mocks.ontology.domainCount = 100
	mocks.vestingRewards.totalSupply = big.NewInt(1_000_000_000_000)

	obs := k.ObserveAll(ctx)
	scores := k.ComputeScores(ctx, obs)
	corrections := k.GenerateCorrections(ctx, scores)

	if len(corrections) != 0 {
		t.Fatalf("expected no corrections when all healthy, got %d", len(corrections))
	}

	category := k.CategorizeHealth(ctx, scores.Composite)
	if category != types.CategoryHealthy {
		t.Fatalf("expected healthy category, got %s (composite=%d)", category, scores.Composite)
	}
}

// --- Test 5: Emergency halt disables observations ---

func TestEmergencyHaltDisablesObservations(t *testing.T) {
	k, mocks, ctx := setupKeeper(t)
	mocks.emergency.halted = true

	// Run EndBlock at an observation interval.
	ctx = ctx.WithBlockHeight(200) // 200 % 100 == 0

	am := alignment.NewAppModule(nil, k)
	if err := am.EndBlock(ctx); err != nil {
		t.Fatalf("EndBlock failed: %v", err)
	}

	// No observation should have been recorded.
	_, found := k.GetObservation(ctx, 200)
	if found {
		t.Fatal("expected no observation when chain is halted")
	}
}

// --- Test 6: Genesis import/export round-trip ---

func TestGenesisRoundtrip(t *testing.T) {
	k, _, ctx := setupKeeper(t)

	// Create some state.
	obs := &types.AlignmentObservation{
		Height:                  100,
		Timestamp:               time.Now().Unix(),
		KnowledgeQuality:        700_000,
		EconomicStability:       600_000,
		GovernanceParticipation: 500_000,
		NetworkSecurity:         720_720,
		StakingRatio:            600_000,
	}
	k.SetObservation(ctx, obs)

	scores := &types.DimensionScores{
		Height:                  100,
		KnowledgeQuality:        700_000,
		EconomicStability:       600_000,
		GovernanceParticipation: 500_000,
		NetworkSecurity:         720_720,
		StakingRatio:            600_000,
		Composite:               624_144,
	}
	k.SetScores(ctx, scores)

	hi := &types.AlignmentHealthIndex{
		Height:               100,
		CompositeScore:       624_144,
		Category:             types.CategoryDegraded,
		DimensionalScores:    scores,
		CorrectionsGenerated: 0,
	}
	k.SetHealthIndex(ctx, hi)

	k.AddCorrection(ctx, &types.CorrectionRecord{
		Height:    100,
		Dimension: types.DimKnowledgeQuality,
		Parameter: "knowledge.reward_multiplier",
		Direction: "increase",
		Magnitude: 100_000,
		Applied:   false,
		Timestamp: time.Now().Unix(),
	})

	state := &types.AlignmentState{
		Enabled:               true,
		LastObservationHeight: 100,
		ObservationCount:      1,
	}
	k.SetState(ctx, state)

	// Export.
	genState := k.ExportGenesis(ctx)
	if genState.State.LastObservationHeight != 100 {
		t.Fatalf("expected last_observation_height=100, got %d", genState.State.LastObservationHeight)
	}
	if len(genState.Observations) != 1 {
		t.Fatalf("expected 1 observation, got %d", len(genState.Observations))
	}
	if len(genState.Corrections) != 1 {
		t.Fatalf("expected 1 correction, got %d", len(genState.Corrections))
	}

	// Re-import on fresh keeper.
	k2, _, ctx2 := setupKeeper(t)
	k2.InitGenesis(ctx2, genState)

	// Verify import.
	state2 := k2.GetState(ctx2)
	if state2.LastObservationHeight != 100 {
		t.Fatalf("expected imported last_observation_height=100, got %d", state2.LastObservationHeight)
	}

	obs2, found := k2.GetObservation(ctx2, 100)
	if !found {
		t.Fatal("expected observation at height 100 after import")
	}
	if obs2.KnowledgeQuality != 700_000 {
		t.Fatalf("expected knowledge=700000 after import, got %d", obs2.KnowledgeQuality)
	}

	corrections, total := k2.GetCorrections(ctx2, 100, 0)
	if total != 1 {
		t.Fatalf("expected total=1 after import, got %d", total)
	}
	if len(corrections) != 1 {
		t.Fatalf("expected 1 correction after import, got %d", len(corrections))
	}
}

// --- Test 7: Different weight configs change composite score ---

func TestDifferentWeightsChangeComposite(t *testing.T) {
	k, _, ctx := setupKeeper(t)

	obs := k.ObserveAll(ctx)

	// Default equal weights.
	scores1 := k.ComputeScores(ctx, obs)
	composite1 := scores1.Composite

	// Change weights: heavily favor knowledge (60%).
	params := k.GetParams(ctx)
	params.WeightKnowledgeQuality = 600_000
	params.WeightEconomicStability = 100_000
	params.WeightGovernanceParticipation = 100_000
	params.WeightNetworkSecurity = 100_000
	params.WeightStakingRatio = 100_000
	k.SetParams(ctx, params)

	scores2 := k.ComputeScores(ctx, obs)
	composite2 := scores2.Composite

	// Knowledge is 620k (above average), so weighting it more should increase composite.
	if composite2 <= composite1 {
		t.Fatalf("expected higher composite with knowledge-heavy weights: got %d (was %d)", composite2, composite1)
	}
}

// --- Test 8: Autopoiesis nil-safe — corrections logged with applied=false ---

func TestAutopoiesisNilSafe(t *testing.T) {
	k, mocks, ctx := setupKeeper(t)

	// Ensure autopoiesis is NOT set (nil).
	// Set a low dimension to trigger corrections.
	mocks.knowledge.verificationRate = 100_000 // critical

	obs := k.ObserveAll(ctx)
	scores := k.ComputeScores(ctx, obs)
	corrections := k.GenerateCorrections(ctx, scores)

	// Apply with nil autopoiesis.
	k.ApplyCorrections(ctx, corrections)

	// Verify corrections stored with applied=false.
	stored, total := k.GetCorrections(ctx, 100, 0)
	if total == 0 {
		t.Fatal("expected stored corrections")
	}
	for _, c := range stored {
		if c.Applied {
			t.Fatalf("expected applied=false when autopoiesis nil, got true for %s", c.Dimension)
		}
	}

	// Now wire autopoiesis and verify corrections can be applied.
	autoMock := &mockAutopoiesisKeeper{}
	mocks.autopoiesis = autoMock
	k.SetAutopoiesisKeeper(autoMock)

	corrections2 := k.GenerateCorrections(ctx, scores)
	k.ApplyCorrections(ctx, corrections2)

	if len(autoMock.adjustments) == 0 {
		t.Fatal("expected adjustments after wiring autopoiesis")
	}
}

// --- Test 10: Bounded correction — small magnitude auto-applied ---

func TestBoundedCorrectionSmallAutoApplied(t *testing.T) {
	k, mocks, ctx := setupKeeper(t)

	autoMock := &mockAutopoiesisKeeper{}
	mocks.autopoiesis = autoMock
	k.SetAutopoiesisKeeper(autoMock)

	corrections := []*types.CorrectionRecord{{
		Height:    100,
		Dimension: types.DimKnowledgeQuality,
		Parameter: "knowledge.reward_multiplier",
		Direction: "increase",
		Magnitude: 100_000, // 10% — below 50% default max
		Timestamp: 1000,
	}}

	k.ApplyCorrections(ctx, corrections)

	if len(autoMock.adjustments) != 1 {
		t.Fatalf("expected 1 adjustment, got %d", len(autoMock.adjustments))
	}
	stored, _ := k.GetCorrections(ctx, 100, 0)
	if len(stored) == 0 || !stored[0].Applied {
		t.Fatal("expected correction marked as applied")
	}
}

// --- Test 11: Bounded correction — large magnitude blocked ---

func TestBoundedCorrectionLargeBlocked(t *testing.T) {
	k, mocks, ctx := setupKeeper(t)

	params := k.GetParams(ctx)
	params.MaxAutoApplyMagnitudeBps = 50_000 // 5%
	k.SetParams(ctx, params)

	autoMock := &mockAutopoiesisKeeper{}
	mocks.autopoiesis = autoMock
	k.SetAutopoiesisKeeper(autoMock)

	corrections := []*types.CorrectionRecord{{
		Height:    100,
		Dimension: types.DimKnowledgeQuality,
		Parameter: "knowledge.reward_multiplier",
		Direction: "increase",
		Magnitude: 200_000, // 20% — exceeds 5% max
		Timestamp: 1000,
	}}

	k.ApplyCorrections(ctx, corrections)

	if len(autoMock.adjustments) != 0 {
		t.Fatalf("expected 0 adjustments for large correction, got %d", len(autoMock.adjustments))
	}
	stored, _ := k.GetCorrections(ctx, 100, 0)
	if len(stored) == 0 {
		t.Fatal("expected correction stored")
	}
	if stored[0].Applied {
		t.Fatal("expected correction NOT applied (magnitude exceeds bounds)")
	}
}

// --- Test: Health transition to degraded enables double frequency ---

func TestHealthTransitionDegradedDoubleFrequency(t *testing.T) {
	k, mocks, ctx := setupKeeper(t)

	k.SetState(ctx, &types.AlignmentState{
		Enabled:          true,
		PreviousCategory: types.CategoryHealthy,
	})

	// Set dimensions to produce degraded composite (< 700k healthy threshold).
	mocks.knowledge.verificationRate = 300_000
	mocks.knowledge.consensusDiversity = 300_000
	mocks.staking.totalStaked = big.NewInt(400_000_000_000)
	mocks.staking.activeValidators = 50
	mocks.staking.targetValidators = 111
	mocks.ontology.domainCount = 30
	mocks.vestingRewards.totalSupply = big.NewInt(1_000_000_000_000)

	ctx = ctx.WithBlockHeight(100)
	am := alignment.NewAppModule(nil, k)
	if err := am.EndBlock(ctx); err != nil {
		t.Fatalf("EndBlock failed: %v", err)
	}

	state := k.GetState(ctx)
	if !state.DegradedFrequencyActive {
		t.Error("expected DegradedFrequencyActive=true after transition to degraded")
	}
	if state.PreviousCategory != types.CategoryDegraded && state.PreviousCategory != types.CategoryCritical {
		t.Errorf("expected PreviousCategory degraded or critical, got %s", state.PreviousCategory)
	}
}

// --- Test: Health transition recovery resets frequency ---

func TestHealthTransitionRecoveryResetsFrequency(t *testing.T) {
	k, mocks, ctx := setupKeeper(t)

	k.SetState(ctx, &types.AlignmentState{
		Enabled:                 true,
		PreviousCategory:        types.CategoryDegraded,
		DegradedFrequencyActive: true,
	})

	// Set all dimensions high to produce healthy composite.
	mocks.knowledge.verificationRate = 900_000
	mocks.knowledge.consensusDiversity = 900_000
	mocks.staking.totalStaked = big.NewInt(800_000_000_000)
	mocks.staking.activeValidators = 111
	mocks.staking.targetValidators = 111
	mocks.ontology.domainCount = 100
	mocks.vestingRewards.totalSupply = big.NewInt(1_000_000_000_000)

	ctx = ctx.WithBlockHeight(100)
	am := alignment.NewAppModule(nil, k)
	if err := am.EndBlock(ctx); err != nil {
		t.Fatalf("EndBlock failed: %v", err)
	}

	state := k.GetState(ctx)
	if state.DegradedFrequencyActive {
		t.Error("expected DegradedFrequencyActive=false after recovery to healthy")
	}
	if state.PreviousCategory != types.CategoryHealthy {
		t.Errorf("expected PreviousCategory=healthy, got %s", state.PreviousCategory)
	}
}

// --- Test: Degraded frequency affects interval ---

func TestDegradedFrequencyAffectsInterval(t *testing.T) {
	k, mocks, ctx := setupKeeper(t)

	// Pre-set degraded state with frequency active.
	k.SetState(ctx, &types.AlignmentState{
		Enabled:                 true,
		DegradedFrequencyActive: true,
		PreviousCategory:        types.CategoryDegraded,
	})

	mocks.knowledge.verificationRate = 300_000
	mocks.knowledge.consensusDiversity = 300_000
	mocks.staking.totalStaked = big.NewInt(400_000_000_000)
	mocks.staking.activeValidators = 50
	mocks.staking.targetValidators = 111
	mocks.ontology.domainCount = 30
	mocks.vestingRewards.totalSupply = big.NewInt(1_000_000_000_000)

	// Default interval = 100, degraded = 50.
	// Height 50 should trigger (50 % 50 == 0).
	ctx = ctx.WithBlockHeight(50)
	am := alignment.NewAppModule(nil, k)
	if err := am.EndBlock(ctx); err != nil {
		t.Fatalf("EndBlock failed: %v", err)
	}

	_, found := k.GetObservation(ctx, 50)
	if !found {
		t.Error("expected observation at height 50 (degraded frequency: interval=50)")
	}
}

// --- Test 9: Param validation rejects invalid configs ---

func TestParamValidation(t *testing.T) {
	tests := []struct {
		name   string
		modify func(*types.Params)
		errMsg string
	}{
		{
			name: "weights not summing to 1M",
			modify: func(p *types.Params) {
				p.WeightKnowledgeQuality = 300_000 // sum = 300k + 200k*4 = 1.1M
			},
			errMsg: "weights",
		},
		{
			name: "bad threshold order: critical >= degraded",
			modify: func(p *types.Params) {
				p.CriticalThreshold = 500_000
				p.DegradedThreshold = 400_000
			},
			errMsg: "threshold",
		},
		{
			name: "bad threshold order: degraded >= healthy",
			modify: func(p *types.Params) {
				p.DegradedThreshold = 800_000
				p.HealthyThreshold = 700_000
			},
			errMsg: "threshold",
		},
		{
			name: "zero interval",
			modify: func(p *types.Params) {
				p.ObservationIntervalBlocks = 0
			},
			errMsg: "interval",
		},
		{
			name: "threshold exceeds BPS",
			modify: func(p *types.Params) {
				p.HealthyThreshold = 1_100_000
			},
			errMsg: "threshold",
		},
		{
			name: "max_auto_apply exceeds BPS",
			modify: func(p *types.Params) {
				p.MaxAutoApplyMagnitudeBps = types.BPS + 1
			},
			errMsg: "max_auto_apply",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := types.DefaultParams()
			tt.modify(&params)
			err := params.Validate()
			if err == nil {
				t.Fatalf("expected validation error for: %s", tt.name)
			}
		})
	}

	// Valid params should pass.
	params := types.DefaultParams()
	if err := params.Validate(); err != nil {
		t.Fatalf("default params should be valid: %v", err)
	}
}

// --- Test: Query HealthHistory returns entries in reverse order ---

func TestQueryHealthHistory(t *testing.T) {
	k, _, ctx := setupKeeper(t)
	qs := keeper.NewQueryServerImpl(k)

	for i := uint64(100); i <= 500; i += 100 {
		k.SetHealthIndex(ctx, &types.AlignmentHealthIndex{
			Height:         i,
			CompositeScore: 700_000 + i,
			Category:       types.CategoryHealthy,
		})
	}

	resp, err := qs.HealthHistory(ctx, &types.QueryHealthHistoryRequest{Limit: 3})
	if err != nil {
		t.Fatalf("query HealthHistory failed: %v", err)
	}
	if len(resp.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(resp.Entries))
	}
	if resp.Entries[0].Height != 500 {
		t.Errorf("expected first entry height=500 (most recent), got %d", resp.Entries[0].Height)
	}
}

func TestGetRecentHealthIndicesEmpty(t *testing.T) {
	k, _, ctx := setupKeeper(t)
	results := k.GetRecentHealthIndices(ctx, 0)
	if len(results) != 0 {
		t.Errorf("expected 0 entries, got %d", len(results))
	}
}

// --- Test: Full EndBlocker lifecycle — healthy → degraded → recovery ---

func TestEndBlockerFullCycle(t *testing.T) {
	k, mocks, ctx := setupKeeper(t)

	// Wire autopoiesis.
	autoMock := &mockAutopoiesisKeeper{}
	k.SetAutopoiesisKeeper(autoMock)

	// Set all dimensions to healthy values.
	mocks.knowledge.verificationRate = 800_000
	mocks.knowledge.consensusDiversity = 700_000
	mocks.staking.totalStaked = big.NewInt(800_000_000_000)
	mocks.staking.activeValidators = 100
	mocks.staking.targetValidators = 111
	mocks.ontology.domainCount = 80
	mocks.vestingRewards.totalSupply = big.NewInt(1_000_000_000_000)

	am := alignment.NewAppModule(nil, k)

	// --- Block 100: First observation (should be healthy) ---
	ctx = ctx.WithBlockHeight(100)
	if err := am.EndBlock(ctx); err != nil {
		t.Fatalf("EndBlock at 100 failed: %v", err)
	}

	obs, found := k.GetObservation(ctx, 100)
	if !found {
		t.Fatal("expected observation at height 100")
	}
	if obs.KnowledgeQuality == 0 {
		t.Error("expected non-zero knowledge quality")
	}

	hi, found := k.GetHealthIndex(ctx, 100)
	if !found {
		t.Fatal("expected health index at height 100")
	}
	if hi.Category != types.CategoryHealthy {
		t.Errorf("expected healthy at block 100, got %s (composite=%d)", hi.Category, hi.CompositeScore)
	}

	state := k.GetState(ctx)
	if state.ObservationCount != 1 {
		t.Errorf("expected observation_count=1, got %d", state.ObservationCount)
	}
	if state.PreviousCategory != types.CategoryHealthy {
		t.Errorf("expected PreviousCategory=healthy, got %s", state.PreviousCategory)
	}

	// --- Degrade dimensions ---
	mocks.knowledge.verificationRate = 200_000
	mocks.knowledge.consensusDiversity = 200_000
	mocks.staking.totalStaked = big.NewInt(200_000_000_000)
	mocks.staking.activeValidators = 30
	mocks.ontology.domainCount = 10

	// --- Block 200: Second observation (should be degraded/critical) ---
	ctx = ctx.WithBlockHeight(200)
	if err := am.EndBlock(ctx); err != nil {
		t.Fatalf("EndBlock at 200 failed: %v", err)
	}

	hi2, found := k.GetHealthIndex(ctx, 200)
	if !found {
		t.Fatal("expected health index at height 200")
	}
	if hi2.Category == types.CategoryHealthy {
		t.Error("expected NOT healthy with degraded dimensions")
	}

	state2 := k.GetState(ctx)
	if state2.ObservationCount != 2 {
		t.Errorf("expected observation_count=2, got %d", state2.ObservationCount)
	}
	if !state2.DegradedFrequencyActive {
		t.Error("expected DegradedFrequencyActive=true after health degradation")
	}

	// Verify corrections were generated (dimensions below degraded threshold).
	corrections, total := k.GetCorrections(ctx, 100, 0)
	if total == 0 {
		t.Error("expected corrections generated during degraded observation")
	}
	_ = corrections

	// --- Recover dimensions ---
	mocks.knowledge.verificationRate = 900_000
	mocks.knowledge.consensusDiversity = 900_000
	mocks.staking.totalStaked = big.NewInt(900_000_000_000)
	mocks.staking.activeValidators = 111
	mocks.ontology.domainCount = 100

	// --- Recovery: Degraded frequency means interval=50, so height 250 should trigger ---
	ctx = ctx.WithBlockHeight(250)
	if err := am.EndBlock(ctx); err != nil {
		t.Fatalf("EndBlock at 250 failed: %v", err)
	}

	hi3, found := k.GetHealthIndex(ctx, 250)
	if !found {
		t.Fatal("expected health index at height 250")
	}
	if hi3.Category != types.CategoryHealthy {
		t.Errorf("expected healthy after recovery, got %s (composite=%d)", hi3.Category, hi3.CompositeScore)
	}

	state3 := k.GetState(ctx)
	if state3.DegradedFrequencyActive {
		t.Error("expected DegradedFrequencyActive=false after recovery to healthy")
	}
	if state3.PreviousCategory != types.CategoryHealthy {
		t.Errorf("expected PreviousCategory=healthy after recovery, got %s", state3.PreviousCategory)
	}
	if state3.ObservationCount != 3 {
		t.Errorf("expected observation_count=3, got %d", state3.ObservationCount)
	}

	// --- Verify history query returns all 3 observations ---
	results := k.GetRecentHealthIndices(ctx, 10)
	if len(results) != 3 {
		t.Errorf("expected 3 health history entries, got %d", len(results))
	}
	if len(results) > 0 && results[0].Height != 250 {
		t.Errorf("expected most recent entry at height 250, got %d", results[0].Height)
	}
}
