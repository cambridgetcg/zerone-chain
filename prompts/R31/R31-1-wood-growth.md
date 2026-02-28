# R31-1 — 木 Wood (Growth): Knowledge as the Source

## Phase Identity

Wood is the phase of growth — upward, expanding, branching. In ZERONE, the knowledge module IS wood: facts are born, reproduce (via citation children), branch into domains, and grow toward the light (higher confidence, fitness).

## Relationships

### Generates → Fire (Wood fuels activity)

**Status:** ✅ Already exists. Fact submission triggers verification rounds. This is the primary generating relationship and needs no new work.

**Verify only:** Write a test confirming that `MsgAddClaim` on a verified fact creates a `VerificationRound` automatically. This should already pass.

### ← Controlled by Metal (Defense prunes growth)

**Status:** ⚠️ Partial. R29-1 added domain carrying capacity, and R29-5 connected capture defense to partnerships. But capture defense flags don't affect carrying capacity.

**New connection:** When capture_defense flags a domain, reduce that domain's effective carrying capacity. Captured domains shouldn't be growing — they should be shedding facts faster until the capture is resolved.

```go
// In knowledge keeper's GetDomainCarryingCapacity:
func (k Keeper) GetDomainCarryingCapacity(ctx context.Context, domain string) uint64 {
    params := k.GetParams(ctx)
    base := params.DomainBaseCapacity
    
    // Existing: citation-based growth
    inboundCitations := k.GetInboundCrossDomainCitationCount(ctx, domain)
    bonus := inboundCitations * params.DomainCapacityGrowthPerCitation
    
    // NEW: capture flag penalty (Metal controls Wood)
    if k.captureDefenseKeeper != nil {
        flagged, penaltyBps := k.captureDefenseKeeper.GetDomainCapturePenalty(ctx, domain)
        if flagged {
            reduction := (base + bonus) * penaltyBps / BPS
            if reduction >= base + bonus {
                return 1 // minimum capacity — can't go to zero
            }
            return base + bonus - reduction
        }
    }
    
    return base + bonus
}
```

**New interface method on CaptureDefenseKeeper:**
```go
GetDomainCapturePenalty(ctx context.Context, domain string) (flagged bool, penaltyBps uint64)
```

Implementation in capture_defense:
```go
func (k Keeper) GetDomainCapturePenalty(ctx context.Context, domain string) (bool, uint64) {
    metrics, found := k.GetCaptureMetrics(ctx, domain)
    if !found || !metrics.Flagged {
        return false, 0
    }
    // Penalty scales with HHI — higher concentration = more capacity reduction
    // At HHI threshold (250,000): 25% capacity reduction
    // At HHI 500,000: 50% reduction
    // At HHI 1,000,000 (monopoly): 100% reduction (capacity = 1)
    return true, metrics.Hhi
}
```

**New param in capture_defense:**
```
CaptureCapacityPenaltyBps uint64 // default: 500_000 — at HHI threshold, 50% of HHI as capacity reduction
```

### Controls → Earth (Growth disrupts stability)

**Status:** ❌ Missing. Knowledge growth has no effect on governance.

**New connection:** When the rate of new fact creation exceeds a threshold, emit a signal that governance can consume to auto-adjust verification parameters. This isn't governance *proposals* — it's a governance *advisory* that makes the alignment module aware of knowledge growth rate.

Add a knowledge growth rate metric to the alignment observation:

```go
// In alignment's senseKnowledgeQuality (existing sensor):
func (k Keeper) senseKnowledgeQuality(ctx context.Context) uint64 {
    // ... existing quality metrics ...
    
    // NEW: Factor in growth rate sustainability
    // If facts are being created much faster than they're being verified,
    // knowledge quality is degraded (Wood overwhelming Earth)
    if k.knowledgeKeeper != nil {
        pendingRatio := k.knowledgeKeeper.GetPendingVerificationRatio(ctx)
        // pendingRatio > 1.0 (in BPS: > 1,000,000) means more pending than active
        if pendingRatio > 1_500_000 { // 150% — verification backlog
            qualityScore = qualityScore * 800_000 / BPS // 20% penalty
        }
    }
    
    return qualityScore
}
```

**New interface method on KnowledgeKeeper (for alignment):**
```go
GetPendingVerificationRatio(ctx context.Context) uint64 // pending claims / active facts, in BPS
```

Implementation:
```go
func (k Keeper) GetPendingVerificationRatio(ctx context.Context) uint64 {
    pending := k.GetPendingClaimCount(ctx)
    active := k.GetActiveFactCount(ctx)
    if active == 0 {
        if pending > 0 { return BPS * 2 } // infinite ratio capped at 200%
        return BPS // 1:1 when both zero
    }
    return pending * BPS / active
}
```

This means rapid knowledge growth → alignment detects quality degradation → generates corrections → autopoiesis adjusts parameters (e.g., increases verification requirements) → growth slows naturally. Wood disrupts Earth, Earth responds through Metal.

## Events

```
zerone.knowledge.capacity_penalty_applied {
    domain: string
    base_capacity: uint64
    effective_capacity: uint64
    capture_penalty_bps: uint64
    reason: "capture_flagged"
}

zerone.alignment.growth_pressure_detected {
    pending_ratio_bps: uint64
    quality_penalty_applied: bool
}
```

## Tests

1. **Metal controls Wood:** Flag domain for capture → verify carrying capacity decreases proportionally to HHI.
2. **Unflag restores capacity:** Clear capture flag → carrying capacity returns to base + citation bonus.
3. **Growth disrupts Earth:** Create facts faster than verification → pending ratio rises → alignment quality sensor detects degradation.
4. **End-to-end:** Rapid fact creation in captured domain → capacity drops + quality degrades → corrections generated → system self-adjusts.
5. **Existing generating relationship:** Verify MsgAddClaim → VerificationRound still works (regression check).

## What This Changes

Before R31-1: Knowledge growth is bounded only by carrying capacity (R29-1). Capture defense flags don't affect knowledge layer. Knowledge growth rate doesn't affect governance/alignment.

After R31-1: Wood is controlled by Metal (captured domains shrink) and controls Earth (growth pressure triggers alignment corrections). The generating relationship (Wood → Fire) was already in place. Wood is now fully wired into the Wu Xing cycle.
