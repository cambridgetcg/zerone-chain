# R20-3 — Natural Selection: Competitive Pruning + Niche Dynamics

## Context

R20-1 gives facts fitness scores. R20-2 gives them metabolism. This session adds the **competitive layer** — facts don't just die from starvation, they die because *better facts take their niche*.

In ecology, two species can't occupy the same niche indefinitely (competitive exclusion principle). In the Tree of Knowledge, two facts about the same subject in the same domain compete. The fitter one wins resources; the weaker one starves.

This is how "water boils at 100°C" gets naturally displaced by "water boils at 99.97°C at 101.325 kPa" — the more precise fact captures the queries, citations, and energy that the vague one needs to survive.

## Design

### Niche Definition

A fact's **niche** is defined by:

```
niche(fact) = (domain, subject, claim_type)
```

Two facts occupy the same niche if they share the same domain, same subject (from R19-4 structured fields), and same claim type. Facts without structure are in a niche of one (no competition — but also no niche protection).

### Competitive Exclusion

At each metabolism epoch, for each niche with >1 fact:

1. **Rank facts by fitness** (R20-1)
2. **The top fact is the niche leader** — it gets a fitness bonus (niche dominance bonus)
3. **Lower-ranked facts pay an extra maintenance cost** (competition tax) proportional to the gap:
   ```
   competition_tax(fact) = base_maintenance × (1 - fact.fitness / leader.fitness)
   ```
   A fact with half the leader's fitness pays +100% maintenance. A fact with 90% of the leader's fitness pays +10%.
4. **Facts below a fitness ratio threshold are marked as redundant** (if fact.fitness < leader.fitness × redundancy_threshold)

### Niche Dynamics

**Niche splitting:** If a fact has structured fields that are *more specific* than the leader (e.g., leader subject="water boiling point", new subject="water boiling point at altitude"), it's in a *sub-niche*, not competing. Sub-niches are determined by the `scope` field — different scope = different niche.

**Niche succession:** When the niche leader dies (pruned or disproven), the second-ranked fact inherits the niche dominance bonus. Knowledge evolves.

**Cross-niche symbiosis:** Facts that are in different niches but linked via `SUPPORTS` relations (R19-3) boost each other's fitness. Supporting a healthy fact makes you healthier. This creates stable knowledge clusters.

### Displacement Event

When a new fact enters a niche and has higher fitness than the current leader within 3 epochs:

```
Event: zerone.knowledge.niche_displacement
  displaced_fact: old leader
  new_leader: new fact
  niche: (domain, subject, type)
```

This is the evolutionary moment — better knowledge replacing worse knowledge.

## Task

### 1. Proto: Add Niche Fields

In `proto/zerone/knowledge/v1/types.proto`, add to `Fact`:

```protobuf
string niche_key            = 39;  // Computed: hash(domain + subject + claim_type)
bool   niche_leader         = 40;  // Is this the top-ranked fact in its niche?
uint64 niche_rank           = 41;  // Rank within niche (1 = leader)
uint64 niche_size           = 42;  // How many facts in this niche
uint64 competition_tax      = 43;  // Extra maintenance from competition (energy units)
```

### 2. Proto: Add Competition Params

In `proto/zerone/knowledge/v1/genesis.proto`:

```protobuf
// ─── Competition ─────────────────────────────────────────────────
uint64 competition_niche_dominance_bonus_bps = <next>;  // Fitness bonus for niche leader
uint64 competition_redundancy_threshold_bps  = <next>;  // Below this ratio of leader fitness = redundant
uint64 competition_max_niche_size            = <next>;  // Max facts per niche before forced pruning
uint64 competition_symbiosis_bonus_bps       = <next>;  // Fitness bonus per SUPPORTS link to healthy fact
```

### 3. Genesis Defaults

```go
CompetitionNicheDominanceBonusBps:  100_000,  // +10% fitness for niche leader
CompetitionRedundancyThresholdBps:  200_000,  // Below 20% of leader = redundant
CompetitionMaxNicheSize:            10,        // Max 10 facts per niche
CompetitionSymbiosisBonusBps:       50_000,   // +5% fitness per healthy SUPPORTS link
```

### 4. Niche Index

In `x/knowledge/types/keys.go`:

```go
NicheIndexPrefix    = []byte{0x38}  // 0x38 | niche_key | fitness_score (desc) | fact_id → []byte{1}
NicheMembersPrefix  = []byte{0x39}  // 0x39 | niche_key → []fact_id
```

In `x/knowledge/keeper/state.go`:

```go
func (k Keeper) ComputeNicheKey(fact *types.Fact) string  // hash(domain + subject + claim_type)
func (k Keeper) GetNicheMembers(ctx context.Context, nicheKey string) []*types.Fact
func (k Keeper) GetNicheLeader(ctx context.Context, nicheKey string) (*types.Fact, bool)
func (k Keeper) UpdateNicheIndex(ctx context.Context, fact *types.Fact) error
```

### 5. Competition Engine

In `x/knowledge/keeper/competition.go` (**NEW**):

