# R38-2 Access & Payment — Design

## Summary

Sample and dataset access with automatic payment processing. Prices calculated from base fee + quality multiplier. Revenue queued for batched distribution.

## Components

### AccessSample (access.go)
- `calculateSamplePrice`: base_fee * quality_multiplier / 10000
- Quality tiers: gold=3x, silver=2x, bronze=1x (from params)
- Updates: access_count++, last_accessed_block, total_revenue
- Calls RestoreEnergyOnAccess (ecology.go)
- Queues revenue via PendingRevenue store

### AccessDataset (access.go → AccessDatasetWithPricing)
- If bulk_price set: use it directly
- Otherwise: sum individual prices with 20% bulk discount
- Updates each matching sample (access_count, energy, revenue)
- Per-sample revenue = total_price / sample_count
- 95% curator commission, 5% module retention

### Revenue Queue (state.go)
- PendingRevenuePrefix (0xAA): sampleID → uint64 accumulated uzrn
- Get/Set/Delete/Iterate CRUD
- Distribution deferred to EndBlocker (future task)

## Pricing Formula
```
price = AccessFeePerSample × QualityMultiplier / 10000
bulk  = sum(individual prices) × (1 - 0.20)
```
