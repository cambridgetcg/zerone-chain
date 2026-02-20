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
		"verification_round_created",
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
		// Return claim stake to submitter
		if err := k.returnClaimStake(ctx, claim); err != nil {
			k.Logger(ctx).Error("failed to return claim stake", "claim_id", claim.Id, "error", err)
		}
		claim.Status = types.ClaimStatus_CLAIM_STATUS_ACCEPTED

	case types.Verdict_VERDICT_REJECT:
		// Slash and burn the claim stake
		params, _ := k.GetParams(ctx)
		if err := k.slashAndBurnClaimStake(ctx, claim, params.InvalidClaimSlashBps); err != nil {
			k.Logger(ctx).Error("failed to slash claim stake", "claim_id", claim.Id, "error", err)
		}
		claim.Status = types.ClaimStatus_CLAIM_STATUS_REJECTED

	case types.Verdict_VERDICT_INCONCLUSIVE:
		// Return claim stake — insufficient evidence
		if err := k.returnClaimStake(ctx, claim); err != nil {
			k.Logger(ctx).Error("failed to return claim stake", "claim_id", claim.Id, "error", err)
		}
		claim.Status = types.ClaimStatus_CLAIM_STATUS_INSUFFICIENT
	}

	if err := k.SetClaim(ctx, claim); err != nil {
		return err
	}
	if err := k.SetVerificationRound(ctx, round); err != nil {
		return err
	}

	// Distribute rewards and slashes to verifiers
	for _, reward := range result.Rewards {
		k.distributeVerifierReward(ctx, reward.Verifier, reward.Amount)
	}
	for _, slash := range result.Slashes {
		if k.stakingKeeper != nil {
			_ = k.stakingKeeper.SlashValidator(ctx, slash.Verifier, slash.SlashBps)
		}
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"verification_round_completed",
		sdk.NewAttribute("round_id", round.Id),
		sdk.NewAttribute("claim_id", round.ClaimId),
		sdk.NewAttribute("verdict", round.Verdict.String()),
	))

	return nil
}

// returnClaimStake sends the locked claim stake back to the submitter.
func (k Keeper) returnClaimStake(ctx context.Context, claim *types.Claim) error {
	if k.bankKeeper == nil || claim.Stake == "" {
		return nil
	}
	stakeAmt, ok := new(big.Int).SetString(claim.Stake, 10)
	if !ok || stakeAmt.Sign() <= 0 {
		return nil
	}

	recipientAddr, err := sdk.AccAddressFromBech32(claim.Submitter)
	if err != nil {
		return fmt.Errorf("invalid submitter address: %w", err)
	}

	coins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(stakeAmt)))
	return k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, recipientAddr, coins)
}

// slashAndBurnClaimStake burns a portion of the claim stake and returns the remainder.
func (k Keeper) slashAndBurnClaimStake(ctx context.Context, claim *types.Claim, slashBps uint64) error {
	if k.bankKeeper == nil || claim.Stake == "" {
		return nil
	}
	stakeAmt, ok := new(big.Int).SetString(claim.Stake, 10)
	if !ok || stakeAmt.Sign() <= 0 {
		return nil
	}

	// Calculate slash amount
	slashAmt := safeMulDiv(stakeAmt.Uint64(), slashBps, 1_000_000)
	if slashAmt > 0 {
		burnCoins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(new(big.Int).SetUint64(slashAmt))))
		if err := k.bankKeeper.BurnCoins(ctx, types.ModuleName, burnCoins); err != nil {
			return fmt.Errorf("failed to burn slashed stake: %w", err)
		}
	}

	// Return remainder to submitter
	remainder := stakeAmt.Uint64() - slashAmt
	if remainder > 0 {
		recipientAddr, err := sdk.AccAddressFromBech32(claim.Submitter)
		if err != nil {
			return err
		}
		returnCoins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(new(big.Int).SetUint64(remainder))))
		return k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, recipientAddr, returnCoins)
	}

	return nil
}

// createFactFromClaim creates a new Fact from an accepted claim.
func (k Keeper) createFactFromClaim(ctx context.Context, claim *types.Claim, round *types.VerificationRound, confidence uint64) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	height := uint64(sdkCtx.BlockHeight())

	factID := GenerateFactID(claim.Id, height)

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

	// Create vesting schedule via vesting_rewards keeper
	if k.vestingRewardsKeeper != nil {
		_ = k.vestingRewardsKeeper.CreateVestingScheduleFromKnowledge(
			ctx, claim.Id, factID, claim.Submitter, claim.Stake, claim.Category,
		)
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"fact_created",
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
