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

// GetFactDomain returns the domain of a fact by its ID.
func (a *CaptureDefenseKnowledgeAdapter) GetFactDomain(ctx context.Context, factId string) (string, bool) {
	fact, found := a.k.GetFact(ctx, factId)
	if !found {
		return "", false
	}
	return fact.Domain, true
}

// GetFactSubmitter returns the submitter of a fact by its ID.
func (a *CaptureDefenseKnowledgeAdapter) GetFactSubmitter(ctx context.Context, factId string) (string, bool) {
	fact, found := a.k.GetFact(ctx, factId)
	if !found {
		return "", false
	}
	return fact.Submitter, true
}

// GetDomainVerificationActivity returns the verification activity level for a domain (R31-4).
func (a *CaptureDefenseKnowledgeAdapter) GetDomainVerificationActivity(ctx context.Context, domain string) uint64 {
	return a.k.GetDomainVerificationActivity(ctx, domain)
}
