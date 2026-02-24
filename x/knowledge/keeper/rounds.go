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

	switch result.Verdict {
	case types.Verdict_VERDICT_ACCEPT:
		// Create fact from accepted claim
		if err := k.createFactFromClaim(ctx, claim, round, result.Confidence); err != nil {
			return err
		}
		// Review fee already distributed at submission time — no refund.
		claim.Status = types.ClaimStatus_CLAIM_STATUS_ACCEPTED

	case types.Verdict_VERDICT_REJECT:
		// Review fee already distributed at submission time — the fee IS the cost of rejection.
		claim.Status = types.ClaimStatus_CLAIM_STATUS_REJECTED

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
	for _, slash := range result.Slashes {
		if k.stakingKeeper != nil {
			_ = k.stakingKeeper.SlashValidator(ctx, slash.Verifier, slash.SlashBps)
		}
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
func (k Keeper) createFactFromClaim(ctx context.Context, claim *types.Claim, round *types.VerificationRound, confidence uint64) error {
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
	}

	// Apply stratum confidence ceiling if ontology keeper is available
	if k.ontologyKeeper != nil && claim.Domain != "" {
		stratum, err := k.ontologyKeeper.GetStratumForDomain(ctx, claim.Domain)
		if err == nil && stratum != "" {
			ceiling, err := k.ontologyKeeper.GetConfidenceCeiling(ctx, stratum)
			if err == nil && ceiling > 0 && fact.Confidence > ceiling {
				fact.Confidence = ceiling
			}
			fact.Stratum = stratum
		}
	}

	if err := k.SetFact(ctx, fact); err != nil {
		return err
	}

	// Index fact by structured subject and tags
	if fact.Structure != nil {
		if err := k.IndexFactBySubject(ctx, fact); err != nil {
			return fmt.Errorf("failed to index fact by subject: %w", err)
		}
	}

	// Index fact by canonical hash
	if fact.CanonicalHash != "" {
		if err := k.SetCanonicalHash(ctx, fact.CanonicalHash, fact.Id); err != nil {
			return fmt.Errorf("failed to index fact by canonical hash: %w", err)
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
			return fmt.Errorf("failed to store fact relation: %w", err)
		}

		sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
			"zerone.knowledge.fact_relation_created",
			sdk.NewAttribute("source", factRel.SourceFactId),
			sdk.NewAttribute("target", factRel.TargetFactId),
			sdk.NewAttribute("relation", factRel.Relation.String()),
		))
	}

	// Create vesting schedule via vesting_rewards keeper
	if k.vestingRewardsKeeper != nil {
		_ = k.vestingRewardsKeeper.CreateVestingScheduleFromKnowledge(
			ctx, claim.Id, factID, claim.Submitter, claim.Stake, claim.Category,
		)
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.knowledge.fact_created",
		sdk.NewAttribute("fact_id", factID),
		sdk.NewAttribute("claim_id", claim.Id),
		sdk.NewAttribute("domain", claim.Domain),
		sdk.NewAttribute("confidence", fmt.Sprintf("%d", fact.Confidence)),
	))

	return nil
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
