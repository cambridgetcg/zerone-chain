package keeper

import (
	"context"
	"sort"

	"github.com/zerone-chain/zerone/x/agent_understanding/types"
)

// ComposeProfile builds the full UnderstandingProfile for an agent
// at the current block. Reads from upstream keepers; writes nothing.
//
// Composition strategy:
//
//   1. Find all domains where the agent has SOMETHING (qualification
//      record, authored facts, counterexample contributions, inquiry
//      answers). Union the four sets.
//   2. For each domain, build a DomainProfile from per-domain reads.
//   3. Roll up totals, compute frontier_reach, compute composite.
//
// All counts come from upstream module reads. The synthesizer adds
// no signals of its own — its job is to compose existing signals
// in a topic-scoped, queryable shape.
func (k Keeper) ComposeProfile(ctx context.Context, agent string) *types.UnderstandingProfile {
	p := &types.UnderstandingProfile{Agent: agent}
	if agent == "" {
		return p
	}

	// 1. Find all domains the agent touches.
	domainSet := make(map[string]struct{})

	// Qualification domains.
	if k.qualification != nil {
		for _, d := range k.qualification.QualifiedDomains(ctx, agent) {
			domainSet[d] = struct{}{}
		}
	}

	// Authored facts → domains.
	authoredFactsByDomain := make(map[string]uint64)
	if k.knowledge != nil {
		for _, ref := range k.knowledge.FactsBySubmitter(ctx, agent) {
			domainSet[ref.Domain] = struct{}{}
			authoredFactsByDomain[ref.Domain]++
		}
	}

	// Sort the domains so output is deterministic.
	domains := make([]string, 0, len(domainSet))
	for d := range domainSet {
		domains = append(domains, d)
	}
	sort.Strings(domains)

	params := k.GetParams(ctx)
	maxDomains := int(params.MaxDomainsPerQuery)
	if maxDomains > 0 && len(domains) > maxDomains {
		domains = domains[:maxDomains]
	}

	// Used by counterexample-by-domain to look up parent fact's domain.
	factDomainLookup := func(factID string) string {
		if k.knowledge == nil {
			return ""
		}
		return k.knowledge.FactDomain(ctx, factID)
	}

	// 2. Per-domain profiles.
	var totalActivityWeight uint64
	var sparseActivityWeight uint64
	for _, d := range domains {
		dp := &types.DomainProfile{
			Domain:        d,
			FactsAuthored: authoredFactsByDomain[d],
		}
		if k.qualification != nil {
			ver, correct, acc, status, weight, ok := k.qualification.AgentDomainCalibration(ctx, agent, d)
			if ok {
				dp.VerificationsCast = ver
				dp.VerificationsCorrect = correct
				dp.AccuracyBps = acc
				dp.QualificationStatus = status
				dp.QualificationWeight = weight
			}
		}
		if k.counterexamples != nil {
			authored, validated := k.counterexamples.CounterexamplesByAuthorAndDomain(ctx, agent, d, factDomainLookup)
			dp.CounterexamplesAuthored = authored
			dp.CounterexamplesValidated = validated
		}
		if k.inquiry != nil {
			answered, won := k.inquiry.InquiryAnswersByAgentAndDomain(ctx, agent, d)
			dp.InquiriesAnswered = answered
			dp.InquiriesWon = won
		}

		// Domain-level activity: sum of per-domain activity counts,
		// used to weight the frontier_reach metric.
		domainActivity := dp.VerificationsCast + dp.FactsAuthored +
			dp.CounterexamplesAuthored + dp.InquiriesAnswered
		totalActivityWeight += domainActivity

		// Classify domain as sparse based on chain-wide fact count.
		if k.knowledge != nil {
			factCount := k.knowledge.CountFactsByDomain(ctx, d)
			if factCount < params.SparseDomainFactThreshold {
				sparseActivityWeight += domainActivity
			}
		}

		p.Domains = append(p.Domains, dp)
	}

	// 3. Roll-up totals & composite signals.
	if k.counterexamples != nil {
		_, validated := k.counterexamples.CounterexamplesByAuthor(ctx, agent)
		p.TotalCounterexamplesValidated = validated
	}
	if k.inquiry != nil {
		_, won := k.inquiry.InquiryAnswersByAgent(ctx, agent)
		p.TotalInquiriesWon = won
	}
	for _, dp := range p.Domains {
		p.TotalFactsAuthored += dp.FactsAuthored
	}

	// frontier_reach_bps = sparseActivityWeight / totalActivityWeight,
	// scaled to BPS. Zero activity → zero reach.
	if totalActivityWeight > 0 {
		p.FrontierReachBps = sparseActivityWeight * 1_000_000 / totalActivityWeight
	}

	// composite_score_bps: a deliberately simple aggregation. Read
	// the per-domain breakdown for anything load-bearing. The
	// formula: average accuracy across active domains weighted by
	// activity, plus bonuses for breadth (number of active domains)
	// and depth (counterexample contributions). Result is BPS, capped
	// at 1,000,000.
	p.CompositeScoreBps = computeComposite(p)

	return p
}

