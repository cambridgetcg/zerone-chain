package types

import "context"

// FactRef is the minimal data the synthesizer needs about a fact:
// the id and the domain. Avoiding the full Fact proto keeps the
// adapter surface narrow and the synthesizer module independent of
// x/knowledge's evolving Fact shape.
type FactRef struct {
	ID     string
	Domain string
}

// KnowledgeKeeper is the read-only contract this module needs from
// x/knowledge. All implementations are adapters around the real
// keeper, declared in x/knowledge to avoid circular imports.
type KnowledgeKeeper interface {
	// FactsBySubmitter returns the facts authored by the given
	// agent. Called once per profile query; for very prolific agents
	// the response can be large but is bounded by what the chain
	// holds for that agent.
	FactsBySubmitter(ctx context.Context, submitter string) []FactRef
	// FactDomain looks up just the domain for a fact id. Used by
	// the per-domain counterexample counter.
	FactDomain(ctx context.Context, factID string) string
	// CountFactsByDomain returns the total verified-fact count
	// for a domain. Used by the frontier_reach computation to
	// classify a domain as sparse vs dense.
	CountFactsByDomain(ctx context.Context, domain string) uint64
}

// QualificationKeeper exposes per-agent, per-domain calibration and
// status reads.
type QualificationKeeper interface {
	// QualifiedDomains returns the list of domains the agent has a
	// qualification record in (any status).
	QualifiedDomains(ctx context.Context, agent string) []string
	// AgentDomainCalibration returns (verifications, correct,
	// accuracyBps, status, weight) for an agent in a domain.
	// Returns ok=false if no qualification record exists.
	AgentDomainCalibration(ctx context.Context, agent, domain string) (verifications, correct, accuracyBps uint64, status string, weight uint32, ok bool)
}

// CounterexamplesKeeper aggregates per-agent counterexample stats.
type CounterexamplesKeeper interface {
	// CounterexamplesByAuthor returns (authored, validated)
	// counts across all facts. validated counts only those that
	// passed the validation gate.
	CounterexamplesByAuthor(ctx context.Context, author string) (authored, validated uint64)
	// CounterexamplesByAuthorAndDomain returns the same counts
	// scoped to a single domain. The factDomain function is
	// passed in so the counterexamples keeper does not need to
	// directly depend on x/knowledge.
	CounterexamplesByAuthorAndDomain(ctx context.Context, author, domain string, factDomain func(factID string) string) (authored, validated uint64)
}

// InquiryKeeper aggregates per-agent inquiry answer stats.
type InquiryKeeper interface {
	// InquiryAnswersByAgent returns (answered, won) totals across
	// all domains. answered is the total Answer records linked to
	// the agent; won is the subset that resolved as the winning
	// bounty payee.
	InquiryAnswersByAgent(ctx context.Context, agent string) (answered, won uint64)
	// InquiryAnswersByAgentAndDomain returns the same totals
	// scoped to one domain.
	InquiryAnswersByAgentAndDomain(ctx context.Context, agent, domain string) (answered, won uint64)
}
