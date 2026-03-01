package keeper

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/partnerships/types"
)

const stratumBPS = 1_000_000

// ScoreFormationMatchWithStratum applies a cross-stratum bonus to formation match scores.
// Partnerships that bridge different strata get priority (R31-4: Metal channels Water).
func (k Keeper) ScoreFormationMatchWithStratum(ctx sdk.Context, match *types.FormationMatch) uint64 {
	if k.ontologyKeeper == nil || match == nil {
		return 0
	}

	score := match.Score

	// Get the domains from the matched pool entries
	e1, found1 := k.GetPoolEntry(ctx, match.Addr1)
	e2, found2 := k.GetPoolEntry(ctx, match.Addr2)
	if !found1 || !found2 {
		return score
	}

	// Check if the partners bridge different strata
	e1Strata := k.collectStrata(ctx, e1.Domains)
	e2Strata := k.collectStrata(ctx, e2.Domains)

	if hasCrossStratum(e1Strata, e2Strata) {
		// 20% bonus for cross-stratum partnerships
		score = score * 1_200_000 / stratumBPS

		ctx.EventManager().EmitEvent(sdk.NewEvent(
			"zerone.partnerships.cross_stratum_bonus",
			sdk.NewAttribute("addr1", match.Addr1),
			sdk.NewAttribute("addr2", match.Addr2),
			sdk.NewAttribute("bonus_bps", fmt.Sprintf("%d", 200_000)),
		))
	}

	return score
}

// collectStrata gathers all related strata for a set of domains.
func (k Keeper) collectStrata(ctx sdk.Context, domains []string) map[string]bool {
	strata := make(map[string]bool)
	for _, domain := range domains {
		related := k.ontologyKeeper.GetRelatedStrata(ctx, domain)
		for _, s := range related {
			strata[s] = true
		}
		// Also include the domain itself as a "stratum location"
		strata[domain] = true
	}
	return strata
}

// hasCrossStratum returns true if the two stratum sets have elements NOT in common,
// indicating the partners bridge different strata.
func hasCrossStratum(s1, s2 map[string]bool) bool {
	if len(s1) == 0 || len(s2) == 0 {
		return false
	}
	// Check if s1 has any element not in s2 or vice versa
	for k := range s1 {
		if !s2[k] {
			return true
		}
	}
	for k := range s2 {
		if !s1[k] {
			return true
		}
	}
	return false
}
