# R19-3 — Semantic Anchors (Fact Relationship Graph)

## Context

Facts currently have a `references` field (list of fact IDs), but it's untyped — you know fact A references fact B, but not *how*. Does A support B? Contradict it? Depend on it? Refine it?

Semantic anchors turn the flat fact list into a typed knowledge graph. Agents can traverse relationships:
- "Give me everything that supports this claim" → follow `supports` edges
- "Are there contradictions?" → follow `contradicts` edges
- "What prerequisites does this fact assume?" → follow `requires` edges

This is the difference between a bag of facts and a **knowledge graph**.

## Task

### 1. Proto: Add FactRelation Message and Enum

In `proto/zerone/knowledge/v1/types.proto`, add:

```protobuf
// RelationType defines how one fact relates to another.
enum RelationType {
  RELATION_TYPE_UNSPECIFIED  = 0;
  RELATION_TYPE_SUPPORTS     = 1;  // This fact provides evidence for the target
  RELATION_TYPE_CONTRADICTS  = 2;  // This fact conflicts with the target
  RELATION_TYPE_REQUIRES     = 3;  // This fact depends on the target being true
  RELATION_TYPE_REFINES      = 4;  // This fact is a more precise version of the target
  RELATION_TYPE_GENERALIZES  = 5;  // This fact is a broader version of the target
  RELATION_TYPE_SUPERSEDES   = 6;  // This fact replaces the target (newer/better)
}

// FactRelation is a typed, directional edge in the knowledge graph.
message FactRelation {
  string source_fact_id   = 1;  // The fact declaring the relationship
  string target_fact_id   = 2;  // The fact being referenced
  RelationType relation   = 3;  // How source relates to target
  uint64 created_at_block = 4;
  string creator          = 5;  // Address that declared this relation
}
```

### 2. Proto: Add Relations to Fact

In `types.proto`, add to `Fact` (after field 24):

```protobuf
repeated FactRelation outgoing_relations = 25;  // Relations this fact declares
repeated FactRelation incoming_relations = 26;  // Relations pointing to this fact
```

### 3. Proto: Add Relations to MsgSubmitClaim

In `tx.proto`, add to `MsgSubmitClaim`:

```protobuf
// Typed relationships to existing facts.
// Replaces untyped `references` for new claims (references kept for backward compat).
repeated ClaimRelation relations = 9;
```

Add the `ClaimRelation` message in `tx.proto`:

```protobuf
message ClaimRelation {
  string target_fact_id = 1;
  RelationType relation = 2;
}
```

### 4. Store: Add Relation Key Prefix and Storage

In `x/knowledge/types/keys.go`, add:

```go
FactRelationPrefix         = []byte{0x30}  // 0x30 | source_fact_id | target_fact_id → FactRelation
FactRelationReversePrefix  = []byte{0x31}  // 0x31 | target_fact_id | source_fact_id → FactRelation (reverse index)
```

In `x/knowledge/keeper/state.go`, add:

```go
func (k Keeper) SetFactRelation(ctx context.Context, rel *types.FactRelation) error
func (k Keeper) GetFactRelations(ctx context.Context, factID string) ([]*types.FactRelation, error)        // outgoing
func (k Keeper) GetIncomingRelations(ctx context.Context, factID string) ([]*types.FactRelation, error)     // incoming
func (k Keeper) GetRelationsByType(ctx context.Context, factID string, relType types.RelationType) ([]*types.FactRelation, error)
```

Dual-write: every relation is stored under both forward and reverse prefix for bidirectional lookup.

### 5. Claim Submission: Validate and Store Relations

In `x/knowledge/keeper/msg_server.go`, `SubmitClaim()`:

```go
// Validate relations — target facts must exist
for _, rel := range msg.Relations {
    if _, found := m.keeper.GetFact(ctx, rel.TargetFactId); !found {
        return nil, fmt.Errorf("relation target fact %s not found", rel.TargetFactId)
    }
    if rel.Relation == types.RelationType_RELATION_TYPE_UNSPECIFIED {
        return nil, fmt.Errorf("relation type must be specified")
    }
}
```

Store relations on the Claim. On acceptance (in `createFactFromClaim`), convert `ClaimRelation` to `FactRelation` and store:

```go
for _, claimRel := range claim.Relations {
    factRel := &types.FactRelation{
        SourceFactId:  fact.Id,
        TargetFactId:  claimRel.TargetFactId,
        Relation:      claimRel.Relation,
        CreatedAtBlock: height,
        Creator:       claim.Submitter,
    }
    k.SetFactRelation(ctx, factRel)
}
```

### 6. Contradiction Detection

In `SubmitClaim()`, if any relation is `RELATION_TYPE_CONTRADICTS`:

```go
for _, rel := range msg.Relations {
    if rel.Relation == types.RelationType_RELATION_TYPE_CONTRADICTS {
        // Auto-mark the target fact as contested
        targetFact, _ := m.keeper.GetFact(ctx, rel.TargetFactId)
        if targetFact.Status == types.FactStatus_FACT_STATUS_VERIFIED ||
           targetFact.Status == types.FactStatus_FACT_STATUS_ACTIVE {
            targetFact.Status = types.FactStatus_FACT_STATUS_CONTESTED
            m.keeper.SetFact(ctx, targetFact)
        }
    }
}
```

