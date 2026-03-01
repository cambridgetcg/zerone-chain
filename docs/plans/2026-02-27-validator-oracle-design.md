# Validator Evaluation Oracle — Design Document

**Date:** 2026-02-27
**Prompt:** R27-6
**Status:** Approved

## Problem

Validators hardcode `verdict = "accept"` with `confidence = 600,000` for all claims (vote_extensions.go:169-172). This means every claim — including obviously false ones like "Water freezes at 200C" — gets unanimously verified. The rational strategy is vote-with-the-crowd because validators have no tools to evaluate claim content.

## Solution

An **advisory sidecar process** (`zerone-oracle`) that runs alongside the validator node. Validators can optionally query it before voting. Oracle failure never blocks consensus.

## Architecture

```
+---------------+     HTTP/JSON     +--------------------+
|  zeroned      |  POST /evaluate   |  zerone-oracle     |
|  (validator)  | ----------------> |  (sidecar)         |
|               | <---------------- |                    |
|  vote ext     |  verdict/conf     |  Tier 1: Static    |
|  handler      |                   |  Tier 2: LLM       |
|  (abci)       |  POST /prefetch   |  Cache (LRU 1000)  |
|               | ----------------> |                    |
+---------------+                   +--------------------+
```

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Architecture | Separate sidecar binary | Crash isolation, independent upgrades, industry pattern (Skip, Slinky) |
| Protocol | HTTP/JSON | Simpler than gRPC, testable with curl, adequate for localhost |
| LLM backend | Anthropic Claude API | Project identity alignment, direct HTTP (no heavy SDK) |
| Static evaluator | Keyword + numerical contradiction | Honest about limitations: catches number mismatches and explicit negations, not semantic contradictions |
| Confidence threshold | 0.6 minimum (configurable) | Protects validators from slashing on weak oracle signals |
| LLM timeout | 2s hard limit | Must complete within vote extension window |
| Cache key | sha256(domain + "\|" + claim_text) | Domain-aware to handle same text in different contexts |

## Oracle Sidecar (`cmd/oracle/`)

### Endpoints

**POST /evaluate**
```json
Request:  { "claim": "...", "domain": "...", "claim_type": "..." }
Response: { "verdict": "accept|reject|uncertain", "confidence": 0.0-1.0, "reasoning": "..." }
```

**POST /prefetch** — Same request body, returns immediately. Pre-warms cache for upcoming vote extension.

**GET /health** — Returns `{"status": "ok", "tier": "static|llm"}`

### Evaluation Pipeline

1. **Static check (always runs):**
   - Load 777 genesis axioms from embedded JSON (compile-time, no filesystem dependency)
   - Build keyword index: lowercase tokens from axiom `Statement`, indexed by domain
   - For incoming claim: find matching axioms in same domain by keyword overlap
   - Contradiction detection:
     - **Numerical:** Extract numbers + units from claim and axiom, compare values (with basic unit normalization for C/K/F)
     - **Negation:** Detect explicit negation words ("not", "never", "false", "incorrect") that flip the meaning
   - Return: `reject` if contradiction found (confidence = match strength), `accept` if claim closely matches axiom, `uncertain` otherwise

2. **LLM check (if tier=llm and API key configured):**
   - Send claim + top matching axioms to Claude API with fact-checking system prompt
   - 2s hard timeout — return `uncertain` on timeout
   - Parse structured response for verdict + confidence
   - Cache result keyed by sha256(domain + "|" + claim_text), LRU 1000 entries

3. **Combine:**
   - Static high-confidence verdict (accept OR reject, confidence > 0.7) → use it, short-circuit
   - Static uncertain + LLM available → use LLM result
   - Static uncertain + no LLM → return `uncertain`, confidence 0.5

### Tier 1 Limitations (document honestly)

Tier 1 catches:
- Claims with numerical values that contradict axiom values in the same domain
- Claims with explicit negation of known axioms

Tier 1 does NOT catch:
- Semantic contradictions ("the sun orbits the earth")
- Paraphrased falsehoods
- Claims in domains with no matching axioms
- Subtle logical errors

Tier 2 (LLM) is needed for general fact-checking.

## Vote Extension Integration

### Oracle Client (`app/oracle_client.go`)

```go
type OracleEvaluation struct {
    Verdict    string  // "accept", "reject", "uncertain"
    Confidence float64 // 0.0 - 1.0
    Reasoning  string
}

type OracleClient interface {
    Evaluate(ctx context.Context, claim, domain, claimType string) (*OracleEvaluation, error)
}
```

### Integration Point: `app/vote_extensions.go:169-172`

Replace hardcoded stub with `evaluateWithOracle()`:

1. `OracleClient == nil` → return `"accept"`, `600_000` (unchanged behavior)
2. Fetch claim via `KnowledgeKeeper.GetClaim(ctx, round.ClaimId)`
3. Call `OracleClient.Evaluate()` with 2s context timeout
4. On error → log warning, return `"accept"`, `600_000` (graceful degradation)
5. On success:
   - **Confidence threshold:** If confidence < 0.6 → treat as uncertain → `"accept"`, `500_000`
   - Map verdict to vote string, map float64 confidence to uint64 (x 1,000,000)
   - `"uncertain"` verdict → default to `"accept"` with oracle's confidence

### Config (app.toml)

```toml
[oracle]
enabled = false
endpoint = "http://localhost:8081"
timeout = "2s"
min_confidence = 0.6
```

### Safety Guarantees

- Oracle disabled = zero behavior change (nil check)
- Oracle timeout = fallback to accept (context deadline)
- Oracle crash = fallback to accept (HTTP error)
- Oracle low confidence = fallback to accept (threshold check)

## Files

### Create
- `cmd/oracle/main.go` — Sidecar HTTP server, flag parsing, startup
- `cmd/oracle/evaluator.go` — Evaluator interface, combine logic
- `cmd/oracle/evaluator_static.go` — Tier 1: keyword index, numerical/negation contradiction
- `cmd/oracle/evaluator_llm.go` — Tier 2: Claude API client, response parsing, cache
- `cmd/oracle/evaluator_static_test.go` — Static evaluator unit tests
- `cmd/oracle/evaluator_llm_test.go` — LLM evaluator unit tests (mock HTTP)
- `cmd/oracle/main_test.go` — HTTP endpoint integration tests
- `app/oracle_client.go` — OracleClient interface + HTTP implementation
- `app/oracle_client_test.go` — Client tests (mock server, timeout, fallback)
- `docs/validator-oracle.md` — Operator documentation
- `scripts/test-oracle.sh` — Integration test script

### Modify
- `app/vote_extensions.go` — Replace stub verdict with oracle query
- `app/vote_extensions_test.go` — Add oracle-enabled test cases
- `app/app.go` — Wire oracle client from config
- `cmd/zeroned/cmd/config.go` — Add [oracle] config section

## Testing

### Unit Tests
- Static evaluator: axiom loading, keyword index, numerical contradiction, negation detection, domain filtering, uncertain for unrelated claims
- LLM evaluator: mock Claude API, timeout handling, cache hit/miss, response parsing
- Oracle client: mock HTTP server, timeout behavior, error fallback, confidence threshold, nil passthrough
- Vote extensions: oracle-enabled evaluation, timeout fallback, disabled = unchanged

### Integration Test
- Start oracle sidecar with --tier static
- Submit true claim matching axiom → expect accept
- Submit false claim contradicting axiom → expect reject
- Submit unrelated claim → expect uncertain
- Kill oracle → vote extension still works (fallback)
