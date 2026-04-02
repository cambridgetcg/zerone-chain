package keeper

import (
	"context"
	"encoding/json"
	"strconv"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// stakingOutcome represents the binary result of reviewer vote classification.
type stakingOutcome int

const (
	outcomeAccept       stakingOutcome = iota
	outcomeReject
	outcomeDeepContested
)

// reviewerSide classifies a reviewer's position.
type reviewerSide int

const (
	sideAccept reviewerSide = iota
	sideReject
)

// EscrowReviewerStake locks a reviewer's stake for a quality round.
// The stake amount is submitterStake × ReviewerStakeRatioBps / 10000.
// Returns nil if no stake is required (zero ratio or zero submitter stake).
func (k Keeper) EscrowReviewerStake(ctx context.Context, roundID, verifier string, submitterStake sdkmath.Int) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	rp := k.GetReviewerStakingParams(ctx)
	if rp.ReviewerStakeRatioBps == 0 || !submitterStake.IsPositive() {
		return nil
	}
	reviewerStake := submitterStake.Mul(sdkmath.NewInt(int64(rp.ReviewerStakeRatioBps))).Quo(sdkmath.NewInt(10_000))
	if !reviewerStake.IsPositive() {
		return nil
	}
	verifierAddr, err := sdk.AccAddressFromBech32(verifier)
	if err != nil {
		return nil // Skip escrow for non-bech32 verifier IDs (e.g. legacy rounds).
	}
	if err := k.bankKeeper.SendCoinsFromAccountToModule(
		sdkCtx, verifierAddr, types.ModuleName,
		sdk.NewCoins(sdk.NewCoin("uzrn", reviewerStake)),
	); err != nil {
		return types.ErrReviewerStakeInsufficient.Wrap(err.Error())
	}
	return k.SetReviewerStake(ctx, roundID, verifier, reviewerStake.String())
}

// RecordContestedStrike increments the contested-deep count for a content hash
// and returns whether the content should be permanently rejected.
func (k Keeper) RecordContestedStrike(ctx context.Context, contentHash string, rp types.ReviewerStakingParams) (count uint64, permanent bool, err error) {
	if contentHash == "" {
		return 0, false, nil
	}
	count, err = k.IncrementContestedDeepCount(ctx, contentHash)
	if err != nil {
		return 0, false, err
	}
	permanent = count >= rp.MaxContestedDeepCount
	return count, permanent, nil
}

// distributeReviewerStakes handles all stake distribution after aggregation.
// If reviewer stakes exist for this round, it distributes according to the
// dual-staking mechanism. Otherwise it falls back to returning the submitter's
// full stake (legacy behavior for rounds without reviewer staking).
func (k Keeper) distributeReviewerStakes(
	ctx context.Context,
	round *types.QualityRound,
	sub *types.Submission,
	params *types.Params,
) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	submitterStake, ok := sdkmath.NewIntFromString(sub.Stake)
	if !ok || !submitterStake.IsPositive() {
		return nil // No stake to distribute
	}

	// Check if any reviewer stakes were escrowed for this round.
	// If not, fall back to legacy behavior (unconditional stake return).
	allStakes := k.GetAllReviewerStakes(ctx, round.Id)
	if len(allStakes) == 0 {
		return k.returnSubmitterStake(sdkCtx, sub, submitterStake)
	}

	rp := k.GetReviewerStakingParams(ctx)

	// Classify each revealed voter.
	sides := classifyVoters(round, params)
	if len(sides) == 0 {
		// No reveals but stakes exist → return all stakes (grace).
		if err := k.returnSubmitterStake(sdkCtx, sub, submitterStake); err != nil {
			return err
		}
		return k.returnAllReviewerStakes(sdkCtx, round.Id, allStakes)
	}

	// Count sides.
	var acceptCount, rejectCount int
	for _, s := range sides {
		if s == sideAccept {
			acceptCount++
		} else {
			rejectCount++
		}
	}
	totalRevealed := len(sides)

	// Determine outcome using 2/3 supermajority.
	outcome := determineOutcome(acceptCount, rejectCount, totalRevealed)

	switch outcome {
	case outcomeAccept:
		return k.distributeAccept(sdkCtx, round, sub, submitterStake, sides, rp)
	case outcomeReject:
		return k.distributeReject(sdkCtx, round, sub, submitterStake, sides, rp)
	default:
		return k.distributeDeepContested(sdkCtx, round, sub, submitterStake, rp)
	}
}

