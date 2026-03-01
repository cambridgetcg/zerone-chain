package keeper

import (
	"encoding/json"
	"fmt"

	"cosmossdk.io/core/store"
	"cosmossdk.io/log"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/bvm/types"
)

// Keeper manages the bvm module's state.
type Keeper struct {
	cdc          codec.Codec
	storeService store.KVStoreService
	bankKeeper   types.BankKeeper
	authority    string

	// Optional cross-module keepers (set via setters after init)
	knowledgeKeeper types.KnowledgeKeeper
	billingKeeper   types.BillingKeeper
	homeKeeper      types.HomeKeeper
	authKeeper      types.AuthKeeper
}

// NewKeeper creates a new bvm module Keeper.
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

// prefixEndBytes returns the end key for prefix iteration.
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

func (k Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", fmt.Sprintf("x/%s", types.ModuleName))
}

func (k Keeper) GetAuthority() string { return k.authority }

func (k *Keeper) SetKnowledgeKeeper(kk types.KnowledgeKeeper) { k.knowledgeKeeper = kk }
func (k *Keeper) SetBillingKeeper(bk types.BillingKeeper)     { k.billingKeeper = bk }
func (k *Keeper) SetHomeKeeper(hk types.HomeKeeper)           { k.homeKeeper = hk }
func (k *Keeper) SetAuthKeeper(ak types.AuthKeeper)           { k.authKeeper = ak }
func (k Keeper) GetAuthKeeper() types.AuthKeeper              { return k.authKeeper }

// InitGenesis initializes the module's state from genesis.
func (k Keeper) InitGenesis(ctx sdk.Context, gs *types.GenesisState) {
	k.SetParams(ctx, gs.Params)
	for _, contract := range gs.Contracts {
		k.SetContract(ctx, contract)
	}
	for _, code := range gs.Codes {
		k.SetCode(ctx, code)
	}
	for _, schedule := range gs.Schedules {
		k.SetSchedule(ctx, schedule)
	}
	for _, entry := range gs.State {
		k.SetContractState(ctx, entry.ContractAddress, entry.Key, entry.Value)
	}
}

// ExportGenesis exports the module's state.
func (k Keeper) ExportGenesis(ctx sdk.Context) *types.GenesisState {
	p := k.GetParams(ctx)
	gs := &types.GenesisState{
		Params:    p,
		Contracts: []*types.DeployedContract{},
		Codes:     []*types.ContractCode{},
		Schedules: []*types.ContractSchedule{},
		State:     []*types.ContractStateEntry{},
	}

	k.IterateContracts(ctx, func(c *types.DeployedContract) bool {
		gs.Contracts = append(gs.Contracts, c)
		return false
	})
	k.IterateCode(ctx, func(c *types.ContractCode) bool {
		gs.Codes = append(gs.Codes, c)
		return false
	})
	k.IterateSchedules(ctx, func(s *types.ContractSchedule) bool {
		gs.Schedules = append(gs.Schedules, s)
		return false
	})
	for _, contract := range gs.Contracts {
		k.IterateContractState(ctx, contract.Address, func(key, value string) bool {
			gs.State = append(gs.State, &types.ContractStateEntry{
				ContractAddress: contract.Address,
				Key:             key,
				Value:           value,
			})
			return false
		})
	}
	return gs
}

// ExportGenesisJSON exports the module's genesis state as JSON.
func (k Keeper) ExportGenesisJSON(ctx sdk.Context) json.RawMessage {
	gs := k.ExportGenesis(ctx)
	bz, err := json.Marshal(gs)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal bvm genesis: %v", err))
	}
	return bz
}
