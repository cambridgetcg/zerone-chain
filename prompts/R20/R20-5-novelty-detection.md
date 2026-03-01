# R20-5 — Novelty Detection: Redundancy Scoring Against Common Knowledge

## Context

"Water boils at 100°C" is true but useless — every LLM already knows it from training data. Submitting it to the Tree of Knowledge wastes verifier time and occupies a niche that adds zero marginal value to any agent.

Novelty detection scores how much **new information** a claim adds beyond what's already available in:
1. The existing knowledge base (on-chain facts)
2. Common LLM training data (the "everyone already knows this" problem)

Low-novelty facts get penalized in fitness scoring, making them more likely to be pruned. High-novelty facts get rewarded — they're contributing genuinely new knowledge.

## Design

### Novelty Scoring Approach

Novelty has two components:

**1. On-chain novelty** (computable deterministically):
- How similar is this claim to existing facts in the same domain/niche?
- Does this claim add information beyond what existing facts already cover?
- Measured via canonical form overlap, subject overlap, and content hash similarity

**2. Common knowledge penalty** (requires heuristic):
- Is this something any general-purpose LLM would know?
- Measured via a **common knowledge registry** — a curated list of "things everyone knows" against which new claims are checked
- The registry itself is a set of on-chain facts tagged as `COMMON_KNOWLEDGE`

### Common Knowledge Registry

At genesis, seed a set of canonical subjects that represent common knowledge baselines:

```
domain=mathematics: "basic arithmetic", "pythagorean theorem", "prime numbers definition"
domain=physics: "water boiling point", "speed of light", "gravity acceleration"
domain=economics: "supply and demand", "inflation definition"
...
```

Claims whose structured subject matches a common knowledge entry receive a **novelty penalty**. This doesn't prevent submission — it just reduces fitness, making the fact harder to sustain.

### Novelty Score Calculation

```
novelty(claim) = 
    1,000,000                               // Start at max novelty
    - common_knowledge_penalty(claim)        // Deduct if subject is common knowledge
    - subject_overlap_penalty(claim)         // Deduct if similar subject exists on-chain
    + precision_bonus(claim)                 // Add if claim is more precise than existing
    + cross_domain_bonus(claim)              // Add if claim bridges domains
```

## Task

### 1. Proto: Add Common Knowledge Entry

In `proto/zerone/knowledge/v1/types.proto`:

```protobuf
// CommonKnowledgeEntry represents a subject that LLMs already know.
// Claims matching these subjects receive a novelty penalty.
message CommonKnowledgeEntry {
    string id           = 1;
    string domain       = 2;
    string subject      = 3;   // Normalized subject string
    string description  = 4;   // Human-readable explanation
    uint64 penalty_bps  = 5;   // Novelty penalty (0-1,000,000)
    uint64 added_block  = 6;
}
```

### 2. Proto: Add Novelty Fields to Fact

In `types.proto`, add to `Fact`:

```protobuf
uint64 novelty_score          = 49;  // 0-1,000,000 (feeds into fitness)
bool   common_knowledge_match = 50;  // True if subject matches common knowledge registry
```

### 3. Proto: Add Novelty Params

In `genesis.proto`:

```protobuf
// ─── Novelty detection ──────────────────────────────────────────
uint64 novelty_common_knowledge_penalty_bps = <next>;  // Default penalty for common knowledge match
uint64 novelty_subject_overlap_penalty_bps  = <next>;  // Penalty per existing fact with same subject
uint64 novelty_precision_bonus_bps          = <next>;  // Bonus if more precise than existing
uint64 novelty_cross_domain_bonus_bps       = <next>;  // Bonus if subject spans multiple domains
uint64 novelty_max_overlap_facts            = <next>;  // Cap on overlap penalty (after N, no more penalty)
```

### 4. Genesis: Seed Common Knowledge Registry

In `x/knowledge/keeper/genesis.go`, `InitGenesis()`:

```go
// Seed common knowledge registry
for _, entry := range DefaultCommonKnowledgeEntries() {
    k.SetCommonKnowledgeEntry(ctx, entry)
}
```

Provide a curated default set covering ~50-100 entries across all 18 domains. Examples:

