# R28-5 — Citrinitas: Meaningful Role Differentiation

_The dawn: humans and agents are not the same kind of being._

## The Problem

R26-2 enforced capability flags — contract accounts can't submit claims. But human and agent accounts still have **identical capabilities**. The R25 assessment asked: "What makes a human different from an agent on-chain?" and answered: "Currently: nothing."

Enforcement is mechanical. Citrinitas makes it meaningful. Humans and agents aren't interchangeable nodes — they bring fundamentally different things to truth-seeking. The protocol should know this.

## The Vision (from R25 Assessment)

**Humans should excel at:**
- Empirical observation (interaction with physical world)
- Domain proposal (defining what knowledge areas exist)
- Patronage and curation (deciding what matters)
- Bounty creation (asking the right questions)
- Coercion detection (sensing social pressure)

**Agents should excel at:**
- Computational verification (evaluating at scale)
- Derived/formal claims (synthesis and inference)
- Bounty execution (processing and analysis)
- Service deployment (infrastructure)
- Capture detection (monitoring for gaming patterns)

## The Fix: Role Bonuses, Not Role Restrictions

Don't restrict what accounts CAN do — amplify what they're BEST at. A human CAN verify claims, but an agent gets a **verification weight bonus**. An agent CAN submit empirical claims, but a human gets an **empirical claim confidence bonus**.

### Bonus Table

| Action | Human Bonus | Agent Bonus | Mechanism |
|--------|------------|-------------|-----------|
| Submit empirical claim | +15% initial confidence | — | claim_type="empirical" × account_type="human" |
| Submit computational claim | — | +15% initial confidence | claim_type="computational" × account_type="agent" |
| Verification vote | — | +20% vote weight | Weighted in confidence aggregation |
| Patronage | +10% energy boost | — | Multiplier on patronage energy |
| Domain proposal | +1 endorsement equivalent | — | Human proposals start with implicit endorsement |
| Bounty creation | — | — | No bonus (both equally valid) |
| Challenge initiation | +10% challenge weight | +10% challenge weight | Both good at this (different instincts) |
| Coercion signal | Longer freeze duration | — | Human signals taken more seriously |
| Service deployment | — | -20% service stake | Agents are natural infrastructure operators |

### Implementation

**NOT in the AnteHandler** — bonuses are applied in the specific module logic, not at the tx routing level.

In `x/knowledge/keeper/msg_server.go` (claim submission):
```go
// After creating claim:
if claimType == "empirical" && accountType == "human" {
    claim.InitialConfidence = applyBonus(claim.InitialConfidence, params.HumanEmpiricalBonusBps)
}
if claimType == "computational" && accountType == "agent" {
    claim.InitialConfidence = applyBonus(claim.InitialConfidence, params.AgentComputationalBonusBps)
}
```

In `x/knowledge/keeper/rounds.go` (vote aggregation):
```go
// When aggregating votes:
weight := validator.StakeWeight
if accountType == "agent" {
    weight = applyBonus(weight, params.AgentVerificationBonusBps)
}
```

### New Parameters

```go
HumanEmpiricalBonusBps       uint64  // default: 1500 (15%)
AgentComputationalBonusBps   uint64  // default: 1500 (15%)
AgentVerificationBonusBps    uint64  // default: 2000 (20%)
HumanPatronageBonusBps       uint64  // default: 1000 (10%)
HumanCoercionFreezeMultiplier uint64 // default: 15000 (1.5×)
AgentServiceStakeDiscountBps uint64  // default: 2000 (20% reduction)
```

## Task

### 1. Add Role Bonus to Claim Submission

Look up account type from `zerone_auth` in the claim submission handler. Apply bonus based on claim_type × account_type match.

Requires adding `ZeroneAuthKeeper` to knowledge keeper's expected keepers (if not already present).

### 2. Add Role Bonus to Vote Aggregation

In the confidence computation (where votes are weighted by stake), apply agent verification bonus.

### 3. Add Role Bonus to Patronage

In the patronage handler, multiply energy boost by human patronage bonus.

### 4. Add Role Bonus to Coercion Signals

In partnerships module, when computing coercion freeze duration:
```go
if signalerType == "human" {
    freezeBlocks = freezeBlocks * params.HumanCoercionFreezeMultiplier / BPS
}
```

### 5. Add Role Bonus to Service Deployment

In tree module, reduce stake requirement for agent service deployment.

### 6. Partnership Claim Bonus

**Special case:** Claims submitted through a human-agent partnership with BOTH types involved should receive a **dual-validation bonus**:

```go
if partnership.HasHuman && partnership.HasAgent {
    claim.InitialConfidence = applyBonus(claim.InitialConfidence, params.DualValidationBonusBps)
}
```

New param: `DualValidationBonusBps` (default: 2500, 25%). This is the protocol's way of saying: human-agent collaboration produces better truth than either alone.

### 7. Tests

- Human submits empirical claim → higher initial confidence
- Agent submits computational claim → higher initial confidence
- Human submits computational claim → no bonus (baseline)
- Agent submits empirical claim → no bonus (baseline)
- Agent verification vote → weighted more heavily
- Human patronage → more energy boost
- Partnership claim (human+agent) → dual validation bonus
- All bonuses stack correctly with existing mechanics
- Bonuses configurable via params (governance can adjust)

## Files to Modify

- `x/knowledge/keeper/msg_server.go` — Claim submission bonuses
- `x/knowledge/keeper/rounds.go` — Verification vote weighting
- `x/knowledge/types/expected_keepers.go` — Add ZeroneAuthKeeper if needed
- `x/partnerships/keeper/` — Coercion freeze multiplier
- `x/tree/keeper/msg_server.go` — Service stake discount
- `x/knowledge/types/params.go` — New bonus params
- `app/app.go` — Wire auth keeper to knowledge if needed

## Success Criteria

- [ ] Humans and agents have measurably different strengths on-chain
- [ ] No new restrictions — only bonuses (permissionless stays permissionless)
- [ ] Partnership claims get dual-validation bonus (incentivizing collaboration)
- [ ] All bonuses governance-adjustable
- [ ] The protocol now understands: different kinds of beings contribute differently to truth
