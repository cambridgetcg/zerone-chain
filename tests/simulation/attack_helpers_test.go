package simulation_test

// ============================================================================
// Protocol constants (from x/knowledge/types, x/staking/types, x/alignment,
// x/autopoiesis, x/tree, x/research genesis defaults)
// ============================================================================

const (
	bpsBasis = uint64(1_000_000) // 100% in basis points

	// FARM-1: Rubber-stamp verifier detection
	conformityThresholdBps  = uint64(950_000) // >95% conformity triggers penalty
	conformityMinSamples    = uint64(20)      // need 20+ votes before tracking
	conformityPenaltyMaxBps = uint64(200_000) // up to 20% reward reduction

	// FARM-2: Trivial claim detection
	calibrationTrivialThreshold  = uint64(950_000) // >95% acceptance rate = suspicious
	maxCalibrationTrivialPenalty = uint64(100_000)  // -10% confidence penalty
	calibrationNeutralRate       = uint64(700_000)  // 70% acceptance = neutral
	maxCalibrationBonus          = uint64(50_000)   // +5% max
	maxCalibrationPenalty        = uint64(150_000)   // -15% max
	calibrationScalingFactor     = uint64(5)

	// FARM-6: Misbehavior vesting pause
	misbehaviorRejectionThreshold = uint64(300_000) // 30% rejection rate
	misbehaviorMinSamples         = uint32(10)

	// FARM-7: Domain squatting prevention
	minDomainContributorsForNovelty = uint64(3)

	// FARM-9: Proportional challenge economics
	challengeStakeRatioMinBps = uint64(500_000)   // must stake >=50% of claim stake
	challengeRewardCapBps     = uint64(1_000_000) // reward capped at 100% of challenger's stake

	// FARM-10: Semantic novelty floor
	minInformationGainBps = uint64(250_000) // 25%
	restatingPenaltyBps   = uint64(200_000) // 20%

	// Reward / slash parameters (from genesis.go defaults)
	verificationRewardBase       = uint64(3_000_000)  // 3 ZRN in uzrn
	invalidClaimSlashBps         = uint64(220_000)     // 22% of claim stake
	successfulChallengeRewardBps = uint64(300_000)     // 30% of slashed amount
	failedChallengeSlashBps      = uint64(220_000)     // 22% slash on failed challenge

	// Tier multipliers (from confidence.go applyTierMultiplier)
	tierApprenticeMultiplier = uint64(100_000)   // 0.1x
	tierVerifiedMultiplier   = uint64(500_000)   // 0.5x
	tierBondedMultiplier     = uint64(1_000_000) // 1.0x
	tierGuardianMultiplier   = uint64(2_000_000) // 2.0x

	// Staking requirements
	apprenticeVirtualStake = uint64(11_000_000)    // 11 ZRN in uzrn
	bondedMinStake         = uint64(1_111_000_000) // 1,111 ZRN
	guardianMinStake       = uint64(11_111_000_000) // 11,111 ZRN

	// ---- Alignment (x/alignment) ----
	alignmentWeightPerDimension = uint64(200_000) // 20% each, 5 dimensions
	alignmentCriticalThreshold  = uint64(200_000) // 20%
	alignmentDegradedThreshold  = uint64(400_000) // (< HealthyThreshold)
	alignmentHealthyThreshold   = uint64(700_000) // 70%

	// ---- Autopoiesis (x/autopoiesis) ----
	autopoiesisMaxChangePerEpoch = uint64(10_000)    // 1% per epoch
	autopoiesisSlashMultMin      = uint64(500_000)   // 0.5x floor
	autopoiesisSlashMultMax      = uint64(2_000_000) // 2.0x ceiling

	// ---- Tree revenue (x/tree) ----
	treeContributorsBp     = uint32(550_000) // 55%
	treeProtocolTreasuryBp = uint32(220_000) // 22%
	treeResearchFundBp     = uint32(33_300)  // 3.33%
	treeDevelopmentBp      = uint32(196_700) // 19.67%

	// ---- Research (x/research) ----
	researchMinStake              = uint64(1_000_000)  // 1 ZRN in uzrn
	researchMinReviewerCount      = uint32(3)
	researchAcceptanceThreshold   = uint32(70)          // 70/100
	researchRejectionSlashBps     = uint64(330_000)     // 33%
)

