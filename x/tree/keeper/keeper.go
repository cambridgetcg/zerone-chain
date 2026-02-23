package keeper

import (
	"encoding/json"
	"fmt"

	"cosmossdk.io/core/store"
	"cosmossdk.io/log"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/tree/types"
)

// Keeper manages the tree module's state.
type Keeper struct {
	cdc          codec.Codec
	storeService store.KVStoreService

	bankKeeper            types.BankKeeper
	channelsKeeper        types.ChannelsKeeper
	researchFundDepositor types.ResearchFundDepositor

	authority string
}

// NewKeeper creates a new tree module Keeper.
func NewKeeper(
	cdc codec.Codec,
	storeService store.KVStoreService,
	bk types.BankKeeper,
	authority string,
	rfd types.ResearchFundDepositor,
) Keeper {
	if rfd == nil {
		panic("tree: ResearchFundDepositor is required")
	}
	return Keeper{
		cdc:                   cdc,
		storeService:          storeService,
		bankKeeper:            bk,
		authority:             authority,
		researchFundDepositor: rfd,
	}
}

// prefixEndBytes returns the end key for a prefix scan (exclusive).
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

// SetChannelsKeeper sets the channels keeper for channel-gated service calls.
func (k *Keeper) SetChannelsKeeper(ck types.ChannelsKeeper) {
	k.channelsKeeper = ck
}

// Logger returns a module-specific logger.
func (k Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", fmt.Sprintf("x/%s", types.ModuleName))
}

// GetAuthority returns the module authority address.
func (k Keeper) GetAuthority() string {
	return k.authority
}

// InitGenesis initializes the module's state from genesis.
func (k Keeper) InitGenesis(ctx sdk.Context, gs *types.GenesisState) {
	if gs.Params != nil {
		k.SetParams(ctx, gs.Params)
	}
	for _, p := range gs.Projects {
		if p != nil {
			k.SetProject(ctx, p)
		}
	}
	for _, t := range gs.Tasks {
		if t != nil {
			k.SetTask(ctx, t)
		}
	}
	for _, s := range gs.Services {
		if s != nil {
			k.SetService(ctx, s)
		}
	}
	for _, s := range gs.Seeds {
		if s != nil {
			k.SetSeed(ctx, s)
		}
	}
}

// ExportGenesis exports the module's state.
func (k Keeper) ExportGenesis(ctx sdk.Context) *types.GenesisState {
	params := k.GetParams(ctx)
	gs := &types.GenesisState{
		Params:   params,
		Projects: []*types.ProductProject{},
		Tasks:    []*types.ProjectTask{},
		Services: []*types.ServiceLeaf{},
		Seeds:    []*types.OpportunitySeed{},
	}

	k.IterateProjects(ctx, func(p *types.ProductProject) bool {
		gs.Projects = append(gs.Projects, p)
		return false
	})
	k.IterateTasks(ctx, func(t *types.ProjectTask) bool {
		gs.Tasks = append(gs.Tasks, t)
		return false
	})
	k.IterateServices(ctx, func(s *types.ServiceLeaf) bool {
		gs.Services = append(gs.Services, s)
		return false
	})
	k.IterateSeeds(ctx, func(s *types.OpportunitySeed) bool {
		gs.Seeds = append(gs.Seeds, s)
		return false
	})

	return gs
}

// ExportGenesisJSON exports genesis as JSON for the module interface.
func (k Keeper) ExportGenesisJSON(ctx sdk.Context) json.RawMessage {
	gs := k.ExportGenesis(ctx)
	bz, err := json.Marshal(gs)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal tree genesis: %v", err))
	}
	return bz
}
