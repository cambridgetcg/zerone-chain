package keeper

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Sharding Params CRUD ───────────────────────────────────────────────────

// SetShardingParams stores the sharding parameters as JSON.
func (k Keeper) SetShardingParams(ctx context.Context, params types.ShardingParams) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := params.MarshalJSON()
	if err != nil {
		return fmt.Errorf("failed to marshal sharding params: %w", err)
	}
	return store.Set(types.ShardingParamsKey, bz)
}

// GetShardingParams returns the sharding parameters, or defaults if unset.
func (k Keeper) GetShardingParams(ctx context.Context) types.ShardingParams {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.ShardingParamsKey)
	if err != nil || bz == nil {
		return types.DefaultShardingParams()
	}
	var params types.ShardingParams
	if err := params.UnmarshalJSON(bz); err != nil {
		return types.DefaultShardingParams()
	}
	return params
}

// ─── Shard Assignment CRUD ──────────────────────────────────────────────────

// SetShardAssignment stores a shard assignment as JSON.
func (k Keeper) SetShardAssignment(ctx context.Context, assignment types.ShardAssignment) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(assignment)
	if err != nil {
		return fmt.Errorf("failed to marshal shard assignment: %w", err)
	}
	return store.Set(types.ShardAssignmentKey(assignment.ValidatorAddr, assignment.SnapshotHeight), bz)
}

// GetShardAssignment returns a validator's shard assignment at a snapshot height.
func (k Keeper) GetShardAssignment(ctx context.Context, validatorAddr string, snapshotHeight int64) (types.ShardAssignment, bool) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.ShardAssignmentKey(validatorAddr, snapshotHeight))
	if err != nil || bz == nil {
		return types.ShardAssignment{}, false
	}
	var assignment types.ShardAssignment
	if err := json.Unmarshal(bz, &assignment); err != nil {
		return types.ShardAssignment{}, false
	}
	return assignment, true
}

// DeleteShardAssignment removes a shard assignment.
func (k Keeper) DeleteShardAssignment(ctx context.Context, validatorAddr string, snapshotHeight int64) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Delete(types.ShardAssignmentKey(validatorAddr, snapshotHeight))
}

// ─── Storage Attestation CRUD ───────────────────────────────────────────────

// SetStorageAttestation stores a proof-of-storage attestation as JSON.
func (k Keeper) SetStorageAttestation(ctx context.Context, attestation types.StorageAttestation) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(attestation)
	if err != nil {
		return fmt.Errorf("failed to marshal storage attestation: %w", err)
	}
	return store.Set(types.ShardAttestationKey(attestation.ValidatorAddr, attestation.SnapshotHeight), bz)
}

// GetStorageAttestation returns a validator's attestation for a snapshot.
func (k Keeper) GetStorageAttestation(ctx context.Context, validatorAddr string, snapshotHeight int64) (types.StorageAttestation, bool) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.ShardAttestationKey(validatorAddr, snapshotHeight))
	if err != nil || bz == nil {
		return types.StorageAttestation{}, false
	}
	var attestation types.StorageAttestation
	if err := json.Unmarshal(bz, &attestation); err != nil {
		return types.StorageAttestation{}, false
	}
	return attestation, true
}

// ─── Core Sharding Logic ────────────────────────────────────────────────────

// ComputeShardAssignments deterministically assigns TDUs to validators using SHA-256 seeding.
// For each TDU, seed = SHA-256(tdu_hash || snapshot_block_hash), then deterministic_select_R(seed, validators).
// Returns a map of validator address → assigned TDU IDs.
func ComputeShardAssignments(snapshotBlockHash []byte, tduHashes []string, validators []string, replicationFactor uint32) map[string][]string {
	result := make(map[string][]string, len(validators))
	for _, v := range validators {
		result[v] = nil
	}

	if len(validators) == 0 || len(tduHashes) == 0 {
		return result
	}

	// Sort validators for deterministic ordering
	sortedValidators := make([]string, len(validators))
	copy(sortedValidators, validators)
	sort.Strings(sortedValidators)

	// Cap replication factor at number of validators
	r := replicationFactor
	if uint32(len(sortedValidators)) < r {
		r = uint32(len(sortedValidators))
	}

	for _, tduHash := range tduHashes {
		// Compute seed: SHA-256(tdu_hash || snapshot_block_hash)
		h := sha256.New()
		h.Write([]byte(tduHash))
		h.Write(snapshotBlockHash)
		seed := h.Sum(nil)

		// Deterministically select R validators using seed
		selected := deterministicSelectR(seed, sortedValidators, r)
		for _, v := range selected {
			result[v] = append(result[v], tduHash)
		}
	}

	return result
}

