package keeper

import (
	"context"

	cctypes "github.com/zerone-chain/zerone/x/capture_challenge/types"
)

// ChallengeCaptureDefenseAdapter wraps capture_defense Keeper to satisfy
// capture_challenge's CaptureDefenseKeeper interface.
type ChallengeCaptureDefenseAdapter struct {
	k Keeper
}

// NewChallengeCaptureDefenseAdapter creates a new adapter.
func NewChallengeCaptureDefenseAdapter(k Keeper) *ChallengeCaptureDefenseAdapter {
	return &ChallengeCaptureDefenseAdapter{k: k}
}

// Ensure compile-time interface compliance.
var _ cctypes.CaptureDefenseKeeper = (*ChallengeCaptureDefenseAdapter)(nil)

// GetCaptureMetrics implements capture_challenge types.CaptureDefenseKeeper.
func (a *ChallengeCaptureDefenseAdapter) GetCaptureMetrics(ctx context.Context, domain string) (*cctypes.CaptureMetricsData, bool) {
	m, found := a.k.GetCaptureMetrics(ctx, domain)
	if !found {
		return nil, false
	}
	return &cctypes.CaptureMetricsData{
		Domain:              m.Domain,
		HerfindahlIndex:     m.HerfindahlIndex,
		TimingCorrelation:   m.TimingCorrelation,
		VerdictCorrelation:  m.VerdictCorrelation,
		Top3Share:           m.Top3Share,
		RiskScore:           m.RiskScore,
		TotalParticipations: m.TotalParticipations,
		AnalyzedAtBlock:     m.AnalyzedAtBlock,
		Flagged:             m.Flagged,
	}, true
}

// ClearCaptureFlag implements capture_challenge types.CaptureDefenseKeeper.
func (a *ChallengeCaptureDefenseAdapter) ClearCaptureFlag(ctx context.Context, domain string) {
	a.k.ClearCaptureFlag(ctx, domain)
}
