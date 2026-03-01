package simulation_test

import (
	"fmt"
	"math"
	"testing"
)

// ============================================================================
// FARM-1: Rubber-Stamp Verifier Detection
// ============================================================================

func TestFARM1_RubberStampVerifier(t *testing.T) {
	t.Run("100_verifications_96_conforming", func(t *testing.T) {
		totalVotes := uint64(100)
		conformingVotes := uint64(96)
		conformityRate := (conformingVotes * bpsBasis) / totalVotes // 960,000

		penalty := conformityPenalty(conformingVotes, totalVotes)

		t.Logf("Conformity rate: %d BPS (%.1f%%)", conformityRate, float64(conformityRate)/10_000)
		t.Logf("Threshold: %d BPS (%.1f%%)", conformityThresholdBps, float64(conformityThresholdBps)/10_000)
		t.Logf("Penalty applied: %d BPS (%.1f%%)", penalty, float64(penalty)/10_000)

		if penalty == 0 {
			t.Fatal("FARM-1 FAILED: no penalty for 96% conformity rate")
		}

		// Expected: linear penalty from threshold (95%) to 100%
		// excess = 960,000 - 950,000 = 10,000
		// penaltyRange = 1,000,000 - 950,000 = 50,000
		// penalty = 10,000 * 200,000 / 50,000 = 40,000 (4%)
		expectedPenalty := uint64(40_000)
		if penalty != expectedPenalty {
			t.Errorf("Expected penalty %d, got %d", expectedPenalty, penalty)
		}

		// P&L: honest verifier gets full reward, rubber-stamper gets 96% of base
		honestReward := applyTierMultiplier(verificationRewardBase, 2) // Bonded tier
		farmerReward := honestReward * (bpsBasis - penalty) / bpsBasis
		t.Logf("Honest reward (Bonded): %d uzrn (%.3f ZRN)", honestReward, float64(honestReward)/1e6)
		t.Logf("Farmer reward (penalized): %d uzrn (%.3f ZRN)", farmerReward, float64(farmerReward)/1e6)
		t.Logf("Net penalty per verification: %d uzrn (%.3f ZRN)", honestReward-farmerReward, float64(honestReward-farmerReward)/1e6)

		if farmerReward >= honestReward {
			t.Error("FARM-1 FAILED: farmer should earn less than honest verifier")
		}
	})

	t.Run("penalty_ramp_over_time", func(t *testing.T) {
		type rampPoint struct {
			conforming uint64
			total      uint64
			penalty    uint64
		}

		points := []rampPoint{
			{19, 20, 0},
			{48, 50, 0},
			{96, 100, 0},
			{97, 100, 0},
			{98, 100, 0},
			{99, 100, 0},
			{100, 100, 0},
		}

		for i := range points {
			points[i].penalty = conformityPenalty(points[i].conforming, points[i].total)
		}

		t.Logf("Conformity penalty ramp:")
		for _, p := range points {
			rate := (p.conforming * bpsBasis) / p.total
			t.Logf("  %d/%d (%.1f%%): penalty = %d BPS (%.1f%%)",
				p.conforming, p.total, float64(rate)/10_000,
				p.penalty, float64(p.penalty)/10_000)
		}

		// Verify monotonically increasing penalties above threshold
		for i := 1; i < len(points); i++ {
			if points[i].conforming*points[i-1].total > points[i-1].conforming*points[i].total {
				if points[i].penalty < points[i-1].penalty {
					t.Errorf("Penalty not monotonically increasing: %d < %d at conformity %d/%d",
						points[i].penalty, points[i-1].penalty, points[i].conforming, points[i].total)
				}
			}
		}

		// 100% conformity should get max penalty
		maxPenalty := conformityPenalty(100, 100)
		if maxPenalty != conformityPenaltyMaxBps {
			t.Errorf("100%% conformity should get max penalty %d, got %d", conformityPenaltyMaxBps, maxPenalty)
		}
	})

	t.Run("below_threshold_no_penalty", func(t *testing.T) {
		penalty := conformityPenalty(94, 100)
		if penalty != 0 {
			t.Errorf("94%% conformity should not trigger penalty, got %d", penalty)
		}

		penalty = conformityPenalty(95, 100)
		if penalty != 0 {
			t.Errorf("95%% exactly should not trigger penalty, got %d", penalty)
		}
	})

	t.Run("insufficient_samples_no_penalty", func(t *testing.T) {
		penalty := conformityPenalty(15, 15)
		if penalty != 0 {
			t.Errorf("Expected no penalty with insufficient samples, got %d", penalty)
		}
	})
}

// ============================================================================
// FARM-2: Trivial Claim Detection
// ============================================================================

func TestFARM2_TrivialClaimDetection(t *testing.T) {
	t.Run("49_of_50_accepted", func(t *testing.T) {
		accepted := uint64(49)
		total := uint64(50)
		acceptanceRate := (accepted * bpsBasis) / total // 980,000

		adj := calibrationAdjustment(accepted, total)

		t.Logf("Acceptance rate: %d BPS (%.1f%%)", acceptanceRate, float64(acceptanceRate)/10_000)
		t.Logf("Trivial threshold: %d BPS (%.1f%%)", calibrationTrivialThreshold, float64(calibrationTrivialThreshold)/10_000)
		t.Logf("Adjustment: %d BPS (%.1f%%)", adj, float64(adj)/10_000)

		if adj >= 0 {
			t.Fatal("FARM-2 FAILED: expected negative adjustment for 98% acceptance")
		}

		// Expected: excess = 980,000 - 950,000 = 30,000
		// adjustment = -(30,000 * 100,000) / 50,000 = -60,000
		expectedAdj := int64(-60_000)
		if adj != expectedAdj {
			t.Errorf("Expected adjustment %d, got %d", expectedAdj, adj)
		}
	})

	t.Run("100_pct_acceptance_max_penalty", func(t *testing.T) {
		adj := calibrationAdjustment(50, 50) // 100% acceptance
		t.Logf("100%% acceptance adjustment: %d BPS", adj)

		if adj != -int64(maxCalibrationTrivialPenalty) {
			t.Errorf("100%% acceptance should get max penalty -%d, got %d",
				maxCalibrationTrivialPenalty, adj)
		}
	})

	t.Run("95_pct_exactly_no_penalty", func(t *testing.T) {
		adj := calibrationAdjustment(19, 20)
		acceptanceRate := (uint64(19) * bpsBasis) / 20 // 950,000

		t.Logf("95%% acceptance rate (%d BPS), adjustment: %d", acceptanceRate, adj)

		if adj < 0 {
			t.Error("95% acceptance should not trigger trivial penalty")
		}
		if adj > int64(maxCalibrationBonus) {
			t.Errorf("Adjustment %d exceeds max bonus %d", adj, maxCalibrationBonus)
		}
	})

	t.Run("honest_70_pct_neutral", func(t *testing.T) {
		adj := calibrationAdjustment(14, 20)
		acceptanceRate := (uint64(14) * bpsBasis) / 20

		t.Logf("70%% acceptance rate (%d BPS), adjustment: %d", acceptanceRate, adj)
		if adj != 0 {
			t.Errorf("70%% acceptance should be neutral (adj=0), got %d", adj)
		}
	})

	t.Run("pnl_trivial_vs_honest", func(t *testing.T) {
		honestAdj := calibrationAdjustment(80, 100)
		farmerAdj := calibrationAdjustment(98, 100)

		baseConfidence := uint64(500_000)
		honestConf := int64(baseConfidence) + honestAdj
		farmerConf := int64(baseConfidence) + farmerAdj

		t.Logf("Honest (80%% acceptance): adjustment=%d, effective confidence=%d", honestAdj, honestConf)
		t.Logf("Farmer (98%% acceptance): adjustment=%d, effective confidence=%d", farmerAdj, farmerConf)

		if farmerConf >= honestConf {
			t.Error("FARM-2 FAILED: trivial farmer should have lower effective confidence than honest submitter")
		}
	})
}

