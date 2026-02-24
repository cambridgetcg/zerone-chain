# R20-4 — Reproduction: Fact Derivation + Lineage Tracking

## Context

In biology, successful organisms reproduce. In the Tree of Knowledge, successful facts should **inspire derivative claims** — refinements, applications, extensions. A thriving physics fact about entropy should spawn related claims in chemistry, engineering, cosmology.

Lineage tracking turns the Tree of Knowledge into an actual tree — facts have parents, children, and ancestral lines. This enables:
- **Credit propagation** — original discoverers earn royalties when their fact spawns derivatives
- **Impact measurement** — how many facts descended from this one?
- **Pruning intelligence** — if a parent is disproven, all descendants need re-examination

## Design

### Lineage Model

```
fact_A (parent)
  ├── fact_B (child: REFINES A — more precise version)
  ├── fact_C (child: DERIVES from A — applied in different domain)
  └── fact_D (child: EXTENDS A — adds new information building on A)
      └── fact_E (grandchild: REFINES D)
```

A fact's **lineage** is its chain of ancestors. A fact's **progeny** is its tree of descendants.

### Reproduction Incentive

When a new claim declares a `REFINES`, `DERIVES`, or `EXTENDS` relation to a parent fact (R19-3 semantic anchors):

1. The **parent fact gains energy** — its knowledge is reproducing (citation energy + reproduction bonus)
2. The **parent's submitter earns a lineage royalty** — a percentage of the child's vesting rewards
3. The **child starts with a bonus** — inheriting some of the parent's fitness is like inheriting good genes

### Lineage Royalty

```
royalty(child) = child_vesting_reward × lineage_royalty_bps × (decay_per_generation ^ depth)
```

- `lineage_royalty_bps`: 50,000 (5%) — parent gets 5% of child's ongoing rewards
- `decay_per_generation`: 500,000 (50%) — grandparent gets 2.5%, great-grandparent gets 1.25%
- Max depth: 5 generations — no royalties beyond great-great-great-grandparent

This creates a powerful incentive: submit foundational facts that inspire others, and you earn royalties from the entire lineage tree.

## Task

### 1. Proto: Add Lineage Fields to Fact

In `proto/zerone/knowledge/v1/types.proto`, add to `Fact`:

```protobuf
string parent_fact_id          = 44;  // Direct parent (empty if original)
repeated string child_fact_ids = 45;  // Direct children
uint64 lineage_depth           = 46;  // 0 = original, 1 = child, 2 = grandchild...
uint64 progeny_count           = 47;  // Total descendants (recursively)
string lineage_root_id         = 48;  // ID of the original ancestor
```

### 2. Proto: Add Reproduction Params

In `proto/zerone/knowledge/v1/genesis.proto`:

```protobuf
// ─── Reproduction ────────────────────────────────────────────────
uint64 reproduction_royalty_bps             = <next>;  // Royalty to parent per child reward epoch
uint64 reproduction_royalty_decay_bps       = <next>;  // Decay per generation (BPS of previous)
uint64 reproduction_max_royalty_depth       = <next>;  // Max generations for royalty propagation
uint64 reproduction_parent_energy_bonus     = <next>;  // Energy bonus to parent when child is created
uint64 reproduction_child_fitness_inheritance_bps = <next>;  // % of parent fitness inherited by child
uint64 reproduction_max_children            = <next>;  // Max direct children per fact
```

### 3. Genesis Defaults

```go
ReproductionRoyaltyBps:               50_000,    // 5% of child rewards to parent
ReproductionRoyaltyDecayBps:          500_000,   // 50% per generation
ReproductionMaxRoyaltyDepth:          5,         // Max 5 generations
ReproductionParentEnergyBonus:        300,       // 300 energy to parent on child creation
ReproductionChildFitnessInheritanceBps: 200_000, // Child starts with 20% of parent fitness
ReproductionMaxChildren:              20,        // Max 20 direct children per fact
```

### 4. Lineage Registration

In `x/knowledge/keeper/rounds.go`, `createFactFromClaim()`:

