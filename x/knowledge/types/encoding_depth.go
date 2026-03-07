package types

import (
	sdkmath "cosmossdk.io/math"
)

// ─── Encoding Depth & Type-Specific Decay (R52) ────────────────────────────
//
// Two neuroscience principles that were missing from our memory system:
//
// 1. ENCODING DEPTH: Not all memories are created equal. Deep processing
//    (elaboration, emotional significance) creates stronger initial traces.
//    In ToK: quality round outcomes determine initial fitness, not a flat 0.5.
//
// 2. TYPE-SPECIFIC DECAY: Different memory types decay at different rates.
//    - Semantic (facts): slow decay — "Paris is the capital of France" persists
//    - Episodic (events): fast decay — "what I had for lunch" fades quickly
//    - Procedural (skills): slowest decay — "how to ride a bicycle" endures
//    In ToK: TDU types get different decay rate modifiers.

// ─── Memory Classification ──────────────────────────────────────────────────

// MemoryClass maps TDU types to LTM subsystems.
type MemoryClass int

const (
	MemoryClassSemantic   MemoryClass = iota // Facts, concepts — robust
	MemoryClassEpisodic                       // Events, conversations — fragile
	MemoryClassProcedural                     // Skills, reasoning — durable
)

// String returns a human-readable name.
func (c MemoryClass) String() string {
	switch c {
	case MemoryClassSemantic:
		return "semantic"
	case MemoryClassEpisodic:
		return "episodic"
	case MemoryClassProcedural:
		return "procedural"
	default:
		return "unknown"
	}
}

// DecayModifier returns the type-specific decay rate modifier.
// Applied on top of the memory tier modifier and reconsolidation penalty.
//
// effective_decay = base × tier_modifier × reconsolidation_penalty × type_modifier
func (c MemoryClass) DecayModifier() sdkmath.LegacyDec {
	switch c {
	case MemoryClassSemantic:
		return sdkmath.LegacyNewDecWithPrec(8, 1)  // 0.8× — facts persist
	case MemoryClassEpisodic:
		return sdkmath.LegacyNewDecWithPrec(12, 1) // 1.2× — events fade faster
	case MemoryClassProcedural:
		return sdkmath.LegacyNewDecWithPrec(6, 1)  // 0.6× — skills endure
	default:
		return sdkmath.LegacyOneDec()               // 1.0× — neutral
	}
}

// ClassifyFromSampleType maps proto SampleType to MemoryClass.
// Discourse-format types map to LTM subsystems based on their nature.
func ClassifyFromSampleType(st SampleType) MemoryClass {
	switch st {
	// Semantic: facts, explanations, annotations — stable knowledge
	case SampleType_SAMPLE_TYPE_EXPLANATION,
		SampleType_SAMPLE_TYPE_ANNOTATION,
		SampleType_SAMPLE_TYPE_Q_AND_A:
		return MemoryClassSemantic

	// Episodic: events, conversations, stories, opinions — contextual
	case SampleType_SAMPLE_TYPE_DISCUSSION,
		SampleType_SAMPLE_TYPE_DEBATE,
		SampleType_SAMPLE_TYPE_NARRATIVE,
		SampleType_SAMPLE_TYPE_OPINION,
		SampleType_SAMPLE_TYPE_REVIEW:
		return MemoryClassEpisodic

	// Procedural: how-to, problem-solving, creative skills — durable
	case SampleType_SAMPLE_TYPE_TUTORIAL,
		SampleType_SAMPLE_TYPE_TROUBLESHOOT,
		SampleType_SAMPLE_TYPE_CREATIVE,
		SampleType_SAMPLE_TYPE_CORRECTION:
		return MemoryClassProcedural

	default:
		return MemoryClassSemantic // default to semantic (most common)
	}
}

// ─── Encoding Depth ─────────────────────────────────────────────────────────

// EncodingDepth computes how deeply a TDU was encoded during its quality round.
// Deeper encoding = higher initial fitness score.
//
// Factors:
//   - Consensus strength: unanimous > supermajority > bare majority
//   - Reviewer quality: average reputation of majority reviewers
//   - Stake amount: higher stake = higher emotional salience
//   - TDU type: procedural gets a bonus (skills are more deeply processed)
//
// Returns a value in [0.3, 0.8] that becomes the initial fitness score.
func EncodingDepth(
	consensusStrength sdkmath.LegacyDec, // [0, 1] — from quality round
	avgReviewerRep sdkmath.LegacyDec,    // [0, 1] — average reputation of majority
	stakeRatio sdkmath.LegacyDec,        // actual_stake / base_stake (1.0 = minimum, 5.0 = max difficulty)
	memoryClass MemoryClass,
) sdkmath.LegacyDec {
	// Weights: consensus 35%, reviewer quality 25%, stake 25%, type bonus 15%
	w1 := sdkmath.LegacyNewDecWithPrec(35, 2) // 0.35
	w2 := sdkmath.LegacyNewDecWithPrec(25, 2) // 0.25
	w3 := sdkmath.LegacyNewDecWithPrec(25, 2) // 0.25
	w4 := sdkmath.LegacyNewDecWithPrec(15, 2) // 0.15

	// Normalize stake ratio to [0, 1]: (stake_ratio - 1) / 4, capped at 1.
	stakeNorm := stakeRatio.Sub(sdkmath.LegacyOneDec()).Quo(sdkmath.LegacyNewDec(4))
	if stakeNorm.GT(sdkmath.LegacyOneDec()) {
		stakeNorm = sdkmath.LegacyOneDec()
	}
	if stakeNorm.IsNegative() {
		stakeNorm = sdkmath.LegacyZeroDec()
	}

	// Type bonus: procedural 1.0, semantic 0.7, episodic 0.4
	var typeBonus sdkmath.LegacyDec
	switch memoryClass {
	case MemoryClassProcedural:
		typeBonus = sdkmath.LegacyOneDec()              // 1.0 — deepest processing
	case MemoryClassSemantic:
		typeBonus = sdkmath.LegacyNewDecWithPrec(7, 1)  // 0.7 — moderate
	case MemoryClassEpisodic:
		typeBonus = sdkmath.LegacyNewDecWithPrec(4, 1)  // 0.4 — shallowest
	default:
		typeBonus = sdkmath.LegacyNewDecWithPrec(5, 1)  // 0.5
	}

	// Weighted sum: raw score in [0, 1].
	raw := consensusStrength.Mul(w1).
		Add(avgReviewerRep.Mul(w2)).
		Add(stakeNorm.Mul(w3)).
		Add(typeBonus.Mul(w4))

	// Map [0, 1] → [0.3, 0.8]: initial_fitness = 0.3 + raw × 0.5
	minScore := sdkmath.LegacyNewDecWithPrec(3, 1) // 0.3
	scoreRange := sdkmath.LegacyNewDecWithPrec(5, 1) // 0.5
	result := minScore.Add(raw.Mul(scoreRange))

	// Clamp to [0.3, 0.8].
	if result.GT(sdkmath.LegacyNewDecWithPrec(8, 1)) {
		result = sdkmath.LegacyNewDecWithPrec(8, 1)
	}
	if result.LT(minScore) {
		result = minScore
	}

	return result
}

// ─── Events ─────────────────────────────────────────────────────────────────

const (
	EventEncodingDepthComputed = "encoding_depth_computed"

	AttributeEncodingDepth = "encoding_depth"
	AttributeMemoryClass   = "memory_class"
	AttributeInitialFitness = "initial_fitness"
)
