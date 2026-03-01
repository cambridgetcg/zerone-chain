package keeper

import (
	"context"
	"fmt"
	"math/big"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// CreateVerificationRound creates a new verification round for a claim.
func (k Keeper) CreateVerificationRound(ctx context.Context, claim *types.Claim) (*types.VerificationRound, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	height := uint64(sdkCtx.BlockHeight())

	params, err := k.GetParams(ctx)
	if err != nil {
		return nil, err
	}

	roundID := GenerateRoundID(claim.Id, height)

	round := &types.VerificationRound{
		Id:                  roundID,
		ClaimId:             claim.Id,
		StartedAtBlock:      height,
		Phase:               types.VerificationPhase_VERIFICATION_PHASE_COMMIT,
		CommitDeadline:      height + params.CommitPhaseBlocks,
		RevealDeadline:      height + params.CommitPhaseBlocks + params.RevealPhaseBlocks,
		AggregationDeadline: height + params.CommitPhaseBlocks + params.RevealPhaseBlocks + params.AggregationPhaseBlocks,
	}

	if err := k.SetVerificationRound(ctx, round); err != nil {
		return nil, err
	}

	// Update claim with round reference
	claim.VerificationRoundId = roundID
	claim.Status = types.ClaimStatus_CLAIM_STATUS_IN_VERIFICATION
	if err := k.SetClaim(ctx, claim); err != nil {
		return nil, err
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.knowledge.verification_round_created",
		sdk.NewAttribute("round_id", roundID),
		sdk.NewAttribute("claim_id", claim.Id),
		sdk.NewAttribute("phase", "COMMIT"),
	))

	return round, nil
}