```go
// Check for reproductive relations (REFINES, EXTENDS, or relation implying derivation)
for _, rel := range claim.Relations {
    if rel.Relation == types.RelationType_RELATION_TYPE_REFINES ||
       rel.Relation == types.RelationType_RELATION_TYPE_GENERALIZES {
        // This is a child of the target fact
        parentFact, found := k.GetFact(ctx, rel.TargetFactId)
        if found {
            // Check max children
            if uint64(len(parentFact.ChildFactIds)) >= params.ReproductionMaxChildren {
                k.Logger(ctx).Info("parent at max children", "parent", parentFact.Id)
                continue
            }
            
            // Set lineage
            fact.ParentFactId = parentFact.Id
            fact.LineageDepth = parentFact.LineageDepth + 1
            fact.LineageRootId = parentFact.LineageRootId
            if fact.LineageRootId == "" {
                fact.LineageRootId = parentFact.Id  // parent is the root
            }
            
            // Inherit fitness
            inheritedFitness := safeMulDiv(parentFact.FitnessScore, params.ReproductionChildFitnessInheritanceBps, 1_000_000)
            fact.FitnessScore = inheritedFitness
            
            // Update parent
            parentFact.ChildFactIds = append(parentFact.ChildFactIds, fact.Id)
            parentFact.ProgenyCount++
            parentFact.Energy += params.ReproductionParentEnergyBonus
            if parentFact.Energy > params.MetabolismEnergyCap {
                parentFact.Energy = params.MetabolismEnergyCap
            }
            k.SetFact(ctx, parentFact)
            
            // Propagate progeny count up the lineage
            k.PropagateProgenyCount(ctx, parentFact.ParentFactId)
            
            break  // Only one parent relationship
        }
    }
}
```

### 5. Royalty Distribution

In `x/vesting_rewards/keeper/keeper.go` (or knowledge keeper via hook):

When vesting rewards are distributed for a fact, also distribute lineage royalties:

```go
func (k Keeper) DistributeLineageRoyalties(ctx context.Context, factID string, rewardAmount uint64) error {
    knowledgeKeeper := k.knowledgeKeeper
    params := knowledgeKeeper.GetParams(ctx)
    
    currentFactID := factID
    depth := uint64(0)
    currentRoyaltyBps := params.ReproductionRoyaltyBps
    
    for depth < params.ReproductionMaxRoyaltyDepth {
        fact, found := knowledgeKeeper.GetFact(ctx, currentFactID)
        if !found || fact.ParentFactId == "" {
            break
        }
        
        parentFact, found := knowledgeKeeper.GetFact(ctx, fact.ParentFactId)
        if !found {
            break
        }
        
        // Calculate royalty for this ancestor
        royalty := safeMulDiv(rewardAmount, currentRoyaltyBps, 1_000_000)
        if royalty > 0 {
            parentAddr, err := sdk.AccAddressFromBech32(parentFact.Submitter)
            if err == nil {
                coins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(new(big.Int).SetUint64(royalty))))
                k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, parentAddr, coins)
            }
        }
        
        // Decay for next generation
        currentRoyaltyBps = safeMulDiv(currentRoyaltyBps, params.ReproductionRoyaltyDecayBps, 1_000_000)
        currentFactID = fact.ParentFactId
        depth++
    }
    
    return nil
}
```

### 6. Disproven Cascade

When a fact is disproven or pruned, its children need re-examination:

```go
// In CompleteRound, when verdict = ACCEPT on a challenge (original fact disproven):
func (k Keeper) CascadeDisproven(ctx context.Context, factID string) {
    fact, found := k.GetFact(ctx, factID)
    if !found {
        return
    }
    
    for _, childID := range fact.ChildFactIds {
        child, found := k.GetFact(ctx, childID)
        if !found {
            continue
        }
        
        // Children of disproven facts lose energy and enter AT_RISK
        child.Energy = child.Energy / 2  // Halve energy
        if child.Status == types.FactStatus_FACT_STATUS_ACTIVE ||
           child.Status == types.FactStatus_FACT_STATUS_VERIFIED {
            child.Status = types.FactStatus_FACT_STATUS_AT_RISK
        }
        k.SetFact(ctx, child)
        
        sdkCtx := sdk.UnwrapSDKContext(ctx)
        sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
            "zerone.knowledge.lineage_cascade",
            sdk.NewAttribute("parent_disproven", factID),
            sdk.NewAttribute("child_at_risk", childID),
            sdk.NewAttribute("child_energy", fmt.Sprintf("%d", child.Energy)),
        ))
        
        // Recursive cascade (depth-limited)
        // Children's children also affected but with diminishing impact
    }
}
```

### 7. Query: Lineage Exploration

