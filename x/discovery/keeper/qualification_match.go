package keeper

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const BPS = 1_000_000

// ScoreDiscoveryMatch scores a discovery match between two accounts based on
// qualification complementarity (R31-4: Metal generates Water).
func (k Keeper) ScoreDiscoveryMatch(ctx sdk.Context, seeker, candidate string, baseScore uint64) uint64 {
	if k.qualificationKeeper == nil {
		return baseScore
	}

	seekerDomains := k.qualificationKeeper.GetQualifiedDomains(ctx, seeker)
	candidateDomains := k.qualificationKeeper.GetQualifiedDomains(ctx, candidate)

	total := countUnion(seekerDomains, candidateDomains)
	if total == 0 {
		return baseScore
	}

	overlap := countOverlap(seekerDomains, candidateDomains)

	// Complementary qualifications → higher score
	// Partners who cover different domains are more valuable
	complementarity := uint64(total-overlap) * BPS / uint64(total)
	bonusBps := complementarity * 200_000 / BPS // up to 20% bonus
	result := baseScore * (BPS + bonusBps) / BPS

	// Emit event
	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.discovery.qualification_match_bonus",
		sdk.NewAttribute("seeker", seeker),
		sdk.NewAttribute("candidate", candidate),
		sdk.NewAttribute("complementarity_bps", fmt.Sprintf("%d", complementarity)),
		sdk.NewAttribute("bonus_bps", fmt.Sprintf("%d", bonusBps)),
	))

	return result
}

// countOverlap counts domains that appear in both slices.
func countOverlap(a, b []string) int {
	set := make(map[string]bool, len(a))
	for _, d := range a {
		set[d] = true
	}
	count := 0
	for _, d := range b {
		if set[d] {
			count++
		}
	}
	return count
}

// countUnion counts unique domains across both slices.
func countUnion(a, b []string) int {
	set := make(map[string]bool, len(a)+len(b))
	for _, d := range a {
		set[d] = true
	}
	for _, d := range b {
		set[d] = true
	}
	return len(set)
}