This replaces the current `SubmitContradiction` handler with a more general mechanism — any claim can declare a contradiction as part of its relations.

### 7. Query: Add Graph Traversal Endpoints

In `proto/zerone/knowledge/v1/query.proto`, add:

```protobuf
rpc FactRelations(QueryFactRelationsRequest) returns (QueryFactRelationsResponse) {
    option (google.api.http).get = "/zerone/knowledge/v1/fact/{fact_id}/relations";
}

message QueryFactRelationsRequest {
  string fact_id           = 1;
  RelationType relation    = 2;  // Optional: filter by type
  string direction         = 3;  // "outgoing", "incoming", or "both" (default: "both")
}

message QueryFactRelationsResponse {
  repeated FactRelation relations = 1;
}
```

In `x/knowledge/keeper/grpc_query.go`, implement the handler.

### 8. CLI

Add `--relations` flag to `submit-claim`:

```
--relations string    Typed relations: "supports:FACT_ID,contradicts:FACT_ID,requires:FACT_ID"
```

Parse as comma-separated `type:factID` pairs.

Add `fact-relations` query command:

```
zeroned query knowledge fact-relations [fact-id] [--type supports] [--direction outgoing]
```

### 9. Context Server: Graph-Aware Context

In `tools/knowledge-context/main.go`, add a `/graph` endpoint:

```
GET /graph?fact_id=abc123&depth=2&relation=supports,requires
```

Returns the fact plus its relationship subgraph up to N depth. This lets an agent pull a fact *and all its dependencies* in one call.

Update `/context` to include relations in output:

```xml
<fact id="abc" domain="physics" confidence="95%" status="verified">
  Entropy cannot decrease in a closed system
  <supports>def456</supports>
  <requires>ghi789</requires>
</fact>
```

JSON format:

```json
{
  "id": "abc",
  "content": "...",
  "relations": {
    "supports": ["def456"],
    "requires": ["ghi789"],
    "contradicts": []
  }
}
```

### 10. Events

Emit relation events on fact creation:

```go
sdk.NewEvent("zerone.knowledge.fact_relation_created",
    sdk.NewAttribute("source", factRel.SourceFactId),
    sdk.NewAttribute("target", factRel.TargetFactId),
    sdk.NewAttribute("relation", factRel.Relation.String()),
)
```

### 11. Tests

1. **TestSubmitClaim_WithRelations** — claim with supports/requires relations stores correctly
2. **TestCreateFactFromClaim_PropagatesRelations** — accepted claim → fact has correct FactRelations
3. **TestSubmitClaim_ContradictionAutoContests** — contradiction relation auto-sets target fact to CONTESTED
4. **TestSubmitClaim_InvalidRelationTarget** — relation to non-existent fact rejected
5. **TestSubmitClaim_UnspecifiedRelationType** — unspecified relation type rejected
6. **TestGetFactRelations_Bidirectional** — outgoing from source and incoming to target both work
7. **TestGetRelationsByType** — type filter returns only matching relations
8. **TestQueryFactRelations_REST** — gRPC-gateway endpoint returns correct data
9. **TestGraphTraversal_TwoHop** — fact A requires B, B requires C → depth-2 from A returns all three

## Design Notes

- **Dual indexing** (forward + reverse prefix) is essential — agents need both "what does this fact support?" and "what supports this fact?"
- **Relations are immutable** — once a fact declares a relationship, it's permanent. If the relationship is wrong, the fact itself needs to be challenged.
- **`references` field kept for backward compat** — existing claims with untyped references continue to work. New claims should use `relations` instead. Migration: treat existing `references` as `RELATION_TYPE_SUPPORTS`.
- **`SUPERSEDES` triggers status change** — when fact B supersedes fact A, A should transition to `FACT_STATUS_SUPERSEDED`. This is the clean upgrade path for evolving knowledge.
- **Depth-limited graph queries** — the `/graph` endpoint MUST have a max depth (suggest 5) to prevent unbounded traversal. Knowledge graphs can be deep.

## Files Modified

- `proto/zerone/knowledge/v1/types.proto` — RelationType enum, FactRelation message, fields on Fact
- `proto/zerone/knowledge/v1/tx.proto` — ClaimRelation message, field on MsgSubmitClaim
- `proto/zerone/knowledge/v1/query.proto` — FactRelations query
- `x/knowledge/types/*.pb.go` — regenerated
- `x/knowledge/types/keys.go` — relation key prefixes
- `x/knowledge/keeper/state.go` — relation CRUD + bidirectional index
- `x/knowledge/keeper/msg_server.go` — relation validation, contradiction detection, propagation
- `x/knowledge/keeper/rounds.go` — propagate relations in createFactFromClaim
- `x/knowledge/keeper/grpc_query.go` — FactRelations query handler
- `tools/knowledge-context/main.go` — graph endpoint + relation output
- CLI tx + query commands

## Commit

Single commit: `feat(knowledge): add typed semantic relations (knowledge graph edges)`
