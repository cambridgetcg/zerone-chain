# R20-1 — Fitness Score: Usage-Based Fact Vitality

## Context

Facts currently live forever once verified. There's no signal for whether a fact is actually useful. A fact cited by 100 other facts and queried 10,000 times by agents has the same on-chain status as a fact nobody has ever looked at.

The **fitness score** is a composite metric that measures a fact's value to the ecosystem. It's the fact's "life force" — when it drops to zero, the fact is dying. When it's high, the fact is thriving.

## Design

### Fitness Function

```
fitness(fact, epoch) = 
    w_query  × query_rate(fact, epoch)        +  // How often agents fetch this fact
    w_cite   × citation_rate(fact, epoch)     +  // How many facts reference this
    w_bridge × bridge_score(fact)             +  // Cross-domain connectivity
    w_depth  × dependency_depth(fact)         +  // How many facts depend on this (transitively)
    w_patron × patronage_active(fact)         +  // Active patronage (someone paying to keep it alive)
    w_unique × uniqueness_score(fact, epoch)  -  // Inverse of redundancy
    w_age    × age_penalty(fact, epoch)          // Older uncited facts lose fitness
```

**Default weights (BPS, sum = flexible):**

| Weight | Default | Rationale |
|--------|---------|-----------|
| `w_query` | 300,000 (30%) | Agent usage is the primary signal |
| `w_cite` | 250,000 (25%) | Facts cited by other facts are foundational |
| `w_bridge` | 100,000 (10%) | Cross-domain facts are rare and valuable |
| `w_depth` | 100,000 (10%) | Facts with deep dependency trees are load-bearing |
| `w_patron` | 50,000 (5%) | Someone is willing to pay for this fact's survival |
| `w_unique` | 100,000 (10%) | Non-redundant facts score higher |
| `w_age` | 100,000 (10%) | Uncited old facts decay — cited old facts don't |

### Fitness Scale

0–1,000,000 BPS (same as confidence). Stored on the Fact proto.

- **0–100,000**: Critical — fact is at risk of pruning
- **100,000–300,000**: Low — fact is underperforming
- **300,000–600,000**: Healthy — fact is contributing
- **600,000–800,000**: Thriving — fact is highly valuable
- **800,000–1,000,000**: Keystone — fact is essential to the ecosystem

## Task

### 1. Proto: Add Fitness Fields to Fact

In `proto/zerone/knowledge/v1/types.proto`, add to `Fact`:

```protobuf
uint64 fitness_score           = 30;  // 0-1,000,000 composite fitness
uint64 fitness_updated_block   = 31;  // Last block fitness was recalculated
uint64 query_count             = 32;  // Lifetime query count
uint64 query_count_epoch       = 33;  // Queries in current epoch
uint64 epoch_born              = 34;  // Epoch when fact was created
```

### 2. Proto: Add Fitness Params

In `proto/zerone/knowledge/v1/genesis.proto`, add to `Params`:

```protobuf
// ─── Fitness scoring ─────────────────────────────────────────────
uint64 fitness_epoch_blocks        = <next>;  // Blocks per fitness epoch
uint64 fitness_weight_query_bps    = <next>;  // Weight for query rate
uint64 fitness_weight_citation_bps = <next>;  // Weight for citation rate
uint64 fitness_weight_bridge_bps   = <next>;  // Weight for bridge score
uint64 fitness_weight_depth_bps    = <next>;  // Weight for dependency depth
uint64 fitness_weight_patron_bps   = <next>;  // Weight for active patronage
uint64 fitness_weight_unique_bps   = <next>;  // Weight for uniqueness
uint64 fitness_weight_age_bps      = <next>;  // Weight for age penalty
uint64 fitness_initial_score       = <next>;  // Score assigned at birth (grace period)
uint64 fitness_grace_epochs        = <next>;  // Epochs before age penalty kicks in
```

### 3. Genesis Defaults

```go
FitnessEpochBlocks:       10000,    // ~7 hours at 2.5s blocks
FitnessWeightQueryBps:    300000,   // 30%
FitnessWeightCitationBps: 250000,   // 25%
FitnessWeightBridgeBps:   100000,   // 10%
FitnessWeightDepthBps:    100000,   // 10%
FitnessWeightPatronBps:    50000,   // 5%
FitnessWeightUniqueBps:   100000,   // 10%
FitnessWeightAgeBps:      100000,   // 10%
FitnessInitialScore:      500000,   // Born healthy — 50%
FitnessGraceEpochs:       10,       // ~3 days before age penalty
```

### 4. Query Tracking

Every time a fact is fetched via gRPC query, increment its query counter.

In `x/knowledge/keeper/grpc_query.go`, `Fact()`:

```go
func (k Keeper) Fact(ctx context.Context, req *types.QueryFactRequest) (*types.QueryFactResponse, error) {
    fact, found := k.GetFact(ctx, req.FactId)
    if !found {
        return nil, status.Error(codes.NotFound, "fact not found")
    }

    // Track query — increment counter (non-blocking, best-effort)
    // Only count external queries, not internal keeper calls
    if req.TrackQuery {
        k.IncrementFactQueryCount(ctx, req.FactId)
    }

    return &types.QueryFactResponse{Fact: fact}, nil
}
```