// ============================================================================
// Simulation helpers — pure math, no keepers
// ============================================================================

// safeMulDiv computes (a * b) / c avoiding overflow for reasonable uint64 values.
func safeMulDiv(a, b, c uint64) uint64 {
	if c == 0 {
		return 0
	}
	return (a * b) / c
}

// applyTierMultiplier scales a base reward by tier.
func applyTierMultiplier(base uint64, tier int) uint64 {
	switch tier {
	case 0:
		return base / 10 // 0.1x
	case 1:
		return base / 2 // 0.5x
	case 2:
		return base // 1.0x
	case 3:
		return base * 2 // 2.0x
	default:
		return base
	}
}

// conformityPenalty returns the reward penalty BPS for a given conformity rate.
func conformityPenalty(conformingVotes, totalVotes uint64) uint64 {
	if totalVotes < conformityMinSamples {
		return 0
	}
	conformityRate := (conformingVotes * bpsBasis) / totalVotes
	if conformityRate <= conformityThresholdBps {
		return 0
	}
	excess := conformityRate - conformityThresholdBps
	penaltyRange := bpsBasis - conformityThresholdBps
	if penaltyRange == 0 {
		return conformityPenaltyMaxBps
	}
	penalty := (excess * conformityPenaltyMaxBps) / penaltyRange
	if penalty > conformityPenaltyMaxBps {
		penalty = conformityPenaltyMaxBps
	}
	return penalty
}

// calibrationAdjustment returns the confidence adjustment for a submitter's acceptance rate.
func calibrationAdjustment(accepted, total uint64) int64 {
	if total < uint64(misbehaviorMinSamples) {
		return 0
	}
	acceptanceRate := (accepted * bpsBasis) / total

	if acceptanceRate > calibrationTrivialThreshold {
		excess := int64(acceptanceRate) - int64(calibrationTrivialThreshold)
		adj := -(excess * int64(maxCalibrationTrivialPenalty)) / 50_000
		if adj < -int64(maxCalibrationTrivialPenalty) {
			adj = -int64(maxCalibrationTrivialPenalty)
		}
		return adj
	}

	deviation := int64(acceptanceRate) - int64(calibrationNeutralRate)
	adj := deviation / int64(calibrationScalingFactor)
	if adj > int64(maxCalibrationBonus) {
		adj = int64(maxCalibrationBonus)
	}
	if adj < -int64(maxCalibrationPenalty) {
		adj = -int64(maxCalibrationPenalty)
	}
	return adj
}

// isMisbehaviorPaused returns true if rejection rate exceeds the FARM-6 threshold.
func isMisbehaviorPaused(rejected, total uint64) bool {
	if total < uint64(misbehaviorMinSamples) {
		return false
	}
	rejectionRate := (rejected * bpsBasis) / total
	return rejectionRate > misbehaviorRejectionThreshold
}

// domainNoveltyScale returns the fraction (BPS) of novelty bonus allowed
// given the number of unique contributors.
func domainNoveltyScale(contributors uint64) uint64 {
	if contributors >= minDomainContributorsForNovelty {
		return bpsBasis // full bonus
	}
	return (contributors * bpsBasis) / minDomainContributorsForNovelty
}

// challengeMinStake returns the minimum stake a challenger needs for a given claim stake.
func challengeMinStake(claimStake uint64) uint64 {
	return safeMulDiv(claimStake, challengeStakeRatioMinBps, bpsBasis)
}

