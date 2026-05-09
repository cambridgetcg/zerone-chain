package keeper

import (
	"crypto/sha256"
	"fmt"
	"math/big"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	"github.com/zerone-chain/zerone/x/vesting_rewards/types"
)

// DistributeRevenue routes a reward through the governance-adjustable 4-way split.
//
// The split is driven by RevenueSplit from params (not constants):
//   - ContributorBps: goes to the reward recipient (e.g. block producer)
//   - ProtocolBps:    split further via ProtocolSubSplit (citation/verification/treasury)
//   - ResearchBps:    deposited into the research fund (with founder auto-split)
//   - DevelopmentBps:    development fund (bug bounties, truth discovery, protocol development)
//
// All block-level rewards flow through this router for consistent revenue routing.
func (k Keeper) DistributeRevenue(
	ctx sdk.Context,
	source types.RewardSource,
	amount string,
	recipient string,
	factId string,
) (*types.RewardRouting, error) {
	amountBig := new(big.Int)
	if _, ok := amountBig.SetString(amount, 10); !ok {
		return nil, types.ErrInvalidRewardAmount
	}

	if amountBig.Sign() <= 0 {
		return nil, types.ErrInvalidRewardAmount
	}

	split := k.GetRevenueSplit(ctx)
	subSplit := k.GetProtocolSubSplit(ctx)
	params := k.GetParams(ctx)

	bps := big.NewInt(1000000)

	// 4-way split
	contributorAmount := new(big.Int).Mul(amountBig, big.NewInt(int64(split.ContributorBps)))
	contributorAmount.Div(contributorAmount, bps)

	protocolAmount := new(big.Int).Mul(amountBig, big.NewInt(int64(split.ProtocolBps)))
	protocolAmount.Div(protocolAmount, bps)

	researchAmount := new(big.Int).Mul(amountBig, big.NewInt(int64(split.ResearchBps)))
	researchAmount.Div(researchAmount, bps)

	// Development = remainder to avoid rounding leaks
	devAmount := new(big.Int).Set(amountBig)
	devAmount.Sub(devAmount, contributorAmount)
	devAmount.Sub(devAmount, protocolAmount)
	devAmount.Sub(devAmount, researchAmount)
	if devAmount.Sign() < 0 {
		devAmount.SetInt64(0)
	}

	// Protocol sub-split
	citationPool := new(big.Int).Mul(protocolAmount, big.NewInt(int64(subSplit.CitationBps)))
	citationPool.Div(citationPool, bps)

	verificationPool := new(big.Int).Mul(protocolAmount, big.NewInt(int64(subSplit.VerificationBps)))
	verificationPool.Div(verificationPool, bps)

	// Treasury = remainder of protocol share
	treasuryShare := new(big.Int).Set(protocolAmount)
	treasuryShare.Sub(treasuryShare, citationPool)
	treasuryShare.Sub(treasuryShare, verificationPool)
	if treasuryShare.Sign() < 0 {
		treasuryShare.SetInt64(0)
	}

	// Founder share (deducted from research portion)
	founderShare := new(big.Int)
	if k.isFounderShareActive(ctx, params) {
		founderShare = new(big.Int).Mul(researchAmount, big.NewInt(int64(params.FounderShareBps)))
		founderShare.Div(founderShare, bps)
		researchAmount.Sub(researchAmount, founderShare) // reduce research by founder portion
	}

	routing := &types.RewardRouting{
		Source:           string(source),
		OriginalAmount:   amount,
		ContributorShare: contributorAmount.String(),
		ProtocolShare:    protocolAmount.String(),
		ResearchShare:    researchAmount.String(),
		DevelopmentAmount: devAmount.String(),
		Recipient:        recipient,
		FactId:           factId,
		BlockNumber:      uint64(ctx.BlockHeight()),
		FounderShare:     founderShare.String(),
		CitationPool:     citationPool.String(),
		VerificationPool: verificationPool.String(),
		TreasuryShare:    treasuryShare.String(),
	}

	return routing, nil
}