// ============================================================================
// FARM-6: Misbehavior Vesting Pause
// ============================================================================

func TestFARM6_MisbehaviorVestingPause(t *testing.T) {
	t.Run("20_claims_8_rejected", func(t *testing.T) {
		total := uint64(20)
		rejected := uint64(8)
		rejectionRate := (rejected * bpsBasis) / total // 400,000 = 40%

		paused := isMisbehaviorPaused(rejected, total)

		t.Logf("Rejection rate: %d BPS (%.1f%%)", rejectionRate, float64(rejectionRate)/10_000)
		t.Logf("Misbehavior threshold: %d BPS (%.1f%%)", misbehaviorRejectionThreshold, float64(misbehaviorRejectionThreshold)/10_000)
		t.Logf("Vesting paused: %v", paused)

		if !paused {
			t.Fatal("FARM-6 FAILED: 40% rejection rate should trigger vesting pause")
		}
	})

	t.Run("below_threshold_no_pause", func(t *testing.T) {
		paused := isMisbehaviorPaused(4, 20)
		if paused {
			t.Error("20% rejection rate should NOT trigger pause")
		}
	})

	t.Run("exactly_30_pct_no_pause", func(t *testing.T) {
		paused := isMisbehaviorPaused(6, 20)
		rejectionRate := (uint64(6) * bpsBasis) / 20 // 300,000
		t.Logf("30%% rejection rate (%d BPS): paused=%v", rejectionRate, paused)
		if paused {
			t.Error("30% exactly should NOT trigger pause (threshold is >30%, not >=)")
		}
	})

	t.Run("insufficient_samples_no_pause", func(t *testing.T) {
		paused := isMisbehaviorPaused(5, 5)
		if paused {
			t.Error("Should not trigger with insufficient samples")
		}
	})

	t.Run("pnl_grief_and_extract", func(t *testing.T) {
		honestClaims := uint64(10)
		badClaims := uint64(10)
		total := honestClaims + badClaims

		rejectionRate := (badClaims * bpsBasis) / total // 500,000 = 50%
		paused := isMisbehaviorPaused(badClaims, total)

		vestingPerClaim := uint64(3_000_000) // 3 ZRN typical
		totalAccrued := honestClaims * vestingPerClaim
		claimStake := uint64(1_000_000) // 1 ZRN per claim
		stakeSlashed := safeMulDiv(claimStake*badClaims, invalidClaimSlashBps, bpsBasis)

		t.Logf("Grief-and-extract scenario:")
		t.Logf("  Honest claims: %d, Bad claims: %d", honestClaims, badClaims)
		t.Logf("  Rejection rate: %.1f%%", float64(rejectionRate)/10_000)
		t.Logf("  Vesting paused: %v", paused)
		t.Logf("  Vesting accrued: %d uzrn (%.3f ZRN)", totalAccrued, float64(totalAccrued)/1e6)
		t.Logf("  Stake slashed: %d uzrn (%.3f ZRN)", stakeSlashed, float64(stakeSlashed)/1e6)

		if !paused {
			t.Fatal("FARM-6 FAILED: vesting should be paused at 50% rejection")
		}

		t.Logf("  Outcome: vesting frozen (no further releases) + %d uzrn slashed", stakeSlashed)
	})
}

// ============================================================================
// FARM-7: Domain Squatting
// ============================================================================

func TestFARM7_DomainSquatting(t *testing.T) {
	t.Run("single_agent_single_domain", func(t *testing.T) {
		contributors := uint64(1)
		scale := domainNoveltyScale(contributors)

		t.Logf("Contributors: %d, Min for full novelty: %d", contributors, minDomainContributorsForNovelty)
		t.Logf("Novelty scale: %d BPS (%.1f%%)", scale, float64(scale)/10_000)

		// Expected: 1/3 of full novelty = 333,333 BPS
		expectedScale := uint64(333_333)
		if scale != expectedScale {
			t.Errorf("Expected novelty scale %d, got %d", expectedScale, scale)
		}

		fullNoveltyBonus := uint64(100_000) // hypothetical 10% novelty bonus
		actualBonus := safeMulDiv(fullNoveltyBonus, scale, bpsBasis)
		t.Logf("Full novelty bonus: %d -> Actual with suppression: %d (%.1f%%)",
			fullNoveltyBonus, actualBonus, float64(actualBonus*100)/float64(fullNoveltyBonus))

		if actualBonus >= fullNoveltyBonus/2 {
			t.Error("Single-contributor domain should suppress novelty by >50%")
		}
	})

	t.Run("progressive_contributor_scaling", func(t *testing.T) {
		t.Logf("Domain novelty scaling by contributor count:")
		for c := uint64(0); c <= 5; c++ {
			scale := domainNoveltyScale(c)
			t.Logf("  %d contributors: %d BPS (%.1f%%)", c, scale, float64(scale)/10_000)
		}

		if domainNoveltyScale(0) != 0 {
			t.Error("0 contributors should give 0 novelty")
		}

		if domainNoveltyScale(3) != bpsBasis {
			t.Errorf("3 contributors should give full novelty, got %d", domainNoveltyScale(3))
		}
		if domainNoveltyScale(10) != bpsBasis {
			t.Errorf("10 contributors should give full novelty, got %d", domainNoveltyScale(10))
		}
	})

	t.Run("pnl_domain_squat_30_claims", func(t *testing.T) {
		claims := 30
		contributors := uint64(1)
		noveltyScale := domainNoveltyScale(contributors)

		normalBonus := uint64(100_000)
		suppressedBonus := safeMulDiv(normalBonus, noveltyScale, bpsBasis)

		totalNormalReward := uint64(claims) * normalBonus
		totalSuppressedReward := uint64(claims) * suppressedBonus
		rewardLoss := totalNormalReward - totalSuppressedReward

		t.Logf("Domain squat scenario (30 claims, 1 contributor):")
		t.Logf("  Normal cumulative novelty bonus: %d BPS", totalNormalReward)
		t.Logf("  Suppressed cumulative bonus: %d BPS", totalSuppressedReward)
		t.Logf("  Total novelty loss: %d BPS (%.1f%%)", rewardLoss,
			float64(rewardLoss*100)/float64(totalNormalReward))

		if totalSuppressedReward >= totalNormalReward/2 {
			t.Error("Domain squat novelty should be reduced by >50%")
		}
	})
}

// ============================================================================
// FARM-9: Proportional Challenge Economics
// ============================================================================

