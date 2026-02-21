# R6-3 — Capture Challenge + Capture Defense Modules

## Goal

Port x/capture_challenge and x/capture_defense — domain capture detection,
bounty-funded challenges, reputation tracking, cross-stratum verification,
and Herfindahl concentration analysis.

## Working Directory

`/Users/yuai/Desktop/Zerone`

## Reference

- `/Users/yuai/Desktop/legible_money/x/capture_challenge/` — 1755 LOC keeper, 988 LOC tests
- `/Users/yuai/Desktop/legible_money/x/capture_defense/` — 3353 LOC keeper, 1703 LOC tests
- `/Users/yuai/Desktop/legible_money/proto/legible/capture_challenge/v1/` — protos
- `/Users/yuai/Desktop/legible_money/proto/legible/capture_defense/v1/` — protos

## Module 1: Capture Challenge

### `proto/zerone/capture_challenge/v1/types.proto`
```protobuf
syntax = "proto3";
package zerone.capture_challenge.v1;
option go_package = "github.com/zerone-chain/zerone/x/capture_challenge/types";

message CaptureEvidence {
  string id = 1;
  string evidence_type = 2;       // "concentration", "collusion", "timing", "verdict_correlation"
  string description = 3;
  repeated string fact_ids = 4;
  repeated string validator_addrs = 5;
  uint64 block_start = 6;
  uint64 block_end = 7;
  string data_hash = 8;
  uint64 submitted_at = 9;
  string submitted_by = 10;
}

message CaptureResolution {
  string outcome = 1;             // "upheld", "rejected", "partial"
  string challenger_reward = 2;   // uzrn
  repeated ValidatorSlash validator_slashes = 3;
  repeated string facts_invalidated = 4;
  uint64 resolved_at_block = 5;
  string resolved_by = 6;
  string justification = 7;
}

message ValidatorSlash {
  string validator_addr = 1;
  string amount = 2;              // uzrn
}

message CaptureChallenge {
  string id = 1;
  string domain = 2;
  string challenger = 3;
  string challenge_stake = 4;     // uzrn
  repeated CaptureEvidence evidence = 5;
  repeated string accused_validators = 6;
  uint64 submitted_at_block = 7;
  uint64 evidence_deadline = 8;
  uint64 review_deadline = 9;
  string status = 10;             // "open", "under_review", "resolved"
  CaptureResolution resolution = 11;
  uint64 domain_paused_until = 12;
}

message DomainBountyPool {
  string domain = 1;
  string balance = 2;             // uzrn
  uint64 challenges_submitted = 3;
  uint64 challenges_upheld = 4;
  uint64 challenges_rejected = 5;
  string total_rewards_paid = 6;  // uzrn
  uint64 capture_risk_score = 7;  // 0-1M BPS
  uint64 last_analyzed_block = 8;
  uint64 created_at_block = 9;
}
```

### Genesis + Params
```protobuf
message Params {
  string min_challenge_stake = 1;          // default: "10000000" (10 ZRN)
  uint64 evidence_period_blocks = 2;       // default: 2000
  uint64 review_period_blocks = 3;         // default: 5000
  uint64 domain_pause_blocks = 4;          // default: 1000
  uint64 min_evidence_pieces = 5;          // default: 2
  uint64 reward_rate_bps = 6;              // default: 500000 (50% of bounty)
  uint64 slash_rate_bps = 7;               // default: 200000 (20% of accused stake)
  string bounty_contribution_per_fact = 8; // default: "1000" (0.001 ZRN per fact)
  uint64 risk_analysis_interval = 9;       // default: 10000 blocks
}
```

### Messages
```protobuf
service Msg {
  rpc SubmitChallenge(MsgSubmitChallenge) returns (MsgSubmitChallengeResponse);
  rpc AddEvidence(MsgAddEvidence) returns (MsgAddEvidenceResponse);
  rpc ResolveChallenge(MsgResolveChallenge) returns (MsgResolveChallengeResponse);
  rpc FundBountyPool(MsgFundBountyPool) returns (MsgFundBountyPoolResponse);
}
```

### Implementation
- `SubmitChallenge` → escrow stake, create challenge, start evidence period, pause domain verification
- `AddEvidence` → challenger adds evidence during evidence period
- `ResolveChallenge` → governance/guardian resolves: upheld (slash accused, reward challenger from bounty), rejected (slash challenger stake)
- BeginBlocker: auto-fund bounty pools from fact creation fees, run periodic risk analysis (Herfindahl index)

---

## Module 2: Capture Defense

