# R28-4 Albedo: Knowledge Metabolism Refinement — Design

## Summary

Refine the knowledge fact lifecycle: fix patronage energy recovery, enforce confidence caps, rescale energy to 0–1M, add multi-level status thresholds, activate confidence growth, and add a metabolism dashboard query.

## Design Decisions

### 1. Energy System Rescaling (0–10K → 0–1M)

Keep the existing cost/income model. Rescale all energy params to match the fitness BPS scale (0–1,000,000). Multiply all defaults by 100:

| Param | Old | New |
|-------|-----|-----|
| MetabolismEnergyCap | 10,000 | 1,000,000 |
| MetabolismBaseCost | 100 | 10,000 |
| MetabolismEnergyPerQuery | 10 | 1,000 |
| MetabolismEnergyPerCitation | 50 | 5,000 |
| MetabolismEnergyPerPatronage | 200 | 20,000 |
| MetabolismEnergyChallengeSurvival | 1,000 | 100,000 |
| MetabolismInitialEnergy | (proportional) | (proportional) |

Add three new threshold params for multi-level status transitions:

| Param | Value | Meaning |
|-------|-------|---------|
| MetabolismActiveThreshold | 300,000 | 30% — below this → AT_RISK |
| MetabolismAtRiskThreshold | 100,000 | 10% — below this → EXTINCT-eligible |
| MetabolismExtinctionThreshold | 10,000 | 1% — immediate EXTINCT |

Status transitions become multi-level instead of binary (energy=0 → AT_RISK):
- Energy ≥ ActiveThreshold → ACTIVE/VERIFIED (healthy)
- Energy < ActiveThreshold → AT_RISK
- Energy < ExtinctionThreshold for N epochs → EXPIRED → PRUNED

### 2. Immediate Patronage Energy Recovery

In `MsgPatroniseFact` handler, after recording patronage:
- Calculate upfront energy boost: `MetabolismEnergyPerPatronage * durationEpochs / 10` (proportional to commitment)
- Add to fact.Energy, cap at MetabolismEnergyCap
- If fact was AT_RISK and energy now ≥ ActiveThreshold, transition to ACTIVE
- Emit `fact_status_changed` event with reason `patronage_recovery`

### 3. Confidence Cap: Root Cause + Hard Cap

**Root cause found:** `MsgAddFact` (governance fact injection in msg_server.go:424-464) directly assigns `msg.Confidence` with zero ceiling checks. The normal verification path (`createFactFromClaim` in rounds.go) correctly enforces the stratum ceiling, but the authority bypass skips all of it.

**Fix:**
1. Add stratum ceiling validation to `MsgAddFact` handler (same logic as `createFactFromClaim`)
2. Add `MaxConfidence` param (default 880,000) as a global hard cap
3. Apply `clampConfidence()` helper everywhere confidence is assigned:
   - `AggregateVerificationResult()` — after stratum ceiling
   - `createFactFromClaim()` — after setting confidence
   - `MsgAddFact` — after accepting authority confidence
   - Confidence growth (new, see below)

### 4. Confidence Growth Activation

Wire up the unused `ConfidenceGrowthEpochBlocks` and `ConfidenceGrowthPerEpochBps` params:
- At each fitness epoch, ACTIVE/VERIFIED facts grow confidence by `ConfidenceGrowthPerEpochBps` (default 11,000 = 1.1%)
- Always clamped by `MaxConfidence` hard cap
- AT_RISK/EXPIRED facts do not grow confidence

### 5. Unified Fact Lifecycle Events

Replace per-status events (`fact_at_risk`, `fact_expired`, `fact_pruned`, `fact_recovered`) with a single `fact_status_changed` event:

Attributes:
- `fact_id`, `old_status`, `new_status`, `energy`, `reason`

Reasons: `decay`, `patronage_recovery`, `challenge_degradation`, `challenge_survival`, `extinction`, `confidence_growth`

### 6. Satisfaction Feedback Loop

Design decision: **no additional satisfaction-based decay modifier**. Query income already rewards popular facts via `MetabolismEnergyPerQuery`. The fitness score's satisfaction weight (15%) handles visibility ranking. Adding a decay modifier would double-count popularity.

### 7. Metabolism Dashboard Query

New gRPC query `QueryMetabolismStatus`:

```json
{
    "total_facts": 800,
    "active": 750,
    "at_risk": 35,
    "expired": 10,
    "extinct": 5,
    "avg_energy": 650000,
    "current_epoch": 42,
    "next_epoch_block": 43000,
    "recent_recoveries": 3,
    "recent_extinctions": 1
}
```

CLI: `zeroned query knowledge metabolism-status`

### 8. Testnet Parameter Tuning

`FitnessEpochBlocks`: 1000 blocks (~100 min at 6s blocks). No separate `MetabolismEpochBlocks` needed — metabolism runs at the same fitness epoch boundary.

## Files to Modify

- `x/knowledge/types/params.go` — New params, rescaled defaults
- `x/knowledge/types/genesis.pb.go` — Proto-generated param fields
- `x/knowledge/keeper/msg_server.go` — Patronage energy recovery, MsgAddFact ceiling fix
- `x/knowledge/keeper/metabolism.go` — Multi-level thresholds, unified events
- `x/knowledge/keeper/confidence.go` — clampConfidence helper, growth logic
- `x/knowledge/keeper/rounds.go` — Apply clampConfidence
- `x/knowledge/keeper/fitness.go` — Confidence growth at epoch
- `x/knowledge/keeper/grpc_query.go` — Metabolism dashboard query
- `x/knowledge/client/cli/query.go` — CLI for metabolism-status
- `x/knowledge/keeper/metabolism_test.go` — Tests for all changes
