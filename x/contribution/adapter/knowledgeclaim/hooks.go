package knowledgeclaim

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	contribkeeper "github.com/zerone-chain/zerone/x/contribution/keeper"
	contribtypes "github.com/zerone-chain/zerone/x/contribution/types"
	knowledgetypes "github.com/zerone-chain/zerone/x/knowledge/types"
)

// KnowledgeHooksAdapter implements knowledgetypes.KnowledgeHooks.
// It mirrors claim lifecycle into Contribution lifecycle.
type KnowledgeHooksAdapter struct {
	contribKeeper *contribkeeper.Keeper
	adapter       Adapter
}

// NewKnowledgeHooksAdapter constructs the hooks adapter.
func NewKnowledgeHooksAdapter(ck *contribkeeper.Keeper, a Adapter) KnowledgeHooksAdapter {
	return KnowledgeHooksAdapter{contribKeeper: ck, adapter: a}
}

var _ knowledgetypes.KnowledgeHooks = KnowledgeHooksAdapter{}

// AfterClaimSubmitted constructs the Contribution mirror in
// STATUS_SUBMITTED, runs Classify + SubstrateLink, transitions to
// STATUS_CLASSIFIED on success or STATUS_CLASSIFICATION_FAILED on error.
func (h KnowledgeHooksAdapter) AfterClaimSubmitted(ctx context.Context, claimID string, snap knowledgetypes.ClaimSnapshot) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	c := BuildContributionFromSnapshot(claimID, snap, sdkCtx.BlockHeight())

	// Stage ② — Classify.
	if err := h.adapter.Classify(ctx, c); err != nil {
		c.Status = contribtypes.ContributionStatus_STATUS_CLASSIFICATION_FAILED
		_ = h.contribKeeper.WriteContribution(ctx, c)
		h.contribKeeper.EmitClassificationFailed(ctx, c.Id, err.Error())
		return nil
	}
	linkBps, err := h.adapter.SubstrateLink(ctx, c)
	if err != nil {
		c.Status = contribtypes.ContributionStatus_STATUS_CLASSIFICATION_FAILED
		_ = h.contribKeeper.WriteContribution(ctx, c)
		h.contribKeeper.EmitClassificationFailed(ctx, c.Id, err.Error())
		return nil
	}
	c.SubstrateLinkBps = linkBps
	c.Status = contribtypes.ContributionStatus_STATUS_CLASSIFIED
	if err := h.contribKeeper.WriteContribution(ctx, c); err != nil {
		return err
	}
	h.contribKeeper.EmitContributionSubmitted(ctx, c)
	h.contribKeeper.EmitContributionClassified(ctx, c)
	return nil
}

// AfterClaimVerificationFinalized sets the verification_score and
// transitions to STATUS_VERIFIED or STATUS_VERIFICATION_FAILED.
// Emits useful_work_attested + useful_work_settled + recursion_weight_computed.
func (h KnowledgeHooksAdapter) AfterClaimVerificationFinalized(ctx context.Context, claimID string, scoreBps uint32) error {
	c, found := h.contribKeeper.GetContributionByBackRef(ctx, claimID)
	if !found {
		return nil // mirror absent — claim wasn't submitted under our hooks
	}
	c.VerificationScoreBps = scoreBps
	if scoreBps >= contribtypes.MinVerificationScoreBps {
		c.Status = contribtypes.ContributionStatus_STATUS_VERIFIED
	} else {
		c.Status = contribtypes.ContributionStatus_STATUS_VERIFICATION_FAILED
	}
	if err := h.contribKeeper.WriteContribution(ctx, c); err != nil {
		return err
	}
	h.contribKeeper.EmitUsefulWorkAttested(ctx, c)
	h.contribKeeper.EmitUsefulWorkSettled(ctx, c)       // shape-only at Phase 1
	h.contribKeeper.EmitRecursionWeightComputed(ctx, c) // all-zero at Phase 1
	return nil
}

// AfterClaimAccepted transitions to STATUS_ADMITTED and records the
// resulting fact_id in back_ref.
func (h KnowledgeHooksAdapter) AfterClaimAccepted(ctx context.Context, claimID string, factID string) error {
	c, found := h.contribKeeper.GetContributionByBackRef(ctx, claimID)
	if !found {
		return nil
	}
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	c.AdmittedAtBlock = uint64(sdkCtx.BlockHeight())
	c.Status = contribtypes.ContributionStatus_STATUS_ADMITTED
	// Update back_ref to the resulting fact_id (was claim_id at SUBMITTED).
	c.BackRef = factID
	if err := h.contribKeeper.WriteContribution(ctx, c); err != nil {
		return err
	}
	h.contribKeeper.EmitContributionAdmitted(ctx, c)
	return nil
}

// AfterClaimDisproven transitions to STATUS_REVOKED.
func (h KnowledgeHooksAdapter) AfterClaimDisproven(ctx context.Context, claimID string, disproverArtifactID string) error {
	c, found := h.contribKeeper.GetContributionByBackRef(ctx, claimID)
	if !found {
		return nil
	}
	c.Status = contribtypes.ContributionStatus_STATUS_REVOKED
	if err := h.contribKeeper.WriteContribution(ctx, c); err != nil {
		return err
	}
	h.contribKeeper.EmitContributionRevoked(ctx, c, disproverArtifactID)
	return nil
}
