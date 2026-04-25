package keeper

import (
	"context"

	auitypes "github.com/zerone-chain/zerone/x/agent_understanding/types"
)

// AgentUnderstandingQualificationAdapter exposes per-(agent, domain)
// reads in the shape x/agent_understanding expects.
type AgentUnderstandingQualificationAdapter struct {
	k Keeper
}

func NewAgentUnderstandingQualificationAdapter(k Keeper) *AgentUnderstandingQualificationAdapter {
	return &AgentUnderstandingQualificationAdapter{k: k}
}

// QualifiedDomains returns the list of domains the agent has any
// qualification record in (any status).
func (a *AgentUnderstandingQualificationAdapter) QualifiedDomains(ctx context.Context, agent string) []string {
	return a.k.GetQualifiedDomains(ctx, agent)
}

// AgentDomainCalibration reads the DomainQualification record for
// (agent, domain).
func (a *AgentUnderstandingQualificationAdapter) AgentDomainCalibration(ctx context.Context, agent, domain string) (uint64, uint64, uint64, string, uint32, bool) {
	q, ok := a.k.GetQualification(ctx, agent, domain)
	if !ok || q == nil {
		return 0, 0, 0, "", 0, false
	}
	var ver, correct, acc uint64
	if q.Metrics != nil {
		ver = q.Metrics.TotalVerifications
		correct = q.Metrics.CorrectVerifications
		acc = q.Metrics.AccuracyBps
	}
	return ver, correct, acc, q.Status.String(), q.Weight, true
}

var _ auitypes.QualificationKeeper = (*AgentUnderstandingQualificationAdapter)(nil)
