package keeper

import (
	"context"
	"fmt"
	"math/big"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/qualification/types"
)

// QualifyByStake creates a qualification via the stake pathway.
func (k Keeper) QualifyByStake(ctx context.Context, validator string, domain string, stakeAmount string) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params := k.GetParams(ctx)

	// Check not already qualified.
	if _, found := k.GetQualification(ctx, validator, domain); found {
		return fmt.Errorf("%w: %s/%s", types.ErrQualificationExists, validator, domain)
	}

	// Validate stake amount.
	amt := new(big.Int)
	if _, ok := amt.SetString(stakeAmount, 10); !ok || amt.Sign() <= 0 {
		return fmt.Errorf("%w: %s", types.ErrInvalidAmount, stakeAmount)
	}
	minStake := new(big.Int)
	minStake.SetString(params.MinStakeAmount, 10)
	if amt.Cmp(minStake) < 0 {
		return fmt.Errorf("%w: need %s, got %s", types.ErrInsufficientStake, params.MinStakeAmount, stakeAmount)
	}

	// Lock stake: send from validator account to module account.
	if k.bankKeeper != nil {
		senderAddr, err := sdk.AccAddressFromBech32(validator)
		if err != nil {
			return fmt.Errorf("%w: %s", types.ErrInvalidValidator, validator)
		}
		coins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(amt)))
		if err := k.bankKeeper.SendCoinsFromAccountToModule(ctx, senderAddr, types.ModuleName, coins); err != nil {
			return fmt.Errorf("failed to lock stake: %w", err)
		}
	}

	q := &types.DomainQualification{
		Validator:    validator,
		Domain:       domain,
		Pathway:      types.QualificationPathway_QUALIFICATION_PATHWAY_STAKE,
		Status:       types.QualificationStatus_QUALIFICATION_STATUS_ACTIVE,
		Weight:       50, // default weight for stake pathway
		StakedAmount: stakeAmount,
		GrantedAt:    uint64(sdkCtx.BlockHeight()),
		ExpiresAt:    uint64(sdkCtx.BlockHeight()) + params.QualificationPeriod,
		Metrics:      &types.QualificationMetrics{},
	}
	k.SetQualification(ctx, q)

	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.qualification.qualification_granted",
			sdk.NewAttribute("validator", validator),
			sdk.NewAttribute("domain", domain),
			sdk.NewAttribute("pathway", "stake"),
			sdk.NewAttribute("weight", fmt.Sprintf("%d", q.Weight)),
		),
	)
	return nil
}

