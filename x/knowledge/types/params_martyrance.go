package types

import (
	"encoding/json"
	"fmt"
)

// MartyranceParams holds governance-tunable parameters for the martyrance queue.
// Stored as JSON (not proto) alongside ReviewerStakingParams to keep the proto
// Params message stable. All ratios use BPS with 10,000 = 100%.
type MartyranceParams struct {
	// MinStakeRatioBps: minimum fraction of submitter balance to escrow (default 9000 = 90%).
	MinStakeRatioBps uint64 `json:"min_stake_ratio_bps"`
	// VerifierMinTier: minimum reputation tier required for verifiers (default 3).
	VerifierMinTier uint32 `json:"verifier_min_tier"`
	// DeadlineMultiplier: multiplier applied to CommitPeriodBlocks and RevealPeriodBlocks (default 2).
	DeadlineMultiplier uint64 `json:"deadline_multiplier"`
	// ReputationMultiplier: multiplier applied to submitter reputation gain on success (default 5).
	ReputationMultiplier uint64 `json:"reputation_multiplier"`
	// MaxActiveMartyranceClaims: maximum number of concurrent martyrance submissions (default 10).
	MaxActiveMartyranceClaims uint64 `json:"max_active_martyrance_claims"`
}

// DefaultMartyranceParams returns the default martyrance parameters.
func DefaultMartyranceParams() MartyranceParams {
	return MartyranceParams{
		MinStakeRatioBps:          9000, // 90%
		VerifierMinTier:           3,
		DeadlineMultiplier:        2,
		ReputationMultiplier:      5,
		MaxActiveMartyranceClaims: 10,
	}
}

// Validate checks all fields are within valid ranges.
func (p MartyranceParams) Validate() error {
	if p.MinStakeRatioBps == 0 || p.MinStakeRatioBps > 10_000 {
		return fmt.Errorf("min_stake_ratio_bps must be in (0, 10000], got %d", p.MinStakeRatioBps)
	}
	if p.VerifierMinTier == 0 {
		return fmt.Errorf("verifier_min_tier must be > 0, got %d", p.VerifierMinTier)
	}
	if p.DeadlineMultiplier == 0 {
		return fmt.Errorf("deadline_multiplier must be > 0, got %d", p.DeadlineMultiplier)
	}
	if p.ReputationMultiplier == 0 {
		return fmt.Errorf("reputation_multiplier must be > 0, got %d", p.ReputationMultiplier)
	}
	if p.MaxActiveMartyranceClaims == 0 {
		return fmt.Errorf("max_active_martyrance_claims must be > 0, got %d", p.MaxActiveMartyranceClaims)
	}
	return nil
}

// MarshalJSON marshals to JSON.
func (p MartyranceParams) MarshalJSON() ([]byte, error) {
	type alias MartyranceParams
	return json.Marshal(alias(p))
}

// UnmarshalJSON unmarshals from JSON.
func (p *MartyranceParams) UnmarshalJSON(bz []byte) error {
	type alias MartyranceParams
	var a alias
	if err := json.Unmarshal(bz, &a); err != nil {
		return err
	}
	*p = MartyranceParams(a)
	return nil
}