```go
func DefaultCommonKnowledgeEntries() []*types.CommonKnowledgeEntry {
    return []*types.CommonKnowledgeEntry{
        // Mathematics
        {Domain: "mathematics", Subject: "addition", Penalty: 800_000, Description: "Basic arithmetic operations"},
        {Domain: "mathematics", Subject: "pythagorean theorem", Penalty: 700_000},
        {Domain: "mathematics", Subject: "prime number definition", Penalty: 700_000},
        {Domain: "mathematics", Subject: "pi value", Penalty: 800_000},
        
        // Physics
        {Domain: "physics", Subject: "water boiling point", Penalty: 800_000},
        {Domain: "physics", Subject: "speed of light", Penalty: 700_000},
        {Domain: "physics", Subject: "gravity acceleration", Penalty: 700_000},
        {Domain: "physics", Subject: "newton laws of motion", Penalty: 700_000},
        {Domain: "physics", Subject: "conservation of energy", Penalty: 600_000},
        
        // Economics
        {Domain: "economics", Subject: "supply and demand", Penalty: 700_000},
        {Domain: "economics", Subject: "inflation definition", Penalty: 700_000},
        
        // Biology
        {Domain: "biology", Subject: "dna structure", Penalty: 600_000},
        {Domain: "biology", Subject: "evolution natural selection", Penalty: 600_000},
        
        // ... more entries across all domains
    }
}
```

### 5. Store: Common Knowledge Index

In `x/knowledge/types/keys.go`:

```go
CommonKnowledgePrefix = []byte{0x3A}  // 0x3A | domain | subject_hash → CommonKnowledgeEntry
```

### 6. Novelty Calculator

In `x/knowledge/keeper/novelty.go` (**NEW**):

```go
// CalculateNovelty scores how much new information a fact contributes.
func (k Keeper) CalculateNovelty(ctx context.Context, fact *types.Fact) uint64 {
    params, _ := k.GetParams(ctx)
    novelty := uint64(1_000_000)  // Start at max
    
    // ─── Common knowledge penalty ─────────────────────────
    if fact.Structure != nil && fact.Structure.Subject != "" {
        entry, found := k.FindCommonKnowledge(ctx, fact.Domain, fact.Structure.Subject)
        if found {
            fact.CommonKnowledgeMatch = true
            penalty := entry.PenaltyBps
            if penalty > novelty {
                novelty = 0
            } else {
                novelty -= penalty
            }
        }
    }
    
    // ─── Subject overlap penalty ──────────────────────────
    // How many existing facts share the same subject in the same domain?
    overlapCount := k.CountFactsBySubject(ctx, fact.Domain, fact.Structure.GetSubject())
    overlapCount-- // Don't count self
    if overlapCount > 0 {
        cappedOverlap := min(overlapCount, params.NoveltyMaxOverlapFacts)
        overlapPenalty := cappedOverlap * params.NoveltySubjectOverlapPenaltyBps
        if overlapPenalty > novelty {
            novelty = 0
        } else {
            novelty -= overlapPenalty
        }
    }
    
    // ─── Precision bonus ──────────────────────────────────
    // If this fact has a more specific scope than existing facts in same niche
    if fact.Structure != nil && fact.Structure.Scope != "" {
        nicheMembers := k.GetNicheMembers(ctx, fact.NicheKey)
        hasLessSpecific := false
        for _, m := range nicheMembers {
            if m.Id != fact.Id && (m.Structure == nil || m.Structure.Scope == "" || len(m.Structure.Scope) < len(fact.Structure.Scope)) {
                hasLessSpecific = true
                break
            }
        }
        if hasLessSpecific {
            novelty += params.NoveltyPrecisionBonusBps
        }
    }
    
    // ─── Cross-domain bonus ───────────────────────────────
    // If this fact has SUPPORTS relations to facts in different domains
    if fact.BridgeScore > 0 {
        novelty += params.NoveltyCrossDomainBonusBps
    }
    
    // Cap
    if novelty > 1_000_000 {
        novelty = 1_000_000
    }
    
    return novelty
}

// FindCommonKnowledge does fuzzy matching against the common knowledge registry.
// Uses normalized subject comparison: lowercase, trimmed, common synonyms resolved.
func (k Keeper) FindCommonKnowledge(ctx context.Context, domain, subject string) (*types.CommonKnowledgeEntry, bool) {
    normalized := normalizeSubject(subject)
    
    // Exact match first
    entry, found := k.GetCommonKnowledgeEntry(ctx, domain, normalized)
    if found {
        return entry, true
    }
    
    // Prefix/contains match (e.g., "water boiling point at altitude" matches "water boiling point")
    entries := k.GetCommonKnowledgeByDomain(ctx, domain)
    for _, e := range entries {
        entryNorm := normalizeSubject(e.Subject)
        if strings.Contains(normalized, entryNorm) || strings.Contains(entryNorm, normalized) {
            return e, true
        }
    }
    
    return nil, false
}
```

### 7. Integration with Fitness (R20-1)

In `x/knowledge/keeper/fitness.go`, `CalculateFitness()`:

The `uniqueness_score` component is now powered by the novelty calculator:

```go
// Replace the old uniqueScore with novelty
uniqueScore := k.CalculateNovelty(ctx, fact)
```

### 8. Governance: Manage Common Knowledge Registry

