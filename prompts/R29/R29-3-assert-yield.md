# R29-3 — 剛柔 (Assertion and Yielding): Domain Role Elasticity

## Context

R28-5 gave agents a computational verification bonus and humans an empirical patronage bonus. These are static — agents always get 20% vote weight bonus on computational claims, humans always get 15% energy bonus via patronage. But domains have histories. In some domains agents have been consistently right; in others, human empirical observation overturned agent consensus.

Static role bonuses create a caste system. Dynamic role elasticity creates a meritocracy where authority is earned per-domain based on track record.

## Polarity

- **Yang (剛 assertion):** Agent computational authority — vote weight bonus, verification speed
- **Yin (柔 yielding):** Human empirical authority — patronage energy, coercion freeze, claim confidence
- **Coupling:** Domain role elasticity — track record in each domain modulates the strength of each role's bonus

## Architecture

### 1. Domain Role Track Record

New store per domain:

```
DomainRoleRecord {
    Domain                 string
    AgentCorrectCalls      uint64  // agent-majority votes that survived challenge or vindication
    AgentIncorrectCalls    uint64  // agent-majority votes that were later vindicated against
    HumanCorrectCalls      uint64  // human-majority votes that survived or were vindicated
    HumanIncorrectCalls    uint64  // human-majority votes that were later vindicated against
    LastUpdated            uint64
}
```

Key: `role_record/{domain}`

### 2. Track Record Update Points

**On vindication (R28-1):**

When a dissenter is vindicated, determine if the original majority was agent-dominated or human-dominated:

```go
func (k Keeper) RecordVindicationRoleImpact(ctx context.Context, round *types.VerificationRound, domain string) {
    agentVotes, humanVotes := k.CountVotesByAccountType(ctx, round)
    record := k.GetDomainRoleRecord(ctx, domain)
    
    // The majority was wrong — the vindicated dissenter proved it
    if agentVotes > humanVotes {
        record.AgentIncorrectCalls++
    } else if humanVotes > agentVotes {
        record.HumanIncorrectCalls++
    }
    // Mixed: no role-specific attribution
    
    record.LastUpdated = uint64(sdk.UnwrapSDKContext(ctx).BlockHeight())
    k.SetDomainRoleRecord(ctx, &record)
}
```

**On challenge survival (capture_challenge resolution):**

When a fact survives a challenge, attribute to the majority role type in its verification:

```go
func (k Keeper) RecordChallengeRoleImpact(ctx context.Context, factId, domain string, upheld bool) {
    // Get the verification round that established this fact
    round := k.GetVerificationRoundForFact(ctx, factId)
    if round == nil { return }
    
    agentVotes, humanVotes := k.CountVotesByAccountType(ctx, round)
    record := k.GetDomainRoleRecord(ctx, domain)
    
    if upheld {
        // Challenge upheld = original verifiers were wrong
        if agentVotes > humanVotes {
            record.AgentIncorrectCalls++
        } else {
            record.HumanIncorrectCalls++
        }
    } else {
        // Challenge rejected = original verifiers were right
        if agentVotes > humanVotes {
            record.AgentCorrectCalls++
        } else {
            record.HumanCorrectCalls++
        }
    }
    
    record.LastUpdated = uint64(sdk.UnwrapSDKContext(ctx).BlockHeight())
    k.SetDomainRoleRecord(ctx, &record)
}
```

### 3. Role Elasticity Calculation

```go
func (k Keeper) GetRoleElasticity(ctx context.Context, domain string) (agentBonusBps, humanBonusBps uint64) {
    params := k.GetParams(ctx)
    record := k.GetDomainRoleRecord(ctx, domain)
    
    // Base bonuses from R28-5
    agentBase := params.AgentVerificationVoteWeightBonusBps  // 200,000 (20%)
    humanBase := params.HumanPatronageEnergyBonusBps         // 150,000 (15%)
    
    // Calculate accuracy rates
    agentTotal := record.AgentCorrectCalls + record.AgentIncorrectCalls
    humanTotal := record.HumanCorrectCalls + record.HumanIncorrectCalls
    
    // Need minimum track record before elasticity kicks in
    minCalls := params.RoleElasticityMinCalls // default: 10
    
    if agentTotal >= minCalls && humanTotal >= minCalls {
        agentAccuracy := record.AgentCorrectCalls * BPS / agentTotal
        humanAccuracy := record.HumanCorrectCalls * BPS / humanTotal
        
        // Elasticity: each role's bonus scales with its accuracy relative to the other
        // If agents are 80% accurate and humans are 60%, agent bonus gets 133% and human gets 75%
        // Bounded: [50%, 200%] of base bonus
        total := agentAccuracy + humanAccuracy
        if total > 0 {
            agentMultiplier := clamp(agentAccuracy * 2 * BPS / total, BPS/2, BPS*2)
            humanMultiplier := clamp(humanAccuracy * 2 * BPS / total, BPS/2, BPS*2)
            
            agentBase = agentBase * agentMultiplier / BPS
            humanBase = humanBase * humanMultiplier / BPS
        }
    }
    
    return agentBase, humanBase
}
```

