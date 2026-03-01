package keeper

import (
	"context"

	"cosmossdk.io/core/store"
	"cosmossdk.io/log"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/alignment/types"
)

// Keeper manages the alignment module's state.
type Keeper struct {
	storeService        store.KVStoreService
	cdc                 codec.BinaryCodec
	authority           string
	knowledgeKeeper     types.KnowledgeKeeper
	stakingKeeper       types.StakingKeeper
	ontologyKeeper      types.OntologyKeeper
	autopoiesisKeeper   types.AutopoiesisKeeper
	emergencyKeeper      types.EmergencyKeeper
	vestingRewardsKeeper types.VestingRewardsKeeper
	captureDefenseKeeper types.CaptureDefenseKeeper // nil-safe, set post-init
}

// NewKeeper creates a new alignment module Keeper.
func NewKeeper(
	storeService store.KVStoreService,
	cdc codec.BinaryCodec,
	authority string,
	knowledgeKeeper types.KnowledgeKeeper,
	stakingKeeper types.StakingKeeper,
	ontologyKeeper types.OntologyKeeper,
	emergencyKeeper types.EmergencyKeeper,
	vestingRewardsKeeper types.VestingRewardsKeeper,
) Keeper {
	return Keeper{
		storeService:         storeService,
		cdc:                  cdc,
		authority:            authority,
		knowledgeKeeper:      knowledgeKeeper,
		stakingKeeper:        stakingKeeper,
		ontologyKeeper:       ontologyKeeper,
		emergencyKeeper:      emergencyKeeper,
		vestingRewardsKeeper: vestingRewardsKeeper,
	}
}

// SetAutopoiesisKeeper sets the autopoiesis keeper post-init.
// This allows breaking circular dependencies during module wiring.
func (k *Keeper) SetAutopoiesisKeeper(ak types.AutopoiesisKeeper) {
	k.autopoiesisKeeper = ak
}

// SetCaptureDefenseKeeper sets the capture defense keeper post-initialization.
func (k *Keeper) SetCaptureDefenseKeeper(cdk types.CaptureDefenseKeeper) {
	k.captureDefenseKeeper = cdk
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

// Logger returns a module-scoped logger.
func (k Keeper) Logger(ctx context.Context) log.Logger {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	return sdkCtx.Logger().With("module", "x/"+types.ModuleName)
}

// GetAuthority returns the module authority address.
func (k Keeper) GetAuthority() string {
	return k.authority
}
