package keeper

import (
	"context"

	auitypes "github.com/zerone-chain/zerone/x/agent_understanding/types"
	"github.com/zerone-chain/zerone/x/inquiry/types"
)

// AgentUnderstandingAdapter aggregates inquiry-answer stats by agent
// for x/agent_understanding.
type AgentUnderstandingAdapter struct {
	k Keeper
}

func NewAgentUnderstandingAdapter(k Keeper) *AgentUnderstandingAdapter {
	return &AgentUnderstandingAdapter{k: k}
}

// InquiryAnswersByAgent walks all inquiries (and their answers) to
// tally how many the agent answered and how many they won. O(N over
// inquiries × answers per inquiry). Acceptable for testnet.
func (a *AgentUnderstandingAdapter) InquiryAnswersByAgent(ctx context.Context, agent string) (uint64, uint64) {
	return a.tally(ctx, agent, "")
}

// InquiryAnswersByAgentAndDomain scopes the same tally to a single
// domain.
func (a *AgentUnderstandingAdapter) InquiryAnswersByAgentAndDomain(ctx context.Context, agent, domain string) (uint64, uint64) {
	return a.tally(ctx, agent, domain)
}

func (a *AgentUnderstandingAdapter) tally(ctx context.Context, agent, domainFilter string) (uint64, uint64) {
	var answered, won uint64
	_ = a.k.IterateAllInquiries(ctx, func(q *types.Inquiry) bool {
		if domainFilter != "" && q.Domain != domainFilter {
			return false
		}
		_ = a.k.IterateAnswersByInquiry(ctx, q.Id, func(ans *types.Answer) bool {
			if ans.Answerer != agent {
				return false
			}
			answered++
			if ans.Status == types.AnswerStatus_ANSWER_STATUS_WON {
				won++
			}
			return false
		})
		return false
	})
	return answered, won
}

var _ auitypes.InquiryKeeper = (*AgentUnderstandingAdapter)(nil)
