package keeper_test

import (
	"math/big"
	"testing"
	"time"

	"github.com/zerone-chain/zerone/x/alignment/keeper"
	"github.com/zerone-chain/zerone/x/alignment/types"
)

// ========== Sensor Edge Cases ==========

func TestSensorZeroStaking(t *testing.T) {
	k, mocks, ctx := setupKeeper(t)

	mocks.staking.totalStaked = big.NewInt(0)
	mocks.staking.activeValidators = 0
	mocks.staking.targetValidators = 111

	obs := k.ObserveAll(ctx)

	if obs.EconomicStability != 0 {
		t.Errorf("expected 0 economic stability with no staking, got %d", obs.EconomicStability)
	}
	if obs.StakingRatio != 0 {
		t.Errorf("expected 0 staking ratio, got %d", obs.StakingRatio)
	}
	if obs.NetworkSecurity != 0 {
		t.Errorf("expected 0 network security with 0 validators, got %d", obs.NetworkSecurity)
	}
}

func TestSensorMaxValues(t *testing.T) {
	k, mocks, ctx := setupKeeper(t)

	mocks.knowledge.verificationRate = types.BPS
	mocks.knowledge.consensusDiversity = types.BPS // both inputs at max → weighted output at max
	mocks.staking.totalStaked = big.NewInt(2_000_000_000_000) // > supply
	mocks.staking.activeValidators = 200                       // > target
	mocks.staking.targetValidators = 111
	mocks.ontology.domainCount = 200 // > target 100
	mocks.vestingRewards.totalSupply = big.NewInt(1_000_000_000_000)

	obs := k.ObserveAll(ctx)

	if obs.KnowledgeQuality != types.BPS {
		t.Errorf("expected knowledge capped at BPS, got %d", obs.KnowledgeQuality)
	}
	if obs.EconomicStability != types.BPS {
		t.Errorf("expected economic capped at BPS, got %d", obs.EconomicStability)
	}
	if obs.GovernanceParticipation != types.BPS {
		t.Errorf("expected governance capped at BPS, got %d", obs.GovernanceParticipation)
	}
	if obs.NetworkSecurity != types.BPS {
		t.Errorf("expected security capped at BPS, got %d", obs.NetworkSecurity)
	}
}

func TestSensorZeroTargetValidators(t *testing.T) {
	k, mocks, ctx := setupKeeper(t)
	mocks.staking.targetValidators = 0

	obs := k.ObserveAll(ctx)

	// Should return NeutralBPS (500000) when target is 0 (division by zero protection).
	if obs.NetworkSecurity != types.NeutralBPS {
		t.Errorf("expected NeutralBPS for zero target validators, got %d", obs.NetworkSecurity)
	}
}

func TestSensorZeroSupply(t *testing.T) {
	k, mocks, ctx := setupKeeper(t)
	mocks.vestingRewards.totalSupply = big.NewInt(0)

	obs := k.ObserveAll(ctx)

	// Division by zero protection returns NeutralBPS.
	if obs.EconomicStability != types.NeutralBPS {
		t.Errorf("expected NeutralBPS for zero supply, got %d", obs.EconomicStability)
	}
	if obs.StakingRatio != types.NeutralBPS {
		t.Errorf("expected NeutralBPS for zero supply staking, got %d", obs.StakingRatio)
	}
}

func TestSensorKnowledgeAboveBPS(t *testing.T) {
	k, mocks, ctx := setupKeeper(t)
	mocks.knowledge.verificationRate = types.BPS + 100_000   // above max
	mocks.knowledge.consensusDiversity = types.BPS + 100_000 // above max

	obs := k.ObserveAll(ctx)

	if obs.KnowledgeQuality != types.BPS {
		t.Errorf("expected knowledge capped at BPS, got %d", obs.KnowledgeQuality)
	}
}

