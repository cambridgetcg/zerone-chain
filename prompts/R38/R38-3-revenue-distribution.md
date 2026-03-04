# R38-3 — Revenue Distribution Engine

## Objective

Implement the revenue split: submitters, validators, protocol, and curators all get their share. Quality and consent multipliers adjust submitter earnings.

## Tasks

### 1. Epoch Revenue Distribution

In EndBlocker at epoch boundary:

```go
func (k Keeper) distributeEpochRevenue(ctx context.Context, params *types.Params) {
    k.IteratePendingRevenue(ctx, func(sampleId string, amount sdk.Coin) bool {
        sample := k.GetSample(ctx, sampleId)

        // Split revenue:
        submitterShare := amount.Amount.Mul(params.SubmitterRevenueShareBps).Quo(10000)  // 55%
        validatorShare := amount.Amount.Mul(params.ValidatorRevenueShareBps).Quo(10000)  // 22%
        protocolShare  := amount.Amount.Sub(submitterShare).Sub(validatorShare)            // remainder

        // Apply consent multiplier to submitter share
        consentMultiplier := k.getConsentMultiplier(sample.Consent.Type, params)
        adjustedSubmitterShare := submitterShare.Mul(consentMultiplier).Quo(10000)

        // Difference goes to protocol (reward ethical sourcing)
        consentBonus := submitterShare.Sub(adjustedSubmitterShare)
        protocolShare = protocolShare.Add(consentBonus)

        // Send to submitter
        k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, sample.Submitter, adjustedSubmitterShare)

        // Distribute validator share to round validators (proportional to accuracy)
        k.distributeToValidators(ctx, sample.SubmissionId, validatorShare)

        // Protocol share: to protocol fee collector
        // This feeds into vesting_rewards module for further split

        // Update sample revenue tracking
        sample.TotalRevenue = addUzrn(sample.TotalRevenue, amount)
        k.SetSample(ctx, sample)

        // Clear pending
        k.DeletePendingRevenue(ctx, sampleId)
        return false
    })
}
```

### 2. Consent Multipliers

```go
func (k Keeper) getConsentMultiplier(consentType types.ConsentType, params *types.Params) sdk.Int {
    switch consentType {
    case types.CONSENT_TYPE_SELF_AUTHORED: return sdk.NewInt(int64(params.SelfAuthoredMultiplier))  // 15000
    case types.CONSENT_TYPE_OPT_IN:       return sdk.NewInt(int64(params.OptInMultiplier))          // 13000
    case types.CONSENT_TYPE_PUBLIC_LICENSE: return sdk.NewInt(int64(params.PublicLicenseMultiplier)) // 10000
    case types.CONSENT_TYPE_PLATFORM_TOS: return sdk.NewInt(int64(params.PlatformTosMultiplier))    // 8000
    case types.CONSENT_TYPE_FAIR_USE:     return sdk.NewInt(int64(params.FairUseMultiplier))        // 5000
    default:                               return sdk.NewInt(10000) // 1x default
    }
}
```

Self-authored content earns 50% more than the base rate. Fair use earns 50% less. This creates a strong economic incentive for ethical data sourcing.

### 3. Validator Revenue Distribution

```go
func (k Keeper) distributeToValidators(ctx context.Context, submissionId string, totalShare sdk.Int) {
    round := k.GetRoundBySubmission(ctx, submissionId)
    // Distribute proportional to accuracy score in this round
    // Validators who scored closer to consensus get more
    // Outlier validators get less (they were already slashed, this is just revenue)
}
```

### 4. Curator Commission

When revenue flows through a dataset access (not direct sample access):
```go
curatorCommission := amount.Mul(500).Quo(10000)  // 5% curator commission
// Deducted from protocol share, not submitter share
```

### 5. Revenue Tracking

Track per-sample, per-submitter, and per-domain revenue for analytics:

```go
type RevenueStats struct {
    TotalRevenue      string  // Lifetime total
    EpochRevenue      string  // This epoch
    SubmitterEarnings string  // Total paid to submitters
    ValidatorEarnings string  // Total paid to validators
}
```

### 6. Tests

- Basic revenue split (55/22/23)
- Self-authored consent → 1.5x submitter share
- Fair use consent → 0.5x submitter share
- Consent penalty redirected to protocol
- Validator distribution proportional to accuracy
- Curator commission on dataset access
- Revenue tracking updated correctly
- Multiple accesses accumulate correctly
- Zero-revenue sample (no accesses) → no distribution
- Revenue stats queries

Target: ≥ 15 tests.
