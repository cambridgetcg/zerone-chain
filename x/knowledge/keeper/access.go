package keeper

import (
	"context"
	"fmt"
	"strconv"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

const (
	// BulkDiscountBPS is the discount for dataset bulk access (20%).
	BulkDiscountBPS = 200_000
	// QualityMultiplierDenom is the denominator for quality multiplier BPS.
	QualityMultiplierDenom = 10_000
)

// AccessSample processes an individual sample access with payment.
func (k Keeper) AccessSample(ctx context.Context, msg *types.MsgAccessSample) (*types.MsgAccessSampleResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	sample, found := k.GetSample(ctx, msg.SampleId)
	if !found {
		return nil, types.ErrSampleNotFound.Wrapf("sample %q not found", msg.SampleId)
	}

	if !isActiveSample(sample.Status) {
		return nil, types.ErrSampleNotFound.Wrap("sample is not active")
	}

	params, err := k.GetParams(ctx)
	if err != nil {
		return nil, err
	}

	// Calculate price
	price := k.calculateSamplePrice(sample, params)
	if !price.IsPositive() {
		return nil, types.ErrInsufficientPayment.Wrap("calculated price is zero")
	}

	// Verify max_payment
	if msg.MaxPayment != "" {
		maxPay, ok := sdkmath.NewIntFromString(msg.MaxPayment)
		if ok && price.GT(maxPay) {
			return nil, types.ErrInsufficientPayment.Wrapf("price %s exceeds max_payment %s", price.String(), msg.MaxPayment)
		}
	}

	// Transfer payment: consumer → module
	consumerAddr, err := sdk.AccAddressFromBech32(msg.Consumer)
	if err != nil {
		return nil, err
	}
	coins := sdk.NewCoins(sdk.NewCoin("uzrn", price))
	if err := k.bankKeeper.SendCoinsFromAccountToModule(ctx, consumerAddr, types.ModuleName, coins); err != nil {
		return nil, types.ErrInsufficientPayment.Wrap(err.Error())
	}

	// Update sample state
	sample.AccessCount++
	sample.LastAccessedBlock = uint64(sdkCtx.BlockHeight())
	addRevenue(sample, price)

	// Restore energy
	k.RestoreEnergyOnAccess(ctx, sample, params)

	if err := k.SetSample(ctx, sample); err != nil {
		return nil, err
	}

	// Reinforce TDU fitness via UsageCorrelation signal
	if _, found := k.GetFitnessRecord(ctx, msg.SampleId); found {
		fitnessParams := k.GetFitnessDecayParams(ctx)
		currentCycle := uint64(sdkCtx.BlockHeight()) / fitnessParams.GetFitnessEpochBlocks()
		usageSignal := types.FitnessSignal{
			TrainingInfluence: sdkmath.LegacyNewDecWithPrec(5, 1), // 0.5 neutral
			UsageCorrelation:  sdkmath.LegacyOneDec(),             // 1.0 max positive
			Redundancy:        sdkmath.LegacyNewDecWithPrec(5, 1), // 0.5 neutral
		}
		_ = k.UpdateFitnessScoreWithEvent(ctx, msg.SampleId, usageSignal, currentCycle)
	}

	// Queue revenue for batched distribution
	k.queueRevenueDistribution(ctx, sample.Id, price)

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"sample_accessed",
		sdk.NewAttribute("sample_id", msg.SampleId),
		sdk.NewAttribute("consumer", msg.Consumer),
		sdk.NewAttribute("payment", price.String()),
	))

	return &types.MsgAccessSampleResponse{Payment: price.String()}, nil
}

// calculateSamplePrice computes the access price for a single sample.
// price = base_fee * quality_multiplier / 10000
func (k Keeper) calculateSamplePrice(sample *types.Sample, params *types.Params) sdkmath.Int {
	baseFee := parseUzrn(params.AccessFeePerSample)
	if baseFee == 0 {
		return sdkmath.ZeroInt()
	}

	multiplier := k.getQualityMultiplier(sample, params)

	price := sdkmath.NewInt(int64(baseFee)).
		Mul(sdkmath.NewInt(int64(multiplier))).
		Quo(sdkmath.NewInt(QualityMultiplierDenom))

	return price
}