// RouteFees intercepts transaction fees before x/distribution sweeps them to validators.
// Applies the full 4-way revenue split to accumulated fees in fee_collector.
// Must run in BeginBlock BEFORE x/distribution's BeginBlocker.
func (k Keeper) RouteFees(ctx sdk.Context) error {
	if k.bankKeeper == nil {
		return nil
	}

	split := k.GetRevenueSplit(ctx)
	// If all non-contributor shares are zero, nothing to route
	if split.ProtocolBps == 0 && split.ResearchBps == 0 && split.DevelopmentBps == 0 {
		return nil
	}

	feeCollectorBalances := k.bankKeeper.GetAllBalances(ctx, authtypes.NewModuleAddress(authtypes.FeeCollectorName))
	if feeCollectorBalances.IsZero() {
		return nil
	}

	bps := int64(1000000)

	for _, coin := range feeCollectorBalances {
		if coin.Denom != "uzrn" {
			continue
		}

		totalAmount := coin.Amount

		// Research share
		researchTotal := totalAmount.MulRaw(int64(split.ResearchBps)).QuoRaw(bps)
		if researchTotal.IsPositive() {
			researchCoins := sdk.NewCoins(sdk.NewCoin(coin.Denom, researchTotal))
			// Escrow from fee_collector to vesting_rewards module
			if err := k.bankKeeper.SendCoinsFromModuleToModule(ctx, authtypes.FeeCollectorName, types.ModuleName, researchCoins); err != nil {
				k.Logger(ctx).Warn("failed to escrow fee research share", "err", err)
				continue
			}
			// Route through canonical depositor (handles founder split)
			if err := k.DepositToResearchFund(ctx, types.ModuleName, researchCoins); err != nil {
				k.Logger(ctx).Warn("failed to deposit fee research share", "err", err)
			}
		}

		// Development fund share
		devTotal := totalAmount.MulRaw(int64(split.DevelopmentBps)).QuoRaw(bps)
		if devTotal.IsPositive() {
			devCoins := sdk.NewCoins(sdk.NewCoin(coin.Denom, devTotal))
			if err := k.bankKeeper.SendCoinsFromModuleToModule(ctx, authtypes.FeeCollectorName, types.DevelopmentFundModuleName, devCoins); err != nil {
				k.Logger(ctx).Warn("failed to route fee development share", "err", err)
				continue
			}
		}

		// Protocol share stays in fee_collector for x/distribution to sweep to validators.
		// Contributor share is irrelevant for fees (fees come from tx senders, not contributors).
	}

	return nil
}

// DepositToResearchFund routes a deposit to the research fund with founder auto-split.
// All modules that send funds to research_fund SHOULD call this instead of sending directly,
// so the founder's 7% share is consistently applied regardless of deposit source.
//
// sourceModule must hold the funds in its module account before calling this method.
// The method splits the amount: 7% to founder (if active), remainder to research_fund.
// Falls back to 100% research_fund if founder address is invalid/empty or governance has sunset.
func (k Keeper) DepositToResearchFund(ctx sdk.Context, sourceModule string, amount sdk.Coins) error {
	if amount.IsZero() {
		return nil
	}

	// Escrow to vesting_rewards if source is a different module.
	if sourceModule != types.ModuleName {
		if err := k.bankKeeper.SendCoinsFromModuleToModule(ctx, sourceModule, types.ModuleName, amount); err != nil {
			return fmt.Errorf("research fund escrow to vesting_rewards failed: %w", err)
		}
	}

	params := k.GetParams(ctx)
	founderActive := k.isFounderShareActive(ctx, params)

	for _, coin := range amount {
		if coin.Amount.IsZero() {
			continue
		}

		founderAmount := sdkmath.ZeroInt()
		researchAmount := coin.Amount

		if founderActive {
			founderAmount = coin.Amount.MulRaw(int64(params.FounderShareBps)).QuoRaw(1_000_000)
			researchAmount = coin.Amount.Sub(founderAmount)
		}

		// Send research portion to research_fund
		if researchAmount.IsPositive() {
			researchCoins := sdk.NewCoins(sdk.NewCoin(coin.Denom, researchAmount))
			if err := k.bankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, types.ResearchFundModuleName, researchCoins); err != nil {
				return fmt.Errorf("research fund deposit failed: %w", err)
			}
		}

		// Send founder portion directly to founder address
		if founderAmount.IsPositive() {
			founderAddr, addrErr := sdk.AccAddressFromBech32(params.FounderAddress)
			if addrErr != nil {
				// Invalid founder address — send full amount to research_fund instead
				fallbackCoins := sdk.NewCoins(sdk.NewCoin(coin.Denom, founderAmount))
				if err := k.bankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, types.ResearchFundModuleName, fallbackCoins); err != nil {
					return fmt.Errorf("research fund fallback deposit failed: %w", err)
				}
			} else {
				founderCoins := sdk.NewCoins(sdk.NewCoin(coin.Denom, founderAmount))
				if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, founderAddr, founderCoins); err != nil {
					k.Logger(ctx).Warn("failed to send founder share, routing to research_fund",
						"source", sourceModule, "error", err)
					if err2 := k.bankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, types.ResearchFundModuleName, founderCoins); err2 != nil {
						return fmt.Errorf("research fund fallback deposit failed: %w", err2)
					}
				}
			}
		}

		ctx.EventManager().EmitEvent(sdk.NewEvent(
			"zerone.vesting_rewards.research_fund_deposit",
			sdk.NewAttribute("source_module", sourceModule),
			sdk.NewAttribute("denom", coin.Denom),
			sdk.NewAttribute("total", coin.Amount.String()),
			sdk.NewAttribute("research", researchAmount.String()),
			sdk.NewAttribute("founder", founderAmount.String()),
		))
	}

	return nil
}

