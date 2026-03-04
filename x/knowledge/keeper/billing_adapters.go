package keeper

import (
	"context"
	"fmt"

	billingtypes "github.com/zerone-chain/zerone/x/billing/types"
)

// BillingKnowledgeAdapter wraps the knowledge Keeper to satisfy
// billingtypes.KnowledgeKeeper interface.
type BillingKnowledgeAdapter struct {
	k Keeper
}

// NewBillingKnowledgeAdapter returns an adapter that bridges the knowledge keeper
// to the billing module's expected interface.
func NewBillingKnowledgeAdapter(k Keeper) *BillingKnowledgeAdapter {
	return &BillingKnowledgeAdapter{k: k}
}

// Ensure compile-time interface compliance.
var _ billingtypes.KnowledgeKeeper = (*BillingKnowledgeAdapter)(nil)

func (a *BillingKnowledgeAdapter) GetFactConfidence(_ context.Context, _ string) (uint64, bool) {
	return 0, false // Facts removed in training data protocol
}

func (a *BillingKnowledgeAdapter) GetFactCitationCount(_ context.Context, _ string) (uint64, bool) {
	return 0, false
}

func (a *BillingKnowledgeAdapter) GetFactSubmitter(_ context.Context, _ string) (string, bool) {
	return "", false
}

func (a *BillingKnowledgeAdapter) GetFactCreatedBlock(_ context.Context, _ string) (uint64, bool) {
	return 0, false
}

func (a *BillingKnowledgeAdapter) IncrementCitationCount(_ context.Context, factId string) error {
	return fmt.Errorf("fact not found: %s (training data protocol)", factId)
}
