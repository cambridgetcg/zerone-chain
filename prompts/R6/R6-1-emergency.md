# R6-1 — Emergency Module: Kill Switch

## Goal

Port x/emergency — 3-phase BFT emergency protocol (Halt → Revert → Resume)
with guardian-only access, quorum thresholds, ceremony lifecycle, and audit trail.

## Working Directory

`/Users/yuai/Desktop/Zerone`

## Reference

- `/Users/yuai/Desktop/legible_money/x/emergency/` — full module (3149 LOC keeper, 1465 LOC tests)
- `/Users/yuai/Desktop/legible_money/proto/legible/emergency/v1/` — all protos

## Proto Files

### `proto/zerone/emergency/v1/types.proto`
```protobuf
syntax = "proto3";
package zerone.emergency.v1;
option go_package = "github.com/zerone-chain/zerone/x/emergency/types";

message EmergencyHaltProposal {
  string id = 1;
  string proposer = 2;
  string reason = 3;
  string evidence_hash = 4;
  uint64 proposed_at_block = 5;
  EmergencyCategory category = 6;
}

message EmergencyVote {
  string voter = 1;
  bool approve = 2;
}

message EmergencyPrecommit {
  string voter = 1;
}

message PrevoteEntry {
  string key = 1;
  EmergencyVote value = 2;
}

message PrecommitEntry {
  string key = 1;
  EmergencyPrecommit value = 2;
}

message EmergencyCeremony {
  string id = 1;
  string ceremony_type = 2;       // "halt", "revert", "resume"
  string phase = 3;               // "prevote", "precommit", "executed", "failed", "timed_out"
  bytes proposal_data = 4;
  uint64 start_block = 5;
  uint64 prevote_deadline = 6;
  uint64 precommit_deadline = 7;
  uint64 timeout_deadline = 8;
  repeated PrevoteEntry prevotes = 9;
  repeated PrecommitEntry precommits = 10;
  string yes_prevote_stake = 11;  // uzrn
  string no_prevote_stake = 12;   // uzrn
  string precommit_stake = 13;    // uzrn
  string failure_reason = 14;
}

message EmergencyAuditEntry {
  int64 timestamp = 1;
  uint64 block_number = 2;
  string action = 3;
  string actor = 4;
  string ceremony_id = 5;
  string details = 6;
}

enum EmergencyCategory {
  EMERGENCY_CATEGORY_UNSPECIFIED = 0;
  EMERGENCY_CATEGORY_SECURITY_BREACH = 1;
  EMERGENCY_CATEGORY_CONSENSUS_FAILURE = 2;
  EMERGENCY_CATEGORY_ECONOMIC_EXPLOIT = 3;
  EMERGENCY_CATEGORY_STATE_CORRUPTION = 4;
}
```

### `proto/zerone/emergency/v1/genesis.proto`
```protobuf
syntax = "proto3";
package zerone.emergency.v1;
option go_package = "github.com/zerone-chain/zerone/x/emergency/types";

import "zerone/emergency/v1/types.proto";

message GenesisState {
  Params params = 1;
  string status = 2;              // "normal", "halted"
  repeated EmergencyCeremony ceremonies = 3;
  repeated EmergencyAuditEntry audit_log = 4;
}

message Params {
  // Quorum thresholds (1M BPS scale)
  uint64 halt_quorum = 1;           // default: 750000 (75%)
  uint64 revert_quorum = 2;         // default: 800000 (80%)
  uint64 resume_quorum = 3;         // default: 800000 (80%)

  // Ceremony timing (blocks)
  uint64 halt_prevote_blocks = 4;      // default: 100
  uint64 halt_precommit_blocks = 5;    // default: 50
  uint64 halt_timeout_blocks = 6;      // default: 200
  uint64 revert_prevote_blocks = 7;    // default: 200
  uint64 revert_precommit_blocks = 8;  // default: 100
  uint64 revert_timeout_blocks = 9;    // default: 500
  uint64 resume_prevote_blocks = 10;   // default: 100
  uint64 resume_precommit_blocks = 11; // default: 50
  uint64 resume_timeout_blocks = 12;   // default: 200

  // Anti-abuse
  uint64 max_proposals_per_epoch = 13;              // default: 5
  uint64 max_proposals_per_guardian_per_epoch = 14;  // default: 2
  uint64 cooldown_blocks = 15;                       // default: 500
  string min_guardian_stake = 16;                    // default: "100000000" (100 ZRN)
  uint64 min_distinct_voters = 17;                   // default: 3

  // Revert constraints
  uint64 max_revert_depth = 18;     // default: 1000

  // Epoch
  uint64 epoch_blocks = 19;         // default: 10000

  // Genesis council (bootstrap only)
  repeated string genesis_council = 20;
  uint64 council_expiry_block = 21;
  string council_virtual_stake = 22;

  // Auto-resume
  uint64 max_halt_duration_blocks = 23;  // default: 50000
}
```

