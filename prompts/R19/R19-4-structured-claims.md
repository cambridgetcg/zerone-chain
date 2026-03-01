# R19-4 — Structured Claim Fields (Subject/Predicate/Scope)

## Context

Claims are currently opaque strings. Two claims about the same subject use completely different phrasing, making dedup, search, and agent retrieval unreliable. Structured fields let agents do exact lookups: *"What does the knowledge base say about `entropy`?"* becomes a field query, not a semantic search.

This builds on R19-2 (claim types) and R19-3 (semantic anchors). Together they transform claims from prose → structured, typed, linked knowledge.

## Task

### 1. Proto: Add ClaimStructure Message

In `proto/zerone/knowledge/v1/types.proto`, add:

```protobuf
// ClaimStructure provides machine-readable decomposition of a claim.
// The full claim text (fact_content) remains the canonical human-readable form.
// Structure is optional but strongly encouraged — agents prioritize structured facts.
message ClaimStructure {
  string subject           = 1;  // What the claim is about: "entropy of a closed system"
  string predicate         = 2;  // What is being asserted: "cannot decrease spontaneously"
  string object            = 3;  // Optional target: "second law of thermodynamics"
  string scope             = 4;  // Conditions/context: "classical thermodynamics, isolated systems"
  string temporal_scope    = 5;  // Time bounds if any: "post-Big-Bang", "since 2024", "" for timeless
  bool   negatable         = 6;  // Can this claim be meaningfully negated? (false for definitions)
  repeated string tags     = 7;  // Free-form searchable tags: ["thermodynamics", "entropy", "physics"]
}
```

### 2. Proto: Add Structure to Fact and Claim

In `types.proto`, add to `Fact`:

```protobuf
ClaimStructure structure = 27;  // Machine-readable decomposition (optional)
```

In `types.proto`, add to `Claim`:

```protobuf
ClaimStructure structure = 16;  // Machine-readable decomposition (optional)
```

In `tx.proto`, add to `MsgSubmitClaim`:

```protobuf
ClaimStructure structure = 10;  // Optional structured decomposition
```

Regenerate Go proto files.

### 3. Submission Handling

In `x/knowledge/keeper/msg_server.go`, `SubmitClaim()`:

```go
// Validate structure if provided
if msg.Structure != nil {
    if msg.Structure.Subject == "" {
        return nil, fmt.Errorf("claim structure: subject is required when structure is provided")
    }
    if msg.Structure.Predicate == "" {
        return nil, fmt.Errorf("claim structure: predicate is required when structure is provided")
    }
    if len(msg.Structure.Tags) > 10 {
        return nil, fmt.Errorf("claim structure: max 10 tags allowed")
    }
    for _, tag := range msg.Structure.Tags {
        if len(tag) > 50 {
            return nil, fmt.Errorf("claim structure: tag too long (max 50 chars): %s", tag)
        }
    }
}
```

Propagate structure to Claim, then to Fact on acceptance.

### 4. Subject-Based Dedup Enhancement

In `SubmitClaim()`, add secondary dedup check when structure is present:

```go
// If structured, also check for existing facts with same subject+predicate in same domain
if msg.Structure != nil && msg.Structure.Subject != "" {
    if existingFactID := m.keeper.FindFactBySubjectPredicate(ctx, msg.Domain, msg.Structure.Subject, msg.Structure.Predicate); existingFactID != "" {
        // Don't reject — but emit a warning event. The claim may refine/supersede the existing fact.
        sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
            "zerone.knowledge.duplicate_subject_warning",
            sdk.NewAttribute("existing_fact_id", existingFactID),
            sdk.NewAttribute("subject", msg.Structure.Subject),
        ))
    }
}
```

### 5. Store: Subject Index

In `x/knowledge/types/keys.go`, add:

```go
FactSubjectPrefix = []byte{0x32}  // 0x32 | domain | subject_hash → fact_id
FactTagPrefix     = []byte{0x33}  // 0x33 | tag | fact_id → []byte{1}
```

In `x/knowledge/keeper/state.go`, add:

```go
func (k Keeper) IndexFactBySubject(ctx context.Context, fact *types.Fact) error
func (k Keeper) FindFactBySubjectPredicate(ctx context.Context, domain, subject, predicate string) string
func (k Keeper) FindFactsByTag(ctx context.Context, tag string) ([]string, error)  // returns fact IDs
```

### 6. Query: Add Subject and Tag Search

In `proto/zerone/knowledge/v1/query.proto`, add:

