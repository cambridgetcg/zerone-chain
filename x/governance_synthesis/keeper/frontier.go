package keeper

import (
	"context"
	"sort"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/governance_synthesis/types"
)

// ComposeFrontier produces the chain's per-domain sparsity map.
// Walks every domain returned by the ontology keeper and composes a
// DomainFrontier for each, then sorts by sparsity descending.
//
// Returns an empty Frontier if any required upstream keeper is
// missing — the synthesizer is opt-in by design.
func (k Keeper) ComposeFrontier(ctx context.Context, limit uint32) types.Frontier {
	out := types.Frontier{}
	if k.ontologyKeeper == nil || k.frontierKnowledgeKeeper == nil {
		return out
	}

	// Gather domain names. The full list is bounded by the chain's
	// configured ontology, which is already capped per-stratum.
	domains := []string{}
	k.ontologyKeeper.IterateDomainNames(ctx, func(name string) bool {
		if name != "" {
			domains = append(domains, name)
		}
		return false
	})

	rows := make([]*types.DomainFrontier, 0, len(domains))
	for _, d := range domains {
		row := &types.DomainFrontier{Domain: d}
		row.VerifiedFacts = k.frontierKnowledgeKeeper.CountFactsByDomain(ctx, d)
		if k.frontierInquiryKeeper != nil {
			row.OpenInquiries = k.frontierInquiryKeeper.CountOpenInquiriesByDomain(ctx, d)
		}
		// Counterexamples scope-counter needs to resolve each
		// candidate counterexample's parent fact's domain; bind a
		// lookup against the knowledge keeper.
		if k.frontierCounterexamples != nil {
			factDomain := func(factID string) string {
				return k.frontierKnowledgeKeeper.FactDomain(ctx, factID)
			}
			authored, validated := k.frontierCounterexamples.CountCounterexamplesByDomain(ctx, d, factDomain)
			row.CounterexamplesAuthored = authored
			row.CounterexamplesValidated = validated
		}
		// Coverage: validated / facts × 1M. If facts == 0, coverage
		// is undefined; report 0 (no facts → no coverage to compute).
		if row.VerifiedFacts > 0 {
			row.CounterexampleCoverageBps = row.CounterexamplesValidated * 1_000_000 / row.VerifiedFacts
		}
		row.SparsityScoreBps = computeSparsity(row)
		rows = append(rows, row)
	}

	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].SparsityScoreBps > rows[j].SparsityScoreBps
	})

	if limit > 0 && uint32(len(rows)) > limit {
		rows = rows[:limit]
	}
	out.Domains = rows
	out.ComputedAtBlock = uint64(sdk.UnwrapSDKContext(ctx).BlockHeight())
	return out
}

// computeSparsity composes the per-domain sparsity score in BPS.
//
// Three components, each contributing up to 333,333 BPS:
//
//   1. Inverse fact density:
//      - 0 facts → 333_333 (max sparse)
//      - 50+ facts (default sparse threshold) → 0 (mapped territory)
//      - linear in between
//
//   2. Open-inquiry pressure:
//      - 10+ open inquiries → 333_333 (max demand)
//      - 0 open inquiries → 0
//      - linear in between
//
//   3. Inverse counterexample coverage:
//      - 0% coverage → 333_333 (no alignment-by-structure work yet)
//      - 100%+ coverage → 0 (well-covered)
//      - linear in between
//
// The components sum to a 0..1_000_000 score. Higher = sparser =
// higher-priority for inquiry. The formula is intentionally simple;
// downstream consumers should treat it as a sketch, not a verdict.
func computeSparsity(row *types.DomainFrontier) uint64 {
	const componentMax = uint64(333_333)
	const sparseFactThreshold = uint64(50)
	const demandSaturation = uint64(10)
	const coverageSaturation = uint64(1_000_000) // BPS

	// 1. Inverse fact density.
	var factSparsity uint64
	if row.VerifiedFacts >= sparseFactThreshold {
		factSparsity = 0
	} else {
		factSparsity = componentMax * (sparseFactThreshold - row.VerifiedFacts) / sparseFactThreshold
	}

	// 2. Open-inquiry pressure.
	var demand uint64
	if row.OpenInquiries >= demandSaturation {
		demand = componentMax
	} else {
		demand = componentMax * row.OpenInquiries / demandSaturation
	}

	// 3. Inverse counterexample coverage.
	var coverageGap uint64
	if row.CounterexampleCoverageBps >= coverageSaturation {
		coverageGap = 0
	} else {
		coverageGap = componentMax * (coverageSaturation - row.CounterexampleCoverageBps) / coverageSaturation
	}

	score := factSparsity + demand + coverageGap
	if score > 1_000_000 {
		score = 1_000_000
	}
	return score
}
