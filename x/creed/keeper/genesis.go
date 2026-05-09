package keeper

import (
	"context"

	"github.com/zerone-chain/zerone/x/creed/types"
)

// InitGenesis seeds the chain's params and the version-1 pin (the
// Genesis Creed). Historical pins, if any, are recorded in
// ascending order so chain queries against them work the same as
// queries against post-launch pins.
//
// docs/TRUTH_SEEKING.md commitment 10 (forward-only audit): the
// genesis pin is version 1; any historical pins MUST be strictly
// older. The validator runs on InitGenesis, but the keeper trusts
// the validator and re-checks invariants here so an unvalidated
// genesis file (e.g., dev chains) still gets the structural guard.
func (k Keeper) InitGenesis(ctx context.Context, gs *types.GenesisState) {
	if gs == nil {
		return
	}
	if gs.Params != nil {
		_ = k.SetParams(ctx, *gs.Params)
	}
	// History first (older versions), then the genesis pin (current).
	for _, p := range gs.History {
		if p == nil {
			continue
		}
		_ = k.SetPin(ctx, p)
	}
	if gs.GenesisPin != nil {
		_ = k.SetPin(ctx, gs.GenesisPin)
	}
}

// ExportGenesis dumps the current pinned state. Pin history is
// included in ascending version order; the highest version is the
// current canonical pin.
func (k Keeper) ExportGenesis(ctx context.Context) *types.GenesisState {
	params := k.GetParams(ctx)
	cur := k.GetCurrentVersion(ctx)
	gs := &types.GenesisState{
		Params: &params,
	}
	if cur == 0 {
		return gs
	}
	pins := make([]*types.PinnedCreed, 0, cur)
	for v := uint32(1); v <= cur; v++ {
		p, ok := k.GetPin(ctx, v)
		if ok {
			pins = append(pins, p)
		}
	}
	if len(pins) == 0 {
		return gs
	}
	// Last pin is the current canonical; rest are history.
	gs.GenesisPin = pins[len(pins)-1]
	if len(pins) > 1 {
		gs.History = pins[:len(pins)-1]
	}
	return gs
}
