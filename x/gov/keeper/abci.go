package keeper

import (
	"fmt"
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/gov/types"
)

// BeginBlocker processes automatic stage transitions and tally resolution.
func (k Keeper) BeginBlocker(ctx sdk.Context) {
	currentHeight := uint64(ctx.BlockHeight())
	params := k.GetParams(ctx)

	// 1. Auto-advance "review" LIPs to "last_call" after review_blocks.
	reviewLIPs := k.GetLIPsByStatus(ctx, types.StatusReview)
	for _, lip := range reviewLIPs {
		catCfg := types.GetCategoryConfig(params, lip.Category)
		reviewBlocks := uint64(0)
		if catCfg != nil {
			reviewBlocks = catCfg.ReviewBlocks
		}
		if currentHeight >= lip.ReviewStartedBlock+reviewBlocks {
			lip.Stage = types.StatusLastCall
			lip.LastCallStartedBlock = currentHeight
			k.SetLIP(ctx, lip)

			ctx.EventManager().EmitEvent(
				sdk.NewEvent("zerone.gov.lip_stage_transition",
					sdk.NewAttribute("lip_id", lip.Id),
					sdk.NewAttribute("from_stage", types.StatusReview),
					sdk.NewAttribute("to_stage", types.StatusLastCall),
					sdk.NewAttribute("block_height", fmt.Sprintf("%d", currentHeight)),
				),
			)
		}
	}

	// 2. Auto-advance "last_call" LIPs to "voting" after discussion_period_blocks.
	lastCallLIPs := k.GetLIPsByStatus(ctx, types.StatusLastCall)
	for _, lip := range lastCallLIPs {
		if currentHeight >= lip.LastCallStartedBlock+params.DiscussionPeriodBlocks {
			lip.Stage = types.StatusVoting
			lip.VotingEndBlock = currentHeight + params.VotingPeriodBlocks
			k.SetLIP(ctx, lip)

			ctx.EventManager().EmitEvent(
				sdk.NewEvent("zerone.gov.lip_stage_transition",
					sdk.NewAttribute("lip_id", lip.Id),
					sdk.NewAttribute("from_stage", types.StatusLastCall),
					sdk.NewAttribute("to_stage", types.StatusVoting),
					sdk.NewAttribute("block_height", fmt.Sprintf("%d", currentHeight)),
					sdk.NewAttribute("voting_end_block", fmt.Sprintf("%d", lip.VotingEndBlock)),
				),
			)
		}
	}

	// 3. Tally expired voting LIPs.
	votingLIPs := k.GetLIPsByStatus(ctx, types.StatusVoting)
	for _, lip := range votingLIPs {
		if lip.VotingEndBlock > 0 && currentHeight >= lip.VotingEndBlock {
			k.tallyAndResolve(ctx, lip, params)
		}
	}

	// 4. Process research spend proposal expiry.
	k.ProcessResearchSpendExpiry(ctx, currentHeight)
}

// tallyAndResolve tallies votes and sets the LIP to passed or failed.
func (k Keeper) tallyAndResolve(ctx sdk.Context, lip *types.LIP, params *types.Params) {
	quorumMet, passed := k.checkQuorumAndSupport(ctx, lip, params)

	if quorumMet && passed {
		lip.Stage = types.StatusPassed
		k.SetLIP(ctx, lip)
		// Execute param changes for parameter-category LIPs.
		if lip.Category == types.CategoryParameter && len(lip.ParamChanges) > 0 {
			k.executeParamChanges(ctx, lip)
		}
		// If this is an upgrade-category LIP with an attached plan, schedule the upgrade.
		if lip.Category == types.CategoryUpgrade {
			if plan, found := k.GetUpgradePlan(ctx, lip.Id); found {
				if uk := k.GetUpgradeKeeper(); uk != nil {
					if err := uk.ScheduleUpgrade(ctx, plan); err != nil {
						k.Logger(ctx).Error("failed to schedule upgrade from LIP",
							"lip_id", lip.Id,
							"upgrade_name", plan.Name,
							"error", err,
						)
					} else {
						ctx.EventManager().EmitEvent(
							sdk.NewEvent("zerone.gov.upgrade_scheduled",
								sdk.NewAttribute("lip_id", lip.Id),
								sdk.NewAttribute("upgrade_name", plan.Name),
								sdk.NewAttribute("height", fmt.Sprintf("%d", plan.Height)),
							),
						)
						k.Logger(ctx).Info("software upgrade scheduled via LIP governance",
							"lip_id", lip.Id, "upgrade_name", plan.Name, "height", plan.Height,
						)
					}
				}
			}
		}
	} else {
		lip.Stage = types.StatusFailed
		k.SetLIP(ctx, lip)
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent("zerone.gov.lip_tallied",
			sdk.NewAttribute("lip_id", lip.Id),
			sdk.NewAttribute("outcome", lip.Stage),
			sdk.NewAttribute("yes_stake", lip.YesStake),
			sdk.NewAttribute("no_stake", lip.NoStake),
			sdk.NewAttribute("abstain_stake", lip.AbstainStake),
			sdk.NewAttribute("unique_voters", fmt.Sprintf("%d", lip.UniqueVoters)),
			sdk.NewAttribute("quorum_met", fmt.Sprintf("%t", quorumMet)),
		),
	)
}

