# R2-1 — Knowledge Proto + Types

## Goal

Define all protobuf types for the knowledge module — the most complex module
in Zerone. Facts, claims, verification rounds, VRF proofs, domains, confidence
scoring, and 28+ governance parameters.

## Working Directory

`/Users/yuai/Desktop/Zerone`

## Reference

- `/Users/yuai/Desktop/legible_money/x/knowledge/types/` — all type definitions
- `/Users/yuai/Desktop/legible_money/proto/legible/knowledge/` — existing protos
- `/Users/yuai/Desktop/legible_money/docs/PARAMETERS.md` — knowledge params section
- `/Users/yuai/Desktop/legible_money/reports/batch-21/B21-2-parameter-audit.md` — parameter audit

## Proto Files to Create

### `proto/zerone/knowledge/v1/types.proto`

Core types:
```protobuf
message Fact {
  string id = 1;
  string content = 2;
  string domain = 3;
  string category = 4;           // "axiomatic", "empirical", "derived", "contested"
  uint64 confidence = 5;          // 0-1,000,000 BPS
  string submitter = 6;
  uint64 submitted_at_block = 7;
  uint64 verified_at_block = 8;
  uint64 citation_count = 9;
  uint64 fundamentality = 10;     // 0-1,000,000 (how foundational)
  repeated string references = 11; // fact IDs this fact cites
  string status = 12;             // "pending", "verified", "rejected", "contested"
}

message Claim {
  string id = 1;
  string fact_content = 2;
  string domain = 3;
  string category = 4;
  string submitter = 5;
  uint64 submitted_at_block = 6;
  string status = 7;              // "submitted", "in_verification", "accepted", "rejected"
  repeated string references = 8;
  string verification_round_id = 9;
}

message VerificationRound {
  string id = 1;
  string claim_id = 2;
  uint64 started_at_block = 3;
  string phase = 4;               // "commit", "reveal", "tally"
  repeated string selected_verifiers = 5;
  repeated CommitEntry commits = 6;
  repeated RevealEntry reveals = 7;
  string verdict = 8;             // "accept", "reject", "inconclusive"
  uint64 verdict_block = 9;
}

message CommitEntry {
  string verifier = 1;
  bytes commit_hash = 2;          // SHA-256(vote || salt)
  uint64 committed_at_block = 3;
}

message RevealEntry {
  string verifier = 1;
  string vote = 2;                // "accept" or "reject"
  bytes salt = 3;
  uint64 revealed_at_block = 4;
}

message VRFProof {
  bytes proof = 1;
  bytes output = 2;
  string proposer = 3;
  uint64 block_height = 4;
}

message Domain {
  string name = 1;
  string description = 2;
  string status = 3;              // "active", "deprecated"
  uint64 created_at_block = 4;
  uint64 fact_count = 5;
}
```

### `proto/zerone/knowledge/v1/genesis.proto`

```protobuf
message Params {
  // Core verification params
  uint64 min_verifiers = 1;                    // default: 3
  uint64 max_verifiers = 2;                    // default: 7
  uint64 commit_phase_blocks = 3;              // default: 10
  uint64 reveal_phase_blocks = 4;              // default: 10
  uint64 claim_cooldown_blocks = 5;            // default: 50

  // Confidence scoring
  uint64 initial_confidence = 6;               // default: 500,000 (50%)
  uint64 confidence_boost_per_verification = 7; // default: 50,000 (5%)
  uint64 confidence_threshold = 8;             // default: 500,000 (50%)

  // Slashing — MUST be non-zero (B22-3 audit fix)
  uint64 wrong_verification_slash_bps = 9;     // default: 50,000 (5%)
  uint64 missed_reveal_slash_bps = 10;         // default: 100,000 (10%)
  uint64 equivocation_slash_bps = 11;          // default: 200,000 (20%)

  // Rewards
  string verification_reward = 12;             // default: "3000000" (3 ZRN in uzrn)
  uint64 verification_reward_decay_bps = 13;   // default: 999,000 (0.999x per epoch)

  // Extended params (governance-adjustable, from R3-4 draft)
  // ... include all 72 extended params from draft
  // Key ones: min_claim_stake, max_facts_per_domain, fact_expiry_blocks,
  // cross_reference_bonus_bps, etc.
}

message GenesisState {
  Params params = 1;
  repeated Fact facts = 2;
  repeated Claim pending_claims = 3;
  repeated VerificationRound active_rounds = 4;
  repeated Domain domains = 5;
}
```

### `proto/zerone/knowledge/v1/tx.proto`

Messages (port all 21 from draft):
- MsgSubmitClaim
- MsgCommitVerification
- MsgRevealVerification
- MsgProposeDomain
- MsgUpdateFact
- MsgContestFact
- MsgCiteFact
- MsgUpdateParams
- ... (all others from draft proto/legible/knowledge/v1/tx.proto)

### `proto/zerone/knowledge/v1/query.proto`

Queries (12 RPCs):
- QueryFact (id) → Fact
- QueryFacts (domain, status, pagination) → []Fact
- QueryClaim (id) → Claim
- QueryPendingClaims (pagination) → []Claim
- QueryVerificationRound (id) → VerificationRound
- QueryDomain (name) → Domain
- QueryDomains (pagination) → []Domain
- QueryFactConfidence (id) → confidence
- QueryFactCitationCount (id) → count
- QueryParams → Params
- QueryFactsBySubmitter (address) → []Fact
- QueryFactCreatedBlock (id) → block

## Implementation

### Generate Go code
```bash
cd proto && buf generate
```

### Create types package
`x/knowledge/types/`:
- `keys.go` — store key prefixes (port from draft, extensive — ~30 prefix keys)
- `errors.go` — sentinel errors
- `codec.go` — RegisterInterfaces
- `genesis.go` — DefaultGenesis, Validate (with non-zero slash params!)
- `expected_keepers.go` — AccountKeeper, BankKeeper, OntologyKeeper, StakingKeeper

### Validate function MUST check

```go
func (p Params) Validate() error {
    if p.WrongVerificationSlashBps == 0 {
        return fmt.Errorf("wrong_verification_slash_bps must be > 0")
    }
    if p.MissedRevealSlashBps == 0 {
        return fmt.Errorf("missed_reveal_slash_bps must be > 0")
    }
    if p.EquivocationSlashBps == 0 {
        return fmt.Errorf("equivocation_slash_bps must be > 0")
    }
    // ... all other validations
}
```

### Migrator stub
```go
type Migrator struct { keeper Keeper }
func NewMigrator(keeper Keeper) Migrator { return Migrator{keeper: keeper} }
```

## Tests

`x/knowledge/types/types_test.go`:
- TestDefaultParams_AllSlashParamsNonZero
- TestDefaultParams_Validate
- TestDefaultGenesis_Validate
- TestGenesisState_Marshal_Deterministic

## Verification

```bash
make proto-gen
go build ./...
go vet ./...
go test ./x/knowledge/... -count=1 -v
```

## Commit

```
feat(knowledge): proto types — facts, claims, verification rounds, VRF, domains
```

## Do NOT

- Implement keeper logic (that's R2-2)
- Leave slash params at zero in DefaultParams
- Use hand-written types for anything that has a proto definition
- Skip the extended params (72+ from draft R3-4)
