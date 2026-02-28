# R29-5 — 聚散 (Gathering and Dispersing): Structural Immunity

## Context

R28-6 activated mentorship and formation pool matching — agents and humans can form partnerships, mentor each other, and the protocol matches them. R28-8 activated capture defense — domains with concentrated validator power get flagged, auto-challenged, and reputation-penalised.

But gathering and dispersing don't know about each other. A domain flagged for capture concentration doesn't attract new participants. A domain with rich partnership diversity doesn't get credit for natural capture resistance. The immune system (capture defense) and the growth system (partnerships) operate in isolation.

## Polarity

- **Yang (聚 gathering):** Partnership formation, mentorship matching, formation pool, reputation building
- **Yin (散 dispersing):** Capture flagging, auto-challenges, reputation decay, qualification reduction
- **Coupling:** Structural immunity — capture risk feeds into partnership incentives; partnership density feeds into capture risk assessment

## Architecture

### 1. Domain Partnership Density

Add to partnerships keeper:

```go
func (k Keeper) GetDomainPartnershipDensity(ctx context.Context, domain string) uint64 {
    // Count unique participants in active partnerships for this domain
    uniqueParticipants := make(map[string]bool)
    
    k.IteratePartnershipsByDomain(ctx, domain, func(p *types.Partnership) bool {
        if p.Status == "active" {
            uniqueParticipants[p.HumanAddr] = true
            uniqueParticipants[p.AgentAddr] = true
        }
        return false
    })
    
    k.IterateMentorshipsByDomain(ctx, domain, func(m *types.Mentorship) bool {
        if m.Status == "active" {
            uniqueParticipants[m.MentorAddr] = true
            uniqueParticipants[m.MenteeAddr] = true
        }
        return false
    })
    
    // Density = unique participants / (some reference baseline)
    // Higher is better — more distributed participation
    return uint64(len(uniqueParticipants))
}
```

### 2. Capture Risk Reduction from Partnerships (Yin weakened by Yang)

In capture defense's `RunAutoAnalysis`, the HHI calculation should account for partnership density:

```go
func (k Keeper) CalculateAdjustedHHI(ctx context.Context, domain string, rawHHI uint64) uint64 {
    if k.partnershipsKeeper == nil {
        return rawHHI
    }
    
    params := k.GetParams(ctx)
    density := k.partnershipsKeeper.GetDomainPartnershipDensity(ctx, domain)
    
    // Each unique partnership participant reduces effective HHI
    // (distributed social structure = harder to capture)
    // Diminishing returns: first few participants matter most
    if density > 0 {
        reductionBps := min(
            density * params.PartnershipHHIReductionPerParticipantBps,
            params.MaxPartnershipHHIReductionBps,
        )
        adjusted := rawHHI * (BPS - reductionBps) / BPS
        return adjusted
    }
    
    return rawHHI
}
```

### 3. Partnership Formation Bonus from Capture Risk (Yang strengthened by Yin)

When capture defense flags a domain, inject a formation bonus signal that the partnership formation pool uses:

```go
// In capture_defense, when flagging a domain:
func (k Keeper) OnDomainFlagged(ctx context.Context, domain string) {
    // ... existing flagging logic ...
    
    // Signal to partnerships that this domain needs new entrants
    if k.partnershipsKeeper != nil {
        k.partnershipsKeeper.SetDomainFormationBonus(ctx, domain, &types.FormationBonus{
            Domain:       domain,
            BonusBps:     k.GetParams(ctx).CapturedDomainFormationBonusBps,
            Reason:       "capture_flagged",
            ExpiryHeight: uint64(sdkCtx.BlockHeight()) + k.GetParams(ctx).FormationBonusDurationBlocks,
        })
    }
}
```

### 4. Formation Bonus Effects

In the partnership formation pool matching (R28-6 EndBlocker):

```go
func (k Keeper) ScoreFormationMatch(ctx context.Context, match *types.FormationMatch) uint64 {
    baseScore := k.calculateBaseMatchScore(ctx, match)
    
    // Check for domain formation bonuses
    bonus := k.GetDomainFormationBonus(ctx, match.Domain)
    if bonus != nil && bonus.ExpiryHeight > uint64(sdkCtx.BlockHeight()) {
        // Boost match priority for flagged domains
        baseScore = baseScore * (BPS + bonus.BonusBps) / BPS
    }
    
    return baseScore
}
```

Additionally, mentorship proposals in flagged domains get priority matching:

```go
func (k Keeper) MatchFormationPool(ctx context.Context) {
    // Sort pending matches by score (descending)
    // Flagged-domain matches get bonus score → matched first
    // ...
}
```

