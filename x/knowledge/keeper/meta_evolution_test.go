package keeper_test

import (
	"testing"

	sdkmath "cosmossdk.io/math"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── R57: Meta-Evolution Tests ──────────────────────────────────────────────

func TestMetaEvolutionParamsDefaults(t *testing.T) {
	k, ctx := setupKeeperForTest(t)

	params := k.GetMetaEvolutionParams(ctx)
	if params.EpochDurationBlocks != 10_000 {
		t.Errorf("default epoch duration: got %d, want 10000", params.EpochDurationBlocks)
	}
	if params.MinStrategiesPerEpoch != 2 {
		t.Errorf("default min strategies: got %d, want 2", params.MinStrategiesPerEpoch)
	}
	if params.MaxParamHistory != 20 {
		t.Errorf("default max history: got %d, want 20", params.MaxParamHistory)
	}
}

func TestMetaEvolutionParamsValidation(t *testing.T) {
	k, ctx := setupKeeperForTest(t)

	valid := types.DefaultMetaEvolutionParams()
	if err := k.SetMetaEvolutionParams(ctx, valid); err != nil {
		t.Fatalf("valid params rejected: %v", err)
	}

	invalid := valid
	invalid.EpochDurationBlocks = 0
	if err := k.SetMetaEvolutionParams(ctx, invalid); err == nil {
		t.Error("should reject epoch_duration = 0")
	}

	invalid2 := valid
	invalid2.MinStrategiesPerEpoch = 0
	if err := k.SetMetaEvolutionParams(ctx, invalid2); err == nil {
		t.Error("should reject min_strategies = 0")
	}

	invalid3 := valid
	invalid3.DefaultStepSize = "2.000000000000000000"
	if err := k.SetMetaEvolutionParams(ctx, invalid3); err == nil {
		t.Error("should reject step_size > 1")
	}
}

func TestEvolutionEpochCRUD(t *testing.T) {
	k, ctx := setupKeeperForTest(t)

	epoch := &types.EvolutionEpoch{
		EpochID:    "epoch-test-1",
		Domain:     "mathematics",
		StartBlock: 100,
		EndBlock:   10100,
		Strategies: []types.StrategyOutcome{
			{StrategyID: "strat-1", AgentID: "agent-1", Score: "0.800000000000000000"},
			{StrategyID: "strat-2", AgentID: "agent-2", Score: "0.600000000000000000"},
		},
		WinnerStrategyID: "strat-1",
		Status:           "completed",
	}

	kvStore := k.StoreService().OpenKVStore(ctx)
	bz, _ := epoch.Marshal()
	_ = kvStore.Set(types.EvolutionEpochKey(epoch.EpochID), bz)
	_ = kvStore.Set(types.EpochByDomainKey(epoch.Domain, epoch.EpochID), []byte{0x01})

	got, found := k.GetEvolutionEpoch(ctx, "epoch-test-1")
	if !found {
		t.Fatal("epoch not found")
	}
	if got.Domain != "mathematics" {
		t.Errorf("domain: got %s", got.Domain)
	}
	if len(got.Strategies) != 2 {
		t.Errorf("strategies: got %d, want 2", len(got.Strategies))
	}
	if got.WinnerStrategyID != "strat-1" {
		t.Errorf("winner: got %s, want strat-1", got.WinnerStrategyID)
	}

	// By domain.
	epochs := k.GetEpochsByDomain(ctx, "mathematics")
	if len(epochs) != 1 {
		t.Errorf("expected 1 epoch, got %d", len(epochs))
	}

	_, found = k.GetEvolutionEpoch(ctx, "nonexistent")
	if found {
		t.Error("should not find nonexistent")
	}
}

func TestMetaParameterCRUD(t *testing.T) {
	k, ctx := setupKeeperForTest(t)

	param := &types.MetaParameter{
		ParamID:      "fitness-weight",
		Name:         "Fitness Score Weight",
		Domain:       "science",
		CurrentValue: "0.500000000000000000",
		MinValue:     "0.100000000000000000",
		MaxValue:     "0.900000000000000000",
		StepSize:     "0.050000000000000000",
		History: []types.MetaParamTrial{
			{Value: "0.450000000000000000", EpochID: "epoch-1", Better: true},
		},
	}

	if err := k.SetMetaParameter(ctx, param); err != nil {
		t.Fatalf("SetMetaParameter failed: %v", err)
	}

	got, found := k.GetMetaParameter(ctx, "fitness-weight")
	if !found {
		t.Fatal("param not found")
	}
	if got.CurrentValue != "0.500000000000000000" {
		t.Errorf("value: got %s", got.CurrentValue)
	}
	if got.Name != "Fitness Score Weight" {
		t.Errorf("name: got %s", got.Name)
	}
	if len(got.History) != 1 {
		t.Errorf("history: got %d, want 1", len(got.History))
	}

	// By domain.
	domainParams := k.GetMetaParametersByDomain(ctx, "science")
	if len(domainParams) != 1 {
		t.Errorf("expected 1 param for science, got %d", len(domainParams))
	}
}

func TestStrategyOutcomeScore(t *testing.T) {
	tests := []struct {
		name  string
		score string
		want  string
	}{
		{"normal", "0.750000000000000000", "0.750000000000000000"},
		{"zero", "0.000000000000000000", "0.000000000000000000"},
		{"empty", "", "0.000000000000000000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			so := &types.StrategyOutcome{Score: tt.score}
			got := so.GetScore()
			expected, _ := sdkmath.LegacyNewDecFromStr(tt.want)
			if !got.Equal(expected) {
				t.Errorf("got %s, want %s", got, expected)
			}
		})
	}
}

func TestMetaParamCurrentValue(t *testing.T) {
	param := &types.MetaParameter{CurrentValue: "0.650000000000000000"}
	got := param.GetCurrentValue()
	expected := sdkmath.LegacyNewDecWithPrec(65, 2)
	if !got.Equal(expected) {
		t.Errorf("got %s, want %s", got, expected)
	}

	empty := &types.MetaParameter{}
	if !empty.GetCurrentValue().Equal(sdkmath.LegacyZeroDec()) {
		t.Error("empty should be zero")
	}
}

func TestCurrentEpochTracking(t *testing.T) {
	k, ctx := setupKeeperForTest(t)

	// Initially no current epoch.
	_, hasActive := k.GetCurrentEpochID(ctx, "test-domain")
	if hasActive {
		t.Error("should have no active epoch initially")
	}
}

func TestEpochMarshal(t *testing.T) {
	epoch := types.EvolutionEpoch{
		EpochID:    "epoch-marshal",
		Domain:     "test",
		StartBlock: 100,
		EndBlock:   200,
		Status:     "active",
		Insights:   []string{"test insight"},
	}

	bz, err := epoch.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded types.EvolutionEpoch
	if err := decoded.Unmarshal(bz); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.EpochID != "epoch-marshal" {
		t.Errorf("ID: got %s", decoded.EpochID)
	}
	if len(decoded.Insights) != 1 {
		t.Errorf("insights: got %d", len(decoded.Insights))
	}
}

func TestDefaultMetaEvolutionParamsValid(t *testing.T) {
	params := types.DefaultMetaEvolutionParams()
	if err := params.Validate(); err != nil {
		t.Fatalf("defaults should be valid: %v", err)
	}
}
