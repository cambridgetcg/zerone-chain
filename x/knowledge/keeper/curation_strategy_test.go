package keeper_test

import (
	"testing"

	sdkmath "cosmossdk.io/math"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── R54: Strategic Curation Tests ──────────────────────────────────────────

func TestCurationStrategyParamsDefaults(t *testing.T) {
	k, ctx := setupKeeperForTest(t)

	params := k.GetCurationStrategyParams(ctx)
	if params.MinTDUsForHealthy != 10 {
		t.Errorf("default min TDUs: got %d, want 10", params.MinTDUsForHealthy)
	}
	if params.StalenessThreshold != 100_000 {
		t.Errorf("default staleness: got %d, want 100000", params.StalenessThreshold)
	}
	if !params.AutoBountyEnabled {
		t.Error("auto bounty should be enabled by default")
	}
	if params.HealthComputeInterval != 5_000 {
		t.Errorf("default health interval: got %d, want 5000", params.HealthComputeInterval)
	}
}

func TestCurationStrategyParamsValidation(t *testing.T) {
	k, ctx := setupKeeperForTest(t)

	// Valid.
	valid := types.DefaultCurationStrategyParams()
	if err := k.SetCurationStrategyParams(ctx, valid); err != nil {
		t.Fatalf("valid params rejected: %v", err)
	}

	// Invalid: fitness > 1.
	invalid := valid
	invalid.MinFitnessForHealthy = "2.000000000000000000"
	if err := k.SetCurationStrategyParams(ctx, invalid); err == nil {
		t.Error("should reject min fitness > 1")
	}

	// Invalid: negative severity.
	invalid2 := valid
	invalid2.AutoBountySeverityMin = "-0.100000000000000000"
	if err := k.SetCurationStrategyParams(ctx, invalid2); err == nil {
		t.Error("should reject negative severity")
	}
}

func TestKnowledgeGapCRUD(t *testing.T) {
	k, ctx := setupKeeperForTest(t)

	gap := &types.KnowledgeGap{
		GapID:       "gap-test-1",
		Domain:      "mathematics",
		GapType:     types.GapTypeCoverage,
		Description: "too few TDUs in mathematics",
		Severity:    "0.800000000000000000",
		DetectedBy:  "agent-scout-1",
		DetectedAt:  100,
		Status:      "open",
	}

	// Store directly.
	kvStore := k.StoreService().OpenKVStore(ctx)
	bz, _ := gap.Marshal()
	_ = kvStore.Set(types.KnowledgeGapKey(gap.GapID), bz)
	_ = kvStore.Set(types.KnowledgeGapByDomainKey(gap.Domain, gap.GapID), []byte{0x01})
	_ = kvStore.Set(types.KnowledgeGapOpenKey(gap.GapID), []byte{0x01})

	// Get.
	got, found := k.GetKnowledgeGap(ctx, "gap-test-1")
	if !found {
		t.Fatal("gap not found")
	}
	if got.Domain != "mathematics" {
		t.Errorf("domain: got %s, want mathematics", got.Domain)
	}
	if got.GapType != types.GapTypeCoverage {
		t.Errorf("gap type: got %s, want coverage", got.GapType)
	}
	if got.Status != "open" {
		t.Errorf("status: got %s, want open", got.Status)
	}

	// Get by domain.
	domainGaps := k.GetOpenGapsByDomain(ctx, "mathematics")
	if len(domainGaps) != 1 {
		t.Fatalf("expected 1 gap for mathematics, got %d", len(domainGaps))
	}

	// Not found.
	_, found = k.GetKnowledgeGap(ctx, "nonexistent")
	if found {
		t.Error("should not find nonexistent gap")
	}
}

func TestDomainHealthCRUD(t *testing.T) {
	k, ctx := setupKeeperForTest(t)

	health := &types.DomainHealth{
		Domain:             "physics",
		TotalTDUs:          25,
		ActiveTDUs:         20,
		AvgFitness:         "0.650000000000000000",
		OrphanCount:        5,
		ContradictionCount: 2,
		HealthScore:        "0.720000000000000000",
		ComputedAt:         200,
	}

	// Store.
	kvStore := k.StoreService().OpenKVStore(ctx)
	bz, _ := health.Marshal()
	_ = kvStore.Set(types.DomainHealthKey(health.Domain), bz)

	// Get.
	got, found := k.GetDomainHealth(ctx, "physics")
	if !found {
		t.Fatal("health not found")
	}
	if got.TotalTDUs != 25 {
		t.Errorf("total TDUs: got %d, want 25", got.TotalTDUs)
	}
	if got.OrphanCount != 5 {
		t.Errorf("orphans: got %d, want 5", got.OrphanCount)
	}
	if got.HealthScore != "0.720000000000000000" {
		t.Errorf("health score: got %s, want 0.72", got.HealthScore)
	}

	// Not found.
	_, found = k.GetDomainHealth(ctx, "nonexistent")
	if found {
		t.Error("should not find nonexistent domain health")
	}
}

func TestCurationStrategyCRUD(t *testing.T) {
	k, ctx := setupKeeperForTest(t)

	strategy := &types.CurationStrategy{
		StrategyID:    "strat-test-1",
		AgentID:       "agent-strategist",
		FocusDomains:  []string{"biology", "chemistry"},
		Priorities:    []types.GapType{types.GapTypeCoverage, types.GapTypeFitness},
		GapsIdentified: 10,
		GapsFilled:     7,
		Effectiveness:  "0.700000000000000000",
		CreatedAt:      100,
		UpdatedAt:      200,
	}

	// Store.
	kvStore := k.StoreService().OpenKVStore(ctx)
	bz, _ := strategy.Marshal()
	_ = kvStore.Set(types.CurationStrategyKey(strategy.StrategyID), bz)
	_ = kvStore.Set(types.CurationStrategyByAgentKey(strategy.AgentID, strategy.StrategyID), []byte{0x01})

	// Get.
	got, found := k.GetCurationStrategy(ctx, "strat-test-1")
	if !found {
		t.Fatal("strategy not found")
	}
	if got.AgentID != "agent-strategist" {
		t.Errorf("agent ID: got %s, want agent-strategist", got.AgentID)
	}
	if len(got.FocusDomains) != 2 {
		t.Errorf("focus domains: got %d, want 2", len(got.FocusDomains))
	}
	if got.Effectiveness != "0.700000000000000000" {
		t.Errorf("effectiveness: got %s, want 0.7", got.Effectiveness)
	}

	// Get by agent.
	agentStrats := k.GetStrategiesByAgent(ctx, "agent-strategist")
	if len(agentStrats) != 1 {
		t.Fatalf("expected 1 strategy for agent, got %d", len(agentStrats))
	}
}

func TestGapSeverityParsing(t *testing.T) {
	tests := []struct {
		name     string
		severity string
		want     string
	}{
		{"high", "0.900000000000000000", "0.900000000000000000"},
		{"medium", "0.500000000000000000", "0.500000000000000000"},
		{"zero", "0.000000000000000000", "0.000000000000000000"},
		{"empty", "", "0.000000000000000000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gap := &types.KnowledgeGap{Severity: tt.severity}
			got := gap.GetSeverity()
			expected, _ := sdkmath.LegacyNewDecFromStr(tt.want)
			if !got.Equal(expected) {
				t.Errorf("got %s, want %s", got, expected)
			}
		})
	}
}

