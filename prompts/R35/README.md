# R35 — 門 (Mon): The Gate — Testnet Launch Qualification

**Goal:** Final qualification gate before public testnet. Run the complete test pipeline, fix everything that fails, generate the testnet genesis, and perform a dress rehearsal.

門 (Mon) means "gate" — the threshold between preparation and action. Everything before this was building. Everything after is live.

## Sessions (4)

| # | File | Scope |
|---|------|-------|
| R35-1 | R35-1-full-pipeline.md | Run ALL tests end-to-end: unit → integration → cross-stack → simulation → E2E → adversarial. Fix failures. |
| R35-2 | R35-2-testnet-genesis.md | Generate production-quality testnet genesis: all params tuned, initial validators, faucet, explorer config |
| R35-3 | R35-3-dress-rehearsal.md | Full dress rehearsal: genesis ceremony → chain start → 1000 blocks → upgrade → IBC transfer → export |
| R35-4 | R35-4-launch-checklist.md | Final checklist: security audit items, known limitations, launch announcement, monitoring live |

## Run Order

Sequential: R35-1 → R35-2 → R35-3 → R35-4

Each session depends on the previous passing.

## The Complete Test Pipeline

```
┌─────────────────────────────────────────────────────┐
│                    R35 Pipeline                      │
├─────────────────────────────────────────────────────┤
│ 1. Unit tests (493+ tests, all modules)             │
│ 2. Integration tests (event audit, param bounds)    │
│ 3. Cross-stack tests (15+ R28-R31 scenarios)        │
│ 4. Simulation (500-block fuzz, all invariants)      │
│ 5. E2E tests (real chain, real txs, real consensus) │
│ 6. Adversarial tests (attack simulations)           │
│ 7. Stress tests (load and performance)              │
│ 8. IBC tests (real relayer, real counterparty)      │
│ 9. Upgrade test (Cosmovisor binary swap)            │
│ 10. Genesis round-trip (export → reimport)          │
└─────────────────────────────────────────────────────┘
```

## The Deeper Pattern

| Framework | R-batch | What it does |
|-----------|---------|-------------|
| 鍛 Tan (R32) | Forging | System tested by fire |
| 試 Shi (R33) | Trial | System tested by chaos |
| 橋 Hashi (R34) | Bridge | System connected to the world |
| **門 Mon (R35)** | **Gate** | **Final qualification — ready or not** |

## After R35

If R35 passes: **public testnet launch**.

The testnet is not mainnet — it's the final testing ground where external validators, real users, and real conditions provide the ultimate test. But the gate of R35 ensures we only go live with a system that has passed every test we know how to write.

## Roadmap to Mainnet

```
R32-R35 (testing)  →  Public Testnet  →  Security Audit  →  Mainnet
   (current)           (weeks)            (external firm)     (target)
```
