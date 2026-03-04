package keeper

import (
	"context"
	"strconv"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Constants ──────────────────────────────────────────────────────────────

const (
	// EcologyEpochBlocks is the interval at which ecology processing runs.
	EcologyEpochBlocks = 100

	// DefaultEnergyCap is the initial energy cap for new samples.
	DefaultEnergyCap uint64 = 1_000_000

	// maxAccess is the normalization ceiling for access count.
	maxAccess uint64 = 1000

	// maxRevenue is the normalization ceiling for total revenue (in uzrn).
	maxRevenue uint64 = 1_000_000_000 // 1000 ZRN

	// maxThreadBonus caps the thread fitness bonus at 30%.
	maxThreadBonus uint64 = 300_000

	// threadBonusPerMessage is the bonus per thread message.
	threadBonusPerMessage uint64 = 50_000
)

// ─── Fitness Scoring ────────────────────────────────────────────────────────

// ComputeSampleFitness calculates the weighted fitness score for a sample.
// Returns a value in 0–1,000,000 BPS range.
func (k Keeper) ComputeSampleFitness(ctx context.Context, sample *types.Sample, params *types.Params) uint64 {
	qualityComponent := sample.QualityScore
	accessComponent := normalize(sample.AccessCount, maxAccess)
	noveltyComponent := sample.NoveltyScore
	diversityComponent := sample.DiversityScore
	reasoningComponent := sample.ReasoningDepth
	revenueComponent := normalize(parseUzrn(sample.TotalRevenue), maxRevenue)

	fitness := (qualityComponent*25 +
		accessComponent*25 +
		noveltyComponent*20 +
		diversityComponent*10 +
		reasoningComponent*10 +
		revenueComponent*10) / 100

	return fitness
}

// normalize maps a value into 0–1,000,000 BPS, clamped at max.
func normalize(value, max uint64) uint64 {
	if max == 0 {
		return 0
	}
	if value >= max {
		return 1_000_000
	}
	return value * 1_000_000 / max
}

// parseUzrn parses a uzrn string amount to uint64. Returns 0 on error.
func parseUzrn(s string) uint64 {
	if s == "" {
		return 0
	}
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0
	}
	return v
}
