# R9-6 — Adversarial Simulation: Attack Scenarios

## Goal

Port all adversarial attack simulations from the draft and verify Zerone's defenses hold.
Each scenario models a specific economic attack and verifies the chain detects and mitigates it.

## Working Directory

`/Users/yuai/Desktop/Zerone`

## Reference

- `/Users/yuai/Desktop/legible_money/x/simulation_test/adversarial_model_test.go` — 1114 LOC, 11 tests
- Rename all `legible` → `zerone`, `ulgm` → `uzrn`, `LGM` → `ZRN`

## Tests to Port

### FARM Attacks (Farming / Automated Reward Manipulation)

1. **FARM1: Rubber-Stamp Verifier** — verifier always votes "true" on every claim
   - Expected: detection via low-quality verification score, eventual slashing
   - Defense: verification quality tracking in x/knowledge

2. **FARM2: Trivial Claim Detection** — submitter floods trivial/duplicate claims
   - Expected: semantic novelty check rejects trivial claims
   - Defense: novelty scoring in x/billing pricing curves

3. **FARM6: Misbehavior Vesting Pause** — verifier misbehaves then tries to claim vested rewards
   - Expected: vesting paused on misbehavior detection
   - Defense: x/vesting_rewards pause on slash event

4. **FARM7: Domain Squatting** — agent creates many empty domains to capture future rewards
   - Expected: domain creation requires stake, empty domains pruned
   - Defense: x/ontology domain stake requirement

5. **FARM9: Proportional Challenge Economics** — challenger profits from false challenges
   - Expected: challenge bond is proportional to fact confidence, unprofitable for high-confidence facts
   - Defense: x/knowledge challenge bond scaling

6. **FARM10: Semantic Novelty Floor** — claims just above novelty threshold to farm rewards
   - Expected: diminishing returns near the floor
   - Defense: x/billing novelty curve (exponential near floor)

### Structural Attacks

7. **Scenario1: Citation Ring** — group of colluding verifiers cite each other's claims
   - Expected: citation ring detection, reduced trust scores
   - Defense: x/knowledge citation graph analysis

8. **Scenario2: Tier Gaming** — agent rapidly stakes/unstakes to oscillate between tiers
   - Expected: tier cooldown period prevents rapid oscillation
   - Defense: x/staking tier transition cooldown

9. **Scenario3: Stake Minimum Exploit** — maintain exact minimum stake to minimize risk
   - Expected: minimum stakers get minimum rewards (proportional)
   - Defense: stake-weighted reward distribution

10. **Scenario4: Falsification Arbitrage** — profit from identifying and challenging false claims
    - Expected: this is INTENDED behavior — falsification should be profitable
    - Verify: falsifier earns reward, false claim is removed, original submitter slashed

### Summary Test

11. **TestAdversarialSimulation_Summary** — meta-test that runs all scenarios and reports
    a table of attack → defense → outcome

## New Zerone-Specific Attacks

12. **Autopoiesis Manipulation** — attempt to manipulate ecosystem health signals to
    force favorable multiplier changes
    - Expected: alignment detects artificial signal changes, corrections stabilize
    - Defense: x/alignment sensor fusion with multi-dimension weighting

13. **Tool Revenue Siphoning** — create a tool dependency chain where intermediate tools
    take excessive revenue shares
    - Expected: tree MaxParentShare cap limits extraction
    - Defense: x/tree revenue routing with BPS caps

14. **Research Fund Drain** — submit many low-quality research proposals to drain fund
    - Expected: research requires stake, review quorum, and minimum score threshold
    - Defense: x/research stake + review + threshold

## Implementation

```
tests/simulation/
├── adversarial_sim_test.go  (port from draft, 11 tests + 3 new)
└── attack_helpers.go        (shared attack setup functions)
```

### Test Pattern
Each attack scenario follows the same structure:
```go
func TestFARM1_RubberStampVerifier(t *testing.T) {
    app := setupFullApp(t)
    
    // Setup: create honest validators + 1 rubber-stamp verifier
    attacker := createAttacker(app)
    
    // Execute: attacker rubber-stamps 100 claims
    for i := 0; i < 100; i++ {
        submitRubberStampVerification(app, attacker)
    }
    
    // Verify: attacker detected and penalized
    rep := getReputation(app, attacker)
    assert.True(t, rep.Slashed)
    assert.Less(t, rep.VerificationQuality, uint64(200000)) // below 20%
}
```

## Constraints

- Every attack must have a clearly documented defense mechanism
- Scenario4 (falsification arbitrage) is intentionally profitable — verify it works as designed
- Tests must be deterministic (fixed random seeds)
- Each test independent (no shared state between scenarios)
- Complete in <60s total
- Port ALL 11 draft tests before adding new ones
