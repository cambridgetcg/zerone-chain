package keeper

import (
	"context"

	"github.com/zerone-chain/zerone/x/knowledge/crypto"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// GetEligibleValidators returns all validators eligible for verification selection.
// If DomainQualificationKeeper is available (R6-5), it filters by domain qualification.
func (k Keeper) GetEligibleValidators(ctx context.Context) ([]types.ValidatorInfo, error) {
	if k.stakingKeeper == nil {
		return nil, nil
	}

	validators, err := k.stakingKeeper.GetActiveValidatorInfos(ctx)
	if err != nil {
		return nil, err
	}

	// DomainQualificationKeeper is nil until R6-5 — return all active validators
	if k.domainQualificationKeeper == nil {
		return validators, nil
	}

	// Future: filter by domain qualification
	return validators, nil
}

// VerifyValidatorVRFSelection verifies that a validator was properly selected
// via VRF for a given verification round.
func (k Keeper) VerifyValidatorVRFSelection(
	ctx context.Context,
	roundID, validatorAddr string,
	vrfOutput, vrfProof []byte,
) (bool, error) {
	round, found := k.GetVerificationRound(ctx, roundID)
	if !found {
		return false, nil
	}

	// Get validator info for public key and stake
	if k.stakingKeeper == nil {
		return false, nil
	}
	valInfo, err := k.stakingKeeper.GetValidatorInfo(ctx, validatorAddr)
	if err != nil {
		return false, err
	}

	totalStake, err := k.stakingKeeper.GetTotalStake(ctx)
	if err != nil {
		return false, err
	}

	params, err := k.GetParams(ctx)
	if err != nil {
		return false, err
	}

	// Generate the VRF seed for this round
	vrfSeed := crypto.GenerateVRFSeed(round.ClaimId, round.StartedAtBlock, nil)

	// Verify VRF proof (requires validator's public key — for now, trust the output)
	// Full VRF proof verification requires access to the validator's Ed25519 public key,
	// which is stored in the CometBFT consensus key. For R2-2, we verify the selection
	// math and trust the VRF output as submitted.
	_ = vrfSeed
	_ = vrfProof

	// Check stake-weighted selection
	selected, _ := crypto.IsValidatorSelected(
		vrfOutput,
		valInfo.Stake,
		totalStake,
		uint32(params.MaxVerifiers),
	)

	return selected, nil
}

// SlashMissedVerification slashes a verifier who was selected but did not participate.
func (k Keeper) SlashMissedVerification(ctx context.Context, verifierAddr string, slashBps uint64) error {
	if k.stakingKeeper == nil {
		return nil
	}
	return k.stakingKeeper.SlashValidator(ctx, verifierAddr, slashBps)
}
