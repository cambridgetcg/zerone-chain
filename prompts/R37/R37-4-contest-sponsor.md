# R37-4 — Contest & Sponsor: Data Integrity

## Objective

Implement ContestSample (disputes), SponsorSample (preservation), and the re-validation flow. Adapted from ChallengeFact and PatronizeFact.

## Tasks

### 1. ContestSample

```go
func (k Keeper) ContestSample(ctx context.Context, msg *types.MsgContestSample) (*types.MsgContestSampleResponse, error) {
    // 1. Verify sample exists and is active (gold/silver/bronze)
    // 2. Lock challenger's stake
    // 3. Behavior depends on contest_type:
    //    CONSENT: Triggers consent re-review round
    //    QUALITY: Triggers full quality re-evaluation
    //    DUPLICATE: Check content_hash against index; if match, fast-track resolve
    //    TOXIC: Triggers toxicity-focused review
    //    COPYRIGHT: Flags for human review (governance proposal)
    // 4. Create new QualityRound (re-validation)
    // 5. Mark sample as CONTESTED
    // 6. Emit event
}
```

Contest outcomes:
- **Upheld** (sample stays): challenger loses stake, sample owner gets portion
- **Overturned** (sample removed): challenger gets reward, sample downgraded or pruned
- **Weakened** (quality re-scored lower): partial rewards both ways

### 2. SponsorSample

```go
func (k Keeper) SponsorSample(ctx context.Context, msg *types.MsgSponsorSample) (*types.MsgSponsorSampleResponse, error) {
    // 1. Verify sample exists
    // 2. Transfer amount from sponsor to module account
    // 3. Set/extend patronage_amount and patronage_expiry_block
    // 4. Restore energy to cap (sponsored samples don't decay)
    // 5. Emit event
}
```

Sponsored samples are immune to pruning while patronage is active.

### 3. Consent-Specific Contest Flow

When contest_type == CONSENT:
- Validators specifically evaluate consent proof validity
- Lower stake requirement (consent is a factual check, not subjective)
- If consent fails: sample immediately removed, submitter slashed
- Original content preserved in dispute record for audit trail

### 4. Duplicate Contest Fast Path

When contest_type == DUPLICATE:
- Challenger provides the ID of the alleged duplicate
- If content_hash matches: automatic resolution, no round needed
- If content_hash differs but content is semantically similar: triggers round
- Validators compare the two submissions for substantial similarity

### 5. Tests

- Contest for consent violation → upheld
- Contest for consent violation → overturned (consent was fine)
- Contest for quality → re-scoring
- Contest for duplicate → fast-path resolution
- Contest for toxicity
- Sponsor a sample → energy restored
- Sponsor extends existing patronage
- Sponsored sample immune to pruning
- Contest stake locking and return
- Cannot contest an already-contested sample
- Cannot contest a pruned sample

Target: ≥ 25 tests.
