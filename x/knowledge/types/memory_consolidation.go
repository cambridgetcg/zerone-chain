package types

import (
	"encoding/json"

	sdkmath "cosmossdk.io/math"
)

// ─── Memory Consolidation Types (R50) ───────────────────────────────────────
//
// Models how biological memory works:
//
//   Encoding → Short-term → Retrieval → Consolidation → Long-term
//
// In Zerone:
//   Submit TDU → Active (fitness 0.5) → Used in training → Consolidated → Canonical
//
// Key neuroscience principles applied:
//
// 1. RETRIEVAL STRENGTHENING (Testing Effect)
//    Each time a TDU is retrieved for training, its activation count increases.
//    Higher activation = stronger memory trace = higher fitness floor.
//
// 2. SPACED REPETITION
//    Activations spread across many cycles are worth more than burst activations.
//    spacing_factor = unique_cycles_activated / total_activations
//    A spacing factor near 1.0 means well-distributed retrieval (strong memory).
//
// 3. CONSOLIDATION EVENTS ("Sleep Cycles")
//    Periodic chain events that review activation patterns and promote
//    heavily-activated TDUs to "consolidated" status with decay protection.
//    Like how hippocampus replays memories during sleep → cortex.
//
// 4. MEMORY TIERS
//    Working → Active → Consolidated → Canonical
//    Each tier has different decay rates and protection levels.
//    Canonical data is the chain's permanent knowledge — effectively immune to decay.

// MemoryTier represents the consolidation level of a TDU.
type MemoryTier int

const (
	MemoryTierWorking      MemoryTier = iota // Just submitted, unproven
	MemoryTierActive                          // Has some activation history
	MemoryTierConsolidated                    // Repeatedly activated, decay-protected
	MemoryTierCanonical                       // Chain's permanent knowledge base
)

// String returns a human-readable name for the memory tier.
func (t MemoryTier) String() string {
	switch t {
	case MemoryTierWorking:
		return "working"
	case MemoryTierActive:
		return "active"
	case MemoryTierConsolidated:
		return "consolidated"
	case MemoryTierCanonical:
		return "canonical"
	default:
		return "unknown"
	}
}

// ActivationRecord tracks a TDU's retrieval history — the memory trace.
// This is the neuroscience-inspired layer on top of the fitness system.
type ActivationRecord struct {
	SampleID string `json:"sample_id"`

	// Retrieval history.
	TotalActivations  uint64 `json:"total_activations"`   // total times retrieved for training
	UniqueCycles      uint64 `json:"unique_cycles"`        // distinct cycles in which activation occurred
	FirstActivation   uint64 `json:"first_activation"`     // cycle of first retrieval
	LastActivation    uint64 `json:"last_activation"`      // cycle of most recent retrieval
	ActivationCycles  string `json:"activation_cycles"`    // JSON array of cycle numbers (compact)

	// Performance correlation.
	AvgModelDelta     string `json:"avg_model_delta"`      // average benchmark improvement when this TDU was in training set
	PositiveOutcomes  uint64 `json:"positive_outcomes"`    // times the model improved with this TDU
	NegativeOutcomes  uint64 `json:"negative_outcomes"`    // times the model degraded with this TDU

	// Consolidation state.
	MemoryTier        int    `json:"memory_tier"`          // MemoryTier enum value
	ConsolidatedAt    uint64 `json:"consolidated_at"`      // cycle when promoted to Consolidated (0 = never)
	CanonicalAt       uint64 `json:"canonical_at"`         // cycle when promoted to Canonical (0 = never)

	// Hebbian associations: TDUs frequently co-activated strengthen each other.
	CoActivations     map[string]uint64 `json:"co_activations,omitempty"` // tduID → co-activation count
}

// GetSpacingFactor returns how well-distributed the activations are across cycles.
// 1.0 = perfect spacing (every activation in a unique cycle)
// 0.0 = all activations in the same cycle (cramming)
func (r *ActivationRecord) GetSpacingFactor() sdkmath.LegacyDec {
	if r.TotalActivations == 0 {
		return sdkmath.LegacyZeroDec()
	}
	return sdkmath.LegacyNewDec(int64(r.UniqueCycles)).Quo(
		sdkmath.LegacyNewDec(int64(r.TotalActivations)),
	)
}

