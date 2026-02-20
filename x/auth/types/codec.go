package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// RegisterCodec registers the zerone_auth module's types with the amino codec.
func RegisterCodec(cdc *codec.LegacyAmino) {
	cdc.RegisterConcrete(&MsgRegisterAccount{}, "zerone_auth/RegisterAccount", nil)
	cdc.RegisterConcrete(&MsgRotateKey{}, "zerone_auth/RotateKey", nil)
	cdc.RegisterConcrete(&MsgCreateSession{}, "zerone_auth/CreateSession", nil)
	cdc.RegisterConcrete(&MsgRevokeSession{}, "zerone_auth/RevokeSession", nil)
	cdc.RegisterConcrete(&MsgFreezeAccount{}, "zerone_auth/FreezeAccount", nil)
	cdc.RegisterConcrete(&MsgUnfreezeAccount{}, "zerone_auth/UnfreezeAccount", nil)
	cdc.RegisterConcrete(&MsgSetRecoveryConfig{}, "zerone_auth/SetRecoveryConfig", nil)
	cdc.RegisterConcrete(&MsgInitiateRecovery{}, "zerone_auth/InitiateRecovery", nil)
	cdc.RegisterConcrete(&MsgSubmitRecoveryShard{}, "zerone_auth/SubmitRecoveryShard", nil)
	cdc.RegisterConcrete(&MsgChallengeRecovery{}, "zerone_auth/ChallengeRecovery", nil)
	cdc.RegisterConcrete(&MsgExecuteRecovery{}, "zerone_auth/ExecuteRecovery", nil)
	cdc.RegisterConcrete(&MsgUpdateParams{}, "zerone_auth/UpdateParams", nil)
}

// RegisterInterfaces registers the zerone_auth module's interfaces.
func RegisterInterfaces(registry cdctypes.InterfaceRegistry) {
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgRegisterAccount{},
		&MsgRotateKey{},
		&MsgCreateSession{},
		&MsgRevokeSession{},
		&MsgFreezeAccount{},
		&MsgUnfreezeAccount{},
		&MsgSetRecoveryConfig{},
		&MsgInitiateRecovery{},
		&MsgSubmitRecoveryShard{},
		&MsgChallengeRecovery{},
		&MsgExecuteRecovery{},
		&MsgUpdateParams{},
	)
}

var (
	Amino     = codec.NewLegacyAmino()
	ModuleCdc = codec.NewProtoCodec(cdctypes.NewInterfaceRegistry())
)