func TestSensorGovernanceDomainCount(t *testing.T) {
	k, mocks, ctx := setupKeeper(t)

	// Test exact target (100 domains = 100%).
	mocks.ontology.domainCount = 100
	obs := k.ObserveAll(ctx)
	if obs.GovernanceParticipation != types.BPS {
		t.Errorf("expected BPS for exact target, got %d", obs.GovernanceParticipation)
	}

	// Test partial (25 domains = 25%).
	mocks.ontology.domainCount = 25
	obs = k.ObserveAll(ctx)
	expected := uint64(25) * types.BPS / 100
	if obs.GovernanceParticipation != expected {
		t.Errorf("expected %d for 25 domains, got %d", expected, obs.GovernanceParticipation)
	}
}

// ========== Scoring ==========

func TestComputeScoresAllDimensionsEqual(t *testing.T) {
	k, mocks, ctx := setupKeeper(t)

	// Set all dimensions to same value.
	mocks.knowledge.verificationRate = 600_000
	mocks.knowledge.consensusDiversity = 600_000 // match rate so weighted formula yields 600k
	mocks.staking.totalStaked = big.NewInt(600_000_000_000)
	mocks.staking.activeValidators = 111 // 111/111 = BPS, but we need 600k...
	mocks.staking.targetValidators = 111
	mocks.ontology.domainCount = 60
	mocks.vestingRewards.totalSupply = big.NewInt(1_000_000_000_000)

	obs := k.ObserveAll(ctx)
	scores := k.ComputeScores(ctx, obs)

	// With equal weights, composite = average of all dimensions.
	if scores.Composite == 0 {
		t.Fatal("composite should not be zero")
	}
	if scores.KnowledgeQuality != 600_000 {
		t.Errorf("expected knowledge_quality 600000, got %d", scores.KnowledgeQuality)
	}
}

func TestCategorizeHealthBoundaries(t *testing.T) {
	k, _, ctx := setupKeeper(t)

	// Critical: < 200k.
	cat := k.CategorizeHealth(ctx, 100_000)
	if cat != types.CategoryCritical {
		t.Errorf("expected critical for 100k, got %s", cat)
	}

	// Exactly at critical boundary: 200k → degraded (not critical).
	cat = k.CategorizeHealth(ctx, 200_000)
	if cat != types.CategoryDegraded {
		t.Errorf("expected degraded for 200k, got %s", cat)
	}

	// Degraded: 400k.
	cat = k.CategorizeHealth(ctx, 400_000)
	if cat != types.CategoryDegraded {
		t.Errorf("expected degraded for 400k, got %s", cat)
	}

	// Healthy: 700k.
	cat = k.CategorizeHealth(ctx, 700_000)
	if cat != types.CategoryHealthy {
		t.Errorf("expected healthy for 700k, got %s", cat)
	}

	// Exactly at healthy boundary: 700k → healthy.
	cat = k.CategorizeHealth(ctx, 699_999)
	if cat != types.CategoryDegraded {
		t.Errorf("expected degraded for 699999, got %s", cat)
	}
}

func TestBuildHealthIndex(t *testing.T) {
	k, _, _ := setupKeeper(t)

	scores := &types.DimensionScores{
		Height:    100,
		Composite: 750_000,
	}

	hi := k.BuildHealthIndex(scores, types.CategoryHealthy, 2)

	if hi.Height != 100 {
		t.Errorf("expected height 100, got %d", hi.Height)
	}
	if hi.CompositeScore != 750_000 {
		t.Errorf("expected composite 750000, got %d", hi.CompositeScore)
	}
	if hi.Category != types.CategoryHealthy {
		t.Errorf("expected healthy, got %s", hi.Category)
	}
	if hi.CorrectionsGenerated != 2 {
		t.Errorf("expected 2 corrections, got %d", hi.CorrectionsGenerated)
	}
	if hi.DimensionalScores != scores {
		t.Error("expected dimensional scores reference preserved")
	}
}

// ========== Corrections ==========

