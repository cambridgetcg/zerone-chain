package types

import (
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// LIP stage constants.
const (
	StatusDraft     = "draft"
	StatusReview    = "review"
	StatusLastCall  = "last_call"
	StatusVoting    = "voting"
	StatusPassed    = "passed"
	StatusFailed    = "failed"
	StatusWithdrawn = "withdrawn"
)

// LIP category constants.
const (
	CategoryParameter    = "parameter"
	CategoryUpgrade      = "upgrade"
	CategoryText         = "text"
	CategoryResearchSpend = "research_spend"
)

// Vote option constants.
const (
	VoteYes     = "yes"
	VoteNo      = "no"
	VoteAbstain = "abstain"
)

// BPSScale is the basis point scale used for quorum and support thresholds.
const BPSScale = uint64(1_000_000)

// IsTerminal returns true if the status is a terminal state.
func IsTerminal(status string) bool {
	switch status {
	case StatusPassed, StatusFailed, StatusWithdrawn:
		return true
	}
	return false
}

// GetCategoryConfig returns the CategoryConfig for a given category, or nil if not found.
func GetCategoryConfig(params *Params, category string) *CategoryConfig {
	for _, cfg := range params.CategoryConfigs {
		if cfg.Category == category {
			return cfg
		}
	}
	return nil
}

// AddBigIntStrings adds two big integer strings and returns the result as a string.
func AddBigIntStrings(a, b string) string {
	ai, ok := new(big.Int).SetString(a, 10)
	if !ok {
		ai = big.NewInt(0)
	}
	bi, ok := new(big.Int).SetString(b, 10)
	if !ok {
		bi = big.NewInt(0)
	}
	return new(big.Int).Add(ai, bi).String()
}

// CmpBigIntStrings compares two big integer strings.
// Returns -1, 0, or 1.
func CmpBigIntStrings(a, b string) int {
	ai, ok := new(big.Int).SetString(a, 10)
	if !ok {
		ai = big.NewInt(0)
	}
	bi, ok := new(big.Int).SetString(b, 10)
	if !ok {
		bi = big.NewInt(0)
	}
	return ai.Cmp(bi)
}

// ValidateBasic performs stateless validation on MsgSubmitLIP.
func (m *MsgSubmitLIP) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Proposer); err != nil {
		return ErrInvalidAddress
	}
	if m.Title == "" {
		return ErrInvalidParams
	}
	if m.Description == "" {
		return ErrInvalidParams
	}
	if m.InitialStake == "" {
		return ErrInsufficientStake
	}
	stake, ok := new(big.Int).SetString(m.InitialStake, 10)
	if !ok || stake.Sign() <= 0 {
		return ErrInsufficientStake
	}
	return nil
}

// GetSigners returns the expected signers for MsgSubmitLIP.
func (m *MsgSubmitLIP) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Proposer)
	return []sdk.AccAddress{addr}
}

// ValidateBasic performs stateless validation on MsgCastVote.
func (m *MsgCastVote) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Voter); err != nil {
		return ErrInvalidAddress
	}
	if m.LipId == "" {
		return ErrInvalidParams
	}
	if m.Option != VoteYes && m.Option != VoteNo && m.Option != VoteAbstain {
		return ErrInvalidParams
	}
	return nil
}

// GetSigners returns the expected signers for MsgCastVote.
func (m *MsgCastVote) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Voter)
	return []sdk.AccAddress{addr}
}

// ValidateBasic performs stateless validation on MsgStakeLIP.
func (m *MsgStakeLIP) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Staker); err != nil {
		return ErrInvalidAddress
	}
	if m.LipId == "" {
		return ErrInvalidParams
	}
	if m.Amount == "" {
		return ErrInsufficientStake
	}
	amt, ok := new(big.Int).SetString(m.Amount, 10)
	if !ok || amt.Sign() <= 0 {
		return ErrInsufficientStake
	}
	return nil
}

// GetSigners returns the expected signers for MsgStakeLIP.
func (m *MsgStakeLIP) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Staker)
	return []sdk.AccAddress{addr}
}

// ValidateBasic performs stateless validation on MsgAdvanceLIPStage.
func (m *MsgAdvanceLIPStage) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Authority); err != nil {
		return ErrInvalidAddress
	}
	if m.LipId == "" {
		return ErrInvalidParams
	}
	return nil
}

