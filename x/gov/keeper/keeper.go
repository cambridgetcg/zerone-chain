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
	paramRouter    types.ParamRouter    // set post-init to break circular deps
	emergencyKeeper types.EmergencyKeeper // set post-init (circular dep break)
	alignmentKeeper    types.AlignmentKeeper    // set post-init (R31-3, Wood→Earth)
	partnershipsKeeper types.PartnershipsKeeper // set post-init (R31-3, Earth→Water)
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

// SetParamRouter sets the parameter router (post-init to break circular deps).
func (k *Keeper) SetParamRouter(pr types.ParamRouter) {
	k.paramRouter = pr
}

// GetParamRouter returns the parameter router.
func (k Keeper) GetParamRouter() types.ParamRouter {
	return k.paramRouter
}

// SetEmergencyKeeper sets the emergency keeper (post-init to break circular deps).
func (k *Keeper) SetEmergencyKeeper(ek types.EmergencyKeeper) {
	k.emergencyKeeper = ek
}

// SetAlignmentKeeper sets the alignment keeper (post-init to break circular deps).
func (k *Keeper) SetAlignmentKeeper(ak types.AlignmentKeeper) {
	k.alignmentKeeper = ak
}

// SetPartnershipsKeeper sets the partnerships keeper (post-init to break circular deps).
func (k *Keeper) SetPartnershipsKeeper(pk types.PartnershipsKeeper) {
	k.partnershipsKeeper = pk
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
