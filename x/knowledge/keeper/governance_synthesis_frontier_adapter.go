package keeper

import (
	"context"

	govsynthtypes "github.com/zerone-chain/zerone/x/governance_synthesis/types"
	knowledgetypes "github.com/zerone-chain/zerone/x/knowledge/types"
)

// GovernanceSynthesisFrontierAdapter exposes per-domain fact counts
// for the Frontier synthesizer's sparsity computation.
type GovernanceSynthesisFrontierAdapter struct {
	k Keeper
}

func NewGovernanceSynthesisFrontierAdapter(k Keeper) *GovernanceSynthesisFrontierAdapter {
	return &GovernanceSynthesisFrontierAdapter{k: k}
}

// FactDomain returns just the domain for a fact id, or "" if not
// found. Used by the Frontier synthesizer's per-domain
// counterexample scope counter.
func (a *GovernanceSynthesisFrontierAdapter) FactDomain(ctx context.Context, factID string) string {
	f, ok := a.k.GetFact(ctx, factID)
	if !ok || f == nil {
		return ""
	}
	return f.Domain
}

// CountFactsByDomain returns the count of VERIFIED or ACTIVE facts
// in the given domain. Uses the by-domain index; counts facts in
// states that contribute to the corpus (excluding DISPROVEN,
// REVOKED, EXPIRED, CONTESTED).
func (a *GovernanceSynthesisFrontierAdapter) CountFactsByDomain(ctx context.Context, domain string) uint64 {
	count := uint64(0)
	a.k.IterateFactsByDomain(ctx, domain, func(factID string) bool {
		f, ok := a.k.GetFact(ctx, factID)
		if !ok || f == nil {
			return false
		}
		switch f.Status {
		case knowledgetypes.FactStatus_FACT_STATUS_VERIFIED,
			knowledgetypes.FactStatus_FACT_STATUS_ACTIVE:
			count++
		}
		return false
	})
	return count
}

var _ govsynthtypes.FrontierKnowledgeKeeper = (*GovernanceSynthesisFrontierAdapter)(nil)
