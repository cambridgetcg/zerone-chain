package keeper

import (
	"context"

	disputestypes "github.com/zerone-chain/zerone/x/disputes/types"
	knowledgetypes "github.com/zerone-chain/zerone/x/knowledge/types"
)

// DisputesKnowledgeAdapter wraps the knowledge Keeper to satisfy
// disputestypes.KnowledgeKeeper interface.
type DisputesKnowledgeAdapter struct {
	k Keeper
}

// NewDisputesKnowledgeAdapter returns an adapter that bridges the knowledge keeper
// to the disputes module's expected interface.
func NewDisputesKnowledgeAdapter(k Keeper) *DisputesKnowledgeAdapter {
	return &DisputesKnowledgeAdapter{k: k}
}

// Ensure compile-time interface compliance.
var _ disputestypes.KnowledgeKeeper = (*DisputesKnowledgeAdapter)(nil)

func (a *DisputesKnowledgeAdapter) GetSample(_ context.Context, _ string) (*knowledgetypes.Sample, bool) {
	return nil, false // Stub: samples not yet stored (R37)
}
