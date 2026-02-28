# R29-2 — 信疑 (Trust and Doubt): Epistemic Temperature

## Context

R28-1 added vindication — dissenters who were later proven right get retroactive rewards. R28-2 added conformity scoring — domains where all verifiers always agree get flagged. But these two systems don't talk to each other.

A domain with 100% conformity still grows confidence at the same rate as one with healthy dissent. A domain that just had a vindication event (proving its consensus was wrong) still accepts new claims at the same confidence threshold. Trust-building and doubt-raising operate in parallel but never intersect.

## Polarity

- **Yang (信 trust):** Confidence growth per epoch, verification quorum → acceptance, fitness scoring rewards
- **Yin (疑 doubt):** Dissent rewards, vindication bonuses, conformity alerts, confidence caps
- **Coupling:** Epistemic temperature — a domain's recent conformity/vindication history modulates how fast confidence can grow and how high it can reach

## Architecture

### 1. Domain Epistemic Temperature

A new derived metric stored per domain:

```
DomainEpistemicState {
    Domain                string
    Temperature           uint64  // BPS scale: 500_000 = neutral
    ConformityStreak      uint64  // consecutive epochs with >threshold conformity
    VindicationCount      uint64  // vindications in last N blocks
    LastTemperatureUpdate uint64  // block height
}
```

Key: `epistemic_state/{domain}`

**Temperature semantics:**
- **Cold (< 300,000):** High conformity, no recent vindication. System is stagnant — confidence grows slowly, caps are lower.
- **Neutral (300,000 - 700,000):** Healthy mix of agreement and dissent. Normal dynamics.
- **Hot (> 700,000):** Recent vindication events, active dissent. System is self-correcting — confidence can grow faster because doubt has been validated.

### 2. Temperature Parameters

Add to knowledge params:
```
EpistemicTemperatureDecayBps       uint64  // default: 995_000 — 99.5% per epoch, slow drift to neutral
EpistemicConformityCoolingBps      uint64  // default: 50_000  — each high-conformity epoch reduces temp by 5%
EpistemicVindicationHeatingBps     uint64  // default: 100_000 — each vindication increases temp by 10%
EpistemicColdConfidenceCapBps      uint64  // default: 600_000 — 60% max confidence in cold domains
EpistemicHotConfidenceGrowthBps    uint64  // default: 1_500_000 — 150% confidence growth rate in hot domains
EpistemicTemperatureWindowBlocks   uint64  // default: 10_000  — lookback window for vindication counting
```

### 3. Temperature Update Logic

Run in knowledge BeginBlocker at fitness epochs:

```go
func (k Keeper) UpdateEpistemicTemperature(ctx context.Context, domain string) {
    state := k.GetDomainEpistemicState(ctx, domain)
    params := k.GetParams(ctx)
    height := uint64(sdk.UnwrapSDKContext(ctx).BlockHeight())
    
    // 1. Decay toward neutral (500,000)
    neutral := uint64(500_000)
    if state.Temperature > neutral {
        diff := state.Temperature - neutral
        state.Temperature = neutral + (diff * params.EpistemicTemperatureDecayBps / BPS)
    } else if state.Temperature < neutral {
        diff := neutral - state.Temperature
        state.Temperature = neutral - (diff * params.EpistemicTemperatureDecayBps / BPS)
    }
    
    // 2. Conformity cooling
    diversity := k.GetDomainDiversityScore(ctx, domain)
    if diversity != nil && diversity.DiversityScore < params.ConformityAlertThresholdBps {
        state.ConformityStreak++
        cooling := params.EpistemicConformityCoolingBps * min(state.ConformityStreak, 10) / 10
        if state.Temperature > cooling {
            state.Temperature -= cooling
        } else {
            state.Temperature = 0
        }
    } else {
        state.ConformityStreak = 0
    }
    
    // 3. Vindication heating
    recentVindications := k.CountVindicationsInWindow(ctx, domain, height, params.EpistemicTemperatureWindowBlocks)
    if recentVindications > state.VindicationCount {
        newVindications := recentVindications - state.VindicationCount
        heating := params.EpistemicVindicationHeatingBps * newVindications
        state.Temperature = min(state.Temperature + heating, BPS)
    }
    state.VindicationCount = recentVindications
    
    state.LastTemperatureUpdate = height
    k.SetDomainEpistemicState(ctx, &state)
    
    // Emit event on category change
    k.emitTemperatureEvent(ctx, domain, state.Temperature)
}
```

