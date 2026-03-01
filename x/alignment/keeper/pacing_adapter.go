package keeper

import "context"

// AlignmentPacingAdapter wraps the alignment Keeper to satisfy PacingKeeper
// interfaces in consuming modules (knowledge, capture_defense, partnerships, discovery).
type AlignmentPacingAdapter struct {
	keeper Keeper
}

// NewAlignmentPacingAdapter creates a new pacing adapter.
func NewAlignmentPacingAdapter(k Keeper) *AlignmentPacingAdapter {
	return &AlignmentPacingAdapter{keeper: k}
}

// GetGlobalPacingMultiplier delegates to the alignment keeper.
func (a *AlignmentPacingAdapter) GetGlobalPacingMultiplier(ctx context.Context) (creationBps, analysisBps uint64) {
	return a.keeper.GetGlobalPacingMultiplier(ctx)
}
