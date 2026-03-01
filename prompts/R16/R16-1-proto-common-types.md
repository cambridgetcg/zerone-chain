# R16-1 — Proto Definitions & Common Types

## Objective

Update all proto definitions and generated Go code for the new revenue split.

## Changes Required

### 1. `proto/zerone/common/v1/common.proto`

**Already done** (verify):
```protobuf
message RevenueSplit {
  uint64 contributor_bps  = 1; // 550,000 (55%)
  uint64 protocol_bps     = 2; // 220,000 (22%)
  uint64 research_bps     = 3; //  33,300 (3.33%)
  uint64 development_bps  = 4; // 196,700 (19.67%)
  // Must sum to 1,000,000. No burn.
}
```

Field number 4 stays the same (was `burn_bps`). The proto wire format is unchanged — only the name differs. This is safe for forward/backward compatibility.

### 2. `proto/zerone/vesting_rewards/v1/state.proto`

In `RewardRouting` message:
- Rename `burn_amount` (field 6) → `development_amount`

In `BlockRewardDistribution` message:
- Rename `burn_amount` (field 8) → `development_amount`

### 3. `proto/zerone/vesting_rewards/v1/genesis.proto`

In `Params` message:
- Remove or deprecate `governance_activation_height` (field 8)
  - Option A: keep field number reserved, remove from logic
  - Option B: add comment "DEPRECATED — founder share is governance-immune"
- Verify `founder_share_bps` (field 6) and `founder_address` (field 7) remain

### 4. `proto/zerone/toolbox/v1/genesis.proto`

Toolbox has its own revenue split fields:
- Rename `burn_bps` (field 23) → `development_bps`

### 5. Regenerate all `*.pb.go` files

```bash
make proto-gen
```

Verify:
- `BurnBps` → `DevelopmentBps` in all generated types
- `BurnAmount` → `DevelopmentAmount` in RewardRouting / BlockRewardDistribution
- All `*.pb.go` and `*.pb.gw.go` files compile

### 6. `x/common/types/` — any hand-written helpers

Check for any utility functions that reference `BurnBps` in hand-written Go files (not generated).

## Verification

```bash
# After proto-gen, verify no old references in generated code
grep -rn "BurnBps\|burn_bps" --include="*.pb.go" | wc -l
# Should be 0

# Verify build
go build ./...
```

## Commit

```
R16-1: rename burn_bps → development_bps in proto definitions

Proto field 4 in RevenueSplit renamed from burn_bps to development_bps.
RewardRouting.burn_amount → development_amount.
BlockRewardDistribution.burn_amount → development_amount.
Toolbox genesis burn_bps → development_bps.
Wire format unchanged (same field numbers).
```