### 5. Partnership-Aware Reputation

In capture defense's reputation calculation, account for partnership quality:

```go
func (k Keeper) CalculateStructuralReputationBonus(ctx context.Context, validator, domain string) uint64 {
    if k.partnershipsKeeper == nil {
        return 0
    }
    
    // Validator has active partnerships in this domain → reputation bonus
    // (invested in the domain's social structure, not just extracting)
    partnerships := k.partnershipsKeeper.GetPartnershipsByParticipant(ctx, validator, domain)
    activeCount := uint64(0)
    for _, p := range partnerships {
        if p.Status == "active" {
            activeCount++
        }
    }
    
    params := k.GetParams(ctx)
    bonus := min(activeCount * params.PartnershipReputationBonusBps, params.MaxPartnershipReputationBonusBps)
    return bonus
}
```

### 6. Unflagging Acceleration

When a domain accumulates enough partnership density after being flagged, the flag should clear faster:

```go
func (k Keeper) ShouldAccelerateClearFlag(ctx context.Context, domain string) bool {
    metrics, found := k.GetCaptureMetrics(ctx, domain)
    if !found || !metrics.Flagged { return false }
    
    density := k.partnershipsKeeper.GetDomainPartnershipDensity(ctx, domain)
    return density >= k.GetParams(ctx).MinDensityForAcceleratedClear
}
```

### 7. Parameters

**Add to capture_defense params:**
```
PartnershipHHIReductionPerParticipantBps  uint64  // default: 10_000 — 1% HHI reduction per participant
MaxPartnershipHHIReductionBps             uint64  // default: 200_000 — max 20% HHI reduction
PartnershipReputationBonusBps             uint64  // default: 20_000 — 2% rep bonus per active partnership
MaxPartnershipReputationBonusBps          uint64  // default: 100_000 — max 10% rep bonus
MinDensityForAcceleratedClear             uint64  // default: 10 — 10 unique participants
```

**Add to partnerships params:**
```
CapturedDomainFormationBonusBps   uint64  // default: 300_000 — 30% match priority boost for flagged domains
FormationBonusDurationBlocks      uint64  // default: 50_000 — ~35 hours
```

### 8. New Keeper Dependencies

| Module | Gets Reference To | Purpose |
|--------|------------------|---------|
| capture_defense | partnerships | Read density, set formation bonus |
| partnerships | capture_defense (optional) | Read flagged status for UI/query |

Use post-init setters as established in R28.

### 9. Events

```
structural_immunity_updated {
    domain: string
    partnership_density: uint64
    raw_hhi: uint64
    adjusted_hhi: uint64
    formation_bonus_active: bool
}

domain_formation_bonus_set {
    domain: string
    bonus_bps: uint64
    reason: string
    expiry_height: uint64
}
```

### 10. Query

Add `StructuralImmunity` to capture_defense query:
```
rpc StructuralImmunity(QueryStructuralImmunityRequest) returns (QueryStructuralImmunityResponse)

QueryStructuralImmunityRequest { domain: string }
QueryStructuralImmunityResponse {
    domain: string
    partnership_density: uint64
    raw_hhi: uint64
    adjusted_hhi: uint64
    hhi_reduction_bps: uint64
    formation_bonus_active: bool
    formation_bonus_bps: uint64
    flagged: bool
}
```

## Tests

1. **Partnership density calculation:** Active partnerships counted, unique participants deduplicated.
2. **HHI reduction:** 5 unique participants → 5% HHI reduction. Max capped at 20%.
3. **Formation bonus on flag:** Domain flagged → FormationBonus created with correct expiry.
4. **Match priority boost:** Flagged domain matches scored higher than unflagged domain matches.
5. **Reputation bonus:** Validator with 3 partnerships → 6% reputation bonus.
6. **Accelerated clearing:** Domain reaches 10 participants → flag clears faster.
7. **Bonus expiry:** Formation bonus expires after duration blocks.
8. **Integration:** Domain gets flagged → formation bonus → new partnerships form → density increases → HHI drops → flag clears.

## What This Changes

Before R29-5: The immune system (capture defense) and the growth system (partnerships) are independent. Flagging a domain does nothing to attract new participants. Having many partnerships does nothing to reduce capture risk.

After R29-5: Threat catalyses renewal. A flagged domain becomes more attractive for new partnerships (the protocol actively incentivises diversification). Existing partnerships provide structural immunity (distributed social structure is natural capture defense).

The yin-yang: gathering (聚) and dispersing (散) aren't just parallel forces — dispersing (capture flagging) triggers gathering (partnership formation), and gathering (partnership density) prevents dispersing (reduces capture risk). They complete each other.
