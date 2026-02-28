package types

import (
	"fmt"
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

type PartnershipStatus = string

const (
	StatusPending   PartnershipStatus = "pending"
	StatusActive    PartnershipStatus = "active"
	StatusForming   PartnershipStatus = "forming"
	StatusCooling   PartnershipStatus = "cooling" // alias: dissolving
	StatusDissolved PartnershipStatus = "dissolved"
	StatusSuspended PartnershipStatus = "suspended"
)

type PartnershipTier = uint32

const (
	TierTrial    PartnershipTier = 0
	TierStandard PartnershipTier = 1
	TierTrusted  PartnershipTier = 2
	TierElite    PartnershipTier = 3
)

type OperationStatus = string

const (
	OpStatusPending  OperationStatus = "pending"
	OpStatusApproved OperationStatus = "approved"
	OpStatusRejected OperationStatus = "rejected"
	OpStatusExpired  OperationStatus = "expired"
)

type AmountTier = string

const (
	AmountTierMicro  AmountTier = "micro"
	AmountTierSmall  AmountTier = "small"
	AmountTierMedium AmountTier = "medium"
	AmountTierLarge  AmountTier = "large"
	AmountTierMajor  AmountTier = "major"
)

// LockTierConfig defines lock tier parameters.
type LockTierConfig struct {
	MinBlocks      uint64
	MultiplierBps  uint64 // 1000000 = 1.0x
	ExitPenaltyBps uint64 // 1000000 scale
}

// LockTiers defines the 6 commitment tiers.
var LockTiers = [6]LockTierConfig{
	{22222, 1000000, 110000},   // Tier 0: 1.00x, 11% penalty
	{77777, 1110000, 220000},   // Tier 1: 1.11x, 22% penalty
	{222222, 1220000, 330000},  // Tier 2: 1.22x, 33% penalty
	{777777, 1440000, 440000},  // Tier 3: 1.44x, 44% penalty
	{1111111, 1550000, 550000}, // Tier 4: 1.55x, 55% penalty
	{2222222, 1770000, 660000}, // Tier 5: 1.77x, 66% penalty
}

// Validate performs parameter validation.
func (p *Params) Validate() error {
	if p.FormationWindowBlocks == 0 {
		return fmt.Errorf("formation_window_blocks must be positive")
	}
	if p.CoolingPeriodBlocks == 0 {
		return fmt.Errorf("cooling_period_blocks must be positive")
	}
	if p.CommonPotShareBps == 0 || p.CommonPotShareBps > 1000000 {
		return fmt.Errorf("common_pot_share_bps must be between 1 and 1000000")
	}
	if p.SafetyFreezeDurationBlocks == 0 {
		return fmt.Errorf("safety_freeze_duration_blocks must be positive")
	}
	if p.MaxFreezesPerEpoch == 0 {
		return fmt.Errorf("max_freezes_per_epoch must be positive")
	}
	if p.CoercionReviewBlocks == 0 {
		return fmt.Errorf("coercion_review_blocks must be positive")
	}
	if p.BaseCooldownBlocks == 0 {
		return fmt.Errorf("base_cooldown_blocks must be positive")
	}
	if p.MaxCounterProposalDepth == 0 {
		return fmt.Errorf("max_counter_proposal_depth must be positive")
	}
	if p.DefaultHumanSplitBps+p.DefaultAgentSplitBps != 1000000 {
		return fmt.Errorf("default splits must sum to 1000000 bps, got %d", p.DefaultHumanSplitBps+p.DefaultAgentSplitBps)
	}
	// HumanCoercionFreezeMultiplierBps: must be >= 1x (1,000,000) and <= 10x (10,000,000), or 0 (disabled) (R28-5)
	if p.HumanCoercionFreezeMultiplierBps > 0 && p.HumanCoercionFreezeMultiplierBps < 1_000_000 {
		return fmt.Errorf("human_coercion_freeze_multiplier_bps must be >= 1,000,000 (1.0x) or 0 (disabled)")
	}
	if p.HumanCoercionFreezeMultiplierBps > 10_000_000 {
		return fmt.Errorf("human_coercion_freeze_multiplier_bps must be <= 10,000,000 (10x)")
	}
	return nil
}

// --- GetSigners and ValidateBasic for proto-generated Msg types ---

func (msg *MsgProposePartnership) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Proposer)
	return []sdk.AccAddress{addr}
}

func (msg *MsgProposePartnership) ValidateBasic() error {
	if msg.Proposer == "" {
		return fmt.Errorf("proposer cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(msg.Proposer); err != nil {
		return fmt.Errorf("invalid proposer address: %w", err)
	}
	if msg.Partner == "" {
		return fmt.Errorf("partner cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(msg.Partner); err != nil {
		return fmt.Errorf("invalid partner address: %w", err)
	}
	if msg.Proposer == msg.Partner {
		return fmt.Errorf("cannot form partnership with self")
	}
	if msg.ProposedTier > 5 {
		return fmt.Errorf("invalid proposed tier: max is 5")
	}
	return nil
}

func (msg *MsgAcceptPartnership) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Accepter)
	return []sdk.AccAddress{addr}
}