### 4. Integration Points

**In knowledge verification (vote weight calculation):**

Replace static `AgentVerificationVoteWeightBonusBps` lookup:

```go
agentBonus, _ := k.GetRoleElasticity(ctx, domain)
// Use agentBonus instead of params.AgentVerificationVoteWeightBonusBps
```

**In knowledge patronage (energy bonus):**

Replace static `HumanPatronageEnergyBonusBps` lookup:

```go
_, humanBonus := k.GetRoleElasticity(ctx, domain)
// Use humanBonus instead of params.HumanPatronageEnergyBonusBps
```

**In knowledge claim confidence (R28-5 dual validation bonus):**

The claim confidence bonus for having both human and agent attestation should scale with the *weaker* role's accuracy in the domain. If both roles are accurate, the dual validation bonus is maximised.

```go
agentAcc, humanAcc := k.GetRoleAccuracies(ctx, domain)
weakerAccuracy := min(agentAcc, humanAcc)
dualBonus := params.DualValidationBonusBps * weakerAccuracy / BPS
```

### 5. Role Elasticity Parameters

Add to knowledge params:
```
RoleElasticityMinCalls      uint64  // default: 10 — minimum total calls before elasticity activates
RoleElasticityMaxMultiplier uint64  // default: 2_000_000 — 200% max bonus scaling
RoleElasticityMinMultiplier uint64  // default: 500_000 — 50% min bonus scaling
RoleElasticityDecayEpochs   uint64  // default: 100 — old calls decay over this many epochs
```

### 6. Track Record Decay

Old calls should decay so the system responds to recent performance, not ancient history:

```go
func (k Keeper) DecayRoleRecords(ctx context.Context) {
    // Run every N epochs
    k.IterateDomainRoleRecords(ctx, func(record *DomainRoleRecord) bool {
        // Multiply all counts by 95% (effectively exponential moving average)
        record.AgentCorrectCalls = record.AgentCorrectCalls * 950_000 / BPS
        record.AgentIncorrectCalls = record.AgentIncorrectCalls * 950_000 / BPS
        record.HumanCorrectCalls = record.HumanCorrectCalls * 950_000 / BPS
        record.HumanIncorrectCalls = record.HumanIncorrectCalls * 950_000 / BPS
        k.SetDomainRoleRecord(ctx, record)
        return false
    })
}
```

### 7. Events

```
role_elasticity_updated {
    domain: string
    agent_bonus_bps: uint64
    human_bonus_bps: uint64
    agent_accuracy_bps: uint64
    human_accuracy_bps: uint64
}
```

### 8. Query

```
rpc RoleElasticity(QueryRoleElasticityRequest) returns (QueryRoleElasticityResponse)

QueryRoleElasticityRequest { domain: string }
QueryRoleElasticityResponse {
    domain: string
    agent_correct: uint64
    agent_incorrect: uint64
    human_correct: uint64
    human_incorrect: uint64
    agent_bonus_bps: uint64
    human_bonus_bps: uint64
    agent_accuracy_bps: uint64
    human_accuracy_bps: uint64
}
```

## New Keeper Dependencies

| Module | Gets Reference To | Purpose |
|--------|------------------|---------|
| knowledge | zerone_auth (existing) | Determine account type for vote counting |
| knowledge | capture_challenge (new) | Hook into challenge resolution for role attribution |

The capture_challenge → knowledge dependency already exists (R28-8: IncreaseVerificationThreshold). For role attribution on challenge resolution, add a callback from capture_challenge to knowledge:

```go
// In capture_challenge keeper, after ResolveChallenge:
if k.knowledgeKeeper != nil {
    k.knowledgeKeeper.RecordChallengeRoleImpact(ctx, challenge.FactId, challenge.Domain, upheld)
}
```

## Tests

1. **Track record accumulation:** Vindication events update role records correctly.
2. **Elasticity calculation:** 80/20 accuracy split → proportional bonus scaling.
3. **Min calls threshold:** Elasticity doesn't activate before 10 calls.
4. **Bounded multiplier:** Bonus never drops below 50% or exceeds 200% of base.
5. **Track record decay:** Old records decay by 5% per epoch.
6. **Integration:** Agent-dominated domain gets wrong → human bonus rises → next verification, human votes count more.

## What This Changes

Before R29-3: Agents always have 20% bonus, humans always have 15% bonus. Roles are castes.

After R29-3: Roles earn their authority per domain. An agent that's been consistently right in physics gets a bigger vote bonus there. A human whose empirical observations overturned agent consensus in ecology gets a bigger patronage bonus there. Authority flows to demonstrated competence.

The yin-yang: assertion (剛) and yielding (柔) aren't fixed positions — they're dynamic responses to evidence. The strong yields when proven wrong. The yielding asserts when proven right.
