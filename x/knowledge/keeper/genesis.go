package keeper

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

func (k Keeper) InitGenesis(ctx context.Context, gs *types.GenesisState) error {
	if gs.Params != nil {
		if err := k.SetParams(ctx, gs.Params); err != nil {
			return err
		}
	}
	for _, domain := range gs.Domains {
		if domain == nil {
			continue
		}
		if err := k.SetDomain(ctx, domain); err != nil {
			return err
		}
	}
	return nil
}

func (k Keeper) ExportGenesis(ctx context.Context) *types.GenesisState {
	params, err := k.GetParams(ctx)
	if err != nil {
		p := types.DefaultParams()
		params = &p
	}
	var domains []*types.Domain
	k.IterateDomains(ctx, func(domain *types.Domain) bool {
		domains = append(domains, domain)
		return false
	})
	return &types.GenesisState{
		Params:  params,
		Domains: domains,
	}
}

// ─── Sharding Genesis ────────────────────────────────────────────────────────

// shardingGenesisKey is the KVStore key for persisted sharding genesis state.
var shardingGenesisKey = []byte{0xbf} // after existing sharding prefixes

// ExportShardingGenesis collects all sharding state into a ShardingGenesisState.
func (k Keeper) ExportShardingGenesis(ctx context.Context) types.ShardingGenesisState {
	gs := types.ShardingGenesisState{
		Params: k.GetShardingParams(ctx),
	}

	k.IterateShardAssignments(ctx, func(a types.ShardAssignment) bool {
		gs.Assignments = append(gs.Assignments, a)
		return false
	})

	k.IterateStorageAttestations(ctx, func(a types.StorageAttestation) bool {
		gs.Attestations = append(gs.Attestations, a)
		return false
	})

	return gs
}

// ImportShardingGenesis restores sharding state from a ShardingGenesisState.
func (k Keeper) ImportShardingGenesis(ctx context.Context, gs types.ShardingGenesisState) error {
	if err := k.SetShardingParams(ctx, gs.Params); err != nil {
		return fmt.Errorf("failed to set sharding params: %w", err)
	}

	for _, a := range gs.Assignments {
		if err := k.SetShardAssignment(ctx, a); err != nil {
			return fmt.Errorf("failed to set shard assignment for %s: %w", a.ValidatorAddr, err)
		}
	}

	for _, a := range gs.Attestations {
		if err := k.SetStorageAttestation(ctx, a); err != nil {
			return fmt.Errorf("failed to set storage attestation for %s: %w", a.ValidatorAddr, err)
		}
	}

	return nil
}

// PersistShardingGenesis stores the sharding genesis as JSON in the KVStore.
// Used for chain export compatibility.
func (k Keeper) PersistShardingGenesis(ctx context.Context, gs types.ShardingGenesisState) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(gs)
	if err != nil {
		return err
	}
	return store.Set(shardingGenesisKey, bz)
}

// LoadShardingGenesis loads the sharding genesis from the KVStore.
func (k Keeper) LoadShardingGenesis(ctx context.Context) (types.ShardingGenesisState, error) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(shardingGenesisKey)
	if err != nil || bz == nil {
		return types.DefaultShardingGenesisState(), nil
	}
	var gs types.ShardingGenesisState
	if err := json.Unmarshal(bz, &gs); err != nil {
		return types.DefaultShardingGenesisState(), err
	}
	return gs, nil
}