Add `track_query` bool to `QueryFactRequest`:

```protobuf
message QueryFactRequest {
    string fact_id    = 1;
    bool track_query  = 2;  // Increment query counter (agents set this)
}
```

The context server sets `track_query=true`. Direct CLI queries don't.

**REST tracking:** Add middleware to the `/context` endpoint that calls a tracking endpoint:

```
POST /zerone/knowledge/v1/track_query
{ "fact_ids": ["abc", "def"] }
```

This is a separate tx-less endpoint — query tracking shouldn't require a transaction (too expensive). Instead, track in ephemeral state that gets committed periodically.

**Alternative (simpler):** Track queries in the context server's memory, batch-submit a single `MsgReportQueries` transaction every N minutes with aggregated counts. This avoids per-query on-chain cost.

### 5. Fitness Calculator

In `x/knowledge/keeper/fitness.go` (**NEW**):

```go
// CalculateFitness computes the fitness score for a single fact.
func (k Keeper) CalculateFitness(ctx context.Context, fact *types.Fact, epoch uint64) uint64 {
    params, _ := k.GetParams(ctx)

    // ─── Query rate component ──────────────────────────────
    // Normalize: queries per epoch, capped at 1000 for scaling
    queryRate := min(fact.QueryCountEpoch, 1000)
    queryScore := safeMulDiv(queryRate, 1_000_000, 1000)  // 0-1M based on queries

    // ─── Citation rate component ───────────────────────────
    // incoming_citation_count from Fact proto
    citationScore := min(fact.IncomingCitationCount * 100_000, 1_000_000)  // 10 citations = max

    // ─── Bridge score component ────────────────────────────
    // Already on Fact proto (0-1,000,000)
    bridgeScore := fact.BridgeScore

    // ─── Dependency depth component ────────────────────────
    // How many facts transitively depend on this fact
    depthCount := k.CountTransitiveDependents(ctx, fact.Id, 5)  // max depth 5
    depthScore := min(depthCount * 200_000, 1_000_000)  // 5 dependents = max

    // ─── Patronage component ───────────────────────────────
    patronScore := uint64(0)
    if fact.PatronageAmount != "" && fact.PatronageAmount != "0" {
        sdkCtx := sdk.UnwrapSDKContext(ctx)
        if uint64(sdkCtx.BlockHeight()) < fact.PatronageExpiryBlock {
            patronScore = 1_000_000  // Binary: active patronage = full score
        }
    }

    // ─── Uniqueness component ──────────────────────────────
    // Inverse of how many facts share the same subject in the same domain
    uniqueScore := k.CalculateUniqueness(ctx, fact)

    // ─── Age penalty component ─────────────────────────────
    factAge := epoch - fact.EpochBorn
    agePenalty := uint64(0)
    if factAge > params.FitnessGraceEpochs {
        // Penalty grows linearly after grace period, capped at full weight
        penaltyEpochs := factAge - params.FitnessGraceEpochs
        agePenalty = min(penaltyEpochs * 50_000, 1_000_000)  // 20 epochs past grace = max penalty
    }
    // BUT: cited facts resist aging — each citation reduces penalty by 100k
    if fact.IncomingCitationCount > 0 {
        reduction := min(fact.IncomingCitationCount * 100_000, agePenalty)
        agePenalty -= reduction
    }

    // ─── Weighted sum ──────────────────────────────────────
    fitness := uint64(0)
    fitness += safeMulDiv(queryScore, params.FitnessWeightQueryBps, 1_000_000)
    fitness += safeMulDiv(citationScore, params.FitnessWeightCitationBps, 1_000_000)
    fitness += safeMulDiv(bridgeScore, params.FitnessWeightBridgeBps, 1_000_000)
    fitness += safeMulDiv(depthScore, params.FitnessWeightDepthBps, 1_000_000)
    fitness += safeMulDiv(patronScore, params.FitnessWeightPatronBps, 1_000_000)
    fitness += safeMulDiv(uniqueScore, params.FitnessWeightUniqueBps, 1_000_000)

    // Subtract age penalty
    ageDeduction := safeMulDiv(agePenalty, params.FitnessWeightAgeBps, 1_000_000)
    if ageDeduction > fitness {
        fitness = 0
    } else {
        fitness -= ageDeduction
    }

    // Cap at 1,000,000
    if fitness > 1_000_000 {
        fitness = 1_000_000
    }

    return fitness
}
```

### 6. Epoch Fitness Update (BeginBlocker)

In `x/knowledge/keeper/phases.go`, `BeginBlocker()`, add:

```go
// Check if we're at a fitness epoch boundary
if height % params.FitnessEpochBlocks == 0 {
    if err := k.UpdateAllFitnessScores(ctx); err != nil {
        k.Logger(ctx).Error("fitness update failed", "error", err)
    }
}
```