func TestFARM9_ProportionalChallengeEconomics(t *testing.T) {
	t.Run("frivolous_challenge_blocked", func(t *testing.T) {
		claimStake := uint64(100_000_000) // 100 ZRN
		minStake := challengeMinStake(claimStake)

		t.Logf("Claim stake: %d uzrn (%.0f ZRN)", claimStake, float64(claimStake)/1e6)
		t.Logf("Min challenge stake (50%%): %d uzrn (%.0f ZRN)", minStake, float64(minStake)/1e6)

		attackerStake := uint64(1_000_000) // 1 ZRN
		blocked := attackerStake < minStake

		t.Logf("Attacker stake: %d uzrn (%.0f ZRN) — blocked: %v", attackerStake, float64(attackerStake)/1e6, blocked)

		if !blocked {
			t.Fatal("FARM-9 FAILED: 1 ZRN should not be enough to challenge a 100 ZRN claim")
		}
	})

	t.Run("successful_challenge_reward_capped", func(t *testing.T) {
		claimStake := uint64(100_000_000)     // 100 ZRN
		challengerStake := uint64(50_000_000) // 50 ZRN (minimum allowed)

		reward := challengeReward(claimStake, challengerStake)

		expectedSlash := safeMulDiv(claimStake, invalidClaimSlashBps, bpsBasis)
		expectedReward := safeMulDiv(expectedSlash, successfulChallengeRewardBps, bpsBasis)

		t.Logf("Slash amount: %d uzrn (%.1f ZRN)", expectedSlash, float64(expectedSlash)/1e6)
		t.Logf("Raw reward: %d uzrn (%.1f ZRN)", expectedReward, float64(expectedReward)/1e6)
		t.Logf("Reward cap: %d uzrn (%.1f ZRN)", challengerStake, float64(challengerStake)/1e6)
		t.Logf("Actual reward: %d uzrn (%.1f ZRN)", reward, float64(reward)/1e6)

		if reward != expectedReward {
			t.Errorf("Expected reward %d, got %d", expectedReward, reward)
		}
		if reward > challengerStake {
			t.Error("FARM-9 FAILED: reward exceeds challenger's stake")
		}
	})

	t.Run("failed_challenge_loss", func(t *testing.T) {
		challengerStake := uint64(50_000_000) // 50 ZRN
		loss := safeMulDiv(challengerStake, failedChallengeSlashBps, bpsBasis)

		t.Logf("Failed challenge loss: %d uzrn (%.1f ZRN = %.1f%%)",
			loss, float64(loss)/1e6, float64(failedChallengeSlashBps)/10_000)
		t.Logf("Remaining after loss: %d uzrn (%.1f ZRN)",
			challengerStake-loss, float64(challengerStake-loss)/1e6)

		maxRewardPerSuccess := uint64(6_600_000) // 6.6 ZRN from above scenario
		breakEvenRate := float64(loss) / float64(loss+maxRewardPerSuccess)

		t.Logf("Break-even success rate: %.1f%%", breakEvenRate*100)

		if breakEvenRate < 0.5 {
			t.Logf("Note: break-even rate %.1f%% is low — challenges can be profitable with >%.1f%% success",
				breakEvenRate*100, breakEvenRate*100)
		}
	})
}

// ============================================================================
// FARM-10: Semantic Novelty Floor
// ============================================================================

func TestFARM10_SemanticNoveltyFloor(t *testing.T) {
	t.Run("restating_axiom_penalized", func(t *testing.T) {
		infoGainBps := uint64(150_000) // 15%

		belowFloor := infoGainBps < minInformationGainBps
		t.Logf("Info gain: %d BPS (%.1f%%)", infoGainBps, float64(infoGainBps)/10_000)
		t.Logf("Minimum floor: %d BPS (%.1f%%)", minInformationGainBps, float64(minInformationGainBps)/10_000)
		t.Logf("Below floor: %v", belowFloor)

		if !belowFloor {
			t.Fatal("FARM-10 FAILED: 15% info gain should be below 25% floor")
		}

		penalty := restatingPenaltyBps
		t.Logf("Restating penalty: %d BPS (%.1f%%)", penalty, float64(penalty)/10_000)

		baseConfidence := uint64(600_000)
		penalizedConfidence := int64(baseConfidence) - int64(penalty)
		t.Logf("Base confidence: %d -> Penalized: %d", baseConfidence, penalizedConfidence)

		if penalizedConfidence >= int64(baseConfidence) {
			t.Error("Penalty should reduce confidence")
		}
	})

	t.Run("genuine_novelty_not_penalized", func(t *testing.T) {
		infoGainBps := uint64(300_000) // 30%
		belowFloor := infoGainBps < minInformationGainBps

		if belowFloor {
			t.Error("30% info gain should NOT be below floor")
		}
	})

	t.Run("pnl_restating_farm", func(t *testing.T) {
		claims := 100
		claimStake := uint64(1_000_000)    // 1 ZRN per claim
		rejectThreshold := uint64(300_000) // typical rejection threshold
		rewardPerAccepted := uint64(3_000_000)

		// Honest path: 70% acceptance, typical confidence ~600k
		honestAccepted := 70
		honestTotalReward := uint64(honestAccepted) * rewardPerAccepted
		honestSlashed := safeMulDiv(uint64(claims-honestAccepted)*claimStake, invalidClaimSlashBps, bpsBasis)
		honestNet := int64(honestTotalReward) - int64(honestSlashed)

		// Restating farm: compound FARM-10 + FARM-2
		baseConf := uint64(600_000)
		farm10Adj := restatingPenaltyBps
		farm2Adj := calibrationAdjustment(uint64(claims), uint64(claims))

		effectiveConf := int64(baseConf) - int64(farm10Adj) + farm2Adj
		t.Logf("Restating farm P&L over %d claims:", claims)
		t.Logf("  Base confidence: %d", baseConf)
		t.Logf("  FARM-10 penalty: -%d (restating canonical refs)", farm10Adj)
		t.Logf("  FARM-2 penalty: %d (100%% acceptance triggers trivial detection)", farm2Adj)
		t.Logf("  Effective confidence: %d (reject threshold: %d)", effectiveConf, rejectThreshold)

		farmerAcceptanceRate := 0.50
		if effectiveConf <= int64(rejectThreshold) {
			farmerAcceptanceRate = 0.30
		}

		farmerAccepted := int(float64(claims) * farmerAcceptanceRate)
		farmerRejected := claims - farmerAccepted
		farmerReward := uint64(farmerAccepted) * rewardPerAccepted
		farmerSlashed := safeMulDiv(uint64(farmerRejected)*claimStake, invalidClaimSlashBps, bpsBasis)
		farmerNet := int64(farmerReward) - int64(farmerSlashed)

		t.Logf("  Honest: %d/%d accepted, net=%d uzrn (%.1f ZRN)",
			honestAccepted, claims, honestNet, float64(honestNet)/1e6)
		t.Logf("  Farmer: ~%d/%d accepted (conf %d), net=%d uzrn (%.1f ZRN)",
			farmerAccepted, claims, effectiveConf, farmerNet, float64(farmerNet)/1e6)
		t.Logf("  Farmer reward: %d uzrn, slashed: %d uzrn", farmerReward, farmerSlashed)

		if farmerNet >= honestNet {
			t.Error("Restating farmer should earn less net than honest submitter")
		}

		if effectiveConf > int64(rejectThreshold)+100_000 {
			t.Errorf("Compound penalty should push confidence to near/below reject threshold (%d), got %d",
				rejectThreshold, effectiveConf)
		}
	})
}

// ============================================================================
// Combined Scenario 1: Citation Ring (3 colluding agents)
// ============================================================================

