package keeper

import (
	"context"

	cdtypes "github.com/zerone-chain/zerone/x/capture_defense/types"
)

// CaptureDefenseKnowledgeAdapter wraps the knowledge Keeper to satisfy
// cdtypes.KnowledgeKeeper interface.
type CaptureDefenseKnowledgeAdapter struct {
	k Keeper
}

// NewCaptureDefenseKnowledgeAdapter returns an adapter that bridges the knowledge keeper
// to the capture_defense module's expected interface.
func NewCaptureDefenseKnowledgeAdapter(k Keeper) *CaptureDefenseKnowledgeAdapter {
	return &CaptureDefenseKnowledgeAdapter{k: k}
}

// Ensure compile-time interface compliance.
var _ cdtypes.KnowledgeKeeper = (*CaptureDefenseKnowledgeAdapter)(nil)

// GetFactDomain returns empty (facts removed in training data protocol).
func (a *CaptureDefenseKnowledgeAdapter) GetFactDomain(_ context.Context, _ string) (string, bool) {
	return "", false
}

// GetFactSubmitter returns empty (facts removed in training data protocol).
func (a *CaptureDefenseKnowledgeAdapter) GetFactSubmitter(_ context.Context, _ string) (string, bool) {
	return "", false
}

// GetDomainVerificationActivity returns 0 (verification activity tracking removed).
func (a *CaptureDefenseKnowledgeAdapter) GetDomainVerificationActivity(_ context.Context, _ string) uint64 {
	return 0
}