`UpdateAllFitnessScores` iterates all verified/active facts, recalculates fitness, stores the new score, and resets epoch query counters.

### 7. Query: Fitness-Sorted Retrieval

In `proto/zerone/knowledge/v1/query.proto`, add:

```protobuf
rpc FactsByFitness(QueryFactsByFitnessRequest) returns (QueryFactsByFitnessResponse) {
    option (google.api.http).get = "/zerone/knowledge/v1/facts_by_fitness";
}

message QueryFactsByFitnessRequest {
    string domain          = 1;  // Optional domain filter
    uint64 min_fitness     = 2;  // Minimum fitness score
    uint64 limit           = 3;  // Max results (default 50)
    string order           = 4;  // "desc" (default) or "asc"
}
```

### 8. Context Server: Fitness-Aware Context

Update `tools/knowledge-context/main.go`:

- Add `sort=fitness` parameter to `/context` (sort facts by fitness instead of confidence)
- Add `min_fitness=N` parameter to filter low-fitness facts
- Include fitness score in all output formats:

```xml
<fact id="abc" domain="physics" confidence="95%" fitness="720000" fitness_label="thriving">
```

```json
{
  "fitness_score": 720000,
  "fitness_pct": 72.0,
  "fitness_label": "thriving"
}
```

### 9. Events

```go
sdk.NewEvent("zerone.knowledge.fitness_updated",
    sdk.NewAttribute("fact_id", fact.Id),
    sdk.NewAttribute("fitness_score", fmt.Sprintf("%d", newFitness)),
    sdk.NewAttribute("fitness_label", fitnessLabel(newFitness)),
    sdk.NewAttribute("query_count_epoch", fmt.Sprintf("%d", fact.QueryCountEpoch)),
    sdk.NewAttribute("epoch", fmt.Sprintf("%d", epoch)),
)
```

Only emit for significant changes (>50,000 BPS delta) to avoid event spam.

### 10. Tests

1. **TestCalculateFitness_HighQuery** — heavily queried fact scores high
2. **TestCalculateFitness_HighCitation** — well-cited fact scores high
3. **TestCalculateFitness_ZeroUsage** — unused fact decays after grace period
4. **TestCalculateFitness_AgeResistance** — cited facts resist age penalty
5. **TestCalculateFitness_BridgeBonus** — cross-domain facts get bridge bonus
6. **TestCalculateFitness_PatronageKeepsAlive** — patronage prevents fitness death
7. **TestCalculateFitness_GracePeriod** — new facts have grace period before decay
8. **TestUpdateAllFitness_EpochBoundary** — bulk update triggers at epoch boundary
9. **TestQueryByFitness_Sorted** — facts returned sorted by fitness
10. **TestFitnessLabel** — score maps to correct label (critical/low/healthy/thriving/keystone)

## Design Notes

- **Query tracking is the hardest part.** On-chain per-query tracking is too expensive. The recommended approach: context server tracks queries in memory, batch-submits a `MsgReportQueries` every ~100 blocks with aggregated counts. This needs a new message type and a trusted reporter pattern (the context server address is whitelisted, or queries are signed).
- **Fitness is not confidence.** Confidence measures how sure we are the fact is true. Fitness measures how useful the fact is. A fact can be 100% confident (definitely true) but 0% fit (nobody cares). "Water boils at 100°C" is high confidence, low fitness.
- **Initial score (500k) + grace period (10 epochs)** — new facts are born healthy and get ~3 days to prove themselves. This prevents immediate death of legitimate facts that haven't been discovered yet.
- **Age penalty resisted by citations** — old facts that are well-cited don't decay. "2+2=4" submitted in epoch 1 and cited by 50 other facts never loses fitness. "The weather in London on Feb 24 was rainy" with zero citations decays normally.
- **Patronage as life support** — anyone can keep a fact alive by patronizing it (paying uzrn). This is the "endangered species protection" mechanism. If you think a fact is valuable but underqueried, pay to keep it.
- **All weights are governance-adjustable** — the ecosystem can evolve its own fitness criteria.

## Files Modified

- `proto/zerone/knowledge/v1/types.proto` — fitness fields on Fact
- `proto/zerone/knowledge/v1/genesis.proto` — fitness params
- `proto/zerone/knowledge/v1/query.proto` — FactsByFitness, TrackQuery, QueryFactRequest.track_query
- `x/knowledge/types/*.pb.go` — regenerated
- `x/knowledge/types/genesis.go` — defaults + validation
- `x/knowledge/keeper/fitness.go` — **NEW**: fitness calculator, epoch update, uniqueness
- `x/knowledge/keeper/phases.go` — epoch boundary trigger
- `x/knowledge/keeper/grpc_query.go` — query tracking, FactsByFitness handler
- `tools/knowledge-context/main.go` — fitness-aware output + sorting
- Tests: 10 new tests

## Commit

Single commit: `feat(knowledge): add fitness score — usage-based fact vitality metric`
