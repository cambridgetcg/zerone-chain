package keeper

import (
	"cosmossdk.io/log"
	"cosmossdk.io/store/prefix"
	storetypes "cosmossdk.io/store/types"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/gov/types"
)

// Keeper manages the governance module's state.
type Keeper struct {
	cdc           codec.Codec
	storeKey      *storetypes.KVStoreKey
	authority     string
	bankKeeper    types.BankKeeper
	stakingKeeper types.StakingKeeper
	vestingKeeper types.VestingRewardsKeeper // set post-init to break circular deps
	upgradeKeeper types.UpgradeKeeper       // set post-init to break circular deps
}

// SetVestingKeeper sets the vesting rewards keeper (post-init to break circular deps).
func (k *Keeper) SetVestingKeeper(vk types.VestingRewardsKeeper) {
	k.vestingKeeper = vk
}

// SetUpgradeKeeper sets the upgrade keeper (post-init to break circular deps).
func (k *Keeper) SetUpgradeKeeper(uk types.UpgradeKeeper) {
	k.upgradeKeeper = uk
}

// GetUpgradeKeeper returns the upgrade keeper.
func (k Keeper) GetUpgradeKeeper() types.UpgradeKeeper {
	return k.upgradeKeeper
}

// NewKeeper creates a new governance keeper.
func NewKeeper(
	cdc codec.Codec,
	storeKey *storetypes.KVStoreKey,
	authority string,
	bankKeeper types.BankKeeper,
	stakingKeeper types.StakingKeeper,
) Keeper {
	return Keeper{
		cdc:           cdc,
		storeKey:      storeKey,
		authority:     authority,
		bankKeeper:    bankKeeper,
		stakingKeeper: stakingKeeper,
	}
}

// GetAuthority returns the governance module's authority address.
func (k Keeper) GetAuthority() string {
	return k.authority
}

// Logger returns a module-specific logger.
func (k Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", "x/"+types.ModuleName)
}

// storeForPrefix returns a prefixed KV store.
func (k Keeper) storeForPrefix(ctx sdk.Context, pfx []byte) prefix.Store {
	store := ctx.KVStore(k.storeKey)
	return prefix.NewStore(store, pfx)
}
