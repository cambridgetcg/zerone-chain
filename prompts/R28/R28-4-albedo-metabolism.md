# R28-4 — Albedo: Knowledge Metabolism Refinement

_Purifying the lifecycle of facts — what lives, what dies, what transforms._

## The Problem

From R25 assessment:
- "Metabolism: 6/10 — Fitness epochs work but 10K block interval too long"
- "Energy at 0 (AT_RISK) doesn't recover until epoch boundary even after patronage"
- "Satisfaction: 4/10 — Confidence cap not enforced (950K observed despite 880K cap)"

Facts have a lifecycle (energy, fitness, metabolism), but the lifecycle has bugs. Facts that should be dying aren't dying cleanly. Facts that receive patronage don't recover immediately. The confidence cap is cosmetic.

## Task

### 1. Fix Energy Recovery on Patronage

When a fact receives patronage (`MsgPatroniseFact`), its energy should increase immediately — not wait for the next epoch boundary.

Find the patronage handler and add:
```go
// After recording patronage:
fact.Energy += patronageEnergyBoost(amount, params)
if fact.Energy > params.EnergyCap {
    fact.Energy = params.EnergyCap
}
// If fact was AT_RISK and energy now > threshold, transition to ACTIVE
if fact.Status == "AT_RISK" && fact.Energy >= params.ActiveEnergyThreshold {
    fact.Status = "ACTIVE"
    // Emit recovery event
}
k.SetFact(ctx, fact)
```

### 2. Enforce Confidence Cap

Find where confidence is set/updated and enforce the cap:

```bash
grep -rn "Confidence\|confidence" --include="*.go" x/knowledge/keeper/ | grep -i "set\|update\|assign"
```

The cap (`params.MaxConfidence`, apparently 880,000) should be enforced:
- On round completion when setting initial confidence
- On patronage when confidence increases
- On any update path

R25 observed 950K confidence despite an 880K cap. Find and fix the path that bypasses the cap.

### 3. Tune Metabolism Parameters for Testnet

The 10K block epoch (~7 hours on localnet, ~17 hours on testnet) is too long for iteration. Add testnet-appropriate defaults:

```go
// Testnet params
MetabolismEpochBlocks: 1000,        // ~100 min
EnergyDecayRate:       100,          // per epoch (BPS of max)
ActiveEnergyThreshold: 300000,       // 30% of max to stay ACTIVE
AtRiskThreshold:       100000,       // 10% of max → AT_RISK
ExtinctionThreshold:   10000,        // 1% of max → EXTINCT
```

### 4. Add Fact Lifecycle Events

Ensure every status transition emits a clear event:
- `fact_status_changed` with `{fact_id, old_status, new_status, energy, reason}`
- Reasons: `decay`, `patronage_recovery`, `challenge_degradation`, `extinction`

These events feed monitoring and alignment sensors.

### 5. Fix Satisfaction Feedback Loop

R25 noted "5 query RPCs without CLI" for satisfaction. Beyond CLI (covered in R27-2), verify the feedback loop works:

- Query satisfaction → returns relevance score
- Relevance score feeds into metabolism (high relevance = slower decay?)
- If this loop doesn't exist, document it as a design decision: should popular facts live longer?

### 6. Add Metabolism Dashboard Query

New query that returns a health overview:
```
query knowledge metabolism-status
```
Returns:
```json
{
    "total_facts": 800,
    "active": 750,
    "at_risk": 35,
    "extinct": 15,
    "avg_energy": 650000,
    "epoch": 42,
    "next_epoch_block": 43000,
    "recent_recoveries": 3,
    "recent_extinctions": 1
}
```

### 7. Tests

- Fact receives patronage → energy increases immediately (not at epoch)
- AT_RISK fact with patronage → transitions to ACTIVE immediately
- Confidence never exceeds cap (test the specific path that was 950K)
- Epoch decay reduces energy correctly
- Energy below threshold → AT_RISK
- Energy near zero → EXTINCT
- Extinct facts are not verifiable/challengeable
- Status transition events emitted with correct reasons
- Metabolism dashboard query returns accurate counts

## Files to Modify

- `x/knowledge/keeper/msg_server.go` — Fix patronage energy recovery
- `x/knowledge/keeper/rounds.go` — Enforce confidence cap on round completion
- `x/knowledge/keeper/` — Fix any other confidence update paths
- `x/knowledge/module.go` — Metabolism epoch logic (if needed)
- `x/knowledge/types/params.go` — Testnet defaults
- `x/knowledge/client/cli/query.go` — Metabolism dashboard query
- `x/knowledge/keeper/` — New file or extend existing: `metabolism.go`

## Success Criteria

- [ ] Patronage immediately restores energy (no waiting for epoch)
- [ ] AT_RISK facts recover to ACTIVE on sufficient patronage
- [ ] Confidence cap enforced everywhere (no more 950K)
- [ ] Metabolism parameters tuned for testnet iteration speed
- [ ] Status transition events emitted for all lifecycle changes
- [ ] Metabolism dashboard query works
- [ ] Facts now have a real lifecycle: they breathe, they weaken, they can be saved or they die
