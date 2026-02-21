# R6-4 — Qualification Module: Domain Qualifications

## Goal

Port x/qualification — domain qualification system for validators. Validators
earn qualifications in knowledge domains through multiple pathways, with
endorsements, stratum-based hierarchy, and expiry/renewal.

## Working Directory

`/Users/yuai/Desktop/Zerone`

## Reference

- `/Users/yuai/Desktop/legible_money/x/qualification/` — 2704 LOC keeper, 1476 LOC tests
- `/Users/yuai/Desktop/legible_money/proto/legible/qualification/v1/` — all protos

## Proto Files

### `proto/zerone/qualification/v1/types.proto`
```protobuf
syntax = "proto3";
package zerone.qualification.v1;
option go_package = "github.com/zerone-chain/zerone/x/qualification/types";

message QualificationMetrics {
  uint32 total_verifications     = 1;
  uint32 correct_verifications   = 2;
  uint32 accuracy_bps            = 3;   // 1M scale
  uint64 last_verification_block = 4;
  uint32 consecutive_correct     = 5;
  uint32 consecutive_incorrect   = 6;
  uint32 challenges_received     = 7;
  uint32 challenges_lost         = 8;
  uint32 effective_weight_bps    = 9;   // 1M scale
}

message DomainQualification {
  string validator_addr            = 1;
  string domain                    = 2;
  uint32 stratum                   = 3;  // 1-4
  string pathway                   = 4;  // "stake_commitment", "track_record", "cross_reference", "stratum_inheritance", "genesis"
  string status                    = 5;  // "active", "probationary", "suspended", "revoked", "expired"
  uint64 earned_at_block           = 6;
  uint64 expires_at_block          = 7;  // 0 = no expiry
  QualificationMetrics metrics     = 8;
  string locked_stake              = 9;  // uzrn
  uint64 stake_unlocks_at          = 10;
  repeated string endorsers        = 11;
  string inherited_from            = 12; // domain inherited from
  uint32 inheritance_discount_bps  = 13; // 1M scale (1M = no discount)
}

message QualificationEndorsement {
  string id                = 1;
  string endorser_addr     = 2;
  string target_validator  = 3;
  string domain            = 4;
  string stake             = 5;    // uzrn
  uint64 endorsed_at_block = 6;
  uint64 expires_at_block  = 7;
}
```

### `proto/zerone/qualification/v1/genesis.proto`
```protobuf
syntax = "proto3";
package zerone.qualification.v1;
option go_package = "github.com/zerone-chain/zerone/x/qualification/types";

import "zerone/qualification/v1/types.proto";

message GenesisState {
  Params params = 1;
  repeated DomainQualification qualifications = 2;
  repeated QualificationEndorsement endorsements = 3;
}

message Params {
  // Stake commitment pathway
  string min_stake_per_domain = 1;              // default: "50000000" (50 ZRN)
  uint64 stake_lock_blocks = 2;                 // default: 100000

  // Track record pathway
  uint32 min_verifications_for_track_record = 3; // default: 50
  uint32 min_accuracy_bps = 4;                   // default: 800000 (80%)

  // Cross-reference pathway
  uint32 min_endorsers = 5;                      // default: 3
  string min_endorser_stake = 6;                 // default: "10000000" (10 ZRN)

  // Stratum inheritance
  uint32 max_inheritance_discount_bps = 7;       // default: 500000 (50%)
  uint32 min_parent_stratum = 8;                 // default: 2

  // General
  uint64 qualification_duration_blocks = 9;      // default: 500000 (~2 weeks)
  uint32 max_qualifications_per_validator = 10;  // default: 50
  uint32 max_domains = 11;                       // default: 1000
  uint64 probation_duration_blocks = 12;         // default: 50000
  uint32 suspension_threshold_incorrect = 13;    // default: 5 consecutive
  uint32 revocation_threshold_challenges = 14;   // default: 3 lost challenges
}
```

