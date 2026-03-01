# R19-2 — Add Claim Type Tags

## Context

Claims are currently untyped free-text. An agent querying the Tree of Knowledge gets back a flat list of strings with no structural hint about *what kind* of knowledge each claim represents. This forces semantic parsing on every query — slow, unreliable, and lossy.

Adding a `ClaimType` enum lets agents filter and prioritize by knowledge shape:
- Grab `DEFINITION` facts first (shared vocabulary)
- Then `ASSERTION` facts (grounding)
- Then `CONSTRAINT` facts (guardrails)
- Ignore `RELATION` facts unless doing graph traversal

## Task

### 1. Proto: Add ClaimType Enum

In `proto/zerone/knowledge/v1/types.proto`, add after the `Verdict` enum:

```protobuf
// ClaimType classifies the epistemic shape of a knowledge claim.
// Agents use this to filter and prioritize facts for prompt injection.
enum ClaimType {
  CLAIM_TYPE_UNSPECIFIED = 0;  // Legacy/untyped — treated as assertion
  CLAIM_TYPE_ASSERTION   = 1;  // "X is true" — direct factual statement
  CLAIM_TYPE_RELATION    = 2;  // "X relates to Y via Z" — graph edge
  CLAIM_TYPE_DEFINITION  = 3;  // "X means Y" — term/concept definition
  CLAIM_TYPE_CONSTRAINT  = 4;  // "X must/cannot Y" — rule or boundary
  CLAIM_TYPE_NEGATION    = 5;  // "X is NOT true" — explicit falsity marker
  CLAIM_TYPE_OBSERVATION = 6;  // "X was observed at time/place" — empirical data point
}
```

### 2. Proto: Add claim_type to Fact and Claim Messages

In `types.proto`, add to `Fact` (after field 23):

```protobuf
ClaimType claim_type = 24;
```

In `types.proto`, add to `Claim` (after field 14):

```protobuf
ClaimType claim_type = 15;
```

In `tx.proto`, add to `MsgSubmitClaim` (after field 7):

```protobuf
ClaimType claim_type = 8;  // Optional — defaults to ASSERTION if unset
```

Regenerate Go proto files.

### 3. Default Handling

In `x/knowledge/keeper/msg_server.go`, `SubmitClaim()`:

```go
// Default unspecified to assertion (backward compat)
claimType := msg.ClaimType
if claimType == types.ClaimType_CLAIM_TYPE_UNSPECIFIED {
    claimType = types.ClaimType_CLAIM_TYPE_ASSERTION
}
```

Store `claimType` on both the `Claim` and propagate to `Fact` in `createFactFromClaim()`.

### 4. Query Filtering

In `proto/zerone/knowledge/v1/query.proto`, add to `QueryFactsRequest`:

```protobuf
ClaimType claim_type = 4;  // Optional filter by type
```

In `x/knowledge/keeper/grpc_query.go`, `Facts()`, add filtering:

```go
if req.ClaimType != types.ClaimType_CLAIM_TYPE_UNSPECIFIED {
    if fact.ClaimType != req.ClaimType {
        continue // skip non-matching types
    }
}
```

### 5. CLI

Update `submit-claim` CLI to accept `--claim-type` flag:

```
--claim-type string    Claim type: assertion (default), relation, definition, constraint, negation, observation
```

Map string input to enum in the CLI handler.

Update `facts` query CLI to accept `--claim-type` filter flag.

### 6. Context Server Update

In `tools/knowledge-context/main.go`, add `claim_type` to the `Fact` struct and include it in all output formats:

- XML: `<fact ... type="assertion">`
- JSON: `"claim_type": "assertion"`
- Add `&type=assertion,definition` query parameter to `/context` endpoint

### 7. Events

Add `claim_type` attribute to `zerone.knowledge.submit_claim` event:

```go
sdk.NewAttribute("claim_type", claimType.String()),
```

### 8. Tests

In `x/knowledge/keeper/msg_server_test.go` (or equivalent):

1. **TestSubmitClaim_DefaultType** — unspecified → assertion
2. **TestSubmitClaim_ExplicitType** — each type stores correctly
3. **TestCreateFactFromClaim_PropagatesType** — accepted claim → fact preserves type
4. **TestQueryFacts_FilterByType** — type filter returns only matching facts

## Design Notes

- `UNSPECIFIED` maps to `ASSERTION` for backward compatibility — all existing claims become assertions
- `OBSERVATION` is separate from `ASSERTION` because observations are time/context-bound ("BTC was $50k on 2026-01-01") while assertions are general ("water freezes at 0°C")
- `NEGATION` is explicit rather than derived because "X is false" is different from "X is not yet verified" — agents need the distinction for constraint satisfaction
- The type is set at submission time and immutable — changing a claim's type would change its epistemic meaning

## Files Modified

- `proto/zerone/knowledge/v1/types.proto` — ClaimType enum, fields on Fact + Claim
- `proto/zerone/knowledge/v1/tx.proto` — field on MsgSubmitClaim
- `proto/zerone/knowledge/v1/query.proto` — filter on QueryFactsRequest
- `x/knowledge/types/*.pb.go` — regenerated
- `x/knowledge/keeper/msg_server.go` — default handling + propagation
- `x/knowledge/keeper/grpc_query.go` — query filtering
- `x/knowledge/keeper/rounds.go` — propagate type in createFactFromClaim
- `tools/knowledge-context/main.go` — output claim_type
- CLI tx + query commands

## Commit

Single commit: `feat(knowledge): add ClaimType enum for structured knowledge classification`