// CompleteRound finalizes a verification round based on the aggregated result.
func (k Keeper) CompleteRound(ctx context.Context, round *types.VerificationRound, result *VerificationResult) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	height := uint64(sdkCtx.BlockHeight())

	claim, found := k.GetClaim(ctx, round.ClaimId)
	if !found {
		return fmt.Errorf("claim %s not found for round %s", round.ClaimId, round.Id)
	}

	round.Verdict = result.Verdict
	round.VerdictBlock = height
	round.Phase = types.VerificationPhase_VERIFICATION_PHASE_COMPLETE

	var factId string
	switch result.Verdict {
	case types.Verdict_VERDICT_ACCEPT:
		// Create fact from accepted claim
		var err error
		factId, err = k.createFactFromClaim(ctx, claim, round, result.Confidence)
		if err != nil {
			return err
		}
		// Review fee already distributed at submission time — no refund.
		claim.Status = types.ClaimStatus_CLAIM_STATUS_ACCEPTED

	case types.Verdict_VERDICT_REJECT:
		// Review fee already distributed at submission time — the fee IS the cost of rejection.
		claim.Status = types.ClaimStatus_CLAIM_STATUS_REJECTED
		// If this was a challenge claim, the original fact survived — energy boost
		k.handleChallengeSurvival(ctx, claim)

	case types.Verdict_VERDICT_MALFORMED:
		// Review fee already distributed at submission time — no additional slashing needed.
		claim.Status = types.ClaimStatus_CLAIM_STATUS_MALFORMED

	case types.Verdict_VERDICT_INCONCLUSIVE:
		// Review fee is non-refundable — verifiers still did work even if inconclusive.
		claim.Status = types.ClaimStatus_CLAIM_STATUS_INSUFFICIENT
	}

	if err := k.SetClaim(ctx, claim); err != nil {
		return err
	}
	if err := k.SetVerificationRound(ctx, round); err != nil {
		return err
	}

	// Distribute verifier rewards from the 55% fee pool
	k.distributeVerifierRewardsFromPool(ctx, claim, result)

	// Slash loop: route vindication-eligible slashes to escrow when enabled
	params, _ := k.GetParams(ctx)
	var vindicationEntries []types.VindicationEntry

	for _, slash := range result.Slashes {
		if k.stakingKeeper == nil {
			continue
		}
		if slash.VindicationEligible && params.VindicationRefundEnabled {
			slashedAmt, err := k.stakingKeeper.SlashValidatorToModule(ctx, slash.Verifier, slash.SlashBps, types.VindicationEscrowModuleName)
			if err == nil && slashedAmt.IsPositive() {
				vote := ""
				for _, reveal := range round.Reveals {
					if reveal.Verifier == slash.Verifier {
						vote = reveal.Vote
						break
					}
				}
				vindicationEntries = append(vindicationEntries, types.VindicationEntry{
					Verifier:    slash.Verifier,
					Vote:        vote,
					SlashAmount: slashedAmt.String(),
					SlashBps:    slash.SlashBps,
					RoundId:     round.Id,
					Height:      height,
				})
			}
		} else {
			_ = k.stakingKeeper.SlashValidator(ctx, slash.Verifier, slash.SlashBps)
		}
	}

	// Store vindication entries if any, associated with the created fact
	if len(vindicationEntries) > 0 && factId != "" {
		for i := range vindicationEntries {
			vindicationEntries[i].FactId = factId
		}
		_ = k.SetVindicationPending(ctx, factId, vindicationEntries)
	}

	// If this was a challenge claim that was ACCEPTED, the original fact is disproven.
	// This triggers vindication for the original fact's minority voters.
	if result.Verdict == types.Verdict_VERDICT_ACCEPT && claim.ProvisionalFactId != "" {
		k.handleChallengeDisproven(ctx, claim, factId)
	}

	// Record verification outcomes for domain qualification tracking (R26-3).
	// Rewarded verifiers voted correctly; slashed verifiers voted incorrectly.
	if k.domainQualificationKeeper != nil && claim.Domain != "" &&
		result.Verdict != types.Verdict_VERDICT_INCONCLUSIVE {
		for _, reward := range result.Rewards {
			if err := k.domainQualificationKeeper.RecordVerificationOutcome(ctx, reward.Verifier, claim.Domain, true); err != nil {
				k.Logger(ctx).Debug("failed to record correct verification outcome", "verifier", reward.Verifier, "error", err)
			}
		}
		for _, slash := range result.Slashes {
			if err := k.domainQualificationKeeper.RecordVerificationOutcome(ctx, slash.Verifier, claim.Domain, false); err != nil {
				k.Logger(ctx).Debug("failed to record incorrect verification outcome", "verifier", slash.Verifier, "error", err)
			}
		}
	}

	// Feed verification history to capture defense (R28-8).
	if k.captureDefenseKeeper != nil {
		verdictVote := verdictToVoteString(result.Verdict)
		roundValidators := make([]string, 0, len(round.Reveals))
		roundVerdicts := make([]bool, 0, len(round.Reveals))
		for _, reveal := range round.Reveals {
			roundValidators = append(roundValidators, reveal.Verifier)
			roundVerdicts = append(roundVerdicts, reveal.Vote == verdictVote)
		}
		k.captureDefenseKeeper.RecordVerificationHistory(ctx, claim.Domain, round.Id, roundValidators, roundVerdicts, nil)

		// Update reputations — get stratum for domain context.
		stratum := ""
		if k.ontologyKeeper != nil {
			stratum, _ = k.ontologyKeeper.GetStratumForDomain(ctx, claim.Domain)
		}
		for _, reveal := range round.Reveals {
			wasCorrect := reveal.Vote == verdictVote
			k.captureDefenseKeeper.UpdateReputation(ctx, reveal.Verifier, claim.Domain, stratum, wasCorrect)
		}
	}

	// Record round diversity metrics (R28-2)
	if err := k.RecordRoundDiversity(ctx, round.Id, claim.Domain, result.AcceptCount, result.RejectCount); err != nil {
		k.Logger(ctx).Error("failed to record round diversity", "round_id", round.Id, "error", err)
	}

	// Update validator independence for each revealed voter
	majorityVote := verdictToVoteString(result.Verdict)
	if majorityVote != "" {
		for _, reveal := range round.Reveals {
			if err := k.UpdateValidatorIndependence(ctx, reveal.Verifier, reveal.Vote, majorityVote); err != nil {
				k.Logger(ctx).Debug("failed to update validator independence", "verifier", reveal.Verifier, "error", err)
			}
		}
	}

	// Index completed round for window-based metrics (R31-2)
	hasDissent := roundHasDissent(round)
	duration := height - round.StartedAtBlock
	completionMeta := &types.CompletedRoundMeta{
		Domain:         claim.Domain,
		HasDissent:     hasDissent,
		DurationBlocks: duration,
	}
	if idxErr := k.IndexCompletedRound(ctx, height, round.Id, completionMeta); idxErr != nil {
		k.Logger(ctx).Debug("failed to index completed round", "round", round.Id, "error", idxErr)
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.knowledge.verification_round_completed",
		sdk.NewAttribute("round_id", round.Id),
		sdk.NewAttribute("claim_id", round.ClaimId),
		sdk.NewAttribute("verdict", round.Verdict.String()),
	))

	return nil
}

