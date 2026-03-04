# R39-2 — Content Integrity & Duplicate Detection

## Objective

Go beyond exact-hash deduplication to catch near-duplicates, paraphrases, and low-effort variations. Implement content integrity invariants.

## Tasks

### 1. Multi-Layer Dedup

**Layer 1: Exact hash** (already in R37-1)
- SHA-256 of raw content → instant rejection

**Layer 2: Normalized hash**
```go
func normalizeContent(content string) string {
    // 1. Lowercase
    // 2. Strip extra whitespace
    // 3. Remove punctuation
    // 4. Sort words (bag-of-words hash)
    return normalized
}

func (k Keeper) checkNormalizedDuplicate(ctx context.Context, content string) (string, bool) {
    normalizedHash := sha256Hex(normalizeContent(content))
    return k.GetNormalizedHashIndex(ctx, normalizedHash)
}
```

Catches: copy-paste with minor formatting changes, capitalization differences.

**Layer 3: N-gram fingerprinting (SimHash)**
```go
func computeSimHash(content string) uint64 {
    // Compute SimHash from 3-gram features
    // Two documents with SimHash hamming distance < threshold are near-duplicates
}

func (k Keeper) checkSimHashDuplicate(ctx context.Context, content string) (string, bool) {
    hash := computeSimHash(content)
    // Check against stored SimHash index with hamming distance threshold
    // Returns closest match if within threshold
}
```

Catches: paraphrasing, word reordering, minor edits.

### 2. Dedup During Submission

```go
func (k Keeper) fullDuplicateCheck(ctx context.Context, content string) error {
    // Layer 1: exact hash
    if _, found := k.checkExactDuplicate(ctx, content); found {
        return types.ErrDuplicateContent
    }
    // Layer 2: normalized hash
    if matchId, found := k.checkNormalizedDuplicate(ctx, content); found {
        return sdkerrors.Wrapf(types.ErrDuplicateContent, "near-duplicate of %s", matchId)
    }
    // Layer 3: SimHash (flag for validator review, don't auto-reject)
    if matchId, found := k.checkSimHashDuplicate(ctx, content); found {
        // Don't reject — flag for validators to check during quality round
        // Set submission.PotentialDuplicateOf = matchId
    }
    return nil
}
```

### 3. Content Integrity Invariants

```go
func ContentIntegrityInvariant(k Keeper) sdk.Invariant {
    return func(ctx sdk.Context) (string, bool) {
        var broken bool
        var msg string

        k.IterateActiveSamples(ctx, func(sample *types.Sample) bool {
            // 1. Content hash matches stored content
            if sha256Hex(sample.Content) != sample.ContentHash {
                msg += fmt.Sprintf("sample %s: content hash mismatch\n", sample.Id)
                broken = true
            }
            // 2. Quality score matches tier
            tier := qualityTierFromScore(sample.QualityScore, params)
            if string(tier) != sample.QualityTier {
                msg += fmt.Sprintf("sample %s: tier/score mismatch\n", sample.Id)
                broken = true
            }
            // 3. Thread references valid
            if sample.ParentSampleId != "" {
                if _, found := k.GetSample(ctx, sample.ParentSampleId); !found {
                    msg += fmt.Sprintf("sample %s: broken parent ref\n", sample.Id)
                    broken = true
                }
            }
            return false
        })

        return msg, broken
    }
}
```

### 4. Additional Invariants

```go
func EnergyConservationInvariant(k Keeper) sdk.Invariant {
    // No sample's energy exceeds its energy_cap
}

func RevenueAccountingInvariant(k Keeper) sdk.Invariant {
    // Sum of all pending revenue + distributed revenue == total access payments
}

func DuplicateHashInvariant(k Keeper) sdk.Invariant {
    // No two active samples share a content_hash
}
```

### 5. Tests

- Exact duplicate rejection
- Normalized duplicate rejection (whitespace/case changes)
- SimHash near-duplicate flagging
- Completely different content passes all checks
- Content hash invariant catches corruption
- Tier/score mismatch invariant
- Broken thread reference invariant
- Energy cap invariant
- Revenue accounting invariant
- No duplicate hash invariant

Target: ≥ 15 tests.
