# Consensus Diversity Scoring — Design Document

_R28-2: Nigredo — You cannot transform what you cannot see._

## Problem

The verification system has no metric for conformity. Validators tend to vote unanimously, but the chain cannot measure this. Without measurement, the shadow stays hidden.

## Solution: Verification Diversity Index

A per-domain, per-epoch metric tracking how diverse verification votes are. Unanimous agreement on everything is a red flag — it means either the claims are trivially true or nobody is actually evaluating.

## Key Design Decision: Raw Vote Counts

Diversity entropy uses **raw headcounts** (1 validator = 1 signal), not stake-weighted votes. The purpose is to detect independent minds, not economic weight. Stake weighting the verdict is correct for sybil resistance; stake weighting the diversity measurement defeats the purpose.

Capital concentration is already measured by HHI in capture defense.

## Architecture

### Data Flow

```
Round completes → computeRoundEntropy(acceptCount, rejectCount)
                  → store entropy in round result
                  → update validator independence counters
                  → store domain round tracking

Epoch boundary  → aggregateDomainDiversity(domain, epoch)
(BeginBlocker)    → compute mean entropy across rounds
                  → check conformity streaks
                  → emit alerts if threshold breached

Alignment tick  → senseKnowledgeQuality()
                  → reads verification rate (60%) + diversity (40%)
```

### Epoch Cadence

Reuses the existing `FitnessEpochBlocks` boundary in `BeginBlocker`. No separate diversity epoch — diversity aggregation runs in the same epoch tick as fitness, competition, and metabolism. It's a cheap operation that belongs in the same "aggregate recent data" pass.

## Metrics

### 1. Vote Entropy (per round)

Shannon entropy for binary votes on BPS scale (0–1,000,000):

```
entropy = -(p_accept * log2(p_accept) + p_reject * log2(p_reject)) / BPS
```

Where `p_accept = acceptCount * BPS / total`, `p_reject = rejectCount * BPS / total`.

- Entropy = 0: unanimous (all same vote) — minimum diversity
- Entropy = BPS: perfect split (50/50) — maximum diversity

**Fixed-point log2**: Lookup table with ~20 interpolation points. Deterministic, no floating point, consensus-safe.

### 2. Domain Consensus Diversity (per epoch)

Average entropy across all completed rounds in a domain over the epoch.

Stored as `DomainDiversityScore{domain, epoch, avgEntropy, roundCount, unanimousCount}`.

### 3. Validator Independence Score (per validator)

How often a validator's vote differs from the majority verdict:

```
independence = minorityVotes * BPS / totalVotes
```

- Independence = 0: always votes with majority (follower)
- Independence > 300,000: frequently dissents
- Healthy range: 50,000–200,000

Updated on round completion. Rolling window = last epoch (reset and carry forward at epoch boundary).

### 4. Conformity Alerts

When domain diversity drops below threshold for consecutive epochs, emit `EventConformityAlert`.

**Threshold note for small validator sets**: With 4 validators and binary votes, the possible entropy values are sparse — 4-0 gives 0, 3-1 gives ~811K, 2-2 gives 1M BPS. There is no gradual spectrum. A single dissenter jumps from 0 to ~81% entropy. The conformity alert on small sets effectively detects "did at least one validator dissent in any round this epoch?" — still useful, but the threshold must be low enough to avoid false alerts. Default is 50,000 BPS (5%). The threshold becomes more meaningful as the validator set grows.

## Storage Layout

| Prefix | Key | Value |
|--------|-----|-------|
| `0x40` | `roundID` | `RoundDiversity{entropy, acceptCount, rejectCount, totalVoters}` |
| `0x41` | `domain/epoch` | `DomainDiversityScore{avgEntropy, roundCount, unanimousCount}` |
| `0x42` | `validatorAddr` | `ValidatorIndependence{totalVotes, minorityVotes, lastEpoch}` |
| `0x43` | `domain` | `ConformityStreak{consecutiveEpochs, lastEpoch}` |
| `0x44` | `domain/epoch` | `roundID` index (domain epoch round membership) |

## Alignment Integration

Extend `KnowledgeKeeper` interface with `GetConsensusDiversity(ctx) uint64`. The adapter reads the most recent global diversity score (average across all domains with activity).

`senseKnowledgeQuality` becomes:

```go
func (k Keeper) senseKnowledgeQuality(ctx context.Context) uint64 {
    rate := k.knowledgeKeeper.GetVerificationRate(ctx)
    diversity := k.knowledgeKeeper.GetConsensusDiversity(ctx)
    // Weighted: 60% verification rate, 40% diversity
    return (rate*6 + diversity*4) / 10
}
```

A system that verifies everything unanimously scores LOWER on knowledge quality, not higher.

## Parameters

Two new params (reuse `FitnessEpochBlocks` for epoch cadence):

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `ConformityAlertThreshold` | uint64 (BPS) | 50,000 (5%) | Domain avg entropy below this triggers streak increment |
| `ConformityAlertEpochs` | uint64 | 3 | Consecutive low-diversity epochs before alert |

## CLI Queries

- `query knowledge domain-diversity [domain]` — current epoch diversity
- `query knowledge domain-diversity-history [domain] [epochs]` — historical
- `query knowledge validator-independence [validator]` — independence score
- `query knowledge conformity-alerts` — active conformity alerts

## Files

| File | Action |
|------|--------|
| `x/knowledge/keeper/diversity.go` | New — entropy computation, aggregation, independence, alerts |
| `x/knowledge/keeper/diversity_test.go` | New — full test suite |
| `x/knowledge/types/diversity.go` | New — diversity Go structs |
| `x/knowledge/types/keys.go` | Modify — add 5 new prefix constants |
| `x/knowledge/types/genesis.go` | Modify — add 2 new params with defaults + validation |
| `x/knowledge/keeper/rounds.go` | Modify — call diversity hooks in CompleteRound |
| `x/knowledge/keeper/state.go` | Modify — add diversity store methods |
| `x/knowledge/keeper/phases.go` | Modify — add diversity aggregation to epoch boundary |
| `x/knowledge/keeper/alignment_adapters.go` | Modify — implement GetConsensusDiversity |
| `x/alignment/types/expected_keepers.go` | Modify — add GetConsensusDiversity to interface |
| `x/alignment/keeper/sensors.go` | Modify — update senseKnowledgeQuality |
| `x/knowledge/client/cli/query.go` | Modify — add 4 new query commands |

## Test Cases

- Unanimous round → entropy = 0
- Split round (50/50) → entropy = BPS (maximum)
- 80/20 split → entropy between 0 and BPS
- Domain with all unanimous rounds → low diversity → conformity alert after 3 epochs
- Domain with mixed rounds → healthy diversity → no alert
- Validator who always agrees → independence = 0
- Validator who dissents 15% → independence = 150,000 (healthy)
- Alignment senseKnowledgeQuality incorporates diversity (60/40 weighted)
- Conformity streak resets when diversity recovers
- Edge: 0 voters → entropy = 0, no crash
- Edge: 1 voter → entropy = 0 (unanimous by definition)
