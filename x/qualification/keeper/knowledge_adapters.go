package keeper

import (
	"context"

	knowledgetypes "github.com/zerone-chain/zerone/x/knowledge/types"
)

// KnowledgeDomainQualificationAdapter wraps the qualification Keeper to satisfy the
// knowledge module's DomainQualificationKeeper interface.
type KnowledgeDomainQualificationAdapter struct {
	k Keeper
}

// NewKnowledgeDomainQualificationAdapter returns an adapter that bridges the qualification keeper
// to the knowledge module's DomainQualificationKeeper interface.
func NewKnowledgeDomainQualificationAdapter(k Keeper) *KnowledgeDomainQualificationAdapter {
	return &KnowledgeDomainQualificationAdapter{k: k}
}

// Ensure compile-time interface compliance.
var _ knowledgetypes.DomainQualificationKeeper = (*KnowledgeDomainQualificationAdapter)(nil)

func (a *KnowledgeDomainQualificationAdapter) IsQualified(ctx context.Context, validatorAddr, domain string) (bool, error) {
	return a.k.IsQualified(ctx, validatorAddr, domain), nil
}

func (a *KnowledgeDomainQualificationAdapter) GetQualificationWeight(ctx context.Context, validatorAddr, domain string) (uint64, error) {
	return uint64(a.k.GetQualificationWeight(ctx, validatorAddr, domain)), nil
}

func (a *KnowledgeDomainQualificationAdapter) GetQualifiedValidators(ctx context.Context, domain string) ([]string, error) {
	return a.k.GetQualifiedValidators(ctx, domain), nil
}

func (a *KnowledgeDomainQualificationAdapter) RecordVerificationOutcome(ctx context.Context, validatorAddr, domain string, accepted bool) error {
	return a.k.RecordVerificationOutcome(ctx, validatorAddr, domain, accepted)
}