// DisburseFromResearchFund sends coins from the research fund module account to a recipient.
func (k Keeper) DisburseFromResearchFund(ctx sdk.Context, recipient sdk.AccAddress, amount sdk.Coins) error {
	if k.bankKeeper == nil {
		return fmt.Errorf("bank keeper not available")
	}
	return k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ResearchFundModuleName, recipient, amount)
}

// DisburseFromDevelopmentFund sends coins from the development fund module account to a recipient.
// Called by governance proposals for bug bounties and development grants.
func (k Keeper) DisburseFromDevelopmentFund(ctx sdk.Context, recipient sdk.AccAddress, amount sdk.Coins) error {
	if k.bankKeeper == nil {
		return fmt.Errorf("bank keeper not available")
	}
	return k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.DevelopmentFundModuleName, recipient, amount)
}

// applyDecay computes: amount * (decayBps/1000000)^epochs using integer exponentiation by squaring.
// decayBps is on a 1,000,000 scale (900000 = 0.9).
func applyDecay(amount *big.Int, decayBps uint64, epochs uint64) *big.Int {
	if epochs == 0 {
		return new(big.Int).Set(amount)
	}

	denom := big.NewInt(1000000)
	base := big.NewInt(int64(decayBps))
	exp := epochs

	// Exponentiation by squaring for decay^epochs in fixed-point (denom scale)
	result := new(big.Int).Set(denom) // start at 1.0
	for exp > 0 {
		if exp%2 == 1 {
			result.Mul(result, base)
			result.Div(result, denom)
		}
		base.Mul(base, base)
		base.Div(base, denom)
		exp /= 2
	}

	// amount * result / denom
	out := new(big.Int).Mul(amount, result)
	out.Div(out, denom)
	return out
}

// calculateBlockReward computes the per-block reward under pure PoT minting.
// Formula: R = max(initialReward * decayFactor^epoch, floorReward)
func calculateBlockReward(ctx sdk.Context, params *types.Params) *big.Int {
	height := uint64(ctx.BlockHeight())

	initialReward := new(big.Int)
	if _, ok := initialReward.SetString(params.BlockReward, 10); !ok || initialReward.Sign() <= 0 {
		return new(big.Int)
	}

	epoch := height / params.BlocksPerRewardEpoch

	reward := applyDecay(initialReward, params.RewardDecayBps, epoch)

	floorReward := new(big.Int)
	if params.FloorReward != "" {
		floorReward.SetString(params.FloorReward, 10)
	}
	if floorReward.Sign() > 0 && reward.Cmp(floorReward) < 0 {
		reward.Set(floorReward)
	}

	return reward
}

