package alignment

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/spf13/cobra"

	"cosmossdk.io/core/appmodule"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"

	"github.com/zerone-chain/zerone/x/alignment/client/cli"
	"github.com/zerone-chain/zerone/x/alignment/keeper"
	"github.com/zerone-chain/zerone/x/alignment/types"
)

var (
	_ module.AppModule           = AppModule{}
	_ module.AppModuleBasic      = AppModuleBasic{}
	_ module.AppModuleSimulation = AppModule{}
	_ appmodule.AppModule        = AppModule{}
)

// AppModuleBasic implements the AppModuleBasic interface.
type AppModuleBasic struct {
	cdc codec.Codec
}

func (AppModuleBasic) Name() string { return types.ModuleName }

func (AppModuleBasic) RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
	types.RegisterCodec(cdc)
}

func (a AppModuleBasic) RegisterInterfaces(reg cdctypes.InterfaceRegistry) {
	types.RegisterInterfaces(reg)
}

func (AppModuleBasic) DefaultGenesis(cdc codec.JSONCodec) json.RawMessage {
	bz, err := json.Marshal(types.DefaultGenesis())
	if err != nil {
		panic("failed to marshal default genesis: " + err.Error())
	}
	return bz
}

func (AppModuleBasic) ValidateGenesis(cdc codec.JSONCodec, config client.TxEncodingConfig, bz json.RawMessage) error {
	var genState types.GenesisState
	if err := json.Unmarshal(bz, &genState); err != nil {
		return fmt.Errorf("failed to unmarshal %s genesis state: %w", types.ModuleName, err)
	}
	return genState.Validate()
}

// RegisterGRPCGatewayRoutes — no-op, deferred to gateway v2.
func (AppModuleBasic) RegisterGRPCGatewayRoutes(_ client.Context, _ *runtime.ServeMux) {}

func (a AppModuleBasic) GetTxCmd() *cobra.Command {
	return cli.NewTxCmd()
}

func (AppModuleBasic) GetQueryCmd() *cobra.Command {
	return cli.NewQueryCmd()
}

// AppModule implements the AppModule interface.
type AppModule struct {
	AppModuleBasic
	keeper keeper.Keeper
}

// NewAppModule creates a new AppModule.
func NewAppModule(cdc codec.Codec, keeper keeper.Keeper) AppModule {
	return AppModule{
		AppModuleBasic: AppModuleBasic{cdc: cdc},
		keeper:         keeper,
	}
}

func (am AppModule) IsOnePerModuleType() {}
func (am AppModule) IsAppModule()        {}

// RegisterServices registers module services.
func (am AppModule) RegisterServices(cfg module.Configurator) {
	types.RegisterMsgServer(cfg.MsgServer(), keeper.NewMsgServerImpl(am.keeper))
	types.RegisterQueryServer(cfg.QueryServer(), keeper.NewQueryServerImpl(am.keeper))

	migrator := keeper.NewMigrator(am.keeper)
	if err := cfg.RegisterMigration(types.ModuleName, 1, migrator.Migrate1to2); err != nil {
		panic(fmt.Sprintf("failed to register %s migration: %v", types.ModuleName, err))
	}
}

// InitGenesis initializes the module from genesis.
func (am AppModule) InitGenesis(ctx sdk.Context, cdc codec.JSONCodec, data json.RawMessage) {
	var genState types.GenesisState
	if err := json.Unmarshal(data, &genState); err != nil {
		panic("failed to unmarshal genesis: " + err.Error())
	}
	am.keeper.InitGenesis(ctx, &genState)
}

// ExportGenesis exports the module's genesis state.
func (am AppModule) ExportGenesis(ctx sdk.Context, cdc codec.JSONCodec) json.RawMessage {
	genState := am.keeper.ExportGenesis(ctx)
	bz, err := json.Marshal(genState)
	if err != nil {
		panic("failed to marshal genesis: " + err.Error())
	}
	return bz
}

// ConsensusVersion returns the module's consensus version.
func (AppModule) ConsensusVersion() uint64 { return 1 }

// BeginBlock is a no-op for alignment.
func (am AppModule) BeginBlock(_ context.Context) error {
	return nil
}

