package types

import (
	"encoding/json"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// ─── SubmissionType ──────────────────────────────────────────────────────────

// SubmissionType classifies whether a submission is standard or martyrance.
type SubmissionType int32

const (
	SubmissionTypeStandard   SubmissionType = 0
	SubmissionTypeMartyrance SubmissionType = 1
)

func (t SubmissionType) String() string {
	switch t {
	case SubmissionTypeMartyrance:
		return "MARTYRANCE"
	default:
		return "STANDARD"
	}
}

// ─── MartyranceSubmissionMeta ─────────────────────────────────────────────────

// MartyranceSubmissionMeta stores martyrance-specific fields separately from
// the core Submission proto struct (following the project's extended-state pattern).
type MartyranceSubmissionMeta struct {
	SubmissionID string `json:"submission_id"`
	Testimony    string `json:"testimony"`
}

func (m *MartyranceSubmissionMeta) UnmarshalJSON(bz []byte) error {
	type alias MartyranceSubmissionMeta
	var a alias
	if err := json.Unmarshal(bz, &a); err != nil {
		return err
	}
	*m = MartyranceSubmissionMeta(a)
	return nil
}

// ─── MartyranceRoundMeta ──────────────────────────────────────────────────────

// MartyranceRoundMeta stores martyrance-specific quality round flags separately
// from the core QualityRound proto struct.
type MartyranceRoundMeta struct {
	RoundID               string   `json:"round_id"`
	IsMartyrance          bool     `json:"is_martyrance"`
	IsSecondaryMartyrance bool     `json:"is_secondary_martyrance"`
	ExcludedVerifiers     []string `json:"excluded_verifiers,omitempty"`
}

func (m *MartyranceRoundMeta) UnmarshalJSON(bz []byte) error {
	type alias MartyranceRoundMeta
	var a alias
	if err := json.Unmarshal(bz, &a); err != nil {
		return err
	}
	*m = MartyranceRoundMeta(a)
	return nil
}

// ─── MsgSubmitMartyranceClaim ─────────────────────────────────────────────────

// MsgSubmitMartyranceClaim is the transaction message for martyrance testimony.
// Manually implemented pending proto-gen integration.
type MsgSubmitMartyranceClaim struct {
	Submitter string `json:"submitter"`
	Content   string `json:"content"`
	Domain    string `json:"domain"`
	Category  string `json:"category"`
	Testimony string `json:"testimony"` // Required justification
}

func (*MsgSubmitMartyranceClaim) ProtoMessage()           {}
func (*MsgSubmitMartyranceClaim) Reset()                  {}
func (*MsgSubmitMartyranceClaim) String() string          { return "MsgSubmitMartyranceClaim" }
func (*MsgSubmitMartyranceClaim) XXX_MessageName() string { return "zerone.knowledge.v1.MsgSubmitMartyranceClaim" }

func (m *MsgSubmitMartyranceClaim) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Submitter); err != nil {
		return fmt.Errorf("invalid submitter address: %w", err)
	}
	if m.Content == "" {
		return fmt.Errorf("content is required")
	}
	if m.Domain == "" {
		return fmt.Errorf("domain is required")
	}
	if m.Testimony == "" {
		return ErrMartyranceTestimonyEmpty
	}
	return nil
}

func (m *MsgSubmitMartyranceClaim) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Submitter)
	return []sdk.AccAddress{addr}
}

func (m *MsgSubmitMartyranceClaim) GetSubmitter() string { return m.Submitter }
func (m *MsgSubmitMartyranceClaim) GetContent() string   { return m.Content }
func (m *MsgSubmitMartyranceClaim) GetDomain() string    { return m.Domain }
func (m *MsgSubmitMartyranceClaim) GetCategory() string  { return m.Category }
func (m *MsgSubmitMartyranceClaim) GetTestimony() string { return m.Testimony }

// MsgSubmitMartyranceClaimResponse is the response for MsgSubmitMartyranceClaim.
type MsgSubmitMartyranceClaimResponse struct {
	SubmissionId string `json:"submission_id,omitempty"`
	RoundId      string `json:"round_id,omitempty"`
	StakeAmount  string `json:"stake_amount,omitempty"`
}

func (*MsgSubmitMartyranceClaimResponse) ProtoMessage()  {}
func (*MsgSubmitMartyranceClaimResponse) Reset()         {}
func (*MsgSubmitMartyranceClaimResponse) String() string { return "MsgSubmitMartyranceClaimResponse" }

func (m *MsgSubmitMartyranceClaimResponse) GetSubmissionId() string { return m.SubmissionId }
func (m *MsgSubmitMartyranceClaimResponse) GetRoundId() string      { return m.RoundId }
func (m *MsgSubmitMartyranceClaimResponse) GetStakeAmount() string  { return m.StakeAmount }
