package keeper

import (
	"context"

	"cosmossdk.io/core/store"
	"cosmossdk.io/log"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/capture_defense/types"
)

// Keeper manages the capture_defense module's state.
type Keeper struct {
	storeService    store.KVStoreService
	cdc             codec.BinaryCodec
	authority       string
	knowledgeKeeper types.KnowledgeKeeper
	stakingKeeper   types.StakingKeeper
	ontologyKeeper  types.OntologyKeeper // nil-safe, set post-init
}

// NewKeeper creates a new capture_defense module Keeper.
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

// Logger returns a module-scoped logger.
func (k Keeper) Logger(ctx context.Context) log.Logger {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	return sdkCtx.Logger().With("module", "x/"+types.ModuleName)
}

// GetAuthority returns the module authority address.
func (k Keeper) GetAuthority() string { return k.authority }

// SetKnowledgeKeeper sets the knowledge keeper post-initialization.
func (k *Keeper) SetKnowledgeKeeper(kk types.KnowledgeKeeper) { k.knowledgeKeeper = kk }

// SetStakingKeeper sets the staking keeper post-initialization.
func (k *Keeper) SetStakingKeeper(sk types.StakingKeeper) { k.stakingKeeper = sk }

// SetOntologyKeeper sets the ontology keeper post-initialization.
func (k *Keeper) SetOntologyKeeper(ok types.OntologyKeeper) { k.ontologyKeeper = ok }

// InitGenesis sets initial state from genesis.
func (k Keeper) InitGenesis(ctx sdk.Context, gs *types.GenesisState) {
	k.SetParams(ctx, gs.Params)
	for _, r := range gs.GlobalReputations {
		k.SetGlobalReputation(ctx, r)
	}
	for _, r := range gs.StratumReputations {
		k.SetStratumReputation(ctx, r)
	}
	for _, r := range gs.DomainReputations {
		k.SetDomainReputation(ctx, r)
	}
	for _, m := range gs.CaptureMetrics {
		k.SetCaptureMetrics(ctx, m)
	}
	for _, c := range gs.CrossStratumRequirements {
		k.SetCrossStratumRequirement(ctx, c)
	}
}

// ExportGenesis exports the current state.
func (k Keeper) ExportGenesis(ctx sdk.Context) *types.GenesisState {
	return &types.GenesisState{
		Params:                   k.GetParams(ctx),
		GlobalReputations:        k.GetAllGlobalReputations(ctx),
		StratumReputations:       k.GetAllStratumReputations(ctx),
		DomainReputations:        k.GetAllDomainReputations(ctx),
		CaptureMetrics:           k.GetAllCaptureMetrics(ctx),
		CrossStratumRequirements: k.GetAllCrossStratumRequirements(ctx),
	}
}

// BeginBlocker runs reputation decay and auto-analysis.
func (k Keeper) BeginBlocker(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params := k.GetParams(sdkCtx)

	height := uint64(sdkCtx.BlockHeight())

	// Periodic decay
	if height > 0 && params.DecayEpochBlocks > 0 && height%params.DecayEpochBlocks == 0 {
		k.DecayAllReputations(sdkCtx, params)
	}

	// Auto risk analysis
	if height > 0 && params.RiskAnalysisInterval > 0 && height%params.RiskAnalysisInterval == 0 {
		k.RunAutoAnalysis(sdkCtx, params)
	}

	return nil
}

// RunAutoAnalysis runs capture detection on all domains with recent history.
func (k Keeper) RunAutoAnalysis(ctx sdk.Context, params *types.Params) {
	domains := k.GetDomainsWithHistory(ctx)
	for _, domain := range domains {
		k.AnalyzeCaptureRisk(ctx, domain, params)
	}
}
