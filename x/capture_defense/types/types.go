package types

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	BPSScale = 1_000_000
)

func DefaultParams() *Params {
	return &Params{
		DecayEpochBlocks:         10000,  // ~7 hours at 2.5s blocks
		MinVerificationsForScore: 5,      // min verifications before domain rep used
		HhiThreshold:             250000, // 25% HHI threshold for flagging
		RiskAnalysisInterval:     1000,   // every ~42 minutes
		HistoryRetentionBlocks:   50000,  // ~35 hours
		BaseReputationScore:      500000, // 50% floor
		MaxHistoryPerDomain:      100,
		BaseReputationRecoveryBps:   50_000,  // 5% recovery per decay epoch
		ActivityRecoveryBonusMaxBps: 500_000, // max 50% acceleration from activity
	}
}

func (p *Params) Validate() error {
	if p.DecayEpochBlocks == 0 {
		return fmt.Errorf("decay_epoch_blocks must be > 0")
	}
	if p.HhiThreshold > BPSScale {
		return fmt.Errorf("hhi_threshold must be <= %d", BPSScale)
	}
	if p.BaseReputationScore > BPSScale {
		return fmt.Errorf("base_reputation_score must be <= %d", BPSScale)
	}
	return nil
}

// StructuralImmunityParams holds parameters for the partnership-based HHI adjustment (R29-5).
// Stored separately from proto-generated Params to avoid proto regeneration.
type StructuralImmunityParams struct {
	PartnershipHHIReductionPerParticipantBps uint64 // HHI reduction per unique participant (default: 10,000 = 1%)
	MaxPartnershipHHIReductionBps            uint64 // Max total HHI reduction (default: 200,000 = 20%)
	PartnershipReputationBonusBps            uint64 // Rep bonus per active partnership (default: 20,000 = 2%)
	MaxPartnershipReputationBonusBps         uint64 // Max total rep bonus (default: 100,000 = 10%)
	MinDensityForAcceleratedClear            uint64 // Min participants for accelerated flag clearing (default: 10)
	CapturedDomainFormationBonusBps          uint64 // Match priority boost for flagged domains (default: 300,000 = 30%)
	FormationBonusDurationBlocks             uint64 // How long formation bonus lasts (default: 50,000 = ~35 hours)
	ActivityThresholdRelaxationBps           uint64 // max HHI threshold relaxation from verification activity (R31-4, default: 200,000 = 20%)
}

// DefaultStructuralImmunityParams returns default structural immunity parameters.
func DefaultStructuralImmunityParams() *StructuralImmunityParams {
	return &StructuralImmunityParams{
		PartnershipHHIReductionPerParticipantBps: 10_000,  // 1% per participant
		MaxPartnershipHHIReductionBps:            200_000, // max 20%
		PartnershipReputationBonusBps:            20_000,  // 2% per partnership
		MaxPartnershipReputationBonusBps:         100_000, // max 10%
		MinDensityForAcceleratedClear:            10,
		CapturedDomainFormationBonusBps:          300_000, // 30% match priority boost
		FormationBonusDurationBlocks:             50_000,  // ~35 hours
		ActivityThresholdRelaxationBps:           200_000, // max 20% HHI threshold relaxation (R31-4)
	}
}

func DefaultCrossStratumRules() []*CrossStratumRequirement {
	return []*CrossStratumRequirement{
		{
			TargetStratum:            "theoretical",
			RequiredStrata:           []string{"empirical"},
			MinValidatorsPerStratum:  1,
		},
		{
			TargetStratum:            "applied",
			RequiredStrata:           []string{"theoretical", "empirical"},
			MinValidatorsPerStratum:  1,
		},
	}
}

// ---------- Msg Validation ----------

func (msg *MsgRecordVerification) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(msg.Authority); err != nil {
		return fmt.Errorf("invalid authority address: %w", err)
	}
	if msg.Domain == "" {
		return fmt.Errorf("domain is required")
	}
	if msg.RoundId == "" {
		return fmt.Errorf("round_id is required")
	}
	if len(msg.Validators) == 0 {
		return fmt.Errorf("at least one validator is required")
	}
	if len(msg.Validators) != len(msg.Verdicts) {
		return fmt.Errorf("validators and verdicts must have equal length")
	}
	if len(msg.SubmitBlocks) > 0 && len(msg.SubmitBlocks) != len(msg.Validators) {
		return fmt.Errorf("submit_blocks must match validators length when provided")
	}
	return nil
}

func (msg *MsgRecordVerification) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Authority)
	return []sdk.AccAddress{addr}
}

func (msg *MsgAnalyzeDomain) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(msg.Sender); err != nil {
		return fmt.Errorf("invalid sender address: %w", err)
	}
	if msg.Domain == "" {
		return fmt.Errorf("domain is required")
	}
	return nil
}

func (msg *MsgAnalyzeDomain) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Sender)
	return []sdk.AccAddress{addr}
}
