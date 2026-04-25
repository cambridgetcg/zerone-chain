package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
)

// Read-only synthesizer; no msg types to register.
func RegisterCodec(_ *codec.LegacyAmino)            {}
func RegisterInterfaces(_ cdctypes.InterfaceRegistry) {}
