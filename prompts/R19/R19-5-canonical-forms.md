# R19-5 — Canonical Form Normalization

## Context

The same knowledge can be expressed infinitely many ways in natural language. "Water freezes at 0°C" and "The freezing point of water is 0 degrees Celsius" and "H₂O undergoes liquid→solid phase transition at 273.15K" are all the same fact. Without normalization, the knowledge base accumulates semantic duplicates that waste verifier time and confuse agent retrieval.

Canonical forms solve this: a machine-readable, deterministic representation of a claim's core meaning. Two claims with the same canonical form are the same fact, regardless of prose.

This is the hardest of the four optimizations to get right. It depends on R19-2 (claim types) and R19-4 (structured fields) being in place first.

## Task

### 1. Proto: Add Canonical Form to Fact and Claim

In `proto/zerone/knowledge/v1/types.proto`, add to `Fact`:

```protobuf
string canonical_form      = 28;  // Machine-readable normalized form
string canonical_hash      = 29;  // SHA-256 of canonical_form for dedup
```

Add to `Claim`:

```protobuf
string canonical_form      = 17;  // Machine-readable normalized form
string canonical_hash      = 18;  // SHA-256 of canonical_form for dedup
```

Add to `MsgSubmitClaim` in `tx.proto`:

```protobuf
string canonical_form      = 11;  // Optional — auto-derived from structure if omitted
```

### 2. Canonical Form Spec

Define a deterministic canonical form grammar. The form is constructed from the claim type + structured fields:

```
ASSERTION:   assert(domain, subject, predicate[, scope])
RELATION:    relate(domain, subject, relation_verb, object[, scope])
DEFINITION:  define(domain, term, meaning)
CONSTRAINT:  constrain(domain, subject, constraint[, scope])
NEGATION:    negate(domain, subject, predicate[, scope])
OBSERVATION: observe(domain, subject, predicate, temporal_scope)
```

**Examples:**
```
assert(physics, "entropy of closed system", "cannot decrease spontaneously", "classical thermodynamics")
define(mathematics, "prime number", "natural number greater than 1 with no positive divisors other than 1 and itself")
relate(physics, "curry-howard correspondence", "isomorphism", "typed lambda calculus")
constrain(economics, "zrn max supply", "must not exceed 222222222")
observe(economics, "btc price", "50000 usd", "2026-01-01")
negate(physics, "perpetual motion machine", "can exist")
```

**Normalization rules:**
1. All strings lowercased
2. Leading/trailing whitespace trimmed
3. Internal whitespace collapsed to single space
4. Domain must match registered domain name exactly
5. UTF-8 normalized to NFC
6. Numbers in decimal notation (no scientific notation in canonical form)

### 3. Implementation: Canonical Form Builder

In `x/knowledge/types/canonical.go`:

```go
// BuildCanonicalForm constructs a canonical form from a claim's type and structure.
// Returns empty string if structure is insufficient.
func BuildCanonicalForm(claimType ClaimType, structure *ClaimStructure, domain string) string

// NormalizeCanonicalForm applies normalization rules to a canonical form string.
func NormalizeCanonicalForm(form string) string

// HashCanonicalForm returns the SHA-256 hex digest of a normalized canonical form.
func HashCanonicalForm(form string) string
```

`BuildCanonicalForm` logic:
- If structure is nil or subject is empty, return ""
- Map ClaimType to function name (assert/relate/define/constrain/negate/observe)
- Build canonical form string from normalized fields
- Apply NormalizeCanonicalForm

### 4. Submission: Auto-derive or Validate

In `x/knowledge/keeper/msg_server.go`, `SubmitClaim()`:

```go
canonicalForm := msg.CanonicalForm
if canonicalForm == "" && msg.Structure != nil {
    // Auto-derive from structure + type
    canonicalForm = types.BuildCanonicalForm(claimType, msg.Structure, msg.Domain)
}
if canonicalForm != "" {
    canonicalForm = types.NormalizeCanonicalForm(canonicalForm)
    canonicalHash := types.HashCanonicalForm(canonicalForm)

    // Dedup against canonical hash (stronger than content_hash)
    if existingID, exists := m.keeper.GetClaimByCanonicalHash(ctx, canonicalHash); exists {
        return nil, fmt.Errorf("canonical duplicate: matches existing claim %s", existingID)
    }
}
```

### 5. Store: Canonical Hash Index

In `x/knowledge/types/keys.go`:

```go
CanonicalHashPrefix = []byte{0x34}  // 0x34 | canonical_hash → claim_id/fact_id
```

In `x/knowledge/keeper/state.go`:

