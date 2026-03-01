# R14-4 — Port Autopoiesis, Alignment, and Evidence Management Tests

## Context

Three modules critical to self-regulation are severely under-tested:

| Module | Zerone | Prototype | Gap |
|--------|--------|-----------|-----|
| autopoiesis | 7 | 121 | 114 |
| evidence_mgmt | 12 | 70 | 58 |
| alignment | 9 | 48 | 39 |

**Autopoiesis** is the hormone system — it dynamically adjusts protocol behaviour. 7 tests for a self-modifying system is dangerously low.

**Evidence management** tracks audit trails and oracle submissions. Under-tested means potential evidence tampering.

**Alignment** has 5 sensors measuring ecosystem health. These feed into autopoiesis. Under-tested sensors produce garbage multipliers.

## Task

### Autopoiesis (target: ≥ 80 tests)

Port from `legible-money/x/autopoiesis/keeper/`:

1. **Hormone system** — multiplier calculation, decay, accumulation
2. **Activation/deactivation** — lifecycle, authorization checks
3. **Override mechanism** — governance override, freeze, unfreeze
4. **Multiplier dynamics** — bounds checking, rate limiting, oscillation dampening
5. **Cross-module effects** — how multipliers affect block rewards (via vesting_rewards)
6. **Security** — unauthorized override, multiplier manipulation, overflow

### Evidence Management (target: ≥ 55 tests)

Port from `legible-money/x/evidence_mgmt/keeper/`:

1. **Evidence submission** — create, validate, store
2. **Audit trails** — chain of evidence, tamper detection
3. **Oracle submissions** — external data ingestion, confidence scoring
4. **Evidence lifecycle** — expiry, archival, retrieval
5. **Security** — evidence forgery, backdating, duplicate submission

### Alignment (target: ≥ 35 tests)

Port from `legible-money/x/alignment/keeper/`:

1. **Sensor readings** — all 5 sensors produce valid scores
2. **Sensor fusion** — combined health score calculation
3. **Corrections** — when sensors detect misalignment
4. **Meta-loop** — alignment checking its own effectiveness
5. **Edge cases** — missing sensor data, sensor disagreement, extreme values

## Approach

- Read existing zerone tests first for each module
- Port in dependency order: evidence_mgmt → alignment → autopoiesis (alignment feeds autopoiesis)
- Adapt naming, denom, BPS scale
- Run module tests after each batch

## Verification

```bash
go test ./x/autopoiesis/... -count=1 -v
go test ./x/evidence_mgmt/... -count=1 -v
go test ./x/alignment/... -count=1 -v
go vet ./x/autopoiesis/... ./x/evidence_mgmt/... ./x/alignment/...
```

## Commit Convention

```
test(R14-4): port evidence_mgmt audit trail + oracle tests
test(R14-4): port alignment sensor + fusion tests
test(R14-4): port autopoiesis hormone system + security tests
```
