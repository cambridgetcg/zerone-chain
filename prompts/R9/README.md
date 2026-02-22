# R9 — Integration Testing + Multi-Validator + Simulations

**Goal:** Everything works together under realistic conditions. 4 validators run PoT consensus.
IBC works between two chains. Economic model holds over 1000+ blocks. Adversarial attacks
are detected and mitigated.

## Sessions

| # | File | Scope |
|---|------|-------|
| R9-1 | R9-1-revenue-integration.md | Revenue flow integration tests: port 10 tests from draft + new cross-module flows |
| R9-2 | R9-2-genesis-validation.md | Genesis validation: full round-trip, ante handler integration tests, boot-test fix |
| R9-3 | R9-3-multi-validator.md | 4-validator local testnet: PoT rounds, tier progression, slashing |
| R9-4 | R9-4-ibc-e2e.md | IBC E2E: two-chain transfers, rate limiting, timeout handling |
| R9-5 | R9-5-economic-sim.md | Economic simulation: 1000-block run, token conservation, pool solvency |
| R9-6 | R9-6-adversarial-sim.md | Adversarial simulation: 10 attack scenarios from draft + new Zerone-specific attacks |

**Exit criteria:** 4 validators run PoT consensus. IBC transfers succeed with rate limiting.
No economic leaks over 1000 blocks. All adversarial attacks detected.

## Dependencies (from R1–R8)
- All 32 modules wired and booting
- Full ante handler chain active
- `zeroned start` produces blocks
- 35 test packages passing

## Parallelism
- **Wave 1** (parallel): R9-1, R9-2, R9-3, R9-4
- **Wave 2** (parallel): R9-5, R9-6 (may depend on harness improvements from wave 1)
