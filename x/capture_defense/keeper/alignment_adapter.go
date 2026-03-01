package keeper

import (
	"context"

	alignmenttypes "github.com/zerone-chain/zerone/x/alignment/types"
)

// AlignmentCaptureDefenseAdapter wraps the capture_defense Keeper to satisfy
// the alignment module's CaptureDefenseKeeper interface.
type AlignmentCaptureDefenseAdapter struct {
	k Keeper
}

// NewAlignmentCaptureDefenseAdapter creates a new adapter.
func NewAlignmentCaptureDefenseAdapter(k Keeper) *AlignmentCaptureDefenseAdapter {
	return &AlignmentCaptureDefenseAdapter{k: k}
}

// Ensure compile-time interface compliance.
var _ alignmenttypes.CaptureDefenseKeeper = (*AlignmentCaptureDefenseAdapter)(nil)

// GetFlaggedDomainCount implements alignment types.CaptureDefenseKeeper.
func (a *AlignmentCaptureDefenseAdapter) GetFlaggedDomainCount(ctx context.Context) uint64 {
	return a.k.GetFlaggedDomainCount(ctx)
}
