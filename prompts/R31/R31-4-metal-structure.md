# R31-4 — 金 Metal (Structure): Defense as Refinement

## Phase Identity

Metal is the phase of structure — contracting, refining, separating pure from impure. Capture defense, domain qualification, and ontology are metal: they create the rigid structures that separate qualified from unqualified, captured from healthy, stratum from stratum. Metal is the immune system — it identifies what doesn't belong and removes it.

## Relationships

### Generates → Water (Structure channels flow)

**Status:** ⚠️ Partial. R29-5 connected capture defense to partnerships (structural immunity). But qualification gating doesn't inform discovery matching, and ontology structure doesn't shape partnership recommendations.

**New connection:** Domain qualification status and ontology stratum structure should influence discovery and partnership matching. Partners should be matched based on complementary qualifications, and discovery should prefer to match agents/humans with domains where they're qualified.

```go
// New interface method on DomainQualificationKeeper (for discovery):
GetQualifiedDomains(ctx context.Context, account string) []string
```

Implementation:
```go
func (k Keeper) GetQualifiedDomains(ctx context.Context, account string) []string {
    var domains []string
    k.IterateQualifications(ctx, func(q *types.DomainQualification) bool {
        if q.Account == account && q.Qualified {
            domains = append(domains, q.Domain)
        }
        return false
    })
    return domains
}
```

In discovery's matching algorithm:
```go
func (k Keeper) ScoreDiscoveryMatch(ctx context.Context, seeker, candidate string) uint64 {
    baseScore := k.calculateBaseMatchScore(ctx, seeker, candidate)
    
    // Metal generates Water: qualification overlap boosts match quality
    if k.qualificationKeeper != nil {
        seekerDomains := k.qualificationKeeper.GetQualifiedDomains(ctx, seeker)
        candidateDomains := k.qualificationKeeper.GetQualifiedDomains(ctx, candidate)
        
        // Complementary qualifications → higher score
        // (partners who cover different domains are more valuable)
        overlap := countOverlap(seekerDomains, candidateDomains)
        total := countUnion(seekerDomains, candidateDomains)
        
        if total > 0 {
            complementarity := (total - overlap) * BPS / total
            baseScore = baseScore * (BPS + complementarity * 200_000 / BPS) / BPS // up to 20% bonus
        }
    }
    
    return baseScore
}
```

**New keeper dependency in discovery:**
```go
type DomainQualificationKeeper interface {
    GetQualifiedDomains(ctx context.Context, account string) []string
}
```

Also, cross-stratum partnership recommendations from ontology:
```go
// New interface method on OntologyKeeper (for partnerships):
GetRelatedStrata(ctx context.Context, domain string) []string
```

In partnerships formation matching:
```go
// Bonus for partnerships that bridge different strata
// (theoretical + empirical partnership > two theoretical)
func (k Keeper) ScoreFormationMatchWithStratum(ctx context.Context, match *types.FormationMatch) uint64 {
    score := k.ScoreFormationMatch(ctx, match)
    
    if k.ontologyKeeper != nil {
        mentorStrata := k.ontologyKeeper.GetRelatedStrata(ctx, match.MentorDomain)
        menteeDomain := match.Domain
        
        // Cross-stratum mentorship gets priority (Metal channels flow across boundaries)
        if !contains(mentorStrata, menteeDomain) {
            score = score * 1_200_000 / BPS // 20% bonus for cross-stratum
        }
    }
    
    return score
}
```

### ← Controlled by Fire (Activity melts rigidity)

**Status:** ⚠️ Partial. R29-3 role elasticity changes based on vindication outcomes. But verification activity doesn't directly modulate defense sensitivity.

**New connection:** High verification activity in a domain should make capture defense *less sensitive* (lower the effective HHI threshold). The reasoning: a domain with lots of active verification is self-policing — it doesn't need the capture defense hammer as much.