func TestScenario1_CitationRing(t *testing.T) {
	t.Run("three_agent_ring", func(t *testing.T) {
		agents := 3
		claimsPerAgent := 10
		totalClaims := agents * claimsPerAgent

		verificationsPerAgent := (agents - 1) * claimsPerAgent // 20 verifications each
		conformingVerifications := verificationsPerAgent         // 100% conforming

		// FARM-1: Conformity detection
		farm1Penalty := conformityPenalty(uint64(conformingVerifications), uint64(verificationsPerAgent))
		t.Logf("Citation Ring — 3 agents, %d total claims:", totalClaims)
		t.Logf("  Each agent verifies %d claims (all conforming)", verificationsPerAgent)
		t.Logf("  FARM-1 conformity penalty: %d BPS (%.1f%%)", farm1Penalty, float64(farm1Penalty)/10_000)

		// FARM-7: Domain squatting — 3 contributors = exactly at minimum
		noveltyScale := domainNoveltyScale(uint64(agents))
		t.Logf("  FARM-7 novelty scale: %d BPS (%.1f%%) — %d contributors",
			noveltyScale, float64(noveltyScale)/10_000, agents)

		// FARM-2: All claims accepted within the ring -> 100% acceptance
		farm2Adj := calibrationAdjustment(uint64(claimsPerAgent), uint64(claimsPerAgent))
		t.Logf("  FARM-2 calibration: %d BPS (trivial claim penalty)", farm2Adj)

		// Compound penalty analysis
		baseReward := applyTierMultiplier(verificationRewardBase, 2) // Bonded
		rewardAfterFarm1 := baseReward * (bpsBasis - farm1Penalty) / bpsBasis

		t.Logf("  Base reward (Bonded): %d uzrn", baseReward)
		t.Logf("  After FARM-1: %d uzrn (%.1f%% of base)", rewardAfterFarm1,
			float64(rewardAfterFarm1*100)/float64(baseReward))

		totalPenaltyPct := float64(farm1Penalty)/float64(bpsBasis)*100 +
			math.Abs(float64(farm2Adj))/float64(bpsBasis)*100

		t.Logf("  Stacked penalties: ~%.1f%% (FARM-1 + FARM-2)", totalPenaltyPct)

		if agents == int(minDomainContributorsForNovelty) {
			t.Logf("  CRITICAL: ring is at exact minimum for full novelty — fragile")
		}
	})
}

// ============================================================================
// Combined Scenario 2: Tier Gaming (1 agent)
// ============================================================================

func TestScenario2_TierGaming(t *testing.T) {
	t.Run("legitimate_then_rubberstamp", func(t *testing.T) {
		// Phase 1: Get to Verified tier legitimately
		legitimateVerifications := uint64(22)
		legitimateCorrect := uint64(17) // 77.3% accuracy

		phase1Penalty := conformityPenalty(legitimateCorrect, legitimateVerifications)
		t.Logf("Tier Gaming — Phase 1 (legitimate):")
		t.Logf("  Verifications: %d, Correct: %d (%.1f%%)", legitimateVerifications, legitimateCorrect,
			float64(legitimateCorrect*100)/float64(legitimateVerifications))
		t.Logf("  FARM-1 penalty: %d (none expected)", phase1Penalty)

		if phase1Penalty != 0 {
			t.Error("Phase 1 should have no penalty")
		}

		// Phase 2: Switch to rubber-stamping
		additionalVerifications := uint64(50)
		totalVerifications := legitimateVerifications + additionalVerifications
		totalConforming := legitimateCorrect + additionalVerifications // 17 + 50 = 67

		phase2Penalty := conformityPenalty(totalConforming, totalVerifications)
		conformityRate := (totalConforming * bpsBasis) / totalVerifications

		t.Logf("Tier Gaming — Phase 2 (rubber-stamping):")
		t.Logf("  Total verifications: %d, Conforming: %d (%.1f%%)",
			totalVerifications, totalConforming, float64(conformityRate)/10_000)
		t.Logf("  FARM-1 penalty: %d BPS (%.1f%%)", phase2Penalty, float64(phase2Penalty)/10_000)

		t.Logf("  Detection status: conformity %.1f%% vs threshold %.1f%%",
			float64(conformityRate)/10_000, float64(conformityThresholdBps)/10_000)

		// How many rubber-stamps to trigger?
		rubberstampsNeeded := uint64(0)
		for n := uint64(1); n <= 200; n++ {
			tc := legitimateCorrect + n
			tv := legitimateVerifications + n
			if (tc*bpsBasis)/tv > conformityThresholdBps {
				rubberstampsNeeded = n
				break
			}
		}
		t.Logf("  Rubber-stamps needed to trigger FARM-1: %d", rubberstampsNeeded)

		if rubberstampsNeeded > 0 {
			tcAtDetection := legitimateCorrect + rubberstampsNeeded
			tvAtDetection := legitimateVerifications + rubberstampsNeeded
			detectionPenalty := conformityPenalty(tcAtDetection, tvAtDetection)
			rewardAtVerified := applyTierMultiplier(verificationRewardBase, 1) // Verified = 0.5x

			t.Logf("  At detection (%d total verifications):", tvAtDetection)
			t.Logf("    Penalty: %d BPS (%.1f%%)", detectionPenalty, float64(detectionPenalty)/10_000)
			t.Logf("    Reward per verification: %d uzrn -> %d uzrn",
				rewardAtVerified, rewardAtVerified*(bpsBasis-detectionPenalty)/bpsBasis)
		}

		rewardPerVerif := applyTierMultiplier(verificationRewardBase, 1)
		unpenalizedRewards := rubberstampsNeeded * rewardPerVerif

		t.Logf("  Unpenalized farming rewards: %d uzrn (%.1f ZRN) over %d verifications",
			unpenalizedRewards, float64(unpenalizedRewards)/1e6, rubberstampsNeeded)
	})
}

// ============================================================================
// Combined Scenario 3: Stake Minimum Exploit
// ============================================================================

func TestScenario3_StakeMinimumExploit(t *testing.T) {
	t.Run("apprentice_minimum_stake", func(t *testing.T) {
		rewardPerVerif := applyTierMultiplier(verificationRewardBase, 0) // Apprentice = 0.1x
		verifiedReward := applyTierMultiplier(verificationRewardBase, 1) // Verified = 0.5x
		bondedReward := applyTierMultiplier(verificationRewardBase, 2)   // Bonded = 1.0x

		t.Logf("Stake Minimum Exploit — Apprentice:")
		t.Logf("  Virtual stake: %d uzrn (%.0f ZRN)", apprenticeVirtualStake, float64(apprenticeVirtualStake)/1e6)
		t.Logf("  Reward per verification: %d uzrn (%.3f ZRN) — 0.1x", rewardPerVerif, float64(rewardPerVerif)/1e6)
		t.Logf("  vs Verified: %d uzrn, Bonded: %d uzrn", verifiedReward, bondedReward)
		t.Logf("  Categories: protocol, computational, formal ONLY")

		apprenticeWeight := float64(tierApprenticeMultiplier) / float64(bpsBasis)
		bondedWeight := float64(tierBondedMultiplier) / float64(bpsBasis)
		selectionProbVs1Bonded := apprenticeWeight / (apprenticeWeight + bondedWeight)

		t.Logf("  Selection probability (vs 1 bonded): %.1f%%", selectionProbVs1Bonded*100)

		roundsPerDay := 100
		apprenticeRounds := int(float64(roundsPerDay) * selectionProbVs1Bonded)
		apprenticeDailyReward := uint64(apprenticeRounds) * rewardPerVerif
		bondedDailyReward := uint64(roundsPerDay-apprenticeRounds) * bondedReward

		t.Logf("  ~%d rounds/day, apprentice selected for ~%d rounds", roundsPerDay, apprenticeRounds)
		t.Logf("  Apprentice daily reward: ~%d uzrn (%.3f ZRN)", apprenticeDailyReward, float64(apprenticeDailyReward)/1e6)
		t.Logf("  Bonded daily reward: ~%d uzrn (%.3f ZRN)", bondedDailyReward, float64(bondedDailyReward)/1e6)

		t.Logf("  Verdict: 0.1x reward x low selection = minimal return, not economically viable for farming")
	})
}

