package keeper

import (
	"context"

	"github.com/zerone-chain/zerone/x/substrate_bridge/types"
)

func (k Keeper) InitGenesis(ctx context.Context, gs *types.GenesisState) error {
	if gs.Params != nil {
		if err := k.SetParams(ctx, *gs.Params); err != nil {
			return err
		}
	}
	for _, a := range gs.Adapters {
		if err := k.WriteAdapter(ctx, a); err != nil {
			return err
		}
	}
	return nil
}

func (k Keeper) ExportGenesis(ctx context.Context) *types.GenesisState {
	params := k.GetParams(ctx)
	gs := &types.GenesisState{Params: &params}
	k.IterateAdapters(ctx, func(a *types.AdapterRegistration) bool {
		gs.Adapters = append(gs.Adapters, a)
		return false
	})
	return gs
}
