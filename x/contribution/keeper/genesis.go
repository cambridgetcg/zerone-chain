package keeper

import (
	"context"

	corestoretypes "cosmossdk.io/core/store"

	"github.com/zerone-chain/zerone/x/contribution/types"
)

// InitGenesis writes all Contribution records from genesis state.
func (k Keeper) InitGenesis(ctx context.Context, gs *types.GenesisState) {
	for _, c := range gs.Contributions {
		if err := k.WriteContribution(ctx, c); err != nil {
			panic(err)
		}
	}
}

// ExportGenesis dumps all Contributions by iterating the primary store.
func (k Keeper) ExportGenesis(ctx context.Context) *types.GenesisState {
	store := k.storeService.OpenKVStore(ctx)
	gs := types.DefaultGenesis()

	end := prefixEndBytes(types.ContributionKey)
	iter, err := openIterator(store, types.ContributionKey, end)
	if err != nil {
		panic(err)
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var c types.Contribution
		if err := k.cdc.Unmarshal(iter.Value(), &c); err != nil {
			panic(err)
		}
		gs.Contributions = append(gs.Contributions, &c)
	}
	return gs
}

// openIterator wraps the store's Iterator method behind a typed alias.
// Kept private to the package so we can swap iteration implementations
// if needed.
type kvIterator interface {
	Valid() bool
	Next()
	Key() []byte
	Value() []byte
	Close() error
}

func openIterator(store corestoretypes.KVStore, start, end []byte) (kvIterator, error) {
	return store.Iterator(start, end)
}