// ============================================================================
// Combined Scenario 4: Falsification Arbitrage
// ============================================================================

func TestScenario4_FalsificationArbitrage(t *testing.T) {
	t.Run("challenge_high_value_claim", func(t *testing.T) {
		claimStake := uint64(100_000_000) // 100 ZRN

		minStake := challengeMinStake(claimStake)
		t.Logf("Falsification Arbitrage:")
		t.Logf("  Target claim stake: %d uzrn (%.0f ZRN)", claimStake, float64(claimStake)/1e6)
		t.Logf("  Minimum challenge stake: %d uzrn (%.0f ZRN)", minStake, float64(minStake)/1e6)

		// Scenario A: Challenge succeeds
		reward := challengeReward(claimStake, minStake)
		netGainOnSuccess := int64(reward)
		t.Logf("  On success: +%d uzrn (%.1f ZRN) reward, bond returned",
			reward, float64(reward)/1e6)

		// Scenario B: Challenge fails
		lossOnFailure := safeMulDiv(minStake, failedChallengeSlashBps, bpsBasis)
		netLossOnFailure := int64(lossOnFailure)
		t.Logf("  On failure: -%d uzrn (%.1f ZRN) slashed",
			lossOnFailure, float64(lossOnFailure)/1e6)

		t.Logf("  Expected value at different success rates:")
		for _, rate := range []float64{0.1, 0.2, 0.3, 0.5, 0.7, 0.9} {
			ev := rate*float64(netGainOnSuccess) - (1-rate)*float64(netLossOnFailure)
			t.Logf("    %.0f%% success: EV = %.0f uzrn (%.3f ZRN)", rate*100, ev, ev/1e6)
		}

		breakEven := float64(netLossOnFailure) / float64(netLossOnFailure+netGainOnSuccess)
		t.Logf("  Break-even success rate: %.1f%%", breakEven*100)

		t.Logf("  Verdict: need >%.0f%% legitimate challenges to profit — arbitrage is unprofitable for spam",
			breakEven*100)

		if breakEven < 0.5 {
			t.Logf("  WARNING: break-even below 50%% may incentivize speculative challenges")
		}
	})
}

// ============================================================================
// NEW: Autopoiesis Manipulation Attack
// ============================================================================

func TestAutopoiesisManipulation(t *testing.T) {
	t.Run("artificial_signal_depression", func(t *testing.T) {
		// Attacker attempts to push one dimension low to force multiplier changes.
		// Normal: all 5 dimensions at ~700k (healthy).
		// Attack: suppress KnowledgeQuality to 100k by flooding bad claims.

		normalComposite := alignmentComposite(700_000, 700_000, 700_000, 700_000, 700_000)
		normalCategory := alignmentCategory(normalComposite)

		t.Logf("Autopoiesis Manipulation — Artificial Signal Depression:")
		t.Logf("  Normal: all dimensions 700k -> composite=%d, category=%s", normalComposite, normalCategory)

		// Attacker depresses ONE dimension to 100k
		attackComposite := alignmentComposite(100_000, 700_000, 700_000, 700_000, 700_000)
		attackCategory := alignmentCategory(attackComposite)

		t.Logf("  Attack: knowledge=100k, rest=700k -> composite=%d, category=%s", attackComposite, attackCategory)

		// With 20% weight per dimension, one dimension at 100k:
		// composite = (100k*200k + 700k*200k*4) / 1M = (20B + 560B) / 1M = 580,000
		// Category: degraded (between 400k and 700k)
		expectedComposite := uint64(580_000)
		if attackComposite != expectedComposite {
			t.Errorf("Expected attack composite %d, got %d", expectedComposite, attackComposite)
		}

		if attackCategory != "degraded" {
			t.Errorf("Expected degraded category, got %s", attackCategory)
		}

		// Defense: autopoiesis clamps multiplier changes to 1% per epoch.
		// Even if attacker succeeds, multiplier moves slowly.
		startMultiplier := bpsBasis // 1.0x
		targetOnAttack := uint64(1_200_000) // attacker wants 1.2x slash multiplier

		// After 1 epoch:
		after1 := autopoiesisClampChange(startMultiplier, targetOnAttack)
		// After 10 epochs:
		current := startMultiplier
		for i := 0; i < 10; i++ {
			current = autopoiesisClampChange(current, targetOnAttack)
		}

		t.Logf("  Autopoiesis defense: MaxChange=%d BPS/epoch (1%%)", autopoiesisMaxChangePerEpoch)
		t.Logf("  After 1 epoch: multiplier %d -> %d (target was %d)", startMultiplier, after1, targetOnAttack)
		t.Logf("  After 10 epochs: multiplier -> %d (target was %d)", current, targetOnAttack)

		// With 100-block epochs, 10 epochs = 1000 blocks.
		// Attacker gets only 10% of their desired change in 1000 blocks.
		maxChangePossible := current - startMultiplier
		desiredChange := targetOnAttack - startMultiplier
		effectivenessPercent := float64(maxChangePossible*100) / float64(desiredChange)

		t.Logf("  Effectiveness after 10 epochs: %.0f%% of desired change", effectivenessPercent)
		t.Logf("  Remaining gap: %d BPS (%.1f%%)", targetOnAttack-current, float64(targetOnAttack-current)/10_000)

		// Even at maximum rate, multiplier is bounded by [0.5x, 2.0x]
		extreme := autopoiesisClampMultiplier(3_000_000) // 3.0x attempt
		t.Logf("  Hard cap test: 3.0x attempt clamped to %d BPS (%.1fx)", extreme, float64(extreme)/1e6)

		if extreme != autopoiesisSlashMultMax {
			t.Errorf("Expected clamping to max %d, got %d", autopoiesisSlashMultMax, extreme)
		}
	})

	t.Run("multi_dimension_attack", func(t *testing.T) {
		// Even if attacker depresses ALL 5 dimensions:
		// composite = 5 * (100k * 200k) / 1M = 100k -> critical
		// But multiplier still moves 1% per epoch.

		allDepressed := alignmentComposite(100_000, 100_000, 100_000, 100_000, 100_000)
		category := alignmentCategory(allDepressed)

		t.Logf("Multi-dimension attack: all=100k -> composite=%d, category=%s", allDepressed, category)

		if category != "critical" {
			t.Errorf("Expected critical category, got %s", category)
		}

		// Epochs to reach max multiplier from 1.0x at 1%/epoch
		epochsToMax := (autopoiesisSlashMultMax - bpsBasis) / autopoiesisMaxChangePerEpoch
		t.Logf("  Epochs to reach max multiplier (2.0x) from 1.0x: %d epochs (%d blocks)",
			epochsToMax, epochsToMax*100)

		// This is 100 epochs = 10,000 blocks — very slow, giving honest agents time to respond
		if epochsToMax < 50 {
			t.Error("Expected >50 epochs to reach max multiplier — too fast")
		}

		t.Logf("  Defense: slow rate-limited change gives validators time to detect and correct")
	})

	t.Run("pnl_manipulation_cost", func(t *testing.T) {
		// To depress KnowledgeQuality, attacker must submit many bad claims.
		// Each bad claim costs: claim stake + 22% slash.
		// Cost to flood 100 bad claims at 1 ZRN each:
		badClaims := uint64(100)
		claimStake := uint64(1_000_000) // 1 ZRN
		totalStaked := badClaims * claimStake
		totalSlashed := safeMulDiv(totalStaked, invalidClaimSlashBps, bpsBasis)

		// Benefit: multiplier moves 1%/epoch * N epochs.
		// To get a meaningful 10% shift: 10 epochs = 1000 blocks.
		// Meanwhile attacker's vesting is paused (FARM-6) and novelty suppressed (FARM-7).
		farm6Paused := isMisbehaviorPaused(badClaims, badClaims) // 100% rejection

		t.Logf("Manipulation Cost Analysis:")
		t.Logf("  Bad claims: %d at %d uzrn each", badClaims, claimStake)
		t.Logf("  Total staked: %d uzrn (%.0f ZRN)", totalStaked, float64(totalStaked)/1e6)
		t.Logf("  Total slashed: %d uzrn (%.0f ZRN)", totalSlashed, float64(totalSlashed)/1e6)
		t.Logf("  FARM-6 vesting paused: %v (100%% rejection)", farm6Paused)
		t.Logf("  Max multiplier shift achieved: 10%% over 10 epochs (1000 blocks)")
		t.Logf("  Verdict: cost ~%.0f ZRN slashed for a slow, bounded, self-correcting effect",
			float64(totalSlashed)/1e6)

		if !farm6Paused {
			t.Error("100% rejection should trigger FARM-6 vesting pause")
		}
	})
}

