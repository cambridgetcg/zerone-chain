package keeper

import (
	"context"

	auitypes "github.com/zerone-chain/zerone/x/agent_understanding/types"
	knowledgetypes "github.com/zerone-chain/zerone/x/knowledge/types"
)

// AgentUnderstandingKnowledgeAdapter exposes the narrow read surface
// x/agent_understanding needs from x/knowledge:
//   - facts authored by an agent (with domain)
//   - fact's domain by id
//   - per-domain verified-fact count (for sparsity classification)
type AgentUnderstandingKnowledgeAdapter struct {
	k Keeper
}

func NewAgentUnderstandingKnowledgeAdapter(k Keeper) *AgentUnderstandingKnowledgeAdapter {
	return &AgentUnderstandingKnowledgeAdapter{k: k}
}

// FactsBySubmitter returns id+domain for every fact authored by the
// given agent. Walks the IterateFactsBySubmitter index.
func (a *AgentUnderstandingKnowledgeAdapter) FactsBySubmitter(ctx context.Context, submitter string) []auitypes.FactRef {
	out := []auitypes.FactRef{}
	a.k.IterateFactsBySubmitter(ctx, submitter, func(factID string) bool {
		f, ok := a.k.GetFact(ctx, factID)
		if !ok || f == nil {
			return false
		}
		// Only count facts that made it to verified status.
		switch f.Status {
		case knowledgetypes.FactStatus_FACT_STATUS_VERIFIED,
			knowledgetypes.FactStatus_FACT_STATUS_ACTIVE,
			knowledgetypes.FactStatus_FACT_STATUS_CONTESTED,
			knowledgetypes.FactStatus_FACT_STATUS_CHALLENGED:
		default:
			return false
		}
		out = append(out, auitypes.FactRef{ID: f.Id, Domain: f.Domain})
		return false
	})
	return out
}

// FactDomain returns just the domain for a fact id, or "" if not found.
func (a *AgentUnderstandingKnowledgeAdapter) FactDomain(ctx context.Context, factID string) string {
	f, ok := a.k.GetFact(ctx, factID)
	if !ok || f == nil {
		return ""
	}
	return f.Domain
}

// CountFactsByDomain returns the total verified-fact count for a
// domain. Walks the by-domain index.
func (a *AgentUnderstandingKnowledgeAdapter) CountFactsByDomain(ctx context.Context, domain string) uint64 {
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

var _ auitypes.KnowledgeKeeper = (*AgentUnderstandingKnowledgeAdapter)(nil)
