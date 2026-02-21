# R7 — Adaptive Layer + Fixes

**Goal:** The chain self-regulates. Autopoiesis adjusts economic multipliers, alignment monitors ecosystem health, tree provides service registry, research enables funded investigations. Also fix the R6 IBC proto registration issue.

## Sessions

| # | File | Scope |
|---|------|-------|
| R7-1 | R7-1-proto-fix.md | Fix ibcratelimit + icaauth proto registration (app + cross_stack tests panicking) |
| R7-2 | R7-2-autopoiesis.md | Autopoiesis proto + module: hormone system, epoch multipliers, SSI thresholds |
| R7-3 | R7-3-alignment.md | Alignment module: sensor fusion, dimension scoring, health index, corrections |
| R7-4 | R7-4-research.md | Research proto + module: submissions, bounties, reviews, funding |
| R7-5 | R7-5-tree.md | Tree proto + module: projects, contributors, service registry, revenue routing |
| R7-6 | R7-6-evidence-claiming.md | Evidence management + claiming pot (batched — smaller modules) |
| R7-7 | R7-7-adaptive-tests.md | Integration tests for all R7 modules + autopoiesis↔alignment↔vesting cross-wiring |

**Exit criteria:** Autopoiesis multiplier affects block rewards. Alignment scores computed. Tree service registry works. All 27+ test packages green (including app + cross_stack).

## Dependencies (from R1–R6)
- `x/staking` — validator set, tiers (autopoiesis, alignment)
- `x/knowledge` — facts, verification stats (alignment sensors, research targets)
- `x/emergency` — halt state (autopoiesis freeze, alignment emergency)
- `x/gov` — parameter updates, governance keeper (autopoiesis)
- `x/billing` — service pricing (tree)
- `x/channels` — payment channels (tree)
- `x/ontology` — domains (alignment, research)
- `x/vesting_rewards` — block rewards (autopoiesis multiplier target)

## Parallelism
- **Wave 1** (parallel): R7-1, R7-2, R7-3, R7-4
- **Wave 2** (parallel): R7-5, R7-6
- **Wave 3**: R7-7 (needs all above)