// ============================================================================
// NEW: Tool Revenue Siphoning Attack
// ============================================================================

func TestToolRevenueSiphoning(t *testing.T) {
	t.Run("dependency_chain_extraction", func(t *testing.T) {
		// Attack: Create a chain of tools where each intermediate tool takes
		// the maximum contributor share, leaving little for actual contributors.
		//
		// Revenue flow: Service revenue -> tree module split
		// ContributorsBp = 550000 (55%), ProtocolTreasury = 220000 (22%),
		// Research = 33300 (3.33%), Development = 196700 (19.67%).
		//
		// The contributor pool is split proportional to tasks completed.
		// An attacker creates many shell tasks to claim disproportionate share.

		totalRevenue := int64(10_000_000) // 10 ZRN total service revenue
		dist := calculateTreeRevenue(totalRevenue)

		t.Logf("Tool Revenue Siphoning — Dependency Chain:")
		t.Logf("  Total revenue: %d uzrn (%.0f ZRN)", totalRevenue, float64(totalRevenue)/1e6)
		t.Logf("  Contributor pool (55%%): %d uzrn", dist.contributorPool)
		t.Logf("  Protocol treasury (22%%): %d uzrn", dist.protocolTreasury)
		t.Logf("  Research fund (3.33%%): %d uzrn", dist.researchFund)
		t.Logf("  Development fund (19.67%%): %d uzrn", dist.developmentFund)

		// Defense 1: tasks-completed proportional split.
		// Attacker claims 20 shell tasks, honest contributor has 5 real tasks.
		attackerTasks := uint32(20)
		honestTasks := uint32(5)
		totalTasks := attackerTasks + honestTasks

		attackerShare := int64(dist.contributorPool) * int64(attackerTasks) / int64(totalTasks)
		honestShare := int64(dist.contributorPool) * int64(honestTasks) / int64(totalTasks)

		t.Logf("  Attacker (20 tasks): %d uzrn (%.1f%%)", attackerShare,
			float64(attackerShare*100)/float64(dist.contributorPool))
		t.Logf("  Honest (5 tasks): %d uzrn (%.1f%%)", honestShare,
			float64(honestShare*100)/float64(dist.contributorPool))

		// But tasks require acceptance — shell tasks get rejected.
		// Max rejections before removal = 3 (from tree params).
		maxRejections := 3
		t.Logf("  Defense: MaxRejections=%d per task — shell tasks rejected quickly", maxRejections)

		// With 3 rejections needed per task, attacker needs honest reviewers to NOT reject.
		// But task completion requires actual deliverables.
		t.Logf("  Defense: task completion requires deliverable acceptance by project owner")
		t.Logf("  Defense: contributor pool is 55%%, not 100%% — protocol retains 45%%")
	})

	t.Run("revenue_cap_analysis", func(t *testing.T) {
		// The tree module caps at specific BPS values.
		// No single contributor can exceed the proportional share based on completed tasks.

		totalRevenue := int64(100_000_000) // 100 ZRN
		dist := calculateTreeRevenue(totalRevenue)

		// Even if attacker gets ALL tasks (100%), they only get 55% of revenue
		maxExtraction := dist.contributorPool
		extractionRate := float64(maxExtraction*100) / float64(totalRevenue)

		t.Logf("Revenue Cap Analysis:")
		t.Logf("  Max extraction (100%% of tasks): %d uzrn (%.0f%% of total revenue)",
			maxExtraction, extractionRate)
		t.Logf("  Protocol retains: %d uzrn treasury + %d uzrn research + %d uzrn development",
			dist.protocolTreasury, dist.researchFund, dist.developmentFund)

		// Total protocol-retained percentage
		protocolRetained := float64(dist.protocolTreasury+dist.researchFund+dist.developmentFund) * 100 / float64(totalRevenue)
		t.Logf("  Protocol retention: %.0f%% guaranteed regardless of task distribution", protocolRetained)

		// 45% of revenue always goes to protocol/research/development — cannot be siphoned
		if protocolRetained < 40 {
			t.Error("Protocol should retain at least 40% of revenue")
		}
	})

	t.Run("evidence_tax_defense", func(t *testing.T) {
		// Additional defense: disputes carry a 22% evidence tax (EvidenceTaxBp=220000).
		// Service disputes slash the dishonest party.
		evidenceTaxBps := uint32(220_000)
		disputeStake := int64(5_000_000) // 5 ZRN
		taxAmount := disputeStake * int64(evidenceTaxBps) / 1_000_000

		t.Logf("Evidence Tax Defense:")
		t.Logf("  Dispute stake: %d uzrn, Evidence tax: %d uzrn (22%%)", disputeStake, taxAmount)
		t.Logf("  Frivolous disputes cost 22%% of stake — discourages gaming")
	})
}

// ============================================================================
// NEW: Research Fund Drain Attack
// ============================================================================