// classifyVoters returns a map of verifier → side based on their individual votes.
func classifyVoters(round *types.QualityRound, params *types.Params) map[string]reviewerSide {
	sides := make(map[string]reviewerSide, len(round.Reveals))
	for _, reveal := range round.Reveals {
		var vote types.QualityVote
		if err := json.Unmarshal([]byte(reveal.Vote), &vote); err != nil {
			continue
		}
		if vote.OverallQuality >= params.BronzeThreshold {
			sides[reveal.Verifier] = sideAccept
		} else {
			sides[reveal.Verifier] = sideReject
		}
	}
	return sides
}

// determineOutcome checks for 2/3 supermajority.
func determineOutcome(acceptCount, rejectCount, total int) stakingOutcome {
	// 2/3 supermajority: count * 3 >= total * 2
	if acceptCount*3 >= total*2 {
		return outcomeAccept
	}
	if rejectCount*3 >= total*2 {
		return outcomeReject
	}
	return outcomeDeepContested
}

// distributeAccept handles the ACCEPT outcome: submitter gets full stake back
// plus accept bonus from minority pot; majority reviewers get stake + show-up
// rewards from minority pot. Show-up rewards come from minority pot only — no
// rewards on unanimous votes.
func (k Keeper) distributeAccept(
	sdkCtx sdk.Context,
	round *types.QualityRound,
	sub *types.Submission,
	submitterStake sdkmath.Int,
	sides map[string]reviewerSide,
	rp types.ReviewerStakingParams,
) error {
	// Collect majority and minority verifiers.
	var majority, minority []string
	for verifier, side := range sides {
		if side == sideAccept {
			majority = append(majority, verifier)
		} else {
			minority = append(minority, verifier)
		}
	}

	// Show-up rewards come from minority pot only (no rewards on unanimous).
	minorityPot := k.sumReviewerStakes(sdkCtx, round.Id, minority)

	// DOKIMANT: return a fraction of the minority pot to minority verifiers (constitutive reward).
	minorityRetention := sdkmath.ZeroInt()
	if len(minority) > 0 && rp.MinorityRetentionBps > 0 {
		minorityRetention = minorityPot.Mul(sdkmath.NewInt(int64(rp.MinorityRetentionBps))).Quo(sdkmath.NewInt(10_000))
		perMinority := minorityRetention.Quo(sdkmath.NewInt(int64(len(minority))))
		if perMinority.IsPositive() {
			for _, verifier := range minority {
				addr, err := sdk.AccAddressFromBech32(verifier)
				if err == nil {
					_ = k.bankKeeper.SendCoinsFromModuleToAccount(
						sdkCtx, types.ModuleName, addr,
						sdk.NewCoins(sdk.NewCoin("uzrn", perMinority)),
					)
				}
			}
		}
	}
	// Only the non-retained fraction of the minority pot flows to majority distribution.
	effectiveMinorityPot := minorityPot.Sub(minorityRetention)

	showUpPool := effectiveMinorityPot.Mul(sdkmath.NewInt(int64(rp.ShowUpRewardRatioBps))).Quo(sdkmath.NewInt(10_000))
	afterShowUp := effectiveMinorityPot.Sub(showUpPool)

	acceptReward := submitterStake.Mul(sdkmath.NewInt(int64(rp.AcceptRewardRatioBps))).Quo(sdkmath.NewInt(10_000))
	if acceptReward.GT(afterShowUp) {
		acceptReward = afterShowUp
	}
	remainingPot := afterShowUp.Sub(acceptReward)

	// Pay submitter: full stake back + accept reward.
	submitterPayout := submitterStake.Add(acceptReward)
	if submitterPayout.IsPositive() {
		submitterAddr, err := sdk.AccAddressFromBech32(sub.Submitter)
		if err == nil {
			_ = k.bankKeeper.SendCoinsFromModuleToAccount(
				sdkCtx, types.ModuleName, submitterAddr,
				sdk.NewCoins(sdk.NewCoin("uzrn", submitterPayout)),
			)
		}
	}

	// Pay majority reviewers: own stake + share of (showUpPool + remainingPot).
	numMaj := int64(len(majority))
	if numMaj > 0 {
		distribPool := showUpPool.Add(remainingPot)
		distribPerMaj := distribPool.Quo(sdkmath.NewInt(numMaj))

		for _, verifier := range majority {
			stakeStr, found := k.GetReviewerStake(sdkCtx, round.Id, verifier)
			if !found {
				continue
			}
			reviewerStake, ok := sdkmath.NewIntFromString(stakeStr)
			if !ok {
				continue
			}
			payout := reviewerStake.Add(distribPerMaj)
			if payout.IsPositive() {
				addr, err := sdk.AccAddressFromBech32(verifier)
				if err == nil {
					_ = k.bankKeeper.SendCoinsFromModuleToAccount(
						sdkCtx, types.ModuleName, addr,
						sdk.NewCoins(sdk.NewCoin("uzrn", payout)),
					)
				}
			}
		}
	}

	// Minority: partial retention already paid above; remainder stays in module.
	// Protocol: rounding dust stays in module account.

	k.emitStakingEvent(sdkCtx, round.Id, "accept", len(majority), len(minority))
	return nil
}

