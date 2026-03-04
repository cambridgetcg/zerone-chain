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

// GetVerificationRate returns neutral BPS (no verification data in training data protocol).
func (a *AlignmentKnowledgeAdapter) GetVerificationRate(_ context.Context) uint64 {
	return 500_000 // NeutralBPS
}

// GetTotalFacts returns 0 (fact concept removed in training data protocol).
func (a *AlignmentKnowledgeAdapter) GetTotalFacts(_ context.Context) uint64 {
	return 0
}

// GetConsensusDiversity returns neutral BPS.
func (a *AlignmentKnowledgeAdapter) GetConsensusDiversity(_ context.Context) uint64 {
	return 500_000
}

// GetPendingVerificationRatio returns 0 (no pending claims in training data protocol).
func (a *AlignmentKnowledgeAdapter) GetPendingVerificationRatio(_ context.Context) uint64 {
	return 0
}

// GetVerificationHealth returns neutral defaults.
func (a *AlignmentKnowledgeAdapter) GetVerificationHealth(_ context.Context) (uint64, uint64, uint64) {
	return 500_000, 0, 0
}
