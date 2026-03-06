# R43-1 — Protobuf Cleanup

## Objective

Formalize all Go-only types into proper protobuf definitions and regenerate. Remove manual proto marshaling code. Update GenesisState to include all new state types.

## Current State

Many types were added as Go-only structs (stored as JSON in KVStore) to avoid proto changes during rapid development. Now we consolidate them into proper `.proto` definitions.

## Types to Add to `types.proto`

### 1. Fitness Types
```protobuf
// FitnessRecord tracks the fitness lifecycle of a TDU.
message FitnessRecord {
  string sample_id = 1;
  string score = 2;          // sdkmath.LegacyDec as string
  string peak_score = 3;     // highest score ever reached
  int64 last_signal_block = 4;
  int64 created_at_block = 5;
  string lifecycle_status = 6;  // "core", "active", "dormant", "pruned"
}

// FitnessSignal represents a signal that updates a TDU's fitness score.
message FitnessSignal {
  string sample_id = 1;
  string signal_type = 2;    // "training_influence", "usage_correlation", "redundancy"
  string value = 3;          // sdkmath.LegacyDec as string
  string weight = 4;         // sdkmath.LegacyDec as string
  int64 block_height = 5;
}

// FitnessParams holds governance-tunable parameters for the fitness subsystem.
message FitnessParams {
  uint64 fitness_epoch_blocks = 1;       // blocks between fitness decay cycles (default 100)
  string decay_rate_unscored = 2;        // decay per epoch for TDUs with no new signals
  string longevity_reward_amount = 3;    // uzrn rewarded to Core TDUs per epoch
  string prune_threshold = 4;            // fitness below this = pruned (default "0.1")
}
```

### 2. Reputation Types
```protobuf
// ReputationRecord tracks an agent's reputation in a specific domain.
message ReputationRecord {
  string agent = 1;
  string domain = 2;
  string score = 3;          // sdkmath.LegacyDec as string
  string peak_score = 4;
  int64 last_active_block = 5;
  int64 created_at_block = 6;
}

// ReputationDecayParams holds governance-tunable parameters for reputation.
message ReputationDecayParams {
  uint32 decay_rate_bps = 1;             // basis points per interval (default 500 = 5%)
  uint64 decay_interval_blocks = 2;      // blocks of inactivity before decay (default 432000)
  uint32 floor_ratio_bps = 3;            // min score as BPS of peak (default 2500 = 25%)
  string submitter_reputation_gain = 4;  // LegacyDec (default "10")
  string reviewer_reputation_gain = 5;   // LegacyDec (default "3")
  string reviewer_reputation_penalty = 6; // LegacyDec (default "5")
  uint32 reputation_multiplier_bps = 7;  // max vote weight boost (default 20000 = 2.0x)
  string base_vote_weight = 8;           // LegacyDec (default "1")
}
```

### 3. Sharding Types
```protobuf
// ShardingParams holds governance-tunable parameters for dataset sharding.
message ShardingParams {
  uint32 replication_factor = 1;    // validators per TDU (default 3)
  uint64 snapshot_interval = 2;     // blocks between reshuffles (default 1000)
  uint32 min_validators = 3;       // minimum validators for sharding (default 3)
}

// ShardAssignment maps a TDU to its assigned validators at a given snapshot.
message ShardAssignment {
  string sample_id = 1;
  repeated string validators = 2;
  int64 snapshot_height = 3;
}

// AttestationRecord tracks a validator's proof-of-storage attestation.
message AttestationRecord {
  string validator = 1;
  int64 snapshot_height = 2;
  string data_hash = 3;
  int64 attested_at_block = 4;
  bool verified = 5;
}
```

### 4. Reviewer Staking Types
```protobuf
// ReviewerStakingParams holds dual-staking economics parameters.
message ReviewerStakingParams {
  uint64 reviewer_stake_ratio_bps = 1;   // reviewer stake = submitter × ratio (default 3000 = 30%)
  uint64 show_up_reward_ratio_bps = 2;   // show-up from minority pot (default 1000 = 10%)
  uint64 accept_reward_ratio_bps = 3;    // accept bonus for submitter (default 3000 = 30%)
  uint64 reject_bonus_ratio_bps = 4;     // challenge bonus (default 5000 = 50%)
  uint64 max_contested_deep_count = 5;   // strikes before permanent reject (default 3)
}
```

