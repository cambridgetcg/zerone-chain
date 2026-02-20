package types

import (
	"fmt"
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// VestingCategoryStr is the string representation of epistemic vesting categories.
type VestingCategoryStr string

const (
	CategoryAxiomatic     VestingCategoryStr = "axiomatic"
	CategoryFormalProof   VestingCategoryStr = "formal_proof"
	CategoryOnChain       VestingCategoryStr = "on_chain"
	CategoryCryptographic VestingCategoryStr = "cryptographic"
	CategoryComputational VestingCategoryStr = "computational"
	CategoryPeerReviewed  VestingCategoryStr = "peer_reviewed"
	CategoryReplicated    VestingCategoryStr = "replicated"
	CategoryOracleFeed    VestingCategoryStr = "oracle_feed"
	CategoryAttestation   VestingCategoryStr = "attestation"
	CategoryContested     VestingCategoryStr = "contested"
)

// VestingStatus tracks the lifecycle of a vesting schedule.
type VestingStatus string

const (
	VestingStatusActive    VestingStatus = "vesting"
	VestingStatusPaused    VestingStatus = "paused"
	VestingStatusCompleted VestingStatus = "completed"
	VestingStatusFalsified VestingStatus = "falsified"
	VestingStatusAbandoned VestingStatus = "abandoned"
)

// RewardSource identifies where a reward originated.
type RewardSource string

const (
	SourceBlockProduction   RewardSource = "block_production"
	SourceVerification      RewardSource = "verification"
	SourceAPICall           RewardSource = "api_call"
	SourceCitation          RewardSource = "citation"
	SourceChallengeWin      RewardSource = "challenge_win"
	SourceChallengeDefense  RewardSource = "challenge_defense"
	SourceBountyFulfillment RewardSource = "bounty_fulfillment"
	SourceFalsification     RewardSource = "falsification"
)

// AutopoiesisKeeper defines the expected autopoiesis keeper interface.
type AutopoiesisKeeper interface {
	GetMultiplier(ctx sdk.Context, path string) uint64
}

// --- ValidateBasic methods for proto-generated message types ---

func (m *MsgCreateVesting) ValidateBasic() error {
	if m.Authority == "" {
		return fmt.Errorf("authority required")
	}
	if m.Beneficiary == "" {
		return fmt.Errorf("beneficiary required")
	}
	if _, err := sdk.AccAddressFromBech32(m.Beneficiary); err != nil {
		return fmt.Errorf("invalid beneficiary address: %w", err)
	}
	if m.Amount == "" {
		return fmt.Errorf("amount must be positive")
	}
	amt := new(big.Int)
	if _, ok := amt.SetString(m.Amount, 10); !ok {
		return fmt.Errorf("invalid amount: not a valid integer")
	}
	if amt.Sign() <= 0 {
		return fmt.Errorf("amount must be positive")
	}
	return nil
}

func (m *MsgCreateVesting) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Authority)
	return []sdk.AccAddress{addr}
}

func (m *MsgClaimVesting) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Claimer); err != nil {
		return fmt.Errorf("invalid claimer address: %w", err)
	}
	return nil
}

func (m *MsgClaimVesting) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Claimer)
	return []sdk.AccAddress{addr}
}

func (m *MsgFalsifyVesting) ValidateBasic() error {
	if m.Challenger == "" {
		return fmt.Errorf("challenger address required")
	}
	if m.VestingId == "" {
		return fmt.Errorf("vesting ID required")
	}
	return nil
}

func (m *MsgFalsifyVesting) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Challenger)
	return []sdk.AccAddress{addr}
}

func (m *MsgPauseVesting) ValidateBasic() error {
	if m.Authority == "" {
		return fmt.Errorf("authority required")
	}
	if m.VestingId == "" {
		return fmt.Errorf("vesting ID required")
	}
	return nil
}

func (m *MsgPauseVesting) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Authority)
	return []sdk.AccAddress{addr}
}

func (m *MsgResumeVesting) ValidateBasic() error {
	if m.Authority == "" {
		return fmt.Errorf("authority required")
	}
	if m.VestingId == "" {
		return fmt.Errorf("vesting ID required")
	}
	return nil
}

func (m *MsgResumeVesting) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Authority)
	return []sdk.AccAddress{addr}
}

func (m *MsgAccelerateVesting) ValidateBasic() error {
	if m.Authority == "" {
		return fmt.Errorf("authority required")
	}
	if m.VestingId == "" {
		return fmt.Errorf("vesting ID required")
	}
	return nil
}

func (m *MsgAccelerateVesting) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Authority)
	return []sdk.AccAddress{addr}
}

func (m *MsgCompleteVesting) ValidateBasic() error {
	if m.Authority == "" {
		return fmt.Errorf("authority required")
	}
	if m.VestingId == "" {
		return fmt.Errorf("vesting ID required")
	}
	return nil
}

func (m *MsgCompleteVesting) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Authority)
	return []sdk.AccAddress{addr}
}

func (m *MsgUpdateParams) ValidateBasic() error {
	if m.Authority == "" {
		return fmt.Errorf("authority required")
	}
	return nil
}

func (m *MsgUpdateParams) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Authority)
	return []sdk.AccAddress{addr}
}
