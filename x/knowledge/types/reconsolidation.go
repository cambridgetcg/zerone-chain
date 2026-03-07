package types

import (
	sdkmath "cosmossdk.io/math"
)

// ─── Reconsolidation Types (R51) ────────────────────────────────────────────
//
// When a memory is retrieved and produces a prediction error (negative model
// outcome), it enters a labile state — temporarily modifiable. This is how
// the brain keeps memories accurate: errors trigger updates.
//
// In ToK: TDU retrieved for training → model degrades → reconsolidation
// window opens → corrections facilitated → data re-stabilizes.
//
// Without reconsolidation, data would be frozen at encoding — never updated,
// eventually wrong. This mechanism keeps the chain's knowledge alive and accurate.

// ReconsolidationStatus tracks the state of a reconsolidation window.
type ReconsolidationStatus string

const (
	ReconsolidationOpen     ReconsolidationStatus = "open"      // window is active, corrections welcome
	ReconsolidationResolved ReconsolidationStatus = "resolved"  // correction accepted during window
	ReconsolidationExpired  ReconsolidationStatus = "expired"   // window closed without correction
)

// ReconsolidationWindow represents a labile period for a TDU after negative
// training outcome. During this window, corrections are facilitated.
type ReconsolidationWindow struct {
	WindowID        string                `json:"window_id"`
	SampleID        string                `json:"sample_id"`         // the TDU in reconsolidation
	TriggeredAt     int64                 `json:"triggered_at"`      // block height when window opened
	ExpiresAt       int64                 `json:"expires_at"`        // block height when window closes
	TriggerCycle    uint64                `json:"trigger_cycle"`     // fitness cycle that triggered this
	ModelDelta      string                `json:"model_delta"`       // how much the model degraded
	OriginalFitness string                `json:"original_fitness"`  // fitness score before reconsolidation
	MemoryTierAtOpen int                  `json:"memory_tier"`       // tier when window opened

	// Correction tracking.
	CorrectionIDs   []string              `json:"correction_ids,omitempty"` // correction TDUs submitted during window
	BountyID        string                `json:"bounty_id,omitempty"`      // auto-generated correction bounty

	// Resolution.
	Status          ReconsolidationStatus `json:"status"`
	ResolvedAt      int64                 `json:"resolved_at,omitempty"`   // block height when resolved
	FitnessAfter    string                `json:"fitness_after,omitempty"` // fitness after re-stabilization
}

// ReconsolidationHistory tracks a TDU's reconsolidation events over time.
type ReconsolidationHistory struct {
	SampleID               string   `json:"sample_id"`
	TotalWindows           uint64   `json:"total_windows"`            // times reconsolidation triggered
	UncorrectedCount       uint64   `json:"uncorrected_count"`        // windows that expired without correction
	CorrectedCount         uint64   `json:"corrected_count"`          // windows resolved with correction
	LastWindowAt           int64    `json:"last_window_at"`           // block height of most recent window
	ActiveWindowID         string   `json:"active_window_id"`         // current open window (empty if none)
	CorrectionChain        []string `json:"correction_chain,omitempty"` // ordered list of correction TDU IDs
}

// GetReconsolidationPenalty computes the decay penalty from uncorrected reconsolidations.
// penalty = 1.0 + (0.1 × uncorrected_count)
// Data that keeps failing without correction decays faster.
func (h *ReconsolidationHistory) GetReconsolidationPenalty() sdkmath.LegacyDec {
	base := sdkmath.LegacyOneDec()
	if h.UncorrectedCount == 0 {
		return base
	}
	penalty := sdkmath.LegacyNewDecWithPrec(1, 1) // 0.1 per uncorrected
	return base.Add(penalty.MulInt64(int64(h.UncorrectedCount)))
}

// HasActiveWindow returns true if there's currently an open reconsolidation window.
func (h *ReconsolidationHistory) HasActiveWindow() bool {
	return h.ActiveWindowID != ""
}

