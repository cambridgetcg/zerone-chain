package keeper

import (
	"fmt"
	"math/big"

	"cosmossdk.io/core/store"
	"cosmossdk.io/log"
	sdkmath "cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/research/types"
)

// Keeper manages the research module's state.
type Keeper struct {
	cdc          codec.Codec
	storeService store.KVStoreService
	bankKeeper   types.BankKeeper
	authority    string
}

// NewKeeper creates a new research module Keeper.
func NewKeeper(
	storeService store.KVStoreService,
	cdc codec.Codec,
	authority string,
	bk types.BankKeeper,
) Keeper {
	return Keeper{
		cdc:          cdc,
		storeService: storeService,
		bankKeeper:   bk,
		authority:    authority,
	}
}

// prefixEndBytes returns the end key for prefix iteration (exclusive).
func prefixEndBytes(prefix []byte) []byte {
	if len(prefix) == 0 {
		return nil
	}
	end := make([]byte, len(prefix))
	copy(end, prefix)
	for i := len(end) - 1; i >= 0; i-- {
		end[i]++
		if end[i] != 0 {
			return end
		}
	}
	return nil
}

// Logger returns a module-specific logger.
func (k Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", fmt.Sprintf("x/%s", types.ModuleName))
}

// GetAuthority returns the module authority address.
func (k Keeper) GetAuthority() string {
	return k.authority
}

// InitGenesis initializes the module's state from genesis.
func (k Keeper) InitGenesis(ctx sdk.Context, genState *types.GenesisState) {
	if genState.Params != nil {
		k.SetParams(ctx, genState.Params)
	}
	if genState.TreasuryBalance != nil {
		k.SetTreasuryBalance(ctx, genState.TreasuryBalance.Balance)
	}

	for _, sub := range genState.Researches {
		if sub != nil {
			k.SetResearch(ctx, sub)
		}
	}
	for _, bounty := range genState.Bounties {
		if bounty != nil {
			k.SetBounty(ctx, bounty)
		}
	}
	for _, review := range genState.PeerReviews {
		if review != nil {
			k.SetPeerReview(ctx, review)
		}
	}
}

// AutoResolveResearch resolves research submissions that have met review conditions.
func (k Keeper) AutoResolveResearch(ctx sdk.Context) error {
	params := k.GetParams(ctx)
	currentBlock := uint64(ctx.BlockHeight())

	researches := k.GetResearchesByStatus(ctx, types.ResearchStatusUnderReview)
	for _, research := range researches {
		// Use earliest review time (when review period started), not UpdatedAt
		// which resets on every review submission
		reviews := k.GetReviewsForResearch(ctx, research.Id)
		var firstReviewAt uint64
		for _, r := range reviews {
			if firstReviewAt == 0 || r.ReviewedAt < firstReviewAt {
				firstReviewAt = r.ReviewedAt
			}
		}
		if firstReviewAt == 0 || currentBlock-firstReviewAt < params.ReviewPeriodBlocks {
			continue
		}
		if research.ReviewCount < params.MinReviewerCount {
			continue
		}

		stakeInt := new(big.Int)
		stakeInt.SetString(research.Stake, 10)

		if research.AggregateScore >= params.AcceptanceScoreThreshold {
			research.Status = string(types.ResearchStatusAccepted)

			submitterAddr, err := sdk.AccAddressFromBech32(research.Submitter)
			if err != nil {
				k.Logger(ctx).Error("invalid submitter address", "research_id", research.Id, "error", err)
				continue
			}
			coins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(stakeInt)))
			if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, submitterAddr, coins); err != nil {
				k.Logger(ctx).Error("failed to return stake", "research_id", research.Id, "error", err)
				continue
			}
		} else {
			research.Status = string(types.ResearchStatusRejected)

			slashRate := new(big.Int).SetUint64(params.RejectionSlashBps)
			slashAmount := new(big.Int).Mul(stakeInt, slashRate)
			slashAmount.Div(slashAmount, new(big.Int).SetUint64(1000000))

			if slashAmount.Sign() > 0 {
				slashCoins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(slashAmount)))
				if err := k.bankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, "development_fund", slashCoins); err != nil {
					k.Logger(ctx).Error("failed to slash to dev fund", "research_id", research.Id, "error", err)
					continue
				}
			}

			remainder := new(big.Int).Sub(stakeInt, slashAmount)
			if remainder.Sign() > 0 {
				submitterAddr, err := sdk.AccAddressFromBech32(research.Submitter)
				if err != nil {
					k.Logger(ctx).Error("invalid submitter address", "research_id", research.Id, "error", err)
					continue
				}
				returnCoins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(remainder)))
				if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, submitterAddr, returnCoins); err != nil {
					k.Logger(ctx).Error("failed to return remainder", "research_id", research.Id, "error", err)
					continue
				}
			}
		}

		research.UpdatedAt = currentBlock
		k.SetResearch(ctx, research)

		var outcomeStr string
		if research.Status == string(types.ResearchStatusAccepted) {
			outcomeStr = "accepted"
		} else {
			outcomeStr = "rejected"
		}

		ctx.EventManager().EmitEvent(
			sdk.NewEvent(
				"zerone.research.research_auto_resolved",
				sdk.NewAttribute("research_id", research.Id),
				sdk.NewAttribute("outcome", outcomeStr),
				sdk.NewAttribute("aggregate_score", fmt.Sprintf("%d", research.AggregateScore)),
			),
		)
	}
	return nil
}