// distributeReject handles the REJECT outcome: submitter loses everything;
// majority rejectors get their stake + challenge bonus + minority pot.
// Show-up rewards come from minority pot only (no separate deduction from submitter).
func (k Keeper) distributeReject(
	sdkCtx sdk.Context,
	round *types.QualityRound,
	sub *types.Submission,
	submitterStake sdkmath.Int,
	sides map[string]reviewerSide,
	rp types.ReviewerStakingParams,
) error {
	// Collect majority (rejectors) and minority (acceptors).
	var majority, minority []string
	for verifier, side := range sides {
		if side == sideReject {
			majority = append(majority, verifier)
		} else {
			minority = append(minority, verifier)
		}
	}

	// Challenge bonus from submitter stake; minority pot (minus partial retention) goes to majority.
	challengeBonus := submitterStake.Mul(sdkmath.NewInt(int64(rp.RejectBonusRatioBps))).Quo(sdkmath.NewInt(10_000))
	minorityPot := k.sumReviewerStakes(sdkCtx, round.Id, minority)

	// DOKIMANT: return a fraction of the minority pot to minority verifiers (constitutive reward).
	minorityRetention := sdkmath.ZeroInt()
	if len(minority) > 0 && rp.MinorityRetentionBps > 0 {
		minorityRetention = minorityPot.Mul(sdkmath.NewInt(int64(rp.MinorityRetentionBps))).Quo(sdkmath.NewInt(10_000))
		perMinority := minorityRetention.Quo(sdkmath.NewInt(int64(len(minority))))
		if perMinority.IsPositive() {
			for _, verifier := range minority {
				addr, err := sdk.AccAddressFromBech32(verifier)
				if err == nil {
					_ = k.bankKeeper.SendCoinsFromModuleToAccount(
						sdkCtx, types.ModuleName, addr,
						sdk.NewCoins(sdk.NewCoin("uzrn", perMinority)),
					)
				}
			}
		}
	}
	effectiveMinorityPot := minorityPot.Sub(minorityRetention)

	// Submitter: loses everything (no payout).

	// Pay majority reviewers: own stake + (challengeBonus + effectiveMinorityPot) / numMaj.
	numMaj := int64(len(majority))
	if numMaj > 0 {
		rewardPool := challengeBonus.Add(effectiveMinorityPot)
		rewardPerMaj := rewardPool.Quo(sdkmath.NewInt(numMaj))

		for _, verifier := range majority {
			stakeStr, found := k.GetReviewerStake(sdkCtx, round.Id, verifier)
			if !found {
				continue
			}
			reviewerStake, ok := sdkmath.NewIntFromString(stakeStr)
			if !ok {
				continue
			}
			payout := reviewerStake.Add(rewardPerMaj)
			if payout.IsPositive() {
				addr, err := sdk.AccAddressFromBech32(verifier)
				if err == nil {
					_ = k.bankKeeper.SendCoinsFromModuleToAccount(
						sdkCtx, types.ModuleName, addr,
						sdk.NewCoins(sdk.NewCoin("uzrn", payout)),
					)
				}
			}
		}
	}

	// Protocol gets: submitter stake minus challenge bonus.
	protocolShare := submitterStake.Sub(challengeBonus)
	if protocolShare.IsPositive() {
		k.depositProtocolRevenue(sdkCtx, protocolShare)
	}

	// Minority: partial retention already paid above; remainder stays in module.

	k.emitStakingEvent(sdkCtx, round.Id, "reject", len(majority), len(minority))
	return nil
}

