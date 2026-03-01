package types

import (
	"fmt"
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// ---------- Enums (not in proto) ----------

type ResearchType string

const (
	ResearchTypeReplication        ResearchType = "replication"
	ResearchTypeFraudInvestigation ResearchType = "fraud_investigation"
	ResearchTypeMethodologyAudit   ResearchType = "methodology_audit"
	ResearchTypeDataValidation     ResearchType = "data_validation"
)

func IsValidResearchType(rt ResearchType) bool {
	switch rt {
	case ResearchTypeReplication, ResearchTypeFraudInvestigation,
		ResearchTypeMethodologyAudit, ResearchTypeDataValidation:
		return true
	}
	return false
}

type ResearchStatus string

const (
	ResearchStatusSubmitted   ResearchStatus = "submitted"
	ResearchStatusUnderReview ResearchStatus = "under_review"
	ResearchStatusAccepted    ResearchStatus = "accepted"
	ResearchStatusRejected    ResearchStatus = "rejected"
	ResearchStatusChallenged  ResearchStatus = "challenged"
)

type BountyStatus string

const (
	BountyStatusOpen      BountyStatus = "open"
	BountyStatusClaimed   BountyStatus = "claimed"
	BountyStatusFulfilled BountyStatus = "fulfilled"
	BountyStatusExpired   BountyStatus = "expired"
)

// IsValidReviewVerdict checks whether a proto ReviewVerdict enum value is valid.
func IsValidReviewVerdict(v ReviewVerdict) bool {
	switch v {
	case ReviewVerdict_REVIEW_VERDICT_APPROVE,
		ReviewVerdict_REVIEW_VERDICT_REJECT,
		ReviewVerdict_REVIEW_VERDICT_REVISE:
		return true
	}
	return false
}

// ---------- Params ----------

func DefaultParams() Params {
	return Params{
		MinResearchStake:              "1000000",      // 1 ZRN in uzrn
		MinChallengeStake:             "1000000",      // 1 ZRN in uzrn
		ReviewPeriodBlocks:            68544,          // ~2 days
		MinReviewerCount:              3,
		AcceptanceScoreThreshold:      70,             // 70/100 per spec
		RejectionSlashBps:             330000,         // 33%
		MaxBountyReward:               "10000000000",  // 10,000 ZRN in uzrn
		BountyMinDeadlineBlocks:       34272,          // ~1 day
		BountyFulfillmentPeriodBlocks: 34272,          // ~1 day
	}
}

// ---------- Genesis ----------

func DefaultGenesis() *GenesisState {
	params := DefaultParams()
	return &GenesisState{
		Params:          &params,
		Researches:      []*Research{},
		Bounties:        []*Bounty{},
		PeerReviews:     []*PeerReview{},
		TreasuryBalance: &TreasuryBalance{Balance: "0"},
	}
}

func (gs *GenesisState) Validate() error {
	if gs.Params != nil {
		if err := validateParams(gs.Params); err != nil {
			return fmt.Errorf("invalid params: %w", err)
		}
	}
	seen := make(map[string]bool)
	for _, r := range gs.Researches {
		if seen[r.Id] {
			return fmt.Errorf("duplicate research id: %s", r.Id)
		}
		seen[r.Id] = true
	}
	return nil
}

func validateParams(p *Params) error {
	minStake := new(big.Int)
	if _, ok := minStake.SetString(p.MinResearchStake, 10); !ok || minStake.Sign() < 0 {
		return fmt.Errorf("invalid min_research_stake: %s", p.MinResearchStake)
	}
	if p.AcceptanceScoreThreshold > 100 {
		return fmt.Errorf("acceptance_score_threshold must be <= 100, got %d", p.AcceptanceScoreThreshold)
	}
	if p.RejectionSlashBps > 1000000 {
		return fmt.Errorf("rejection_slash_bps must be <= 1000000, got %d", p.RejectionSlashBps)
	}
	return nil
}

// ---------- ValidateBasic / GetSigners ----------

func (m *MsgSubmitResearch) ValidateBasic() error {
	if m.Submitter == "" {
		return fmt.Errorf("submitter cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(m.Submitter); err != nil {
		return fmt.Errorf("invalid submitter address: %w", err)
	}
	if m.Title == "" {
		return fmt.Errorf("title cannot be empty")
	}
	if m.Domain == "" {
		return fmt.Errorf("domain cannot be empty")
	}
	amount := new(big.Int)
	if _, ok := amount.SetString(m.Stake, 10); !ok || amount.Sign() <= 0 {
		return fmt.Errorf("invalid stake amount")
	}
	return nil
}

func (m *MsgSubmitResearch) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Submitter)
	return []sdk.AccAddress{addr}
}

