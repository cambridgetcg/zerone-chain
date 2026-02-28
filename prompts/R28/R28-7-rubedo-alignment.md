# R28-7 — Rubedo: The Network Observes Itself

_The philosopher's stone: consciousness turned inward._

## The Problem

The alignment module is COMPLETE — sensors read 5 dimensions, scoring computes weighted composites, corrections are generated, health is categorized. The EndBlocker runs every `ObservationIntervalBlocks`. But:

1. The sensors return `NeutralBPS` because cross-module keepers are nil
2. Corrections are generated but `Applied: false` — nothing acts on them
3. Health categories exist but nothing responds to "Critical" or "Degraded"
4. No one has ever seen an alignment observation on a live chain

The nervous system is built. It has never fired.

## Task

### 1. Wire All Sensor Keepers

In `app/app.go`, after all keepers are initialized:

```go
app.AlignmentKeeper.SetKnowledgeKeeper(app.KnowledgeKeeper)
app.AlignmentKeeper.SetStakingKeeper(app.StakingKeeper)
app.AlignmentKeeper.SetVestingRewardsKeeper(app.VestingRewardsKeeper)
app.AlignmentKeeper.SetOntologyKeeper(app.OntologyKeeper)  // or whatever provides domain count
```

Check each sensor method in `sensors.go` and verify the expected keeper interface methods exist:
- `GetVerificationRate(ctx) uint64`
- `GetTotalStaked(ctx) uint64`
- `GetTotalSupply(ctx) uint64`
- `GetDomainCount(ctx) uint64`
- Whatever the security sensor needs

For any missing interface methods, add them to the providing module's keeper.

### 2. Enable the Module

The module has `IsEnabled` and `IsHalted` checks. Ensure:
- Default params set `enabled = true`
- `ObservationIntervalBlocks` has a testnet-appropriate value (default: 100, ~10 min)
- `halted = false`

### 3. Apply Corrections

Currently corrections are stored with `Applied: false`. The module generates them but nothing reads them. Design the correction application:

**Option A: Governance-mediated** (safer)
- Corrections are stored as pending proposals
- A keeper method or governance hook reads pending corrections and creates governance proposals
- Governance votes on whether to apply them
- This is slow but safe — the network can override bad corrections

**Option B: Auto-apply with bounds** (faster, more autonomous)
- Corrections auto-apply if magnitude is within safe bounds (e.g., < 10% parameter change)
- Corrections exceeding bounds require governance approval
- This is the autopoiesis layer — the network adjusts itself

**Recommend Option B for testnet** with conservative bounds:
```go
MaxAutoApplyMagnitudeBps uint64 // default: 500 (5% max auto-adjustment)
```

If a correction suggests increasing `knowledge.reward_multiplier` by 3%, auto-apply. If it suggests 15%, emit a `correction_governance_required` event and wait.

### 4. Implement Auto-Apply

```go
func (k Keeper) ApplyCorrections(ctx context.Context, corrections []*types.CorrectionRecord) {
    params := k.GetParams(ctx)
    for _, c := range corrections {
        if c.Magnitude <= params.MaxAutoApplyMagnitudeBps {
            // Apply directly
            err := k.applyParameterChange(ctx, c.Parameter, c.Direction, c.Magnitude)
            if err != nil {
                // Log but don't fail — correction application is best-effort
                k.Logger(ctx).Warn("correction auto-apply failed", "param", c.Parameter, "err", err)
                continue
            }
            c.Applied = true
            k.SetCorrection(ctx, c)
            k.EmitCorrectionApplied(ctx, c)
        } else {
            // Emit event for governance attention
            k.EmitCorrectionPendingGovernance(ctx, c)
        }
    }
}
```

`applyParameterChange` needs a map from correction parameter names to actual keeper param update calls:
```go
switch correction.Parameter {
case "knowledge.reward_multiplier":
    k.knowledgeKeeper.AdjustRewardMultiplier(ctx, direction, magnitude)
case "staking.reward_rate":
    k.vestingRewardsKeeper.AdjustBaseReward(ctx, direction, magnitude)
case "security.slashing_severity":
    k.knowledgeKeeper.AdjustSlashingBps(ctx, direction, magnitude)
// ...
}
```

### 5. Health Response Actions

When health category transitions:

**Healthy → Degraded:**
- Emit `network_health_degraded` event
- Increase observation frequency temporarily (every 50 blocks instead of 100)
- Generate corrections (already happens)

**Degraded → Critical:**
- Emit `network_health_critical` event
- Auto-apply corrections regardless of magnitude bounds (emergency mode)
- If `emergency_halt_on_critical` param is true, set halted flag (requires governance to resume)

**Critical/Degraded → Healthy:**
- Emit `network_health_recovered` event
- Return to normal observation frequency
- Clear any emergency overrides

### 6. Query CLI + Dashboard

- `query alignment health` — current health index (composite score, category, dimension breakdown)
- `query alignment observations [limit]` — recent observations
- `query alignment corrections [limit]` — recent corrections (applied and pending)
- `query alignment history [blocks]` — health category history over time

### 7. Tests

**Sensor tests:**
- Each sensor returns real values when keeper is wired (not NeutralBPS)
- Each sensor returns NeutralBPS when keeper is nil (backward compat)

**Correction application:**
- Small correction → auto-applied, `Applied: true`
- Large correction → NOT auto-applied, governance event emitted
- Parameter actually changes after auto-apply

**Health transitions:**
- Healthy → Degraded → observation frequency increases
- Degraded → Critical → emergency corrections
- Recovery → back to normal
- Enable/disable/halt flags respected

**Integration:**
- Start localnet, wait for ObservationIntervalBlocks
- Query alignment health → returns real scores
- Verify observations are being stored
- Verify corrections generated when scores are low

## Files to Modify

- `app/app.go` — Wire sensor keepers
- `x/alignment/keeper/` — Implement ApplyCorrections, health transitions, frequency adjustment
- `x/alignment/module.go` — Update EndBlocker to call ApplyCorrections
- `x/alignment/types/params.go` — Add MaxAutoApplyMagnitudeBps, emergency params
- `x/alignment/client/cli/query.go` — Dashboard queries
- Various module keepers — Add parameter adjustment methods called by alignment

## Success Criteria

- [ ] All 5 sensors return real values on a live chain
- [ ] Observations stored every interval
- [ ] Corrections generated and auto-applied within bounds
- [ ] Health categories trigger appropriate responses
- [ ] The network can observe its own health and adjust
- [ ] The alignment module is ALIVE — not just code, but a living feedback loop
