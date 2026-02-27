package keeper

import (
	"context"

	alignmenttypes "github.com/zerone-chain/zerone/x/alignment/types"
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

// GetVerificationRate returns the current verification rate in BPS.
// Stub: returns 500,000 (50%) until knowledge module exposes real metrics.
func (a *AlignmentKnowledgeAdapter) GetVerificationRate(_ context.Context) uint64 {
	return 500_000
}

// GetTotalFacts returns the total number of facts.
// Stub: returns 0 until knowledge module exposes real metrics.
func (a *AlignmentKnowledgeAdapter) GetTotalFacts(_ context.Context) uint64 {
	return 0
}

// GetConsensusDiversity returns the global consensus diversity score in BPS.
func (a *AlignmentKnowledgeAdapter) GetConsensusDiversity(ctx context.Context) uint64 {
	return a.k.GetGlobalConsensusDiversity(ctx)
}