```go
func (k Keeper) SetCanonicalHash(ctx context.Context, hash string, id string) error
func (k Keeper) GetClaimByCanonicalHash(ctx context.Context, hash string) (string, bool)
```

### 6. Query: Canonical Form Lookup

In `proto/zerone/knowledge/v1/query.proto`:

```protobuf
rpc FactByCanonical(QueryFactByCanonicalRequest) returns (QueryFactByCanonicalResponse) {
    option (google.api.http).get = "/zerone/knowledge/v1/fact_by_canonical/{canonical_hash}";
}

message QueryFactByCanonicalRequest {
  string canonical_hash = 1;  // SHA-256 hex of normalized canonical form
  string canonical_form = 2;  // Or provide the form directly (server hashes it)
}

message QueryFactByCanonicalResponse {
  Fact fact = 1;
}
```

### 7. Context Server: Canonical Form in Output

Add to all output formats:

XML:
```xml
<fact id="abc" domain="physics" confidence="95%">
  <content>Entropy cannot decrease in a closed system</content>
  <canonical>assert(physics, "entropy of closed system", "cannot decrease spontaneously", "classical thermodynamics")</canonical>
</fact>
```

JSON:
```json
{
  "canonical_form": "assert(physics, \"entropy of closed system\", \"cannot decrease spontaneously\")",
  "canonical_hash": "a1b2c3..."
}
```

Add `/context` param: `canonical=true` includes canonical forms (off by default to save bandwidth).

### 8. CLI

Add to `submit-claim`:

```
--canonical string    Explicit canonical form (auto-derived from structure if omitted)
```

Add query:

```
zeroned query knowledge fact-by-canonical [canonical-form-or-hash]
```

### 9. Tests

1. **TestBuildCanonicalForm_Assertion** — correct form for each claim type
2. **TestBuildCanonicalForm_NoStructure** — returns empty string
3. **TestNormalizeCanonicalForm** — case, whitespace, NFC normalization
4. **TestHashCanonicalForm** — deterministic hash
5. **TestCanonicalDedup** — same canonical form = rejected as duplicate
6. **TestCanonicalDedup_DifferentProse** — different text, same canonical → duplicate caught
7. **TestAutoDerive_FromStructure** — omitted canonical form → auto-built from structure
8. **TestQueryFactByCanonical_Hash** — lookup by hash works
9. **TestQueryFactByCanonical_Form** — lookup by form (server-side hash) works

## Design Notes

- **Auto-derivation is the happy path** — most submitters provide structure (R19-4), canonical form is built automatically. Manual canonical form is for power users who want to override.
- **Canonical form is NOT the claim** — it's a normalized fingerprint. The human-readable `fact_content` remains authoritative. Canonical form is for dedup and machine lookup.
- **This won't catch all semantic duplicates** — "entropy increases" and "disorder increases" have different canonical forms. That's fine. Canonical dedup catches the easy cases (80/20 rule). Full semantic dedup is an AI layer problem, not a protocol layer problem.
- **Backward compat** — existing facts have no canonical form. That's fine — they're findable via content_hash. New facts should have canonical forms.
- **Grammar is extensible** — new claim types get new function names. The grammar is a convention, not a hard protocol rule. Validation ensures well-formedness but doesn't parse the grammar.

## Dependencies

- R19-2 (ClaimType enum) — needed for function name mapping
- R19-4 (ClaimStructure) — needed for auto-derivation

## Files Modified

- `proto/zerone/knowledge/v1/types.proto` — canonical_form + canonical_hash on Fact, Claim
- `proto/zerone/knowledge/v1/tx.proto` — canonical_form on MsgSubmitClaim
- `proto/zerone/knowledge/v1/query.proto` — FactByCanonical query
- `x/knowledge/types/*.pb.go` — regenerated
- `x/knowledge/types/canonical.go` — **NEW**: BuildCanonicalForm, NormalizeCanonicalForm, HashCanonicalForm
- `x/knowledge/types/canonical_test.go` — **NEW**: form building + normalization tests
- `x/knowledge/types/keys.go` — canonical hash prefix
- `x/knowledge/keeper/state.go` — canonical hash index
- `x/knowledge/keeper/msg_server.go` — auto-derive, validate, dedup
- `x/knowledge/keeper/rounds.go` — propagate canonical form to fact
- `x/knowledge/keeper/grpc_query.go` — FactByCanonical handler
- `tools/knowledge-context/main.go` — canonical form output
- CLI tx + query commands

## Commit

Single commit: `feat(knowledge): add canonical form normalization for semantic dedup`
