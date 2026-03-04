# R38-2 — Access & Payment

## Objective

Implement sample and dataset access with automatic payment processing. When a consumer accesses training data, they pay and the payment is held for distribution.

## Tasks

### 1. AccessSample

```go
func (k Keeper) AccessSample(ctx context.Context, msg *types.MsgAccessSample) (*types.MsgAccessSampleResponse, error) {
    sample, found := k.GetSample(ctx, msg.SampleId)
    if !found { return nil, types.ErrSampleNotFound }

    // 1. Calculate price (base fee + quality modifier)
    price := k.calculateSamplePrice(ctx, sample)

    // 2. Verify consumer can pay (max_payment >= price)
    if parseCoins(msg.MaxPayment).IsLT(price) {
        return nil, types.ErrInsufficientPayment
    }

    // 3. Transfer payment from consumer to module account
    k.bankKeeper.SendCoinsFromAccountToModule(ctx, consumer, types.ModuleName, price)

    // 4. Record access (increment counter, restore energy)
    sample.AccessCount++
    k.restoreEnergyOnAccess(ctx, sample, params)
    sample.LastAccessedBlock = currentBlock
    k.SetSample(ctx, sample)

    // 5. Queue revenue distribution (batched, not per-access)
    k.queueRevenueDistribution(ctx, sample.Id, price)

    // 6. Emit event with sample content (or content hash for verification)
    return &types.MsgAccessSampleResponse{Payment: price.String()}, nil
}
```

### 2. AccessDataset (bulk)

```go
func (k Keeper) AccessDataset(ctx context.Context, msg *types.MsgAccessDataset) (*types.MsgAccessDatasetResponse, error) {
    dataset, found := k.GetDataset(ctx, msg.DatasetId)
    if !found { return nil, types.ErrDatasetNotFound }

    // 1. Calculate bulk price
    //    - If dataset.BulkPrice set: use it
    //    - Otherwise: sum of individual sample prices with 20% bulk discount
    price := k.calculateDatasetPrice(ctx, dataset)

    // 2. Process payment
    // 3. Record access for all matching samples
    // 4. Queue revenue distribution (per-sample pro-rata)

    return &types.MsgAccessDatasetResponse{
        Payment: price.String(),
        SampleCount: dataset.SampleCount,
    }, nil
}
```

### 3. Price Calculation

```go
func (k Keeper) calculateSamplePrice(ctx context.Context, sample *types.Sample) sdk.Coin {
    params := k.GetParams(ctx)
    basePrice := parseCoins(params.AccessFeePerSample)

    // Quality multiplier
    var multiplier uint64
    switch sample.QualityTier {
    case "gold":   multiplier = params.GoldQualityMultiplier   // 30000 = 3x
    case "silver": multiplier = params.SilverQualityMultiplier // 20000 = 2x
    default:       multiplier = params.BronzeQualityMultiplier // 10000 = 1x
    }
    price := basePrice.Amount.Mul(sdk.NewInt(int64(multiplier))).Quo(sdk.NewInt(10000))
    return sdk.NewCoin("uzrn", price)
}
```

### 4. Revenue Queue

Don't distribute revenue on every access (too expensive). Batch it:

```go
// Queue entry: sample_id → accumulated undistributed revenue
func (k Keeper) queueRevenueDistribution(ctx context.Context, sampleId string, amount sdk.Coin) {
    current := k.GetPendingRevenue(ctx, sampleId)
    k.SetPendingRevenue(ctx, sampleId, current.Add(amount))
}
```

Distribution happens in EndBlocker (batched per epoch).

### 5. Tests

- Access individual sample → correct payment
- Access sample → energy restored
- Access sample → counter incremented
- Access dataset → bulk pricing
- Insufficient payment rejected
- Sample not found → error
- Dataset not found → error
- Quality multiplier correctly applied
- Revenue queued correctly
- Multiple accesses accumulate in queue

Target: ≥ 15 tests.