// distributeVerifierRewardsFromPool distributes the 55% verifier pool among correct verifiers.
func (k Keeper) distributeVerifierRewardsFromPool(ctx context.Context, claim *types.Claim, result *VerificationResult) {
	if k.bankKeeper == nil || len(result.Rewards) == 0 {
		return
	}

	// Calculate pool: 55% of the original review fee
	feeAmt, ok := new(big.Int).SetString(claim.Stake, 10)
	if !ok || feeAmt.Sign() <= 0 {
		return
	}
	poolAmount := verifierPoolFromFee(feeAmt.Uint64())
	if poolAmount == 0 {
		return
	}

	// Divide pool equally among rewarded verifiers
	perVerifier := poolAmount / uint64(len(result.Rewards))
	remainder := poolAmount - (perVerifier * uint64(len(result.Rewards)))

	for i, reward := range result.Rewards {
		amount := perVerifier
		if i == 0 {
			amount += remainder // first verifier gets dust
		}
		k.distributeVerifierReward(ctx, reward.Verifier, amount)
	}
}

// createFactFromClaim creates a new Fact from an accepted claim.
// Returns the generated factID and any error.
func (k Keeper) createFactFromClaim(ctx context.Context, claim *types.Claim, round *types.VerificationRound, confidence uint64) (string, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	height := uint64(sdkCtx.BlockHeight())

	factID := GenerateFactID(claim.Id, height)

	// Calculate fitness epoch from current height
	params, _ := k.GetParams(ctx)
	epochBorn := uint64(0)
	if params.FitnessEpochBlocks > 0 {
		epochBorn = height / params.FitnessEpochBlocks
	}

	fact := &types.Fact{
		Id:                factID,
		Content:           claim.FactContent,
		Domain:            claim.Domain,
		Category:          claim.Category,
		Confidence:        confidence,
		Submitter:         claim.Submitter,
		SubmittedAtBlock:  claim.SubmittedAtBlock,
		VerifiedAtBlock:   height,
		LastVerifiedBlock: height,
		References:        claim.References,
		Status:            types.FactStatus_FACT_STATUS_VERIFIED,
		ClaimId:           claim.Id,
		ClaimType:         claim.ClaimType,
		Structure:         claim.Structure,
		CanonicalForm:     claim.CanonicalForm,
		CanonicalHash:     claim.CanonicalHash,
		// Fitness fields
		FitnessScore:       params.FitnessInitialScore,
		FitnessUpdatedBlock: height,
		EpochBorn:          epochBorn,
		// Metabolism fields
		Energy:           params.MetabolismInitialEnergy,
		EnergyCap:        params.MetabolismEnergyCap,
		EnergyLastUpdated: height,
	}

	// Apply domain carrying capacity birth pressure (R29-1)
	fact.Energy = k.ApplyBirthPressure(ctx, claim.Domain, fact.Energy)

	// Apply role bonus — claim type × account type (R28-5)
	accountType := k.getAccountType(ctx, claim.Submitter)
	fact.Confidence = ApplyRoleBonusToConfidence(fact.Confidence, claim.ClaimType, accountType, params)

	// Apply dual validation bonus for partnership claims (R28-5)
	// Scale by weaker role's accuracy in the domain (R29-3)
	if claim.PartnershipId != "" {
		agentAcc, humanAcc := k.GetRoleAccuracies(ctx, claim.Domain)
		weakerAccuracy := agentAcc
		if humanAcc < weakerAccuracy || weakerAccuracy == 0 {
			weakerAccuracy = humanAcc
		}
		if weakerAccuracy > 0 {
			scaledBonusBps := safeMulDiv(params.DualValidationBonusBps, weakerAccuracy, BPS)
			fact.Confidence = safeMulDiv(fact.Confidence, 1_000_000+scaledBonusBps, 1_000_000)
		} else {
			// No track record yet — use full static bonus
			fact.Confidence = ApplyDualValidationBonus(fact.Confidence, params)
		}
	}

	// Apply confidence ceiling (stratum + global MaxConfidence hard cap)
	fact.Confidence = k.ClampConfidence(ctx, fact.Confidence, claim.Domain)
	if k.ontologyKeeper != nil && claim.Domain != "" {
		stratum, err := k.ontologyKeeper.GetStratumForDomain(ctx, claim.Domain)
		if err == nil && stratum != "" {
			fact.Stratum = stratum
		}
	}

	if err := k.SetFact(ctx, fact); err != nil {
		return "", err
	}

	// Update domain stats for carrying capacity (R29-1)
	k.IncrementDomainFactCount(ctx, fact.Domain, true, fact.Energy)

	// Index fact by structured subject and tags
	if fact.Structure != nil {
		if err := k.IndexFactBySubject(ctx, fact); err != nil {
			return "", fmt.Errorf("failed to index fact by subject: %w", err)
		}
	}

	// ─── Niche registration (competitive dynamics) ─────────────────────
	nicheKey := k.ComputeNicheKey(fact)
	fact.NicheKey = nicheKey
	fact.NicheLeader = true // new fact starts as leader-candidate
	fact.NicheRank = 1
	fact.NicheSize = 1
	currentLeader, hasLeader := k.GetNicheLeader(ctx, nicheKey)
	if hasLeader {
		sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
			"zerone.knowledge.niche_challenger",
			sdk.NewAttribute("new_fact", fact.Id),
			sdk.NewAttribute("current_leader", currentLeader.Id),
			sdk.NewAttribute("niche", nicheKey),
			sdk.NewAttribute("domain", fact.Domain),
		))
		fact.NicheLeader = false // not leader yet — competition will decide
		fact.NicheSize = uint64(len(k.GetNicheMembers(ctx, nicheKey))) + 1
	}
	if err := k.UpdateNicheIndex(ctx, fact); err != nil {
		return "", fmt.Errorf("failed to update niche index: %w", err)
	}
	// Re-save fact with niche fields set
	if err := k.SetFact(ctx, fact); err != nil {
		return "", fmt.Errorf("failed to save fact with niche fields: %w", err)
	}

	// Index fact by canonical hash
	if fact.CanonicalHash != "" {
		if err := k.SetCanonicalHash(ctx, fact.CanonicalHash, fact.Id); err != nil {
			return "", fmt.Errorf("failed to index fact by canonical hash: %w", err)
		}
	}

	// Convert claim relations to fact relations and store in graph index
	for _, claimRel := range claim.Relations {
		factRel := &types.FactRelation{
			SourceFactId:   factID,
			TargetFactId:   claimRel.TargetFactId,
			Relation:       claimRel.Relation,
			CreatedAtBlock: height,
			Creator:        claim.Submitter,
		}
		if err := k.SetFactRelation(ctx, factRel); err != nil {
			return "", fmt.Errorf("failed to store fact relation: %w", err)
		}

		// Track new citation for metabolism energy income
		k.IncrementNewCitationEpoch(ctx, claimRel.TargetFactId)

		sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
			"zerone.knowledge.fact_relation_created",
			sdk.NewAttribute("source", factRel.SourceFactId),
			sdk.NewAttribute("target", factRel.TargetFactId),
			sdk.NewAttribute("relation", factRel.Relation.String()),
		))
	}

	// ─── Lineage registration (reproduction) ──────────────────────────────
	// Check for reproductive relations (REFINES or GENERALIZES imply parent-child)
	for _, rel := range claim.Relations {
		if rel.Relation == types.RelationType_RELATION_TYPE_REFINES ||
			rel.Relation == types.RelationType_RELATION_TYPE_GENERALIZES {
			parentFact, found := k.GetFact(ctx, rel.TargetFactId)
			if !found {
				continue
			}

			// Check max children
			if uint64(len(parentFact.ChildFactIds)) >= params.ReproductionMaxChildren {
				k.Logger(ctx).Info("parent at max children", "parent", parentFact.Id)
				continue
			}

			// Set lineage on child
			fact.ParentFactId = parentFact.Id
			fact.LineageDepth = parentFact.LineageDepth + 1
			fact.LineageRootId = parentFact.LineageRootId
			if fact.LineageRootId == "" {
				fact.LineageRootId = parentFact.Id // parent is the root
			}

			// Inherit fitness from parent
			inheritedFitness := safeMulDiv(parentFact.FitnessScore, params.ReproductionChildFitnessInheritanceBps, 1_000_000)
			fact.FitnessScore = inheritedFitness

			// Update parent: add child, bump progeny, energy bonus
			parentFact.ChildFactIds = append(parentFact.ChildFactIds, fact.Id)
			parentFact.ProgenyCount++
			parentFact.Energy += params.ReproductionParentEnergyBonus
			if parentFact.Energy > params.MetabolismEnergyCap {
				parentFact.Energy = params.MetabolismEnergyCap
			}
			_ = k.SetFact(ctx, parentFact)

			// Propagate progeny count up the lineage
			k.PropagateProgenyCount(ctx, parentFact.ParentFactId)

			// Save updated child fact with lineage fields
			_ = k.SetFact(ctx, fact)

			sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
				"zerone.knowledge.lineage_created",
				sdk.NewAttribute("child_fact_id", fact.Id),
				sdk.NewAttribute("parent_fact_id", parentFact.Id),
				sdk.NewAttribute("lineage_depth", fmt.Sprintf("%d", fact.LineageDepth)),
				sdk.NewAttribute("inherited_fitness", fmt.Sprintf("%d", inheritedFitness)),
			))

			break // Only one parent relationship
		}
	}

	// Route submitter reward: through partnership split or direct vesting (R26-4)
	if claim.PartnershipId != "" && k.partnershipKeeper != nil {
		stakeAmt, ok := new(big.Int).SetString(claim.Stake, 10)
		if ok && stakeAmt.Sign() > 0 {
			rewardCoins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(stakeAmt)))
			err := k.partnershipKeeper.DistributeReward(ctx, claim.PartnershipId, rewardCoins, "knowledge_verification")
			if err != nil {
				// Fallback to direct vesting on partnership error
				k.Logger(ctx).Error("partnership reward routing failed, falling back to vesting",
					"partnership_id", claim.PartnershipId, "err", err)
				if k.vestingRewardsKeeper != nil {
					_ = k.vestingRewardsKeeper.CreateVestingScheduleFromKnowledge(
						ctx, claim.Id, factID, claim.Submitter, claim.Stake, claim.Category,
					)
				}
			}
		}
	} else if k.vestingRewardsKeeper != nil {
		// Direct vesting (no partnership — existing behavior)
		_ = k.vestingRewardsKeeper.CreateVestingScheduleFromKnowledge(
			ctx, claim.Id, factID, claim.Submitter, claim.Stake, claim.Category,
		)
	}

	// Check if this fact fills an active knowledge bounty
	k.ClaimBountyForFact(ctx, fact, claim)

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.knowledge.fact_created",
		sdk.NewAttribute("fact_id", factID),
		sdk.NewAttribute("claim_id", claim.Id),
		sdk.NewAttribute("domain", claim.Domain),
		sdk.NewAttribute("confidence", fmt.Sprintf("%d", fact.Confidence)),
	))

	return factID, nil
}

