package types

import (
	"fmt"
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// DefaultParams returns the default qualification module parameters.
func DefaultParams() *Params {
	return &Params{
		MinStakeAmount:               "100000000", // 100 ZRN
		StakeLockPeriod:              100800,      // ~7 days at 6s blocks
		MinVerifications:             100,
		MinAccuracyBps:              800000, // 80%
		MinReputationScore:          500000, // 50%
		QualificationPeriod:         1209600, // ~84 days at 6s blocks
		ProbationPeriod:             302400,  // ~21 days
		RenewalWindow:               100800,  // ~7 days before expiry
		MaxEndorsements:             50,
		CrossRefMinWeight:           30,
		CrossRefWeightDiscountBps:   200000, // 20% discount
		InheritanceWeightDiscountBps: 300000, // 30% discount
		EndorsementMaxOverlapBps:    600000, // 60% — anti-ring guard (L3)
	}
}

// Validate validates the qualification module parameters.
func (p *Params) Validate() error {
	minStake := new(big.Int)
	if _, ok := minStake.SetString(p.MinStakeAmount, 10); !ok || minStake.Sign() <= 0 {
		return fmt.Errorf("invalid min_stake_amount: %s", p.MinStakeAmount)
	}
	if p.MinAccuracyBps > 1000000 {
		return fmt.Errorf("min_accuracy_bps cannot exceed 1000000, got %d", p.MinAccuracyBps)
	}
	if p.CrossRefWeightDiscountBps > 1000000 {
		return fmt.Errorf("cross_ref_weight_discount_bps cannot exceed 1000000, got %d", p.CrossRefWeightDiscountBps)
	}
	if p.InheritanceWeightDiscountBps > 1000000 {
		return fmt.Errorf("inheritance_weight_discount_bps cannot exceed 1000000, got %d", p.InheritanceWeightDiscountBps)
	}
	return nil
}

// DefaultGenesis returns the default genesis state.
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params:            DefaultParams(),
		Qualifications:    []*DomainQualification{},
		Endorsements:      []*QualificationEndorsement{},
		NextEndorsementId: 1,
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
	for _, q := range gs.Qualifications {
		key := q.Validator + "/" + q.Domain
		if seen[key] {
			return fmt.Errorf("duplicate qualification: %s", key)
		}
		seen[key] = true
	}
	return nil
}

// QualificationPenalty represents a temporary reduction in qualification weight.
type QualificationPenalty struct {
	Validator    string `json:"validator"`
	Domain       string `json:"domain"`
	ReductionBps uint64 `json:"reduction_bps"`
	ExpiryHeight uint64 `json:"expiry_height"`
	CreatedAt    uint64 `json:"created_at"`
}

// ---- GetSigners / ValidateBasic for Msg types ----

func (msg *MsgQualifyByStake) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Validator)
	return []sdk.AccAddress{addr}
}

func (msg *MsgQualifyByStake) ValidateBasic() error {
	if msg.Validator == "" {
		return fmt.Errorf("validator cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(msg.Validator); err != nil {
		return fmt.Errorf("invalid validator address: %w", err)
	}
	if msg.Domain == "" {
		return fmt.Errorf("domain cannot be empty")
	}
	amt := new(big.Int)
	if _, ok := amt.SetString(msg.StakeAmount, 10); !ok || amt.Sign() <= 0 {
		return fmt.Errorf("stake_amount must be a positive integer")
	}
	return nil
}

func (msg *MsgQualifyByTrackRecord) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Validator)
	return []sdk.AccAddress{addr}
}

func (msg *MsgQualifyByTrackRecord) ValidateBasic() error {
	if msg.Validator == "" {
		return fmt.Errorf("validator cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(msg.Validator); err != nil {
		return fmt.Errorf("invalid validator address: %w", err)
	}
	if msg.Domain == "" {
		return fmt.Errorf("domain cannot be empty")
	}
	return nil
}

func (msg *MsgQualifyByCrossReference) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Validator)
	return []sdk.AccAddress{addr}
}

func (msg *MsgQualifyByCrossReference) ValidateBasic() error {
	if msg.Validator == "" {
		return fmt.Errorf("validator cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(msg.Validator); err != nil {
		return fmt.Errorf("invalid validator address: %w", err)
	}
	if msg.TargetDomain == "" {
		return fmt.Errorf("target_domain cannot be empty")
	}
	if msg.SourceDomain == "" {
		return fmt.Errorf("source_domain cannot be empty")
	}
	if msg.TargetDomain == msg.SourceDomain {
		return fmt.Errorf("target_domain and source_domain cannot be the same")
	}
	return nil
}

func (msg *MsgQualifyByInheritance) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Validator)
	return []sdk.AccAddress{addr}
}

func (msg *MsgQualifyByInheritance) ValidateBasic() error {
	if msg.Validator == "" {
		return fmt.Errorf("validator cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(msg.Validator); err != nil {
		return fmt.Errorf("invalid validator address: %w", err)
	}
	if msg.TargetDomain == "" {
		return fmt.Errorf("target_domain cannot be empty")
	}
	if msg.ParentDomain == "" {
		return fmt.Errorf("parent_domain cannot be empty")
	}
	if msg.TargetDomain == msg.ParentDomain {
		return fmt.Errorf("target_domain and parent_domain cannot be the same")
	}
	return nil
}

func (msg *MsgEndorseQualification) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Endorser)
	return []sdk.AccAddress{addr}
}

func (msg *MsgEndorseQualification) ValidateBasic() error {
	if msg.Endorser == "" {
		return fmt.Errorf("endorser cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(msg.Endorser); err != nil {
		return fmt.Errorf("invalid endorser address: %w", err)
	}
	if msg.Validator == "" {
		return fmt.Errorf("validator cannot be empty")
	}
	if msg.Domain == "" {
		return fmt.Errorf("domain cannot be empty")
	}
	if msg.Weight == 0 || msg.Weight > 100 {
		return fmt.Errorf("weight must be 1-100, got %d", msg.Weight)
	}
	return nil
}

func (msg *MsgRenewQualification) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Validator)
	return []sdk.AccAddress{addr}
}

func (msg *MsgRenewQualification) ValidateBasic() error {
	if msg.Validator == "" {
		return fmt.Errorf("validator cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(msg.Validator); err != nil {
		return fmt.Errorf("invalid validator address: %w", err)
	}
	if msg.Domain == "" {
		return fmt.Errorf("domain cannot be empty")
	}
	return nil
}

func (msg *MsgWithdrawQualification) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Validator)
	return []sdk.AccAddress{addr}
}

func (msg *MsgWithdrawQualification) ValidateBasic() error {
	if msg.Validator == "" {
		return fmt.Errorf("validator cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(msg.Validator); err != nil {
		return fmt.Errorf("invalid validator address: %w", err)
	}
	if msg.Domain == "" {
		return fmt.Errorf("domain cannot be empty")
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
	if msg.Params != nil {
		if err := msg.Params.Validate(); err != nil {
			return fmt.Errorf("invalid params: %w", err)
		}
	}
	return nil
}