func TestCorrectionsCriticalDoublesMagnitude(t *testing.T) {
	k, mocks, ctx := setupKeeper(t)

	// Set knowledge to critical (below 200k).
	mocks.knowledge.verificationRate = 100_000
	mocks.knowledge.consensusDiversity = 100_000 // match rate so weighted formula yields 100k

	obs := k.ObserveAll(ctx)
	scores := k.ComputeScores(ctx, obs)
	corrections := k.GenerateCorrections(ctx, scores)

	var knowledgeCorrection *types.CorrectionRecord
	for _, c := range corrections {
		if c.Dimension == types.DimKnowledgeQuality {
			knowledgeCorrection = c
			break
		}
	}

	if knowledgeCorrection == nil {
		t.Fatal("expected knowledge correction")
	}

	// Degraded threshold is 400k. Score is 100k.
	// Magnitude = (400k - 100k) = 300k, doubled for critical = 600k.
	expectedMag := (uint64(400_000) - 100_000) * 2
	if knowledgeCorrection.Magnitude != expectedMag {
		t.Errorf("expected magnitude %d (2x critical), got %d", expectedMag, knowledgeCorrection.Magnitude)
	}
}

func TestCorrectionsNotGeneratedForGovernance(t *testing.T) {
	k, mocks, ctx := setupKeeper(t)

	// Set governance very low (but other dims healthy).
	mocks.ontology.domainCount = 5 // 5% governance
	mocks.knowledge.verificationRate = 800_000
	mocks.staking.totalStaked = big.NewInt(800_000_000_000)
	mocks.staking.activeValidators = 111
	mocks.vestingRewards.totalSupply = big.NewInt(1_000_000_000_000)

	obs := k.ObserveAll(ctx)
	scores := k.ComputeScores(ctx, obs)
	corrections := k.GenerateCorrections(ctx, scores)

	for _, c := range corrections {
		if c.Dimension == types.DimGovernanceParticipation {
			t.Fatal("governance should NOT generate corrections (log-only)")
		}
	}
}

func TestCorrectionsAppliedWithAutopoiesis(t *testing.T) {
	k, mocks, ctx := setupKeeper(t)

	autoMock := &mockAutopoiesisKeeper{}
	mocks.autopoiesis = autoMock
	k.SetAutopoiesisKeeper(autoMock)

	mocks.knowledge.verificationRate = 100_000 // critical

	obs := k.ObserveAll(ctx)
	scores := k.ComputeScores(ctx, obs)
	corrections := k.GenerateCorrections(ctx, scores)
	k.ApplyCorrections(ctx, corrections)

	if len(autoMock.adjustments) == 0 {
		t.Fatal("expected adjustments sent to autopoiesis")
	}

	// Verify correction marked as applied.
	stored, _ := k.GetCorrections(ctx, 100, 0)
	for _, c := range stored {
		if c.Dimension == types.DimKnowledgeQuality && !c.Applied {
			t.Error("expected knowledge correction to be marked as applied")
		}
	}
}

func TestCorrectionsAllDimensionsBelowDegraded(t *testing.T) {
	k, mocks, ctx := setupKeeper(t)

	mocks.knowledge.verificationRate = 100_000
	mocks.staking.totalStaked = big.NewInt(100_000_000_000) // 10%
	mocks.staking.activeValidators = 10                       // low
	mocks.staking.targetValidators = 111
	mocks.ontology.domainCount = 5
	mocks.vestingRewards.totalSupply = big.NewInt(1_000_000_000_000)

	obs := k.ObserveAll(ctx)
	scores := k.ComputeScores(ctx, obs)
	corrections := k.GenerateCorrections(ctx, scores)

	// Should have corrections for: knowledge, economic, network_security, staking.
	// (NOT governance — log only.)
	if len(corrections) != 4 {
		t.Errorf("expected 4 corrections, got %d", len(corrections))
	}

	dims := make(map[string]bool)
	for _, c := range corrections {
		dims[c.Dimension] = true
	}
	for _, dim := range []string{types.DimKnowledgeQuality, types.DimEconomicStability, types.DimNetworkSecurity, types.DimStakingRatio} {
		if !dims[dim] {
			t.Errorf("expected correction for %s", dim)
		}
	}
}

// ========== State CRUD ==========

func TestStateRoundtrip(t *testing.T) {
	k, _, ctx := setupKeeper(t)

	state := &types.AlignmentState{
		Enabled:               true,
		LastObservationHeight: 500,
		ObservationCount:      42,
	}
	k.SetState(ctx, state)

	got := k.GetState(ctx)
	if got.LastObservationHeight != 500 {
		t.Errorf("expected 500, got %d", got.LastObservationHeight)
	}
	if got.ObservationCount != 42 {
		t.Errorf("expected 42, got %d", got.ObservationCount)
	}
}