### `proto/zerone/capture_defense/v1/types.proto`
```protobuf
syntax = "proto3";
package zerone.capture_defense.v1;
option go_package = "github.com/zerone-chain/zerone/x/capture_defense/types";

message IsolatedReputation {
  string validator_addr = 1;
  GlobalReputation global = 2;
  repeated StratumReputation stratum_reps = 3;
  repeated DomainReputation domain_reps = 4;
  uint64 created_at_block = 5;
  uint64 last_updated_block = 6;
}

message GlobalReputation {
  uint64 score = 1;                    // 0-1M
  uint64 total_verifications = 2;
  uint64 correct_verifications = 3;
  uint64 accuracy_bps = 4;            // 1M scale
}

message StratumReputation {
  uint32 stratum = 1;
  uint64 score = 2;
  uint64 total_verifications = 3;
  uint64 correct_verifications = 4;
  uint64 accuracy_bps = 5;
  uint64 domains_verified = 6;
  uint64 last_activity_block = 7;
  uint64 last_decay_block = 8;
}

message DomainReputation {
  string domain = 1;
  uint32 stratum = 2;
  uint64 score = 3;
  uint64 total_verifications = 4;
  uint64 correct_verifications = 5;
  uint64 accuracy_bps = 6;
  uint64 verdict_consistency = 7;
  uint64 confidence_calibration = 8;
  uint64 challenges_received = 9;
  uint64 challenges_lost = 10;
  uint64 last_verification_block = 11;
  uint64 decayed_score = 12;
  uint64 last_decay_block = 13;
}

message CaptureMetrics {
  string domain = 1;
  uint64 herfindahl_index = 2;        // concentration index (0-1M)
  uint64 timing_correlation = 3;      // suspicious timing (0-1M)
  uint64 verdict_correlation = 4;     // suspicious verdict alignment (0-1M)
  uint64 unique_validators = 5;
  uint64 total_verifications = 6;
  uint64 top3_validator_share_bps = 7; // 1M scale
  uint64 analyzed_at_block = 8;
  uint64 risk_score = 9;              // composite 0-1M
}

message VerificationHistoryEntry {
  string claim_id = 1;
  uint64 block_number = 2;
  repeated string validators = 3;
  repeated string verdicts = 4;
  string final_outcome = 5;
}

message CrossStratumRequirement {
  uint32 claim_stratum = 1;
  repeated uint32 required_strata = 2;
  uint32 min_validators_per_stratum = 3;
  uint64 lower_stratum_weight_bonus_bps = 4;
}
```

### Genesis + Params
```protobuf
message Params {
  uint64 reputation_decay_rate_bps = 1;    // default: 10000 (1% per epoch)
  uint64 decay_epoch_blocks = 2;           // default: 10000
  uint64 min_verifications_for_score = 3;  // default: 10
  uint64 herfindahl_alert_threshold = 4;   // default: 250000 (25%)
  uint64 timing_alert_threshold = 5;       // default: 800000 (80%)
  uint64 verdict_alert_threshold = 6;      // default: 900000 (90%)
  uint64 analysis_window_blocks = 7;       // default: 50000
  uint64 verification_history_limit = 8;   // default: 1000
  repeated CrossStratumRequirement cross_stratum_rules = 9;
}
```

### Messages
```protobuf
service Msg {
  rpc RecordVerification(MsgRecordVerification) returns (MsgRecordVerificationResponse);
  rpc AnalyzeDomain(MsgAnalyzeDomain) returns (MsgAnalyzeDomainResponse);
}
```

### Implementation
- `RecordVerification` — called by x/knowledge after verification rounds. Updates isolated reputation (global, stratum, domain level). Applies cross-stratum weighting bonuses
- `AnalyzeDomain` — computes CaptureMetrics: Herfindahl index (sum of squared validator shares), timing correlation, verdict correlation. Flags high-risk domains
- BeginBlocker: periodic reputation decay, automatic domain analysis at risk_analysis_interval
- **Herfindahl Index:** `H = Σ(share_i²)` where share is each validator's fraction of verifications. H > 250k = concentration risk
- **Cross-stratum:** Higher strata claims require validators from lower strata too (diversity)

### Expected Keepers
```go
type KnowledgeKeeper interface {
    GetFactsByDomain(ctx context.Context, domain string) ([]*Fact, error)
    GetVerificationRound(ctx context.Context, claimID string) (*VerificationRound, bool)
}
type StakingKeeper interface {
    GetValidatorTier(ctx context.Context, addr string) (uint32, error)
    SlashValidator(ctx context.Context, addr string, amount math.Int) error
}
```

### Tests
Per module minimum:
- **Capture Challenge:** Submit → add evidence → resolve (upheld + rejected), bounty pool funding, domain pause
- **Capture Defense:** Record verification → reputation update, Herfindahl calculation, timing correlation detection, reputation decay, cross-stratum requirements

## Conventions
- Token: uzrn. Module path: github.com/zerone-chain/zerone
- BPS: 1,000,000 scale
- Run `go build ./...` and `go test ./x/capture_challenge/... ./x/capture_defense/...` before finishing
