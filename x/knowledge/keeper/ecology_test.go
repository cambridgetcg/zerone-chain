package keeper_test

import (
	"testing"

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
