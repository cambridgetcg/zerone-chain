package keeper

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/tree/types"
)

// LinkDataCollectionProject creates a knowledge module DataBounty linked to a tree project.
// Called when a project with phase "seed" and ProjectTypeDataCollection is created.
func (k Keeper) LinkDataCollectionProject(ctx context.Context, project *types.ProductProject, budget sdk.Coins, targetCount, minQuality uint64) error {
	if k.knowledgeKeeper == nil {
		return fmt.Errorf("knowledge keeper not wired")
	}
	domain := project.KnowledgeDomain
	if domain == "" {
		return fmt.Errorf("project %s: knowledge_domain is required for data collection", project.Id)
	}
	return k.knowledgeKeeper.CreateProjectBounty(ctx, domain, targetCount, minQuality, budget, project.Id)
}

// GetDataCollectionProgress returns the progress of a data collection campaign.
func (k Keeper) GetDataCollectionProgress(ctx context.Context, projectID string) (current, target uint64, found bool) {
	if k.knowledgeKeeper == nil {
		return 0, 0, false
	}
	return k.knowledgeKeeper.GetBountyProgress(ctx, projectID)
}