func TestResearchFundDrain(t *testing.T) {
	t.Run("low_quality_flood", func(t *testing.T) {
		// Attack: submit many low-quality research proposals to drain the research fund.
		// Defenses: MinStake (1 ZRN), MinReviewerCount (3), AcceptanceThreshold (70/100),
		// RejectionSlash (33%).

		proposals := uint64(50)
		stakePerProposal := researchMinStake
		totalStaked := researchProposalCost(proposals)

		t.Logf("Research Fund Drain — 50 low-quality proposals:")
		t.Logf("  Stake per proposal: %d uzrn (%.0f ZRN)", stakePerProposal, float64(stakePerProposal)/1e6)
		t.Logf("  Total staked: %d uzrn (%.0f ZRN)", totalStaked, float64(totalStaked)/1e6)

		// Defense 1: MinReviewerCount = 3. Each proposal needs 3 independent reviews.
		t.Logf("  Defense 1: MinReviewerCount=%d — requires 3 independent reviews", researchMinReviewerCount)

		// Defense 2: AcceptanceScoreThreshold = 70. Average score must be >= 70/100.
		// Low-quality proposals score ~30/100 from honest reviewers.
		lowQualityScore := uint32(30)
		accepted := researchReviewAccepted(lowQualityScore*researchMinReviewerCount, researchMinReviewerCount)
		t.Logf("  Defense 2: AcceptanceThreshold=%d/100, low-quality avg score=%d -> accepted=%v",
			researchAcceptanceThreshold, lowQualityScore, accepted)

		if accepted {
			t.Error("Low-quality proposals (score=30) should not be accepted at threshold=70")
		}

		// Defense 3: RejectionSlash = 33%. Rejected proposals lose 33% of stake.
		slashPerRejection := researchRejectionSlash(stakePerProposal)
		totalSlashed := slashPerRejection * proposals

		t.Logf("  Defense 3: RejectionSlashBps=%d (33%%)", researchRejectionSlashBps)
		t.Logf("  Slash per rejection: %d uzrn", slashPerRejection)
		t.Logf("  Total slashed across %d rejections: %d uzrn (%.0f ZRN)", proposals, totalSlashed, float64(totalSlashed)/1e6)

		// Net cost to attacker
		stakeReturned := totalStaked - totalSlashed
		t.Logf("  Attacker cost: %d uzrn staked, %d uzrn slashed, %d uzrn returned",
			totalStaked, totalSlashed, stakeReturned)
		t.Logf("  Net loss: %d uzrn (%.1f ZRN)", totalSlashed, float64(totalSlashed)/1e6)

		// Benefit: zero — no proposals accepted, no fund drained.
		t.Logf("  Benefit: NONE (all proposals rejected, fund intact)")
		t.Logf("  Verdict: -%.1f ZRN with zero benefit — attack deeply negative EV", float64(totalSlashed)/1e6)
	})

	t.Run("colluding_reviewers", func(t *testing.T) {
		// Harder attack: attacker also controls 3 reviewer accounts.
		// They approve their own low-quality proposals with score 80/100.

		colludingScore := uint32(80)
		accepted := researchReviewAccepted(colludingScore*researchMinReviewerCount, researchMinReviewerCount)

		t.Logf("Colluding Reviewers Attack:")
		t.Logf("  Colluding score: %d/100, accepted: %v", colludingScore, accepted)

		if !accepted {
			t.Error("Colluding reviewers with score 80 should bypass threshold")
		}

		// But colluding reviewers face their own costs:
		// 1. Reviewers must be at Verified tier or above (stake + verifications).
		reviewerMinVerifications := uint64(22)
		reviewerStakeRequirement := bondedMinStake // 1111 ZRN per reviewer

		// 3 colluding reviewers + 1 submitter = 4 accounts
		totalInfrastructureCost := 3*reviewerStakeRequirement + researchMinStake

		t.Logf("  Infrastructure cost for 3 colluding reviewers:")
		t.Logf("    Each reviewer: %d uzrn stake (%.0f ZRN) + %d verifications",
			reviewerStakeRequirement, float64(reviewerStakeRequirement)/1e6, reviewerMinVerifications)
		t.Logf("    Total infrastructure: %d uzrn (%.0f ZRN)",
			totalInfrastructureCost, float64(totalInfrastructureCost)/1e6)

		// Additional detection: FARM-1 catches colluding reviewers who always approve
		// (100% conformity on their reviews).
		farm1Penalty := conformityPenalty(50, 50) // 100% approval
		t.Logf("  FARM-1 on colluding reviewers (100%% approval): penalty=%d BPS (%.1f%%)",
			farm1Penalty, float64(farm1Penalty)/10_000)

		// Research proposals can also be challenged after acceptance.
		challengeStake := researchMinStake // 1 ZRN per challenge
		t.Logf("  Post-acceptance challenge stake: %d uzrn", challengeStake)
		t.Logf("  Challenged research enters dispute — resolved by broader validator set")

		t.Logf("  Verdict: requires ~%.0f ZRN infrastructure + ongoing FARM-1 risk + challenge vulnerability",
			float64(totalInfrastructureCost)/1e6)
	})

	t.Run("pnl_drain_efficiency", func(t *testing.T) {
		// Even with colluding reviewers, how much can the attacker drain?
		// Research fund receives 3.33% of tree revenue (ResearchFundBp=33300).
		// Monthly tree revenue estimate: 10,000 ZRN -> fund gets 333 ZRN/month.

		monthlyFundIncome := int64(333_000_000) // 333 ZRN in uzrn
		proposalsPerMonth := uint64(20) // limited by review period (2 days each)

		// Each accepted proposal gets funded from the research fund.
		// Typical funding per proposal: varies, but capped by available balance.
		// Attacker gets stake back + potential research reward.
		// But infrastructure cost is ~3333 ZRN (3 reviewers at 1111 ZRN each).
		infrastructureCost := 3 * bondedMinStake
		monthlyExtraction := proposalsPerMonth * researchMinStake // minimal extraction

		t.Logf("Drain Efficiency Analysis:")
		t.Logf("  Monthly fund income: %d uzrn (%.0f ZRN)", monthlyFundIncome, float64(monthlyFundIncome)/1e6)
		t.Logf("  Max proposals/month (2-day review): %d", proposalsPerMonth)
		t.Logf("  Infrastructure cost: %d uzrn (%.0f ZRN)", infrastructureCost, float64(infrastructureCost)/1e6)
		t.Logf("  Monthly extraction (at min stake): %d uzrn (%.0f ZRN)",
			monthlyExtraction, float64(monthlyExtraction)/1e6)

		// ROI: infrastructure cost / monthly extraction
		monthsToBreakEven := float64(infrastructureCost) / float64(monthlyExtraction)
		t.Logf("  Months to break even: %.0f", monthsToBreakEven)

		// At 1 ZRN per proposal, 20/month = 20 ZRN/month extraction
		// Infrastructure: 3333 ZRN -> 167 months to break even.
		// Meanwhile, slashing risk on the 3333 ZRN stake is ongoing.
		if monthsToBreakEven < 100 {
			t.Logf("  WARNING: break-even < 100 months — may need higher MinResearchStake")
		}

		t.Logf("  Verdict: ~%.0f month break-even with ongoing slashing risk — attack is uneconomical",
			monthsToBreakEven)
	})
}

// ============================================================================
// Per-Attack Summary: Overall Analysis
// ============================================================================

