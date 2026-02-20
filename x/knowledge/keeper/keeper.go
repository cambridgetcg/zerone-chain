package keeper

import (
	"context"

	"cosmossdk.io/core/store"
	"github.com/cosmos/cosmos-sdk/codec"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// Keeper holds module state for the knowledge module.
type Keeper struct {
	storeService store.KVStoreService
	cdc          codec.BinaryCodec
	authority    string // governance authority address
}

// NewKeeper creates a new knowledge Keeper.
func NewKeeper(
	storeService store.KVStoreService,
	cdc codec.BinaryCodec,
	authority string,
) Keeper {
	return Keeper{
		storeService: storeService,
		cdc:          cdc,
		authority:    authority,
	}
}

// GetAuthority returns the module's governance authority address.
func (k Keeper) GetAuthority() string {
	return k.authority
}

// InitGenesis initialises module state from a genesis state.
// Full implementation in R2-2.
func (k Keeper) InitGenesis(_ context.Context, _ *types.GenesisState) error {
	return nil
}

// ExportGenesis exports the current module state as a genesis state.
// Full implementation in R2-2.
func (k Keeper) ExportGenesis(_ context.Context) *types.GenesisState {
	return types.DefaultGenesis()
}
