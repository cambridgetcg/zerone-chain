package types

import "context"

// KnowledgeHooks consumers register at app init via Keeper.SetHooks.
// x/knowledge calls them at lifecycle moments. Multi-consumer
// dispatch via MultiKnowledgeHooks (registration order).
//
// Hook errors are swallowed (logged but not propagated) by the
// caller — a misbehaving consumer must not break the underlying
// claim flow. Mirrors the x/staking.StakingHooks convention.
type KnowledgeHooks interface {
	AfterClaimSubmitted(ctx context.Context, claimID string, claim ClaimSnapshot) error
	AfterClaimVerificationFinalized(ctx context.Context, claimID string, scoreBps uint32) error
	AfterClaimAccepted(ctx context.Context, claimID string, factID string) error
	AfterClaimDisproven(ctx context.Context, claimID string, disproverArtifactID string) error
}

// ClaimSnapshot is a stable subset of x/knowledge.Claim safe to expose
// to external hook consumers. Defined here (not as Claim itself) so
// internal x/knowledge refactors don't break consumers.
type ClaimSnapshot struct {
	Submitter        string
	Domain           string
	StatementHash    []byte
	MethodologyTrace []byte
	AxiomRefs        []string
	TokManifestCID   string
	SubmittedAtBlock uint64
}

// MultiKnowledgeHooks dispatches to multiple consumers in
// registration order. Errors from any consumer are returned aggregated
// only if needed; the caller (handler) typically swallows.
type MultiKnowledgeHooks []KnowledgeHooks

func (m MultiKnowledgeHooks) AfterClaimSubmitted(ctx context.Context, claimID string, claim ClaimSnapshot) error {
	for _, h := range m {
		_ = h.AfterClaimSubmitted(ctx, claimID, claim)
	}
	return nil
}

func (m MultiKnowledgeHooks) AfterClaimVerificationFinalized(ctx context.Context, claimID string, scoreBps uint32) error {
	for _, h := range m {
		_ = h.AfterClaimVerificationFinalized(ctx, claimID, scoreBps)
	}
	return nil
}

func (m MultiKnowledgeHooks) AfterClaimAccepted(ctx context.Context, claimID string, factID string) error {
	for _, h := range m {
		_ = h.AfterClaimAccepted(ctx, claimID, factID)
	}
	return nil
}

func (m MultiKnowledgeHooks) AfterClaimDisproven(ctx context.Context, claimID string, disproverArtifactID string) error {
	for _, h := range m {
		_ = h.AfterClaimDisproven(ctx, claimID, disproverArtifactID)
	}
	return nil
}
