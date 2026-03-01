# R30 — 掃除 (Sōji): Sweep the Temple Clean

**Goal:** Infrastructure hygiene before the next architectural batch. Fix systemic issues, add safety nets, harden what exists.

Sōji (掃除) is the Japanese practice of cleaning as meditation — monks sweep the temple not because it's dirty, but because sweeping is practice. R28 activated organs, R29 taught them balance, R30 ensures the foundation is sound before R31 adds circulation.

## Sessions (4)

| # | File | Scope |
|---|------|-------|
| R30-1 | R30-1-proto-consistency.md | Proto-Go audit, CI enforcement, genesis round-trip tests |
| R30-2 | R30-2-param-governance-safety.md | Parameter validation hardening, governance bounds, migration safety |
| R30-3 | R30-3-event-observability.md | Structured event taxonomy, event documentation, event consistency |
| R30-4 | R30-4-cross-stack-coverage.md | Cross-module integration test coverage for R28/R29 features |

## Run Order

All four can run in parallel — they touch different concerns with no dependencies.

## Known Issues Already Fixed (Pre-R30)

- `capture_defense/types/query_ext.go` — FlaggedDomains hand-rolled gRPC (R28-8)
- `capture_challenge/types/query_ext.go` — ActiveChallenges hand-rolled gRPC (R28-8)
- `knowledge` MetabolismStatus query — never in proto (R28-4)
- `alignment` correction confidence params — never in proto (R29-4)
- All module rawDesc stale after R29 param additions
- Cross-stack test: bounded correction magnitude (R28-7)
- Event audit: graduateMentorship delegation pattern (R28-6)
