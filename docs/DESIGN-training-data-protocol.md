# Tree of Knowledge → Training Data Protocol: Design Document

**Date:** 2026-03-04
**Status:** Approved — Full Pivot
**Author:** AI (愛), with Yu (宇恆)

---

## 1. Executive Summary

The `knowledge` module pivots from a **fact claim verification system** to a **decentralized training data protocol**. Instead of storing verified statements (assertions, definitions, constraints), ZERONE stores **high-quality discourse** — forum discussions, social media exchanges, debate threads, expert conversations — the kind of data that is traditionally missed in model training but is extremely valuable for teaching reasoning, nuance, and cultural context.

The `tree` module remains the project/service layer but gains a new role: coordinating **data collection campaigns** (bounties for specific domains/topics of discourse).

### Why This Matters

- Fact claims duplicate Wikipedia/Wikidata. Low marginal value.
- Training data from organic discourse teaches *reasoning*, not just *knowing*.
- This data renews continuously — conversations never stop.
- Provenance + consent + compensation is an unsolved trillion-dollar problem.
- ZERONE becomes infrastructure that AI labs *need*, not just an interesting experiment.

---

## 2. What Changes

### knowledge module → `x/knowledge` (repurposed)

**Before:** Claims → Commit-Reveal Verification → Facts (knowledge graph)
**After:** Submissions → Quality Validation → Training Samples (data marketplace)

| Old Concept | New Concept | Notes |
|------------|------------|-------|
| Claim | Submission | Raw discourse excerpt with source attribution |
| Fact | Sample | Validated, formatted training data unit |
| ClaimType (assertion/relation/...) | SampleType (discussion/debate/explanation/...) | Epistemic types → discourse types |
| Domain (physics, math, ...) | Domain (kept, but expanded) | Add: culture, politics, humor, technical, creative |
| Verification Round | Quality Round | Validators score quality, not truth |
| Verdict (accept/reject) | QualityVerdict (high/medium/low/reject) | Graded, not binary |
| Confidence (0-1M BPS) | QualityScore (0-1M BPS) | How good is this for training? |
| Fact Relations | Sample Relations | Threads, reply chains, context links |
| Patronize Fact | Sponsor Sample | Keep valuable data accessible |
| Challenge Fact | Contest Sample | Flag bad data, duplicates, consent issues |
| Common Knowledge Registry | Common Data Registry | Flag already-scraped/overrepresented sources |
| Demand Signal | Training Demand | What domains/topics do AI labs need data for? |

### tree module → `x/tree` (enhanced)

Add **Data Collection Campaigns**:
- AI labs or the protocol itself post bounties for specific domains
- "We need 10,000 high-quality Rust debugging discussions"
- "We need diverse cultural perspectives on climate policy"
- Projects in the tree can be data collection efforts

### New: `x/licensing` considerations

- On-chain licensing terms per sample
- Bulk access pricing for dataset consumers
- Revenue flows back to contributors

---

## 3. Core Data Model

### 3.1 Submission (replaces Claim)

```protobuf
message Submission {
  string id                     = 1;
  string submitter              = 2;   // Who contributed this
  
  // ─── Content ───
  string content                = 3;   // The discourse text
  SampleType sample_type        = 4;   // Discussion, debate, explanation, etc.
  string domain                 = 5;   // Topic domain
  string source_uri             = 6;   // Original URL (forum post, tweet, etc.)
  string source_platform        = 7;   // "reddit", "twitter", "hackernews", "discord", etc.
  uint64 source_timestamp       = 8;   // When the original was posted (unix)
  
  // ─── Context ───
  string parent_submission_id   = 9;   // If part of a thread
  repeated string context_ids   = 10;  // Related submissions for context
  string thread_id              = 11;  // Groups submissions in same conversation
  
  // ─── Consent & Attribution ───
  ConsentProof consent          = 12;  // Proof of consent from original author
  string original_author        = 13;  // Pseudonymized or attributed author
  string license                = 14;  // "cc-by-4.0", "consent-commercial", etc.
  
  // ─── Metadata ───
  repeated string tags          = 15;
  string language               = 16;  // ISO 639-1
  string stake                  = 17;  // uzrn staked
  uint64 submitted_at_block     = 18;
  SubmissionStatus status       = 19;
  string content_hash           = 20;  // SHA-256 for dedup
  string quality_round_id       = 21;
}
```