```protobuf
rpc FactsBySubject(QueryFactsBySubjectRequest) returns (QueryFactsBySubjectResponse) {
    option (google.api.http).get = "/zerone/knowledge/v1/facts_by_subject/{domain}/{subject}";
}

rpc FactsByTag(QueryFactsByTagRequest) returns (QueryFactsByTagResponse) {
    option (google.api.http).get = "/zerone/knowledge/v1/facts_by_tag/{tag}";
}

message QueryFactsBySubjectRequest {
  string domain  = 1;
  string subject = 2;
}

message QueryFactsBySubjectResponse {
  repeated Fact facts = 1;
}

message QueryFactsByTagRequest {
  string tag = 1;
}

message QueryFactsByTagResponse {
  repeated Fact facts = 1;
}
```

Implement handlers in `grpc_query.go`.

### 7. CLI

Update `submit-claim` CLI:

```
--subject string         Claim subject (structured)
--predicate string       Claim predicate (structured)
--object string          Claim object (structured, optional)
--scope string           Claim scope/conditions (structured, optional)
--temporal-scope string  Time bounds (structured, optional)
--negatable              Mark claim as negatable (default true)
--tags string            Comma-separated tags
```

Add query commands:

```
zeroned query knowledge facts-by-subject [domain] [subject]
zeroned query knowledge facts-by-tag [tag]
```

### 8. Context Server: Structured Output

In `tools/knowledge-context/main.go`, add structure to output:

XML:
```xml
<fact id="abc" domain="physics" confidence="95%" type="assertion">
  <content>Entropy cannot decrease in a closed system</content>
  <structure>
    <subject>entropy of a closed system</subject>
    <predicate>cannot decrease spontaneously</predicate>
    <scope>classical thermodynamics</scope>
    <tags>entropy,thermodynamics,second-law</tags>
  </structure>
</fact>
```

Add `/context` query params:
- `subject=entropy` — filter facts by subject match
- `tags=thermodynamics,physics` — filter facts by tags (OR match)

### 9. Tests

1. **TestSubmitClaim_WithStructure** — structure fields stored on claim
2. **TestSubmitClaim_StructureValidation** — missing subject rejected, too many tags rejected
3. **TestCreateFactFromClaim_PropagatesStructure** — structure propagated to fact
4. **TestSubjectIndex_StoreAndRetrieve** — indexed by subject, retrievable
5. **TestFindFactsByTag** — tag index returns correct facts
6. **TestSubjectDedup_EmitsWarning** — duplicate subject emits event but doesn't reject
7. **TestQueryFactsBySubject_REST** — gRPC-gateway endpoint works
8. **TestQueryFactsByTag_REST** — tag query returns results

## Design Notes

- **Structure is optional** — backward compat with existing unstructured claims. But the context server should deprioritize unstructured facts in agent output (structured = higher signal).
- **Subject normalization** — subjects are stored lowercase, trimmed. "Entropy" and "entropy" and " entropy " all resolve to the same key. Do NOT attempt semantic normalization (e.g., "entropy" vs "disorder") — that's a higher-layer problem.
- **Tags are free-form but bounded** — max 10 tags, max 50 chars each. Enough for useful indexing without abuse.
- **`negatable` flag** — definitions aren't meaningfully negatable ("the number 7 is NOT defined as..."). This hints to agents about which facts can be challenged vs which are definitional.
- **`temporal_scope`** — critical for observations and empirical claims. "BTC price is $50k" needs a time bound. Timeless claims leave this empty.

## Files Modified

- `proto/zerone/knowledge/v1/types.proto` — ClaimStructure message, fields on Fact + Claim
- `proto/zerone/knowledge/v1/tx.proto` — field on MsgSubmitClaim
- `proto/zerone/knowledge/v1/query.proto` — FactsBySubject, FactsByTag queries
- `x/knowledge/types/*.pb.go` — regenerated
- `x/knowledge/types/keys.go` — subject + tag index prefixes
- `x/knowledge/keeper/state.go` — subject/tag indexing + lookup
- `x/knowledge/keeper/msg_server.go` — structure validation, dedup warning, propagation
- `x/knowledge/keeper/rounds.go` — propagate structure in createFactFromClaim
- `x/knowledge/keeper/grpc_query.go` — FactsBySubject, FactsByTag handlers
- `tools/knowledge-context/main.go` — structured output + subject/tag query params
- CLI tx + query commands

## Commit

Single commit: `feat(knowledge): add structured claim fields (subject/predicate/scope/tags)`
