package keeper_test

import (
	"testing"

	sdkmath "cosmossdk.io/math"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── R56: Model Composition Tests ───────────────────────────────────────────

func TestCompositionParamsDefaults(t *testing.T) {
	k, ctx := setupKeeperForTest(t)

	params := k.GetCompositionParams(ctx)
	if params.MinComponentsForEnsemble != 2 {
		t.Errorf("default min components: got %d, want 2", params.MinComponentsForEnsemble)
	}
	if params.MaxComponentsPerEnsemble != 10 {
		t.Errorf("default max components: got %d, want 10", params.MaxComponentsPerEnsemble)
	}
	if params.DistillationMinCaptures != 1000 {
		t.Errorf("default min captures: got %d, want 1000", params.DistillationMinCaptures)
	}
}

func TestCompositionParamsValidation(t *testing.T) {
	k, ctx := setupKeeperForTest(t)

	valid := types.DefaultCompositionParams()
	if err := k.SetCompositionParams(ctx, valid); err != nil {
		t.Fatalf("valid params rejected: %v", err)
	}

	invalid := valid
	invalid.MinComponentsForEnsemble = 1
	if err := k.SetCompositionParams(ctx, invalid); err == nil {
		t.Error("should reject min_components < 2")
	}

	invalid2 := valid
	invalid2.MaxComponentsPerEnsemble = 1
	if err := k.SetCompositionParams(ctx, invalid2); err == nil {
		t.Error("should reject max < min")
	}
}

func TestEnsembleCRUD(t *testing.T) {
	k, ctx := setupKeeperForTest(t)

	ensemble := &types.ModelEnsemble{
		EnsembleID: "ens-test-1",
		Name:       "Multi-Domain Expert",
		Components: []types.EnsembleComponent{
			{ModelID: "model-math", Domain: "mathematics", Weight: "0.500000000000000000"},
			{ModelID: "model-bio", Domain: "biology", Weight: "0.500000000000000000"},
		},
		RoutingType:    types.RoutingDomain,
		BenchmarkScore: "0.850000000000000000",
		Domains:        []string{"mathematics", "biology"},
		Status:         "active",
		CreatedAt:      100,
		Creator:        "zerone1creator",
	}

	kvStore := k.StoreService().OpenKVStore(ctx)
	bz, _ := ensemble.Marshal()
	_ = kvStore.Set(types.ModelEnsembleKey(ensemble.EnsembleID), bz)
	_ = kvStore.Set(types.EnsembleByDomainKey("mathematics", ensemble.EnsembleID), []byte{0x01})
	_ = kvStore.Set(types.EnsembleByDomainKey("biology", ensemble.EnsembleID), []byte{0x01})

	got, found := k.GetModelEnsemble(ctx, "ens-test-1")
	if !found {
		t.Fatal("ensemble not found")
	}
	if got.Name != "Multi-Domain Expert" {
		t.Errorf("name: got %s", got.Name)
	}
	if len(got.Components) != 2 {
		t.Errorf("components: got %d, want 2", len(got.Components))
	}
	if got.RoutingType != types.RoutingDomain {
		t.Errorf("routing: got %s, want domain", got.RoutingType)
	}

	// By domain.
	mathEns := k.GetEnsemblesByDomain(ctx, "mathematics")
	if len(mathEns) != 1 {
		t.Errorf("expected 1 ensemble for math, got %d", len(mathEns))
	}
	bioEns := k.GetEnsemblesByDomain(ctx, "biology")
	if len(bioEns) != 1 {
		t.Errorf("expected 1 ensemble for bio, got %d", len(bioEns))
	}

	_, found = k.GetModelEnsemble(ctx, "nonexistent")
	if found {
		t.Error("should not find nonexistent")
	}
}

func TestDistillationJobCRUD(t *testing.T) {
	k, ctx := setupKeeperForTest(t)

	job := &types.DistillationJob{
		JobID:          "dist-test-1",
		EnsembleID:     "ens-1",
		CaptureCount:   5000,
		DomainsCovered: []string{"math", "physics"},
		Status:         "pending",
		StartAt:        100,
		MinBenchmark:   "0.600000000000000000",
	}

	kvStore := k.StoreService().OpenKVStore(ctx)
	bz, _ := job.Marshal()
	_ = kvStore.Set(types.DistillationJobKey(job.JobID), bz)

	got, found := k.GetDistillationJob(ctx, "dist-test-1")
	if !found {
		t.Fatal("job not found")
	}
	if got.CaptureCount != 5000 {
		t.Errorf("captures: got %d, want 5000", got.CaptureCount)
	}
	if got.Status != "pending" {
		t.Errorf("status: got %s, want pending", got.Status)
	}
}

func TestRoutingDecisionCRUD(t *testing.T) {
	k, ctx := setupKeeperForTest(t)

	decision := &types.RoutingDecision{
		DecisionID:    "route-1",
		EnsembleID:    "ens-1",
		SelectedModel: "model-math",
		Domain:        "mathematics",
		Confidence:    "0.950000000000000000",
		BlockHeight:   200,
	}

	kvStore := k.StoreService().OpenKVStore(ctx)
	bz, _ := decision.Marshal()
	_ = kvStore.Set(types.RoutingDecisionKey(decision.DecisionID), bz)

	got, found := k.GetRoutingDecision(ctx, "route-1")
	if !found {
		t.Fatal("decision not found")
	}
	if got.SelectedModel != "model-math" {
		t.Errorf("selected: got %s, want model-math", got.SelectedModel)
	}
	if got.Confidence != "0.950000000000000000" {
		t.Errorf("confidence: got %s", got.Confidence)
	}
}

func TestRoutingTypes(t *testing.T) {
	validTypes := []types.RoutingType{
		types.RoutingDomain,
		types.RoutingConfidence,
		types.RoutingVoting,
		types.RoutingCascade,
	}

	for _, rt := range validTypes {
		if !types.ValidRoutingTypes[rt] {
			t.Errorf("routing type %s should be valid", rt)
		}
	}

	if types.ValidRoutingTypes["invalid"] {
		t.Error("invalid routing type should not be valid")
	}
}

func TestEnsembleBenchmarkParsing(t *testing.T) {
	tests := []struct {
		name  string
		score string
		want  string
	}{
		{"normal", "0.850000000000000000", "0.850000000000000000"},
		{"zero", "0.000000000000000000", "0.000000000000000000"},
		{"empty", "", "0.000000000000000000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &types.ModelEnsemble{BenchmarkScore: tt.score}
			got := e.GetBenchmarkScore()
			expected, _ := sdkmath.LegacyNewDecFromStr(tt.want)
			if !got.Equal(expected) {
				t.Errorf("got %s, want %s", got, expected)
			}
		})
	}
}

