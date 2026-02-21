# R7-3 — Alignment Module: Ecosystem Health Sensing

## Goal

Port x/alignment — the chain's sensor fusion system. Monitors ecosystem health across
multiple dimensions (knowledge quality, economic stability, governance participation,
network security), computes a composite Alignment Health Index (AHI), and generates
correction signals that feed into autopoiesis.

## Working Directory

`/Users/yuai/Desktop/Zerone`

## Reference

- `/Users/yuai/Desktop/legible_money/x/alignment/` — full module (2480 LOC keeper, 2 test files)
- No proto files in draft (types defined in Go) — create protos for Zerone (proto-first principle)
- Rename all `legible` → `zerone`, `ulgm` → `uzrn`

## Proto Files

### `proto/zerone/alignment/v1/types.proto`
```protobuf
syntax = "proto3";
package zerone.alignment.v1;
option go_package = "github.com/zerone-chain/zerone/x/alignment/types";

import "gogoproto/gogo.proto";

// AlignmentState tracks the module's activation and current health.
message AlignmentState {
  bool active = 1;
  uint64 current_epoch = 2;
  uint64 last_observation_block = 3;
}

// AlignmentObservation captures raw sensor readings at an epoch.
message AlignmentObservation {
  uint64 epoch = 1;
  uint64 block_height = 2;
  uint64 knowledge_verification_rate = 3;   // BPS: verified / total claims
  uint64 economic_stability = 4;            // BPS: price volatility inverse
  uint64 governance_participation = 5;      // BPS: voting power participating
  uint64 network_security = 6;             // BPS: active validators / total
  uint64 staking_ratio = 7;                // BPS: staked / total supply
}

// DimensionScores holds weighted scores per dimension.
message DimensionScores {
  uint64 epoch = 1;
  uint64 knowledge = 2;     // weighted score BPS
  uint64 economic = 3;
  uint64 governance = 4;
  uint64 security = 5;
  uint64 staking = 6;
  uint64 composite_ahi = 7; // Alignment Health Index (weighted average)
}

// CorrectionRecord logs a correction signal sent to autopoiesis.
message CorrectionRecord {
  string dimension = 1;
  string direction = 2;   // "increase", "decrease", "stable"
  uint64 magnitude = 3;   // BPS magnitude of correction
  string target_path = 4; // autopoiesis multiplier path affected
}

// AlignmentHealthIndex is the meta-health summary.
message AlignmentHealthIndex {
  uint64 epoch = 1;
  uint64 ahi = 2;           // 0-1M BPS composite score
  string category = 3;     // "critical", "degraded", "healthy", "optimal"
  repeated CorrectionRecord corrections = 4;
}
```

### `proto/zerone/alignment/v1/tx.proto`
- MsgUpdateParams (authority-gated): dimension weights, observation interval, thresholds
- MsgActivate (authority-gated): enable/disable alignment monitoring

### `proto/zerone/alignment/v1/query.proto`
- QueryParams, QueryState, QueryObservation (by epoch), QueryScores (by epoch),
  QueryHealthIndex (latest), QueryCorrectionHistory (paginated)

### `proto/zerone/alignment/v1/genesis.proto`
- GenesisState: params + state + observations + scores + health indices

## Module Implementation

### Keeper
Port from draft keeper (sensors.go, corrections.go, meta.go):
- **Sensors** — each dimension has a sensor function that reads from cross-module keepers:
  - Knowledge: `knowledgeKeeper.GetVerificationRate()`
  - Economic: derive from liquidity pool TWAP stability
  - Governance: `govKeeper.GetVotingParticipation()` or count active proposals
  - Security: active validators / target validator count
  - Staking: total staked / total supply
- **Scoring** — weighted average of dimension scores. Weights from params (default equal: 200k each = 1M total)
- **Corrections** — when a dimension drops below threshold, generate correction signal:
  - Low knowledge → increase rewards.block multiplier
  - Low security → increase slashing.severity
  - Low governance → no automatic correction (log only)
- **EndBlocker** — every `ObservationIntervalBlocks`, run observation → scoring → corrections → snapshot

### Expected Keepers
```go
type KnowledgeKeeper interface {
    GetVerificationRate(ctx sdk.Context) uint64
    GetTotalFacts(ctx sdk.Context) uint64
}
type StakingKeeper interface {
    GetTotalStaked(ctx sdk.Context) math.Int
    GetActiveValidatorCount(ctx sdk.Context) int
    GetTargetValidatorCount(ctx sdk.Context) int
}
type OntologyKeeper interface {
    GetDomainCount(ctx sdk.Context) uint64
}
type AutopoiesisKeeper interface {
    GetMultiplier(ctx sdk.Context, path string) (uint64, error)
    SuggestAdjustment(ctx sdk.Context, path string, direction string, magnitude uint64) error
}
type EmergencyKeeper interface {
    IsHalted(ctx sdk.Context) bool
}
type VestingRewardsKeeper interface {
    GetTotalSupply(ctx sdk.Context) math.Int
}
```

### Default Params
- ObservationIntervalBlocks: 100 (same as autopoiesis epoch)
- DimensionWeights: [200000, 200000, 200000, 200000, 200000] (equal, sum = 1M)
- CriticalThreshold: 200000 (20%)
- DegradedThreshold: 400000 (40%)
- HealthyThreshold: 700000 (70%)
- Enabled: true

## Tests

Port from draft + add:
1. Sensor readings from mock keepers produce correct observations
2. Weighted scoring computes correct AHI
3. Correction signals generated when dimensions below threshold
4. No corrections when all dimensions healthy
5. Emergency halt disables observations
6. Genesis import/export round-trip
7. Different weight configurations change composite score

## Constraints

- Proto-first (draft had no protos — create them fresh)
- Use gogoproto for generation (match existing modules)
- 1M BPS scale for all scores and weights
- Cross-module keeper interfaces only (no direct imports)
- Wire in app.go — add to EndBlocker AFTER autopoiesis
- Alignment reads, autopoiesis writes — alignment suggests, autopoiesis decides