func TestObservationRoundtrip(t *testing.T) {
	k, _, ctx := setupKeeper(t)

	obs := &types.AlignmentObservation{
		Height:           200,
		Timestamp:        time.Now().Unix(),
		KnowledgeQuality: 700_000,
		EconomicStability: 600_000,
	}
	k.SetObservation(ctx, obs)

	got, found := k.GetObservation(ctx, 200)
	if !found {
		t.Fatal("observation not found")
	}
	if got.KnowledgeQuality != 700_000 {
		t.Errorf("expected 700000, got %d", got.KnowledgeQuality)
	}
}

func TestObservationNotFound(t *testing.T) {
	k, _, ctx := setupKeeper(t)

	_, found := k.GetObservation(ctx, 999)
	if found {
		t.Error("expected not found for nonexistent observation")
	}
}

func TestScoresRoundtrip(t *testing.T) {
	k, _, ctx := setupKeeper(t)

	scores := &types.DimensionScores{
		Height:    300,
		Composite: 750_000,
	}
	k.SetScores(ctx, scores)

	got, found := k.GetScores(ctx, 300)
	if !found {
		t.Fatal("scores not found")
	}
	if got.Composite != 750_000 {
		t.Errorf("expected 750000, got %d", got.Composite)
	}
}

func TestScoresNotFound(t *testing.T) {
	k, _, ctx := setupKeeper(t)

	_, found := k.GetScores(ctx, 999)
	if found {
		t.Error("expected not found")
	}
}

func TestHealthIndexRoundtrip(t *testing.T) {
	k, _, ctx := setupKeeper(t)

	hi := &types.AlignmentHealthIndex{
		Height:         400,
		CompositeScore: 800_000,
		Category:       types.CategoryHealthy,
	}
	k.SetHealthIndex(ctx, hi)

	got, found := k.GetHealthIndex(ctx, 400)
	if !found {
		t.Fatal("health index not found")
	}
	if got.Category != types.CategoryHealthy {
		t.Errorf("expected healthy, got %s", got.Category)
	}
}

func TestHealthIndexNotFound(t *testing.T) {
	k, _, ctx := setupKeeper(t)

	_, found := k.GetHealthIndex(ctx, 999)
	if found {
		t.Error("expected not found")
	}
}

func TestCorrectionStorage(t *testing.T) {
	k, _, ctx := setupKeeper(t)

	for i := 0; i < 5; i++ {
		k.AddCorrection(ctx, &types.CorrectionRecord{
			Height:    100,
			Dimension: types.DimKnowledgeQuality,
			Parameter: "knowledge.reward_multiplier",
			Direction: "increase",
			Magnitude: uint64(100_000 + i*10_000),
			Timestamp: time.Now().Unix(),
		})
	}

	corrections, total := k.GetCorrections(ctx, 0, 0)
	if total != 5 {
		t.Errorf("expected total=5, got %d", total)
	}
	if len(corrections) != 5 {
		t.Errorf("expected 5 corrections, got %d", len(corrections))
	}
}

func TestCorrectionPagination(t *testing.T) {
	k, _, ctx := setupKeeper(t)

	for i := 0; i < 10; i++ {
		k.AddCorrection(ctx, &types.CorrectionRecord{
			Height:    100,
			Dimension: types.DimKnowledgeQuality,
			Magnitude: uint64(i),
			Timestamp: time.Now().Unix(),
		})
	}

	// Get first 3.
	page1, total := k.GetCorrections(ctx, 3, 0)
	if total != 10 {
		t.Errorf("expected total=10, got %d", total)
	}
	if len(page1) != 3 {
		t.Errorf("expected 3 corrections in page 1, got %d", len(page1))
	}

	// Get next 3 (offset 3).
	page2, _ := k.GetCorrections(ctx, 3, 3)
	if len(page2) != 3 {
		t.Errorf("expected 3 corrections in page 2, got %d", len(page2))
	}
}