```go
// In capture_defense's RunAutoAnalysis:
func (k Keeper) getEffectiveHHIThreshold(ctx context.Context, domain string) uint64 {
    params := k.GetParams(ctx)
    baseThreshold := params.HhiThreshold
    
    // Fire controls Metal: active verification relaxes defense sensitivity
    if k.knowledgeKeeper != nil {
        activity := k.knowledgeKeeper.GetDomainVerificationActivity(ctx, domain)
        // At full activity (BPS): threshold increases by 20% (harder to flag)
        // At zero activity: threshold stays at base (easy to flag)
        thresholdBonus := baseThreshold * activity * params.ActivityThresholdRelaxationBps / (BPS * BPS)
        return baseThreshold + thresholdBonus
    }
    
    return baseThreshold
}
```

**New param in capture_defense:**
```
ActivityThresholdRelaxationBps uint64 // default: 200_000 — max 20% HHI threshold relaxation from verification activity
```

### Controls → Wood (Structure prunes growth)

**Status:** ⚠️ Partial. R31-1 adds capture penalty to carrying capacity. Additionally, qualification gating already restricts who can submit claims to certain domains. But ontology stratum depth doesn't affect knowledge growth rates.

**New connection:** Higher stratum domains (theoretical, applied) should have naturally lower carrying capacity than foundational domains (empirical). Ontology provides the structural hierarchy; knowledge respects it.

```go
// In knowledge's GetDomainCarryingCapacity:
func (k Keeper) getStratumCapacityMultiplier(ctx context.Context, domain string) uint64 {
    if k.ontologyKeeper == nil {
        return BPS // 1x
    }
    
    depth := k.ontologyKeeper.GetDomainDepth(ctx, domain)
    // Deeper strata (higher depth number) → lower capacity
    // Empirical (depth 1): 100% capacity
    // Theoretical (depth 2): 80% capacity
    // Applied (depth 3): 60% capacity
    // Meta (depth 4+): 50% capacity
    switch {
    case depth <= 1:
        return BPS
    case depth == 2:
        return 800_000
    case depth == 3:
        return 600_000
    default:
        return 500_000
    }
}
```

This is Metal pruning Wood — structural hierarchy constrains growth. A domain of pure theory can't have as many facts as a domain of empirical observation, because theoretical facts need more evidence to justify their existence.

## Events

```
zerone.discovery.qualification_match_bonus {
    seeker: string
    candidate: string
    complementarity_bps: uint64
    bonus_bps: uint64
}

zerone.capture_defense.activity_threshold_relaxation {
    domain: string
    base_hhi_threshold: uint64
    effective_hhi_threshold: uint64
    verification_activity_bps: uint64
}

zerone.knowledge.stratum_capacity_applied {
    domain: string
    stratum_depth: uint64
    capacity_multiplier_bps: uint64
    effective_capacity: uint64
}
```

## New Keeper Dependencies

| Module | Gets Reference To | Purpose |
|--------|------------------|---------|
| discovery | qualification | Qualified domains for match scoring |
| partnerships | ontology | Related strata for cross-stratum matching bonus |
| capture_defense | knowledge (extended) | Domain verification activity (from R31-2) |
| knowledge | ontology (extended) | Domain depth for stratum capacity |

## Tests

1. **Metal → Water:** Discovery matches scored higher when partners have complementary qualifications.
2. **Metal → Water:** Cross-stratum mentorship gets 20% priority bonus.
3. **Fire → Metal:** Domain with high verification activity → effective HHI threshold increases (harder to flag).
4. **Fire → Metal:** Domain with zero activity → base HHI threshold (easy to flag).
5. **Metal → Wood:** Empirical domain (depth 1) → full carrying capacity.
6. **Metal → Wood:** Theoretical domain (depth 2) → 80% carrying capacity.
7. **Metal → Wood:** Combined: captured theoretical domain → reduced capacity from both stratum and capture penalty.

## What This Changes

Before R31-4: Defense structures exist in isolation. Qualification doesn't inform social matching. Ontology depth doesn't affect growth capacity. Verification activity doesn't modulate defense sensitivity.

After R31-4: Metal generates Water (qualification/ontology structure shapes social matching), Fire controls Metal (active verification relaxes defense), and Metal controls Wood (stratum hierarchy constrains growth). The defense layer becomes responsive to the system's actual dynamics instead of operating on fixed thresholds.
