# R37-2 — Quality Round Lifecycle

## Objective

Adapt the commit-reveal verification mechanism from truth verification to quality scoring. The mechanism is structurally identical — validators commit blinded quality scores, then reveal them. Aggregation produces a multi-dimensional quality verdict.

## Tasks

### 1. Initiate Quality Round

When a submission enters review:

```go
func (k Keeper) initiateQualityRound(ctx context.Context, submissionID string, threadID string) error {
    // 1. Select validators via VRF (same as before)
    // 2. Create QualityRound:
    //    - Set commit_deadline = current_block + params.CommitPeriodBlocks
    //    - Set reveal_deadline = commit_deadline + params.RevealPeriodBlocks
    //    - Phase = COMMIT
    // 3. Store round, link to submission
    // 4. If thread: link round to all submissions in thread
    // 5. Emit QualityRoundStarted event
}
```

### 2. Commit Phase (same as before)

```go
func (k Keeper) SubmitCommitment(ctx context.Context, msg *types.MsgSubmitCommitment) error {
    // Identical to old SubmitCommitment:
    // 1. Verify sender is selected validator for this round
    // 2. Verify round is in COMMIT phase
    // 3. Verify deadline not passed
    // 4. Store commit_hash
}
```

### 3. Reveal Phase (adapted)

```go
func (k Keeper) SubmitReveal(ctx context.Context, msg *types.MsgSubmitReveal) error {
    // 1. Verify sender is selected validator
    // 2. Verify round is in REVEAL phase
    // 3. Verify deadline not passed
    // 4. Verify commit: SHA-256(quality_vote_bytes || salt) == stored commit_hash
    // 5. Validate QualityVote scores are in BPS range (0-1,000,000)
    // 6. Store revealed QualityVote
}
```

**Key change:** The reveal contains a `QualityVote` with multiple dimensions instead of a single "accept"/"reject" string.

### 4. Aggregation (new logic)

```go
func (k Keeper) aggregateQualityRound(ctx context.Context, round *types.QualityRound) error {
    // 1. Collect all revealed QualityVotes
    // 2. For each dimension, compute weighted median:
    //    - overall_quality: weighted by validator tier
    //    - reasoning_depth: weighted by validator tier
    //    - novelty: weighted by validator tier
    //    - toxicity: weighted by validator tier
    //    - factual_accuracy: weighted by validator tier
    // 3. Compute consent consensus: majority vote on consent_valid
    // 4. Compute duplicate consensus: majority vote on duplicate
    // 5. Determine QualityVerdict:
    //    - If consent consensus = false → CONSENT_FAIL
    //    - If duplicate consensus = true → map to REJECT
    //    - If toxicity > params.MaxToxicityThreshold → REJECT
    //    - If overall_quality >= params.GoldThreshold → GOLD
    //    - If overall_quality >= params.SilverThreshold → SILVER
    //    - If overall_quality >= params.BronzeThreshold → BRONZE
    //    - Else → REJECT
    // 6. Score validators (reward consensus, slash outliers)
    // 7. If accepted (gold/silver/bronze): create Sample from Submission
    // 8. If rejected: return stake minus slash to submitter
    // 9. Update round with verdict
    // 10. Emit event
}
```

### 5. Sample Creation

```go
func (k Keeper) createSampleFromSubmission(
    ctx context.Context,
    submission *types.Submission,
    verdict types.QualityVerdict,
    scores *types.QualityVote,
) (*types.Sample, error) {
    sample := &types.Sample{
        Id:              k.nextSampleSeq(ctx),
        Content:         submission.Content,
        SampleType:      submission.SampleType,
        Domain:          submission.Domain,
        SourceUri:       submission.SourceUri,
        SourcePlatform:  submission.SourcePlatform,
        SourceTimestamp:  submission.SourceTimestamp,
        QualityScore:    scores.OverallQuality,
        QualityTier:     verdictToTier(verdict),
        NoveltyScore:    scores.Novelty,
        ReasoningDepth:  scores.ReasoningDepth,
        Submitter:       submission.Submitter,
        OriginalAuthor:  submission.OriginalAuthor,
        Consent:         submission.Consent,
        License:         submission.License,
        SubmissionId:    submission.Id,
        ThreadId:        submission.ThreadId,
        Status:          tierToStatus(verdict),
        VerifiedAtBlock: sdkCtx.BlockHeight(),
        Energy:          params.DefaultEnergyCap,  // Start full
        EnergyCap:       params.DefaultEnergyCap,
        // ... compute niche_key, set initial fitness
    }
    // Store sample + all indexes
    // Link thread relationships
    // Check and fulfill any matching bounties
    // Distribute submitter reward (stake return + creation bonus)
}
```

### 6. Validator Scoring

After aggregation, score validators on quality assessment accuracy:

```go
func (k Keeper) scoreValidators(ctx context.Context, round *types.QualityRound, aggregated *types.QualityVote) {
    for _, reveal := range round.Reveals {
        // Compute distance from validator's scores to aggregated scores
        // Weighted by dimension importance
        deviation := computeDeviation(reveal.Scores, aggregated)
        if deviation > outlierThreshold {
            // Slash for being far from consensus
            k.slashValidator(ctx, reveal.Verifier, params.WrongValidationSlashBps)
        } else {
            // Reward for accurate assessment
            k.rewardValidator(ctx, reveal.Verifier, round)
        }
    }
}
```

### 7. Tests

- Quality round initiation
- Commit phase (valid commit, duplicate commit, wrong validator, expired deadline)
- Reveal phase (valid reveal, hash mismatch, wrong phase, expired deadline)
- Aggregation with unanimous gold verdict
- Aggregation with mixed scores → silver
- Aggregation with reject consensus
- Consent fail overrides quality
- Duplicate detection overrides quality
- Toxicity threshold rejection
- Sample creation from successful round
- Thread quality round (multiple submissions, one round)
- Validator slashing for outlier scores
- Validator rewards for consensus
- Missed reveal slashing

Target: ≥ 50 tests.

## Key Difference from Old System

The old system had binary votes ("accept"/"reject") with commit-reveal to prevent copying. The new system has multi-dimensional scores — but commit-reveal is even MORE important here, because quality scoring is inherently more subjective. Validators must independently assess quality without seeing others' scores.

The aggregation uses **weighted median** (not mean) per dimension to resist manipulation by extreme scores.
