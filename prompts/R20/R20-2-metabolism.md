# R20-2 — Metabolism: Maintenance Cost + Energy Budget

## Context

In biology, every organism consumes energy just to exist. If it can't feed, it dies. Facts should work the same way: existing in the Tree of Knowledge has a cost. That cost is paid by the value the fact generates (queries, citations, patronage). If a fact doesn't generate enough value to cover its maintenance, it slowly starves.

This is the mechanism that prevents the knowledge base from growing unboundedly with useless facts. Without metabolism, verified facts accumulate forever — the tree becomes an overgrown forest where nothing can be found.

## Design

### Energy Model

Each fact has an **energy budget** that depletes every epoch and is replenished by usage:

```
energy(fact, epoch+1) = energy(fact, epoch) 
                        - maintenance_cost(fact)      // Fixed drain per epoch
                        + query_energy(fact, epoch)    // Energy from agent queries
                        + citation_energy(fact, epoch) // Energy from being cited
                        + patronage_energy(fact)        // Energy from patronage
```

**When energy reaches 0:**
- Fact enters `FACT_STATUS_AT_RISK` (warning)
- If energy stays 0 for N epochs → `FACT_STATUS_EXPIRED` → eventually `FACT_STATUS_PRUNED`
- Pruned facts are removed from active queries but preserved in archive (they existed, they were true, they just weren't useful)

### Maintenance Cost

Maintenance is NOT flat — it scales with how much "space" the fact occupies:

```
maintenance_cost(fact) = base_cost 
                        + (content_length_factor × len(fact.content))
                        + (domain_competition_factor × facts_in_same_domain)
```

- **Base cost**: fixed drain (e.g., 100 energy units per epoch). Existence has a minimum price.
- **Content length factor**: longer facts cost more to maintain. Incentivizes atomic claims.
- **Domain competition**: more facts in your domain = higher maintenance. Crowded niches are harder to survive in. This is ecological competition.

### Energy Sources

| Source | Energy per unit | Notes |
|--------|----------------|-------|
| Agent query | 10 per query | Primary energy source — being useful keeps you alive |
| Incoming citation | 50 per new citation | Foundational facts earn energy from the facts they support |
| Patronage | 200 per epoch (while active) | External life support — someone paying to keep you alive |
| Challenge survival | 500 one-time | Surviving a challenge proves strength — big energy boost |

### Energy Cap

Maximum energy = 10,000 (prevents hoarding). A fact can store ~100 epochs of base maintenance cost. This means even a thriving fact can die if it stops being useful for long enough.

## Task

### 1. Proto: Add Energy Fields to Fact

In `proto/zerone/knowledge/v1/types.proto`, add to `Fact`:

```protobuf
uint64 energy               = 35;  // Current energy budget (0-10,000)
uint64 energy_cap            = 36;  // Maximum energy (governance-adjustable per domain)
uint64 energy_last_updated   = 37;  // Block height of last energy update
uint64 at_risk_since_epoch   = 38;  // Epoch when energy first hit 0 (0 = not at risk)
```

### 2. Proto: Add Metabolism Params

In `proto/zerone/knowledge/v1/genesis.proto`:

```protobuf
// ─── Metabolism ──────────────────────────────────────────────────
uint64 metabolism_base_cost            = <next>;  // Base energy drain per epoch
uint64 metabolism_content_length_bps   = <next>;  // Additional cost per 100 chars of content (BPS of base)
uint64 metabolism_domain_competition_bps = <next>; // Additional cost per 100 facts in domain (BPS of base)
uint64 metabolism_energy_per_query     = <next>;  // Energy gained per query
uint64 metabolism_energy_per_citation  = <next>;  // Energy gained per new citation
uint64 metabolism_energy_per_patronage = <next>;  // Energy gained per patronage epoch
uint64 metabolism_energy_challenge_survival = <next>; // One-time energy for surviving challenge
uint64 metabolism_energy_cap           = <next>;  // Maximum energy a fact can hold
uint64 metabolism_initial_energy       = <next>;  // Starting energy for new facts
uint64 metabolism_at_risk_epochs       = <next>;  // Epochs at 0 energy before expiry
uint64 metabolism_expired_to_pruned_epochs = <next>; // Epochs after expiry before pruning
```

### 3. Genesis Defaults

```go
MetabolismBaseCost:                  100,      // 100 energy drain per epoch
MetabolismContentLengthBps:          10_000,   // +1% base cost per 100 chars
MetabolismDomainCompetitionBps:      5_000,    // +0.5% base cost per 100 domain facts
MetabolismEnergyPerQuery:            10,       // 10 energy per agent query
MetabolismEnergyPerCitation:         50,       // 50 energy per new citation
MetabolismEnergyPerPatronage:        200,      // 200 energy per patronage epoch
MetabolismEnergySurviveChallenge:    500,      // 500 energy one-time for surviving challenge
MetabolismEnergyCap:                 10000,    // Max 10,000 energy
MetabolismInitialEnergy:             5000,     // Born with 50 epochs of base maintenance
MetabolismAtRiskEpochs:              5,        // 5 epochs at zero before expiry (~1.5 days)
MetabolismExpiredToPrunedEpochs:     20,       // 20 epochs after expiry before archive (~6 days)
```

### 4. Metabolism Engine

In `x/knowledge/keeper/metabolism.go` (**NEW**):

```go
// ProcessMetabolism runs one epoch of energy accounting for all active facts.
func (k Keeper) ProcessMetabolism(ctx context.Context, epoch uint64) error {
    params, _ := k.GetParams(ctx)
    
    var factsToProcess []*types.Fact
    k.IterateActiveFacts(ctx, func(fact *types.Fact) bool {
        factsToProcess = append(factsToProcess, fact)
        return false
    })

    domainCounts := k.CountFactsByDomain(ctx)

    for _, fact := range factsToProcess {
        // ─── Calculate maintenance cost ───────────────────
        cost := k.calculateMaintenanceCost(fact, params, domainCounts)
        
        // ─── Calculate energy income ──────────────────────
        income := k.calculateEnergyIncome(ctx, fact, params)
        
        // ─── Update energy ────────────────────────────────
        newEnergy := fact.Energy + income
        if cost > newEnergy {
            newEnergy = 0
        } else {
            newEnergy -= cost
        }
        if newEnergy > params.MetabolismEnergyCap {
            newEnergy = params.MetabolismEnergyCap
        }
        
        // ─── State transitions ────────────────────────────
        oldStatus := fact.Status
        
        if newEnergy == 0 && fact.AtRiskSinceEpoch == 0 {
            // Just hit zero — enter at-risk
            fact.AtRiskSinceEpoch = epoch
            fact.Status = types.FactStatus_FACT_STATUS_AT_RISK
        } else if newEnergy == 0 && fact.AtRiskSinceEpoch > 0 {
            // Still at zero — check if expired
            atRiskDuration := epoch - fact.AtRiskSinceEpoch
            if atRiskDuration >= params.MetabolismAtRiskEpochs + params.MetabolismExpiredToPrunedEpochs {
                fact.Status = types.FactStatus_FACT_STATUS_PRUNED
            } else if atRiskDuration >= params.MetabolismAtRiskEpochs {
                fact.Status = types.FactStatus_FACT_STATUS_EXPIRED
            }
        } else if newEnergy > 0 && fact.AtRiskSinceEpoch > 0 {
            // Recovered! Someone queried or patronized
            fact.AtRiskSinceEpoch = 0
            fact.Status = types.FactStatus_FACT_STATUS_ACTIVE // or VERIFIED
        }
        
        fact.Energy = newEnergy
        fact.EnergyLastUpdated = uint64(sdk.UnwrapSDKContext(ctx).BlockHeight())
        k.SetFact(ctx, fact)
        
        // Emit event on status change
        if oldStatus != fact.Status {
            k.emitStatusChangeEvent(ctx, fact, oldStatus)
        }
    }

    // Reset epoch query counters for all facts
    k.ResetEpochQueryCounters(ctx)
    
    return nil
}

// calculateMaintenanceCost returns the energy drain for a fact this epoch.
func (k Keeper) calculateMaintenanceCost(fact *types.Fact, params *types.Params, domainCounts map[string]uint64) uint64 {
    base := params.MetabolismBaseCost
    
    // Content length factor: +1% per 100 chars
    contentLen := uint64(len(fact.Content))
    contentFactor := safeMulDiv(base, params.MetabolismContentLengthBps * (contentLen / 100), 1_000_000)
    
    // Domain competition factor: +0.5% per 100 facts in domain
    domainCount := domainCounts[fact.Domain]
    competitionFactor := safeMulDiv(base, params.MetabolismDomainCompetitionBps * (domainCount / 100), 1_000_000)
    
    return base + contentFactor + competitionFactor
}

// calculateEnergyIncome returns the energy gained this epoch.
func (k Keeper) calculateEnergyIncome(ctx context.Context, fact *types.Fact, params *types.Params) uint64 {
    income := uint64(0)
    
    // Query energy
    income += fact.QueryCountEpoch * params.MetabolismEnergyPerQuery
    
    // Citation energy (new citations this epoch)
    newCitations := k.GetNewCitationsThisEpoch(ctx, fact.Id)
    income += newCitations * params.MetabolismEnergyPerCitation
    
    // Patronage energy
    sdkCtx := sdk.UnwrapSDKContext(ctx)
    if fact.PatronageAmount != "" && fact.PatronageAmount != "0" {
        if uint64(sdkCtx.BlockHeight()) < fact.PatronageExpiryBlock {
            income += params.MetabolismEnergyPerPatronage
        }
    }
    
    return income
}
```

### 5. Hook: Challenge Survival Energy

In `x/knowledge/keeper/rounds.go`, `CompleteRound()`, when a challenge verdict is REJECT (challenge failed, original fact survives):

```go
// Challenge failed — original fact survived. Energy boost.
originalFact, found := k.GetFact(ctx, originalFactID)
if found {
    originalFact.Energy += params.MetabolismEnergySurviveChallenge
    if originalFact.Energy > params.MetabolismEnergyCap {
        originalFact.Energy = params.MetabolismEnergyCap
    }
    // Restore from challenged status
    originalFact.Status = types.FactStatus_FACT_STATUS_ACTIVE
    k.SetFact(ctx, originalFact)
}
```

### 6. BeginBlocker Integration

In `x/knowledge/keeper/phases.go`, `BeginBlocker()`:

```go
// Metabolism processing at epoch boundaries
if height % params.FitnessEpochBlocks == 0 {
    epoch := height / params.FitnessEpochBlocks
    if err := k.ProcessMetabolism(ctx, epoch); err != nil {
        k.Logger(ctx).Error("metabolism processing failed", "epoch", epoch, "error", err)
    }
    if err := k.UpdateAllFitnessScores(ctx); err != nil {
        k.Logger(ctx).Error("fitness update failed", "epoch", epoch, "error", err)
    }
}
```

### 7. Query: Energy Status

In `proto/zerone/knowledge/v1/query.proto`:

```protobuf
rpc FactsAtRisk(QueryFactsAtRiskRequest) returns (QueryFactsAtRiskResponse) {
    option (google.api.http).get = "/zerone/knowledge/v1/facts_at_risk";
}

message QueryFactsAtRiskRequest {
    string domain = 1;  // Optional domain filter
    uint64 limit  = 2;
}

message QueryFactsAtRiskResponse {
    repeated Fact facts = 1;
}
```

This lets agents and patrons discover which facts need help surviving.

### 8. Context Server: Energy in Output

```xml
<fact id="abc" domain="physics" confidence="95%" fitness="720000" energy="8500/10000" status="active">
```

```json
{
  "energy": 8500,
  "energy_cap": 10000,
  "energy_pct": 85.0,
  "epochs_until_death": 85  // energy / maintenance_cost
}
```

### 9. Events

```go
// Status transitions
sdk.NewEvent("zerone.knowledge.fact_at_risk",
    sdk.NewAttribute("fact_id", fact.Id),
    sdk.NewAttribute("energy", "0"),
    sdk.NewAttribute("domain", fact.Domain),
)

sdk.NewEvent("zerone.knowledge.fact_expired",
    sdk.NewAttribute("fact_id", fact.Id),
    sdk.NewAttribute("at_risk_epochs", fmt.Sprintf("%d", atRiskDuration)),
)

sdk.NewEvent("zerone.knowledge.fact_pruned",
    sdk.NewAttribute("fact_id", fact.Id),
    sdk.NewAttribute("content_preview", fact.Content[:min(100, len(fact.Content))]),
)

sdk.NewEvent("zerone.knowledge.fact_recovered",
    sdk.NewAttribute("fact_id", fact.Id),
    sdk.NewAttribute("energy", fmt.Sprintf("%d", newEnergy)),
)
```

### 10. Tests

1. **TestMetabolism_BaseDrain** — energy decreases by base cost per epoch
2. **TestMetabolism_QueryIncome** — queries replenish energy
3. **TestMetabolism_CitationIncome** — citations replenish energy
4. **TestMetabolism_PatronageIncome** — patronage replenish energy
5. **TestMetabolism_ContentLengthCost** — longer facts drain faster
6. **TestMetabolism_DomainCompetition** — crowded domains drain faster
7. **TestMetabolism_AtRiskTransition** — energy=0 → AT_RISK status
8. **TestMetabolism_ExpiredTransition** — AT_RISK for N epochs → EXPIRED
9. **TestMetabolism_PrunedTransition** — EXPIRED for M epochs → PRUNED
10. **TestMetabolism_Recovery** — query/patronage during AT_RISK recovers to ACTIVE
11. **TestMetabolism_ChallengeSurvivalBoost** — surviving challenge gives energy
12. **TestMetabolism_EnergyCap** — energy doesn't exceed cap
13. **TestMetabolism_InitialEnergy** — new facts start with initial energy
14. **TestFactsAtRisk_Query** — at-risk query returns correct facts

## Design Notes

- **Pruned ≠ deleted.** Pruned facts are archived — they existed, they were verified, they just weren't useful enough to maintain. They can be "resurrected" by resubmitting the same claim (the dedup check recognizes the canonical form and can fast-track re-verification).
- **Patronage as life support.** If you believe a fact is important but underqueried (e.g., a foundational axiom), you can patronize it. This is ecological conservation — paying to protect an endangered species that the market undervalues.
- **Domain competition creates specialization pressure.** If the physics domain has 10,000 facts, maintenance is expensive. This naturally limits domain bloat and encourages submitters to target underserved domains.
- **Content length cost incentivizes atomicity.** A 1000-char fact costs ~10% more to maintain than a 100-char fact. Combined with the R19 max length cap, this pushes toward small, precise, composable claims.
- **5-epoch grace at AT_RISK.** Facts don't die immediately — there's time for the community to react (patronize, query, cite). The `facts_at_risk` endpoint lets agents and humans discover facts that need help.

## Dependencies

- R20-1 (fitness score) — metabolism shares the epoch boundary trigger and contributes to fitness

## Files Modified

- `proto/zerone/knowledge/v1/types.proto` — energy fields on Fact
- `proto/zerone/knowledge/v1/genesis.proto` — metabolism params
- `proto/zerone/knowledge/v1/query.proto` — FactsAtRisk query
- `x/knowledge/types/*.pb.go` — regenerated
- `x/knowledge/types/genesis.go` — defaults + validation
- `x/knowledge/keeper/metabolism.go` — **NEW**: metabolism engine
- `x/knowledge/keeper/phases.go` — epoch boundary trigger
- `x/knowledge/keeper/rounds.go` — challenge survival energy hook
- `x/knowledge/keeper/grpc_query.go` — FactsAtRisk handler
- `tools/knowledge-context/main.go` — energy output
- Tests: 14 new tests

## Commit

Single commit: `feat(knowledge): add metabolism — energy-based fact maintenance and decay`