### 5. Training Types
```protobuf
// TrainingRecord stores the on-chain record of a completed TEE training run.
message TrainingRecord {
  string operator = 1;
  string enclave_id = 2;
  string attestation_hash = 3;
  string dataset_fingerprint = 4;
  int64 dataset_size = 5;
  string base_model = 6;
  string model_hash = 7;
  string benchmark_score = 8;   // stored as string for precision
  int64 block_height = 9;
}
```

## Messages to Add to `tx.proto`

### MsgRecordTraining
```protobuf
rpc RecordTraining(MsgRecordTraining) returns (MsgRecordTrainingResponse);

message MsgRecordTraining {
  option (cosmos.msg.v1.signer) = "operator";
  string operator = 1;
  string attestation_hash = 2;
  string dataset_fingerprint = 3;
  int64 dataset_size = 4;
  string base_model = 5;
  string model_hash = 6;
  string benchmark_score = 7;
}

message MsgRecordTrainingResponse {
  string attestation_hash = 1;
}
```

### MsgUpdateFitnessBatch
```protobuf
rpc UpdateFitnessBatch(MsgUpdateFitnessBatch) returns (MsgUpdateFitnessBatchResponse);

message MsgUpdateFitnessBatch {
  option (cosmos.msg.v1.signer) = "oracle";
  string oracle = 1;
  repeated FitnessSignal signals = 2;
}

message MsgUpdateFitnessBatchResponse {
  uint64 processed = 1;
}
```

## Update `genesis.proto`

Add to GenesisState:
```protobuf
message GenesisState {
  // ... existing fields ...
  
  // New ToK state
  repeated FitnessRecord fitness_records = 15;
  repeated ReputationRecord reputation_records = 16;
  repeated ShardAssignment shard_assignments = 17;
  repeated AttestationRecord attestation_records = 18;
  repeated RegisteredEnclave enclaves = 19;
  repeated TrainingRecord training_records = 20;
  
  // New params
  FitnessParams fitness_params = 21;
  ReputationDecayParams reputation_params = 22;
  ShardingParams sharding_params = 23;
  ReviewerStakingParams reviewer_staking_params = 24;
}
```

## Update `query.proto`

Add query endpoints for the new types:
```protobuf
// Fitness queries
rpc FitnessRecord(QueryFitnessRecordRequest) returns (QueryFitnessRecordResponse);
rpc FitnessSummary(QueryFitnessSummaryRequest) returns (QueryFitnessSummaryResponse);

// Reputation queries
rpc Reputation(QueryReputationRequest) returns (QueryReputationResponse);
rpc ReputationLeaderboard(QueryReputationLeaderboardRequest) returns (QueryReputationLeaderboardResponse);

// Sharding queries
rpc ShardAssignment(QueryShardAssignmentRequest) returns (QueryShardAssignmentResponse);
rpc ShardStatus(QueryShardStatusRequest) returns (QueryShardStatusResponse);

// TEE/Training queries
rpc Enclave(QueryEnclaveRequest) returns (QueryEnclaveResponse);
rpc TrainingHistory(QueryTrainingHistoryRequest) returns (QueryTrainingHistoryResponse);
```

## Go Cleanup

After proto definitions are in place:

1. **Remove manual Go types** that are now generated:
   - Delete `x/knowledge/types/msg_attest_storage.go` (manual proto encoding)
   - Delete `x/knowledge/types/tee_msgs.go` (manual message types)
   - Remove `MsgRecordTraining` from `training.go` (keep event constants)
   - Remove `EnclaveRecord` from `tee.go` (use proto `RegisteredEnclave`)

2. **Update existing Go types to use proto-generated structs**:
   - `fitness.go` — keep lifecycle helpers, remove struct definitions
   - `reputation.go` — keep decay logic, remove param struct
   - `sharding.go` — keep assignment logic, remove param struct
   - `params_reviewer.go` — keep validation, remove struct

3. **Update keeper code** to use proto types:
   - Replace JSON marshal/unmarshal with proto Marshal/Unmarshal in KVStore operations
   - Update genesis export/import to use new GenesisState fields

4. **Run `make proto-gen`** to regenerate Go code from proto definitions
   - If buf.build BSR is unavailable, manually ensure the generated types match

## Tests

- All existing tests must still pass after migration
- Verify genesis export → import round-trips with proto encoding
- Verify that the new query endpoints return correct data
- Test that MsgAttestStorage, TEE messages, and MsgRecordTraining work with proto encoding

## Constraints

- DO NOT break backward compatibility with existing KVStore data
  - Migration helper: if old JSON data exists, read it and re-store as proto
- Maintain all existing ValidateBasic() logic
- Keep event constants and attribute keys unchanged
- Field numbers in proto must not conflict with existing generated code