// checkQuorumAndSupport checks quorum and support thresholds on 1,000,000 BPS scale.
func (k Keeper) checkQuorumAndSupport(ctx sdk.Context, lip *types.LIP, params *types.Params) (quorumMet bool, passed bool) {
	yesBig, _ := new(big.Int).SetString(lip.YesStake, 10)
	if yesBig == nil {
		yesBig = big.NewInt(0)
	}
	noBig, _ := new(big.Int).SetString(lip.NoStake, 10)
	if noBig == nil {
		noBig = big.NewInt(0)
	}
	abstainBig, _ := new(big.Int).SetString(lip.AbstainStake, 10)
	if abstainBig == nil {
		abstainBig = big.NewInt(0)
	}

	totalVoted := new(big.Int).Add(yesBig, noBig)
	totalVoted.Add(totalVoted, abstainBig)

	// Get total bonded stake.
	totalBonded := big.NewInt(0)
	if k.stakingKeeper != nil {
		bondedStr, err := k.stakingKeeper.GetTotalBondedStake(ctx)
		if err == nil {
			if tb, ok := new(big.Int).SetString(bondedStr, 10); ok {
				totalBonded = tb
			}
		}
	}

	// Quorum check: (totalVoted * 1_000_000) / totalBonded >= quorumThresholdBps
	if totalBonded.Sign() > 0 {
		actualBps := new(big.Int).Mul(totalVoted, big.NewInt(int64(types.BPSScale)))
		actualBps.Div(actualBps, totalBonded)
		quorumMet = actualBps.Uint64() >= params.QuorumThresholdBps
	}

	// Support check: (yesStake * 1_000_000) / (yesStake + noStake) >= supportThresholdBps
	yesNoTotal := new(big.Int).Add(yesBig, noBig)
	if yesNoTotal.Sign() > 0 {
		supportBps := new(big.Int).Mul(yesBig, big.NewInt(int64(types.BPSScale)))
		supportBps.Div(supportBps, yesNoTotal)
		passed = quorumMet && supportBps.Uint64() >= params.SupportThresholdBps
	}

	return quorumMet, passed
}

// executeParamChanges applies parameter changes from a passed LIP.
func (k Keeper) executeParamChanges(ctx sdk.Context, lip *types.LIP) {
	logger := k.Logger(ctx)
	router := k.GetParamRouter()

	for _, pc := range lip.ParamChanges {
		if router == nil {
			logger.Error("param router not set, skipping param change",
				"lip_id", lip.Id, "module", pc.Module, "key", pc.Key,
			)
			ctx.EventManager().EmitEvent(
				sdk.NewEvent("zerone.gov.param_change_failed",
					sdk.NewAttribute("lip_id", lip.Id),
					sdk.NewAttribute("module", pc.Module),
					sdk.NewAttribute("key", pc.Key),
					sdk.NewAttribute("reason", "param router not set"),
				),
			)
			continue
		}

		if err := router.ApplyParamChange(ctx, pc.Module, pc.Key, pc.Value); err != nil {
			logger.Error("param change failed",
				"lip_id", lip.Id, "module", pc.Module, "key", pc.Key, "error", err,
			)
			ctx.EventManager().EmitEvent(
				sdk.NewEvent("zerone.gov.param_change_failed",
					sdk.NewAttribute("lip_id", lip.Id),
					sdk.NewAttribute("module", pc.Module),
					sdk.NewAttribute("key", pc.Key),
					sdk.NewAttribute("reason", err.Error()),
				),
			)
		} else {
			logger.Info("param change applied",
				"lip_id", lip.Id, "module", pc.Module, "key", pc.Key, "value", pc.Value,
			)
			ctx.EventManager().EmitEvent(
				sdk.NewEvent("zerone.gov.param_change_applied",
					sdk.NewAttribute("lip_id", lip.Id),
					sdk.NewAttribute("module", pc.Module),
					sdk.NewAttribute("key", pc.Key),
					sdk.NewAttribute("value", pc.Value),
				),
			)
		}
	}
}
