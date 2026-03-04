# R36-3 — Proto Genesis & Params: Training Data Foundation

## Objective

Rewrite `genesis.proto` with new genesis state for the training data protocol. Replace 777 axioms with a seed dataset. Update `Params` for quality thresholds instead of epistemic params.

## Tasks

### 1. Rewrite GenesisState

```protobuf
message GenesisState {
  Params params                        = 1;
  repeated Sample samples              = 2;   // Seed samples
  repeated Submission submissions      = 3;   // Pending at export
  repeated QualityRound quality_rounds = 4;   // Active rounds at export
  repeated Domain domains              = 5;   // Seed domains
  repeated Dataset datasets            = 6;
  repeated TrainingDemand demands      = 7;
  repeated DataBounty bounties         = 8;
  repeated ScrapedSourceEntry scraped_sources = 9;
  repeated ValidatorInfo validators    = 10;
  uint64 next_sample_seq               = 11;
  uint64 next_submission_seq           = 12;
  uint64 next_round_seq                = 13;
  uint64 next_dataset_seq              = 14;
}
```

### 2. Rewrite Params

**Remove** epistemic params (stratum configs, fundamentality weights, etc.)

**New params:**

```protobuf
message Params {
  // ─── Submission ───
  string min_submission_stake         = 1;   // uzrn
  uint64 max_content_bytes            = 2;   // Max content size per submission
  uint64 max_thread_size              = 3;   // Max submissions per thread batch

  // ─── Quality validation ───
  uint64 commit_period_blocks         = 4;
  uint64 reveal_period_blocks         = 5;
  uint64 min_validators_per_round     = 6;
  uint64 max_validators_per_round     = 7;
  uint64 gold_threshold               = 8;   // Quality score >= this → gold (BPS)
  uint64 silver_threshold             = 9;   // >= this → silver
  uint64 bronze_threshold             = 10;  // >= this → bronze (below = reject)
  uint64 max_toxicity_threshold       = 11;  // Above this → auto-reject
  uint64 consent_required_weight      = 12;  // How much consent_valid matters (BPS)

  // ─── Slashing ───
  uint64 wrong_validation_slash_bps   = 13;
  uint64 missed_reveal_slash_bps      = 14;
  uint64 equivocation_slash_bps       = 15;

  // ─── Economics ───
  string submitter_revenue_share_bps  = 16;  // 5500 = 55%
  string validator_revenue_share_bps  = 17;  // 2200 = 22%
  uint64 gold_quality_multiplier      = 18;  // 30000 = 3x (in BPS/10000)
  uint64 silver_quality_multiplier    = 19;  // 20000 = 2x
  uint64 bronze_quality_multiplier    = 20;  // 10000 = 1x
  string access_fee_per_sample        = 21;  // Default uzrn per access

  // ─── Consent multipliers ───
  uint64 self_authored_multiplier     = 22;  // 15000 = 1.5x
  uint64 opt_in_multiplier            = 23;  // 13000 = 1.3x
  uint64 public_license_multiplier    = 24;  // 10000 = 1.0x
  uint64 platform_tos_multiplier      = 25;  // 8000 = 0.8x
  uint64 fair_use_multiplier          = 26;  // 5000 = 0.5x

  // ─── Ecology (kept from knowledge) ───
  uint64 energy_decay_rate            = 27;
  uint64 energy_per_access            = 28;
  uint64 prune_grace_epochs           = 29;
  uint64 niche_saturation_threshold   = 30;
  uint64 novelty_bonus_bps            = 31;

  // ─── Bounties ───
  uint64 auto_bounty_threshold        = 32;  // Demand count that triggers auto-bounty
  string auto_bounty_amount           = 33;  // uzrn per auto-bounty

  // ─── Research fund (kept) ───
  uint64 research_tax_bps              = 34;
  string research_fund_address         = 35;
  uint64 founder_share_bps            = 36;
  string founder_address              = 37;
  uint64 ai_operations_share_bps      = 38;
  string ai_operations_address        = 39;
}
```

### 3. Create Seed Dataset

Replace `genesis_axioms.json` with `genesis_seeds.json` — approximately 50-100 curated seed samples demonstrating what high-quality training data looks like:

Categories to include:
- 5 discussion threads (multi-turn, showing reasoning)
- 5 troubleshooting exchanges (problem → solution with reasoning)
- 5 debate samples (respectful disagreement with arguments)
- 5 explanation samples (expert explaining to novice)
- 5 Q&A pairs (substantive answers with context)

Each seed should be a realistic example with:
- Content (the actual text)
- SampleType
- Domain
- Source platform (attributed to "zerone-seed")
- Quality tier (all gold — they're curated)
- ConsentProof (self-authored by the protocol)
- Tags and language

Update `axiom_embed.go` → `seed_embed.go`:
```go
//go:embed genesis_seeds.json
var GenesisSeedsJSON []byte
```

### 4. Default Params

Create `DefaultParams()` with sensible defaults. Key values:
- `min_submission_stake`: "1000000uzrn" (1 ZRN)
- `max_content_bytes`: 50000 (50KB per submission)
- `gold_threshold`: 800000 (80%)
- `silver_threshold`: 600000 (60%)
- `bronze_threshold`: 400000 (40%)
- `max_toxicity_threshold`: 200000 (20% — above this auto-rejects)

### 5. Default Domains

Seed domains for the training data protocol:
- `technology` — programming, debugging, system design
- `science` — scientific discourse, explanations, debates
- `culture` — social, cultural, philosophical discussions
- `creative` — creative writing, humor, storytelling
- `business` — business strategy, economics, markets
- `education` — teaching, learning, tutoring exchanges
- `health` — medical discussions (with appropriate caveats)
- `politics` — political discourse, policy debate
- `general` — catch-all for unclassified discourse

## Verification

```bash
make proto-gen
go build ./x/knowledge/types/...  # Types compile
```