// getQualityMultiplier returns the quality-tier multiplier for a sample.
func (k Keeper) getQualityMultiplier(sample *types.Sample, params *types.Params) uint64 {
	switch sample.QualityTier {
	case "gold":
		if params.GoldQualityMultiplier > 0 {
			return params.GoldQualityMultiplier
		}
		return 30_000
	case "silver":
		if params.SilverQualityMultiplier > 0 {
			return params.SilverQualityMultiplier
		}
		return 20_000
	default: // bronze or unspecified
		if params.BronzeQualityMultiplier > 0 {
			return params.BronzeQualityMultiplier
		}
		return 10_000
	}
}

// calculateDatasetPrice computes the total price for bulk dataset access.
// If bulk_price is set, use it. Otherwise, sum individual prices with 20% discount.
func (k Keeper) calculateDatasetPrice(ctx context.Context, dataset *types.Dataset, params *types.Params) sdkmath.Int {
	if dataset.BulkPrice != "" && dataset.BulkPrice != "0" {
		price, ok := sdkmath.NewIntFromString(dataset.BulkPrice)
		if ok && price.IsPositive() {
			return price
		}
	}

	// Sum individual sample prices with bulk discount
	var totalPrice sdkmath.Int = sdkmath.ZeroInt()

	if dataset.Domain != "" {
		sampleIDs := k.GetSamplesByDomain(ctx, dataset.Domain)
		for _, id := range sampleIDs {
			sample, found := k.GetSample(ctx, id)
			if !found {
				continue
			}
			if matchesDatasetFilter(sample, dataset) {
				totalPrice = totalPrice.Add(k.calculateSamplePrice(sample, params))
			}
		}
	} else {
		k.IterateSamples(ctx, func(sample *types.Sample) bool {
			if matchesDatasetFilter(sample, dataset) {
				totalPrice = totalPrice.Add(k.calculateSamplePrice(sample, params))
			}
			return false
		})
	}

	// Apply 20% bulk discount
	discount := totalPrice.Mul(sdkmath.NewInt(BulkDiscountBPS)).Quo(sdkmath.NewInt(MaxBPSDenom))
	return totalPrice.Sub(discount)
}

// queueRevenueDistribution adds to the pending revenue for a sample.
func (k Keeper) queueRevenueDistribution(ctx context.Context, sampleID string, amount sdkmath.Int) {
	current := k.GetPendingRevenue(ctx, sampleID)
	newTotal := current + amount.Uint64()
	_ = k.SetPendingRevenue(ctx, sampleID, newTotal)
}

// addRevenue increments the sample's total_revenue field.
func addRevenue(sample *types.Sample, amount sdkmath.Int) {
	current := parseUzrn(sample.TotalRevenue)
	newTotal := current + amount.Uint64()
	sample.TotalRevenue = strconv.FormatUint(newTotal, 10)
}

// updateSamplesOnDatasetAccess updates access_count, energy, and revenue for all matching samples.
func (k Keeper) updateSamplesOnDatasetAccess(ctx context.Context, dataset *types.Dataset, params *types.Params, perSampleRevenue sdkmath.Int) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	blockHeight := uint64(sdkCtx.BlockHeight())

	updateFn := func(sample *types.Sample) {
		sample.AccessCount++
		sample.LastAccessedBlock = blockHeight
		addRevenue(sample, perSampleRevenue)
		k.RestoreEnergyOnAccess(ctx, sample, params)
		_ = k.SetSample(ctx, sample)

		if perSampleRevenue.IsPositive() {
			k.queueRevenueDistribution(ctx, sample.Id, perSampleRevenue)
		}
	}

	if dataset.Domain != "" {
		sampleIDs := k.GetSamplesByDomain(ctx, dataset.Domain)
		for _, id := range sampleIDs {
			sample, found := k.GetSample(ctx, id)
			if !found {
				continue
			}
			if matchesDatasetFilter(sample, dataset) {
				updateFn(sample)
			}
		}
	} else {
		k.IterateSamples(ctx, func(sample *types.Sample) bool {
			if matchesDatasetFilter(sample, dataset) {
				updateFn(sample)
			}
			return false
		})
	}
}