// QualifyByTrackRecord creates a qualification via the track record pathway.
func (k Keeper) QualifyByTrackRecord(ctx context.Context, validator string, domain string) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params := k.GetParams(ctx)

	if _, found := k.GetQualification(ctx, validator, domain); found {
		return fmt.Errorf("%w: %s/%s", types.ErrQualificationExists, validator, domain)
	}

	// Check verification metrics. If capture defense keeper is available, check reputation.
	var totalVerifications, correctVerifications uint64
	var reputationOK bool

	if k.captureDefenseKeeper != nil {
		rep, found := k.captureDefenseKeeper.GetDomainReputation(ctx, validator, domain)
		if found && rep.Score >= params.MinReputationScore {
			reputationOK = true
		}
	} else {
		// Skip reputation check if keeper not available.
		reputationOK = true
	}

	if !reputationOK {
		return fmt.Errorf("%w: reputation score below minimum %d", types.ErrInsufficientTrackRecord, params.MinReputationScore)
	}

	// We need to check existing metrics. The caller should have recorded verifications
	// previously via RecordVerificationOutcome on a probationary or other qualification.
	// For now, we look for metrics stored elsewhere. In practice, we check if the validator
	// already has a metrics record from prior verification activity.
	// Since there's no existing qualification, we need the metrics to come from an external source.
	// For the initial implementation, we allow the track record pathway if the validator
	// meets the minimum requirements based on historical data.
	totalVerifications = 0
	correctVerifications = 0

	// Check if there was a prior (expired/withdrawn) qualification with metrics.
	// This is the bootstrap mechanism: validators accumulate metrics while unqualified
	// through the RecordVerificationOutcome cross-module method.
	existingQ, found := k.GetQualification(ctx, validator, domain)
	if found && existingQ.Metrics != nil {
		totalVerifications = existingQ.Metrics.TotalVerifications
		correctVerifications = existingQ.Metrics.CorrectVerifications
	}

	if totalVerifications < params.MinVerifications {
		return fmt.Errorf("%w: need %d, got %d (commitment 7: skill is current, not historical — the chain demands a demonstrated track record, not a one-time test)", types.ErrInsufficientTrackRecord, params.MinVerifications, totalVerifications)
	}

	accuracyBps := uint64(0)
	if totalVerifications > 0 {
		accuracyBps = (correctVerifications * 1000000) / totalVerifications
	}
	if accuracyBps < params.MinAccuracyBps {
		return fmt.Errorf("%w: need %d bps, got %d bps (commitment 7: skill is current — accuracy is the demonstration's reading and the bar is meaningfully high)", types.ErrInsufficientAccuracy, params.MinAccuracyBps, accuracyBps)
	}

	// Calculate weight based on accuracy: base 40 + up to 60 based on accuracy above minimum.
	weight := uint32(40)
	if accuracyBps > params.MinAccuracyBps {
		bonus := (accuracyBps - params.MinAccuracyBps) * 60 / (1000000 - params.MinAccuracyBps)
		weight += uint32(bonus)
	}
	if weight > 100 {
		weight = 100
	}

	q := &types.DomainQualification{
		Validator: validator,
		Domain:    domain,
		Pathway:   types.QualificationPathway_QUALIFICATION_PATHWAY_TRACK_RECORD,
		Status:    types.QualificationStatus_QUALIFICATION_STATUS_ACTIVE,
		Weight:    weight,
		GrantedAt: uint64(sdkCtx.BlockHeight()),
		ExpiresAt: uint64(sdkCtx.BlockHeight()) + params.QualificationPeriod,
		Metrics: &types.QualificationMetrics{
			TotalVerifications:    totalVerifications,
			CorrectVerifications:  correctVerifications,
			AccuracyBps:           accuracyBps,
			LastVerificationBlock: uint64(sdkCtx.BlockHeight()),
		},
	}
	k.SetQualification(ctx, q)

	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.qualification.qualification_granted",
			sdk.NewAttribute("validator", validator),
			sdk.NewAttribute("domain", domain),
			sdk.NewAttribute("pathway", "track_record"),
			sdk.NewAttribute("weight", fmt.Sprintf("%d", q.Weight)),
		),
	)
	return nil
}

// QualifyByCrossReference creates a qualification via cross-reference from another domain.
// Discount scales with depth difference: discount = CrossRefWeightDiscountBps * depthDiff.
func (k Keeper) QualifyByCrossReference(ctx context.Context, validator string, targetDomain string, sourceDomain string) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params := k.GetParams(ctx)

	if _, found := k.GetQualification(ctx, validator, targetDomain); found {
		return fmt.Errorf("%w: %s/%s", types.ErrQualificationExists, validator, targetDomain)
	}

	// Check source domain qualification.
	sourceQ, found := k.GetQualification(ctx, validator, sourceDomain)
	if !found || sourceQ.Status != types.QualificationStatus_QUALIFICATION_STATUS_ACTIVE {
		return fmt.Errorf("%w: %s/%s", types.ErrCrossRefNotQualified, validator, sourceDomain)
	}

	if uint64(sourceQ.Weight) < params.CrossRefMinWeight {
		return fmt.Errorf("%w: need %d, got %d", types.ErrCrossRefWeightTooLow, params.CrossRefMinWeight, sourceQ.Weight)
	}

	// Compute depth-scaled discount.
	depthDiff := k.getDepthDiff(ctx, targetDomain, sourceDomain)
	scaledDiscount := params.CrossRefWeightDiscountBps * uint64(depthDiff)
	if scaledDiscount > 1000000 {
		scaledDiscount = 1000000 // cap at 100%
	}

	discountedWeight := uint64(sourceQ.Weight) * (1000000 - scaledDiscount) / 1000000
	if discountedWeight < 1 {
		discountedWeight = 1
	}
	if discountedWeight > 100 {
		discountedWeight = 100
	}

	q := &types.DomainQualification{
		Validator:      validator,
		Domain:         targetDomain,
		Pathway:        types.QualificationPathway_QUALIFICATION_PATHWAY_CROSS_REFERENCE,
		Status:         types.QualificationStatus_QUALIFICATION_STATUS_ACTIVE,
		Weight:         uint32(discountedWeight),
		GrantedAt:      uint64(sdkCtx.BlockHeight()),
		ExpiresAt:      uint64(sdkCtx.BlockHeight()) + params.QualificationPeriod,
		CrossRefDomain: sourceDomain,
		Metrics:        &types.QualificationMetrics{},
	}
	k.SetQualification(ctx, q)

	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.qualification.qualification_granted",
			sdk.NewAttribute("validator", validator),
			sdk.NewAttribute("domain", targetDomain),
			sdk.NewAttribute("pathway", "cross_reference"),
			sdk.NewAttribute("source_domain", sourceDomain),
			sdk.NewAttribute("weight", fmt.Sprintf("%d", q.Weight)),
		),
	)
	return nil
}