```protobuf
rpc FactLineage(QueryFactLineageRequest) returns (QueryFactLineageResponse) {
    option (google.api.http).get = "/zerone/knowledge/v1/fact/{fact_id}/lineage";
}

rpc FactProgeny(QueryFactProgenyRequest) returns (QueryFactProgenyResponse) {
    option (google.api.http).get = "/zerone/knowledge/v1/fact/{fact_id}/progeny";
}

message QueryFactLineageRequest {
    string fact_id  = 1;
    uint64 depth    = 2;  // How far up to trace (default: to root)
}

message QueryFactLineageResponse {
    repeated Fact ancestors = 1;  // Ordered: parent, grandparent, ...root
    string root_id = 2;
}

message QueryFactProgenyRequest {
    string fact_id  = 1;
    uint64 depth    = 2;  // How deep to traverse (default: 3)
}

message QueryFactProgenyResponse {
    Fact root = 1;
    repeated FactWithChildren tree = 2;  // Recursive tree structure
}

message FactWithChildren {
    Fact fact = 1;
    repeated FactWithChildren children = 2;
}
```

### 8. Context Server: Lineage-Aware Context

Add to `/context`:
- `include_lineage=true` — include parent/children info
- `progeny_of=FACT_ID` — return all descendants of a fact

```xml
<fact id="abc" parent="def" children="3" progeny="12" lineage_depth="1" lineage_root="def">
```

### 9. Tests

1. **TestReproduction_ParentChildLink** — child fact links to parent correctly
2. **TestReproduction_LineageDepth** — depth increments through generations
3. **TestReproduction_FitnessInheritance** — child starts with 20% of parent fitness
4. **TestReproduction_ParentEnergyBonus** — parent gains energy when child created
5. **TestReproduction_ProgenyCountPropagation** — progeny count propagates up lineage
6. **TestRoyalty_ParentGets5Percent** — parent receives 5% of child reward
7. **TestRoyalty_GrandparentGets2_5Percent** — grandparent receives 2.5%
8. **TestRoyalty_MaxDepth** — royalties stop at 5 generations
9. **TestDisprovenCascade_ChildrenAtRisk** — disproven parent puts children at risk
10. **TestMaxChildren_Enforced** — parent at 20 children rejects new child links
11. **TestLineageQuery_TracesToRoot** — lineage query returns full ancestor chain
12. **TestProgenyQuery_ReturnsTree** — progeny query returns descendant tree

## Design Notes

- **Single parent only.** A fact can have one parent (the primary relationship). It can have multiple semantic anchors (R19-3) but only one lineage parent. This keeps the tree structure clean. Multiple parents would create a DAG, making royalties complex.
- **Royalty comes from vesting rewards, not the child's review fee.** The fee pays for review. Royalties come from the ongoing economic value the child generates. Parent earns only when child produces value.
- **Disproven cascade is intentionally aggressive.** If your axiom is wrong, everything built on it is suspect. Halving energy + AT_RISK gives children time to prove independence (maybe they're still valid without the parent), but creates urgency.
- **Max 20 children prevents royalty farming.** Without a cap, someone could submit 1000 trivial variations of a popular fact to dilute the niche while generating royalties for the parent.
- **Lineage root tracking** enables "who planted the seed?" queries — trace any fact back to the original insight that started the knowledge tree.

## Dependencies

- R19-3 (semantic anchors) — REFINES/EXTENDS relations trigger reproduction
- R20-1 (fitness score) — fitness inheritance
- R20-2 (metabolism) — energy bonus to parent

## Files Modified

- `proto/zerone/knowledge/v1/types.proto` — lineage fields on Fact
- `proto/zerone/knowledge/v1/genesis.proto` — reproduction params
- `proto/zerone/knowledge/v1/query.proto` — FactLineage, FactProgeny queries
- `x/knowledge/types/*.pb.go` — regenerated
- `x/knowledge/types/genesis.go` — defaults + validation
- `x/knowledge/keeper/reproduction.go` — **NEW**: lineage registration, royalty distribution
- `x/knowledge/keeper/rounds.go` — lineage setup in createFactFromClaim, disproven cascade
- `x/knowledge/keeper/grpc_query.go` — lineage/progeny handlers
- `x/vesting_rewards/keeper/keeper.go` — royalty distribution hook
- `tools/knowledge-context/main.go` — lineage-aware output
- Tests: 12 new tests

## Commit

Single commit: `feat(knowledge): add fact reproduction — lineage tracking + royalty distribution`