// ─── Window Duration by Tier ────────────────────────────────────────────────

// BaseReconsolidationBlocks is the standard window duration: 111 blocks.
// Higher-tier memories get shorter windows (more resistant to change).
const BaseReconsolidationBlocks int64 = 111

// WindowDurationForTier returns the reconsolidation window duration in blocks.
// More consolidated memories are more resistant to change — shorter windows.
func WindowDurationForTier(tier MemoryTier) int64 {
	switch tier {
	case MemoryTierWorking:
		return BaseReconsolidationBlocks * 2  // 222 blocks — most plastic
	case MemoryTierActive:
		return BaseReconsolidationBlocks      // 111 blocks — standard
	case MemoryTierConsolidated:
		return BaseReconsolidationBlocks / 2  // 55 blocks — resistant
	case MemoryTierCanonical:
		return BaseReconsolidationBlocks / 4  // 27 blocks — very resistant
	default:
		return BaseReconsolidationBlocks
	}
}

// ─── Reconsolidation Params ─────────────────────────────────────────────────

// ReconsolidationParams governs the reconsolidation process.
type ReconsolidationParams struct {
	// Enabled: master switch for reconsolidation.
	Enabled bool `json:"enabled"`

	// CorrectionStakeMultiplier: stake discount for corrections during window.
	// 0.5 = half the normal stake required.
	CorrectionStakeMultiplier string `json:"correction_stake_multiplier"`

	// SupesedesPropagationBoost: multiplier for fitness propagation through
	// supersedes edges during reconsolidation. Default 2.0×.
	SupersedesPropagationBoost string `json:"supersedes_propagation_boost"`

	// ExpirationPenalty: fitness reduction when window expires without correction.
	ExpirationPenalty string `json:"expiration_penalty"`

	// CanonicalMinNegativeOutcomes: canonical data requires this many separate
	// negative outcomes across different training runs before entering reconsolidation.
	CanonicalMinNegativeOutcomes uint64 `json:"canonical_min_negative_outcomes"`

	// CorrectionReputationMinRatio: correcting agent's domain reputation must be
	// at least this ratio of the original submitter's reputation.
	CorrectionReputationMinRatio string `json:"correction_reputation_min_ratio"`

	// ActivationInheritanceRatio: fraction of original's activation history
	// inherited by the correction TDU. Default 0.5.
	ActivationInheritanceRatio string `json:"activation_inheritance_ratio"`

	// AutoBountyEnabled: whether to auto-generate correction bounties.
	AutoBountyEnabled bool `json:"auto_bounty_enabled"`
}

// DefaultReconsolidationParams returns sensible defaults.
func DefaultReconsolidationParams() ReconsolidationParams {
	return ReconsolidationParams{
		Enabled:                      true,
		CorrectionStakeMultiplier:    "0.500000000000000000",
		SupersedesPropagationBoost:   "2.000000000000000000",
		ExpirationPenalty:            "0.050000000000000000",
		CanonicalMinNegativeOutcomes: 3,
		CorrectionReputationMinRatio: "1.000000000000000000", // must match or exceed
		ActivationInheritanceRatio:   "0.500000000000000000",
		AutoBountyEnabled:            true,
	}
}

// ─── Events ─────────────────────────────────────────────────────────────────

const (
	EventReconsolidationOpened   = "reconsolidation_opened"
	EventReconsolidationResolved = "reconsolidation_resolved"
	EventReconsolidationExpired  = "reconsolidation_expired"
	EventCorrectionSubmitted     = "reconsolidation_correction_submitted"
	EventCorrectionBountyCreated = "reconsolidation_bounty_created"

	AttributeWindowID        = "window_id"
	AttributeModelDelta      = "model_delta"
	AttributeExpiresAt       = "expires_at"
	AttributeUncorrectedCount = "uncorrected_count"
	AttributeCorrectionID    = "correction_id"
)
