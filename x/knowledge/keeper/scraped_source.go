package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// AddScrapedSource registers a platform/domain as heavily scraped. Authority-only.
func (k Keeper) AddScrapedSource(ctx context.Context, msg *types.MsgAddScrapedSource) (*types.MsgAddScrapedSourceResponse, error) {
	if msg.Authority != k.authority {
		return nil, types.ErrUnauthorized.Wrap("only governance authority can add scraped sources")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)

	id := msg.Platform + "/" + msg.Domain
	entry := &types.ScrapedSourceEntry{
		Id:             id,
		Platform:       msg.Platform,
		Domain:         msg.Domain,
		Description:    msg.Description,
		NoveltyPenalty: msg.NoveltyPenalty,
		AddedBlock:     uint64(sdkCtx.BlockHeight()),
	}

	if err := k.SetScrapedSource(ctx, entry); err != nil {
		return nil, err
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"add_scraped_source",
		sdk.NewAttribute("id", id),
		sdk.NewAttribute("platform", msg.Platform),
		sdk.NewAttribute("domain", msg.Domain),
	))

	return &types.MsgAddScrapedSourceResponse{Id: id}, nil
}

// RemoveScrapedSource removes a scraped source entry. Authority-only.
func (k Keeper) RemoveScrapedSource(ctx context.Context, msg *types.MsgRemoveScrapedSource) (*types.MsgRemoveScrapedSourceResponse, error) {
	if msg.Authority != k.authority {
		return nil, types.ErrUnauthorized.Wrap("only governance authority can remove scraped sources")
	}

	_, found := k.GetScrapedSource(ctx, msg.Id)
	if !found {
		return nil, types.ErrSampleNotFound.Wrapf("scraped source %q not found", msg.Id)
	}

	if err := k.DeleteScrapedSource(ctx, msg.Id); err != nil {
		return nil, err
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"remove_scraped_source",
		sdk.NewAttribute("id", msg.Id),
	))

	return &types.MsgRemoveScrapedSourceResponse{}, nil
}
