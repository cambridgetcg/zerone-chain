package keeper

import (
	"context"

	aptypes "github.com/zerone-chain/zerone/x/autopoiesis/types"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// KnowledgeForAutopoiesisAdapter wraps the knowledge Keeper to satisfy
// aptypes.KnowledgeKeeper (GetVerificationRate).
type KnowledgeForAutopoiesisAdapter struct {
	k Keeper
}

// NewKnowledgeForAutopoiesisAdapter returns an adapter providing knowledge metrics
// to the autopoiesis module.
func NewKnowledgeForAutopoiesisAdapter(k Keeper) *KnowledgeForAutopoiesisAdapter {
	return &KnowledgeForAutopoiesisAdapter{k: k}
}

// Compile-time interface check.
var _ aptypes.KnowledgeKeeper = (*KnowledgeForAutopoiesisAdapter)(nil)

// GetVerificationRate computes accepted / terminal claims in BPS.
func (a *KnowledgeForAutopoiesisAdapter) GetVerificationRate(ctx context.Context) uint64 {
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