### 4. Confidence Growth Modulation (Yang responds to Yin)

In `AdvanceConfidence` (called at ConfidenceGrowthEpoch intervals):

```go
epistemicState := k.GetDomainEpistemicState(ctx, fact.Domain)
growthRate := params.ConfidenceGrowthPerEpochBps

// Hot domains: confidence grows faster (vindication proved self-correction works)
if epistemicState.Temperature > 700_000 {
    growthRate = growthRate * params.EpistemicHotConfidenceGrowthBps / BPS
}

// Cold domains: confidence grows slower (conformity hasn't been tested)
if epistemicState.Temperature < 300_000 {
    growthRate = growthRate * 500_000 / BPS // 50% growth rate
}
```

### 5. Confidence Cap Modulation (Yin constrains Yang)

In all paths that increase confidence (verification, survival, growth):

```go
epistemicState := k.GetDomainEpistemicState(ctx, fact.Domain)

effectiveCap := params.MaxSurvivalConfidence // default 770,000

// Cold domains: lower cap — you can't be highly confident in untested consensus
if epistemicState.Temperature < 300_000 {
    effectiveCap = min(effectiveCap, params.EpistemicColdConfidenceCapBps)
}

// Hot domains: standard cap (vindication earned it)
// Very hot: allow up to SurvivedChallengeConfidenceCap even without surviving a challenge
if epistemicState.Temperature > 800_000 {
    effectiveCap = params.SurvivedChallengeConfidenceCap // 880,000
}

if fact.Confidence > effectiveCap {
    fact.Confidence = effectiveCap
}
```

### 6. Vindication Counter

Add to knowledge keeper:
```go
func (k Keeper) CountVindicationsInWindow(ctx context.Context, domain string, currentHeight, windowBlocks uint64) uint64 {
    startHeight := uint64(0)
    if currentHeight > windowBlocks {
        startHeight = currentHeight - windowBlocks
    }
    
    count := uint64(0)
    k.IterateVindicationRecords(ctx, domain, func(v *types.VindicationRecord) bool {
        if v.VindicatedAtHeight >= startHeight {
            count++
        }
        return false
    })
    return count
}
```

### 7. Events

```
epistemic_temperature_changed {
    domain: string
    temperature_bps: uint64
    category: "cold" | "cool" | "neutral" | "warm" | "hot"
    conformity_streak: uint64
    recent_vindications: uint64
}
```

### 8. Query

Add `EpistemicTemperature` query:
```
rpc EpistemicTemperature(QueryEpistemicTemperatureRequest) returns (QueryEpistemicTemperatureResponse)

QueryEpistemicTemperatureRequest { domain: string }
QueryEpistemicTemperatureResponse {
    domain: string
    temperature_bps: uint64
    category: string
    conformity_streak: uint64
    recent_vindications: uint64
    effective_confidence_cap: uint64
    effective_growth_rate: uint64
}
```

## Tests

1. **Temperature initialisation:** New domain starts at neutral (500,000).
2. **Conformity cooling:** Domain with 100% agreement for 5 epochs → temperature drops below 300,000.
3. **Vindication heating:** Execute vindication → temperature rises above 700,000.
4. **Confidence growth modulation:** Cold domain facts grow confidence at 50% rate. Hot domain facts at 150%.
5. **Confidence cap modulation:** Cold domain facts capped at 600,000 even after verification.
6. **Decay to neutral:** After no events, temperature drifts back to 500,000.
7. **Integration:** Full cycle — conformity streak → cooling → vindication → heating → facts resume growing.

## What This Changes

Before R29-2: Trust and doubt are independent systems. Conformity has no consequence on confidence. Vindication has no consequence beyond the individual reward.

After R29-2: Domains have epistemic temperature. Unchallenged consensus cools the domain — confidence grows slower, peaks lower. Successful dissent heats it — the system earned the right to be more confident because it proved it can correct itself.

The yin-yang: doubt (疑) doesn't destroy trust (信) — it *earns* it. A domain that has never been tested can't be trusted. A domain that has been tested and self-corrected can.
