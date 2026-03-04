package partnerships

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

	"github.com/zerone-chain/zerone/x/partnerships/client/cli"
	"github.com/zerone-chain/zerone/x/partnerships/keeper"
	partnershipssim "github.com/zerone-chain/zerone/x/partnerships/simulation"
	"github.com/zerone-chain/zerone/x/partnerships/types"
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

// Name returns the module name.
func (AppModuleBasic) Name() string { return types.ModuleName }

// RegisterLegacyAminoCodec registers the module's amino codec.
func (AppModuleBasic) RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
	types.RegisterCodec(cdc)
}

// RegisterInterfaces registers the module's interface types.
func (a AppModuleBasic) RegisterInterfaces(reg cdctypes.InterfaceRegistry) {
	types.RegisterInterfaces(reg)
}

// DefaultGenesis returns the module's default genesis state.
func (AppModuleBasic) DefaultGenesis(cdc codec.JSONCodec) json.RawMessage {
	bz, err := json.Marshal(types.DefaultGenesis())
	if err != nil {
		panic("failed to marshal default genesis: " + err.Error())
	}
	return bz
}

// ValidateGenesis performs genesis state validation.
func (AppModuleBasic) ValidateGenesis(cdc codec.JSONCodec, config client.TxEncodingConfig, bz json.RawMessage) error {
	var genState types.GenesisState
	if err := json.Unmarshal(bz, &genState); err != nil {
		return fmt.Errorf("failed to unmarshal %s genesis state: %w", types.ModuleName, err)
	}
	return genState.Validate()
}

// RegisterGRPCGatewayRoutes registers the gRPC Gateway routes.
func (AppModuleBasic) RegisterGRPCGatewayRoutes(_ client.Context, _ *runtime.ServeMux) {
	// Gateway handler generated against grpc-gateway/v2; SDK uses v1.
	// gRPC queries still work via grpc-gateway auto-registration in RegisterServices.
}

// GetTxCmd returns the module's tx CLI command.
func (a AppModuleBasic) GetTxCmd() *cobra.Command {
	return cli.NewTxCmd()
}

// GetQueryCmd returns the module's query CLI command.
func (AppModuleBasic) GetQueryCmd() *cobra.Command {
	return cli.NewQueryCmd()
}

// SimAccountKeeper is the minimal account keeper interface needed by simulation.
type SimAccountKeeper interface {
	GetAccount(ctx context.Context, addr sdk.AccAddress) sdk.AccountI
}

// SimBankKeeper is the minimal bank keeper interface needed by simulation.
type SimBankKeeper interface {
	SpendableCoins(ctx context.Context, addr sdk.AccAddress) sdk.Coins
}

// AppModule implements the AppModule interface.
type AppModule struct {
	AppModuleBasic

	keeper      keeper.Keeper
	simAccountK SimAccountKeeper
	simBankK    SimBankKeeper
}

// NewAppModule creates a new AppModule.
func NewAppModule(cdc codec.Codec, keeper keeper.Keeper) AppModule {
	return AppModule{
		AppModuleBasic: AppModuleBasic{cdc: cdc},
		keeper:         keeper,
	}
}

// SetSimulationKeepers sets the account/bank keepers needed for simulation.
func (am *AppModule) SetSimulationKeepers(ak SimAccountKeeper, bk SimBankKeeper) {
	am.simAccountK = ak
	am.simBankK = bk
}

// IsOnePerModuleType implements the depinject.OnePerModuleType interface.
func (am AppModule) IsOnePerModuleType() {}

// IsAppModule implements the appmodule.AppModule interface.
func (am AppModule) IsAppModule() {}

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

// BeginBlock executes begin-block logic.
//
//   - Expire formation windows that have passed
//   - Lift expired safety freezes and restore partnership status
//   - Expire coercion signals past their review period
//   - Expire consensus operations past their deliberation window
//   - Expire pool entries past their TTL
//   - Expire seed partnerships past their duration
//   - Expire formation matches past their acceptance window
func (am AppModule) BeginBlock(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	am.keeper.ExpireFormations(sdkCtx)
	am.keeper.LiftExpiredFreezes(sdkCtx)
	am.keeper.ExpireCoercionSignals(sdkCtx)
	am.keeper.ExpireConsensusOps(sdkCtx)
	am.keeper.ExpirePoolEntries(sdkCtx)
	am.keeper.ExpireSeedPartnerships(sdkCtx)
	am.keeper.ExpireFormationMatches(sdkCtx)
	am.keeper.ExpireFormationBonuses(sdkCtx) // R29-5: expire structural immunity bonuses
	am.keeper.ExpireFormationFreezes(sdkCtx) // R31-3: expire domain formation freezes

	return nil
}

// EndBlock executes end-block logic.
//
//   - Settle cooling partnerships that have completed their cooling period
//   - Auto-graduate mentorships that have reached their duration
//   - Run formation matching engine at configured intervals
func (am AppModule) EndBlock(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	am.keeper.SettleCoolingPartnerships(sdkCtx)
	am.keeper.AutoGraduateMentorships(sdkCtx)
	am.keeper.RunFormationMatching(sdkCtx)
	return nil
}

// ── Simulation interface (AppModuleSimulation) ──

func (AppModule) GenerateGenesisState(simState *module.SimulationState) {}

func (AppModule) RegisterStoreDecoder(_ simtypes.StoreDecoderRegistry) {}

func (am AppModule) WeightedOperations(simState module.SimulationState) []simtypes.WeightedOperation {
	if am.simAccountK == nil || am.simBankK == nil {
		return nil
	}
	return partnershipssim.WeightedOperations(
		simState.AppParams, simState.Cdc, simState.TxConfig,
		am.simAccountK, am.simBankK, am.keeper,
	)
}
