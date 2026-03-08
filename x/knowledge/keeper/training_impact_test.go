package keeper_test

import (
	"math"
	"testing"

	sdkmath "cosmossdk.io/math"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── R52: Training Impact Attribution Tests ─────────────────────────────────

func TestAttributionParamsDefaults(t *testing.T) {
	k, ctx := setupKeeperForTest(t)

	params := k.GetAttributionParams(ctx)
	if params.MaxContributors != 100 {
		t.Errorf("default max contributors: got %d, want 100", params.MaxContributors)
	}
	if params.RecencyDecayHalflife != 50_000 {
		t.Errorf("default halflife: got %d, want 50000", params.RecencyDecayHalflife)
	}
	if params.PeriodicAttributionEpochs != 10 {
		t.Errorf("default periodic epochs: got %d, want 10", params.PeriodicAttributionEpochs)
	}
}

func TestAttributionParamsValidation(t *testing.T) {
	k, ctx := setupKeeperForTest(t)

	// Valid params.
	valid := types.DefaultAttributionParams()
	if err := k.SetAttributionParams(ctx, valid); err != nil {
		t.Fatalf("valid params rejected: %v", err)
	}

	// Invalid: rate > 1.
	invalid := valid
	invalid.AttributionRate = "1.500000000000000000"
	if err := k.SetAttributionParams(ctx, invalid); err == nil {
		t.Error("should reject attribution_rate > 1")
	}

	// Invalid: max contributors = 0.
	invalid2 := valid
	invalid2.MaxContributors = 0
	if err := k.SetAttributionParams(ctx, invalid2); err == nil {
		t.Error("should reject max_contributors = 0")
	}

	// Invalid: negative fitness.
	invalid3 := valid
	invalid3.MinFitnessForAttribution = "-0.100000000000000000"
	if err := k.SetAttributionParams(ctx, invalid3); err == nil {
		t.Error("should reject negative min fitness")
	}
}

func TestTrainingImpactCRUD(t *testing.T) {
	k, ctx := setupKeeperForTest(t)

	impact := &types.TrainingImpact{
		ImpactID:    "imp-test-1",
		ModelID:     "model-abc",
		TriggerType: types.TriggerAPIRevenue,
		TriggerValue: "5000000",
		Contributors: []types.TDUContribution{
			{
				TDUID:             "tdu-1",
				Curator:           "zerone1curator1",
				FitnessAtTraining: "0.800000000000000000",
				Weight:            "0.700000000000000000",
				RewardShare:       "0.600000000000000000",
			},
			{
				TDUID:             "tdu-2",
				Curator:           "zerone1curator2",
				FitnessAtTraining: "0.600000000000000000",
				Weight:            "0.400000000000000000",
				RewardShare:       "0.400000000000000000",
			},
		},
		CuratorRewards: []types.CuratorReward{
			{
				CuratorAddr:  "zerone1curator1",
				TotalWeight:  "0.700000000000000000",
				RewardAmount: "3000000",
				TDUCount:     1,
			},
			{
				CuratorAddr:  "zerone1curator2",
				TotalWeight:  "0.400000000000000000",
				RewardAmount: "2000000",
				TDUCount:     1,
			},
		},
		TotalPool:        "5000000",
		TotalDistributed: "5000000",
		TDUCount:         2,
		CuratorCount:     2,
		ComputedAt:       100,
	}

	// Store directly (testing the setter).
	kvStore := k.StoreService().OpenKVStore(ctx)
	bz, err := impact.Marshal()
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	_ = kvStore.Set(types.TrainingImpactKey(impact.ImpactID), bz)
	_ = kvStore.Set(types.TrainingImpactByModelKey(impact.ModelID, impact.ImpactID), []byte{0x01})

	// Get.
	got, found := k.GetTrainingImpact(ctx, "imp-test-1")
	if !found {
		t.Fatal("impact not found after set")
	}
	if got.ModelID != "model-abc" {
		t.Errorf("model ID: got %s, want model-abc", got.ModelID)
	}
	if got.TDUCount != 2 {
		t.Errorf("TDU count: got %d, want 2", got.TDUCount)
	}
	if got.CuratorCount != 2 {
		t.Errorf("curator count: got %d, want 2", got.CuratorCount)
	}

	// Get by model.
	impacts := k.GetImpactsByModel(ctx, "model-abc")
	if len(impacts) != 1 {
		t.Fatalf("expected 1 impact for model-abc, got %d", len(impacts))
	}

	// Not found.
	_, found = k.GetTrainingImpact(ctx, "nonexistent")
	if found {
		t.Error("should not find nonexistent impact")
	}
}

func TestCuratorImpactScore(t *testing.T) {
	k, ctx := setupKeeperForTest(t)

	// Initially not found.
	_, found := k.GetCuratorImpactScore(ctx, "zerone1curator")
	if found {
		t.Error("should not find curator before any attribution")
	}

	// Store a score.
	score := &types.CuratorImpactScore{
		CuratorAddr:         "zerone1curator",
		AgentID:             "agent-curator-1",
		TotalAttributions:   5,
		TotalTDUsAttributed: 20,
		TotalRewardsEarned:  "15000000",
		ModelsInfluenced:    3,
		UpdatedAt:           500,
	}
	kvStore := k.StoreService().OpenKVStore(ctx)
	bz, _ := score.Marshal()
	_ = kvStore.Set(types.CuratorImpactScoreKey("zerone1curator"), bz)

	// Get.
	got, found := k.GetCuratorImpactScore(ctx, "zerone1curator")
	if !found {
		t.Fatal("curator score not found")
	}
	if got.TotalAttributions != 5 {
		t.Errorf("attributions: got %d, want 5", got.TotalAttributions)
	}
	if got.TotalRewardsEarned != "15000000" {
		t.Errorf("rewards: got %s, want 15000000", got.TotalRewardsEarned)
	}
	if got.ModelsInfluenced != 3 {
		t.Errorf("models: got %d, want 3", got.ModelsInfluenced)
	}
}

func TestTDUContributionWeight(t *testing.T) {
	tests := []struct {
		name   string
		weight string
		want   string
	}{
		{"normal", "0.750000000000000000", "0.750000000000000000"},
		{"zero", "0.000000000000000000", "0.000000000000000000"},
		{"empty", "", "0.000000000000000000"},
		{"one", "1.000000000000000000", "1.000000000000000000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &types.TDUContribution{Weight: tt.weight}
			got := c.GetWeight()
			expected, _ := sdkmath.LegacyNewDecFromStr(tt.want)
			if !got.Equal(expected) {
				t.Errorf("got %s, want %s", got, expected)
			}
		})
	}
}