```go
// ProcessCompetition runs niche-based competition for all active niches.
func (k Keeper) ProcessCompetition(ctx context.Context, epoch uint64) error {
    params, _ := k.GetParams(ctx)
    
    // Collect all unique niches
    niches := k.GetAllNiches(ctx)
    
    for _, nicheKey := range niches {
        members := k.GetNicheMembers(ctx, nicheKey)
        if len(members) <= 1 {
            // Sole occupant — no competition
            if len(members) == 1 {
                members[0].NicheLeader = true
                members[0].NicheRank = 1
                members[0].NicheSize = 1
                members[0].CompetitionTax = 0
                k.SetFact(ctx, members[0])
            }
            continue
        }
        
        // Sort by fitness descending
        sort.Slice(members, func(i, j int) bool {
            return members[i].FitnessScore > members[j].FitnessScore
        })
        
        leader := members[0]
        leaderFitness := leader.FitnessScore
        if leaderFitness == 0 {
            leaderFitness = 1 // avoid division by zero
        }
        
        for rank, fact := range members {
            fact.NicheRank = uint64(rank + 1)
            fact.NicheSize = uint64(len(members))
            fact.NicheLeader = (rank == 0)
            
            if rank == 0 {
                // Leader gets dominance bonus (applied in fitness calculation)
                fact.CompetitionTax = 0
            } else {
                // Competition tax: proportional to fitness gap
                fitnessRatio := safeMulDiv(fact.FitnessScore, 1_000_000, leaderFitness)
                gap := 1_000_000 - fitnessRatio
                fact.CompetitionTax = safeMulDiv(params.MetabolismBaseCost, gap, 1_000_000)
            }
            
            // Check redundancy threshold
            if !fact.NicheLeader {
                ratio := safeMulDiv(fact.FitnessScore, 1_000_000, leaderFitness)
                if ratio < params.CompetitionRedundancyThresholdBps {
                    // Mark as redundant — accelerated decay
                    fact.CompetitionTax *= 3  // Triple maintenance for redundant facts
                }
            }
            
            k.SetFact(ctx, fact)
        }
        
        // Forced pruning if niche exceeds max size
        if uint64(len(members)) > params.CompetitionMaxNicheSize {
            // Prune weakest facts beyond max size
            for i := int(params.CompetitionMaxNicheSize); i < len(members); i++ {
                members[i].Status = types.FactStatus_FACT_STATUS_PRUNED
                members[i].Energy = 0
                k.SetFact(ctx, members[i])
                
                k.emitNichePruneEvent(ctx, members[i], leader)
            }
        }
    }
    
    return nil
}

// ProcessSymbiosis applies fitness bonuses for SUPPORTS relationships.
func (k Keeper) ProcessSymbiosis(ctx context.Context, params *types.Params) {
    // For each fact with outgoing SUPPORTS relations:
    //   If the supported fact is healthy (fitness > 500k), give a symbiosis bonus
    // This creates stable knowledge clusters where connected facts sustain each other
    k.IterateActiveFacts(ctx, func(fact *types.Fact) bool {
        relations := k.GetFactRelationsByType(ctx, fact.Id, types.RelationType_RELATION_TYPE_SUPPORTS)
        symbiosisBonus := uint64(0)
        for _, rel := range relations {
            targetFact, found := k.GetFact(ctx, rel.TargetFactId)
            if found && targetFact.FitnessScore > 500_000 {
                symbiosisBonus += params.CompetitionSymbiosisBonusBps
            }
        }
        if symbiosisBonus > 0 {
            // Add bonus to fitness (capped)
            fact.FitnessScore += safeMulDiv(symbiosisBonus, fact.FitnessScore, 1_000_000)
            if fact.FitnessScore > 1_000_000 {
                fact.FitnessScore = 1_000_000
            }
            k.SetFact(ctx, fact)
        }
        return false
    })
}
```

### 6. Integration with Metabolism

In `x/knowledge/keeper/metabolism.go`, `calculateMaintenanceCost()`, add competition tax:

```go
// Add competition tax from niche dynamics
totalCost := base + contentFactor + competitionFactor + fact.CompetitionTax
```

### 7. BeginBlocker Integration

In `x/knowledge/keeper/phases.go`:

```go
if height % params.FitnessEpochBlocks == 0 {
    epoch := height / params.FitnessEpochBlocks
    
    // Order matters:
    // 1. Update fitness scores (current usage data)
    k.UpdateAllFitnessScores(ctx)
    // 2. Process competition (uses fitness to rank niches)
    k.ProcessCompetition(ctx, epoch)
    // 3. Process symbiosis (adjusts fitness based on relationships)
    k.ProcessSymbiosis(ctx, params)
    // 4. Process metabolism (uses fitness + competition tax to drain/replenish energy)
    k.ProcessMetabolism(ctx, epoch)
}
```

### 8. Niche Displacement Detection

When a new fact is created (in `createFactFromClaim`), check if it enters an existing niche:

