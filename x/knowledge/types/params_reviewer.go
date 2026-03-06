package types

import (
	"encoding/json"
	"fmt"
)

// ReviewerStakingParams holds governance-tunable parameters for the reviewer
// dual-staking mechanism. Stored separately from proto-based Params to avoid
// .proto file changes. All ratios use BPS with 10,000 = 100%.
type ReviewerStakingParams struct {
	// ReviewerStakeRatioBps: reviewer stake = submitterStake × ratio / 10000.
	ReviewerStakeRatioBps uint64 `json:"reviewer_stake_ratio_bps"`
	// ShowUpRewardRatioBps: show-up reward pool = submitterStake × ratio / 10000.
	ShowUpRewardRatioBps uint64 `json:"show_up_reward_ratio_bps"`
	// AcceptRewardRatioBps: accept bonus for submitter = min(submitterStake × ratio / 10000, minorityPot).
	AcceptRewardRatioBps uint64 `json:"accept_reward_ratio_bps"`
	// RejectBonusRatioBps: challenge bonus for rejectors = submitterStake × ratio / 10000.
	RejectBonusRatioBps uint64 `json:"reject_bonus_ratio_bps"`
	// MaxContestedDeepCount: strikes on same content hash before permanent rejection.
	MaxContestedDeepCount uint64 `json:"max_contested_deep_count"`
}

// DefaultReviewerStakingParams returns the default reviewer staking parameters.
func DefaultReviewerStakingParams() ReviewerStakingParams {
	return ReviewerStakingParams{
		ReviewerStakeRatioBps: 3000, // 30%
		ShowUpRewardRatioBps:  1000, // 10%
		AcceptRewardRatioBps:  3000, // 30%
		RejectBonusRatioBps:   5000, // 50%
		MaxContestedDeepCount: 3,
	}
}

// Validate checks all fields are within valid ranges.
func (p ReviewerStakingParams) Validate() error {
	if p.ReviewerStakeRatioBps == 0 || p.ReviewerStakeRatioBps > 10_000 {
		return fmt.Errorf("reviewer_stake_ratio_bps must be in (0, 10000], got %d", p.ReviewerStakeRatioBps)
	}
	if p.ShowUpRewardRatioBps > 10_000 {
		return fmt.Errorf("show_up_reward_ratio_bps must be <= 10000, got %d", p.ShowUpRewardRatioBps)
	}
	if p.AcceptRewardRatioBps > 10_000 {
		return fmt.Errorf("accept_reward_ratio_bps must be <= 10000, got %d", p.AcceptRewardRatioBps)
	}
	if p.RejectBonusRatioBps > 10_000 {
		return fmt.Errorf("reject_bonus_ratio_bps must be <= 10000, got %d", p.RejectBonusRatioBps)
	}
	// ShowUp + RejectBonus must not exceed 100% of submitter stake
	if p.ShowUpRewardRatioBps+p.RejectBonusRatioBps > 10_000 {
		return fmt.Errorf("show_up + reject_bonus must be <= 10000, got %d",
			p.ShowUpRewardRatioBps+p.RejectBonusRatioBps)
	}
	if p.MaxContestedDeepCount == 0 {
		return fmt.Errorf("max_contested_deep_count must be > 0")
	}
	return nil
}

// MarshalJSON marshals to JSON.
func (p ReviewerStakingParams) MarshalJSON() ([]byte, error) {
	type alias ReviewerStakingParams
	return json.Marshal(alias(p))
}

// UnmarshalJSON unmarshals from JSON.
func (p *ReviewerStakingParams) UnmarshalJSON(bz []byte) error {
	type alias ReviewerStakingParams
	var a alias
	if err := json.Unmarshal(bz, &a); err != nil {
		return err
	}
	*p = ReviewerStakingParams(a)
	return nil
}