// ComposeDomainProfile is the cheap single-domain version. Avoids
// walking all domains the agent touches — useful when a query only
// needs one (agent, domain) pair.
func (k Keeper) ComposeDomainProfile(ctx context.Context, agent, domain string) *types.DomainProfile {
	dp := &types.DomainProfile{Domain: domain}
	if agent == "" || domain == "" {
		return dp
	}
	if k.qualification != nil {
		ver, correct, acc, status, weight, ok := k.qualification.AgentDomainCalibration(ctx, agent, domain)
		if ok {
			dp.VerificationsCast = ver
			dp.VerificationsCorrect = correct
			dp.AccuracyBps = acc
			dp.QualificationStatus = status
			dp.QualificationWeight = weight
		}
	}
	if k.knowledge != nil {
		for _, ref := range k.knowledge.FactsBySubmitter(ctx, agent) {
			if ref.Domain == domain {
				dp.FactsAuthored++
			}
		}
	}
	if k.counterexamples != nil {
		factDomainLookup := func(factID string) string {
			if k.knowledge == nil {
				return ""
			}
			return k.knowledge.FactDomain(ctx, factID)
		}
		authored, validated := k.counterexamples.CounterexamplesByAuthorAndDomain(ctx, agent, domain, factDomainLookup)
		dp.CounterexamplesAuthored = authored
		dp.CounterexamplesValidated = validated
	}
	if k.inquiry != nil {
		answered, won := k.inquiry.InquiryAnswersByAgentAndDomain(ctx, agent, domain)
		dp.InquiriesAnswered = answered
		dp.InquiriesWon = won
	}
	return dp
}

// computeComposite produces a single 0..1_000_000 score. It is
// deliberately an opinionated aggregation; downstream consumers
// should prefer the per-domain breakdown for any judgment-loaded use.
func computeComposite(p *types.UnderstandingProfile) uint64 {
	if len(p.Domains) == 0 {
		return 0
	}
	// Activity-weighted accuracy.
	var totalActivity uint64
	var weightedAccuracy uint64
	activeDomains := uint64(0)
	for _, d := range p.Domains {
		activity := d.VerificationsCast + d.FactsAuthored +
			d.CounterexamplesAuthored + d.InquiriesAnswered
		if activity > 0 {
			activeDomains++
		}
		totalActivity += activity
		weightedAccuracy += d.AccuracyBps * activity
	}
	if totalActivity == 0 {
		return 0
	}
	score := weightedAccuracy / totalActivity

	// Breadth bonus: small positive nudge per active domain, capped.
	const breadthCapBps = 100_000 // up to +10%
	const perDomainBonusBps = 5_000 // +0.5% per active domain
	breadth := activeDomains * perDomainBonusBps
	if breadth > breadthCapBps {
		breadth = breadthCapBps
	}
	score += breadth

	// Depth bonus: validated counterexample contributions.
	const depthCapBps = 100_000     // up to +10%
	const perCEBonusBps = 2_000      // +0.2% per validated CE
	depth := p.TotalCounterexamplesValidated * perCEBonusBps
	if depth > depthCapBps {
		depth = depthCapBps
	}
	score += depth

	if score > 1_000_000 {
		return 1_000_000
	}
	return score
}