// EndBlock runs observation→scoring→corrections every ObservationIntervalBlocks.
// Skips if module is disabled or chain is emergency-halted.
// When health degrades, enables 2x observation frequency (halved interval).
// When health recovers to healthy, restores normal frequency.
func (am AppModule) EndBlock(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	height := uint64(sdkCtx.BlockHeight())

	// Skip if disabled or halted.
	if !am.keeper.IsEnabled(ctx) {
		return nil
	}
	if am.keeper.IsHalted(ctx) {
		return nil
	}

	params := am.keeper.GetParams(ctx)
	if params.ObservationIntervalBlocks == 0 {
		return nil
	}

	// Compute effective interval: confidence-modulated, then halved when degraded frequency is active.
	state := am.keeper.GetState(ctx)
	effectiveInterval := am.keeper.GetEffectiveObservationInterval(ctx)
	if state.DegradedFrequencyActive && effectiveInterval > 1 {
		effectiveInterval = effectiveInterval / 2
	}

	if height%effectiveInterval != 0 {
		return nil
	}

	// 1. Observe
	obs := am.keeper.ObserveAll(ctx)
	am.keeper.SetObservation(ctx, obs)

	// 2. Score
	scores := am.keeper.ComputeScores(ctx, obs)
	am.keeper.SetScores(ctx, scores)

	// 2.5 Evaluate pending correction outcomes from previous observation (R29-4).
	am.keeper.EvaluatePendingCorrections(ctx, scores)

	// 3. Corrections
	corrections := am.keeper.GenerateCorrections(ctx, scores)
	am.keeper.ApplyCorrections(ctx, corrections)

	// 4. Health index
	category := am.keeper.CategorizeHealth(ctx, scores.Composite)
	hi := am.keeper.BuildHealthIndex(scores, category, uint32(len(corrections)))
	am.keeper.SetHealthIndex(ctx, hi)

	// 4.5 Emit correction confidence event (R29-4).
	confidence := am.keeper.GetCorrectionConfidence(ctx)
	effectiveMaxMag := am.keeper.GetEffectiveMaxMagnitude(ctx)
	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent("zerone.alignment.correction_confidence_updated",
			sdk.NewAttribute("confidence_bps", fmt.Sprintf("%d", confidence)),
			sdk.NewAttribute("effective_max_magnitude", fmt.Sprintf("%d", effectiveMaxMag)),
			sdk.NewAttribute("category", keeper.CategorizeConfidence(confidence)),
		),
	)

	// 5. Health transition responses
	// Compute pacing for event enrichment (R29-6).
	creationPacing, analysisPacing := am.keeper.GetGlobalPacingMultiplier(ctx)

	previousCategory := state.PreviousCategory
	if previousCategory != "" && previousCategory != category {
		switch {
		case category == types.CategoryDegraded && previousCategory == types.CategoryHealthy:
			state.DegradedFrequencyActive = true
			sdkCtx.EventManager().EmitEvent(
				sdk.NewEvent("zerone.alignment.network_health_degraded",
					sdk.NewAttribute("height", fmt.Sprintf("%d", height)),
					sdk.NewAttribute("composite", fmt.Sprintf("%d", scores.Composite)),
					sdk.NewAttribute("creation_multiplier_bps", fmt.Sprintf("%d", creationPacing)),
					sdk.NewAttribute("analysis_multiplier_bps", fmt.Sprintf("%d", analysisPacing)),
				),
			)
		case category == types.CategoryCritical:
			state.DegradedFrequencyActive = true
			sdkCtx.EventManager().EmitEvent(
				sdk.NewEvent("zerone.alignment.network_health_critical",
					sdk.NewAttribute("height", fmt.Sprintf("%d", height)),
					sdk.NewAttribute("composite", fmt.Sprintf("%d", scores.Composite)),
					sdk.NewAttribute("creation_multiplier_bps", fmt.Sprintf("%d", creationPacing)),
					sdk.NewAttribute("analysis_multiplier_bps", fmt.Sprintf("%d", analysisPacing)),
				),
			)
		case category == types.CategoryHealthy && (previousCategory == types.CategoryDegraded || previousCategory == types.CategoryCritical):
			state.DegradedFrequencyActive = false
			sdkCtx.EventManager().EmitEvent(
				sdk.NewEvent("zerone.alignment.network_health_recovered",
					sdk.NewAttribute("height", fmt.Sprintf("%d", height)),
					sdk.NewAttribute("composite", fmt.Sprintf("%d", scores.Composite)),
					sdk.NewAttribute("creation_multiplier_bps", fmt.Sprintf("%d", creationPacing)),
					sdk.NewAttribute("analysis_multiplier_bps", fmt.Sprintf("%d", analysisPacing)),
				),
			)
		}
	}
	state.PreviousCategory = category

	// 6. Update state
	state.LastObservationHeight = height
	state.ObservationCount++
	am.keeper.SetState(ctx, state)

	// 6.5 Prune old correction outcomes (R29-4).
	am.keeper.PruneOldOutcomes(ctx)

	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent("zerone.alignment.observation_recorded",
			sdk.NewAttribute("height", fmt.Sprintf("%d", height)),
			sdk.NewAttribute("composite_score", fmt.Sprintf("%d", scores.Composite)),
			sdk.NewAttribute("category", category),
			sdk.NewAttribute("correction_count", fmt.Sprintf("%d", len(corrections))),
			sdk.NewAttribute("observation_count", fmt.Sprintf("%d", state.ObservationCount)),
			sdk.NewAttribute("creation_multiplier_bps", fmt.Sprintf("%d", creationPacing)),
			sdk.NewAttribute("analysis_multiplier_bps", fmt.Sprintf("%d", analysisPacing)),
		),
	)

	// Emit growth pressure event if applicable (R31-1)
	am.keeper.EmitGrowthPressureEvent(sdkCtx, obs.KnowledgeQuality)

	am.keeper.Logger(ctx).Info("alignment observation complete",
		"height", height,
		"composite", scores.Composite,
		"category", category,
		"corrections", len(corrections),
	)

	return nil
}

// ── Simulation interface (AppModuleSimulation) ──

func (AppModule) GenerateGenesisState(simState *module.SimulationState) {}

func (AppModule) RegisterStoreDecoder(_ simtypes.StoreDecoderRegistry) {}

func (AppModule) WeightedOperations(_ module.SimulationState) []simtypes.WeightedOperation {
	return nil
}