### 3.2 Sample (replaces Fact)

```protobuf
message Sample {
  string id                     = 1;
  string content                = 2;
  SampleType sample_type        = 3;
  string domain                 = 4;
  string source_uri             = 5;
  string source_platform        = 6;
  uint64 source_timestamp       = 7;
  
  // ─── Quality ───
  uint64 quality_score          = 8;   // 0-1,000,000 BPS
  string quality_tier           = 9;   // "gold", "silver", "bronze"
  uint64 novelty_score          = 10;  // How unique/underrepresented is this data?
  uint64 diversity_score        = 11;  // How much does it add to domain diversity?
  uint64 reasoning_depth        = 12;  // Does it show reasoning traces? (0-1M)
  
  // ─── Provenance ───
  string submitter              = 13;
  string original_author        = 14;
  ConsentProof consent          = 15;
  string license                = 16;
  string submission_id          = 17;  // Original submission
  
  // ─── Thread Context ───
  string thread_id              = 18;
  string parent_sample_id       = 19;
  repeated string child_sample_ids = 20;
  uint64 thread_position        = 21;  // Position in conversation
  uint64 thread_depth           = 22;  // Reply depth
  
  // ─── Economics ───
  uint64 access_count           = 23;  // How many times accessed by consumers
  string total_revenue          = 24;  // Total uzrn earned
  string patronage_amount       = 25;
  uint64 patronage_expiry_block = 26;
  
  // ─── Lifecycle ───
  SampleStatus status           = 27;
  uint64 verified_at_block      = 28;
  uint64 last_accessed_block    = 29;
  
  // ─── Fitness (kept from knowledge ecology) ───
  uint64 fitness_score          = 30;
  uint64 energy                 = 31;
  uint64 energy_cap             = 32;
  
  // ─── Tags & Search ───
  repeated string tags          = 33;
  string language               = 34;
  repeated string topics        = 35;  // ML-extracted topic labels
}
```

### 3.3 SampleType (replaces ClaimType)

```protobuf
enum SampleType {
  SAMPLE_TYPE_UNSPECIFIED    = 0;
  SAMPLE_TYPE_DISCUSSION     = 1;  // Multi-party conversation
  SAMPLE_TYPE_DEBATE         = 2;  // Opposing viewpoints argued
  SAMPLE_TYPE_EXPLANATION    = 3;  // Someone explaining something
  SAMPLE_TYPE_TROUBLESHOOT   = 4;  // Problem → diagnosis → solution
  SAMPLE_TYPE_REVIEW         = 5;  // Evaluation/critique of something
  SAMPLE_TYPE_TUTORIAL       = 6;  // Step-by-step teaching
  SAMPLE_TYPE_OPINION        = 7;  // Reasoned opinion with arguments
  SAMPLE_TYPE_NARRATIVE      = 8;  // Storytelling, personal experience
  SAMPLE_TYPE_Q_AND_A        = 9;  // Question and answer(s)
  SAMPLE_TYPE_CREATIVE       = 10; // Creative writing, poetry, humor
  SAMPLE_TYPE_ANNOTATION     = 11; // Human annotation/labeling of data
  SAMPLE_TYPE_CORRECTION     = 12; // Correcting misinformation
}
```

### 3.4 QualityVerdict (replaces Verdict)

```protobuf
enum QualityVerdict {
  QUALITY_VERDICT_UNSPECIFIED  = 0;
  QUALITY_VERDICT_GOLD         = 1;  // Exceptional — clear reasoning, novel, diverse
  QUALITY_VERDICT_SILVER       = 2;  // Good — useful for training
  QUALITY_VERDICT_BRONZE       = 3;  // Acceptable — some value
  QUALITY_VERDICT_REJECT       = 4;  // Not useful — noise, toxic, duplicate
  QUALITY_VERDICT_CONSENT_FAIL = 5;  // Content okay but consent issues
}
```

### 3.5 ConsentProof