func (m *MsgChallengeResearch) ValidateBasic() error {
	if m.Challenger == "" {
		return fmt.Errorf("challenger cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(m.Challenger); err != nil {
		return fmt.Errorf("invalid challenger address: %w", err)
	}
	if m.ResearchId == "" {
		return fmt.Errorf("research id cannot be empty")
	}
	if m.Reason == "" {
		return fmt.Errorf("reason cannot be empty")
	}
	amount := new(big.Int)
	if _, ok := amount.SetString(m.Stake, 10); !ok || amount.Sign() <= 0 {
		return fmt.Errorf("invalid stake amount")
	}
	return nil
}

func (m *MsgChallengeResearch) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Challenger)
	return []sdk.AccAddress{addr}
}

func (m *MsgReviewResearch) ValidateBasic() error {
	if m.Reviewer == "" {
		return fmt.Errorf("reviewer cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(m.Reviewer); err != nil {
		return fmt.Errorf("invalid reviewer address: %w", err)
	}
	if m.ResearchId == "" {
		return fmt.Errorf("research id cannot be empty")
	}
	if !IsValidReviewVerdict(m.Verdict) {
		return fmt.Errorf("invalid verdict: %s", m.Verdict)
	}
	if m.QualityScore > 100 {
		return fmt.Errorf("score must be 0-100, got %d", m.QualityScore)
	}
	return nil
}

func (m *MsgReviewResearch) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Reviewer)
	return []sdk.AccAddress{addr}
}

func (m *MsgResolveResearch) ValidateBasic() error {
	if m.Authority == "" {
		return fmt.Errorf("authority cannot be empty")
	}
	if m.ResearchId == "" {
		return fmt.Errorf("research id cannot be empty")
	}
	return nil
}

func (m *MsgResolveResearch) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Authority)
	return []sdk.AccAddress{addr}
}

func (m *MsgCreateBounty) ValidateBasic() error {
	if m.Creator == "" {
		return fmt.Errorf("creator cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(m.Creator); err != nil {
		return fmt.Errorf("invalid creator address: %w", err)
	}
	if m.Title == "" {
		return fmt.Errorf("title cannot be empty")
	}
	reward := new(big.Int)
	if _, ok := reward.SetString(m.Reward, 10); !ok || reward.Sign() <= 0 {
		return fmt.Errorf("invalid reward amount")
	}
	if m.DeadlineHeight == 0 {
		return fmt.Errorf("deadline height must be > 0")
	}
	return nil
}

func (m *MsgCreateBounty) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Creator)
	return []sdk.AccAddress{addr}
}

func (m *MsgClaimBounty) ValidateBasic() error {
	if m.Claimer == "" {
		return fmt.Errorf("claimer cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(m.Claimer); err != nil {
		return fmt.Errorf("invalid claimer address: %w", err)
	}
	if m.BountyId == "" {
		return fmt.Errorf("bounty id cannot be empty")
	}
	return nil
}

func (m *MsgClaimBounty) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Claimer)
	return []sdk.AccAddress{addr}
}

func (m *MsgFulfillBounty) ValidateBasic() error {
	if m.Authority == "" {
		return fmt.Errorf("authority cannot be empty")
	}
	if m.BountyId == "" {
		return fmt.Errorf("bounty id cannot be empty")
	}
	return nil
}

func (m *MsgFulfillBounty) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Authority)
	return []sdk.AccAddress{addr}
}

func (m *MsgFundResearch) ValidateBasic() error {
	if m.Funder == "" {
		return fmt.Errorf("funder cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(m.Funder); err != nil {
		return fmt.Errorf("invalid funder address: %w", err)
	}
	amount := new(big.Int)
	if _, ok := amount.SetString(m.Amount, 10); !ok || amount.Sign() <= 0 {
		return fmt.Errorf("invalid fund amount")
	}
	return nil
}

func (m *MsgFundResearch) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Funder)
	return []sdk.AccAddress{addr}
}

func (m *MsgUpdateParams) ValidateBasic() error {
	if m.Authority == "" {
		return fmt.Errorf("authority cannot be empty")
	}
	return nil
}

func (m *MsgUpdateParams) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Authority)
	return []sdk.AccAddress{addr}
}
