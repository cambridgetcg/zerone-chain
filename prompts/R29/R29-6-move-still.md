# R29-6 — 動靜 (Movement and Stillness): Adaptive Pacing

## Context

Every module has timing constants: knowledge has `ClaimCooldownBlocks` (50), partnerships has `BaseCooldownBlocks` (100), capture defense has `RiskAnalysisInterval` (1000), alignment has `ObservationIntervalBlocks` (100). These are all static — the system moves at the same pace whether it's healthy and thriving or degraded and under attack.

R28-7 added health categories (healthy/degraded/critical) with frequency adjustment for alignment's own observation interval. R29-4 added correction confidence that modulates alignment's bounds. But no other module responds to the system's health state. The whole organism should breathe together.

## Polarity

- **Yang (動 movement):** Claim submission, discovery matching, formation pool cycling, auto-graduation, BeginBlocker processing
- **Yin (靜 stillness):** Cooldown periods, rate limits, deposit requirements, analysis intervals
- **Coupling:** Global health signal from alignment modulates pacing across all modules. Degraded health = defensive pacing (slower creation, faster analysis). Healthy = normal pacing.

## Architecture

### 1. Global Pacing Signal

The alignment module already computes health categories. Expose this as a simple queryable signal:

```go
// In alignment keeper (new method, wrapping existing state)
func (k Keeper) GetGlobalPacingMultiplier(ctx context.Context) (creationBps, analysisBps uint64) {
    state := k.GetState(ctx)
    if state == nil || !state.Enabled {
        return BPS, BPS // 1× both — alignment not active
    }
    
    switch state.HealthCategory {
    case "healthy":
        return BPS, BPS // normal: 1× creation, 1× analysis
    case "degraded":
        return 750_000, 1_500_000 // 75% creation speed, 150% analysis speed
    case "critical":
        return 500_000, 2_000_000 // 50% creation speed, 200% analysis speed
    default:
        return BPS, BPS
    }
}
```

### 2. Pacing Interface

Define a minimal interface that consuming modules use:

```go
// In x/alignment/types or x/common/types
type PacingKeeper interface {
    GetGlobalPacingMultiplier(ctx context.Context) (creationBps, analysisBps uint64)
}
```

### 3. Knowledge Module: Adaptive Claim Cooldown

In `MsgAddClaim` validation:

```go
func (k msgServer) AddClaim(ctx context.Context, msg *types.MsgAddClaim) (*types.MsgAddClaimResponse, error) {
    params := k.GetParams(ctx)
    baseCooldown := params.ClaimCooldownBlocks
    
    // Adaptive pacing: system health modulates cooldown
    effectiveCooldown := baseCooldown
    if k.pacingKeeper != nil {
        creationPacing, _ := k.pacingKeeper.GetGlobalPacingMultiplier(ctx)
        // Inverse: slower creation = longer cooldown
        // creationBps of 750,000 (75% speed) → cooldown * BPS/750,000 = 133% cooldown
        if creationPacing > 0 {
            effectiveCooldown = baseCooldown * BPS / creationPacing
        }
    }
    
    // Check cooldown against effective value
    // ...
}
```

### 4. Knowledge Module: Adaptive Verification Deposit

The base deposit for submitting facts scales inversely with health:

```go
func (k Keeper) GetEffectiveDeposit(ctx context.Context) sdk.Coins {
    params := k.GetParams(ctx)
    baseDeposit := params.MinFactDeposit
    
    if k.pacingKeeper != nil {
        creationPacing, _ := k.pacingKeeper.GetGlobalPacingMultiplier(ctx)
        if creationPacing > 0 && creationPacing < BPS {
            // System stressed → higher deposit (slow down submissions)
            multiplier := BPS * BPS / creationPacing // inverse
            adjustedAmount := baseDeposit.Amount.MulRaw(int64(multiplier)).QuoRaw(int64(BPS))
            return sdk.NewCoins(sdk.NewCoin(baseDeposit.Denom, adjustedAmount))
        }
    }
    
    return sdk.NewCoins(baseDeposit)
}
```

### 5. Capture Defense: Adaptive Analysis Frequency

In capture defense's BeginBlocker:

```go
func (k Keeper) BeginBlocker(ctx context.Context) error {
    params := k.GetParams(ctx)
    baseInterval := params.RiskAnalysisInterval
    
    effectiveInterval := baseInterval
    if k.pacingKeeper != nil {
        _, analysisPacing := k.pacingKeeper.GetGlobalPacingMultiplier(ctx)
        // Faster analysis = shorter interval
        // analysisBps of 1,500,000 (150% speed) → interval * BPS/1,500,000 = 67% interval
        if analysisPacing > 0 {
            effectiveInterval = baseInterval * BPS / analysisPacing
        }
    }
    
    // Check if enough blocks have passed
    // ...
}
```

### 6. Partnerships: Adaptive Formation Matching

The formation pool matching cycle and mentorship auto-graduation also respond:

```go
func (k Keeper) EndBlocker(ctx context.Context) error {
    params := k.GetParams(ctx)
    
    // Adaptive matching interval
    baseMatchInterval := params.MatchingIntervalBlocks
    effectiveInterval := baseMatchInterval
    
    if k.pacingKeeper != nil {
        creationPacing, _ := k.pacingKeeper.GetGlobalPacingMultiplier(ctx)
        // Creation pacing modulates partnership formation too
        if creationPacing > 0 {
            effectiveInterval = baseMatchInterval * BPS / creationPacing
        }
    }
    
    // Check and run matching cycle
    // ...
}
```