// handleChallengeSurvival restores a challenged fact and grants survival energy
// when a challenge claim is rejected (the original fact survived).
func (k Keeper) handleChallengeSurvival(ctx context.Context, challengeClaim *types.Claim) {
	if challengeClaim.ProvisionalFactId == "" {
		return
	}
	originalFact, found := k.GetFact(ctx, challengeClaim.ProvisionalFactId)
	if !found {
		return
	}
	params, _ := k.GetParams(ctx)

	// Energy boost for surviving a challenge
	originalFact.Energy += params.MetabolismEnergyChallengeSurvival
	if originalFact.Energy > params.MetabolismEnergyCap {
		originalFact.Energy = params.MetabolismEnergyCap
	}
	// Restore from challenged status
	originalFact.Status = types.FactStatus_FACT_STATUS_ACTIVE
	originalFact.AtRiskSinceEpoch = 0
	_ = k.SetFact(ctx, originalFact)
}

// distributeVerifierReward sends a verification reward to a verifier.
func (k Keeper) distributeVerifierReward(ctx context.Context, verifier string, amount uint64) {
	if k.bankKeeper == nil || amount == 0 {
		return
	}
	addr, err := sdk.AccAddressFromBech32(verifier)
	if err != nil {
		return
	}
	coins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(new(big.Int).SetUint64(amount))))
	_ = k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, addr, coins)
}

// roundHasDissent checks if a round had any verifier dissent (mixed accept/reject votes).
func roundHasDissent(round *types.VerificationRound) bool {
	hasAccept, hasReject := false, false
	for _, reveal := range round.Reveals {
		switch reveal.Vote {
		case "accept":
			hasAccept = true
		case "reject":
			hasReject = true
		}
		if hasAccept && hasReject {
			return true
		}
	}
	return false
}
