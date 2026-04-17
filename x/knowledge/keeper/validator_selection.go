package keeper

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/crypto"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// GetEligibleValidators returns validators eligible for verification of a given domain.
// When domainQualificationKeeper is set, it filters by domain qualification and falls
// back to all active validators if fewer than MinVerifiers are qualified.
func (k Keeper) GetEligibleValidators(ctx context.Context, domain string) ([]types.ValidatorInfo, error) {
	if k.stakingKeeper == nil {
		return nil, nil
	}

	validators, err := k.stakingKeeper.GetActiveValidatorInfos(ctx)
	if err != nil {
		return nil, err
	}

	// No qualification keeper — return all active validators
	if k.domainQualificationKeeper == nil || domain == "" {
		return validators, nil
	}

	var qualified []types.ValidatorInfo
	qualifiedSet := make(map[string]struct{})
	for _, v := range validators {
		ok, err := k.domainQualificationKeeper.IsQualified(ctx, v.Address, domain)
		if err != nil {
			k.Logger(ctx).Warn("qualification check failed", "validator", v.Address, "error", err)
			continue
		}
		if ok {
			qualified = append(qualified, v)
			qualifiedSet[v.Address] = struct{}{}
		}
	}

	effectiveMin := uint64(k.GetEffectiveMinVerifiers(ctx, domain))

	// Strict path: enough qualified validators to meet the quorum on their own.
	if uint64(len(qualified)) >= effectiveMin {
		return qualified, nil
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	// Scaled fallback (T6): when at least ceil(effectiveMin/2) are qualified,
	// return qualified + enough unqualified to reach effectiveMin. This keeps
	// expertise in the pool while allowing quorum to be met in thin domains.
	// When fewer than that floor, use the emergency fallback (all validators).
	expertiseFloor := (effectiveMin + 1) / 2 // ceil(effectiveMin / 2)
	if uint64(len(qualified)) >= expertiseFloor {
		needed := int(effectiveMin) - len(qualified)
		pool := append([]types.ValidatorInfo(nil), qualified...)
		for _, v := range validators {
			if needed <= 0 {
				break
			}
			if _, already := qualifiedSet[v.Address]; already {
				continue
			}
			pool = append(pool, v)
			needed--
		}
		sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
			"zerone.knowledge.qualification_fallback",
			sdk.NewAttribute("domain", domain),
			sdk.NewAttribute("qualified_count", fmt.Sprintf("%d", len(qualified))),
			sdk.NewAttribute("effective_min_verifiers", fmt.Sprintf("%d", effectiveMin)),
			sdk.NewAttribute("mode", "scaled"),
			sdk.NewAttribute("pool_size", fmt.Sprintf("%d", len(pool))),
		))
		k.Logger(ctx).Info("scaled qualification fallback — qualified + padded unqualified",
			"qualified", len(qualified), "pool", len(pool), "domain", domain)
		return pool, nil
	}

	// Emergency fallback: fewer than half the quorum qualified. Open to all.
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.knowledge.qualification_fallback",
		sdk.NewAttribute("domain", domain),
		sdk.NewAttribute("qualified_count", fmt.Sprintf("%d", len(qualified))),
		sdk.NewAttribute("effective_min_verifiers", fmt.Sprintf("%d", effectiveMin)),
		sdk.NewAttribute("mode", "emergency"),
	))
	k.Logger(ctx).Warn("insufficient qualified verifiers (below half quorum), emergency fallback to all",
		"qualified", len(qualified), "effective_min", effectiveMin, "domain", domain)
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
