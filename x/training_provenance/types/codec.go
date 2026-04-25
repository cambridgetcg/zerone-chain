package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
)

// RegisterCodec is a no-op: this module has no msg types.
func RegisterCodec(_ *codec.LegacyAmino) {}

// RegisterInterfaces is a no-op for the same reason. The module's
// query service is registered via RegisterServices on the AppModule.
func RegisterInterfaces(_ cdctypes.InterfaceRegistry) {}
