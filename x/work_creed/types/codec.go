package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
)

// RegisterLegacyAminoCodec registers concrete types on the LegacyAmino
// codec. Phase 0 has no Msg types; this is a placeholder for Phase 1+.
func RegisterLegacyAminoCodec(_ *codec.LegacyAmino) {}

// RegisterInterfaces registers the module's interface types. Phase 0
// has no Msg types; this is a placeholder for Phase 1+.
func RegisterInterfaces(_ cdctypes.InterfaceRegistry) {}
