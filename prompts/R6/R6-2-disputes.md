# R6-2 — Disputes Module: Tiered Resolution

## Goal

Port x/disputes — tiered dispute resolution with commit/reveal evidence,
arbiter voting, escalation, bond slashing, and reward distribution.

## Working Directory

`/Users/yuai/Desktop/Zerone`

## Reference

- `/Users/yuai/Desktop/legible_money/x/disputes/` — full module (4671 LOC keeper, 2718 LOC tests)
- `/Users/yuai/Desktop/legible_money/proto/legible/disputes/v1/` — all protos

## Proto Files

### `proto/zerone/disputes/v1/types.proto`
```protobuf
syntax = "proto3";
package zerone.disputes.v1;
option go_package = "github.com/zerone-chain/zerone/x/disputes/types";

message Dispute {
  string id = 1;
  string target_id = 2;           // fact_id, evidence_id, or validator_addr
  DisputeTargetType target_type = 3;
  string challenger = 4;          // bech32
  string defender = 5;            // bech32 (fact creator or validator)
  string reason = 6;
  string bond = 7;                // uzrn staked by challenger
  uint32 tier = 8;                // escalation tier (1-4)
  DisputePhase phase = 9;
  DisputeOutcome outcome = 10;
  uint64 created_at = 11;
  uint64 evidence_deadline = 12;
  uint64 voting_deadline = 13;
  repeated string arbiters = 14;
  uint32 evidence_count = 15;
}

message DisputeEvidence {
  string id = 1;
  string dispute_id = 2;
  string submitter = 3;
  string side = 4;                // "challenger", "defender"
  string content = 5;
  uint64 submitted_at = 6;
}

message EvidenceCommitment {
  string dispute_id = 1;
  string submitter = 2;
  string side = 3;
  string content_hash = 4;        // SHA256 of content+nonce
  uint64 committed_at = 5;
  bool revealed = 6;
}

message DisputeVote {
  string dispute_id = 1;
  string arbiter = 2;
  ArbiterDecision vote = 3;
  string stake = 4;               // uzrn arbiter stake
  string rationale = 5;
  uint64 voted_at = 6;
}

message TierConfig {
  uint32 tier = 1;
  uint32 arbiter_count = 2;       // how many arbiters needed
  string min_bond = 3;            // uzrn minimum challenger bond
  uint64 evidence_period = 4;     // blocks for evidence submission
  uint64 voting_period = 5;       // blocks for arbiter voting
  uint64 quorum_bps = 6;          // 1M scale
  uint64 majority_bps = 7;        // 1M scale (e.g. 666667 = 2/3)
}

enum DisputeTargetType {
  DISPUTE_TARGET_TYPE_UNSPECIFIED = 0;
  DISPUTE_TARGET_TYPE_FACT = 1;
  DISPUTE_TARGET_TYPE_EVIDENCE = 2;
  DISPUTE_TARGET_TYPE_VALIDATOR = 3;
}

enum DisputePhase {
  DISPUTE_PHASE_UNSPECIFIED = 0;
  DISPUTE_PHASE_EVIDENCE_COMMIT = 1;
  DISPUTE_PHASE_EVIDENCE_REVEAL = 2;
  DISPUTE_PHASE_ARBITRATION = 3;
  DISPUTE_PHASE_ESCALATED = 4;
  DISPUTE_PHASE_SETTLED = 5;
  DISPUTE_PHASE_TIMED_OUT = 6;
}

enum ArbiterDecision {
  ARBITER_DECISION_UNSPECIFIED = 0;
  ARBITER_DECISION_CHALLENGER = 1;
  ARBITER_DECISION_DEFENDER = 2;
  ARBITER_DECISION_ABSTAIN = 3;
}

enum DisputeOutcome {
  DISPUTE_OUTCOME_UNSPECIFIED = 0;
  DISPUTE_OUTCOME_CHALLENGER_WINS = 1;
  DISPUTE_OUTCOME_DEFENDER_WINS = 2;
  DISPUTE_OUTCOME_DRAW = 3;
  DISPUTE_OUTCOME_TIMED_OUT = 4;
}
```

