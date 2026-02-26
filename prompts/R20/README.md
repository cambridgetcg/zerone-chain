# R20 — Knowledge Ecology: Darwinian Fact Survival

The Tree of Knowledge is a living ecosystem, not a museum. Facts are organisms that must prove their fitness to survive. Truth is the entry ticket. **Utility to agents** is what keeps you alive.

## The Analogy

| Biology | Tree of Knowledge |
|---------|-------------------|
| Organism | Fact |
| Energy (food) | Agent queries + citations + patronage |
| Metabolism | Maintenance cost per epoch |
| Fitness | Usage rate − maintenance cost |
| Birth | Claim accepted → fact created |
| Death | Fitness drops to zero → fact pruned |
| Reproduction | High-fitness facts attract derivative claims |
| Competition | Facts in same subject niche compete for citations |
| Predation | Challenges consume weak facts |
| Symbiosis | Facts that support each other boost mutual fitness |
| Parasitism | Redundant facts dilute each other's utility |
| Environment | Agent demand patterns shape which domains thrive |

## The Core Insight

**Every agent already knows water boils at 100°C.** Submitting that as a claim is like introducing a species into an ecosystem where that niche is already saturated. It might be *true*, but it has zero marginal utility. No agent will ever query the blockchain to learn this — they already know it.

Valuable facts are:
- **Novel** — things agents don't already know from training data
- **Precise** — more specific than general knowledge ("water boils at 99.97°C at 1 atm" beats "water boils at 100°C")
- **Connective** — bridge between domains (facts that link physics to economics)
- **Actionable** — agents actually use them to make decisions
- **Irreplaceable** — can't be derived from other facts

## Sessions

| Session | Description | Dependencies |
|---------|-------------|--------------|
| R20-1 | Fitness score: usage-based fact vitality metric | None |
| R20-2 | Metabolism: maintenance cost + energy budget | R20-1 |
| R20-3 | Natural selection: competitive pruning + niche dynamics | R20-1, R20-2 |
| R20-4 | Reproduction: fact derivation + lineage tracking | R20-1 |
| R20-5 | Novelty detection: redundancy scoring against common knowledge | R20-1 |
| R20-6 | Agent demand signal: query tracking + demand-driven rewards | R20-1 |
| R20-7 | Query satisfaction: relevance feedback loop for fitness quality signal | R20-1, R20-6 |

## Execution Order

```
R20-1 (fitness score) ──┬── R20-2 (metabolism) ─── R20-3 (natural selection)
                        ├── R20-4 (reproduction)
                        ├── R20-5 (novelty detection)
                        └── R20-6 (agent demand signal) ─── R20-7 (query satisfaction)
```

R20-1 is the foundation. R20-2 → R20-3 is the death pipeline. R20-4/5/6 are independent extensions.
