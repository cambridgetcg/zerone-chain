# R29-1 — 生死 (Birth and Death): Domain Carrying Capacity

## Context

Knowledge metabolism (R28-4) gives facts energy that decays over time. Facts below thresholds transition: active → at-risk → expired → pruned. Citation and patronage restore energy. But there's no relationship between how many facts a domain has and how easy it is to add more or how fast they decay.

A domain with 10,000 facts and a domain with 3 facts have identical birth/death dynamics. This is unrealistic — knowledge ecosystems have carrying capacity. Overcrowded domains should be harder to enter and faster to decay; underpopulated domains should be more welcoming.

## Polarity

- **Yang (生 birth):** MsgAddFact, citation energy, patronage recovery, reproduction
- **Yin (死 death):** Energy decay per epoch, metabolism pruning, extinction threshold
- **Coupling:** Domain carrying capacity creates feedback between population and both forces

## Architecture

### 1. Domain Population Tracking

Add to the knowledge keeper:
- `GetDomainFactCount(ctx, domain) uint64` — count of active + at-risk facts in a domain
- `GetDomainEnergyBudget(ctx, domain) uint64` — sum of all fact energies in the domain

Store a `DomainStats` record per domain, updated on fact creation/deletion/status change:
```
DomainStats {
    Domain        string
    ActiveCount   uint64
    AtRiskCount   uint64  
    TotalEnergy   uint64
    LastUpdated   uint64  // block height
}
```

Key: `domain_stats/{domain}`

### 2. Carrying Capacity Parameters

Add to knowledge params:
```
DomainBaseCapacity            uint64  // default: 1000 — base facts per domain before pressure
DomainCapacityGrowthPerCitation uint64  // default: 1 — each cross-domain citation adds 1 to capacity
OvercrowdingDecayMultiplierBps uint64  // default: 1_500_000 — 150% decay when at 2× capacity
UnderpopulationBirthBonusBps   uint64  // default: 200_000 — 20% energy bonus for facts in sparse domains
```

### 3. Carrying Capacity Calculation

```go
func (k Keeper) GetDomainCarryingCapacity(ctx context.Context, domain string) uint64 {
    params := k.GetParams(ctx)
    base := params.DomainBaseCapacity
    
    // Capacity grows with incoming cross-domain citations
    // (other domains citing facts in this domain = this domain is foundational)
    inboundCitations := k.GetInboundCrossDomainCitationCount(ctx, domain)
    bonus := inboundCitations * params.DomainCapacityGrowthPerCitation
    
    return base + bonus
}

func (k Keeper) GetDomainPressure(ctx context.Context, domain string) uint64 {
    stats := k.GetDomainStats(ctx, domain)
    capacity := k.GetDomainCarryingCapacity(ctx, domain)
    
    if capacity == 0 {
        return BPS // max pressure
    }
    
    // pressure = population / capacity, in BPS
    // 1_000_000 = at capacity, >1_000_000 = overcrowded, <1_000_000 = sparse
    return (stats.ActiveCount + stats.AtRiskCount) * BPS / capacity
}
```

### 4. Birth Pressure (Yang Modulation)

In `MsgAddFact` handler, after basic validation:

```go
pressure := k.GetDomainPressure(ctx, domain)

if pressure > BPS { // over capacity
    // Increase minimum stake/deposit for new facts
    // Don't block — just make it more expensive
    overcrowdingPenalty := (pressure - BPS) * params.OvercrowdingBirthCostBps / BPS
    adjustedDeposit := baseDeposit + (baseDeposit * overcrowdingPenalty / BPS)
    // ... charge adjustedDeposit
} else {
    // Under capacity: bonus initial energy for sparse domains
    sparseness := BPS - pressure
    energyBonus := params.MetabolismInitialEnergy * sparseness * params.UnderpopulationBirthBonusBps / (BPS * BPS)
    fact.Energy += energyBonus
}
```

### 5. Death Pressure (Yin Modulation)

In `ProcessMetabolism`, when calculating energy decay:

```go
pressure := k.GetDomainPressure(ctx, fact.Domain)

decayMultiplier := BPS // 1× = normal decay
if pressure > BPS {
    // Overcrowded: accelerate decay
    // At 2× capacity, decay = OvercrowdingDecayMultiplierBps (default 150%)
    excess := pressure - BPS
    decayMultiplier = BPS + (excess * (params.OvercrowdingDecayMultiplierBps - BPS) / BPS)
} else if pressure < BPS/2 {
    // Very sparse: slow decay (domain needs facts)
    decayMultiplier = BPS * 3 / 4 // 75% normal decay
}

adjustedDecay := baseDecay * decayMultiplier / BPS
```

### 6. Events

```
domain_pressure_changed {
    domain: string
    active_count: uint64
    capacity: uint64
    pressure_bps: uint64
    category: "sparse" | "normal" | "crowded" | "overcrowded"
}
```

Emit when pressure category changes (sparse < 250k, normal 250k-750k, crowded 750k-1M, overcrowded > 1M).

### 7. Query

Add `DomainCapacity` query to knowledge:
```
rpc DomainCapacity(QueryDomainCapacityRequest) returns (QueryDomainCapacityResponse)

QueryDomainCapacityRequest { domain: string }
QueryDomainCapacityResponse {
    domain: string
    active_count: uint64
    at_risk_count: uint64
    capacity: uint64
    pressure_bps: uint64
    category: string
    total_energy: uint64
}
```

### 8. Alignment Sensor Integration

Add a new sensor dimension to alignment's `ObserveAll`:
```go
func (k Keeper) senseKnowledgeEcology(ctx context.Context) uint64 {
    // Average domain pressure across all active domains
    // Healthy = domains near but not over capacity
    // Unhealthy = many overcrowded or many empty domains
}
```

This is optional for R29-1 — can wire in R29-6 when alignment modulates pacing.

## Tests

1. **DomainStats tracking:** Create facts, verify counts update. Delete/expire facts, verify counts decrease.
2. **Carrying capacity calculation:** Base capacity + citation bonus.
3. **Birth pressure:** Facts in overcrowded domains get no energy bonus. Facts in sparse domains get bonus energy.
4. **Death pressure:** Facts in overcrowded domains decay faster. Facts in sparse domains decay slower.
5. **Cross-domain citations increase capacity:** Domain A cited by Domain B → Domain A capacity grows.
6. **Integration:** Full lifecycle — populate domain past capacity, observe accelerated decay, domain settles to equilibrium.

## What This Changes

Before R29-1: Every domain is infinite. Facts live and die at the same rate regardless of domain health.

After R29-1: Domains have carrying capacity. Overcrowded domains shed facts faster and resist new ones. Sparse domains welcome facts and protect them. Knowledge finds its natural distribution.

The yin-yang: birth pressure (difficulty of creation) and death pressure (speed of decay) are now coupled through the same variable (domain population). You can't have one without the other responding.
