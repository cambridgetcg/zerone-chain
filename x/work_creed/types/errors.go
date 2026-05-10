package types

import sdkerrors "cosmossdk.io/errors"

// Phase 0 has no failure paths beyond genesis sanity. Sentinel errors
// land here in Phase 1+ when AnchorSubCreedPin / QueryPinAtVersion are
// implemented and produce typed refusals.
var (
	// ErrUnknownPhase is returned when a query references a phase
	// outside [0, 8].
	ErrUnknownPhase = sdkerrors.Register(ModuleName, 1, "unknown lifecycle phase")
)
