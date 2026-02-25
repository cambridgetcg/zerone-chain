package keeper

import (
	"context"
	"fmt"
	"math/big"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// PropagateProgenyCount increments progeny_count for all ancestors up the lineage chain.
func (k Keeper) PropagateProgenyCount(ctx context.Context, parentFactID string) {
	if parentFactID == "" {
		return
	}

	currentID := parentFactID
	for currentID != "" {
		ancestor, found := k.GetFact(ctx, currentID)
		if !found {
			break
		}
		ancestor.ProgenyCount++
		_ = k.SetFact(ctx, ancestor)
		currentID = ancestor.ParentFactId
	}
}

// DistributeLineageRoyalties distributes royalty payments up the lineage chain
// when a child fact earns vesting rewards.
func (k Keeper) DistributeLineageRoyalties(ctx context.Context, factID string, rewardAmount uint64) error {
	if k.bankKeeper == nil || rewardAmount == 0 {
		return nil
	}

	params, err := k.GetParams(ctx)
	if err != nil {
		return err
	}

	currentFactID := factID
	depth := uint64(0)
	currentRoyaltyBps := params.ReproductionRoyaltyBps

	for depth < params.ReproductionMaxRoyaltyDepth {
		fact, found := k.GetFact(ctx, currentFactID)
		if !found || fact.ParentFactId == "" {
			break
		}

		parentFact, found := k.GetFact(ctx, fact.ParentFactId)
		if !found {
			break
		}

		// Calculate royalty for this ancestor
		royalty := safeMulDiv(rewardAmount, currentRoyaltyBps, 1_000_000)
		if royalty > 0 {
			parentAddr, err := sdk.AccAddressFromBech32(parentFact.Submitter)
			if err == nil {
				coins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(new(big.Int).SetUint64(royalty))))
				_ = k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, parentAddr, coins)

				sdkCtx := sdk.UnwrapSDKContext(ctx)
				sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
					"zerone.knowledge.lineage_royalty",
					sdk.NewAttribute("child_fact_id", currentFactID),
					sdk.NewAttribute("ancestor_fact_id", parentFact.Id),
					sdk.NewAttribute("ancestor_submitter", parentFact.Submitter),
					sdk.NewAttribute("royalty_amount", fmt.Sprintf("%d", royalty)),
					sdk.NewAttribute("depth", fmt.Sprintf("%d", depth+1)),
				))
			}
		}

		// Decay for next generation
		currentRoyaltyBps = safeMulDiv(currentRoyaltyBps, params.ReproductionRoyaltyDecayBps, 1_000_000)
		currentFactID = fact.ParentFactId
		depth++
	}

	return nil
}

// CascadeDisproven marks children of a disproven fact as AT_RISK with halved energy.
// Recursive cascade with diminishing impact on deeper descendants.
func (k Keeper) CascadeDisproven(ctx context.Context, factID string) {
	fact, found := k.GetFact(ctx, factID)
	if !found {
		return
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)

	for _, childID := range fact.ChildFactIds {
		child, found := k.GetFact(ctx, childID)
		if !found {
			continue
		}

		// Children of disproven facts lose energy and enter AT_RISK
		child.Energy = child.Energy / 2
		if child.Status == types.FactStatus_FACT_STATUS_ACTIVE ||
			child.Status == types.FactStatus_FACT_STATUS_VERIFIED {
			child.Status = types.FactStatus_FACT_STATUS_AT_RISK
		}
		_ = k.SetFact(ctx, child)

		sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
			"zerone.knowledge.lineage_cascade",
			sdk.NewAttribute("parent_disproven", factID),
			sdk.NewAttribute("child_at_risk", childID),
			sdk.NewAttribute("child_energy", fmt.Sprintf("%d", child.Energy)),
		))

		// Recursive cascade — children's children also affected
		k.CascadeDisproven(ctx, childID)
	}
}
