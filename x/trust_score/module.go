package trust_score

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
	"github.com/cosmos/cosmos-sdk/types/module"

	"github.com/zerone-chain/zerone/x/trust_score/keeper"
	"github.com/zerone-chain/zerone/x/trust_score/types"
)

var (
	_ module.AppModule      = AppModule{}
	_ module.AppModuleBasic = AppModuleBasic{}
	_ appmodule.AppModule   = AppModule{}
)

type AppModuleBasic struct{ cdc codec.Codec }

func (AppModuleBasic) Name() string                                          { return types.ModuleName }
func (AppModuleBasic) RegisterLegacyAminoCodec(cdc *codec.LegacyAmino)        { types.RegisterCodec(cdc) }
func (a AppModuleBasic) RegisterInterfaces(reg cdctypes.InterfaceRegistry)    { types.RegisterInterfaces(reg) }
func (AppModuleBasic) DefaultGenesis(_ codec.JSONCodec) json.RawMessage {
	bz, err := json.Marshal(types.DefaultGenesis())
	if err != nil {
		panic("failed to marshal default genesis: " + err.Error())
	}
	return bz
}
func (AppModuleBasic) ValidateGenesis(_ codec.JSONCodec, _ client.TxEncodingConfig, bz json.RawMessage) error {
	var g types.GenesisState
	if err := json.Unmarshal(bz, &g); err != nil {
		return fmt.Errorf("failed to unmarshal %s genesis state: %w", types.ModuleName, err)
	}
	return g.Validate()
}
func (AppModuleBasic) RegisterGRPCGatewayRoutes(_ client.Context, _ *runtime.ServeMux) {}
func (AppModuleBasic) GetTxCmd() *cobra.Command                                        { return nil }
func (AppModuleBasic) GetQueryCmd() *cobra.Command                                     { return nil }

type AppModule struct {
	AppModuleBasic
	keeper keeper.Keeper
}

func NewAppModule(cdc codec.Codec, k keeper.Keeper) AppModule {
	return AppModule{AppModuleBasic: AppModuleBasic{cdc: cdc}, keeper: k}
}

func (am AppModule) IsAppModule()             {}
func (am AppModule) IsOnePerModuleType()      {}
func (am AppModule) ConsensusVersion() uint64 { return 1 }
func (am AppModule) RegisterServices(cfg module.Configurator) {
	types.RegisterQueryServer(cfg.QueryServer(), keeper.NewQueryServerImpl(am.keeper))
}
func (am AppModule) InitGenesis(_ context.Context, _ codec.JSONCodec, _ json.RawMessage) {}
func (am AppModule) ExportGenesis(_ context.Context, _ codec.JSONCodec) json.RawMessage {
	bz, _ := json.Marshal(types.DefaultGenesis())
	return bz
}
