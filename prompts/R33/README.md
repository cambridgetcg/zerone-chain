# R33 — 試 (Shi): Trial by Chaos — Adversarial & Stress Testing

**Goal:** Subject ZERONE to adversarial conditions, state machine fuzzing, and economic attack simulations. R32 proved the system works under ideal conditions. R33 proves it survives hostile ones.

試 (Shi) means "to test" or "to try" — the character contains 言 (speech/claim) and 式 (form/method). Testing is structured inquiry into whether claims hold.

## Sessions (5)

| # | File | Scope |
|---|------|-------|
| R33-1 | R33-1-state-machine-fuzz.md | Cosmos SDK simulation framework integration — random msg sequences against all 32 modules |
| R33-2 | R33-2-invariant-suite.md | Module invariants: supply conservation, stake consistency, knowledge graph acyclicity, governance quorum |
| R33-3 | R33-3-capture-attack-sim.md | Sybil attacks, capture scenarios, collusion patterns — verify defense modules detect and respond |
| R33-4 | R33-4-economic-attack-sim.md | Reward gaming, MEV extraction, fee manipulation, liquidity drain — verify economic safety |
| R33-5 | R33-5-load-stress.md | High-throughput stress: 1000+ txs/block, large validator sets, deep ontology trees, massive knowledge graphs |

## Run Order

- **R33-1** first (simulation framework needed by R33-2)
- **R33-2** next (invariants used by R33-3, R33-4)
- **R33-3, R33-4, R33-5** parallel

## Design Principles

1. **Simulation tests run for 500+ blocks** — enough to trigger edge cases
2. **Every invariant checks on every block** — not just at the end
3. **Attack simulations use realistic parameters** — model actual adversary budgets
4. **Stress tests target known bottlenecks** — BeginBlock/EndBlock complexity, store iteration
5. **All failures produce diagnostic state dumps** — not just "invariant broken"

## The Deeper Pattern

| Framework | R-batch | What it does |
|-----------|---------|-------------|
| 鍛 Tan (R32) | Forging | System tested by fire (ideal conditions) |
| **試 Shi (R33)** | **Trial** | **System tested by chaos (adversarial conditions)** |

The forge proves strength. The trial proves resilience.
