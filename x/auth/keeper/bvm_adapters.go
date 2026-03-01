package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	bvmtypes "github.com/zerone-chain/zerone/x/bvm/types"
)

// BVMAuthAdapter wraps the zerone auth Keeper to satisfy
// bvmtypes.AuthKeeper interface.
type BVMAuthAdapter struct {
	k Keeper
}

// NewBVMAuthAdapter returns an adapter for the BVM module.
func NewBVMAuthAdapter(k Keeper) *BVMAuthAdapter {
	return &BVMAuthAdapter{k: k}
}

// Ensure compile-time interface compliance.
var _ bvmtypes.AuthKeeper = (*BVMAuthAdapter)(nil)

// GetAccountDID resolves a bech32 address to its DID.
func (a *BVMAuthAdapter) GetAccountDID(goCtx context.Context, address string) (string, bool) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	account, found := a.k.GetAccount(ctx, address)
	if !found || account.Did == "" {
		return "", false
	}
	return account.Did, true
}

// GetSessionCapabilities returns active session capabilities for an owner at a block height.
// If the owner has any valid (non-expired) session key at the given height, returns
// the capabilities of the first valid session key found, indicating restricted access.
// Returns (_, false) if no active session key exists, meaning the caller is using
// their identity/operational key and gets full access.
func (a *BVMAuthAdapter) GetSessionCapabilities(goCtx context.Context, owner string, blockHeight uint64) (bvmtypes.SessionCapabilities, bool) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	sessions := a.k.GetSessionKeysForOwner(ctx, owner)
	for _, session := range sessions {
		if session.ExpiresAtBlock > blockHeight && session.Capabilities != nil {
			return bvmtypes.SessionCapabilities{
				CanTransfer:     session.Capabilities.CanTransfer,
				CanStake:        session.Capabilities.CanStake,
				CanSubmitClaims: session.Capabilities.CanSubmitClaims,
				CanVote:         session.Capabilities.CanVote,
			}, true
		}
	}
	return bvmtypes.SessionCapabilities{}, false
}
