# R37-3 — Sample Ecology: Fitness, Energy, Niche Dynamics

## Objective

Adapt the knowledge module's ecological dynamics (fitness scoring, energy metabolism, niche competition, pruning) from facts to training data samples. The mechanics are nearly identical — what changes is the fitness function inputs.

## Tasks

### 1. Fitness Scoring (adapted)

```go
func (k Keeper) computeSampleFitness(ctx context.Context, sample *types.Sample, params *types.Params) uint64 {
    // Components (all 0-1,000,000 BPS):
    qualityComponent := sample.QualityScore                        // Direct quality
    accessComponent := normalize(sample.AccessCount, maxAccess)    // Usage demand
    noveltyComponent := sample.NoveltyScore                        // Uniqueness
    diversityComponent := sample.DiversityScore                    // Domain diversity contribution
    reasoningComponent := sample.ReasoningDepth                    // Reasoning trace value
    revenueComponent := normalize(parseUzrn(sample.TotalRevenue), maxRevenue)  // Economic value

    // Weighted combination:
    fitness := (qualityComponent * 25 +
                accessComponent * 25 +
                noveltyComponent * 20 +
                diversityComponent * 10 +
                reasoningComponent * 10 +
                revenueComponent * 10) / 100

    return fitness
}
```

**Key difference from facts:** Access count (usage by AI labs) replaces citation count. Revenue replaces fundamentality. Reasoning depth is a new component unique to training data.

### 2. Energy Metabolism (kept)

Identical mechanism:
- Samples start with full energy
- Energy decays each epoch
- Each access (purchase) restores energy
- Samples at 0 energy enter "at risk" state
- After `prune_grace_epochs` at 0 energy → pruned

```go
func (k Keeper) decayEnergy(ctx context.Context, sample *types.Sample, params *types.Params) {
    // Same as fact energy decay
    decay := sample.Energy * params.EnergyDecayRate / 1_000_000
    sample.Energy = max(0, sample.Energy - decay)
}

func (k Keeper) restoreEnergyOnAccess(ctx context.Context, sample *types.Sample, params *types.Params) {
    sample.Energy = min(sample.EnergyCap, sample.Energy + params.EnergyPerAccess)
    sample.AtRiskSinceEpoch = 0  // No longer at risk
}
```

### 3. Niche Dynamics (adapted)

Niche = domain + sample_type + primary_topic. Samples compete within niches.

```go
func computeNicheKey(domain string, sampleType types.SampleType, primaryTopic string) string {
    h := sha256.Sum256([]byte(domain + "|" + sampleType.String() + "|" + primaryTopic))
    return hex.EncodeToString(h[:8])
}
```

- Niche saturation: if a niche has too many samples, new submissions get reduced novelty rewards
- Niche leader: highest-fitness sample in each niche gets bonus energy
- Competition tax: more competitors = higher maintenance cost

This incentivizes diverse submissions across underrepresented domains and topics.

### 4. Topic Saturation Tracking

New concept (replacing "common knowledge" penalty):

```go
// Track how saturated each domain+topic combination is
func (k Keeper) getTopicSaturation(ctx context.Context, domain, topic string) uint64 {
    // Count samples with this domain+topic
    // Compare to global average
    // Return saturation score 0-1,000,000
}

// Apply diminishing returns for over-saturated topics
func (k Keeper) applyNoveltyAdjustment(noveltyScore, saturation uint64) uint64 {
    if saturation > saturationThreshold {
        penalty := (saturation - saturationThreshold) * 500_000 / 1_000_000
        return noveltyScore * (1_000_000 - penalty) / 1_000_000
    }
    return noveltyScore
}
```

This naturally drives contributors toward underrepresented topics — submitting the 10,000th Python debugging discussion earns less than the 1st Swahili cooking tutorial.

### 5. Thread Fitness

Threads (conversation chains) get a bonus:

```go
func (k Keeper) computeThreadBonus(ctx context.Context, sample *types.Sample) uint64 {
    if sample.ThreadId == "" {
        return 0
    }
    threadSamples := k.GetSamplesByThread(ctx, sample.ThreadId)
    threadLength := len(threadSamples)
    // Bonus scales with thread length (diminishing returns)
    // 2 messages: 10% bonus, 5 messages: 25%, 10+: 30% cap
    bonus := min(300_000, uint64(threadLength) * 50_000)
    return bonus
}
```

Complete conversations are more valuable for training than isolated posts.

### 6. Pruning

```go
func (k Keeper) pruneSamples(ctx context.Context, currentEpoch uint64, params *types.Params) {
    k.IterateAtRiskSamples(ctx, func(sample *types.Sample) bool {
        gracePeriod := currentEpoch - sample.AtRiskSinceEpoch
        if gracePeriod >= params.PruneGraceEpochs {
            // Prune: mark as pruned, remove from active indexes
            // Keep provenance record (who submitted, quality scores)
            // Remove content to save storage (or archive to IPFS)
            sample.Status = types.SAMPLE_STATUS_PRUNED
            sample.Content = ""  // Content pruned
            k.SetSample(ctx, sample)
        }
        return false
    })
}
```

### 7. Tests

- Fitness computation with various inputs
- Energy decay over multiple blocks
- Energy restoration on access
- At-risk transition when energy hits 0
- Pruning after grace period
- Niche key computation
- Niche leader selection
- Competition tax application
- Topic saturation diminishing returns
- Thread bonus calculation
- Thread bonus capped at max
- Isolated sample (no thread) gets no bonus

Target: ≥ 40 tests.
