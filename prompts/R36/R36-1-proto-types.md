# R36-1 — Proto Types: Training Data Model

## Objective

Rewrite `proto/zerone/knowledge/v1/types.proto` to replace the fact-claim knowledge graph with a training data protocol. The commit-reveal infrastructure stays — what changes is the domain model.

## Context

- **Design doc:** `docs/DESIGN-training-data-protocol.md`
- **Current proto:** `proto/zerone/knowledge/v1/types.proto`
- The module stays as `zerone.knowledge.v1` (no package rename — too much churn)

## Tasks

### 1. Replace Enums

**Remove:**
- `FactStatus`, `ClaimStatus`, `ClaimType`, `Verdict`, `RelationType`, `DomainStatus`

**Add:**

```protobuf
enum SampleType {
  SAMPLE_TYPE_UNSPECIFIED    = 0;
  SAMPLE_TYPE_DISCUSSION     = 1;   // Multi-party conversation
  SAMPLE_TYPE_DEBATE         = 2;   // Opposing viewpoints argued
  SAMPLE_TYPE_EXPLANATION    = 3;   // Someone explaining a concept
  SAMPLE_TYPE_TROUBLESHOOT   = 4;   // Problem → diagnosis → solution
  SAMPLE_TYPE_REVIEW         = 5;   // Evaluation/critique
  SAMPLE_TYPE_TUTORIAL       = 6;   // Step-by-step teaching
  SAMPLE_TYPE_OPINION        = 7;   // Reasoned opinion with arguments
  SAMPLE_TYPE_NARRATIVE      = 8;   // Storytelling, personal experience
  SAMPLE_TYPE_Q_AND_A        = 9;   // Question and answer(s)
  SAMPLE_TYPE_CREATIVE       = 10;  // Creative writing, poetry, humor
  SAMPLE_TYPE_ANNOTATION     = 11;  // Human annotation/labeling
  SAMPLE_TYPE_CORRECTION     = 12;  // Correcting misinformation
}

enum SampleStatus {
  SAMPLE_STATUS_UNSPECIFIED  = 0;
  SAMPLE_STATUS_PENDING      = 1;   // Awaiting quality validation
  SAMPLE_STATUS_IN_REVIEW    = 2;   // Quality round in progress
  SAMPLE_STATUS_GOLD         = 3;   // Exceptional quality
  SAMPLE_STATUS_SILVER       = 4;   // Good quality
  SAMPLE_STATUS_BRONZE       = 5;   // Acceptable quality
  SAMPLE_STATUS_REJECTED     = 6;   // Not useful
  SAMPLE_STATUS_CONTESTED    = 7;   // Under dispute
  SAMPLE_STATUS_EXPIRED      = 8;   // Energy depleted
  SAMPLE_STATUS_PRUNED       = 9;   // Removed from active set
}

enum SubmissionStatus {
  SUBMISSION_STATUS_UNSPECIFIED       = 0;
  SUBMISSION_STATUS_PENDING           = 1;
  SUBMISSION_STATUS_PENDING_REVIEW    = 2;
  SUBMISSION_STATUS_REVIEWED          = 3;
  SUBMISSION_STATUS_ACCEPTED          = 4;
  SUBMISSION_STATUS_REJECTED          = 5;
  SUBMISSION_STATUS_CONSENT_FAILED    = 6;
  SUBMISSION_STATUS_DUPLICATE         = 7;
}

enum QualityVerdict {
  QUALITY_VERDICT_UNSPECIFIED   = 0;
  QUALITY_VERDICT_GOLD          = 1;
  QUALITY_VERDICT_SILVER        = 2;
  QUALITY_VERDICT_BRONZE        = 3;
  QUALITY_VERDICT_REJECT        = 4;
  QUALITY_VERDICT_CONSENT_FAIL  = 5;
}

enum ConsentType {
  CONSENT_TYPE_UNSPECIFIED     = 0;
  CONSENT_TYPE_SELF_AUTHORED   = 1;  // Submitter is the author (strongest)
  CONSENT_TYPE_OPT_IN          = 2;  // Author explicitly opted in
  CONSENT_TYPE_PUBLIC_LICENSE   = 3;  // CC-BY or similar open license
  CONSENT_TYPE_PLATFORM_TOS    = 4;  // Platform TOS permits this use
  CONSENT_TYPE_FAIR_USE        = 5;  // Fair use claim (weakest)
}

// Keep VerificationPhase — identical mechanism, different context
// Keep DomainStatus — domains still exist
```

### 2. Replace Core Messages

**Remove:** `Fact`, `Claim`, `ClaimStructure`, `ClaimRelation`, `FactRelation`, `CommonKnowledgeEntry`

**Add:**