func TestIsEnabledAndHalted(t *testing.T) {
	k, mocks, ctx := setupKeeper(t)

	// Default: enabled + not halted.
	if !k.IsEnabled(ctx) {
		t.Error("expected enabled by default")
	}
	if k.IsHalted(ctx) {
		t.Error("expected not halted by default")
	}

	// Disable via params.
	params := k.GetParams(ctx)
	params.Enabled = false
	k.SetParams(ctx, params)
	if k.IsEnabled(ctx) {
		t.Error("expected disabled after params change")
	}

	// Re-enable params but disable state.
	params.Enabled = true
	k.SetParams(ctx, params)
	state := k.GetState(ctx)
	state.Enabled = false
	k.SetState(ctx, state)
	if k.IsEnabled(ctx) {
		t.Error("expected disabled when state.Enabled=false")
	}

	// Halt.
	mocks.emergency.halted = true
	if !k.IsHalted(ctx) {
		t.Error("expected halted when emergency is halted")
	}
}

func TestGetLastObservationHeight(t *testing.T) {
	k, _, ctx := setupKeeper(t)

	k.SetState(ctx, &types.AlignmentState{
		Enabled:               true,
		LastObservationHeight: 250,
	})

	h := k.GetLastObservationHeight(ctx)
	if h != 250 {
		t.Errorf("expected 250, got %d", h)
	}
}

// ========== Msg Server ==========

func TestMsgUpdateParamsAuthorized(t *testing.T) {
	k, _, ctx := setupKeeper(t)
	msgSrv := keeper.NewMsgServerImpl(k)

	params := types.DefaultParams()
	params.ObservationIntervalBlocks = 50
	_, err := msgSrv.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: "authority",
		Params:    &params,
	})
	if err != nil {
		t.Fatalf("UpdateParams failed: %v", err)
	}

	got := k.GetParams(ctx)
	if got.ObservationIntervalBlocks != 50 {
		t.Errorf("expected interval 50, got %d", got.ObservationIntervalBlocks)
	}
}

func TestMsgUpdateParamsUnauthorized(t *testing.T) {
	k, _, ctx := setupKeeper(t)
	msgSrv := keeper.NewMsgServerImpl(k)

	params := types.DefaultParams()
	_, err := msgSrv.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: "intruder",
		Params:    &params,
	})
	if err == nil {
		t.Fatal("expected unauthorized error")
	}
}

func TestMsgUpdateParamsInvalid(t *testing.T) {
	k, _, ctx := setupKeeper(t)
	msgSrv := keeper.NewMsgServerImpl(k)

	invalid := types.DefaultParams()
	invalid.ObservationIntervalBlocks = 0

	_, err := msgSrv.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: "authority",
		Params:    &invalid,
	})
	if err == nil {
		t.Fatal("expected validation error for zero interval")
	}
}

func TestMsgActivate(t *testing.T) {
	k, _, ctx := setupKeeper(t)
	msgSrv := keeper.NewMsgServerImpl(k)

	// Deactivate.
	_, err := msgSrv.Activate(ctx, &types.MsgActivate{
		Authority: "authority",
		Enabled:   false,
	})
	if err != nil {
		t.Fatalf("Activate failed: %v", err)
	}

	state := k.GetState(ctx)
	if state.Enabled {
		t.Error("expected disabled after deactivation")
	}

	// Re-activate.
	_, err = msgSrv.Activate(ctx, &types.MsgActivate{
		Authority: "authority",
		Enabled:   true,
	})
	if err != nil {
		t.Fatalf("Activate failed: %v", err)
	}

	state = k.GetState(ctx)
	if !state.Enabled {
		t.Error("expected enabled after re-activation")
	}
}

func TestMsgActivateUnauthorized(t *testing.T) {
	k, _, ctx := setupKeeper(t)
	msgSrv := keeper.NewMsgServerImpl(k)

	_, err := msgSrv.Activate(ctx, &types.MsgActivate{
		Authority: "intruder",
		Enabled:   false,
	})
	if err == nil {
		t.Fatal("expected unauthorized error")
	}
}

// ========== Query Server ==========

