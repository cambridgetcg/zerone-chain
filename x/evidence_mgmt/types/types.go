package types

import (
	"fmt"
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// DefaultParams returns the default evidence_mgmt parameters.
func DefaultParams() *Params {
	return &Params{
		MinVerifierTier:       2,
		VerificationQuorum:    3,
		ChallengeBond:         "500000",
		ChallengeWindowBlocks: 50000,
	}
}

// Validate validates the parameters.
func (p *Params) Validate() error {
	if p.MinVerifierTier == 0 {
		return fmt.Errorf("min_verifier_tier must be positive")
	}
	if p.VerificationQuorum == 0 {
		return fmt.Errorf("verification_quorum must be positive")
	}
	bond := new(big.Int)
	if _, ok := bond.SetString(p.ChallengeBond, 10); !ok || bond.Sign() <= 0 {
		return fmt.Errorf("challenge_bond must be a positive integer: %s", p.ChallengeBond)
	}
	if p.ChallengeWindowBlocks == 0 {
		return fmt.Errorf("challenge_window_blocks must be positive")
	}
	return nil
}

// DefaultGenesis returns the default genesis state.
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params:             DefaultParams(),
		Evidences:          []*Evidence{},
		Verifications:      []*VerificationResult{},
		NextEvidenceId:     0,
		NextVerificationId: 0,
	}
}

// Validate validates the genesis state.
func (gs *GenesisState) Validate() error {
	if gs.Params != nil {
		if err := gs.Params.Validate(); err != nil {
			return fmt.Errorf("invalid params: %w", err)
		}
	}
	seen := make(map[string]bool)
	for _, e := range gs.Evidences {
		if seen[e.Id] {
			return fmt.Errorf("duplicate evidence id: %s", e.Id)
		}
		seen[e.Id] = true
	}
	return nil
}

// ---- GetSigners / ValidateBasic for Msg types ----

func (msg *MsgSubmitEvidence) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Submitter)
	return []sdk.AccAddress{addr}
}

func (msg *MsgSubmitEvidence) ValidateBasic() error {
	if msg.Submitter == "" {
		return fmt.Errorf("submitter cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(msg.Submitter); err != nil {
		return fmt.Errorf("invalid submitter address: %w", err)
	}
	if msg.EvidenceType == EvidenceType_EVIDENCE_TYPE_UNSPECIFIED {
		return fmt.Errorf("evidence_type cannot be unspecified")
	}
	if msg.ContentHash == "" {
		return fmt.Errorf("content_hash cannot be empty")
	}
	if len(msg.ContentHash) != 64 {
		return fmt.Errorf("content_hash must be exactly 64 hex characters (SHA-256 digest), got %d", len(msg.ContentHash))
	}
	for _, c := range msg.ContentHash {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return fmt.Errorf("content_hash contains invalid hex character: %c", c)
		}
	}
	return nil
}

func (msg *MsgTransferCustody) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.CurrentCustodian)
	return []sdk.AccAddress{addr}
}

func (msg *MsgTransferCustody) ValidateBasic() error {
	if msg.CurrentCustodian == "" {
		return fmt.Errorf("current_custodian cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(msg.CurrentCustodian); err != nil {
		return fmt.Errorf("invalid current_custodian address: %w", err)
	}
	if msg.EvidenceId == "" {
		return fmt.Errorf("evidence_id cannot be empty")
	}
	if msg.NewCustodian == "" {
		return fmt.Errorf("new_custodian cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(msg.NewCustodian); err != nil {
		return fmt.Errorf("invalid new_custodian address: %w", err)
	}
	return nil
}

func (msg *MsgVerifyEvidence) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Verifier)
	return []sdk.AccAddress{addr}
}

func (msg *MsgVerifyEvidence) ValidateBasic() error {
	if msg.Verifier == "" {
		return fmt.Errorf("verifier cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(msg.Verifier); err != nil {
		return fmt.Errorf("invalid verifier address: %w", err)
	}
	if msg.EvidenceId == "" {
		return fmt.Errorf("evidence_id cannot be empty")
	}
	if msg.Confidence > 1000000 {
		return fmt.Errorf("confidence must be <= 1000000")
	}
	return nil
}

func (msg *MsgChallengeEvidence) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Challenger)
	return []sdk.AccAddress{addr}
}

func (msg *MsgChallengeEvidence) ValidateBasic() error {
	if msg.Challenger == "" {
		return fmt.Errorf("challenger cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(msg.Challenger); err != nil {
		return fmt.Errorf("invalid challenger address: %w", err)
	}
	if msg.EvidenceId == "" {
		return fmt.Errorf("evidence_id cannot be empty")
	}
	if msg.Reason == "" {
		return fmt.Errorf("reason cannot be empty")
	}
	bond := new(big.Int)
	if _, ok := bond.SetString(msg.Bond, 10); !ok || bond.Sign() <= 0 {
		return fmt.Errorf("bond must be a positive integer")
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
