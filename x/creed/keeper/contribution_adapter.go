package keeper

import "context"

// GetCurrentPinVersion returns the latest pinned creed version, or 0 if
// no pin has been recorded. Implements the
// x/contribution/adapter/knowledgeclaim.CreedKeeperReader interface so
// x/contribution can read the truth-floor reference version directly.
//
// Thin alias for GetCurrentVersion preserving the contribution-side
// vocabulary ("PinVersion") used by the truth-floor attestation.
func (k Keeper) GetCurrentPinVersion(ctx context.Context) uint32 {
	return k.GetCurrentVersion(ctx)
}
