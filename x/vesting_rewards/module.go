package vesting_rewards

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

	"github.com/zerone-chain/zerone/x/vesting_rewards/client/cli"
	"github.com/zerone-chain/zerone/x/vesting_rewards/keeper"
	vestingsim "github.com/zerone-chain/zerone/x/vesting_rewards/simulation"
	"github.com/zerone-chain/zerone/x/vesting_rewards/types"
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
	cdc.RegisterConcrete(&types.MsgClaimVesting{}, "vesting_rewards/ClaimVesting", nil)
	cdc.RegisterConcrete(&types.MsgFalsifyVesting{}, "vesting_rewards/FalsifyVesting", nil)
}

// RegisterInterfaces registers the module's interface types.
func (a AppModuleBasic) RegisterInterfaces(reg cdctypes.InterfaceRegistry) {
	types.RegisterInterfaces(reg)
}

// DefaultGenesis returns the module's default genesis state.
func (AppModuleBasic) DefaultGenesis(cdc codec.JSONCodec) json.RawMessage {
	gs := types.DefaultGenesis()
	bz, err := json.Marshal(gs)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal default genesis: %v", err))
	}
	return bz
}

// ValidateGenesis performs genesis state validation.
func (AppModuleBasic) ValidateGenesis(cdc codec.JSONCodec, config client.TxEncodingConfig, bz json.RawMessage) error {
	var gs types.GenesisState
	if err := json.Unmarshal(bz, &gs); err != nil {
		return fmt.Errorf("failed to unmarshal genesis state: %w", err)
	}
	return (&gs).Validate()
}

// RegisterGRPCGatewayRoutes registers the gRPC Gateway routes.
// Note: generated gateway code uses grpc-gateway/v2 but SDK interface requires v1.
// REST gateway registration is deferred until the project migrates to a unified gateway version.
func (AppModuleBasic) RegisterGRPCGatewayRoutes(_ client.Context, _ *runtime.ServeMux) {}

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
	var gs types.GenesisState
	if err := json.Unmarshal(data, &gs); err != nil {
		defGs := types.DefaultGenesis()
		am.keeper.InitGenesis(ctx, defGs)
		return
	}
	am.keeper.InitGenesis(ctx, &gs)
}

// ExportGenesis exports the module's genesis state.
func (am AppModule) ExportGenesis(ctx sdk.Context, cdc codec.JSONCodec) json.RawMessage {
	gs := am.keeper.ExportGenesis(ctx)
	bz, err := json.Marshal(gs)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal genesis state: %v", err))
	}
	return bz
}

// ConsensusVersion returns the module's consensus version.
func (AppModule) ConsensusVersion() uint64 { return 1 }

// BeginBlock executes begin-block logic.
// 1. Routes transaction fees through the 4-way revenue split.
// 2. Distributes block rewards to the block producer with full revenue routing.
func (am AppModule) BeginBlock(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	height := sdkCtx.BlockHeight()

	// Skip genesis block
	if height <= 1 {
		return nil
	}

	// Route accumulated transaction fees via 4-way split.
	// Must run BEFORE x/distribution's BeginBlocker sweeps fee_collector to validators.
	if err := am.keeper.RouteFees(sdkCtx); err != nil {
		am.keeper.Logger(sdkCtx).Error("failed to route fees",
			"block", height, "error", err)
	}

	// Get block proposer from header
	proposerAddr := sdkCtx.BlockHeader().ProposerAddress
	if len(proposerAddr) == 0 {
		return nil
	}
	producer := sdk.AccAddress(proposerAddr).String()

	// Get active validator count for reward scaling
	var activeValidatorCount uint32
	if sk := am.keeper.GetStakingKeeper(); sk != nil {
		activeValidatorCount = sk.GetActiveValidatorCount(sdkCtx)
	}

	// Check if block has transactions (PoT: 0% for empty blocks)
	hasTransactions := am.keeper.GetBlockTxCount() > 0 && activeValidatorCount > 0

	dist, err := am.keeper.DistributeBlockReward(sdkCtx, producer, activeValidatorCount, hasTransactions)
	if err != nil {
		am.keeper.Logger(sdkCtx).Error("failed to distribute block reward",
			"block", height, "error", err)
	} else if hasTransactions && dist != nil {
		sdkCtx.EventManager().EmitEvent(
			sdk.NewEvent("zerone.vesting_rewards.block_reward_distributed",
				sdk.NewAttribute("block_height", fmt.Sprintf("%d", height)),
				sdk.NewAttribute("producer", producer),
				sdk.NewAttribute("total_minted", dist.TotalMinted),
				sdk.NewAttribute("producer_reward", dist.ProducerReward),
				sdk.NewAttribute("active_validators", fmt.Sprintf("%d", activeValidatorCount)),
			),
		)
	}

	return nil
}

// EndBlock executes end-block logic.
// Updates claimable amounts for all active vesting schedules.
func (am AppModule) EndBlock(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	schedules := am.keeper.GetAllActiveVestingSchedules(sdkCtx)
	for _, schedule := range schedules {
		if schedule.Status == string(types.VestingStatusActive) {
			if err := am.keeper.UpdateClaimableAmount(sdkCtx, schedule.Id); err != nil {
				am.keeper.Logger(sdkCtx).Error("failed to update claimable amount",
					"vesting_id", schedule.Id, "error", err)
			}
		}
	}

	return nil
}

// ── Simulation interface (AppModuleSimulation) ──

func (AppModule) GenerateGenesisState(simState *module.SimulationState) {}

func (AppModule) RegisterStoreDecoder(_ simtypes.StoreDecoderRegistry) {}

func (am AppModule) WeightedOperations(simState module.SimulationState) []simtypes.WeightedOperation {
	if am.simAccountK == nil || am.simBankK == nil {
		return nil
	}
	return vestingsim.WeightedOperations(
		simState.AppParams, simState.Cdc, simState.TxConfig,
		am.simAccountK, am.simBankK, am.keeper,
	)
}
