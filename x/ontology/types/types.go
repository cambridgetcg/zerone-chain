package types

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Stratum represents a level in the knowledge ontology.
type Stratum uint32

const (
	StratumAxiomatic     Stratum = 0
	StratumFormal        Stratum = 1
	StratumProtocol      Stratum = 2
	StratumComputational Stratum = 3
	StratumEmpirical     Stratum = 4
	StratumHistorical    Stratum = 5
	StratumTestimonial   Stratum = 6
	MaxStratum = StratumTestimonial
)

func (s Stratum) IsValid() bool {
	return s <= MaxStratum
}

func (dp *DomainProposal) HasVoted(addr string) bool {
	for _, v := range dp.Voters {
		if v == addr {
			return true
		}
	}
	return false
}

func (p *Params) Validate() error {
	if p.CrossStratumDiscount > 1000000 {
		return fmt.Errorf("cross stratum discount cannot exceed 1000000 basis points")
	}
	if p.MinEndorsements == 0 {
		return fmt.Errorf("min endorsements must be greater than 0")
	}
	if p.ProposalVotingPeriod == 0 {
		return fmt.Errorf("proposal voting period must be greater than 0")
	}
	return nil
}

// Logic zone types
type LogicZone string

const (
	ZonePropositional LogicZone = "propositional"
	ZonePresburger    LogicZone = "presburger"
	ZonePeano         LogicZone = "peano"
	ZoneSetTheory     LogicZone = "set_theory"
	ZoneEmpirical     LogicZone = "empirical"
)

type IncompletenessAcknowledgment struct {
	FactId         string    `json:"fact_id"`
	Zone           LogicZone `json:"zone"`
	Reason         string    `json:"reason"`
	AcknowledgedAt uint64    `json:"acknowledged_at"`
	AcknowledgedBy string    `json:"acknowledged_by"`
}

func (ia *IncompletenessAcknowledgment) Reset()         {}
func (ia *IncompletenessAcknowledgment) String() string { return fmt.Sprintf("%+v", *ia) }
func (ia *IncompletenessAcknowledgment) ProtoMessage()  {}

func DefaultLogicZones() []LogicZoneProperties {
	return []LogicZoneProperties{
		{Zone: string(ZonePropositional), Complete: true, Decidable: true, GoedelApplies: false, MaxConfidenceBps: 1000000, Description: "Propositional logic: complete, decidable, truth-table decidable"},
		{Zone: string(ZonePresburger), Complete: true, Decidable: true, GoedelApplies: false, MaxConfidenceBps: 1000000, Description: "Presburger arithmetic: complete, decidable (no multiplication)"},
		{Zone: string(ZonePeano), Complete: false, Decidable: false, GoedelApplies: true, MaxConfidenceBps: 850000, Description: "Peano arithmetic: incomplete (Godel), undecidable, contains true but unprovable statements"},
		{Zone: string(ZoneSetTheory), Complete: false, Decidable: false, GoedelApplies: true, MaxConfidenceBps: 800000, Description: "Set theory (ZFC): incomplete, undecidable, CH-independent"},
		{Zone: string(ZoneEmpirical), Complete: false, Decidable: false, GoedelApplies: false, MaxConfidenceBps: 700000, Description: "Empirical claims: inherently probabilistic, falsifiable but not provable"},
	}
}

const MergeTargetPrefix = "merge_into:"

func ParseMergeTarget(description string) string {
	if len(description) > len(MergeTargetPrefix) && description[:len(MergeTargetPrefix)] == MergeTargetPrefix {
		return description[len(MergeTargetPrefix):]
	}
	return ""
}

func FormatMergeDescription(targetDomain string) string {
	return MergeTargetPrefix + targetDomain
}

func ZoneFormality(zone LogicZone) int {
	switch zone {
	case ZonePropositional: return 0
	case ZonePresburger: return 1
	case ZonePeano: return 2
	case ZoneSetTheory: return 3
	case ZoneEmpirical: return 4
	default: return -1
	}
}