```go
nicheKey := k.ComputeNicheKey(fact)
currentLeader, hasLeader := k.GetNicheLeader(ctx, nicheKey)
if hasLeader {
    sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
        "zerone.knowledge.niche_challenger",
        sdk.NewAttribute("new_fact", fact.Id),
        sdk.NewAttribute("current_leader", currentLeader.Id),
        sdk.NewAttribute("niche", nicheKey),
        sdk.NewAttribute("domain", fact.Domain),
    ))
}
k.UpdateNicheIndex(ctx, fact)
```

### 9. Query: Niche Exploration

```protobuf
rpc NicheInfo(QueryNicheInfoRequest) returns (QueryNicheInfoResponse) {
    option (google.api.http).get = "/zerone/knowledge/v1/niche/{niche_key}";
}

rpc NichesByDomain(QueryNichesByDomainRequest) returns (QueryNichesByDomainResponse) {
    option (google.api.http).get = "/zerone/knowledge/v1/niches/{domain}";
}

message QueryNicheInfoResponse {
    string niche_key        = 1;
    string domain           = 2;
    string subject          = 3;
    Fact leader             = 4;
    repeated Fact members   = 5;  // Sorted by fitness desc
    uint64 total_energy     = 6;  // Sum of all member energy
}
```

### 10. Context Server: Competition-Aware Output

Add to `/context`:
- `niche_leader_only=true` — return only niche leaders (best fact per subject)
- Include niche metadata in output:

```xml
<fact id="abc" niche_rank="1" niche_size="3" niche_leader="true">
```

### 11. Tests

1. **TestNicheKey_SameSubjectSameDomain** — same niche key generated
2. **TestNicheKey_DifferentScope** — different scope = different niche
3. **TestCompetition_LeaderGetsBonus** — highest fitness fact gets dominance bonus
4. **TestCompetition_TaxProportionalToGap** — competition tax scales with fitness gap
5. **TestCompetition_RedundantTripleTax** — below threshold = 3× maintenance
6. **TestCompetition_ForcedPruning** — niche exceeds max size → weakest pruned
7. **TestNicheDisplacement** — new fact overtakes leader → event emitted
8. **TestNicheSuccession** — leader dies → second-ranked inherits
9. **TestSymbiosis_SupportsBonus** — SUPPORTS link to healthy fact boosts fitness
10. **TestSymbiosis_NoBonus_UnhealthyTarget** — SUPPORTS link to unhealthy fact gives nothing
11. **TestCompetition_UnstructuredFacts** — facts without structure have solo niches
12. **TestWaterBoiling_Displacement** — "boils at 100°C" displaced by "boils at 99.97°C at 101.325 kPa"

## Design Notes

- **Niche requires R19-4 structure.** Without subject/scope fields, niche can't be computed. Unstructured facts are each in their own niche — they don't compete but also don't get niche protection. This is intentional: structured claims get competitive advantage.
- **Competition tax is additive to base maintenance.** A redundant fact in a crowded niche pays base + content + domain + 3× competition. It dies fast. This is by design — redundant knowledge should be aggressively pruned.
- **Max niche size (10) prevents infinite competition.** If 100 facts claim the same subject, the bottom 90 are force-pruned. This keeps niche dynamics tractable and prevents bloat attacks.
- **Symbiosis creates knowledge clusters.** Physics facts that SUPPORT each other form stable ecosystems. Isolated facts without connections are more vulnerable — incentivizes building linked knowledge.
- **"Water boils at 100°C" scenario:** submitted in niche (physics, "water boiling point", assertion). Already well-known, so agents don't query it (low query energy). If someone submits "water boils at 99.97°C at 101.325 kPa" in the same niche, it's more precise → more useful → higher fitness → becomes leader → original fact pays competition tax → starves → pruned. Better knowledge wins.

## Dependencies

- R20-1 (fitness score) — competition rankings use fitness
- R20-2 (metabolism) — competition tax feeds into maintenance cost
- R19-3 (semantic anchors) — symbiosis uses SUPPORTS relations
- R19-4 (structured fields) — niche key uses subject + scope

## Files Modified

- `proto/zerone/knowledge/v1/types.proto` — niche fields on Fact
- `proto/zerone/knowledge/v1/genesis.proto` — competition params
- `proto/zerone/knowledge/v1/query.proto` — NicheInfo, NichesByDomain queries
- `x/knowledge/types/*.pb.go` — regenerated
- `x/knowledge/types/keys.go` — niche index prefixes
- `x/knowledge/types/genesis.go` — defaults + validation
- `x/knowledge/keeper/competition.go` — **NEW**: competition engine + symbiosis
- `x/knowledge/keeper/state.go` — niche index CRUD
- `x/knowledge/keeper/metabolism.go` — competition tax integration
- `x/knowledge/keeper/phases.go` — epoch processing order
- `x/knowledge/keeper/rounds.go` — niche registration on fact creation
- `tools/knowledge-context/main.go` — niche-aware output
- Tests: 12 new tests

## Commit

Single commit: `feat(knowledge): add competitive niche dynamics for fact natural selection`
