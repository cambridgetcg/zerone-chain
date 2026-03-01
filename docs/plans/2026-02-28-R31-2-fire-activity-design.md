# R31-2 Fire Activity: Verification as Transformation — Design

## Overview

R31-2 wires the Fire element (verification) into the Wu Xing circulation:
- **Fire → Earth:** Verification health feeds alignment's governance participation sensor
- **Water → Fire:** Partnership density adjusts verification requirements
- **Fire → Metal:** Verification activity accelerates capture defense reputation recovery

## Completion Index (Foundation)

All three connections need window-based round counting. A new store index supports this.

### Store Key

```
Prefix: 0x60
Key:    0x60 | verdictBlock (8 bytes big-endian) | roundID
Value:  CompletedRoundMeta proto {domain, has_dissent, duration_blocks}
```

Written in `CompleteRound()` via `indexCompletedRound(ctx, round)`.

### Proto Message

```proto
message CompletedRoundMeta {
  string domain          = 1;
  bool   has_dissent     = 2;
  uint64 duration_blocks = 3;
}
```

### Dissent Detection

A round has dissent if `round.Reveals` contain both "accept" and "reject" votes.

### Counting Methods

```go
CountCompletedRoundsInWindow(ctx, height, windowBlocks) uint64
CountDisputedRoundsInWindow(ctx, height, windowBlocks) uint64
GetAvgRoundDurationInWindow(ctx, height, windowBlocks) uint64
CountCompletedRoundsForDomainInWindow(ctx, domain, height, windowBlocks) uint64
```

All use range scan on `[height-window, height]` over the 0x60 prefix.

## Fire → Earth: Verification Health → Governance Sensor

### Knowledge Keeper

```go
GetVerificationHealth(ctx) (throughputBps, disputeRateBps, avgRoundDurationBlocks uint64)
```

- Throughput: `completed * BPS / (windowBlocks / commitPhaseBlocks)`
- Dispute rate: `disputed * BPS / completed`
- Window: new param `observation_window_blocks` (default 10,000)

### Alignment Sensor

Modify `senseGovernanceParticipation(ctx)`:
- Existing ontology domain count: 70% weight
- Verification health (throughput, penalized by extreme dispute rate): 30% weight
- Dispute rate > 300,000 BPS (30%): reduce verification health by 30%

### Event

```
zerone.alignment.verification_health_observed {
  throughput_bps, dispute_rate_bps, avg_round_duration
}
```

## Water → Fire: Partnership Density → Verification Requirements

### Knowledge Keeper

```go
GetEffectiveMinVerifiers(ctx, domain) uint32
```

- Density == 0: `base + 1` (no social structure → tighter)
- Density >= SocialSaturationThreshold: `base - 1` (min 2) (saturated → relaxed)
- Otherwise: base unchanged

### New Param

```proto
uint64 social_saturation_threshold = N; // default: 10
```

### Integration

Wire into round creation where `params.MinVerifiers` is currently used.

### Event

```
zerone.knowledge.social_verification_adjustment {
  domain, base_min_verifiers, effective_min_verifiers, partnership_density, reason
}
```

## Fire → Metal: Verification Activity → Reputation Recovery

### Knowledge Keeper

```go
GetDomainVerificationActivity(ctx, domain) uint64
```

- Count completed rounds for domain in 10,000-block window
- Normalize: 10 rounds = BPS (100%), capped at BPS

### Capture Defense

```go
calculateReputationRecoveryRate(ctx, domain) uint64
```

- Base: `BaseReputationRecoveryBps` (default 50,000 = 5%)
- Bonus: `activity * ActivityRecoveryBonusMaxBps / BPS` (max 50% acceleration)
- Final: `baseRate + (baseRate * activityBonus / BPS)`

### New Params

```proto
uint64 base_reputation_recovery_bps = N;       // default: 50,000
uint64 activity_recovery_bonus_max_bps = N;    // default: 500,000
```

### Event

```
zerone.capture_defense.activity_recovery_bonus {
  domain, verification_activity_bps, recovery_rate_bps, bonus_bps
}
```

## Interface Changes

### alignment/types/expected_keepers.go — KnowledgeKeeper

Add: `GetVerificationHealth(ctx context.Context) (uint64, uint64, uint64)`

### capture_defense/types/expected_keepers.go — KnowledgeKeeper

Add: `GetDomainVerificationActivity(ctx context.Context, domain string) uint64`

### knowledge/types/expected_keepers.go — PartnershipKeeper

Already has `GetDomainPartnershipDensity` — no change needed.

## Tests

1. Fire→Earth: High throughput → governance participation improves
2. Fire→Earth: Extreme dispute rate (>30%) → governance participation degrades
3. Water→Fire: Domain with 10+ partnerships → min verifiers reduced by 1
4. Water→Fire: Domain with 0 partnerships → min verifiers increased by 1
5. Fire→Metal: Active domain → reputation recovers faster
6. Fire→Metal: Inactive domain → reputation recovers at base rate only
7. Combined: All connections operating together

## Files Changed

| Module | Files | What |
|--------|-------|------|
| knowledge (proto) | genesis.proto, types.proto | New param + CompletedRoundMeta |
| knowledge (types) | keys.go, expected_keepers.go, params.go | New prefix, param default |
| knowledge (keeper) | verification_metrics.go (new) | Completion index + counting + interface methods |
| knowledge (keeper) | rounds.go | Call indexCompletedRound in CompleteRound |
| alignment (types) | expected_keepers.go | Extend KnowledgeKeeper |
| alignment (keeper) | sensors.go | Blend verification health |
| capture_defense (proto) | genesis.proto | 2 new params |
| capture_defense (types) | expected_keepers.go, params.go | Extend KnowledgeKeeper, param defaults |
| capture_defense (keeper) | reputation.go | Activity-based recovery |
| tests | cross_stack_fire_test.go | 7 test cases |
