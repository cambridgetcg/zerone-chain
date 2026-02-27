package keeper

import (
	"context"

	"cosmossdk.io/core/store"
	"cosmossdk.io/log"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// Keeper holds module state for the knowledge module.
type Keeper struct {
	storeService store.KVStoreService
	cdc          codec.BinaryCodec
	authority    string // governance authority address

	// External keeper dependencies (core — set at construction)
	bankKeeper    types.BankKeeper
	stakingKeeper types.StakingKeeper

	// External keeper dependencies (post-init setters to break circular deps)
	ontologyKeeper            types.OntologyKeeper
	vestingRewardsKeeper      types.VestingRewardsKeeper
	domainQualificationKeeper types.DomainQualificationKeeper // nil until R6-5
	autopoiesisKeeper         types.AutopoiesisKeeper         // nil until R7-1
	partnershipKeeper         types.PartnershipKeeper         // nil until R26-4
}

// NewKeeper creates a new knowledge Keeper.
func NewKeeper(
	storeService store.KVStoreService,
	cdc codec.BinaryCodec,
	authority string,
	bankKeeper types.BankKeeper,
	stakingKeeper types.StakingKeeper,
) Keeper {
	return Keeper{
		storeService:  storeService,
		cdc:           cdc,
		authority:     authority,
		bankKeeper:    bankKeeper,
		stakingKeeper: stakingKeeper,
	}
}

// GetAuthority returns the module's governance authority address.
func (k Keeper) GetAuthority() string {
	return k.authority
}

// GetStakingKeeper returns the staking keeper dependency.
func (k Keeper) GetStakingKeeper() types.StakingKeeper {
	return k.stakingKeeper
}

// SetOntologyKeeper sets the ontology keeper (post-init to break circular dep).
func (k *Keeper) SetOntologyKeeper(ok types.OntologyKeeper) {
	k.ontologyKeeper = ok
}

// SetVestingRewardsKeeper sets the vesting rewards keeper (post-init).
func (k *Keeper) SetVestingRewardsKeeper(vk types.VestingRewardsKeeper) {
	k.vestingRewardsKeeper = vk
}

// SetDomainQualificationKeeper sets the domain qualification keeper (post-init, R6-5).
func (k *Keeper) SetDomainQualificationKeeper(dk types.DomainQualificationKeeper) {
	k.domainQualificationKeeper = dk
}

// SetAutopoiesisKeeper sets the autopoiesis keeper (post-init, R7-1).
func (k *Keeper) SetAutopoiesisKeeper(ak types.AutopoiesisKeeper) {
	k.autopoiesisKeeper = ak
}

// SetPartnershipKeeper sets the partnership keeper (post-init, R26-4).
func (k *Keeper) SetPartnershipKeeper(pk types.PartnershipKeeper) {
	k.partnershipKeeper = pk
}

// Logger returns a module-scoped logger.
func (k Keeper) Logger(ctx context.Context) log.Logger {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	return sdkCtx.Logger().With("module", "x/"+types.ModuleName)
}
