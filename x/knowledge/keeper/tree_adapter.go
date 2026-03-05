package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	treetypes "github.com/zerone-chain/zerone/x/tree/types"
)

// TreeKnowledgeAdapter wraps the knowledge Keeper to satisfy the
// tree module's KnowledgeKeeper interface.
type TreeKnowledgeAdapter struct {
	k Keeper
}

// NewTreeKnowledgeAdapter returns an adapter for the tree module.
func NewTreeKnowledgeAdapter(k Keeper) *TreeKnowledgeAdapter {
	return &TreeKnowledgeAdapter{k: k}
}

// Ensure compile-time interface compliance.
var _ treetypes.KnowledgeKeeper = (*TreeKnowledgeAdapter)(nil)

// CreateProjectBounty creates a data bounty linked to a tree project.
func (a *TreeKnowledgeAdapter) CreateProjectBounty(ctx context.Context, domain string, targetCount, minQuality uint64, budget sdk.Coins, projectID string) error {
	return a.k.CreateProjectBounty(ctx, domain, targetCount, minQuality, budget, projectID)
}

// GetBountyProgress returns the current progress for a project bounty.
func (a *TreeKnowledgeAdapter) GetBountyProgress(ctx context.Context, projectID string) (current uint64, target uint64, found bool) {
	return a.k.GetBountyProgress(ctx, projectID)
}
