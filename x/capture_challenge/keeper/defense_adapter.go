package keeper

import (
	"context"
)

// CaptureDefenseAutoChallenger wraps the capture_challenge Keeper to satisfy
// the capture_defense module's CaptureChallengeKeeper interface.
type CaptureDefenseAutoChallenger struct {
	k Keeper
}

// NewCaptureDefenseAutoChallenger creates a new adapter.
func NewCaptureDefenseAutoChallenger(k Keeper) *CaptureDefenseAutoChallenger {
	return &CaptureDefenseAutoChallenger{k: k}
}

// AutoSubmitChallenge implements capture_defense types.CaptureChallengeKeeper.
func (a *CaptureDefenseAutoChallenger) AutoSubmitChallenge(ctx context.Context, domain string, riskScore, hhi uint64, evidence string) error {
	return a.k.AutoSubmitChallenge(ctx, domain, riskScore, hhi, evidence)
}
