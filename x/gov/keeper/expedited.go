package keeper

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/gov/types"
)

// expeditedTargetModules lists modules whose param-change LIPs can be expedited
// when alignment health is degraded or critical.
var expeditedTargetModules = map[string]bool{
	"knowledge":       true,
	"alignment":       true,
	"capture_defense": true,
}

// getEffectiveVotingPeriod returns the voting period for a LIP, potentially
// halved if alignment health is degraded/critical and the LIP targets
// knowledge-related modules (Wood controls Earth).
func (k Keeper) getEffectiveVotingPeriod(ctx sdk.Context, lip *types.LIP, params *types.Params) uint64 {
	basePeriod := params.VotingPeriodBlocks

	if !isKnowledgeParamLIP(lip) {
		return basePeriod
	}

	if k.alignmentKeeper == nil {
		return basePeriod
	}

	health := k.alignmentKeeper.GetHealthCategory(ctx)
	if health == "degraded" || health == "critical" {
		expedited := basePeriod / 2
		if expedited == 0 {
			expedited = 1
		}

		ctx.EventManager().EmitEvent(
			sdk.NewEvent("zerone.gov.expedited_voting",
				sdk.NewAttribute("lip_id", lip.Id),
				sdk.NewAttribute("target_modules", targetModulesString(lip)),
				sdk.NewAttribute("health_category", health),
				sdk.NewAttribute("base_voting_period", fmt.Sprintf("%d", basePeriod)),
				sdk.NewAttribute("effective_voting_period", fmt.Sprintf("%d", expedited)),
			),
		)

		return expedited
	}

	return basePeriod
}

// isKnowledgeParamLIP returns true if the LIP has param changes targeting
// knowledge, alignment, or capture_defense modules.
func isKnowledgeParamLIP(lip *types.LIP) bool {
	if lip.Category != types.CategoryParameter {
		return false
	}
	for _, pc := range lip.ParamChanges {
		if expeditedTargetModules[pc.Module] {
			return true
		}
	}
	return false
}

// targetModulesString returns a comma-separated list of target modules in the LIP's param changes.
func targetModulesString(lip *types.LIP) string {
	seen := make(map[string]bool)
	var modules string
	for _, pc := range lip.ParamChanges {
		if !seen[pc.Module] {
			if modules != "" {
				modules += ","
			}
			modules += pc.Module
			seen[pc.Module] = true
		}
	}
	return modules
}
