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
	CategoryResearchSpend  = "research_spend"
	CategorySeatElection   = "research_seat_election"
)

// Vote option constants.
const (
	VoteYes     = "yes"
	VoteNo      = "no"
	VoteAbstain = "abstain"
)

// Seat election stage constants.
const (
	SeatStageNominated  = "nominated"
	SeatStageAccepted   = "accepted"
	SeatStageDiscussion = "discussion"
	SeatStageVoting     = "voting"
	SeatStageRunoff     = "runoff"
	SeatStagePassed     = "passed"
	SeatStageFailed     = "failed"
	SeatStageExpired    = "expired"
)

// Seat election timing constants.
const (
	SeatAcceptanceBlocks     = uint64(34_272)    // ~1 day
	SeatDiscussionBlocks     = uint64(34_272)    // ~1 day
	SeatVotingBlocks         = uint64(102_816)   // ~3 days
	SeatTermBlocks           = uint64(6_400_000) // ~6 months
	SeatVacancyWarningBlocks = uint64(1_030_000) // ~30 days
	SeatVacancyNoticeBlocks  = uint64(3_090_000) // ~90 days
	SeatRunoffThresholdBps   = uint64(50_000)    // 5% on 1M scale
)

// Phase 2 initial stagger offsets.
const (
	SeatStaggerOffset0 = uint64(2_133_333) // ~2 months
	SeatStaggerOffset1 = uint64(4_266_666) // ~4 months
	SeatStaggerOffset2 = uint64(6_400_000) // ~6 months
)

// SeatStatementMaxLen is the maximum length of a candidate's governance statement.
const SeatStatementMaxLen = 2000

// MinCandidateGovernanceVotes is the minimum LIP votes required for candidacy.
const MinCandidateGovernanceVotes = uint64(5)

// IsTerminalSeatStage returns true if the stage is terminal.
func IsTerminalSeatStage(stage string) bool {
	switch stage {
	case SeatStagePassed, SeatStageFailed, SeatStageExpired:
		return true
	}
	return false
}

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

// --- Seat Election Messages ---

func (m *MsgNominateSeatElection) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Proposer); err != nil {
		return ErrInvalidAddress
	}
	if _, err := sdk.AccAddressFromBech32(m.Candidate); err != nil {
		return ErrInvalidAddress
	}
	if len(m.Statement) > SeatStatementMaxLen {
		return ErrInvalidParams
	}
	return nil
}

func (m *MsgNominateSeatElection) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Proposer)
	return []sdk.AccAddress{addr}
}

func (m *MsgAcceptSeatNomination) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Candidate); err != nil {
		return ErrInvalidAddress
	}
	if m.ProposalId == 0 {
		return ErrInvalidParams
	}
	return nil
}

func (m *MsgAcceptSeatNomination) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Candidate)
	return []sdk.AccAddress{addr}
}

func (m *MsgVoteSeatElection) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Voter); err != nil {
		return ErrInvalidAddress
	}
	if m.ProposalId == 0 {
		return ErrInvalidParams
	}
	if m.Option != VoteYes && m.Option != VoteNo && m.Option != VoteAbstain {
		return ErrInvalidParams
	}
	return nil
}

func (m *MsgVoteSeatElection) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Voter)
	return []sdk.AccAddress{addr}
}

// --- Research Fund Governance Phase Exit Conditions ---

// PhaseExitConditions defines the thresholds required to transition out of a phase.
type PhaseExitConditions struct {
	MinDistinctVoters      uint64
	MinActiveGuardians     uint64
	MinResearchFundBalance string // uzrn
	MinChainAgeBlocks      uint64
	MinProposalsExecuted   uint64
	MinCommunitySeatVotes  uint64
	MaxEmergencyHalts      uint64
}

// DefaultPhaseExitConditions returns the exit conditions for each phase transition.
// Key: phase being exited → conditions to enter the next phase.
var DefaultPhaseExitConditions = map[ResearchFundPhase]PhaseExitConditions{
	// Phase 0 → Phase 1
	ResearchFundPhase_RESEARCH_FUND_PHASE_GENESIS_PAIR: {
		MinDistinctVoters:      10,
		MinActiveGuardians:     5,
		MinResearchFundBalance: "100000000000", // 100,000 ZRN
		MinChainAgeBlocks:      2_200_000,      // ~6 months
		MinProposalsExecuted:   0,
		MinCommunitySeatVotes:  0,
		MaxEmergencyHalts:      0, // not checked in Phase 0
	},
	// Phase 1 → Phase 2
	ResearchFundPhase_RESEARCH_FUND_PHASE_OBSERVER: {
		MinDistinctVoters:      25,
		MinActiveGuardians:     10,
		MinResearchFundBalance: "0",
		MinChainAgeBlocks:      5_700_000,  // ~18 months
		MinProposalsExecuted:   3,
		MinCommunitySeatVotes:  2,
		MaxEmergencyHalts:      0, // not checked in Phase 1
	},
	// Phase 2 → Phase 3
	ResearchFundPhase_RESEARCH_FUND_PHASE_BALANCED: {
		MinDistinctVoters:      50,
		MinActiveGuardians:     22,
		MinResearchFundBalance: "0",
		MinChainAgeBlocks:      12_600_000, // ~3 years
		MinProposalsExecuted:   10,
		MinCommunitySeatVotes:  0, // not checked in Phase 2 (multiple seats)
		MaxEmergencyHalts:      0, // must be zero emergency halts
	},
}

// GetResearchFundThreshold returns the required approvals and total voters for a phase.
func GetResearchFundThreshold(phase ResearchFundPhase) (required uint32, total uint32) {
	switch phase {
	case ResearchFundPhase_RESEARCH_FUND_PHASE_GENESIS_PAIR:
		return 2, 2
	case ResearchFundPhase_RESEARCH_FUND_PHASE_OBSERVER:
		return 2, 3
	case ResearchFundPhase_RESEARCH_FUND_PHASE_BALANCED:
		return 3, 5
	case ResearchFundPhase_RESEARCH_FUND_PHASE_FULL_GOVERNANCE:
		return 0, 0 // not used — standard LIP
	default:
		return 0, 0
	}
}

// Transition protocol constants.
const (
	TransitionDiscussionBlocks = uint64(1_030_000) // ~30 days
	TransitionActivationDelay  = uint64(240_000)   // ~7 days
	TransitionSupermajorityBps = uint64(667_000)    // 66.7% on 1M scale
	RollbackCooldownBlocks     = uint64(3_700_000)  // ~3 months
)

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