// GetEpochBlockRewardPool estimates the total block rewards for a given epoch (in uzrn).
func (k Keeper) GetEpochBlockRewardPool(ctx sdk.Context, epoch uint64) uint64 {
	params := k.GetParams(ctx)

	initialReward := new(big.Int)
	if _, ok := initialReward.SetString(params.BlockReward, 10); !ok || initialReward.Sign() <= 0 {
		return 0
	}

	perBlock := applyDecay(initialReward, params.RewardDecayBps, epoch)
	if perBlock.Sign() <= 0 {
		return 0
	}

	pool := new(big.Int).Mul(perBlock, big.NewInt(int64(params.BlocksPerRewardEpoch)))
	if !pool.IsUint64() {
		return ^uint64(0)
	}
	return pool.Uint64()
}

// DistributeBlockReward mints and distributes block production rewards via pure PoT.
//
// All ZRN is minted per-block through PoT consensus, capped at 222,222,222 ZRN.
// The reward is scaled by validator participation:
//
//	reward = baseReward * min(1, activeValidators / targetValidators)
//
// After minting, the full 4-way revenue split is applied via DistributeRevenue.
func (k Keeper) DistributeBlockReward(
	ctx sdk.Context,
	producer string,
	activeValidatorCount uint32,
	hasTransactions bool,
) (*types.BlockRewardDistribution, error) {
	params := k.GetParams(ctx)
	height := uint64(ctx.BlockHeight())

	emptyDist := func() *types.BlockRewardDistribution {
		return &types.BlockRewardDistribution{
			BlockHeight:    height,
			ProducerReward: "0",
			ResearchShare:  "0",
			TotalMinted:    "0",
			ValidatorCount: activeValidatorCount,
			DevelopmentAmount:     "0",
			ProtocolShare:  "0",
		}
	}

	// Empty block check: 0% reward for empty blocks (PoT alignment)
	if !hasTransactions && params.EmptyBlockRewardRate == 0 {
		return emptyDist(), nil
	}

	// Calculate reward (decay + floor)
	effectiveReward := calculateBlockReward(ctx, params)
	if effectiveReward == nil || effectiveReward.Sign() <= 0 {
		dist := emptyDist()
		k.SetBlockRewardDistribution(ctx, dist)
		return dist, nil
	}

	// Scale by validator participation: min(1, active/target)
	if params.MinValidatorsForFullReward > 0 && activeValidatorCount < params.MinValidatorsForFullReward {
		effectiveReward.Mul(effectiveReward, big.NewInt(int64(activeValidatorCount)))
		effectiveReward.Div(effectiveReward, big.NewInt(int64(params.MinValidatorsForFullReward)))
	}

	// Apply empty block rate if applicable
	if !hasTransactions && params.EmptyBlockRewardRate > 0 {
		effectiveReward.Mul(effectiveReward, big.NewInt(int64(params.EmptyBlockRewardRate)))
		effectiveReward.Div(effectiveReward, big.NewInt(10000))
	}

	// Knowledge-coupling: scale reward by verification throughput (T9 / thesis).
	// Below target rate → reward decays linearly to KnowledgeCouplingFloorBps.
	// At or above target → full reward. Disabled when target is 0 or knowledge
	// keeper not wired (nil-safe for harnesses).
	if params.KnowledgeCouplingTargetBps > 0 && k.knowledgeKeeper != nil {
		rate := k.knowledgeKeeper.GetVerificationRate(ctx)
		const bps uint64 = 1_000_000
		var multiplier uint64
		switch {
		case rate >= params.KnowledgeCouplingTargetBps:
			multiplier = bps
		default:
			// Linear scaling: rate/target, floored at KnowledgeCouplingFloorBps.
			multiplier = rate * bps / params.KnowledgeCouplingTargetBps
			if multiplier < params.KnowledgeCouplingFloorBps {
				multiplier = params.KnowledgeCouplingFloorBps
			}
		}
		effectiveReward.Mul(effectiveReward, new(big.Int).SetUint64(multiplier))
		effectiveReward.Div(effectiveReward, new(big.Int).SetUint64(bps))

		ctx.EventManager().EmitEvent(sdk.NewEvent(
			"zerone.vesting_rewards.knowledge_coupling_applied",
			sdk.NewAttribute("verification_rate_bps", fmt.Sprintf("%d", rate)),
			sdk.NewAttribute("target_bps", fmt.Sprintf("%d", params.KnowledgeCouplingTargetBps)),
			sdk.NewAttribute("multiplier_bps", fmt.Sprintf("%d", multiplier)),
		))
	}

	if effectiveReward.Sign() <= 0 {
		dist := emptyDist()
		k.SetBlockRewardDistribution(ctx, dist)
		return dist, nil
	}

	// Mint new tokens (supply-cap enforced) into vesting_rewards' own
	// module account; subsequent steps split and route per the revenue
	// distribution.
	actualMinted, err := k.MintWithCap(ctx, types.ModuleName, effectiveReward)
	if err != nil {
		k.Logger(ctx).Error("failed to mint block reward", "error", err)
		dist := emptyDist()
		k.SetBlockRewardDistribution(ctx, dist)
		return dist, nil
	}

	if actualMinted.Sign() <= 0 {
		dist := emptyDist()
		k.SetBlockRewardDistribution(ctx, dist)
		return dist, nil
	}

	// Route through 4-way revenue split
	routing, err := k.DistributeRevenue(ctx, types.SourceBlockProduction, actualMinted.String(), producer, "")
	if err != nil {
		dist := emptyDist()
		k.SetBlockRewardDistribution(ctx, dist)
		return dist, nil
	}

	dist := &types.BlockRewardDistribution{
		BlockHeight:    height,
		ProducerReward: routing.ContributorShare,
		ResearchShare:  routing.ResearchShare,
		TotalMinted:    routing.OriginalAmount,
		ValidatorCount: activeValidatorCount,
		FounderShare:   routing.FounderShare,
		DevelopmentAmount:     routing.DevelopmentAmount,
		ProtocolShare:  routing.ProtocolShare,
	}

	// Distribute minted coins via bank keeper
	if k.bankKeeper != nil {
		// Send contributor share to block producer
		contributorBig := new(big.Int)
		contributorBig.SetString(routing.ContributorShare, 10)
		if contributorBig.Sign() > 0 {
			producerAddr, addrErr := sdk.AccAddressFromBech32(producer)
			if addrErr == nil {
				producerCoins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(contributorBig)))
				if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, producerAddr, producerCoins); err != nil {
					k.Logger(ctx).Error("failed to send producer reward", "error", err)
				}
			}
		}

		// Route research + founder share through canonical depositor
		researchBig := new(big.Int)
		researchBig.SetString(routing.ResearchShare, 10)
		founderBig := new(big.Int)
		founderBig.SetString(routing.FounderShare, 10)
		grossResearch := new(big.Int).Add(researchBig, founderBig)
		if grossResearch.Sign() > 0 {
			researchCoins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(grossResearch)))
			if err := k.DepositToResearchFund(ctx, types.ModuleName, researchCoins); err != nil {
				k.Logger(ctx).Error("failed to deposit research share", "error", err)
			}
		}

		// Development fund share
		devBig := new(big.Int)
		devBig.SetString(routing.DevelopmentAmount, 10)
		if devBig.Sign() > 0 {
			devCoins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(devBig)))
			if err := k.bankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, types.DevelopmentFundModuleName, devCoins); err != nil {
				k.Logger(ctx).Error("failed to route development fund share", "error", err)
			}
		}

		// Split protocol share via ProtocolSubSplit
		verificationBig := new(big.Int)
		verificationBig.SetString(routing.VerificationPool, 10)
		citationBig := new(big.Int)
		citationBig.SetString(routing.CitationPool, 10)

		// Send verification pool to knowledge module
		if verificationBig.Sign() > 0 {
			// Split verification pool: 70% to knowledge, 30% to compute_pool
			computePoolBig := new(big.Int).Mul(verificationBig, big.NewInt(int64(types.ComputePoolShareBps)))
			computePoolBig.Div(computePoolBig, big.NewInt(1000000))
			actualVerificationBig := new(big.Int).Sub(verificationBig, computePoolBig)

			if actualVerificationBig.Sign() > 0 {
				verificationCoins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(actualVerificationBig)))
				if err := k.bankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, types.KnowledgeModuleName, verificationCoins); err != nil {
					k.Logger(ctx).Error("failed to send verification pool share", "error", err)
				}
			}

			if computePoolBig.Sign() > 0 {
				computeCoins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(computePoolBig)))
				if err := k.bankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, types.ComputePoolModuleName, computeCoins); err != nil {
					k.Logger(ctx).Error("failed to send compute pool share", "error", err)
				}
			}
		}

		// Citation pool and treasury stay in module account (or route to citation/treasury modules when they exist)
		_ = citationBig
	}

	// Record total minted after distribution
	dist.FundBalanceAfter = k.GetTotalMinted(ctx).String()

	k.SetBlockRewardDistribution(ctx, dist)

	k.Logger(ctx).Debug("distributed block reward",
		"block", height,
		"producer", producer,
		"contributor", routing.ContributorShare,
		"protocol", routing.ProtocolShare,
		"research", routing.ResearchShare,
		"development", routing.DevelopmentAmount,
		"total_minted", dist.FundBalanceAfter,
	)

	return dist, nil
}

