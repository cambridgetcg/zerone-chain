package keeper

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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

// ─── Energy Metabolism ──────────────────────────────────────────────────────

// initializeSampleEnergy sets a new sample's energy fields to defaults.
func initializeSampleEnergy(sample *types.Sample) {
	sample.EnergyCap = DefaultEnergyCap
	sample.Energy = DefaultEnergyCap
}

// DecayEnergy reduces a sample's energy by the decay rate.
// A minimum decay of 1 is applied when energy > 0 to prevent
// samples from becoming immortal at low energy values.
func (k Keeper) DecayEnergy(ctx context.Context, sample *types.Sample, params *types.Params) {
	if sample.Energy == 0 {
		return
	}
	decay := sample.Energy * params.EnergyDecayRate / 1_000_000
	if decay == 0 {
		decay = 1
	}
	if decay > sample.Energy {
		sample.Energy = 0
	} else {
		sample.Energy -= decay
	}
}

// RestoreEnergyOnAccess adds energy when a sample is accessed (purchased).
func (k Keeper) RestoreEnergyOnAccess(ctx context.Context, sample *types.Sample, params *types.Params) {
	sample.Energy += params.EnergyPerAccess
	if sample.Energy > sample.EnergyCap {
		sample.Energy = sample.EnergyCap
	}
	sample.AtRiskSinceEpoch = 0
}

// CheckAtRiskTransition marks a sample as at-risk if energy is 0.
func (k Keeper) CheckAtRiskTransition(ctx context.Context, sample *types.Sample, currentEpoch uint64, params *types.Params) {
	if sample.Energy == 0 && sample.AtRiskSinceEpoch == 0 {
		sample.AtRiskSinceEpoch = currentEpoch
		_ = k.SetAtRiskIndex(ctx, sample.Id)
	}
}

// ─── Niche Dynamics ─────────────────────────────────────────────────────────

// ComputeNicheKey produces a deterministic 16-char hex key from domain + sample type + primary topic.
func ComputeNicheKey(domain string, sampleType types.SampleType, primaryTopic string) string {
	h := sha256.Sum256([]byte(domain + "|" + sampleType.String() + "|" + primaryTopic))
	return hex.EncodeToString(h[:8])
}

// computeCompetitionTax calculates the extra maintenance cost for saturated niches.
// Returns BPS (0–500,000). Niches below threshold pay no tax.
func computeCompetitionTax(nicheSize, saturationThreshold uint64) uint64 {
	if nicheSize <= saturationThreshold {
		return 0
	}
	tax := (nicheSize - saturationThreshold) * 10_000
	if tax > 500_000 {
		tax = 500_000
	}
	return tax
}

// UpdateNicheLeader finds the highest-fitness sample in a niche and marks it as leader.
func (k Keeper) UpdateNicheLeader(ctx context.Context, nicheKey string) {
	ids := k.GetSamplesByNiche(ctx, nicheKey)
	if len(ids) == 0 {
		return
	}

	var bestID string
	var bestFitness uint64
	for _, id := range ids {
		s, ok := k.GetSample(ctx, id)
		if !ok {
			continue
		}
		if s.FitnessScore > bestFitness {
			bestFitness = s.FitnessScore
			bestID = id
		}
	}

	for _, id := range ids {
		s, ok := k.GetSample(ctx, id)
		if !ok {
			continue
		}
		wasLeader := s.NicheLeader
		isLeader := id == bestID
		if wasLeader != isLeader {
			s.NicheLeader = isLeader
			_ = k.SetSample(ctx, s)
		}
	}
}

// ─── Topic Saturation ───────────────────────────────────────────────────────

const (
	// saturationThreshold is the BPS level above which novelty is penalized.
	saturationThreshold uint64 = 500_000
)

// ComputeTopicSaturation returns a saturation score 0–1,000,000 for a domain+topic.
// Uses the topic count relative to a max count of 100 (2x NicheSaturationThreshold default).
func (k Keeper) ComputeTopicSaturation(ctx context.Context, domain, topic string) uint64 {
	count := k.GetTopicCount(ctx, domain, topic)
	if count == 0 {
		return 0
	}
	maxCount := uint64(100)
	return normalize(count, maxCount)
}

// ApplyNoveltyAdjustment reduces novelty score for over-saturated topics.
func ApplyNoveltyAdjustment(noveltyScore, saturation uint64) uint64 {
	if saturation <= saturationThreshold {
		return noveltyScore
	}
	penalty := (saturation - saturationThreshold) * 500_000 / 1_000_000
	return noveltyScore * (1_000_000 - penalty) / 1_000_000
}

// ─── Thread Bonus ───────────────────────────────────────────────────────────

// ComputeThreadBonus returns a fitness bonus for samples in a conversation thread.
// Bonus scales with thread length, capped at maxThreadBonus (300,000 = 30%).
func (k Keeper) ComputeThreadBonus(ctx context.Context, sample *types.Sample) uint64 {
	if sample.ThreadId == "" {
		return 0
	}
	threadSampleIDs := k.GetSamplesByThread(ctx, sample.ThreadId)
	threadLength := uint64(len(threadSampleIDs))
	bonus := threadLength * threadBonusPerMessage
	if bonus > maxThreadBonus {
		bonus = maxThreadBonus
	}
	return bonus
}

// ─── Pruning ────────────────────────────────────────────────────────────────

// PruneSamples removes samples that have been at-risk beyond the grace period.
// Content is cleared to save storage, but the provenance record is kept.
func (k Keeper) PruneSamples(ctx context.Context, currentEpoch uint64, params *types.Params) {
	var toPrune []string

	k.IterateAtRiskSamples(ctx, func(sampleID string) bool {
		sample, ok := k.GetSample(ctx, sampleID)
		if !ok {
			toPrune = append(toPrune, sampleID) // orphaned index entry
			return false
		}
		if sample.AtRiskSinceEpoch == 0 {
			return false
		}
		gracePeriod := currentEpoch - sample.AtRiskSinceEpoch
		if gracePeriod >= params.PruneGraceEpochs {
			sample.Status = types.SampleStatus_SAMPLE_STATUS_PRUNED
			sample.Content = ""
			_ = k.SetSample(ctx, sample)
			toPrune = append(toPrune, sampleID)
		}
		return false
	})

	// Clean up at-risk index for pruned samples
	for _, id := range toPrune {
		_ = k.DeleteAtRiskIndex(ctx, id)
	}
}