func TestComponentWeight(t *testing.T) {
	comp := &types.EnsembleComponent{Weight: "0.750000000000000000"}
	got := comp.GetWeight()
	expected := sdkmath.LegacyNewDecWithPrec(75, 2)
	if !got.Equal(expected) {
		t.Errorf("got %s, want %s", got, expected)
	}

	empty := &types.EnsembleComponent{}
	if !empty.GetWeight().Equal(sdkmath.LegacyOneDec()) {
		t.Error("empty weight should default to 1.0")
	}
}

func TestEnsembleIDDerivation(t *testing.T) {
	id1 := types.DeriveEnsembleID("test", []string{"a", "b"})
	id2 := types.DeriveEnsembleID("test", []string{"a", "c"})
	id1Again := types.DeriveEnsembleID("test", []string{"a", "b"})

	if id1 != id1Again {
		t.Error("same inputs should produce same ID")
	}
	if id1 == id2 {
		t.Error("different inputs should produce different IDs")
	}
	if id1 == "" {
		t.Error("ID should not be empty")
	}
}

func TestEnsembleMarshal(t *testing.T) {
	ensemble := types.ModelEnsemble{
		EnsembleID:  "ens-marshal",
		Name:        "Test",
		RoutingType: types.RoutingVoting,
		Domains:     []string{"a", "b"},
	}

	bz, err := ensemble.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded types.ModelEnsemble
	if err := decoded.Unmarshal(bz); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.EnsembleID != "ens-marshal" {
		t.Errorf("ID: got %s", decoded.EnsembleID)
	}
	if decoded.RoutingType != types.RoutingVoting {
		t.Errorf("routing: got %s", decoded.RoutingType)
	}
}
