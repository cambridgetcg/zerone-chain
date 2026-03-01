package keeper

import (
	"context"

	alignmenttypes "github.com/zerone-chain/zerone/x/alignment/types"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// AlignmentKnowledgeAdapter wraps the knowledge Keeper to satisfy
// alignmenttypes.KnowledgeKeeper interface.
type AlignmentKnowledgeAdapter struct {
	k Keeper
}

// NewAlignmentKnowledgeAdapter returns an adapter for the alignment module.
func NewAlignmentKnowledgeAdapter(k Keeper) *AlignmentKnowledgeAdapter {
	return &AlignmentKnowledgeAdapter{k: k}
}

// Ensure compile-time interface compliance.
var _ alignmenttypes.KnowledgeKeeper = (*AlignmentKnowledgeAdapter)(nil)

// GetVerificationRate computes accepted / terminal claims in BPS.
func (a *AlignmentKnowledgeAdapter) GetVerificationRate(ctx context.Context) uint64 {
	var accepted, terminal uint64
	a.k.IterateClaims(ctx, func(claim *types.Claim) bool {
		switch claim.Status {
		case types.ClaimStatus_CLAIM_STATUS_ACCEPTED:
			accepted++
			terminal++
		case types.ClaimStatus_CLAIM_STATUS_REJECTED,
			types.ClaimStatus_CLAIM_STATUS_MALFORMED,
			types.ClaimStatus_CLAIM_STATUS_INSUFFICIENT:
			terminal++
		}
		return false
	})
	if terminal == 0 {
		return 500_000 // NeutralBPS — no data yet
	}
	rate := accepted * 1_000_000 / terminal
	if rate > 1_000_000 {
		return 1_000_000
	}
	return rate
}

// GetTotalFacts counts all accepted claims (facts).
func (a *AlignmentKnowledgeAdapter) GetTotalFacts(ctx context.Context) uint64 {
	var count uint64
	a.k.IterateClaims(ctx, func(claim *types.Claim) bool {
		if claim.Status == types.ClaimStatus_CLAIM_STATUS_ACCEPTED {
			count++
		}
		return false
	})
	return count
}

// GetConsensusDiversity returns the global consensus diversity score in BPS.
func (a *AlignmentKnowledgeAdapter) GetConsensusDiversity(ctx context.Context) uint64 {
	return a.k.GetGlobalConsensusDiversity(ctx)
}

// GetPendingVerificationRatio returns pending claims / active facts in BPS (R31-1).
func (a *AlignmentKnowledgeAdapter) GetPendingVerificationRatio(ctx context.Context) uint64 {
	return a.k.GetPendingVerificationRatio(ctx)
}