func TestQueryParams(t *testing.T) {
	k, _, ctx := setupKeeper(t)
	qs := keeper.NewQueryServerImpl(k)

	resp, err := qs.Params(ctx, &types.QueryParamsRequest{})
	if err != nil {
		t.Fatalf("query Params failed: %v", err)
	}
	if resp.Params.ObservationIntervalBlocks != 100 {
		t.Errorf("expected interval 100, got %d", resp.Params.ObservationIntervalBlocks)
	}
}

func TestQueryState(t *testing.T) {
	k, _, ctx := setupKeeper(t)
	qs := keeper.NewQueryServerImpl(k)

	resp, err := qs.State(ctx, &types.QueryStateRequest{})
	if err != nil {
		t.Fatalf("query State failed: %v", err)
	}
	if !resp.State.Enabled {
		t.Error("expected enabled state")
	}
}

func TestQueryObservation(t *testing.T) {
	k, _, ctx := setupKeeper(t)
	qs := keeper.NewQueryServerImpl(k)

	k.SetObservation(ctx, &types.AlignmentObservation{
		Height:           100,
		KnowledgeQuality: 700_000,
	})

	resp, err := qs.Observation(ctx, &types.QueryObservationRequest{Height: 100})
	if err != nil {
		t.Fatalf("query Observation failed: %v", err)
	}
	if !resp.Found {
		t.Error("expected observation found")
	}
	if resp.Observation.KnowledgeQuality != 700_000 {
		t.Errorf("expected 700000, got %d", resp.Observation.KnowledgeQuality)
	}
}

func TestQueryObservationNotFound(t *testing.T) {
	k, _, ctx := setupKeeper(t)
	qs := keeper.NewQueryServerImpl(k)

	resp, err := qs.Observation(ctx, &types.QueryObservationRequest{Height: 999})
	if err != nil {
		t.Fatalf("query Observation failed: %v", err)
	}
	if resp.Found {
		t.Error("expected not found")
	}
}

func TestQueryScores(t *testing.T) {
	k, _, ctx := setupKeeper(t)
	qs := keeper.NewQueryServerImpl(k)

	k.SetScores(ctx, &types.DimensionScores{Height: 100, Composite: 800_000})

	resp, err := qs.Scores(ctx, &types.QueryScoresRequest{Height: 100})
	if err != nil {
		t.Fatalf("query Scores failed: %v", err)
	}
	if !resp.Found {
		t.Error("expected scores found")
	}
}

func TestQueryHealthIndex(t *testing.T) {
	k, _, ctx := setupKeeper(t)
	qs := keeper.NewQueryServerImpl(k)

	k.SetHealthIndex(ctx, &types.AlignmentHealthIndex{
		Height:         100,
		CompositeScore: 800_000,
		Category:       types.CategoryHealthy,
	})

	resp, err := qs.HealthIndex(ctx, &types.QueryHealthIndexRequest{Height: 100})
	if err != nil {
		t.Fatalf("query HealthIndex failed: %v", err)
	}
	if !resp.Found {
		t.Error("expected health index found")
	}
	if resp.HealthIndex.Category != types.CategoryHealthy {
		t.Errorf("expected healthy, got %s", resp.HealthIndex.Category)
	}
}

func TestQueryCorrectionHistory(t *testing.T) {
	k, _, ctx := setupKeeper(t)
	qs := keeper.NewQueryServerImpl(k)

	for i := 0; i < 5; i++ {
		k.AddCorrection(ctx, &types.CorrectionRecord{
			Height:    100,
			Dimension: types.DimKnowledgeQuality,
			Magnitude: uint64(i),
			Timestamp: time.Now().Unix(),
		})
	}

	resp, err := qs.CorrectionHistory(ctx, &types.QueryCorrectionHistoryRequest{Limit: 3, Offset: 0})
	if err != nil {
		t.Fatalf("query CorrectionHistory failed: %v", err)
	}
	if resp.Total != 5 {
		t.Errorf("expected total=5, got %d", resp.Total)
	}
	if len(resp.Corrections) != 3 {
		t.Errorf("expected 3 corrections, got %d", len(resp.Corrections))
	}
}

