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

	"github.com/zerone-chain/zerone/x/alignment/client/cli"
	"github.com/zerone-chain/zerone/x/alignment/keeper"
	"github.com/zerone-chain/zerone/x/alignment/types"
)

var (
	_ module.AppModule      = AppModule{}
	_ module.AppModuleBasic = AppModuleBasic{}
	_ appmodule.AppModule   = AppModule{}
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

	// Only observe at interval boundaries.
	if height%params.ObservationIntervalBlocks != 0 {
		return nil
	}

	// 1. Observe
	obs := am.keeper.ObserveAll(ctx)
	am.keeper.SetObservation(ctx, obs)

	// 2. Score
	scores := am.keeper.ComputeScores(ctx, obs)
	am.keeper.SetScores(ctx, scores)

	// 3. Corrections
	corrections := am.keeper.GenerateCorrections(ctx, scores)
	am.keeper.ApplyCorrections(ctx, corrections)

	// 4. Health index
	category := am.keeper.CategorizeHealth(ctx, scores.Composite)
	hi := am.keeper.BuildHealthIndex(scores, category, uint32(len(corrections)))
	am.keeper.SetHealthIndex(ctx, hi)

	// 5. Update state
	state := am.keeper.GetState(ctx)
	state.LastObservationHeight = height
	state.ObservationCount++
	am.keeper.SetState(ctx, state)

	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent("zerone.alignment.observation_recorded",
			sdk.NewAttribute("height", fmt.Sprintf("%d", height)),
			sdk.NewAttribute("composite_score", fmt.Sprintf("%d", scores.Composite)),
			sdk.NewAttribute("category", category),
			sdk.NewAttribute("correction_count", fmt.Sprintf("%d", len(corrections))),
			sdk.NewAttribute("observation_count", fmt.Sprintf("%d", state.ObservationCount)),
		),
	)

	am.keeper.Logger(ctx).Info("alignment observation complete",
		"height", height,
		"composite", scores.Composite,
		"category", category,
		"corrections", len(corrections),
	)

	return nil
}
