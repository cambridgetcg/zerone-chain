# R31-2 — 火 Fire (Activity): Verification as Transformation

## Phase Identity

Fire is the phase of activity — transforming, illuminating, consuming. Verification rounds are fire: they transform raw claims into verified facts (or ash). Dissent is the flame that tests what's real. Vindication is the brightest fire — the moment the system proves it can burn away its own errors.

## Relationships

### Generates → Earth (Fire produces consensus)

**Status:** ⚠️ Partial. Verification rounds produce verified facts and confidence scores, but the *pattern* of verification results doesn't feed into governance awareness. Governance doesn't know whether verification is functioning well or poorly — it only knows params.

**New connection:** Verification health metrics feed the alignment module's governance participation sensor, and verification patterns can trigger governance advisories.

Add a verification health signal:

```go
// New interface method on KnowledgeKeeper (for alignment):
GetVerificationHealth(ctx context.Context) (throughputBps, disputeRateBps, avgRoundDurationBlocks uint64)
```

Implementation:
```go
func (k Keeper) GetVerificationHealth(ctx context.Context) (uint64, uint64, uint64) {
    params := k.GetParams(ctx)
    height := uint64(sdk.UnwrapSDKContext(ctx).BlockHeight())
    windowBlocks := params.ObservationWindowBlocks // or use a reasonable default like 10000
    
    completed := k.CountCompletedRoundsInWindow(ctx, height, windowBlocks)
    disputed := k.CountDisputedRoundsInWindow(ctx, height, windowBlocks)
    avgDuration := k.GetAvgRoundDurationInWindow(ctx, height, windowBlocks)
    
    throughput := uint64(0)
    if windowBlocks > 0 {
        throughput = completed * BPS / (windowBlocks / params.CommitPhaseBlocks) // relative to theoretical max
    }
    
    disputeRate := uint64(0)
    if completed > 0 {
        disputeRate = disputed * BPS / completed
    }
    
    return throughput, disputeRate, avgDuration
}
```

Wire into alignment's existing sensor framework:
```go
func (k Keeper) senseGovernanceParticipation(ctx context.Context) uint64 {
    // ... existing governance metrics ...
    
    // NEW: Verification health contributes to governance participation score
    // (healthy verification = active participation in truth-seeking governance)
    if k.knowledgeKeeper != nil {
        throughput, disputeRate, _ := k.knowledgeKeeper.GetVerificationHealth(ctx)
        
        // High throughput + moderate dispute rate = healthy fire
        // Low throughput = fire going out
        // Extreme dispute rate = fire out of control
        verificationHealth := throughput
        if disputeRate > 300_000 { // >30% disputes — too contentious
            verificationHealth = verificationHealth * 700_000 / BPS
        }
        
        // Blend with existing governance score (30% weight)
        score = score * 700_000 / BPS + verificationHealth * 300_000 / BPS
    }
    
    return score
}
```

### ← Controlled by Water (Flow quenches excess activity)

**Status:** ❌ Missing. Partnership density has no effect on verification pacing.

**New connection:** In domains with high partnership density (many active mentor-mentee and human-agent pairs), verification rounds can have slightly relaxed requirements. The social structure provides a natural quality filter — people who are partnered and mentored tend to submit better claims, so the protocol can afford to be less aggressive.

Conversely, in domains with NO partnerships (no social structure), verification requirements should be tighter — there's no social accountability.

```go
// In knowledge keeper, when determining verification requirements:
func (k Keeper) GetEffectiveMinVerifiers(ctx context.Context, domain string) uint32 {
    params := k.GetParams(ctx)
    base := params.MinVerifiers
    
    if k.partnershipKeeper == nil {
        return base
    }
    
    density := k.partnershipKeeper.GetDomainPartnershipDensity(ctx, domain)
    
    if density == 0 {
        // No social structure — tighten verification (Water is absent → Fire burns unchecked)
        return base + 1 // one extra verifier required
    }
    
    if density >= params.SocialSaturationThreshold {
        // High social structure — can relax slightly (Water quenches excess)
        // But never below absolute minimum of 2
        if base > 2 {
            return base - 1
        }
    }
    
    return base
}
```