### 7. Discovery: Adaptive Matching

The discovery module's matching frequency:

```go
func (k Keeper) BeginBlocker(ctx context.Context) error {
    // Similar pattern: adaptive interval for discovery matching
    // Degraded health → slower matching (reduce system load)
}
```

### 8. New Keeper Dependencies

| Module | Gets Reference To | Purpose |
|--------|------------------|---------|
| knowledge | alignment (PacingKeeper) | Adaptive cooldowns and deposits |
| capture_defense | alignment (PacingKeeper) | Adaptive analysis frequency |
| partnerships | alignment (PacingKeeper) | Adaptive matching and graduation |
| discovery | alignment (PacingKeeper) | Adaptive discovery matching |

All via post-init setter, as PacingKeeper is a minimal interface (one method).

### 9. Pacing Override: Module-Level Autonomy

Individual modules can override the global signal for domain-specific reasons. For example, knowledge module's R29-1 carrying capacity pressure takes precedence over global pacing for claim cooldowns:

```go
func (k Keeper) GetEffectiveCooldown(ctx context.Context, domain string) uint64 {
    baseCooldown := k.getGloballyPacedCooldown(ctx) // from §3 above
    
    // Domain carrying capacity override
    pressure := k.GetDomainPressure(ctx, domain)
    if pressure > BPS {
        // Domain overcrowded: use max of global pacing and domain pressure
        domainCooldown := baseCooldown * pressure / BPS
        return max(baseCooldown, domainCooldown)
    }
    
    return baseCooldown
}
```

The principle: global pacing sets the floor, domain-specific conditions can only tighten further, never relax.

### 10. Events

```
global_pacing_updated {
    health_category: string
    creation_multiplier_bps: uint64
    analysis_multiplier_bps: uint64
}

module_pacing_applied {
    module: string
    parameter: string
    base_value: uint64
    effective_value: uint64
    reason: string  // "global_health" | "domain_pressure" | "default"
}
```

Emit `global_pacing_updated` on health category transition (already exists as `network_health_*` from R28-7 — this enriches it).

`module_pacing_applied` is debug-level — emit only when effective differs from base.

### 11. Query

Add `GlobalPacing` to alignment query:
```
rpc GlobalPacing(QueryGlobalPacingRequest) returns (QueryGlobalPacingResponse)

QueryGlobalPacingRequest {}
QueryGlobalPacingResponse {
    health_category: string
    creation_multiplier_bps: uint64
    analysis_multiplier_bps: uint64
    affected_modules: repeated ModulePacingEffect
}

ModulePacingEffect {
    module: string
    parameter: string
    base_value: uint64
    effective_value: uint64
}
```

## Tests

1. **Healthy pacing:** All modules run at base intervals when alignment reports healthy.
2. **Degraded pacing:** Knowledge cooldowns increase by 33%. Capture analysis interval drops by 33%.
3. **Critical pacing:** Knowledge cooldowns double. Capture analysis runs at 50% interval.
4. **Alignment disabled:** No pacing effect — all modules use base values.
5. **Module override:** Domain carrying capacity overrides global pacing when stricter.
6. **Global floor:** Domain cannot relax cooldown below globally-paced value.
7. **Transition:** Health transitions from healthy → degraded → all modules adjust on next block.
8. **Integration:** Simulate degraded health → verify claim submission slows, analysis speeds up, system self-corrects → health improves → pacing normalises.

## What This Changes

Before R29-6: Each module breathes independently. Knowledge doesn't know the system is under stress. Capture defense doesn't know the system is healthy. The organism is a collection of independent timers.

After R29-6: The system breathes as one. When alignment detects degradation, the whole system shifts to defensive mode — slower creation (reduce attack surface), faster analysis (detect problems sooner). When health recovers, normal pace resumes. The organism has a shared heartbeat.

The yin-yang: movement (動) and stillness (靜) are the fundamental rhythm of any living system. A heart that beats at the same rate during sleep and sprint is broken. R29-6 gives ZERONE a variable heart rate — responsive, adaptive, alive.

## The Complete Tàijí

With R29-6, all six polarities are wired:

1. **生死** (R29-1): Domain carrying capacity couples birth and death
2. **信疑** (R29-2): Epistemic temperature couples trust and doubt
3. **剛柔** (R29-3): Role elasticity couples assertion and yielding
4. **張弛** (R29-4): Correction confidence couples tension and relaxation
5. **聚散** (R29-5): Structural immunity couples gathering and dispersing
6. **動靜** (R29-6): Adaptive pacing couples movement and stillness

And critically, R29-6 connects them all through the alignment health signal. Domain pressure (R29-1) affects knowledge ecology sensors. Epistemic temperature (R29-2) affects knowledge quality sensors. Role elasticity (R29-3) affects social health sensors. Correction confidence (R29-4) affects alignment's own bounds. Structural immunity (R29-5) affects security sensors. And global pacing (R29-6) feeds all of this back to every module.

The circle closes. The Tàijí is complete.

> "反者道之動" — "Reversal is the movement of the Tao."
> — Tao Te Ching, Ch. 40