// GetRetrievalStrength computes the overall retrieval strength of this memory trace.
// Combines activation count, spacing, and performance correlation.
// Higher = stronger memory, more resistant to decay.
func (r *ActivationRecord) GetRetrievalStrength() sdkmath.LegacyDec {
	if r.TotalActivations == 0 {
		return sdkmath.LegacyZeroDec()
	}

	// Base strength: log2(activations + 1) normalized to [0, 1] (caps at ~10 activations)
	activationScore := sdkmath.LegacyNewDec(int64(r.TotalActivations))
	if activationScore.GT(sdkmath.LegacyNewDec(10)) {
		activationScore = sdkmath.LegacyNewDec(10)
	}
	activationNorm := activationScore.Quo(sdkmath.LegacyNewDec(10)) // [0, 1]

	// Spacing bonus: well-spaced activations are 50% more effective
	spacing := r.GetSpacingFactor()

	// Performance correlation: positive outcomes boost, negative outcomes weaken
	perfScore := sdkmath.LegacyNewDecWithPrec(5, 1) // 0.5 baseline
	total := r.PositiveOutcomes + r.NegativeOutcomes
	if total > 0 {
		perfScore = sdkmath.LegacyNewDec(int64(r.PositiveOutcomes)).Quo(
			sdkmath.LegacyNewDec(int64(total)),
		)
	}

	// Combined: 40% activation frequency + 30% spacing + 30% performance
	strength := activationNorm.Mul(sdkmath.LegacyNewDecWithPrec(4, 1)).    // 0.4
		Add(spacing.Mul(sdkmath.LegacyNewDecWithPrec(3, 1))).              // 0.3
		Add(perfScore.Mul(sdkmath.LegacyNewDecWithPrec(3, 1)))             // 0.3

	return strength
}

// GetMemoryTier returns the typed MemoryTier.
func (r *ActivationRecord) GetMemoryTier() MemoryTier {
	return MemoryTier(r.MemoryTier)
}

// DecayMultiplier returns the decay rate modifier for this tier.
// Higher tier = slower decay.
func (r *ActivationRecord) DecayMultiplier() sdkmath.LegacyDec {
	switch MemoryTier(r.MemoryTier) {
	case MemoryTierWorking:
		return sdkmath.LegacyOneDec()                          // 1.0× — full decay rate
	case MemoryTierActive:
		return sdkmath.LegacyNewDecWithPrec(7, 1)              // 0.7× — 30% slower decay
	case MemoryTierConsolidated:
		return sdkmath.LegacyNewDecWithPrec(2, 1)              // 0.2× — 80% slower decay
	case MemoryTierCanonical:
		return sdkmath.LegacyZeroDec()                          // 0.0× — no decay
	default:
		return sdkmath.LegacyOneDec()
	}
}

// ─── Consolidation Params ───────────────────────────────────────────────────

// ConsolidationParams governs the memory consolidation process.
type ConsolidationParams struct {
	// ConsolidationInterval: blocks between consolidation events ("sleep cycles").
	ConsolidationInterval uint64 `json:"consolidation_interval"` // default: 500

	// Promotion thresholds.
	ActiveMinActivations       uint64 `json:"active_min_activations"`        // default: 3
	ConsolidatedMinActivations uint64 `json:"consolidated_min_activations"`  // default: 10
	ConsolidatedMinSpacing     string `json:"consolidated_min_spacing"`      // default: "0.5" — min spacing factor
	ConsolidatedMinPerformance string `json:"consolidated_min_performance"`  // default: "0.6" — min positive outcome ratio
	CanonicalMinActivations    uint64 `json:"canonical_min_activations"`     // default: 25
	CanonicalMinStrength       string `json:"canonical_min_strength"`        // default: "0.8" — min retrieval strength
	CanonicalMinAge            uint64 `json:"canonical_min_age"`             // default: 1000 — min cycles since first activation

	// Hebbian: minimum co-activations to form an association.
	HebbianThreshold uint64 `json:"hebbian_threshold"` // default: 5
}

// DefaultConsolidationParams returns sensible defaults.
func DefaultConsolidationParams() ConsolidationParams {
	return ConsolidationParams{
		ConsolidationInterval:      500,
		ActiveMinActivations:       3,
		ConsolidatedMinActivations: 10,
		ConsolidatedMinSpacing:     "0.500000000000000000",
		ConsolidatedMinPerformance: "0.600000000000000000",
		CanonicalMinActivations:    25,
		CanonicalMinStrength:       "0.800000000000000000",
		CanonicalMinAge:            1000,
		HebbianThreshold:           5,
	}
}

// MarshalJSON for store serialization.
func (p *ConsolidationParams) MarshalJSON() ([]byte, error) {
	type alias ConsolidationParams
	return json.Marshal((*alias)(p))
}

func (p *ConsolidationParams) UnmarshalJSON(bz []byte) error {
	type alias ConsolidationParams
	return json.Unmarshal(bz, (*alias)(p))
}

// ─── Events ─────────────────────────────────────────────────────────────────

const (
	EventActivationRecorded   = "activation_recorded"
	EventMemoryConsolidated   = "memory_consolidated"    // periodic consolidation event
	EventTierPromoted         = "memory_tier_promoted"
	EventHebbianAssociation   = "hebbian_association"    // two TDUs co-activated enough to form association

	AttributeMemoryTier       = "memory_tier"
	AttributeActivationCount  = "activation_count"
	AttributeSpacingFactor    = "spacing_factor"
	AttributeRetrievalStrength = "retrieval_strength"
	AttributeCoActivatedWith  = "co_activated_with"
)
