package gov

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"

	"github.com/zerone-chain/zerone/x/gov/keeper"
	"github.com/zerone-chain/zerone/x/gov/types"
)

var (
	_ module.AppModuleBasic = AppModuleBasic{}
	_ module.HasGenesis     = AppModule{}
	_ module.HasServices    = AppModule{}
	_ module.AppModule      = AppModule{}
)

// ---------- AppModuleBasic ----------

type AppModuleBasic struct{}

func (AppModuleBasic) Name() string { return types.ModuleName }

func (AppModuleBasic) RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
	types.RegisterCodec(cdc)
}

func (AppModuleBasic) RegisterInterfaces(registry codectypes.InterfaceRegistry) {
	types.RegisterInterfaces(registry)
}

func (AppModuleBasic) DefaultGenesis(cdc codec.JSONCodec) json.RawMessage {
	gs := types.DefaultGenesisState()
	bz, err := json.Marshal(gs)
	if err != nil {
		panic(err)
	}
	return bz
}

func (AppModuleBasic) ValidateGenesis(_ codec.JSONCodec, _ client.TxEncodingConfig, bz json.RawMessage) error {
	var gs types.GenesisState
	if err := json.Unmarshal(bz, &gs); err != nil {
		return fmt.Errorf("failed to unmarshal %s genesis state: %w", types.ModuleName, err)
	}
	return gs.Validate()
}

func (AppModuleBasic) RegisterGRPCGatewayRoutes(_ client.Context, _ *runtime.ServeMux) {}

// ---------- AppModule ----------

type AppModule struct {
	AppModuleBasic
	keeper keeper.Keeper
}

func NewAppModule(_ codec.Codec, k keeper.Keeper) AppModule {
	return AppModule{
		AppModuleBasic: AppModuleBasic{},
		keeper:         k,
	}
}

func (AppModule) IsOnePerModuleType() {}
func (AppModule) IsAppModule()        {}

func (am AppModule) RegisterServices(cfg module.Configurator) {
	types.RegisterMsgServer(cfg.MsgServer(), keeper.NewMsgServerImpl(am.keeper))
	types.RegisterQueryServer(cfg.QueryServer(), keeper.NewQueryServerImpl(am.keeper))

	migrator := keeper.NewMigrator(am.keeper)
	if err := cfg.RegisterMigration(types.ModuleName, 1, migrator.Migrate1to2); err != nil {
		panic(fmt.Sprintf("failed to register %s migration: %v", types.ModuleName, err))
	}
}

func (am AppModule) InitGenesis(ctx sdk.Context, _ codec.JSONCodec, data json.RawMessage) {
	var gs types.GenesisState
	if err := json.Unmarshal(data, &gs); err != nil {
		panic(fmt.Errorf("failed to unmarshal %s genesis state: %w", types.ModuleName, err))
	}
	am.keeper.InitGenesis(ctx, &gs)
}

func (am AppModule) ExportGenesis(ctx sdk.Context, _ codec.JSONCodec) json.RawMessage {
	gs := am.keeper.ExportGenesis(ctx)
	bz, err := json.Marshal(gs)
	if err != nil {
		panic(err)
	}
	return bz
}

func (AppModule) ConsensusVersion() uint64 { return 1 }

func (am AppModule) BeginBlock(ctx context.Context) error {
	am.keeper.BeginBlocker(sdk.UnwrapSDKContext(ctx))
	return nil
}

func (am AppModule) EndBlock(_ context.Context) error {
	return nil
}