### `proto/zerone/disputes/v1/genesis.proto`
```protobuf
syntax = "proto3";
package zerone.disputes.v1;
option go_package = "github.com/zerone-chain/zerone/x/disputes/types";

import "zerone/disputes/v1/types.proto";

message GenesisState {
  Params params = 1;
  repeated Dispute disputes = 2;
}

message Params {
  repeated TierConfig tier_configs = 1;  // 4 tiers
  uint32 max_active_disputes = 2;        // default: 100
  uint64 escalation_delay = 3;           // default: 500 blocks
  uint64 slash_rate_loser_bps = 4;       // default: 500000 (50% of bond)
  uint64 reward_rate_winner_bps = 5;     // default: 400000 (40% of loser's slash)
  uint64 arbiter_reward_bps = 6;         // default: 100000 (10% of loser's slash)
}
```

Default tier configs:
```
Tier 1: 3 arbiters, 1 ZRN bond, 500 block evidence, 1000 block vote, 500k quorum, 666667 majority
Tier 2: 7 arbiters, 10 ZRN bond, 1000 block evidence, 2000 block vote, 500k quorum, 666667 majority
Tier 3: 13 arbiters, 100 ZRN bond, 2000 block evidence, 5000 block vote, 600k quorum, 750000 majority
Tier 4: 21 arbiters, 1000 ZRN bond, 5000 block evidence, 10000 block vote, 666k quorum, 800000 majority
```

### `proto/zerone/disputes/v1/tx.proto`
```protobuf
syntax = "proto3";
package zerone.disputes.v1;
option go_package = "github.com/zerone-chain/zerone/x/disputes/types";

import "cosmos/msg/v1/msg.proto";
import "zerone/disputes/v1/types.proto";
import "zerone/disputes/v1/genesis.proto";

service Msg {
  option (cosmos.msg.v1.service) = true;

  rpc InitiateDispute(MsgInitiateDispute) returns (MsgInitiateDisputeResponse);
  rpc CommitEvidence(MsgCommitEvidence) returns (MsgCommitEvidenceResponse);
  rpc RevealEvidence(MsgRevealEvidence) returns (MsgRevealEvidenceResponse);
  rpc ArbiterVote(MsgArbiterVote) returns (MsgArbiterVoteResponse);
  rpc EscalateDispute(MsgEscalateDispute) returns (MsgEscalateDisputeResponse);
  rpc SettleDispute(MsgSettleDispute) returns (MsgSettleDisputeResponse);
}

message MsgInitiateDispute {
  option (cosmos.msg.v1.signer) = "challenger";
  string challenger = 1; DisputeTargetType target_type = 2;
  string target_id = 3; string reason = 4; string bond = 5;
}
message MsgInitiateDisputeResponse { string dispute_id = 1; }

message MsgCommitEvidence {
  option (cosmos.msg.v1.signer) = "submitter";
  string submitter = 1; string dispute_id = 2; string commitment_hash = 3;
}
message MsgCommitEvidenceResponse {}

message MsgRevealEvidence {
  option (cosmos.msg.v1.signer) = "submitter";
  string submitter = 1; string dispute_id = 2;
  string content = 3; string nonce = 4;
}
message MsgRevealEvidenceResponse {}

message MsgArbiterVote {
  option (cosmos.msg.v1.signer) = "arbiter";
  string arbiter = 1; string dispute_id = 2;
  ArbiterDecision vote = 3; string reasoning = 4;
}
message MsgArbiterVoteResponse {}

message MsgEscalateDispute {
  option (cosmos.msg.v1.signer) = "requester";
  string requester = 1; string dispute_id = 2; string additional_bond = 3;
}
message MsgEscalateDisputeResponse { uint32 new_tier = 1; }

message MsgSettleDispute {
  option (cosmos.msg.v1.signer) = "authority";
  string authority = 1; string dispute_id = 2;
}
message MsgSettleDisputeResponse { DisputeOutcome outcome = 1; }
```