```protobuf
message ConsentProof {
  ConsentType type           = 1;
  string proof_uri           = 2;   // Link to consent evidence
  string author_signature    = 3;   // Optional cryptographic consent
  uint64 consent_timestamp   = 4;   // Unix timestamp
  string consent_terms       = 5;   // What was consented to
}

message Submission {
  string id                     = 1;
  string submitter              = 2;
  string content                = 3;
  SampleType sample_type        = 4;
  string domain                 = 5;
  string source_uri             = 6;
  string source_platform        = 7;
  uint64 source_timestamp       = 8;
  string parent_submission_id   = 9;
  repeated string context_ids   = 10;
  string thread_id              = 11;
  ConsentProof consent          = 12;
  string original_author        = 13;
  string license                = 14;
  repeated string tags          = 15;
  string language               = 16;
  string stake                  = 17;
  uint64 submitted_at_block     = 18;
  SubmissionStatus status       = 19;
  string content_hash           = 20;
  string quality_round_id       = 21;
  bool   sponsored              = 22;
}

message Sample {
  string id                     = 1;
  string content                = 2;
  SampleType sample_type        = 3;
  string domain                 = 4;
  string source_uri             = 5;
  string source_platform        = 6;
  uint64 source_timestamp       = 7;

  // Quality
  uint64 quality_score          = 8;
  string quality_tier           = 9;
  uint64 novelty_score          = 10;
  uint64 diversity_score        = 11;
  uint64 reasoning_depth        = 12;

  // Provenance
  string submitter              = 13;
  string original_author        = 14;
  ConsentProof consent          = 15;
  string license                = 16;
  string submission_id          = 17;

  // Thread context
  string thread_id              = 18;
  string parent_sample_id       = 19;
  repeated string child_sample_ids = 20;
  uint64 thread_position        = 21;
  uint64 thread_depth           = 22;

  // Economics
  uint64 access_count           = 23;
  string total_revenue          = 24;
  string patronage_amount       = 25;
  uint64 patronage_expiry_block = 26;

  // Lifecycle
  SampleStatus status           = 27;
  uint64 verified_at_block      = 28;
  uint64 last_accessed_block    = 29;

  // Fitness / ecology (kept from knowledge)
  uint64 fitness_score          = 30;
  uint64 fitness_updated_block  = 31;
  uint64 energy                 = 32;
  uint64 energy_cap             = 33;
  uint64 energy_last_updated    = 34;
  uint64 at_risk_since_epoch    = 35;

  // Tags & search
  repeated string tags          = 36;
  string language               = 37;
  repeated string topics        = 38;

  // Niche dynamics (adapted)
  string niche_key              = 39;
  bool   niche_leader           = 40;
  uint64 niche_rank             = 41;
  uint64 niche_size             = 42;
  uint64 competition_tax        = 43;
}

message QualityVote {
  uint64 overall_quality   = 1;
  uint64 reasoning_depth   = 2;
  uint64 novelty           = 3;
  uint64 toxicity          = 4;
  uint64 factual_accuracy  = 5;
  bool   consent_valid     = 6;
  bool   duplicate         = 7;
  string notes             = 8;
}
```

### 3. Adapt Kept Messages

**VerificationRound** → rename to `QualityRound`:
- Same structure, same commit-reveal
- `Verdict` field → `QualityVerdict`
- Add `QualityVote aggregate_scores = 13;` for aggregated quality dimensions

**RevealEntry** → update:
- `vote` string changes from "accept"/"reject" to serialized `QualityVote`

**Domain** → keep as-is but update the description to note these are data domains, not epistemic domains.

**ValidatorInfo** → keep (validators still have tiers and accuracy).

**DemandSignal** → rename to `TrainingDemand`:
- Add `SampleType preferred_type`, `string language`, `uint64 bounty_pool`

**KnowledgeBounty** → rename to `DataBounty`:
- Same structure, different semantics

**VRFProof** → keep unchanged.

### 4. New Messages

```protobuf
// ScrapedSourceEntry replaces CommonKnowledgeEntry
// Tracks sources already heavily scraped by AI labs (reduces novelty score)
message ScrapedSourceEntry {
  string id              = 1;
  string platform        = 2;  // "reddit", "stackoverflow", etc.
  string domain          = 3;
  string description     = 4;
  uint64 novelty_penalty = 5;  // 0-1,000,000
  uint64 added_block     = 6;
}

// Dataset — curated collection for bulk access
message Dataset {
  string id              = 1;
  string name            = 2;
  string description     = 3;
  string domain          = 4;
  string license         = 5;
  uint64 sample_count    = 6;
  uint64 total_tokens    = 7;
  string price_per_sample = 8;
  string bulk_price      = 9;
  string curator         = 10;
  repeated string filter_tags = 11;
  SampleType filter_type = 12;
  string filter_language = 13;
  uint64 min_quality     = 14;  // Minimum quality_score for inclusion
  uint64 created_at_block = 15;
  uint64 updated_at_block = 16;
}
```

## Constraints

- Package stays `zerone.knowledge.v1` — don't rename (too much downstream churn)
- All scores use 0-1,000,000 BPS scale (protocol convention)
- Keep field numbering clean (no gaps in new messages)
- All `string` amounts are uzrn coin strings
- `content_hash` is SHA-256 hex of content field

## Verification

```bash
make proto-gen  # Must succeed
go build ./x/knowledge/...  # Will fail (keeper refs old types) — that's expected at this stage
```