// accessDatasetMatchingSampleCount counts matching samples without updating them.
func (k Keeper) accessDatasetMatchingSampleCount(ctx context.Context, dataset *types.Dataset) uint64 {
	count, _ := k.countMatchingSamples(ctx, dataset)
	return count
}

// AccessDatasetWithPricing processes a dataset access request with proper bulk pricing.
func (k Keeper) AccessDatasetWithPricing(ctx context.Context, msg *types.MsgAccessDataset) (*types.MsgAccessDatasetResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	dataset, found := k.GetDataset(ctx, msg.DatasetId)
	if !found {
		return nil, types.ErrDatasetNotFound.Wrapf("dataset %q not found", msg.DatasetId)
	}

	params, err := k.GetParams(ctx)
	if err != nil {
		return nil, err
	}

	// Calculate bulk price
	totalPrice := k.calculateDatasetPrice(ctx, dataset, params)
	if !totalPrice.IsPositive() {
		return nil, types.ErrInsufficientPayment.Wrap("dataset has no valid price")
	}

	// Verify max_payment
	if msg.MaxPayment != "" {
		maxPay, ok := sdkmath.NewIntFromString(msg.MaxPayment)
		if ok && totalPrice.GT(maxPay) {
			return nil, types.ErrInsufficientPayment.Wrapf("price %s exceeds max_payment %s", totalPrice.String(), msg.MaxPayment)
		}
	}

	// Transfer payment: consumer → module
	consumerAddr, err := sdk.AccAddressFromBech32(msg.Consumer)
	if err != nil {
		return nil, err
	}
	coins := sdk.NewCoins(sdk.NewCoin("uzrn", totalPrice))
	if err := k.bankKeeper.SendCoinsFromAccountToModule(ctx, consumerAddr, types.ModuleName, coins); err != nil {
		return nil, types.ErrInsufficientPayment.Wrap(err.Error())
	}

	// Curator commission (95%)
	curatorShare := totalPrice.Mul(sdkmath.NewInt(CuratorCommissionBPS)).Quo(sdkmath.NewInt(MaxBPSDenom))
	if curatorShare.IsPositive() {
		curatorAddr, err := sdk.AccAddressFromBech32(dataset.Curator)
		if err == nil {
			curatorCoins := sdk.NewCoins(sdk.NewCoin("uzrn", curatorShare))
			_ = k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, curatorAddr, curatorCoins)
		}
	}

	// Count matching samples and compute per-sample revenue share
	sampleCount := k.accessDatasetMatchingSampleCount(ctx, dataset)
	var perSampleRevenue sdkmath.Int
	if sampleCount > 0 {
		perSampleRevenue = totalPrice.Quo(sdkmath.NewInt(int64(sampleCount)))
	}

	// Update all matching samples
	if sampleCount > 0 {
		k.updateSamplesOnDatasetAccess(ctx, dataset, params, perSampleRevenue)
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"dataset_accessed",
		sdk.NewAttribute("dataset_id", msg.DatasetId),
		sdk.NewAttribute("consumer", msg.Consumer),
		sdk.NewAttribute("payment", totalPrice.String()),
		sdk.NewAttribute("sample_count", fmt.Sprintf("%d", sampleCount)),
	))

	return &types.MsgAccessDatasetResponse{
		Payment:     totalPrice.String(),
		SampleCount: sampleCount,
	}, nil
}
