package types

import (
	"context"

	creedtypes "github.com/zerone-chain/zerone/x/creed/types"
	knowledgetypes "github.com/zerone-chain/zerone/x/knowledge/types"
)

// KnowledgeKeeper exposes the chain's audit / stress signals: the
// incident log, current circuit-breaker pauses, pending fact-injections
// awaiting guardian veto, and the privileged-action log.
type KnowledgeKeeper interface {
	IterateOpenIncidents(ctx context.Context, cb func(*knowledgetypes.IncidentRecord) bool)
	IteratePausedModules(ctx context.Context, cb func(*knowledgetypes.ModulePause) bool)
	IteratePrivilegedActions(ctx context.Context, cb func(*knowledgetypes.PrivilegedAction) bool)
	IteratePendingFactInjectionsDue(ctx context.Context, height uint64, cb func(*knowledgetypes.PendingFactInjection) bool)
	// IterateAllPendingFactInjections is needed because the keeper's
	// existing iterator is bounded by execute_at_block (the BeginBlocker
	// helper). We expose an unbounded variant via the adapter so the
	// synthesizer can count the queue regardless of maturity.
	IterateAllPendingFactInjections(ctx context.Context, cb func(*knowledgetypes.PendingFactInjection) bool)
}

// CaptureChallengeKeeper exposes the cartel-allegation log. The
// adapter installed in app.go translates capture_challenge's native
// types into the lean ChallengeStatusCounts shape this synthesizer
// needs.
type CaptureChallengeKeeper interface {
	CountChallengesByStatus(ctx context.Context, sinceBlock uint64) ChallengeStatusCounts
}

// ChallengeStatusCounts is the synthesizer's view of capture_challenge.
type ChallengeStatusCounts struct {
	Open           uint32 // submitted/under-review
	UpheldRecent   uint32 // resolved+UPHELD with resolved_block ≥ sinceBlock
}

// AlignmentKeeper exposes the global pacing multipliers — the
// chain's autonomous-throttle signal that the synthesizer surfaces
// at the governance level.
type AlignmentKeeper interface {
	GetGlobalPacingMultiplier(ctx context.Context) (creationBps, analysisBps uint64)
}

// ─── Frontier-query upstreams ────────────────────────────────────────

// OntologyKeeper exposes the list of domains so the frontier
// synthesizer can iterate them.
type OntologyKeeper interface {
	IterateDomainNames(ctx context.Context, cb func(name string) bool)
}

// FrontierKnowledgeKeeper exposes per-domain fact counts plus a
// fact-domain lookup that the counterexamples scope-counter needs.
type FrontierKnowledgeKeeper interface {
	CountFactsByDomain(ctx context.Context, domain string) uint64
	FactDomain(ctx context.Context, factID string) string
}

// FrontierInquiryKeeper exposes per-domain open-inquiry counts.
type FrontierInquiryKeeper interface {
	CountOpenInquiriesByDomain(ctx context.Context, domain string) uint64
}

// FrontierCounterexamplesKeeper exposes per-domain counterexample
// counts (authored and validated). Used to compute
// counterexample_coverage_bps in the frontier signal.
type FrontierCounterexamplesKeeper interface {
	CountCounterexamplesByDomain(ctx context.Context, domain string, factDomain func(factID string) string) (authored, validated uint64)
}

// CreedKeeper exposes the canonical creed surface so the synthesis
// can compose a creed-drift signal. Read-only access to the genesis
// pin, the current pin, and the council registry. The synthesizer
// adds no state of its own; drift is computed live from x/creed.
//
// docs/TRUTH_SEEKING.md commitments 11 and 19: drift is the
// trust-queryability of the chain's voice (11) made specific to the
// creed (19). Without this read path, observers would need to
// stitch x/creed queries together themselves.
type CreedKeeper interface {
	// GetCurrentPin returns the canonical pin. ok=false means the
	// chain is in a pre-anchor state (no genesis pin loaded yet).
	GetCurrentPin(ctx context.Context) (*creedtypes.PinnedCreed, bool)
	// GetPin returns a specific historical pin by version.
	GetPin(ctx context.Context, version uint32) (*creedtypes.PinnedCreed, bool)
	// CouncilTotalActiveWeight is the sum of voting weights across
	// active Creed Council members.
	CouncilTotalActiveWeight(ctx context.Context) uint64
	// IterateCouncilMembers walks the council registry. The
	// synthesizer uses this to count active seats.
	IterateCouncilMembers(ctx context.Context, cb func(*creedtypes.CreedCouncilMember) bool)
}
