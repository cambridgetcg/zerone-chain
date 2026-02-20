package keeper

import (
	"fmt"

	"cosmossdk.io/core/store"
	"cosmossdk.io/log"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/ontology/types"
)

// Keeper manages the ontology module's state.
type Keeper struct {
	cdc          codec.Codec
	storeService store.KVStoreService

	bankKeeper types.BankKeeper

	// Module authority (typically governance module address)
	authority string
}

// NewKeeper creates a new ontology module Keeper.
func NewKeeper(
	cdc codec.Codec,
	storeService store.KVStoreService,
	bankKeeper types.BankKeeper,
	authority string,
) Keeper {
	return Keeper{
		cdc:          cdc,
		storeService: storeService,
		bankKeeper:   bankKeeper,
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

// Logger returns a module-specific logger.
func (k Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", fmt.Sprintf("x/%s", types.ModuleName))
}

// GetAuthority returns the module authority address.
func (k Keeper) GetAuthority() string {
	return k.authority
}

// InitGenesis initializes the module's state from genesis.
func (k Keeper) InitGenesis(ctx sdk.Context, genState *types.GenesisState) {
	if genState.Params != nil {
		k.SetParams(ctx, genState.Params)
	} else {
		p := types.DefaultParams()
		k.SetParams(ctx, &p)
	}

	// Initialize strata
	for _, stratum := range genState.Strata {
		if stratum != nil {
			k.SetStratum(ctx, stratum)
		}
	}

	// Initialize domains
	for _, domain := range genState.Domains {
		if domain != nil {
			k.SetDomain(ctx, domain)
		}
	}

	// Initialize proposals (if any from export)
	for _, proposal := range genState.Proposals {
		if proposal != nil {
			k.SetProposal(ctx, proposal)
		}
	}

	// Initialize cross-stratum links
	for _, link := range genState.CrossLinks {
		if link != nil {
			k.SetLink(ctx, link)
		}
	}

	// Pre-populate default logic zones (won't overwrite genesis-provided ones)
	k.InitializeDefaultZones(ctx)
}

// ExportGenesis exports the module's state.
func (k Keeper) ExportGenesis(ctx sdk.Context) *types.GenesisState {
	params := k.GetParams(ctx)
	return &types.GenesisState{
		Params:     params,
		Strata:     k.GetAllStrata(ctx),
		Domains:    k.GetAllDomains(ctx),
		Proposals:  k.getAllProposals(ctx),
		CrossLinks: k.GetAllLinks(ctx),
	}
}

// getAllProposals returns all proposals (used in ExportGenesis).
func (k Keeper) getAllProposals(ctx sdk.Context) []*types.DomainProposal {
	var proposals []*types.DomainProposal
	k.IterateProposals(ctx, func(p *types.DomainProposal) bool {
		proposals = append(proposals, p)
		return false
	})
	return proposals
}
