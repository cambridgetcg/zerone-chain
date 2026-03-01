package keeper

import (
	"context"

	ptypes "github.com/zerone-chain/zerone/x/partnerships/types"
)

// PartnershipsCaptureDefenseAdapter wraps the capture_defense Keeper to satisfy
// partnerships' CaptureDefenseKeeper interface (R29-5).
type PartnershipsCaptureDefenseAdapter struct {
	k Keeper
}

// NewPartnershipsCaptureDefenseAdapter creates a new adapter.
func NewPartnershipsCaptureDefenseAdapter(k Keeper) *PartnershipsCaptureDefenseAdapter {
	return &PartnershipsCaptureDefenseAdapter{k: k}
}

// Ensure compile-time interface compliance.
var _ ptypes.CaptureDefenseKeeper = (*PartnershipsCaptureDefenseAdapter)(nil)

// IsDomainFlagged implements ptypes.CaptureDefenseKeeper.
func (a *PartnershipsCaptureDefenseAdapter) IsDomainFlagged(ctx context.Context, domain string) bool {
	return a.k.IsDomainFlagged(ctx, domain)
}