// GetSigners returns the expected signers for MsgAdvanceLIPStage.
func (m *MsgAdvanceLIPStage) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Authority)
	return []sdk.AccAddress{addr}
}

// ValidateBasic performs stateless validation on MsgWithdrawLIP.
func (m *MsgWithdrawLIP) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Proposer); err != nil {
		return ErrInvalidAddress
	}
	if m.LipId == "" {
		return ErrInvalidParams
	}
	return nil
}

// GetSigners returns the expected signers for MsgWithdrawLIP.
func (m *MsgWithdrawLIP) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Proposer)
	return []sdk.AccAddress{addr}
}

// ValidateBasic performs stateless validation on MsgAttachUpgradePlan.
func (m *MsgAttachUpgradePlan) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Proposer); err != nil {
		return ErrInvalidAddress
	}
	if m.LipId == "" {
		return ErrInvalidParams
	}
	if m.UpgradeName == "" {
		return ErrInvalidParams
	}
	if m.Height <= 0 {
		return ErrInvalidParams
	}
	return nil
}

// GetSigners returns the expected signers for MsgAttachUpgradePlan.
func (m *MsgAttachUpgradePlan) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Proposer)
	return []sdk.AccAddress{addr}
}

// --- Research Spend Stage Constants ---

type ResearchSpendStage string

const (
	ResearchStageDiscussion ResearchSpendStage = "discussion"
	ResearchStageVoting     ResearchSpendStage = "voting"
	ResearchStageExecuted   ResearchSpendStage = "executed"
	ResearchStageRejected   ResearchSpendStage = "rejected"
	ResearchStageExpired    ResearchSpendStage = "expired"
)

// Default period constants for research spend proposals.
const (
	DefaultResearchDiscussionBlocks uint64 = 68544
	DefaultResearchVotingBlocks     uint64 = 102816
)

// IsTerminalResearchStage returns true if the stage is terminal (no further transitions).
func IsTerminalResearchStage(stage ResearchSpendStage) bool {
	switch stage {
	case ResearchStageExecuted, ResearchStageRejected, ResearchStageExpired:
		return true
	}
	return false
}

// --- Research Spend Messages ---

func (m *MsgSubmitResearchSpend) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Proposer); err != nil {
		return ErrInvalidAddress
	}
	if m.Title == "" {
		return ErrInvalidParams
	}
	if _, err := sdk.AccAddressFromBech32(m.Recipient); err != nil {
		return ErrInvalidAddress
	}
	amt, ok := new(big.Int).SetString(m.Amount, 10)
	if !ok || amt.Sign() <= 0 {
		return ErrInvalidParams
	}
	return nil
}

func (m *MsgSubmitResearchSpend) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Proposer)
	return []sdk.AccAddress{addr}
}

func (m *MsgVoteResearchSpend) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Voter); err != nil {
		return ErrInvalidAddress
	}
	if m.Vote != VoteYes && m.Vote != VoteNo {
		return ErrInvalidParams
	}
	return nil
}

func (m *MsgVoteResearchSpend) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Voter)
	return []sdk.AccAddress{addr}
}

func (m *MsgSetResearchVoters) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Authority); err != nil {
		return ErrInvalidAddress
	}
	if m.Voters == nil {
		return ErrInvalidParams
	}
	if _, err := sdk.AccAddressFromBech32(m.Voters.Voter1); err != nil {
		return ErrInvalidAddress
	}
	if _, err := sdk.AccAddressFromBech32(m.Voters.Voter2); err != nil {
		return ErrInvalidAddress
	}
	return nil
}

func (m *MsgSetResearchVoters) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Authority)
	return []sdk.AccAddress{addr}
}

// ValidateBasic performs stateless validation on MsgUpdateParams.
func (m *MsgUpdateParams) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Authority); err != nil {
		return ErrInvalidAddress
	}
	if m.Params == nil {
		return ErrInvalidParams
	}
	return nil
}

// GetSigners returns the expected signers for MsgUpdateParams.
func (m *MsgUpdateParams) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Authority)
	return []sdk.AccAddress{addr}
}