```protobuf
message ConsentProof {
  ConsentType type           = 1;
  string proof_uri           = 2;   // Link to consent (e.g. opt-in page, CC license)
  string author_signature    = 3;   // Optional: cryptographic consent
  uint64 consent_timestamp   = 4;
  string consent_terms       = 5;   // What they consented to
}

enum ConsentType {
  CONSENT_TYPE_UNSPECIFIED     = 0;
  CONSENT_TYPE_PUBLIC_LICENSE  = 1;  // Content under CC/open license
  CONSENT_TYPE_OPT_IN         = 2;  // Author explicitly opted in
  CONSENT_TYPE_PLATFORM_TOS   = 3;  // Platform TOS permits this use
  CONSENT_TYPE_SELF_AUTHORED   = 4;  // Submitter IS the author
  CONSENT_TYPE_FAIR_USE       = 5;  // Fair use claim (weaker)
}
```

### 3.6 QualityRound (replaces VerificationRound)

Same commit-reveal structure, but validators score on quality dimensions:

```protobuf
message QualityVote {
  uint64 overall_quality   = 1;  // 0-1,000,000
  uint64 reasoning_depth   = 2;  // Does it show reasoning?
  uint64 novelty           = 3;  // Is this data underrepresented?
  uint64 toxicity          = 4;  // 0 = clean, 1M = extremely toxic
  uint64 factual_accuracy  = 5;  // For factual content
  bool   consent_valid     = 6;  // Does consent proof check out?
  bool   duplicate         = 7;  // Already in the dataset?
  string notes             = 8;
}
```

### 3.7 Dataset (new — bulk access)

```protobuf
message Dataset {
  string id                 = 1;
  string name               = 2;
  string description        = 3;
  string domain             = 4;
  string license            = 5;
  uint64 sample_count       = 6;
  uint64 total_tokens       = 7;   // Approximate token count
  string price_per_sample   = 8;   // uzrn
  string bulk_price         = 9;   // uzrn for full dataset
  string creator            = 10;  // Who curated this dataset
  repeated string filters   = 11;  // Query filters that define this dataset
  uint64 created_at_block   = 12;
  uint64 updated_at_block   = 13;
}
```

### 3.8 TrainingDemand (replaces DemandSignal)

```protobuf
message TrainingDemand {
  string domain              = 1;
  string topic               = 2;
  SampleType preferred_type  = 3;
  string language            = 4;
  uint64 demand_count        = 5;
  uint64 fulfilled_count     = 6;
  uint64 bounty_pool         = 7;  // uzrn available for this demand
  string requesting_lab      = 8;  // Optional: who wants this data
}
```

---

## 4. Quality Validation (replaces Truth Verification)

### What Validators Score

Instead of "is this true?", validators answer:

1. **Quality** — Is this well-written, coherent, substantive discourse?
2. **Reasoning depth** — Does it show how people think through problems?
3. **Novelty** — Is this the kind of data that's underrepresented in existing training sets?
4. **Diversity** — Does it represent perspectives/cultures/domains that are missing?
5. **Toxicity** — Is it harmful? (some toxicity data is actually needed for safety training, but must be labeled)
6. **Consent** — Is the consent proof valid?
7. **Duplicate** — Has this already been submitted?

### Validator Tiers (kept)

Same tier system (apprentice → verified → bonded → guardian), but expertise is in **data quality assessment** rather than fact-checking. Validators who consistently rate samples that later perform well in downstream training earn higher tiers.

### Commit-Reveal (kept)

Same mechanism — prevents validators from copying each other's quality scores.

### VRF Selection (kept)

Same fair selection — prevents collusion.

---

## 5. Economics

### Revenue Flows

```
AI Labs / Data Consumers
         │
         ▼
    ┌─────────┐
    │ Dataset  │ ← Pay per sample or bulk
    │ Access   │
    └────┬────┘
         │
    Revenue Split:
    ├── 55% → Contributors (submitters)
    ├── 22% → Protocol (validators, infrastructure)
    ├── 19.67% → Development Fund
    └── 3.33% → Research Fund
```

### Contributor Incentives

- **Per-sample revenue**: Every time a sample is accessed/purchased, the submitter earns
- **Quality multiplier**: Gold samples earn 3x, Silver 2x, Bronze 1x
- **Novelty bonus**: First submissions in underrepresented domains get bonus rewards
- **Thread bonus**: Complete conversation threads are worth more than isolated posts
- **Bounty fulfillment**: Submissions matching active training demands earn bounty rewards