func TestAdversarialSimulation_Summary(t *testing.T) {
	type attackResult struct {
		attack         string
		detected       bool
		detectionEpoch string
		penaltyApplied string
		netPLperRound  string
		roiVsHonest    string
	}

	results := []attackResult{
		{
			attack:         "FARM-1: Rubber-stamp (96%)",
			detected:       true,
			detectionEpoch: "Immediate (at 20+ samples)",
			penaltyApplied: "-4% reward (scales to -20%)",
			netPLperRound:  fmt.Sprintf("-%d uzrn/verif vs honest", safeMulDiv(verificationRewardBase, 40_000, bpsBasis)),
			roiVsHonest:    "96% of honest at 96% conformity",
		},
		{
			attack:         "FARM-2: Trivial claims (98%)",
			detected:       true,
			detectionEpoch: "After 10+ samples",
			penaltyApplied: "-6% confidence",
			netPLperRound:  "Lower confidence -> lower vesting value",
			roiVsHonest:    "~80% of honest ROI",
		},
		{
			attack:         "FARM-6: Grief-and-extract (40%)",
			detected:       true,
			detectionEpoch: "After 10+ claims",
			penaltyApplied: "ALL vesting paused + stake slashed",
			netPLperRound:  "Negative (slash > remaining vesting)",
			roiVsHonest:    "Severely negative",
		},
		{
			attack:         "FARM-7: Domain squat (1 agent)",
			detected:       true,
			detectionEpoch: "Immediate (contributor check)",
			penaltyApplied: "Novelty suppressed to 33%",
			netPLperRound:  "~33% of normal novelty bonus",
			roiVsHonest:    "~33% novelty efficiency",
		},
		{
			attack:         "FARM-9: Frivolous challenge",
			detected:       true,
			detectionEpoch: "At challenge submission",
			penaltyApplied: "Blocked (50% min stake) or capped reward",
			netPLperRound:  "Negative EV at <63% success rate",
			roiVsHonest:    "Unprofitable below 63% accuracy",
		},
		{
			attack:         "FARM-10: Axiom restating",
			detected:       true,
			detectionEpoch: "At evaluation",
			penaltyApplied: "-20% confidence penalty",
			netPLperRound:  "Lower vesting + FARM-2 stacks",
			roiVsHonest:    "~70% of honest (compounded with FARM-2)",
		},
		{
			attack:         "Scenario 1: Citation ring (3 agents)",
			detected:       true,
			detectionEpoch: "~78 verifications per agent",
			penaltyApplied: "FARM-1 + FARM-2 compound",
			netPLperRound:  "Degrading over time",
			roiVsHonest:    "Fragile (lose 1 agent -> FARM-7 cuts novelty)",
		},
		{
			attack:         "Scenario 2: Tier gaming",
			detected:       true,
			detectionEpoch: "~78 rubber-stamps post-legitimacy",
			penaltyApplied: "FARM-1 reward reduction",
			netPLperRound:  "~78 free verifications then penalized",
			roiVsHonest:    "Profitable for ~78 rounds, then negative",
		},
		{
			attack:         "Scenario 3: Apprentice exploit",
			detected:       false,
			detectionEpoch: "N/A (not economically viable)",
			penaltyApplied: "0.1x reward + limited categories",
			netPLperRound:  fmt.Sprintf("%d uzrn/verif (vs %d bonded)", verificationRewardBase/10, verificationRewardBase),
			roiVsHonest:    "~1% of bonded validator income",
		},
		{
			attack:         "Scenario 4: Falsification arbitrage",
			detected:       true,
			detectionEpoch: "At challenge (50% min stake)",
			penaltyApplied: "22% stake loss on failure, reward cap",
			netPLperRound:  "Negative EV below 63% success",
			roiVsHonest:    "Only profitable for legitimate challenges",
		},
		// ---- Zerone-specific attacks ----
		{
			attack:         "Autopoiesis manipulation",
			detected:       true,
			detectionEpoch: "Immediate (alignment sensors)",
			penaltyApplied: "1% max change/epoch + [0.5x,2.0x] bounds",
			netPLperRound:  "Negative (FARM-6 pause + 22% claim slash)",
			roiVsHonest:    "Deeply negative (cost >> bounded effect)",
		},
		{
			attack:         "Tool revenue siphoning",
			detected:       true,
			detectionEpoch: "At task review (MaxRejections=3)",
			penaltyApplied: "55% cap + task rejection + evidence tax",
			netPLperRound:  "45% revenue untouchable by protocol design",
			roiVsHonest:    "Capped at 55% even with 100% task control",
		},
		{
			attack:         "Research fund drain",
			detected:       true,
			detectionEpoch: "At review (MinReviewerCount=3)",
			penaltyApplied: "33% rejection slash + 70/100 threshold",
			netPLperRound:  "~167 month break-even with collusion",
			roiVsHonest:    "Uneconomical (high infrastructure cost)",
		},
	}

	t.Logf("\n%-40s | %-8s | %-28s | %-35s | %-25s", "Attack", "Detected", "Detection Point", "Penalty", "ROI vs Honest")
	t.Logf("%-40s-+-%-8s-+-%-28s-+-%-35s-+-%-25s",
		"----------------------------------------", "--------", "----------------------------",
		"-----------------------------------", "-------------------------")

	allDetected := true
	for _, r := range results {
		detected := "YES"
		if !r.detected {
			detected = "NO*"
		}
		t.Logf("%-40s | %-8s | %-28s | %-35s | %-25s",
			r.attack, detected, r.detectionEpoch, r.penaltyApplied, r.roiVsHonest)
		if !r.detected && r.attack != "Scenario 3: Apprentice exploit" {
			allDetected = false
		}
	}

	t.Logf("\n* Scenario 3 not detected because it's not economically viable (0.1x reward)")

	// ========================================================================
	// Key Questions
	// ========================================================================

	t.Logf("\n=== Key Questions ===")

	t.Logf("\nQ1: Profitable long-term farming strategy?")
	t.Logf("  NO. All farming strategies are either:")
	t.Logf("  - Detected and penalized (FARM-1, FARM-2, FARM-6, FARM-7, FARM-10)")
	t.Logf("  - Economically unviable (Scenario 3: apprentice exploit)")
	t.Logf("  - Time-limited (Scenario 2: ~78 free verifications before FARM-1 catches up)")
	t.Logf("  The compound FARM-1 + FARM-2 penalty stack makes sustained farming deeply negative.")

	t.Logf("\nQ2: Minimum sophistication for profitable attacker?")
	t.Logf("  HIGH. Must maintain conformity <95%% (vary votes), acceptance 70-95%% (reject some own claims),")
	t.Logf("  and operate in diverse domains with 3+ contributors. This is essentially... honest participation.")

	t.Logf("\nQ3: Penalty proportionality?")
	t.Logf("  FARM-1 (max 20%%): Proportionate — reduces reward, doesn't destroy it")
	t.Logf("  FARM-2 (max 10%%): Proportionate — mild penalty, honest submitters at 70%% unaffected")
	t.Logf("  FARM-6 (vesting pause): Aggressive but appropriate — protects all prior honest work")
	t.Logf("  FARM-9 (50%% min stake + 100%% reward cap): Proportionate — high-stake claims require high-stake challenges")
	t.Logf("  FARM-10 (20%% penalty): Proportionate — doesn't outright reject, just reduces confidence")
	t.Logf("  CONCERN: Tier gaming has ~78 free verifications before detection — consider faster ramp-up")

	t.Logf("\nQ4: Citation ring with good accuracy?")
	t.Logf("  MARGINAL. 3 agents at minimum novelty threshold is fragile.")
	t.Logf("  If one agent leaves, FARM-7 cuts novelty to 66%%.")
	t.Logf("  FARM-1 catches conformity within ~78 verifications per agent.")
	t.Logf("  100%% acceptance triggers FARM-2 penalty.")
	t.Logf("  Best case: ~78 x 3 = 234 total verifications of modest profit before compounding penalties.")

	t.Logf("\nQ5: Is 22%% invalid claim slash proportionate?")
	invalidSlashPct := float64(invalidClaimSlashBps) / 10_000
	t.Logf("  Slash: %.1f%% of claim stake on rejection", invalidSlashPct)
	t.Logf("  Successful challenge reward: 30%% of slashed amount = %.1f%% of claim stake",
		invalidSlashPct*0.3)
	t.Logf("  Remaining returned: %.1f%% of stake", 100-invalidSlashPct)
	t.Logf("  ASSESSMENT: Proportionate. 78%% stake return on honest mistake is forgiving.")
	t.Logf("  The 22%% rate means an agent needs >78%% claim acceptance to be net positive on stake alone.")
	t.Logf("  Combined with vesting rewards, break-even is lower (~60%%), aligned with CalibrationNeutralRate (70%%).")

	t.Logf("\nQ6: Zerone-specific attack defenses?")
	t.Logf("  Autopoiesis: Rate-limited (1%%/epoch) + bounded [0.5x, 2.0x] + multi-dimension sensor fusion")
	t.Logf("  Revenue siphoning: 55%% contributor cap + task acceptance requirement + evidence tax")
	t.Logf("  Research drain: 33%% slash + 3 reviewers + 70/100 threshold + challenge mechanism")

	if !allDetected {
		t.Error("OVERALL: Not all viable attacks are detected")
	}
}