### `proto/zerone/disputes/v1/query.proto`
```protobuf
syntax = "proto3";
package zerone.disputes.v1;
option go_package = "github.com/zerone-chain/zerone/x/disputes/types";

import "google/api/annotations.proto";
import "zerone/disputes/v1/types.proto";
import "zerone/disputes/v1/genesis.proto";

service Query {
  rpc Dispute(QueryDisputeRequest) returns (QueryDisputeResponse) {
    option (google.api.http).get = "/zerone/disputes/v1/dispute/{id}";
  }
  rpc DisputesByTarget(QueryByTargetRequest) returns (QueryByTargetResponse) {
    option (google.api.http).get = "/zerone/disputes/v1/by_target/{target_id}";
  }
  rpc ActiveDisputes(QueryActiveRequest) returns (QueryActiveResponse) {
    option (google.api.http).get = "/zerone/disputes/v1/active";
  }
  rpc Params(QueryParamsRequest) returns (QueryParamsResponse) {
    option (google.api.http).get = "/zerone/disputes/v1/params";
  }
}

message QueryDisputeRequest { string id = 1; }
message QueryDisputeResponse { Dispute dispute = 1; }
message QueryByTargetRequest { string target_id = 1; }
message QueryByTargetResponse { repeated Dispute disputes = 1; }
message QueryActiveRequest {}
message QueryActiveResponse { repeated Dispute disputes = 1; }
message QueryParamsRequest {}
message QueryParamsResponse { Params params = 1; }
```

## Implementation

### Keeper

**Dispute Lifecycle:**
1. `InitiateDispute` → validate target exists, bond ≥ tier min_bond, escrow bond, select arbiters (random from qualified validators), set phase=EVIDENCE_COMMIT
2. `CommitEvidence` → challenger or defender commits hash(content+nonce) during commit phase
3. `RevealEvidence` → reveal content+nonce, verify hash matches commitment
4. Phase transitions in BeginBlocker: commit → reveal → arbitration (when deadlines pass)
5. `ArbiterVote` → selected arbiters vote during arbitration phase
6. `EscalateDispute` → move to higher tier, increase bond, select more arbiters
7. `SettleDispute` → tally votes, determine outcome, distribute bonds

**Settlement:**
- Challenger wins: defender's bond → challenger (winner_rate) + arbiters (arbiter_rate) + burn (remainder)
- Defender wins: challenger's bond → defender (winner_rate) + arbiters (arbiter_rate) + burn
- Draw: bonds returned minus arbiter fees
- Timeout: bonds returned

**Arbiter Selection:**
Select from validators with relevant domain qualifications (via StakingKeeper).
Exclude challenger, defender, and their partnership partners.

**BeginBlocker:**
- Advance phases based on deadlines
- Timeout disputes past voting_deadline with no quorum
- Check max_active_disputes

### Expected Keepers
```go
type BankKeeper interface { ... }
type StakingKeeper interface {
    GetQualifiedValidators(ctx context.Context, domain string, count int) ([]string, error)
}
type KnowledgeKeeper interface {
    GetFact(ctx context.Context, factID string) (*knowledgetypes.Fact, bool)
}
```

### Tests
- Full lifecycle: initiate → commit → reveal → vote → settle
- Challenger wins / defender wins / draw / timeout
- Commit/reveal mismatch rejected
- Escalation increases tier and arbiter count
- Bond slashing and distribution
- Arbiter selection excludes parties
- Max active disputes enforcement

## Conventions
- Token: uzrn. Module path: github.com/zerone-chain/zerone
- BPS: 1,000,000 scale
- Run `go build ./...` and `go test ./x/disputes/...` before finishing
