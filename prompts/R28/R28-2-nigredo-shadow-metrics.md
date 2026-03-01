# R28-2 — Nigredo: Consensus Diversity Scoring

_You cannot transform what you cannot see._

## The Problem

There is no way to measure how much conformity exists in the verification system. We know from R25 that validators tend to vote unanimously, but the chain has no metric for this. Without measurement, the shadow stays hidden.

## The Fix: Verification Diversity Index

A per-domain, per-epoch metric that tracks how diverse verification votes are. Unanimous agreement on everything is a red flag — it means either the claims are trivially true or nobody is actually evaluating.

### Metrics

**1. Vote Entropy (per round)**

For a verification round with N voters:
```
entropy = -Σ(p_i × log2(p_i))
```
Where `p_accept = count_accept / N` and `p_reject = count_reject / N`.

- Entropy = 0: unanimous (all same vote) — minimum diversity
- Entropy = 1: perfect split (50/50) — maximum diversity
- Healthy range: 0.2-0.8 (some disagreement is normal and good)

**2. Domain Consensus Diversity (per epoch)**

Average entropy across all rounds in a domain over the epoch. Tracks whether a domain has healthy debate or rubber-stamp verification.

**3. Validator Independence Score (per validator)**

How often a validator's vote differs from the final majority, over a rolling window:
```
independence = minority_votes / total_votes
```

- Independence = 0: always votes with majority (follower)
- Independence > 0.3: frequently dissents (independent thinker OR bad evaluator)
- Healthy range: 0.05-0.20

**4. Conformity Alert**

When domain consensus diversity drops below a threshold for consecutive epochs, emit a `conformity_alert` event. This feeds into `x/alignment` as a knowledge quality signal.

### Integration with Alignment Module

The alignment module's `senseKnowledgeQuality` currently reads a simple verification rate. Extend it to incorporate consensus diversity:

```go
func (k Keeper) senseKnowledgeQuality(ctx context.Context) uint64 {
    rate := k.knowledgeKeeper.GetVerificationRate(ctx)
    diversity := k.knowledgeKeeper.GetConsensusDiversity(ctx)
    // Weighted: 60% verification rate, 40% diversity
    return (rate * 6 + diversity * 4) / 10
}
```

A system that verifies everything unanimously should score LOWER on knowledge quality, not higher.

## Task

### 1. Add Diversity Computation to Round Completion

After a round completes and votes are tallied:

```go
func (k Keeper) computeRoundEntropy(acceptCount, rejectCount uint64) uint64 {
    total := acceptCount + rejectCount
    if total == 0 || acceptCount == 0 || rejectCount == 0 {
        return 0 // unanimous = 0 entropy
    }
    // Fixed-point entropy calculation (BPS scale)
    pAccept := acceptCount * BPS / total
    pReject := rejectCount * BPS / total
    // Shannon entropy in BPS (max = BPS = 1.0)
    entropy := -(fixedLog2(pAccept) * pAccept + fixedLog2(pReject) * pReject) / BPS
    return entropy
}
```

Store per-round entropy with the round result.

### 2. Aggregate Domain Diversity per Epoch

In the metabolism EndBlocker (or a new diversity EndBlocker):
- Aggregate round entropies per domain for the current epoch
- Compute mean domain consensus diversity
- Store `DomainDiversityScore{domain, epoch, avgEntropy, roundCount, unanimousCount}`

### 3. Compute Validator Independence Scores

On round completion, update per-validator counters:
- `total_votes++`
- If in minority: `minority_votes++`
- Store rolling window (last N rounds or last epoch)

New query: `query knowledge validator-independence [validator-addr]`

### 4. Conformity Alerts

In the diversity EndBlocker:
```go
if domainDiversity < params.ConformityAlertThreshold {
    consecutiveCount := k.IncrementConformityStreak(ctx, domain)
    if consecutiveCount >= params.ConformityAlertEpochs {
        k.EmitConformityAlert(ctx, domain, diversity, consecutiveCount)
    }
} else {
    k.ResetConformityStreak(ctx, domain)
}
```

### 5. Feed into Alignment

Add `GetConsensusDiversity(ctx) uint64` to the knowledge keeper's interface used by alignment. Update `senseKnowledgeQuality` to incorporate diversity.

### 6. Query CLI

- `query knowledge domain-diversity [domain]` — current epoch diversity
- `query knowledge domain-diversity-history [domain] [epochs]` — historical
- `query knowledge validator-independence [validator]` — independence score
- `query knowledge conformity-alerts` — active conformity alerts

### 7. Tests

- Unanimous round → entropy = 0
- Split round (50/50) → entropy = BPS (maximum)
- 80/20 split → entropy between 0 and BPS
- Domain with all unanimous rounds → low diversity score → conformity alert
- Domain with mixed rounds → healthy diversity score → no alert
- Validator who always agrees → independence = 0
- Validator who dissents 15% → independence = 1500 (healthy)
- Alignment senseKnowledgeQuality incorporates diversity

## New Parameters

```go
ConformityAlertThreshold uint64  // BPS, default: 1000 (10% entropy = very uniform)
ConformityAlertEpochs    uint64  // consecutive epochs, default: 3
DiversityEpochBlocks     uint64  // same as metabolism epoch or independent, default: 1000
```

## Files to Modify

- `x/knowledge/keeper/rounds.go` — Compute + store round entropy
- `x/knowledge/keeper/` — New file: `diversity.go` for aggregation + alerts
- `x/knowledge/keeper/state.go` — Store methods for diversity scores, independence, streaks
- `x/knowledge/module.go` — Diversity aggregation in EndBlocker
- `x/knowledge/types/` — New proto types + params
- `x/knowledge/client/cli/query.go` — New queries
- `x/alignment/keeper/sensors.go` — Update senseKnowledgeQuality

## Success Criteria

- [ ] Every round stores its vote entropy
- [ ] Domain diversity aggregated per epoch
- [ ] Validator independence scores tracked
- [ ] Conformity alerts emitted when diversity drops
- [ ] Alignment module incorporates diversity into knowledge quality
- [ ] The shadow is now visible: we can SEE when the system is conforming
