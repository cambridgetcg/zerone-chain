package keeper

import (
	"fmt"

	"cosmossdk.io/core/store"
	"cosmossdk.io/log"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/protobuf/proto"

	"github.com/zerone-chain/zerone/x/partnerships/types"
)

// Keeper manages the partnerships module's state.
type Keeper struct {
	cdc          codec.Codec
	storeService store.KVStoreService

	bankKeeper types.BankKeeper

	// Cross-module keepers (set via setter to avoid circular deps)
	homeKeeper           types.HomeKeeper
	zeroneAuthKeeper     types.ZeroneAuthKeeper     // nil until R28-5
	captureDefenseKeeper types.CaptureDefenseKeeper // nil until R29-5
	pacingKeeper         types.PacingKeeper         // nil-safe, R29-6

	authority string
}

// NewKeeper creates a new partnerships module Keeper.
func NewKeeper(
	cdc codec.Codec,
	storeService store.KVStoreService,
	bk types.BankKeeper,
	authority string,
) Keeper {
	return Keeper{
		cdc:          cdc,
		storeService: storeService,
		bankKeeper:   bk,
		authority:    authority,
	}
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

// Logger returns a module-specific logger.
func (k Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", fmt.Sprintf("x/%s", types.ModuleName))
}

// SetHomeKeeper sets the home module keeper reference.
func (k *Keeper) SetHomeKeeper(hk types.HomeKeeper) {
	k.homeKeeper = hk
}

// SetZeroneAuthKeeper sets the zerone auth keeper (post-init, R28-5).
func (k *Keeper) SetZeroneAuthKeeper(ak types.ZeroneAuthKeeper) {
	k.zeroneAuthKeeper = ak
}

// SetCaptureDefenseKeeper sets the capture defense keeper (post-init, R29-5).
func (k *Keeper) SetCaptureDefenseKeeper(ck types.CaptureDefenseKeeper) {
	k.captureDefenseKeeper = ck
}

// SetPacingKeeper sets the pacing keeper for adaptive formation timing (post-init, R29-6).
func (k *Keeper) SetPacingKeeper(pk types.PacingKeeper) {
	k.pacingKeeper = pk
}

// GetAuthority returns the module authority address.
func (k Keeper) GetAuthority() string {
	return k.authority
}

// ---------- Params ----------

// SetParams sets module parameters.
func (k Keeper) SetParams(ctx sdk.Context, params *types.Params) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := proto.Marshal(params)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal params: %v", err))
	}
	_ = kvStore.Set(types.ParamsKey, bz)
}

// GetParams returns module parameters.
func (k Keeper) GetParams(ctx sdk.Context) *types.Params {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.ParamsKey)
	if err != nil || bz == nil {
		return types.DefaultParams()
	}
	var params types.Params
	if err := proto.Unmarshal(bz, &params); err != nil {
		return types.DefaultParams()
	}
	return &params
}

// ---------- Formation Expiry ----------

// ExpireFormations dissolves partnerships whose formation window has passed.
func (k Keeper) ExpireFormations(ctx sdk.Context) {
	currentBlock := uint64(ctx.BlockHeight())
	kvStore := k.storeService.OpenKVStore(ctx)

	iter, err := kvStore.Iterator(types.FormationKeyPrefix, prefixEndBytes(types.FormationKeyPrefix))
	if err != nil {
		return
	}

	var toDelete [][]byte
	for ; iter.Valid(); iter.Next() {
		var expiry uint64
		if _, err := fmt.Sscanf(string(iter.Value()), "%d", &expiry); err != nil {
			continue
		}
		if currentBlock > expiry {
			key := iter.Key()
			partnershipId := string(key[len(types.FormationKeyPrefix):])
			if p, found := k.GetPartnership(ctx, partnershipId); found {
				if p.Status == types.StatusPending {
					p.Status = types.StatusDissolved
					k.SetPartnership(ctx, p)
				}
			}
			toDelete = append(toDelete, append([]byte{}, key...))
		}
	}
	iter.Close()
	for _, key := range toDelete {
		_ = kvStore.Delete(key)
	}
}

// ---------- Genesis ----------

// InitGenesis initializes the module's state from genesis.
func (k Keeper) InitGenesis(ctx sdk.Context, genState *types.GenesisState) {
	if genState.Params != nil {
		k.SetParams(ctx, genState.Params)
	}

	for _, p := range genState.Partnerships {
		if p != nil {
			k.SetPartnership(ctx, p)
		}
	}
	for _, op := range genState.ConsensusOperations {
		if op != nil {
			k.SetConsensusOperation(ctx, op)
		}
	}
	for _, sf := range genState.SafetyFreezes {
		if sf != nil {
			k.SetSafetyFreeze(ctx, sf)
		}
	}
	for _, cs := range genState.CoercionSignals {
		if cs != nil {
			k.SetCoercionSignal(ctx, cs)
		}
	}
	for _, sp := range genState.SeedPartnerships {
		if sp != nil {
			k.SetSeedPartnership(ctx, sp)
		}
	}
	for _, pe := range genState.PoolEntries {
		if pe != nil {
			k.SetPoolEntry(ctx, pe)
		}
	}
	for _, m := range genState.Mentorships {
		if m != nil {
			k.SetMentorship(ctx, m)
		}
	}
	for _, fm := range genState.FormationMatches {
		if fm != nil {
			k.SetFormationMatch(ctx, fm)
		}
	}
}

// ExportGenesis exports the module's state.
func (k Keeper) ExportGenesis(ctx sdk.Context) *types.GenesisState {
	params := k.GetParams(ctx)
	return &types.GenesisState{
		Params:              params,
		Partnerships:        k.GetAllPartnerships(ctx),
		ConsensusOperations: k.GetAllConsensusOperations(ctx),
		SafetyFreezes:       k.GetAllSafetyFreezes(ctx),
		CoercionSignals:     k.GetAllCoercionSignals(ctx),
		SeedPartnerships:    k.GetAllSeedPartnerships(ctx),
		PoolEntries:         k.GetAllPoolEntries(ctx),
		Mentorships:         k.GetAllMentorships(ctx),
		FormationMatches:    k.GetAllFormationMatches(ctx),
	}
}
