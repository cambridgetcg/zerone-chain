# R7-2 — Autopoiesis Module: Self-Regulating Hormones

## Goal

Port x/autopoiesis — the chain's hormone system. Epoch-based multipliers that auto-adjust
economic parameters (block rewards, slashing severity, fee levels) based on ecosystem health
signals. Guardian/governance override capability.

## Working Directory

`/Users/yuai/Desktop/Zerone`

## Reference

- `/Users/yuai/Desktop/legible_money/x/autopoiesis/` — full module (4071 LOC keeper, 6 test files)
- `/Users/yuai/Desktop/legible_money/proto/legible/autopoiesis/v1/tx.proto` — tx proto
- Rename all `legible` → `zerone`, `ulgm` → `uzrn`, `LGM` → `ZRN`

## Proto Files

### `proto/zerone/autopoiesis/v1/types.proto`
```protobuf
syntax = "proto3";
package zerone.autopoiesis.v1;
option go_package = "github.com/zerone-chain/zerone/x/autopoiesis/types";

// MultiplierState tracks a single economic multiplier.
message MultiplierState {
  string path = 1;           // e.g. "rewards.block", "slashing.severity"
  uint64 current_bps = 2;    // current value in BPS (1M scale)
  uint64 target_bps = 3;     // target value
  uint64 min_bps = 4;        // floor
  uint64 max_bps = 5;        // ceiling
  bool frozen = 6;           // if true, cannot auto-adjust
  uint64 last_updated = 7;   // block height
}

// EpochSnapshot captures hormone readings at epoch boundary.
message EpochSnapshot {
  uint64 epoch = 1;
  uint64 block_height = 2;
  repeated MultiplierState multipliers = 3;
  uint64 ssi_score = 4;      // System Stability Index (0-1M BPS)
  string ssi_category = 5;   // "critical", "moderate", "low_moderate", "healthy"
}
```

### `proto/zerone/autopoiesis/v1/tx.proto`
Port from draft — 4 messages: UpdateParams, Activate, OverrideMultiplier, FreezeMultiplier.
All authority-gated.

### `proto/zerone/autopoiesis/v1/query.proto`
- QueryParams, QueryMultiplier (by path), QueryAllMultipliers, QueryEpochSnapshot, QuerySSI

### `proto/zerone/autopoiesis/v1/genesis.proto`
- GenesisState: params + multiplier states + epoch snapshots + activated flag

## Module Implementation

### Keeper
Port from draft keeper with these patterns:
- **Epoch processing** in EndBlocker: every `EpochLengthBlocks`, read all multipliers,
  compute SSI from cross-module signals, adjust multipliers toward targets bounded by
  `MaxChangePerEpochBps`, snapshot the epoch
- **SSI computation**: aggregate health signal from staking participation, knowledge verification
  rate, and emergency state. Categorize by threshold params
- **Multiplier adjustment**: linear interpolation toward target, clamped by min/max and
  max-change-per-epoch. Frozen multipliers skip adjustment
- **Override**: governance can directly set a multiplier value (OverrideMultiplier)
- **Freeze/unfreeze**: governance can freeze a multiplier to prevent auto-adjustment

### Expected Keepers
```go
type StakingKeeper interface {
    GetTotalStaked(ctx sdk.Context) math.Int
    GetActiveValidatorCount(ctx sdk.Context) int
}
type KnowledgeKeeper interface {
    GetVerificationRate(ctx sdk.Context) uint64
}
type EmergencyKeeper interface {
    IsHalted(ctx sdk.Context) bool
}
type GovKeeper interface {
    GetAuthority() string
}
```

### Default Params
- EpochLengthBlocks: 100
- MaxChangePerEpochBps: 10000 (1% of 1M scale)
- SlashMultiplierMin: 100000 (10%)
- SlashMultiplierMax: 900000 (90%)
- SSICriticalThreshold: 200000 (20%)
- SSIModerateThreshold: 400000 (40%)
- SSILowModerateThreshold: 600000 (60%)
- Enabled: true

### Default Multipliers (init at genesis)
- `rewards.block` — 500000 (50%, middle of range)
- `slashing.severity` — 500000
- `fees.base` — 500000

## Tests

Port all test files from draft. Key scenarios:
1. Epoch boundary triggers multiplier adjustment
2. SSI computation from cross-module signals
3. Multiplier respects min/max bounds and max-change-per-epoch
4. Frozen multiplier skips adjustment
5. Override sets value directly
6. Emergency halt disables auto-adjustment
7. Genesis import/export round-trip

## Constraints

- Use gogoproto for all proto generation (match existing modules)
- 1M BPS scale everywhere
- Wire to staking + knowledge + emergency keepers via interfaces
- Register in `app.go` module manager (add to BeginBlocker/EndBlocker order)
- Update `app.go` keeper field + wiring