var ValidZonesForCategory = map[string]map[LogicZone]bool{
	"analytic":      {ZonePropositional: true, ZonePresburger: true},
	"formal":        {ZonePropositional: true, ZonePresburger: true, ZonePeano: true, ZoneSetTheory: true},
	"protocol":      {ZonePropositional: true, ZonePresburger: true},
	"computational": {ZonePropositional: true, ZonePresburger: true, ZonePeano: true},
	"replicated":    {ZonePresburger: true, ZonePeano: true, ZoneEmpirical: true},
	"empirical":     {ZoneEmpirical: true},
	"historical":    {ZoneEmpirical: true},
	"predictive":    {ZoneEmpirical: true},
	"social":        {ZoneEmpirical: true},
}

// GetSigners and ValidateBasic for proto-generated Msg types

func (msg *MsgProposeDomain) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Proposer)
	return []sdk.AccAddress{addr}
}

func (msg *MsgProposeDomain) ValidateBasic() error {
	if msg.Name == "" {
		return fmt.Errorf("domain name cannot be empty")
	}
	if msg.DisplayName == "" {
		return fmt.Errorf("display name cannot be empty")
	}
	if !Stratum(msg.Stratum).IsValid() {
		return fmt.Errorf("invalid stratum: %d", msg.Stratum)
	}
	if msg.Proposer == "" {
		return fmt.Errorf("proposer cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(msg.Proposer); err != nil {
		return fmt.Errorf("invalid proposer address: %w", err)
	}
	return nil
}

func (msg *MsgVoteDomainProposal) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Voter)
	return []sdk.AccAddress{addr}
}

func (msg *MsgVoteDomainProposal) ValidateBasic() error {
	if msg.ProposalId == "" {
		return fmt.Errorf("proposal ID cannot be empty")
	}
	if msg.Voter == "" {
		return fmt.Errorf("voter cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(msg.Voter); err != nil {
		return fmt.Errorf("invalid voter address: %w", err)
	}
	return nil
}

func (msg *MsgUpdateDomain) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Authority)
	return []sdk.AccAddress{addr}
}

func (msg *MsgUpdateDomain) ValidateBasic() error {
	if msg.Authority == "" {
		return fmt.Errorf("authority cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(msg.Authority); err != nil {
		return fmt.Errorf("invalid authority address: %w", err)
	}
	if msg.DomainName == "" {
		return fmt.Errorf("domain name cannot be empty")
	}
	if msg.Status != "" && msg.Status != "active" && msg.Status != "deprecated" {
		return fmt.Errorf("invalid status: must be 'active' or 'deprecated'")
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
	return nil
}

func (msg *MsgRegisterLogicZone) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Authority)
	return []sdk.AccAddress{addr}
}

func (msg *MsgRegisterLogicZone) ValidateBasic() error {
	if msg.Authority == "" {
		return fmt.Errorf("authority cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(msg.Authority); err != nil {
		return fmt.Errorf("invalid authority address: %w", err)
	}
	if msg.ZoneProperties == nil || msg.ZoneProperties.Zone == "" {
		return fmt.Errorf("zone name cannot be empty")
	}
	if msg.ZoneProperties.MaxConfidenceBps > 1000000 {
		return fmt.Errorf("max confidence cannot exceed 1000000 basis points")
	}
	return nil
}

func (msg *MsgAcknowledgeIncompleteness) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Submitter)
	return []sdk.AccAddress{addr}
}

func (msg *MsgAcknowledgeIncompleteness) ValidateBasic() error {
	if msg.Submitter == "" {
		return fmt.Errorf("submitter cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(msg.Submitter); err != nil {
		return fmt.Errorf("invalid submitter address: %w", err)
	}
	if msg.FactId == "" {
		return fmt.Errorf("fact ID cannot be empty")
	}
	if msg.Zone == "" {
		return fmt.Errorf("zone cannot be empty")
	}
	if msg.Reason == "" {
		return fmt.Errorf("reason cannot be empty")
	}
	return nil
}
