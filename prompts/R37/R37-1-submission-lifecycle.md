# R37-1 — Submission Lifecycle

## Objective

Implement the keeper methods for submitting training data: `SubmitData`, `SubmitThread`, content hashing, duplicate detection, and consent validation.

## Tasks

### 1. MsgSubmitData Handler

```go
func (k Keeper) SubmitData(ctx context.Context, msg *types.MsgSubmitData) (*types.MsgSubmitDataResponse, error) {
    // 1. Validate content size against params.MaxContentBytes
    // 2. Compute content_hash = SHA-256(content)
    // 3. Check content_hash index for duplicates → ErrDuplicateContent
    // 4. Validate consent proof (see below)
    // 5. Validate domain exists and is active
    // 6. Lock stake from submitter
    // 7. Create Submission object
    // 8. Store submission + indexes (content_hash, submitter, domain)
    // 9. Check if any active DataBounty matches this submission
    // 10. Initiate quality round (VRF select validators, start commit phase)
    // 11. Emit event
}
```

### 2. MsgSubmitThread Handler

```go
func (k Keeper) SubmitThread(ctx context.Context, msg *types.MsgSubmitThread) (*types.MsgSubmitThreadResponse, error) {
    // 1. Validate thread size against params.MaxThreadSize
    // 2. Generate thread_id if not provided
    // 3. For each item in msg.Items:
    //    a. Set thread_id and thread position
    //    b. Call internal submitDataItem() (shared logic with SubmitData)
    // 4. Link submissions via parent_submission_id chain
    // 5. Lock total stake (single stake covers whole thread)
    // 6. Initiate ONE quality round for the entire thread
    // 7. Emit event with all submission IDs
}
```

### 3. Content Hash & Duplicate Detection

```go
func (k Keeper) computeContentHash(content string) string {
    h := sha256.Sum256([]byte(content))
    return hex.EncodeToString(h[:])
}

func (k Keeper) checkDuplicate(ctx context.Context, contentHash string) error {
    if k.HasContentHash(ctx, contentHash) {
        return types.ErrDuplicateContent
    }
    return nil
}
```

Store content_hash → submission_id in a dedicated index for O(1) dedup.

### 4. Consent Validation

```go
func (k Keeper) validateConsent(consent *types.ConsentProof) error {
    if consent == nil {
        return types.ErrConsentRequired
    }
    switch consent.Type {
    case types.CONSENT_TYPE_SELF_AUTHORED:
        // Strongest — submitter claims authorship. No additional proof needed.
        return nil
    case types.CONSENT_TYPE_OPT_IN:
        // Must have author_signature or proof_uri
        if consent.AuthorSignature == "" && consent.ProofUri == "" {
            return types.ErrInvalidConsent
        }
    case types.CONSENT_TYPE_PUBLIC_LICENSE:
        // Must have proof_uri pointing to license
        if consent.ProofUri == "" {
            return types.ErrInvalidConsent
        }
    case types.CONSENT_TYPE_PLATFORM_TOS:
        // Must have proof_uri
        if consent.ProofUri == "" {
            return types.ErrInvalidConsent
        }
    case types.CONSENT_TYPE_FAIR_USE:
        // Accepted but gets lowest multiplier
        return nil
    default:
        return types.ErrInvalidConsent
    }
    return nil
}
```

Note: Consent validation on-chain is necessarily limited — deeper verification happens during quality rounds (validators check consent proofs).

### 5. Sponsored Submissions

If `msg.Sponsored == true` and submitter can't cover the stake:
- Check bootstrap fund balance
- If sufficient, sponsor the review fee
- Track sponsored amount for potential claw-back on rejection

### 6. Store Operations

Implement all CRUD for Submission:
- `SetSubmission(ctx, submission)`
- `GetSubmission(ctx, id) → (Submission, bool)`
- `DeleteSubmission(ctx, id)`
- `SetContentHash(ctx, hash, submissionId)`
- `HasContentHash(ctx, hash) → bool`
- Iterator: `IterateSubmissions(ctx, fn)`
- Index: `GetSubmissionsByDomain(ctx, domain)`
- Index: `GetSubmissionsBySubmitter(ctx, submitter)`

### 7. Tests

Write tests for:
- Successful single submission
- Successful thread submission (3-5 items)
- Duplicate rejection
- Invalid consent rejection
- Content too large rejection
- Thread too large rejection
- Invalid domain rejection
- Stake locking
- Sponsored submission flow
- Event emissions

Target: ≥ 30 tests for this session.

## Constraints

- Reuse existing store patterns from the old knowledge keeper
- All amounts in uzrn as strings
- Content hash is lowercase hex SHA-256
- Thread IDs are `thread-{seq}` format
