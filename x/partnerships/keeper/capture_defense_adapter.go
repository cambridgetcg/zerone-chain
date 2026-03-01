package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	cdtypes "github.com/zerone-chain/zerone/x/capture_defense/types"
)

// CaptureDefensePartnershipsAdapter wraps the partnerships Keeper to satisfy
// capture_defense's PartnershipsKeeper interface (R29-5).
type CaptureDefensePartnershipsAdapter struct {
	k Keeper
}

// NewCaptureDefensePartnershipsAdapter creates a new adapter.
func NewCaptureDefensePartnershipsAdapter(k Keeper) *CaptureDefensePartnershipsAdapter {
	return &CaptureDefensePartnershipsAdapter{k: k}
}

// Ensure compile-time interface compliance.
var _ cdtypes.PartnershipsKeeper = (*CaptureDefensePartnershipsAdapter)(nil)

// GetDomainPartnershipDensity implements cdtypes.PartnershipsKeeper.
func (a *CaptureDefensePartnershipsAdapter) GetDomainPartnershipDensity(ctx context.Context, domain string) uint64 {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	return a.k.GetDomainPartnershipDensity(sdkCtx, domain)
}

// SetDomainFormationBonus implements cdtypes.PartnershipsKeeper.
func (a *CaptureDefensePartnershipsAdapter) SetDomainFormationBonus(ctx context.Context, domain string, bonusBps uint64, reason string, expiryHeight uint64) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	a.k.SetDomainFormationBonus(sdkCtx, domain, bonusBps, reason, expiryHeight)
}

// GetPartnershipCountByParticipant implements cdtypes.PartnershipsKeeper.
func (a *CaptureDefensePartnershipsAdapter) GetPartnershipCountByParticipant(ctx context.Context, addr string, domain string) uint64 {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	return a.k.GetPartnershipCountByParticipant(sdkCtx, addr, domain)
}
