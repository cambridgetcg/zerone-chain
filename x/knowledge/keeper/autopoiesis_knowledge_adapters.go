package keeper

import (
	"context"

	aptypes "github.com/zerone-chain/zerone/x/autopoiesis/types"
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

// GetVerificationRate returns neutral BPS (no verification data in training data protocol).
func (a *KnowledgeForAutopoiesisAdapter) GetVerificationRate(_ context.Context) uint64 {
	return 500_000 // NeutralBPS
}
