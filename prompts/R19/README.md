# R19 — Agent-Optimized Knowledge Claims

Structural improvements to the knowledge module that transform claims from opaque prose into a typed, linked, machine-readable knowledge graph optimized for AI agent consumption.

## Sessions

| Session | Description | Dependencies |
|---------|-------------|--------------|
| R19-1 | Malformed vote option | None |
| R19-2 | Claim type tags (assertion/relation/definition/constraint/negation/observation) | None |
| R19-3 | Semantic anchors (typed fact relationships: supports/contradicts/requires/refines) | None |
| R19-4 | Structured claim fields (subject/predicate/scope/tags) | R19-2 |
| R19-5 | Canonical form normalization (deterministic dedup) | R19-2, R19-4 |

## Execution Order

```
R19-1 ──────────────────────────────────┐
R19-2 (claim types) ──────┬─── R19-4 ──┼─── R19-5
R19-3 (semantic anchors) ─┘            │
                                        └─── done
```

R19-1, R19-2, R19-3 are independent — can run in any order or parallel.
R19-4 requires R19-2.
R19-5 requires R19-2 + R19-4.

## What This Enables

Before R19: agents get a flat list of prose strings with confidence numbers.

After R19: agents get a typed, structured, linked knowledge graph with:
- **Type tags** → filter by knowledge shape (definitions first, then assertions, then constraints)
- **Semantic anchors** → traverse the graph (what supports this? what contradicts it?)
- **Structured fields** → exact subject/tag lookup (no semantic search needed)
- **Canonical forms** → deterministic dedup across paraphrases
- **Malformed votes** → garbage claims get identified and penalized