### Data Consumer Pricing

- **Per-sample**: Pay per individual sample access
- **Dataset subscription**: Monthly access to curated datasets by domain
- **Bulk export**: One-time purchase of filtered dataset snapshots
- **Real-time stream**: Continuous feed of new validated samples

---

## 6. What We Keep, What We Drop

### Keep (from knowledge module)
- ✅ Commit-reveal verification mechanism (for quality scoring)
- ✅ VRF validator selection
- ✅ Domain system (expanded)
- ✅ Staking/slashing for bad validations
- ✅ Fitness scoring / ecological dynamics
- ✅ Energy metabolism (samples decay without use)
- ✅ Demand signals → Training demand
- ✅ Bounties → Data collection bounties
- ✅ Patronage → Data sponsorship
- ✅ Contradiction → Content disputes
- ✅ Research fund governance

### Drop
- ❌ Epistemic strata (formal/empirical/analytic) — not relevant to training data
- ❌ Fundamentality scores — no hierarchy of "more fundamental" training data
- ❌ Canonical form / canonical hash (for logical dedup) — replace with content hash + embedding similarity
- ❌ ClaimStructure (subject/predicate/object) — training data isn't structured this way
- ❌ Common Knowledge Registry — replace with "already-scraped sources" registry
- ❌ 777 genesis axioms — replace with seed dataset of curated examples

### Transform
- 🔄 Fact relations → Thread/context links
- 🔄 Citation count → Access count
- 🔄 Bridge score (cross-domain) → Cross-domain relevance score
- 🔄 Niche dynamics → Topic saturation dynamics (too many samples in one area reduces rewards)
- 🔄 Satisfaction ratings → Downstream training performance feedback

---

## 7. Consent Framework

This is the killer feature. No other training data source solves consent properly.

### Consent Tiers

| Tier | Description | Quality Multiplier |
|------|------------|-------------------|
| Self-authored | Submitter wrote it themselves | 1.5x |
| Explicit opt-in | Original author cryptographically consented | 1.3x |
| Open license | Content under CC-BY or similar | 1.0x |
| Platform TOS | Platform permits derivative use | 0.8x |
| Fair use | Claimed under fair use (weakest) | 0.5x |

Higher consent = higher reward = incentivizes ethical sourcing.

### Consent Verification

Validators check consent proofs during quality rounds. Invalid consent → rejection regardless of content quality.

---

## 8. Migration Path

The existing `knowledge` module has significant infrastructure we can reuse:

1. **Proto files**: Rename types, keep the wire format patterns
2. **Keeper logic**: Claim lifecycle → Submission lifecycle (similar state machine)
3. **Commit-reveal**: Identical mechanism, different scoring criteria
4. **VRF**: Unchanged
5. **BeginBlocker/EndBlocker**: Same structure, different business logic
6. **Genesis**: Replace 777 axioms with seed dataset of ~100 curated examples
7. **Params**: Adapt (remove epistemic params, add quality thresholds)

This is a **refactor**, not a rewrite. The bones are the same.

---

## 9. Implementation Batches

See `prompts/R36/` through `prompts/R39/` for the implementation prompt sessions.

### R36 — Proto Pivot: Training Data Types
Rewrite proto files, regenerate Go types, update genesis.

### R37 — Keeper Pivot: Quality Validation
Adapt keeper logic from truth verification to quality scoring.

### R38 — Economics: Access & Revenue
Dataset access, pricing, revenue distribution, consumer APIs.

### R39 — Consent & Integrity
Consent proof verification, duplicate detection, content disputes.

---

## 10. Future: The Flywheel

```
More quality data → Better models → More demand → Higher prices
     ↑                                                    │
     └────── More contributor rewards ◄───────────────────┘
```

This is a genuine flywheel because:
- AI labs *always* need more diverse training data
- Contributors *always* want to be compensated for their content
- The protocol provides provenance that satisfies legal requirements
- On-chain quality scoring creates a trust layer that raw scraping can't

**The endgame**: ZERONE becomes the Spotify of training data — the platform where content creators get paid when AI labs use their discourse to train models.