// AutoFulfillBounties fulfills bounties that have been claimed for longer than
// the fulfillment period.
func (k Keeper) AutoFulfillBounties(ctx sdk.Context) error {
	params := k.GetParams(ctx)
	currentBlock := uint64(ctx.BlockHeight())

	k.IterateBounties(ctx, func(b *types.Bounty) bool {
		if b.Status != string(types.BountyStatusClaimed) {
			return false
		}

		if b.ClaimedAt == 0 || currentBlock-b.ClaimedAt < params.BountyFulfillmentPeriodBlocks {
			return false
		}

		rewardInt := new(big.Int)
		if _, ok := rewardInt.SetString(b.Reward, 10); !ok || rewardInt.Sign() <= 0 {
			k.Logger(ctx).Error("invalid bounty reward", "bounty_id", b.Id)
			return false
		}

		claimerAddr, err := sdk.AccAddressFromBech32(b.ClaimedBy)
		if err != nil {
			k.Logger(ctx).Error("invalid claimer address", "bounty_id", b.Id, "error", err)
			return false
		}

		coins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(rewardInt)))
		if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, claimerAddr, coins); err != nil {
			k.Logger(ctx).Error("failed to pay bounty reward", "bounty_id", b.Id, "error", err)
			return false
		}

		b.Status = string(types.BountyStatusFulfilled)
		b.FulfilledBy = b.ClaimedBy
		k.SetBounty(ctx, b)

		ctx.EventManager().EmitEvent(
			sdk.NewEvent(
				"zerone.research.bounty_auto_fulfilled",
				sdk.NewAttribute("bounty_id", b.Id),
				sdk.NewAttribute("fulfilled_by", b.ClaimedBy),
				sdk.NewAttribute("reward", b.Reward),
			),
		)

		return false
	})

	return nil
}

// ExportGenesis exports the module's state.
func (k Keeper) ExportGenesis(ctx sdk.Context) *types.GenesisState {
	var researches []*types.Research
	k.IterateResearches(ctx, func(r *types.Research) bool {
		researches = append(researches, r)
		return false
	})

	var bounties []*types.Bounty
	k.IterateBounties(ctx, func(b *types.Bounty) bool {
		bounties = append(bounties, b)
		return false
	})

	var peerReviews []*types.PeerReview
	k.IterateResearches(ctx, func(r *types.Research) bool {
		reviews := k.GetReviewsForResearch(ctx, r.Id)
		peerReviews = append(peerReviews, reviews...)
		return false
	})

	params := k.GetParams(ctx)
	return &types.GenesisState{
		Params:          params,
		Researches:      researches,
		Bounties:        bounties,
		PeerReviews:     peerReviews,
		TreasuryBalance: &types.TreasuryBalance{Balance: k.GetTreasuryBalance(ctx)},
	}
}
