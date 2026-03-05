package types

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// MaxBPS is the maximum basis points value (100%).
	MaxBPS = 1_000_000
)

// ValidateBasic performs stateless validation for MsgSubmitData.
func (m *MsgSubmitData) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Submitter); err != nil {
		return fmt.Errorf("invalid submitter address: %w", err)
	}
	if len(m.Content) == 0 {
		return ErrInvalidSubmission.Wrap("content must not be empty")
	}
	if len(m.Domain) == 0 {
		return ErrInvalidDomain.Wrap("domain must not be empty")
	}
	if m.Consent == nil {
		return ErrConsentRequired.Wrap("consent proof is required")
	}
	if m.Consent.Type == ConsentType_CONSENT_TYPE_UNSPECIFIED {
		return ErrInvalidConsent.Wrap("consent type must be specified")
	}
	if m.SampleType == SampleType_SAMPLE_TYPE_UNSPECIFIED {
		return ErrInvalidSubmission.Wrap("sample type must be specified")
	}
	if len(m.Language) > 0 && len(m.Language) != 2 {
		return ErrInvalidSubmission.Wrap("language must be ISO 639-1 (2 characters)")
	}
	return nil
}

// ValidateBasic performs stateless validation for MsgSubmitThread.
func (m *MsgSubmitThread) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Submitter); err != nil {
		return fmt.Errorf("invalid submitter address: %w", err)
	}
	if len(m.Items) < 2 {
		return ErrInvalidSubmission.Wrap("thread must have at least 2 items")
	}
	if len(m.Domain) == 0 {
		return ErrInvalidDomain.Wrap("domain must not be empty")
	}
	if len(m.ThreadId) == 0 {
		return ErrInvalidSubmission.Wrap("thread_id must not be empty")
	}
	for i, item := range m.Items {
		if len(item.Content) == 0 {
			return ErrInvalidSubmission.Wrapf("item[%d]: content must not be empty", i)
		}
		if item.ThreadId != "" && item.ThreadId != m.ThreadId {
			return ErrInvalidSubmission.Wrapf("item[%d]: thread_id mismatch", i)
		}
	}
	return nil
}

// ValidateBasic performs stateless validation for MsgSubmitCommitment.
func (m *MsgSubmitCommitment) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Verifier); err != nil {
		return fmt.Errorf("invalid verifier address: %w", err)
	}
	if len(m.RoundId) == 0 {
		return ErrRoundNotFound.Wrap("round_id must not be empty")
	}
	if len(m.CommitHash) == 0 {
		return ErrInvalidCommitment.Wrap("commit_hash must not be empty")
	}
	return nil
}

// ValidateBasic performs stateless validation for MsgSubmitReveal.
func (m *MsgSubmitReveal) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Verifier); err != nil {
		return fmt.Errorf("invalid verifier address: %w", err)
	}
	if len(m.RoundId) == 0 {
		return ErrRoundNotFound.Wrap("round_id must not be empty")
	}
	if m.Scores == nil {
		return ErrInvalidQualityScore.Wrap("scores must not be nil")
	}
	if m.Scores.OverallQuality > MaxBPS {
		return ErrInvalidQualityScore.Wrapf("overall_quality %d exceeds max BPS %d", m.Scores.OverallQuality, MaxBPS)
	}
	if m.Scores.ReasoningDepth > MaxBPS {
		return ErrInvalidQualityScore.Wrapf("reasoning_depth %d exceeds max BPS %d", m.Scores.ReasoningDepth, MaxBPS)
	}
	if m.Scores.Novelty > MaxBPS {
		return ErrInvalidQualityScore.Wrapf("novelty %d exceeds max BPS %d", m.Scores.Novelty, MaxBPS)
	}
	if m.Scores.Toxicity > MaxBPS {
		return ErrInvalidQualityScore.Wrapf("toxicity %d exceeds max BPS %d", m.Scores.Toxicity, MaxBPS)
	}
	if m.Scores.FactualAccuracy > MaxBPS {
		return ErrInvalidQualityScore.Wrapf("factual_accuracy %d exceeds max BPS %d", m.Scores.FactualAccuracy, MaxBPS)
	}
	if len(m.Salt) == 0 {
		return ErrRevealMismatch.Wrap("salt must not be empty")
	}
	return nil
}

// ValidateBasic performs stateless validation for MsgContestSample.
func (m *MsgContestSample) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Challenger); err != nil {
		return fmt.Errorf("invalid challenger address: %w", err)
	}
	if len(m.SampleId) == 0 {
		return ErrSampleNotFound.Wrap("sample_id must not be empty")
	}
	if len(m.Reason) == 0 {
		return ErrInvalidChallenge.Wrap("reason must not be empty")
	}
	if m.ContestType == ContestType_CONTEST_TYPE_UNSPECIFIED {
		return ErrInvalidChallenge.Wrap("contest_type must be specified")
	}
	return nil
}

// ValidateBasic performs stateless validation for MsgCreateDataset.
func (m *MsgCreateDataset) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Curator); err != nil {
		return fmt.Errorf("invalid curator address: %w", err)
	}
	if len(m.Name) == 0 {
		return ErrInvalidSubmission.Wrap("dataset name must not be empty")
	}
	if m.MinQuality > MaxBPS {
		return ErrInvalidQualityScore.Wrapf("min_quality %d exceeds max BPS %d", m.MinQuality, MaxBPS)
	}
	return nil
}