```protobuf
rpc AddCommonKnowledge(MsgAddCommonKnowledge) returns (MsgAddCommonKnowledgeResponse);
rpc RemoveCommonKnowledge(MsgRemoveCommonKnowledge) returns (MsgRemoveCommonKnowledgeResponse);

message MsgAddCommonKnowledge {
    option (cosmos.msg.v1.signer) = "authority";
    string authority   = 1;
    string domain      = 2;
    string subject     = 3;
    string description = 4;
    uint64 penalty_bps = 5;
}
```

Authority-only (governance) to prevent gaming — you shouldn't be able to add your competitor's subject as "common knowledge" to tank their novelty.

### 9. Query

```protobuf
rpc CommonKnowledge(QueryCommonKnowledgeRequest) returns (QueryCommonKnowledgeResponse) {
    option (google.api.http).get = "/zerone/knowledge/v1/common_knowledge";
}

rpc CheckNovelty(QueryCheckNoveltyRequest) returns (QueryCheckNoveltyResponse) {
    option (google.api.http).get = "/zerone/knowledge/v1/check_novelty";
}

message QueryCheckNoveltyRequest {
    string domain  = 1;
    string subject = 2;
    string content = 3;
}

message QueryCheckNoveltyResponse {
    uint64 novelty_score          = 1;
    bool   common_knowledge_match = 2;
    string matched_entry          = 3;
    uint64 subject_overlap_count  = 4;
}
```

The `CheckNovelty` endpoint lets submitters check novelty **before** paying the review fee — avoid wasting money on common knowledge.

### 10. CLI

```
zeroned query knowledge check-novelty [domain] [subject] [content]
zeroned query knowledge common-knowledge [--domain physics]
```

### 11. Tests

1. **TestNovelty_CommonKnowledgeMatch** — "water boiling point" matches registry, gets penalized
2. **TestNovelty_NoMatch** — novel subject gets full score
3. **TestNovelty_FuzzyMatch** — "water boiling point at altitude" matches "water boiling point"
4. **TestNovelty_SubjectOverlap** — multiple facts with same subject reduces novelty
5. **TestNovelty_PrecisionBonus** — more specific scope than existing = bonus
6. **TestNovelty_CrossDomainBonus** — bridge facts get novelty bonus
7. **TestNovelty_Cap** — score capped at 1,000,000
8. **TestCheckNovelty_PreSubmission** — query endpoint returns correct preview
9. **TestCommonKnowledge_GovernanceAdd** — authority can add entries
10. **TestCommonKnowledge_GovernanceRemove** — authority can remove entries

## Design Notes

- **Common knowledge registry is curated, not computed.** Computing "what LLMs know" is impossible from on-chain. A curated list is honest about its limitations. Start small (~100 entries), grow via governance.
- **Fuzzy matching is intentionally simple.** Substring containment, not semantic similarity. Keeps it deterministic and auditable. Sophisticated matching is a future improvement.
- **The penalty doesn't block submission.** You CAN submit "water boils at 100°C." It'll just have low novelty → low fitness → high mortality. The market decides, not a gatekeeper.
- **Precision bonus rewards improvement.** "Water boils at 99.97°C at 101.325 kPa" scores higher than "water boils at 100°C" because it's more specific. This is how knowledge evolves — vague facts get displaced by precise ones.
- **CheckNovelty is free.** No fee, no tx. Just a query. This is a public good — helps submitters avoid wasting review fees on low-novelty claims.

## Dependencies

- R19-4 (structured fields) — subject/scope needed for novelty scoring
- R20-1 (fitness score) — novelty feeds into fitness via uniqueness component
- R20-3 (natural selection) — niche members used for precision bonus

## Files Modified

- `proto/zerone/knowledge/v1/types.proto` — CommonKnowledgeEntry, novelty fields on Fact
- `proto/zerone/knowledge/v1/genesis.proto` — novelty params
- `proto/zerone/knowledge/v1/tx.proto` — AddCommonKnowledge, RemoveCommonKnowledge
- `proto/zerone/knowledge/v1/query.proto` — CommonKnowledge, CheckNovelty queries
- `x/knowledge/types/*.pb.go` — regenerated
- `x/knowledge/types/keys.go` — common knowledge prefix
- `x/knowledge/types/genesis.go` — defaults + common knowledge seed
- `x/knowledge/keeper/novelty.go` — **NEW**: novelty calculator + common knowledge matching
- `x/knowledge/keeper/fitness.go` — wire novelty into uniqueness component
- `x/knowledge/keeper/genesis.go` — seed common knowledge entries
- `x/knowledge/keeper/msg_server.go` — AddCommonKnowledge handler
- `x/knowledge/keeper/grpc_query.go` — CheckNovelty, CommonKnowledge handlers
- Tests: 10 new tests

## Commit

Single commit: `feat(knowledge): add novelty detection with common knowledge registry`