### `proto/zerone/emergency/v1/tx.proto`
```protobuf
syntax = "proto3";
package zerone.emergency.v1;
option go_package = "github.com/zerone-chain/zerone/x/emergency/types";

import "cosmos/msg/v1/msg.proto";
import "zerone/emergency/v1/types.proto";
import "zerone/emergency/v1/genesis.proto";

service Msg {
  option (cosmos.msg.v1.service) = true;

  rpc ProposeHalt(MsgProposeHalt) returns (MsgProposeHaltResponse);
  rpc VoteHalt(MsgVoteHalt) returns (MsgVoteHaltResponse);
  rpc ProposeRevert(MsgProposeRevert) returns (MsgProposeRevertResponse);
  rpc VoteRevert(MsgVoteRevert) returns (MsgVoteRevertResponse);
  rpc ProposeResume(MsgProposeResume) returns (MsgProposeResumeResponse);
  rpc VoteResume(MsgVoteResume) returns (MsgVoteResumeResponse);
  rpc UpdateParams(MsgUpdateParams) returns (MsgUpdateParamsResponse);
}

message MsgProposeHalt {
  option (cosmos.msg.v1.signer) = "proposer";
  string proposer = 1; string reason = 2; EmergencyCategory category = 3;
}
message MsgProposeHaltResponse { string ceremony_id = 1; }

message MsgVoteHalt {
  option (cosmos.msg.v1.signer) = "voter";
  string voter = 1; string ceremony_id = 2; bool approve = 3;
}
message MsgVoteHaltResponse { bool quorum_reached = 1; bool chain_halted = 2; }

message MsgProposeRevert {
  option (cosmos.msg.v1.signer) = "proposer";
  string proposer = 1; uint64 revert_to_height = 2; string justification = 3;
}
message MsgProposeRevertResponse { string ceremony_id = 1; }

message MsgVoteRevert {
  option (cosmos.msg.v1.signer) = "voter";
  string voter = 1; string ceremony_id = 2; bool approve = 3;
}
message MsgVoteRevertResponse { bool quorum_reached = 1; }

message MsgProposeResume {
  option (cosmos.msg.v1.signer) = "proposer";
  string proposer = 1; string justification = 2;
}
message MsgProposeResumeResponse { string ceremony_id = 1; }

message MsgVoteResume {
  option (cosmos.msg.v1.signer) = "voter";
  string voter = 1; string ceremony_id = 2; bool approve = 3;
}
message MsgVoteResumeResponse { bool quorum_reached = 1; bool chain_resumed = 2; }

message MsgUpdateParams {
  option (cosmos.msg.v1.signer) = "authority";
  string authority = 1; Params params = 2;
}
message MsgUpdateParamsResponse {}
```

### `proto/zerone/emergency/v1/query.proto`
```protobuf
syntax = "proto3";
package zerone.emergency.v1;
option go_package = "github.com/zerone-chain/zerone/x/emergency/types";

import "google/api/annotations.proto";
import "zerone/emergency/v1/types.proto";
import "zerone/emergency/v1/genesis.proto";

service Query {
  rpc Status(QueryStatusRequest) returns (QueryStatusResponse) {
    option (google.api.http).get = "/zerone/emergency/v1/status";
  }
  rpc Ceremony(QueryCeremonyRequest) returns (QueryCeremonyResponse) {
    option (google.api.http).get = "/zerone/emergency/v1/ceremony/{id}";
  }
  rpc AuditLog(QueryAuditLogRequest) returns (QueryAuditLogResponse) {
    option (google.api.http).get = "/zerone/emergency/v1/audit";
  }
  rpc Params(QueryParamsRequest) returns (QueryParamsResponse) {
    option (google.api.http).get = "/zerone/emergency/v1/params";
  }
}

message QueryStatusRequest {}
message QueryStatusResponse { string status = 1; EmergencyCeremony active_ceremony = 2; }
message QueryCeremonyRequest { string id = 1; }
message QueryCeremonyResponse { EmergencyCeremony ceremony = 1; }
message QueryAuditLogRequest { uint64 limit = 1; }
message QueryAuditLogResponse { repeated EmergencyAuditEntry entries = 1; }
message QueryParamsRequest {}
message QueryParamsResponse { Params params = 1; }
```

## Implementation

### Keeper
- **State:** ceremonies, audit log, chain status ("normal"/"halted"), proposal counters per epoch
- **Guardian check:** Verify proposer/voter is a Guardian-tier validator (tier 3) via StakingKeeper. During bootstrap, check genesis_council list
- **Ceremony lifecycle:**
  1. ProposeHalt → create ceremony, set prevote deadline
  2. VoteHalt → add prevote, check quorum (75% by stake). If reached, advance to precommit. After precommit quorum → set status="halted"
  3. ProposeRevert (only when halted) → similar ceremony with 80% quorum
  4. ProposeResume (only when halted) → 80% quorum to resume
- **BeginBlocker:** Check ceremony deadlines (timeout → fail), check max halt duration (auto-resume)
- **Audit trail:** Every action logged to EmergencyAuditEntry

### Expected Keepers
```go
type StakingKeeper interface {
    GetValidator(ctx context.Context, addr string) (types.Validator, bool)
    IsGuardianTier(ctx context.Context, addr string) bool
    GetValidatorStake(ctx context.Context, addr string) (math.Int, error)
}
```

### Tests
- Full halt ceremony: propose → vote → quorum → halted
- Halt blocks normal operations
- Resume ceremony restores normal
- Non-guardian proposer rejected
- Quorum not reached → timeout
- Anti-abuse: max proposals per epoch
- Cooldown enforcement
- Auto-resume after max_halt_duration

## Conventions
- Token: uzrn. Module path: github.com/zerone-chain/zerone
- BPS: 1,000,000 scale for quorum thresholds
- Run `go build ./...` and `go test ./x/emergency/...` before finishing