// ValidateBasic performs stateless validation for MsgAccessDataset.
func (m *MsgAccessDataset) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Consumer); err != nil {
		return fmt.Errorf("invalid consumer address: %w", err)
	}
	if len(m.DatasetId) == 0 {
		return ErrDatasetNotFound.Wrap("dataset_id must not be empty")
	}
	return nil
}

// ValidateBasic performs stateless validation for MsgAccessSample.
func (m *MsgAccessSample) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Consumer); err != nil {
		return fmt.Errorf("invalid consumer address: %w", err)
	}
	if len(m.SampleId) == 0 {
		return ErrSampleNotFound.Wrap("sample_id must not be empty")
	}
	return nil
}

// ValidateBasic performs stateless validation for MsgRateSample.
func (m *MsgRateSample) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Rater); err != nil {
		return fmt.Errorf("invalid rater address: %w", err)
	}
	if len(m.SampleId) == 0 {
		return ErrSampleNotFound.Wrap("sample_id must not be empty")
	}
	return nil
}

// ValidateBasic performs stateless validation for MsgFundBounty.
func (m *MsgFundBounty) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Funder); err != nil {
		return fmt.Errorf("invalid funder address: %w", err)
	}
	if len(m.Domain) == 0 {
		return ErrInvalidDomain.Wrap("domain must not be empty")
	}
	if len(m.Amount) == 0 {
		return ErrInsufficientPayment.Wrap("amount must not be empty")
	}
	return nil
}

// ValidateBasic performs stateless validation for MsgSponsorSample.
func (m *MsgSponsorSample) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Sponsor); err != nil {
		return fmt.Errorf("invalid sponsor address: %w", err)
	}
	if len(m.SampleId) == 0 {
		return ErrSampleNotFound.Wrap("sample_id must not be empty")
	}
	if len(m.Amount) == 0 {
		return ErrInsufficientPayment.Wrap("amount must not be empty")
	}
	return nil
}

// ValidateBasic performs stateless validation for MsgProposeDomain.
func (m *MsgProposeDomain) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Proposer); err != nil {
		return fmt.Errorf("invalid proposer address: %w", err)
	}
	if len(m.Name) == 0 {
		return ErrInvalidDomain.Wrap("domain name must not be empty")
	}
	if len(m.Name) > 64 {
		return ErrInvalidDomain.Wrap("domain name must be <= 64 characters")
	}
	return nil
}

// ValidateBasic performs stateless validation for MsgEndorseDomainProposal.
func (m *MsgEndorseDomainProposal) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Endorser); err != nil {
		return fmt.Errorf("invalid endorser address: %w", err)
	}
	if len(m.ProposalId) == 0 {
		return ErrDomainNotFound.Wrap("proposal_id must not be empty")
	}
	return nil
}

// ValidateBasic performs stateless validation for MsgAddScrapedSource.
func (m *MsgAddScrapedSource) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Authority); err != nil {
		return fmt.Errorf("invalid authority address: %w", err)
	}
	if len(m.Platform) == 0 {
		return ErrInvalidSubmission.Wrap("platform must not be empty")
	}
	if len(m.Domain) == 0 {
		return ErrInvalidDomain.Wrap("domain must not be empty")
	}
	if m.NoveltyPenalty > MaxBPS {
		return ErrInvalidQualityScore.Wrapf("novelty_penalty %d exceeds max BPS %d", m.NoveltyPenalty, MaxBPS)
	}
	return nil
}

// ValidateBasic performs stateless validation for MsgRemoveScrapedSource.
func (m *MsgRemoveScrapedSource) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Authority); err != nil {
		return fmt.Errorf("invalid authority address: %w", err)
	}
	if len(m.Id) == 0 {
		return ErrInvalidSubmission.Wrap("id must not be empty")
	}
	return nil
}

// ValidateBasic performs stateless validation for MsgRevokeConsent.
func (m *MsgRevokeConsent) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Requester); err != nil {
		return fmt.Errorf("invalid requester address: %w", err)
	}
	if len(m.SampleId) == 0 {
		return ErrSampleNotFound.Wrap("sample_id must not be empty")
	}
	return nil
}

// ValidateBasic performs stateless validation for MsgUpgradeConsent.
func (m *MsgUpgradeConsent) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Submitter); err != nil {
		return fmt.Errorf("invalid submitter address: %w", err)
	}
	if len(m.SampleId) == 0 {
		return ErrSampleNotFound.Wrap("sample_id must not be empty")
	}
	if m.NewConsent == nil {
		return ErrConsentRequired.Wrap("new consent proof is required")
	}
	if m.NewConsent.Type == ConsentType_CONSENT_TYPE_UNSPECIFIED {
		return ErrInvalidConsent.Wrap("consent type must be specified")
	}
	return nil
}

// ValidateBasic performs stateless validation for MsgReportDemand.
func (m *MsgReportDemand) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Reporter); err != nil {
		return fmt.Errorf("invalid reporter address: %w", err)
	}
	if len(m.Reports) == 0 {
		return ErrInvalidSubmission.Wrap("reports must not be empty")
	}
	for i, r := range m.Reports {
		if len(r.Domain) == 0 {
			return ErrInvalidDomain.Wrapf("report[%d]: domain must not be empty", i)
		}
		if len(r.Subject) == 0 {
			return ErrInvalidSubmission.Wrapf("report[%d]: subject must not be empty", i)
		}
	}
	return nil
}