// MaxInheritanceDepthDiff is the maximum depth difference allowed for inheritance.
const MaxInheritanceDepthDiff = 3

// QualifyByInheritance creates a qualification via stratum inheritance.
// Discount scales with depth difference: discount = InheritanceWeightDiscountBps * depthDiff.
// Blocked if depth difference exceeds MaxInheritanceDepthDiff.
func (k Keeper) QualifyByInheritance(ctx context.Context, validator string, targetDomain string, parentDomain string) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params := k.GetParams(ctx)

	if _, found := k.GetQualification(ctx, validator, targetDomain); found {
		return fmt.Errorf("%w: %s/%s", types.ErrQualificationExists, validator, targetDomain)
	}

	// Check parent domain qualification.
	parentQ, found := k.GetQualification(ctx, validator, parentDomain)
	if !found || parentQ.Status != types.QualificationStatus_QUALIFICATION_STATUS_ACTIVE {
		return fmt.Errorf("%w: %s/%s", types.ErrInheritanceNotQualified, validator, parentDomain)
	}

	// Compute depth-scaled discount with max distance enforcement.
	depthDiff := k.getDepthDiff(ctx, targetDomain, parentDomain)
	if depthDiff > MaxInheritanceDepthDiff {
		return fmt.Errorf("%w: depth distance %d exceeds max %d", types.ErrDepthDistanceTooLarge, depthDiff, MaxInheritanceDepthDiff)
	}

	scaledDiscount := params.InheritanceWeightDiscountBps * uint64(depthDiff)
	if scaledDiscount > 1000000 {
		scaledDiscount = 1000000 // cap at 100%
	}

	discountedWeight := uint64(parentQ.Weight) * (1000000 - scaledDiscount) / 1000000
	if discountedWeight < 1 {
		discountedWeight = 1
	}
	if discountedWeight > 100 {
		discountedWeight = 100
	}

	q := &types.DomainQualification{
		Validator:    validator,
		Domain:       targetDomain,
		Pathway:      types.QualificationPathway_QUALIFICATION_PATHWAY_INHERITANCE,
		Status:       types.QualificationStatus_QUALIFICATION_STATUS_ACTIVE,
		Weight:       uint32(discountedWeight),
		GrantedAt:    uint64(sdkCtx.BlockHeight()),
		ExpiresAt:    uint64(sdkCtx.BlockHeight()) + params.QualificationPeriod,
		ParentDomain: parentDomain,
		Metrics:      &types.QualificationMetrics{},
	}
	k.SetQualification(ctx, q)

	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.qualification.qualification_granted",
			sdk.NewAttribute("validator", validator),
			sdk.NewAttribute("domain", targetDomain),
			sdk.NewAttribute("pathway", "inheritance"),
			sdk.NewAttribute("parent_domain", parentDomain),
			sdk.NewAttribute("weight", fmt.Sprintf("%d", q.Weight)),
		),
	)
	return nil
}

// getDepthDiff returns the absolute depth difference between two domains.
// Falls back to 1 if ontology keeper is unavailable or domain not found.
func (k Keeper) getDepthDiff(ctx context.Context, domainA, domainB string) uint32 {
	if k.ontologyKeeper == nil {
		return 1 // default to 1 if ontology keeper not wired
	}
	depthA, errA := k.ontologyKeeper.GetDepthForDomain(ctx, domainA)
	depthB, errB := k.ontologyKeeper.GetDepthForDomain(ctx, domainB)
	if errA != nil || errB != nil {
		return 1
	}
	if depthA >= depthB {
		return depthA - depthB
	}
	return depthB - depthA
}
