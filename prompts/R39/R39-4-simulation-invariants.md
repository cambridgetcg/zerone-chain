# R39-4 — Simulation & Adversarial Testing

## Objective

Create simulation operations for all new Msg types, register invariants, and write adversarial tests that try to break the training data protocol. This is the quality gate before the knowledge module is considered production-complete.

## Tasks

### 1. Simulation Operations

Create `x/knowledge/simulation/operations.go` with weighted random operations:

```go
func SimulateMsgSubmitData(ak, bk, k) simtypes.Operation {
    // Generate random content (lorem-style, 100-5000 chars)
    // Random domain from active domains
    // Random sample_type
    // Random consent type (weighted toward stronger types)
    // Random language from ["en", "es", "zh", "ja", "de", "fr", "ar"]
    // Random tags
}

func SimulateMsgSubmitThread(ak, bk, k) simtypes.Operation {
    // Generate 2-8 related messages
    // Shared thread_id
    // Alternating "speakers" (different accounts)
}

func SimulateMsgSubmitCommitment(ak, bk, k) simtypes.Operation {
    // Find active round in COMMIT phase
    // Generate random QualityVote, compute commit hash
}

func SimulateMsgSubmitReveal(ak, bk, k) simtypes.Operation {
    // Find active round in REVEAL phase where this validator committed
    // Reveal with stored vote + salt
}

func SimulateMsgContestSample(ak, bk, k) simtypes.Operation {
    // Pick random active sample
    // Random contest type
}

func SimulateMsgAccessSample(ak, bk, k) simtypes.Operation {
    // Pick random active sample
    // Consumer has enough funds
}

func SimulateMsgSponsorSample(ak, bk, k) simtypes.Operation {
    // Pick random sample, sponsor with random amount
}

func SimulateMsgRevokeConsent(ak, bk, k) simtypes.Operation {
    // Pick random sample where requester is submitter/author
}
```

### 2. Register All Invariants

```go
func RegisterInvariants(ir sdk.InvariantRegistry, k Keeper) {
    ir.RegisterRoute(types.ModuleName, "content-integrity", ContentIntegrityInvariant(k))
    ir.RegisterRoute(types.ModuleName, "energy-conservation", EnergyConservationInvariant(k))
    ir.RegisterRoute(types.ModuleName, "revenue-accounting", RevenueAccountingInvariant(k))
    ir.RegisterRoute(types.ModuleName, "no-duplicate-hash", NoDuplicateHashInvariant(k))
    ir.RegisterRoute(types.ModuleName, "thread-consistency", ThreadConsistencyInvariant(k))
    ir.RegisterRoute(types.ModuleName, "quality-tier-score", QualityTierScoreInvariant(k))
    ir.RegisterRoute(types.ModuleName, "stake-accounting", StakeAccountingInvariant(k))
}
```

### 3. Adversarial Tests

Write explicit adversarial scenarios:

```go
func TestAdversarial_SpamSubmissions(t *testing.T) {
    // Submit 1000 low-quality items rapidly
    // Verify: all properly queued, no state corruption, stakes locked
}

func TestAdversarial_CollusionAttempt(t *testing.T) {
    // Multiple validators submit identical quality scores
    // Verify: commit-reveal prevents copying (different salts produce different hashes)
}

func TestAdversarial_ConsentFraud(t *testing.T) {
    // Submit with fake consent proof
    // Verify: validators catch it during quality round
}

func TestAdversarial_DuplicateFlood(t *testing.T) {
    // Submit same content with minor variations
    // Verify: normalized hash catches variations, SimHash flags near-dupes
}

func TestAdversarial_RevenueManipulation(t *testing.T) {
    // Self-access own samples to inflate revenue
    // Verify: access fees are real cost (no free self-access)
}

func TestAdversarial_QualityInflation(t *testing.T) {
    // Validators consistently score everything as gold
    // Verify: outlier detection catches this over time, accuracy drops
}

func TestAdversarial_ConsentRevocationRace(t *testing.T) {
    // Revoke consent while quality round is in progress
    // Verify: round is cancelled, no sample created
}

func TestAdversarial_BountyGaming(t *testing.T) {
    // Create bounty then immediately fill with low-quality data
    // Verify: bounty only fulfilled by accepted (gold/silver/bronze) samples
}
```

### 4. Full Lifecycle Simulation

Run a 500-block simulation with all operations:

```go
func TestFullAppSimulation_TrainingData(t *testing.T) {
    config := simcli.NewConfigFromFlags()
    config.NumBlocks = 500
    config.BlockSize = 50

    // Run full simulation
    // All invariants checked after every operation
    // Verify no panics, no invariant breaks
}
```

### 5. Test Coverage Summary

After R39, the knowledge module should have:
- R37-1: ≥ 30 (submission lifecycle)
- R37-2: ≥ 50 (quality rounds)
- R37-3: ≥ 40 (ecology)
- R37-4: ≥ 25 (contest/sponsor)
- R37-5: ≥ 25 (domain/demand)
- R37-6: ≥ 30 (block processing)
- R38-1: ≥ 15 (datasets)
- R38-2: ≥ 15 (access/payment)
- R38-3: ≥ 15 (revenue)
- R38-4: ≥ 10 (consumer API)
- R39-1: ≥ 20 (consent)
- R39-2: ≥ 15 (integrity)
- R39-3: ≥ 15 (cross-module)
- R39-4: ≥ 20 (simulation/adversarial)
- **Total: ≥ 325 new tests**

Combined with adapted existing tests, target ≥ 400 total.

## Exit Criteria

1. 500-block simulation passes with zero invariant breaks
2. All 8 adversarial scenarios pass
3. All invariants registered and verified
4. `go test ./x/knowledge/...` — all pass
5. `go test ./...` — full chain compiles and passes