// distributeDeepContested handles the DEEP CONTESTED outcome (no 2/3 supermajority):
// all stakes returned (grace). Records contested strike; at max → permanent reject.
func (k Keeper) distributeDeepContested(
	sdkCtx sdk.Context,
	round *types.QualityRound,
	sub *types.Submission,
	submitterStake sdkmath.Int,
	rp types.ReviewerStakingParams,
) error {
	// Return submitter stake.
	if err := k.returnSubmitterStake(sdkCtx, sub, submitterStake); err != nil {
		return err
	}

	// Return all reviewer stakes.
	allStakes := k.GetAllReviewerStakes(sdkCtx, round.Id)
	if err := k.returnAllReviewerStakes(sdkCtx, round.Id, allStakes); err != nil {
		return err
	}

	// Record contested strike.
	count, permanent, err := k.RecordContestedStrike(sdkCtx, sub.ContentHash, rp)
	if err != nil {
		return err
	}

	if count > 0 {
		sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
			"deep_contested",
			sdk.NewAttribute("round_id", round.Id),
			sdk.NewAttribute("content_hash", sub.ContentHash),
			sdk.NewAttribute("strike_count", strconv.FormatUint(count, 10)),
			sdk.NewAttribute("max_strikes", strconv.FormatUint(rp.MaxContestedDeepCount, 10)),
		))

		if permanent {
			sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
				"content_permanently_rejected",
				sdk.NewAttribute("content_hash", sub.ContentHash),
				sdk.NewAttribute("strike_count", strconv.FormatUint(count, 10)),
			))
		}
	}

	k.emitStakingEvent(sdkCtx, round.Id, "deep_contested", 0, 0)
	return nil
}

// returnAllReviewerStakes returns all escrowed reviewer stakes for a round.
func (k Keeper) returnAllReviewerStakes(sdkCtx sdk.Context, roundID string, stakes map[string]string) error {
	for verifier, stakeStr := range stakes {
		amt, ok := sdkmath.NewIntFromString(stakeStr)
		if !ok || !amt.IsPositive() {
			continue
		}
		addr, err := sdk.AccAddressFromBech32(verifier)
		if err != nil {
			continue
		}
		_ = k.bankKeeper.SendCoinsFromModuleToAccount(
			sdkCtx, types.ModuleName, addr,
			sdk.NewCoins(sdk.NewCoin("uzrn", amt)),
		)
	}
	return nil
}

// returnSubmitterStake sends the submitter's full stake back from the module.
func (k Keeper) returnSubmitterStake(sdkCtx sdk.Context, sub *types.Submission, amount sdkmath.Int) error {
	if sub.Submitter == "" || !amount.IsPositive() {
		return nil
	}
	addr, err := sdk.AccAddressFromBech32(sub.Submitter)
	if err != nil {
		return nil
	}
	return k.bankKeeper.SendCoinsFromModuleToAccount(
		sdkCtx, types.ModuleName, addr,
		sdk.NewCoins(sdk.NewCoin("uzrn", amount)),
	)
}

// sumReviewerStakes returns the total escrowed stakes for the given verifiers.
func (k Keeper) sumReviewerStakes(ctx context.Context, roundID string, verifiers []string) sdkmath.Int {
	total := sdkmath.ZeroInt()
	for _, v := range verifiers {
		stakeStr, found := k.GetReviewerStake(ctx, roundID, v)
		if !found {
			continue
		}
		amt, ok := sdkmath.NewIntFromString(stakeStr)
		if !ok {
			continue
		}
		total = total.Add(amt)
	}
	return total
}

// emitStakingEvent emits a reviewer_staking event.
func (k Keeper) emitStakingEvent(sdkCtx sdk.Context, roundID, outcome string, majority, minority int) {
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"reviewer_staking",
		sdk.NewAttribute("round_id", roundID),
		sdk.NewAttribute("outcome", outcome),
		sdk.NewAttribute("majority_count", strconv.Itoa(majority)),
		sdk.NewAttribute("minority_count", strconv.Itoa(minority)),
	))
}
