package types

import (
	"encoding/json"
	"fmt"
)

// ShardingParams holds governance-tunable parameters for dataset sharding.
// Stored as JSON in the KVStore to avoid .proto file changes.
type ShardingParams struct {
	// ReplicationFactor: number of validators that store each TDU. Default 3.
	ReplicationFactor uint32 `json:"replication_factor"`
	// SnapshotInterval: blocks between shard reshuffles. Default 1000.
	SnapshotInterval uint64 `json:"snapshot_interval"`
	// MinValidators: minimum validators required for sharding to operate.
	MinValidators uint32 `json:"min_validators"`
}

// DefaultShardingParams returns sensible defaults.
func DefaultShardingParams() ShardingParams {
	return ShardingParams{
		ReplicationFactor: 3,
		SnapshotInterval:  1000,
		MinValidators:     3,
	}
}

// Validate checks all fields are within valid ranges.
func (p ShardingParams) Validate() error {
	if p.ReplicationFactor == 0 {
		return fmt.Errorf("replication_factor must be > 0, got %d", p.ReplicationFactor)
	}
	if p.SnapshotInterval == 0 {
		return fmt.Errorf("snapshot_interval must be > 0, got %d", p.SnapshotInterval)
	}
	if p.MinValidators == 0 {
		return fmt.Errorf("min_validators must be > 0, got %d", p.MinValidators)
	}
	if p.ReplicationFactor > p.MinValidators {
		return fmt.Errorf("replication_factor (%d) must be <= min_validators (%d)",
			p.ReplicationFactor, p.MinValidators)
	}
	return nil
}

// MarshalJSON marshals to JSON.
func (p ShardingParams) MarshalJSON() ([]byte, error) {
	type alias ShardingParams
	return json.Marshal(alias(p))
}

// UnmarshalJSON unmarshals from JSON.
func (p *ShardingParams) UnmarshalJSON(bz []byte) error {
	type alias ShardingParams
	var a alias
	if err := json.Unmarshal(bz, &a); err != nil {
		return err
	}
	*p = ShardingParams(a)
	return nil
}

// ShardAssignment records which TDU IDs a validator is responsible for at a given snapshot.
type ShardAssignment struct {
	ValidatorAddr  string   `json:"validator_addr"`
	TDUIDs         []string `json:"tdu_ids"`
	SnapshotHeight int64    `json:"snapshot_height"`
	Seed           []byte   `json:"seed"`
}

// StorageAttestation records a validator's proof-of-storage attestation for a snapshot cycle.
type StorageAttestation struct {
	ValidatorAddr  string `json:"validator_addr"`
	SnapshotHeight int64  `json:"snapshot_height"`
	AttestationHex string `json:"attestation_hex"` // hex-encoded attestation data
	BlockHeight    int64  `json:"block_height"`    // block at which attestation was recorded
}

// MsgAttestStorage is a Go-only message type for validators to submit proof-of-storage attestations.
// Not proto-generated — handled through the keeper directly.
type MsgAttestStorage struct {
	ValidatorAddr  string `json:"validator_addr"`
	SnapshotHeight int64  `json:"snapshot_height"`
	AttestationHex string `json:"attestation_hex"` // signed hash of assigned TDU data
}

// ShardingGenesisState holds sharding-specific genesis state.
// Exported/imported as JSON alongside the main GenesisState.
type ShardingGenesisState struct {
	Params       ShardingParams       `json:"params"`
	Assignments  []ShardAssignment    `json:"assignments"`
	Attestations []StorageAttestation `json:"attestations"`
}

// DefaultShardingGenesisState returns an empty sharding genesis state with defaults.
func DefaultShardingGenesisState() ShardingGenesisState {
	return ShardingGenesisState{
		Params: DefaultShardingParams(),
	}
}

// Sharding event types.
const (
	EventShardReshuffle          = "shard_reshuffle"
	EventShardReshuffleSkipped   = "shard_reshuffle_skipped"
	EventStorageAttested         = "storage_attested"
	EventMissingStorageAttestation = "missing_storage_attestation"

	AttributeSnapshotHeight = "snapshot_height"
	AttributeValidatorCount = "validator_count"
	AttributeTDUCount       = "tdu_count"
	AttributeValidatorAddr  = "validator_addr"
	AttributeReason         = "reason"
)