### `proto/zerone/qualification/v1/tx.proto`
```protobuf
syntax = "proto3";
package zerone.qualification.v1;
option go_package = "github.com/zerone-chain/zerone/x/qualification/types";

import "cosmos/msg/v1/msg.proto";
import "zerone/qualification/v1/types.proto";
import "zerone/qualification/v1/genesis.proto";

service Msg {
  option (cosmos.msg.v1.service) = true;

  rpc QualifyByStake(MsgQualifyByStake) returns (MsgQualifyByStakeResponse);
  rpc QualifyByTrackRecord(MsgQualifyByTrackRecord) returns (MsgQualifyByTrackRecordResponse);
  rpc QualifyByCrossReference(MsgQualifyByCrossReference) returns (MsgQualifyByCrossReferenceResponse);
  rpc QualifyByInheritance(MsgQualifyByInheritance) returns (MsgQualifyByInheritanceResponse);
  rpc Endorse(MsgEndorse) returns (MsgEndorseResponse);
  rpc RevokeEndorsement(MsgRevokeEndorsement) returns (MsgRevokeEndorsementResponse);
  rpc RenewQualification(MsgRenewQualification) returns (MsgRenewQualificationResponse);
}

message MsgQualifyByStake {
  option (cosmos.msg.v1.signer) = "validator";
  string validator = 1; string domain = 2; uint32 stratum = 3; string stake = 4;
}
message MsgQualifyByStakeResponse {}

message MsgQualifyByTrackRecord {
  option (cosmos.msg.v1.signer) = "validator";
  string validator = 1; string domain = 2; uint32 stratum = 3;
}
message MsgQualifyByTrackRecordResponse {}

message MsgQualifyByCrossReference {
  option (cosmos.msg.v1.signer) = "validator";
  string validator = 1; string domain = 2; uint32 stratum = 3;
}
message MsgQualifyByCrossReferenceResponse {}

message MsgQualifyByInheritance {
  option (cosmos.msg.v1.signer) = "validator";
  string validator = 1; string domain = 2; string parent_domain = 3;
}
message MsgQualifyByInheritanceResponse { uint32 inheritance_discount_bps = 1; }

message MsgEndorse {
  option (cosmos.msg.v1.signer) = "endorser";
  string endorser = 1; string target_validator = 2;
  string domain = 3; string stake = 4;
}
message MsgEndorseResponse { string endorsement_id = 1; }

message MsgRevokeEndorsement {
  option (cosmos.msg.v1.signer) = "endorser";
  string endorser = 1; string endorsement_id = 2;
}
message MsgRevokeEndorsementResponse {}

message MsgRenewQualification {
  option (cosmos.msg.v1.signer) = "validator";
  string validator = 1; string domain = 2;
}
message MsgRenewQualificationResponse { uint64 new_expiry = 1; }
```

### `proto/zerone/qualification/v1/query.proto`
```protobuf
syntax = "proto3";
package zerone.qualification.v1;
option go_package = "github.com/zerone-chain/zerone/x/qualification/types";

import "google/api/annotations.proto";
import "zerone/qualification/v1/types.proto";
import "zerone/qualification/v1/genesis.proto";

service Query {
  rpc Qualification(QueryQualificationRequest) returns (QueryQualificationResponse) {
    option (google.api.http).get = "/zerone/qualification/v1/qualification/{validator}/{domain}";
  }
  rpc QualificationsByValidator(QueryByValidatorRequest) returns (QueryByValidatorResponse) {
    option (google.api.http).get = "/zerone/qualification/v1/validator/{validator}";
  }
  rpc QualificationsByDomain(QueryByDomainRequest) returns (QueryByDomainResponse) {
    option (google.api.http).get = "/zerone/qualification/v1/domain/{domain}";
  }
  rpc Endorsements(QueryEndorsementsRequest) returns (QueryEndorsementsResponse) {
    option (google.api.http).get = "/zerone/qualification/v1/endorsements/{target_validator}/{domain}";
  }
  rpc Params(QueryParamsRequest) returns (QueryParamsResponse) {
    option (google.api.http).get = "/zerone/qualification/v1/params";
  }
}

message QueryQualificationRequest { string validator = 1; string domain = 2; }
message QueryQualificationResponse { DomainQualification qualification = 1; }
message QueryByValidatorRequest { string validator = 1; }
message QueryByValidatorResponse { repeated DomainQualification qualifications = 1; }
message QueryByDomainRequest { string domain = 1; }
message QueryByDomainResponse { repeated DomainQualification qualifications = 1; }
message QueryEndorsementsRequest { string target_validator = 1; string domain = 2; }
message QueryEndorsementsResponse { repeated QualificationEndorsement endorsements = 1; }
message QueryParamsRequest {}
message QueryParamsResponse { Params params = 1; }
```

## Implementation

### Qualification Pathways

1. **Stake Commitment** — Lock uzrn ≥ min_stake_per_domain for stake_lock_blocks. Instant qualification
2. **Track Record** — Demonstrate min_verifications + min_accuracy in the domain. Checked against x/capture_defense reputation
3. **Cross-Reference** — Get min_endorsers endorsements from already-qualified validators in the domain
4. **Stratum Inheritance** — If qualified in a parent domain at stratum ≥ min_parent_stratum, qualify in child domain at a discount

### Status Transitions
- Active → Probationary: consecutive_incorrect ≥ suspension_threshold
- Probationary → Active: consecutive_correct ≥ 10 during probation
- Probationary → Suspended: still incorrect after probation_duration
- Active/Probationary → Revoked: challenges_lost ≥ revocation_threshold
- Active → Expired: block > expires_at_block

### BeginBlocker
- Expire qualifications past expires_at_block
- Expire endorsements past their expiry
- Unlock stakes past stake_unlocks_at

### Expected Keepers
```go
type BankKeeper interface { ... }
type StakingKeeper interface {
    IsValidator(ctx context.Context, addr string) bool
}
type CaptureDefenseKeeper interface {
    GetDomainReputation(ctx context.Context, validator, domain string) (*DomainReputation, bool)
}
```

### Tests
- All 4 qualification pathways happy path
- Stake lock and unlock
- Track record with insufficient accuracy rejected
- Cross-reference with insufficient endorsers rejected
- Inheritance discount calculation
- Status transitions: active → probationary → suspended
- Revocation after lost challenges
- Expiry in BeginBlocker
- Renewal extends expiry

## Conventions
- Token: uzrn. Module path: github.com/zerone-chain/zerone
- BPS: 1,000,000 scale
- Run `go build ./...` and `go test ./x/qualification/...` before finishing