**New param in knowledge:**
```
SocialSaturationThreshold uint64 // default: 10 — partnership density above which verification relaxes
```

### Controls → Metal (Activity undermines rigidity)

**Status:** ⚠️ Partial. R29-3 role elasticity adjusts bonuses based on vindication/challenge outcomes, but verification activity doesn't directly affect capture defense's structural assumptions.

**New connection:** High verification activity in a domain should increase capture defense's reputation recovery rate. Active verification = the domain is being scrutinised = less risk of undetected capture.

```go
// New interface method on KnowledgeKeeper (for capture_defense):
GetDomainVerificationActivity(ctx context.Context, domain string) uint64 // activity score in BPS
```

Implementation:
```go
func (k Keeper) GetDomainVerificationActivity(ctx context.Context, domain string) uint64 {
    // Count verification rounds completed in this domain in the last N blocks
    height := uint64(sdk.UnwrapSDKContext(ctx).BlockHeight())
    params := k.GetParams(ctx)
    rounds := k.CountCompletedRoundsForDomainInWindow(ctx, domain, height, 10000)
    
    // Normalise: 10 rounds per window = BPS (100% activity)
    activity := rounds * BPS / 10
    if activity > BPS {
        activity = BPS
    }
    return activity
}
```

In capture_defense's reputation calculation:
```go
func (k Keeper) calculateReputationRecoveryRate(ctx context.Context, domain string) uint64 {
    params := k.GetParams(ctx)
    baseRate := params.BaseReputationRecoveryBps
    
    // Fire controls Metal: active verification accelerates reputation recovery
    if k.knowledgeKeeper != nil {
        activity := k.knowledgeKeeper.GetDomainVerificationActivity(ctx, domain)
        // High activity → faster recovery (up to 150% of base rate)
        activityBonus := activity * 500_000 / BPS // max 50% bonus at full activity
        return baseRate + (baseRate * activityBonus / BPS)
    }
    
    return baseRate
}
```

**New param in capture_defense:**
```
BaseReputationRecoveryBps       uint64 // default: 50_000 — 5% recovery per decay epoch
ActivityRecoveryBonusMaxBps     uint64 // default: 500_000 — max 50% acceleration from verification activity
```

## Events

```
zerone.alignment.verification_health_observed {
    throughput_bps: uint64
    dispute_rate_bps: uint64
    avg_round_duration: uint64
}

zerone.knowledge.social_verification_adjustment {
    domain: string
    base_min_verifiers: uint32
    effective_min_verifiers: uint32
    partnership_density: uint64
    reason: "social_saturation" | "no_social_structure" | "default"
}

zerone.capture_defense.activity_recovery_bonus {
    domain: string
    verification_activity_bps: uint64
    recovery_rate_bps: uint64
    bonus_bps: uint64
}
```

## Tests

1. **Fire → Earth:** High verification throughput → alignment governance participation sensor improves.
2. **Fire → Earth:** Extreme dispute rate (>30%) → governance participation sensor degrades.
3. **Water → Fire:** Domain with 10+ partnerships → min verifiers reduced by 1.
4. **Water → Fire:** Domain with 0 partnerships → min verifiers increased by 1.
5. **Fire → Metal:** Active verification domain → capture defense reputation recovers faster.
6. **Fire → Metal:** Inactive domain → reputation recovers at base rate only.
7. **Combined:** Domain with partnerships + active verification + capture flag → recovery accelerates, verification requirements relax, alignment sees healthy governance.

## What This Changes

Before R31-2: Verification operates in isolation. Its throughput doesn't inform governance. Social structure doesn't affect verification requirements. Verification activity doesn't help domains recover from capture flags.

After R31-2: Fire generates Earth (verification health → governance awareness), Water controls Fire (social density → verification requirements), and Fire controls Metal (activity → reputation recovery). The verification layer is now fully integrated into the circulation system.
