# R9-5 — Economic Simulation: 1000-Block Run

## Goal

Simulate 1000+ blocks of chain operation, verifying token conservation, pool solvency,
reward decay, and economic stability. Prove the tokenomics hold under sustained operation.

## Working Directory

`/Users/yuai/Desktop/Zerone`

## Reference

- `/Users/yuai/Desktop/legible_money/x/simulation_test/economic_model_test.go` — 1003 LOC
- Rename all `legible` → `zerone`, `ulgm` → `uzrn`, `LGM` → `ZRN`

## Test: Economic Simulation

Port `TestEconomicSimulation` from draft and extend for Zerone's full module set.

### Simulation Setup
- 4 validators at different tiers (1, 2, 3, 4)
- 10 agent accounts with varying balances
- Knowledge tree seeded with 50 facts across 5 domains
- Toolbox with 5 registered tools
- Autopoiesis enabled with default parameters
- Alignment monitoring enabled

### Per-Block Activity (randomized)
Each block, simulate a mix of:
- **Knowledge claims** — random agents submit claims (30% of blocks)
- **Verification rounds** — commit/reveal cycles complete (20% of blocks)
- **Tool calls** — agents call tools, pay fees (25% of blocks)
- **Transfers** — random ZRN transfers between agents (15% of blocks)
- **Delegation changes** — stake/unstake (5% of blocks)
- **Governance** — parameter proposals + voting (2% of blocks)
- **Research** — submit/review research (3% of blocks)

### Invariants Checked Every Block
1. **Total supply conservation** — total supply unchanged (no inflation/deflation except block rewards)
2. **Module account solvency** — no module account goes negative
3. **Research fund monotonic** — research fund balance ≥ 0 (never overdrawn)
4. **Revenue split integrity** — every fee split sums to 100% (1M BPS)
5. **Reward decay** — block rewards decrease monotonically per the decay curve
6. **Pool solvency** — liquidity pool reserves ≥ minimum

### Epoch-Level Checks (every 100 blocks)
7. **Autopoiesis bounds** — all multipliers within [min, max]
8. **Alignment health** — AHI computed, corrections logged
9. **Staking ratios** — total staked is reasonable (10-90% of supply)
10. **Validator set stability** — no unexpected jailing (all validators online)

### End-of-Simulation Checks
11. **Token accounting** — sum of all account balances + module accounts == total supply
12. **No orphaned tokens** — no tokens in unknown accounts
13. **Reward distribution fairness** — higher-tier validators earned more than lower-tier
14. **Knowledge tree growth** — facts added, some verified, some challenged
15. **Tool revenue generated** — tool creators earned revenue

## Implementation

```
tests/simulation/
├── economic_sim_test.go  (main simulation, port from draft + extend)
├── activity_gen.go       (random activity generator)
└── invariants.go         (invariant check functions)
```

### Simulation Engine Pattern
```go
func TestEconomicSimulation(t *testing.T) {
    app := setupFullApp(t)
    
    // Seed initial state
    seedValidators(app, 4)
    seedAgents(app, 10)
    seedKnowledge(app, 50)
    seedTools(app, 5)
    
    for height := int64(1); height <= 1000; height++ {
        // Generate random activity
        msgs := generateBlockActivity(height, rand)
        
        // Execute block
        app.BeginBlock(...)
        for _, msg := range msgs {
            app.DeliverTx(...)
        }
        app.EndBlock(...)
        app.Commit()
        
        // Check invariants
        checkInvariants(t, app, height)
    }
    
    // Final checks
    checkFinalState(t, app)
}
```

## Constraints

- Simulation must complete in <120s (use in-memory app, no network)
- Random seed must be deterministic (reproducible failures)
- Log the random seed so failures can be reproduced: `t.Logf("seed: %d", seed)`
- Every invariant violation must fail immediately with clear error message
- No mocked balances — use real bank keeper operations
- Block rewards must come from the configured mint/vesting source