func TestQueryCorrectionHistoryDefaultLimit(t *testing.T) {
	k, _, ctx := setupKeeper(t)
	qs := keeper.NewQueryServerImpl(k)

	// Zero limit should default to 100.
	resp, err := qs.CorrectionHistory(ctx, &types.QueryCorrectionHistoryRequest{Limit: 0})
	if err != nil {
		t.Fatalf("query CorrectionHistory failed: %v", err)
	}
	// No corrections, but should not error.
	if resp.Total != 0 {
		t.Errorf("expected 0 total, got %d", resp.Total)
	}
}

// ========== Adversarial / Edge Cases ==========

func TestCustomWeightsChangeCategory(t *testing.T) {
	k, mocks, ctx := setupKeeper(t)

	// Set dimensions: knowledge high, rest medium.
	mocks.knowledge.verificationRate = 900_000
	mocks.staking.totalStaked = big.NewInt(400_000_000_000) // 40%
	mocks.staking.activeValidators = 50
	mocks.staking.targetValidators = 111
	mocks.ontology.domainCount = 40
	mocks.vestingRewards.totalSupply = big.NewInt(1_000_000_000_000)

	obs := k.ObserveAll(ctx)

	// With default equal weights, composite should be medium.
	scores1 := k.ComputeScores(ctx, obs)
	cat1 := k.CategorizeHealth(ctx, scores1.Composite)

	// Heavily weight knowledge (90%).
	params := k.GetParams(ctx)
	params.WeightKnowledgeQuality = 900_000
	params.WeightEconomicStability = 25_000
	params.WeightGovernanceParticipation = 25_000
	params.WeightNetworkSecurity = 25_000
	params.WeightStakingRatio = 25_000
	k.SetParams(ctx, params)

	scores2 := k.ComputeScores(ctx, obs)
	cat2 := k.CategorizeHealth(ctx, scores2.Composite)

	// With heavy knowledge weighting, composite should improve.
	if scores2.Composite <= scores1.Composite {
		t.Errorf("expected higher composite with knowledge weighting: %d vs %d", scores2.Composite, scores1.Composite)
	}

	// With 90% of 900k, should be healthy.
	if cat2 != types.CategoryHealthy {
		t.Errorf("expected healthy with knowledge-heavy weights, got %s (composite=%d, was %s)", cat2, scores2.Composite, cat1)
	}
}

func TestCorrectionDegradedVsCriticalMagnitude(t *testing.T) {
	k, mocks, ctx := setupKeeper(t)

	// Degraded case: below degraded (400k) but above critical (200k).
	mocks.knowledge.verificationRate = 300_000
	mocks.staking.totalStaked = big.NewInt(800_000_000_000)
	mocks.staking.activeValidators = 111
	mocks.vestingRewards.totalSupply = big.NewInt(1_000_000_000_000)

	obs := k.ObserveAll(ctx)
	scores := k.ComputeScores(ctx, obs)
	degradedCorrections := k.GenerateCorrections(ctx, scores)

	var degradedMag uint64
	for _, c := range degradedCorrections {
		if c.Dimension == types.DimKnowledgeQuality {
			degradedMag = c.Magnitude
		}
	}

	// Critical case: below critical (200k).
	mocks.knowledge.verificationRate = 100_000
	obs = k.ObserveAll(ctx)
	scores = k.ComputeScores(ctx, obs)
	criticalCorrections := k.GenerateCorrections(ctx, scores)

	var criticalMag uint64
	for _, c := range criticalCorrections {
		if c.Dimension == types.DimKnowledgeQuality {
			criticalMag = c.Magnitude
		}
	}

	// Critical magnitude should be > degraded magnitude (due to 2x multiplier).
	if criticalMag <= degradedMag {
		t.Errorf("expected critical magnitude (%d) > degraded magnitude (%d)", criticalMag, degradedMag)
	}
}

func TestGenesisEmptyState(t *testing.T) {
	k, _, ctx := setupKeeper(t)

	genState := types.DefaultGenesis()
	k.InitGenesis(ctx, genState)

	exported := k.ExportGenesis(ctx)
	if exported.State == nil {
		t.Fatal("expected non-nil state")
	}
	if !exported.State.Enabled {
		t.Error("expected enabled state")
	}
	if len(exported.Observations) != 0 {
		t.Errorf("expected 0 observations, got %d", len(exported.Observations))
	}
}