// FalsifyClaim handles clawback when a claim is proven false.
//
// Clawback logic:
//   - 33% of already-released rewards are clawed back
//   - All unvested amount is forfeited
//   - Reserve goes to challenger as bonus
func (k Keeper) FalsifyClaim(
	ctx sdk.Context,
	claimId string,
	challenger string,
) (*types.ClawbackRecord, error) {
	schedule, found := k.GetVestingByClaimId(ctx, claimId)
	if !found {
		return nil, types.ErrScheduleNotFound
	}

	if schedule.Status == string(types.VestingStatusFalsified) {
		return nil, types.ErrAlreadyFalsified
	}

	params := k.GetParams(ctx)
	height := uint64(ctx.BlockHeight())

	releasedBig := new(big.Int)
	releasedBig.SetString(schedule.ReleasedAmount, 10)

	// Released clawback: 33% of already released
	releasedClawback := new(big.Int).Mul(releasedBig, big.NewInt(int64(params.ReleasedClawbackRate)))
	releasedClawback.Div(releasedClawback, big.NewInt(10000))

	// Unvested = total - released - reserve
	totalBig := new(big.Int)
	totalBig.SetString(schedule.TotalAmount, 10)
	reserveBig := new(big.Int)
	reserveBig.SetString(schedule.ReserveAmount, 10)

	unvested := new(big.Int).Sub(totalBig, releasedBig)
	unvested.Sub(unvested, reserveBig)
	if unvested.Sign() < 0 {
		unvested = big.NewInt(0)
	}

	// Challenger reward = released clawback + unvested + reserve
	challengerReward := new(big.Int).Add(releasedClawback, unvested)
	challengerReward.Add(challengerReward, reserveBig)

	idInput := fmt.Sprintf("clawback:%s:%d", claimId, height)
	hash := sha256.Sum256([]byte(idInput))
	recordId := fmt.Sprintf("%x", hash[:16])

	record := &types.ClawbackRecord{
		Id:                recordId,
		VestingId:         schedule.Id,
		ReleasedClawback:  releasedClawback.String(),
		UnvestedForfeited: unvested.String(),
		ReserveForfeited:  reserveBig.String(),
		ChallengerReward:  challengerReward.String(),
		BlockHeight:       height,
	}

	schedule.Status = string(types.VestingStatusFalsified)
	schedule.UpdatedAt = height
	k.SetVestingSchedule(ctx, schedule)

	k.SetClawbackRecord(ctx, record)

	k.Logger(ctx).Info("falsified claim, clawback executed",
		"claim_id", claimId,
		"vesting_id", schedule.Id,
		"challenger", challenger,
		"challenger_reward", challengerReward.String(),
	)

	return record, nil
}
