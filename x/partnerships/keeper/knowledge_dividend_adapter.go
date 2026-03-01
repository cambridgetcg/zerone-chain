package keeper

import (
	"context"

	zeroneknowledgekeeper "github.com/zerone-chain/zerone/x/knowledge/keeper"
	"github.com/zerone-chain/zerone/x/partnerships/types"
)

// KnowledgeDividendAdapter wraps the knowledge Keeper to satisfy
// the partnerships module's KnowledgeKeeper interface (R31-5).
type KnowledgeDividendAdapter struct {
	k zeroneknowledgekeeper.Keeper
}

// NewKnowledgeDividendAdapter returns an adapter bridging knowledge keeper
// to the partnerships module's KnowledgeKeeper interface.
func NewKnowledgeDividendAdapter(k zeroneknowledgekeeper.Keeper) *KnowledgeDividendAdapter {
	return &KnowledgeDividendAdapter{k: k}
}

// Ensure compile-time interface compliance.
var _ types.KnowledgeKeeper = (*KnowledgeDividendAdapter)(nil)

func (a *KnowledgeDividendAdapter) ApplyMentorshipDividend(ctx context.Context, domain, mentor, mentee string) {
	a.k.ApplyMentorshipDividend(ctx, domain, mentor, mentee)
}
