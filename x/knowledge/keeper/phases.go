package keeper

import "context"

func (k Keeper) BeginBlocker(_ context.Context) error {
	return nil
}