// challengeReward returns the capped reward from a successful challenge.
func challengeReward(claimStake, challengerStake uint64) uint64 {
	slashAmount := safeMulDiv(claimStake, invalidClaimSlashBps, bpsBasis)
	reward := safeMulDiv(slashAmount, successfulChallengeRewardBps, bpsBasis)
	maxReward := safeMulDiv(challengerStake, challengeRewardCapBps, bpsBasis)
	if reward > maxReward {
		reward = maxReward
	}
	return reward
}

// ---- Alignment / autopoiesis helpers ----

// alignmentComposite computes the weighted composite score from 5 dimension values.
// Each dimension is weighted at alignmentWeightPerDimension (20%).
func alignmentComposite(knowledge, economic, governance, security, staking uint64) uint64 {
	return (knowledge*alignmentWeightPerDimension +
		economic*alignmentWeightPerDimension +
		governance*alignmentWeightPerDimension +
		security*alignmentWeightPerDimension +
		staking*alignmentWeightPerDimension) / bpsBasis
}

// alignmentCategory returns the health category for a composite score.
func alignmentCategory(composite uint64) string {
	if composite < alignmentCriticalThreshold {
		return "critical"
	}
	if composite < alignmentHealthyThreshold {
		return "degraded"
	}
	return "healthy"
}

// autopoiesisClampChange clamps a multiplier change to MaxChangePerEpochBps.
func autopoiesisClampChange(currentBps, targetBps uint64) uint64 {
	if targetBps > currentBps {
		delta := targetBps - currentBps
		if delta > autopoiesisMaxChangePerEpoch {
			return currentBps + autopoiesisMaxChangePerEpoch
		}
		return targetBps
	}
	delta := currentBps - targetBps
	if delta > autopoiesisMaxChangePerEpoch {
		return currentBps - autopoiesisMaxChangePerEpoch
	}
	return targetBps
}

// autopoiesisClampMultiplier clamps a multiplier within the allowed range.
func autopoiesisClampMultiplier(bps uint64) uint64 {
	if bps < autopoiesisSlashMultMin {
		return autopoiesisSlashMultMin
	}
	if bps > autopoiesisSlashMultMax {
		return autopoiesisSlashMultMax
	}
	return bps
}

// ---- Tree revenue helpers ----

// treeRevenueDistribution models the tree module's revenue split.
type treeRevenueDistribution struct {
	contributorPool  int64
	researchFund     int64
	protocolTreasury int64
	developmentFund  int64
}

// calculateTreeRevenue computes revenue split per tree module params.
func calculateTreeRevenue(totalRevenue int64) treeRevenueDistribution {
	if totalRevenue <= 0 {
		return treeRevenueDistribution{}
	}
	const bpDenom = int64(1_000_000)
	return treeRevenueDistribution{
		contributorPool:  totalRevenue * int64(treeContributorsBp) / bpDenom,
		researchFund:     totalRevenue * int64(treeResearchFundBp) / bpDenom,
		protocolTreasury: totalRevenue * int64(treeProtocolTreasuryBp) / bpDenom,
		developmentFund:  totalRevenue * int64(treeDevelopmentBp) / bpDenom,
	}
}

// ---- Research helpers ----

// researchProposalCost returns the total cost of submitting N research proposals.
func researchProposalCost(n uint64) uint64 {
	return n * researchMinStake
}

// researchReviewAccepted returns true if average score >= acceptance threshold.
func researchReviewAccepted(totalScore, reviewCount uint32) bool {
	if reviewCount < researchMinReviewerCount {
		return false
	}
	avgScore := totalScore / reviewCount
	return avgScore >= researchAcceptanceThreshold
}

// researchRejectionSlash computes the slash on a rejected research proposal.
func researchRejectionSlash(stake uint64) uint64 {
	return safeMulDiv(stake, researchRejectionSlashBps, bpsBasis)
}