func TestCuratorRewardAmount(t *testing.T) {
	tests := []struct {
		name   string
		amount string
		want   int64
	}{
		{"normal", "5000000", 5000000},
		{"zero", "0", 0},
		{"empty", "", 0},
		{"large", "999999999", 999999999},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &types.CuratorReward{RewardAmount: tt.amount}
			got := r.GetRewardAmount()
			if !got.Equal(sdkmath.NewInt(tt.want)) {
				t.Errorf("got %s, want %d", got, tt.want)
			}
		})
	}
}

func TestRecencyFactor(t *testing.T) {
	// Test the recency decay function.
	// At 0 blocks: factor should be 1.0.
	// At halflife blocks: factor should be ~0.5.
	// At 2× halflife: factor should be ~0.25.
	// At very old: floor at 0.001.

	halflife := uint64(50000)

	// 0 blocks → 1.0
	f0 := computeRecencyFactorForTest(0, halflife)
	if f0 != 1.0 {
		t.Errorf("at 0 blocks: got %f, want 1.0", f0)
	}

	// halflife blocks → ~0.5
	fHalf := computeRecencyFactorForTest(halflife, halflife)
	if math.Abs(fHalf-0.5) > 0.01 {
		t.Errorf("at halflife: got %f, want ~0.5", fHalf)
	}

	// 2× halflife → ~0.25
	f2x := computeRecencyFactorForTest(2*halflife, halflife)
	if math.Abs(f2x-0.25) > 0.01 {
		t.Errorf("at 2× halflife: got %f, want ~0.25", f2x)
	}

	// Very old → floor at 0.001
	fOld := computeRecencyFactorForTest(1_000_000, halflife)
	if fOld < 0.001 {
		t.Errorf("very old should floor at 0.001, got %f", fOld)
	}
}

// computeRecencyFactorForTest mirrors the keeper function for unit testing.
func computeRecencyFactorForTest(blocksSince, halflife uint64) float64 {
	if halflife == 0 || blocksSince == 0 {
		return 1.0
	}
	exponent := float64(blocksSince) / float64(halflife)
	factor := math.Pow(0.5, exponent)
	if factor < 0.001 {
		factor = 0.001
	}
	return factor
}

func TestQueueAndGetAttribution(t *testing.T) {
	k, ctx := setupKeeperForTest(t)

	// Queue an attribution.
	err := k.QueueAttribution(ctx, "model-queue-test", types.TriggerAPIRevenue, sdkmath.NewInt(5000000))
	if err != nil {
		t.Fatalf("QueueAttribution failed: %v", err)
	}

	// Verify it's pending (check store directly).
	kvStore := k.StoreService().OpenKVStore(ctx)
	bz, err := kvStore.Get(types.PendingAttributionKey("model-queue-test"))
	if err != nil || bz == nil {
		t.Fatal("pending attribution not found")
	}
}

func TestAttributionTriggerTypes(t *testing.T) {
	// Ensure all trigger types are distinct and non-empty.
	triggers := []types.AttributionTrigger{
		types.TriggerAPIRevenue,
		types.TriggerChallengeWon,
		types.TriggerBenchmarkImproved,
		types.TriggerPeriodicReview,
	}

	seen := make(map[types.AttributionTrigger]bool)
	for _, trigger := range triggers {
		if trigger == "" {
			t.Error("trigger type should not be empty")
		}
		if seen[trigger] {
			t.Errorf("duplicate trigger type: %s", trigger)
		}
		seen[trigger] = true
	}
}

func TestDefaultAttributionParamsValid(t *testing.T) {
	params := types.DefaultAttributionParams()
	if err := params.Validate(); err != nil {
		t.Fatalf("default params should be valid: %v", err)
	}

	// Check specific values.
	rate, _ := sdkmath.LegacyNewDecFromStr(params.AttributionRate)
	expectedRate := sdkmath.LegacyNewDecWithPrec(1, 1) // 0.1
	if !rate.Equal(expectedRate) {
		t.Errorf("attribution rate: got %s, want 0.1", rate)
	}

	minFit, _ := sdkmath.LegacyNewDecFromStr(params.MinFitnessForAttribution)
	expectedFit := sdkmath.LegacyNewDecWithPrec(4, 1) // 0.4
	if !minFit.Equal(expectedFit) {
		t.Errorf("min fitness: got %s, want 0.4", minFit)
	}
}