func TestStrategyEffectiveness(t *testing.T) {
	tests := []struct {
		name          string
		effectiveness string
		want          string
	}{
		{"good", "0.800000000000000000", "0.800000000000000000"},
		{"zero", "0.000000000000000000", "0.000000000000000000"},
		{"empty", "", "0.000000000000000000"},
		{"perfect", "1.000000000000000000", "1.000000000000000000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &types.CurationStrategy{Effectiveness: tt.effectiveness}
			got := s.GetEffectiveness()
			expected, _ := sdkmath.LegacyNewDecFromStr(tt.want)
			if !got.Equal(expected) {
				t.Errorf("got %s, want %s", got, expected)
			}
		})
	}
}

func TestGapTypeValidation(t *testing.T) {
	// All valid types should be recognized.
	validTypes := []types.GapType{
		types.GapTypeCoverage,
		types.GapTypeFitness,
		types.GapTypeConnectivity,
		types.GapTypeContradiction,
		types.GapTypeStale,
		types.GapTypeDepth,
	}

	for _, gt := range validTypes {
		if !types.ValidGapTypes[gt] {
			t.Errorf("gap type %s should be valid", gt)
		}
	}

	// Invalid type should not be recognized.
	if types.ValidGapTypes["nonexistent"] {
		t.Error("nonexistent gap type should be invalid")
	}
}

func TestDomainHealthMarshal(t *testing.T) {
	health := types.DomainHealth{
		Domain:       "test",
		TotalTDUs:    100,
		ActiveTDUs:   95,
		AvgFitness:   "0.750000000000000000",
		HealthScore:  "0.820000000000000000",
		OrphanCount:  3,
	}

	bz, err := health.Marshal()
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded types.DomainHealth
	if err := decoded.Unmarshal(bz); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.TotalTDUs != 100 {
		t.Errorf("total TDUs: got %d, want 100", decoded.TotalTDUs)
	}
	if decoded.HealthScore != "0.820000000000000000" {
		t.Errorf("health score: got %s", decoded.HealthScore)
	}
}

func TestDefaultCurationParamsValid(t *testing.T) {
	params := types.DefaultCurationStrategyParams()
	if err := params.Validate(); err != nil {
		t.Fatalf("default params should be valid: %v", err)
	}
}

func TestFillGapMarksAsFilled(t *testing.T) {
	k, ctx := setupKeeperForTest(t)

	// Create an open gap.
	gap := &types.KnowledgeGap{
		GapID:      "gap-fill-test",
		Domain:     "cs",
		GapType:    types.GapTypeCoverage,
		Severity:   "0.700000000000000000",
		DetectedBy: "protocol",
		DetectedAt: 100,
		Status:     "open",
	}

	kvStore := k.StoreService().OpenKVStore(ctx)
	bz, _ := gap.Marshal()
	_ = kvStore.Set(types.KnowledgeGapKey(gap.GapID), bz)
	_ = kvStore.Set(types.KnowledgeGapOpenKey(gap.GapID), []byte{0x01})

	// Fill it.
	err := k.FillGap(ctx, "gap-fill-test")
	if err != nil {
		t.Fatalf("FillGap failed: %v", err)
	}

	// Verify status changed.
	filled, found := k.GetKnowledgeGap(ctx, "gap-fill-test")
	if !found {
		t.Fatal("gap not found after fill")
	}
	if filled.Status != "filled" {
		t.Errorf("status: got %s, want filled", filled.Status)
	}

	// Open index should be removed.
	openBz, err := kvStore.Get(types.KnowledgeGapOpenKey("gap-fill-test"))
	if err == nil && openBz != nil {
		t.Error("gap should be removed from open index after fill")
	}
}
