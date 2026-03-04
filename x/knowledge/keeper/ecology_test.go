package keeper_test

import (
	"testing"

	"github.com/zerone-chain/zerone/x/knowledge/keeper"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── State Infrastructure Tests ─────────────────────────────────────────────

func TestIterateSamples(t *testing.T) {
	k, ctx := setupKeeper(t)

	s1 := &types.Sample{Id: "1", Domain: "tech", Content: "a"}
	s2 := &types.Sample{Id: "2", Domain: "sci", Content: "b"}
	s3 := &types.Sample{Id: "3", Domain: "tech", Content: "c"}
	_ = k.SetSample(ctx, s1)
	_ = k.SetSample(ctx, s2)
	_ = k.SetSample(ctx, s3)

	var collected []string
	k.IterateSamples(ctx, func(s *types.Sample) bool {
		collected = append(collected, s.Id)
		return false
	})
	if len(collected) != 3 {
		t.Fatalf("expected 3 samples, got %d", len(collected))
	}
}

func TestIterateSamples_EarlyStop(t *testing.T) {
	k, ctx := setupKeeper(t)

	_ = k.SetSample(ctx, &types.Sample{Id: "1", Content: "a"})
	_ = k.SetSample(ctx, &types.Sample{Id: "2", Content: "b"})

	count := 0
	k.IterateSamples(ctx, func(s *types.Sample) bool {
		count++
		return true // stop after first
	})
	if count != 1 {
		t.Fatalf("expected 1, got %d", count)
	}
}

func TestNicheIndex_SetAndGet(t *testing.T) {
	k, ctx := setupKeeper(t)

	_ = k.SetNicheIndex(ctx, "niche_abc", "sample_1")
	_ = k.SetNicheIndex(ctx, "niche_abc", "sample_2")
	_ = k.SetNicheIndex(ctx, "niche_xyz", "sample_3")

	ids := k.GetSamplesByNiche(ctx, "niche_abc")
	if len(ids) != 2 {
		t.Fatalf("expected 2 samples in niche_abc, got %d", len(ids))
	}

	ids2 := k.GetSamplesByNiche(ctx, "niche_xyz")
	if len(ids2) != 1 {
		t.Fatalf("expected 1 sample in niche_xyz, got %d", len(ids2))
	}
}

func TestNicheIndex_Delete(t *testing.T) {
	k, ctx := setupKeeper(t)

	_ = k.SetNicheIndex(ctx, "niche_abc", "sample_1")
	_ = k.SetNicheIndex(ctx, "niche_abc", "sample_2")
	_ = k.DeleteNicheIndex(ctx, "niche_abc", "sample_1")

	ids := k.GetSamplesByNiche(ctx, "niche_abc")
	if len(ids) != 1 {
		t.Fatalf("expected 1 after delete, got %d", len(ids))
	}
}

func TestAtRiskIndex_SetIterateDelete(t *testing.T) {
	k, ctx := setupKeeper(t)

	_ = k.SetAtRiskIndex(ctx, "sample_1")
	_ = k.SetAtRiskIndex(ctx, "sample_2")

	var atRisk []string
	k.IterateAtRiskSamples(ctx, func(sampleID string) bool {
		atRisk = append(atRisk, sampleID)
		return false
	})
	if len(atRisk) != 2 {
		t.Fatalf("expected 2 at-risk, got %d", len(atRisk))
	}

	_ = k.DeleteAtRiskIndex(ctx, "sample_1")
	atRisk = nil
	k.IterateAtRiskSamples(ctx, func(sampleID string) bool {
		atRisk = append(atRisk, sampleID)
		return false
	})
	if len(atRisk) != 1 {
		t.Fatalf("expected 1 after delete, got %d", len(atRisk))
	}
}

func TestTopicSaturation_IncrementAndGet(t *testing.T) {
	k, ctx := setupKeeper(t)

	_ = k.IncrementTopicCount(ctx, "tech", "golang")
	_ = k.IncrementTopicCount(ctx, "tech", "golang")
	_ = k.IncrementTopicCount(ctx, "tech", "golang")
	_ = k.IncrementTopicCount(ctx, "sci", "physics")

	count := k.GetTopicCount(ctx, "tech", "golang")
	if count != 3 {
		t.Fatalf("expected 3, got %d", count)
	}

	count2 := k.GetTopicCount(ctx, "sci", "physics")
	if count2 != 1 {
		t.Fatalf("expected 1, got %d", count2)
	}
}

func TestTopicSaturation_UnknownIsZero(t *testing.T) {
	k, ctx := setupKeeper(t)
	count := k.GetTopicCount(ctx, "unknown", "topic")
	if count != 0 {
		t.Fatalf("expected 0 for unknown, got %d", count)
	}
}

// ─── Fitness Scoring Tests ──────────────────────────────────────────────────

func TestComputeSampleFitness_AllMax(t *testing.T) {
	k, ctx := setupKeeper(t)
	params := types.DefaultParams()

	sample := &types.Sample{
		QualityScore:   1_000_000,
		AccessCount:    1000,
		NoveltyScore:   1_000_000,
		DiversityScore: 1_000_000,
		ReasoningDepth: 1_000_000,
		TotalRevenue:   "1000000000",
	}

	fitness := k.ComputeSampleFitness(ctx, sample, &params)
	if fitness != 1_000_000 {
		t.Fatalf("expected 1,000,000 for all-max, got %d", fitness)
	}
}

func TestComputeSampleFitness_AllZero(t *testing.T) {
	k, ctx := setupKeeper(t)
	params := types.DefaultParams()

	sample := &types.Sample{
		TotalRevenue: "0",
	}
	fitness := k.ComputeSampleFitness(ctx, sample, &params)
	if fitness != 0 {
		t.Fatalf("expected 0 for all-zero, got %d", fitness)
	}
}

func TestComputeSampleFitness_MixedValues(t *testing.T) {
	k, ctx := setupKeeper(t)
	params := types.DefaultParams()

	sample := &types.Sample{
		QualityScore:   800_000,
		AccessCount:    500,
		NoveltyScore:   600_000,
		DiversityScore: 400_000,
		ReasoningDepth: 300_000,
		TotalRevenue:   "200000000",
	}

	// quality*25 + access*25 + novelty*20 + diversity*10 + reasoning*10 + revenue*10
	// 800000*25 + 500000*25 + 600000*20 + 400000*10 + 300000*10 + 200000*10
	// = 20000000 + 12500000 + 12000000 + 4000000 + 3000000 + 2000000 = 53500000
	// / 100 = 535000
	expected := uint64(535_000)
	fitness := k.ComputeSampleFitness(ctx, sample, &params)
	if fitness != expected {
		t.Fatalf("expected %d, got %d", expected, fitness)
	}
}

func TestComputeSampleFitness_OverMaxAccess_ClampedToMax(t *testing.T) {
	k, ctx := setupKeeper(t)
	params := types.DefaultParams()

	sample := &types.Sample{
		AccessCount:  5000,
		TotalRevenue: "0",
	}
	fitness := k.ComputeSampleFitness(ctx, sample, &params)
	// access component clamped to 1,000,000: 1,000,000 * 25 / 100 = 250,000
	if fitness != 250_000 {
		t.Fatalf("expected 250000 with clamped access, got %d", fitness)
	}
}

func TestNormalize(t *testing.T) {
	tests := []struct {
		name     string
		value    uint64
		max      uint64
		expected uint64
	}{
		{"zero", 0, 1000, 0},
		{"half", 500, 1000, 500_000},
		{"full", 1000, 1000, 1_000_000},
		{"over", 2000, 1000, 1_000_000},
		{"max_zero", 100, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := keeper.Normalize(tt.value, tt.max)
			if got != tt.expected {
				t.Fatalf("Normalize(%d, %d) = %d, want %d", tt.value, tt.max, got, tt.expected)
			}
		})
	}
}

func TestParseUzrn(t *testing.T) {
	tests := []struct {
		input    string
		expected uint64
	}{
		{"0", 0},
		{"1000000", 1_000_000},
		{"", 0},
		{"invalid", 0},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := keeper.ParseUzrn(tt.input)
			if got != tt.expected {
				t.Fatalf("ParseUzrn(%q) = %d, want %d", tt.input, got, tt.expected)
			}
		})
	}
}