// deterministicSelectR uses a Fisher-Yates shuffle seeded by the hash to select R validators.
// This ensures deterministic, uniform selection without replacement.
func deterministicSelectR(seed []byte, validators []string, r uint32) []string {
	n := uint32(len(validators))
	if r >= n {
		// All validators are selected
		out := make([]string, n)
		copy(out, validators)
		return out
	}

	// Create a working copy of indices
	indices := make([]uint32, n)
	for i := uint32(0); i < n; i++ {
		indices[i] = i
	}

	// Use seed bytes to perform partial Fisher-Yates shuffle
	// Re-hash when we exhaust seed bytes
	currentSeed := seed
	seedOffset := 0

	selected := make([]string, 0, r)
	for i := uint32(0); i < r; i++ {
		// Get 4 bytes from seed for random index
		if seedOffset+4 > len(currentSeed) {
			// Re-hash to get more randomness
			h := sha256.Sum256(currentSeed)
			currentSeed = h[:]
			seedOffset = 0
		}
		randVal := binary.BigEndian.Uint32(currentSeed[seedOffset : seedOffset+4])
		seedOffset += 4

		// Pick from remaining candidates [i..n)
		remaining := n - i
		j := i + (randVal % remaining)

		// Swap
		indices[i], indices[j] = indices[j], indices[i]

		selected = append(selected, validators[indices[i]])
	}

	return selected
}

// GetValidatorShard retrieves the assigned TDU IDs for a validator at a given snapshot.
func (k Keeper) GetValidatorShard(ctx context.Context, validatorAddr string, snapshotHeight int64) ([]string, error) {
	assignment, found := k.GetShardAssignment(ctx, validatorAddr, snapshotHeight)
	if !found {
		return nil, types.ErrShardAssignmentNotFound
	}
	return assignment.TDUIDs, nil
}

// AttestProofOfStorage records a validator's proof-of-storage attestation for a snapshot cycle.
func (k Keeper) AttestProofOfStorage(ctx context.Context, validatorAddr string, snapshotHeight int64, attestation string, blockHeight int64) error {
	if attestation == "" {
		return types.ErrInvalidAttestation
	}

	// Verify the validator has a shard assignment for this snapshot
	_, found := k.GetShardAssignment(ctx, validatorAddr, snapshotHeight)
	if !found {
		return types.ErrShardAssignmentNotFound
	}

	record := types.StorageAttestation{
		ValidatorAddr:  validatorAddr,
		SnapshotHeight: snapshotHeight,
		AttestationHex: attestation,
		BlockHeight:    blockHeight,
	}
	return k.SetStorageAttestation(ctx, record)
}

// ApplyShardAssignments computes and persists shard assignments for a snapshot cycle.
// This is the main entry point called at each snapshot interval.
func (k Keeper) ApplyShardAssignments(ctx context.Context, snapshotBlockHash []byte, snapshotHeight int64, tduHashes []string, validators []string) error {
	params := k.GetShardingParams(ctx)

	if uint32(len(validators)) < params.MinValidators {
		return types.ErrShardingInsufficientValidators
	}

	assignments := ComputeShardAssignments(snapshotBlockHash, tduHashes, validators, params.ReplicationFactor)

	// Compute a global seed for reference
	globalSeed := sha256.Sum256(snapshotBlockHash)

	for validatorAddr, tduIDs := range assignments {
		assignment := types.ShardAssignment{
			ValidatorAddr:  validatorAddr,
			TDUIDs:         tduIDs,
			SnapshotHeight: snapshotHeight,
			Seed:           globalSeed[:],
		}
		if err := k.SetShardAssignment(ctx, assignment); err != nil {
			return err
		}
	}

	return nil
}