func (msg *MsgAcceptPartnership) ValidateBasic() error {
	if msg.Accepter == "" {
		return fmt.Errorf("accepter cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(msg.Accepter); err != nil {
		return fmt.Errorf("invalid accepter address: %w", err)
	}
	if msg.PartnershipId == "" {
		return fmt.Errorf("partnership_id cannot be empty")
	}
	return nil
}

func (msg *MsgProposeConsensusOp) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Proposer)
	return []sdk.AccAddress{addr}
}

func (msg *MsgProposeConsensusOp) ValidateBasic() error {
	if msg.Proposer == "" {
		return fmt.Errorf("proposer cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(msg.Proposer); err != nil {
		return fmt.Errorf("invalid proposer address: %w", err)
	}
	if msg.PartnershipId == "" {
		return fmt.Errorf("partnership_id cannot be empty")
	}
	if msg.OpType == "" {
		return fmt.Errorf("op_type cannot be empty")
	}
	amt := new(big.Int)
	if _, ok := amt.SetString(msg.Amount, 10); !ok || amt.Sign() < 0 {
		return fmt.Errorf("amount must be a non-negative integer")
	}
	return nil
}

func (msg *MsgVoteConsensusOp) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Voter)
	return []sdk.AccAddress{addr}
}

func (msg *MsgVoteConsensusOp) ValidateBasic() error {
	if msg.Voter == "" {
		return fmt.Errorf("voter cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(msg.Voter); err != nil {
		return fmt.Errorf("invalid voter address: %w", err)
	}
	if msg.OperationId == "" {
		return fmt.Errorf("operation_id cannot be empty")
	}
	if msg.CounterAmount != "" {
		amt := new(big.Int)
		if _, ok := amt.SetString(msg.CounterAmount, 10); !ok || amt.Sign() < 0 {
			return fmt.Errorf("counter_amount must be a non-negative integer")
		}
	}
	return nil
}

func (msg *MsgSafetyFreeze) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Freezer)
	return []sdk.AccAddress{addr}
}

func (msg *MsgSafetyFreeze) ValidateBasic() error {
	if msg.Freezer == "" {
		return fmt.Errorf("freezer cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(msg.Freezer); err != nil {
		return fmt.Errorf("invalid freezer address: %w", err)
	}
	if msg.PartnershipId == "" {
		return fmt.Errorf("partnership_id cannot be empty")
	}
	return nil
}

func (msg *MsgRaiseCoercionSignal) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Raiser)
	return []sdk.AccAddress{addr}
}

func (msg *MsgRaiseCoercionSignal) ValidateBasic() error {
	if msg.Raiser == "" {
		return fmt.Errorf("raiser cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(msg.Raiser); err != nil {
		return fmt.Errorf("invalid raiser address: %w", err)
	}
	if msg.PartnershipId == "" {
		return fmt.Errorf("partnership_id cannot be empty")
	}
	return nil
}

func (msg *MsgInitiateDissolution) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Initiator)
	return []sdk.AccAddress{addr}
}

func (msg *MsgInitiateDissolution) ValidateBasic() error {
	if msg.Initiator == "" {
		return fmt.Errorf("initiator cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(msg.Initiator); err != nil {
		return fmt.Errorf("invalid initiator address: %w", err)
	}
	if msg.PartnershipId == "" {
		return fmt.Errorf("partnership_id cannot be empty")
	}
	return nil
}

func (msg *MsgCreateSeedPartnership) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Human)
	return []sdk.AccAddress{addr}
}

func (msg *MsgCreateSeedPartnership) ValidateBasic() error {
	if msg.Human == "" {
		return fmt.Errorf("human cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(msg.Human); err != nil {
		return fmt.Errorf("invalid human address: %w", err)
	}
	if msg.Agent == "" {
		return fmt.Errorf("agent cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(msg.Agent); err != nil {
		return fmt.Errorf("invalid agent address: %w", err)
	}
	if msg.Human == msg.Agent {
		return fmt.Errorf("cannot form seed partnership with self")
	}
	return nil
}

func (msg *MsgJoinFormationPool) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Joiner)
	return []sdk.AccAddress{addr}
}

func (msg *MsgJoinFormationPool) ValidateBasic() error {
	if msg.Joiner == "" {
		return fmt.Errorf("joiner cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(msg.Joiner); err != nil {
		return fmt.Errorf("invalid joiner address: %w", err)
	}
	if len(msg.Domains) == 0 {
		return fmt.Errorf("must specify at least one domain")
	}
	return nil
}

func (msg *MsgLeaveFormationPool) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Leaver)
	return []sdk.AccAddress{addr}
}

func (msg *MsgLeaveFormationPool) ValidateBasic() error {
	if msg.Leaver == "" {
		return fmt.Errorf("leaver cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(msg.Leaver); err != nil {
		return fmt.Errorf("invalid leaver address: %w", err)
	}
	return nil
}

func (msg *MsgUpdateParams) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Authority)
	return []sdk.AccAddress{addr}
}

func (msg *MsgUpdateParams) ValidateBasic() error {
	if msg.Authority == "" {
		return fmt.Errorf("authority cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(msg.Authority); err != nil {
		return fmt.Errorf("invalid authority address: %w", err)
	}
	if msg.Params == nil {
		return fmt.Errorf("params cannot be nil")
	}
	return msg.Params.Validate()
}
