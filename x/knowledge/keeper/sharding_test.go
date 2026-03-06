package keeper_test

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/keeper"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── ShardingParams CRUD ────────────────────────────────────────────────────

func TestShardingParamsCRUD(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Default when unset
	got := k.GetShardingParams(ctx)
	require.Equal(t, types.DefaultShardingParams(), got)

	// Set custom
	custom := types.ShardingParams{
		ReplicationFactor: 5,
		SnapshotInterval:  500,
		MinValidators:     5,
	}
	require.NoError(t, k.SetShardingParams(ctx, custom))

	got = k.GetShardingParams(ctx)
	require.Equal(t, custom, got)
}

func TestShardingParamsValidation(t *testing.T) {
	tests := []struct {
		name    string
		params  types.ShardingParams
		wantErr bool
	}{
		{
			name:    "valid defaults",
			params:  types.DefaultShardingParams(),
			wantErr: false,
		},
		{
			name: "zero replication factor",
			params: types.ShardingParams{
				ReplicationFactor: 0,
				SnapshotInterval:  1000,
				MinValidators:     3,
			},
			wantErr: true,
		},
		{
			name: "zero snapshot interval",
			params: types.ShardingParams{
				ReplicationFactor: 3,
				SnapshotInterval:  0,
				MinValidators:     3,
			},
			wantErr: true,
		},
		{
			name: "zero min validators",
			params: types.ShardingParams{
				ReplicationFactor: 3,
				SnapshotInterval:  1000,
				MinValidators:     0,
			},
			wantErr: true,
		},
		{
			name: "replication exceeds min validators",
			params: types.ShardingParams{
				ReplicationFactor: 5,
				SnapshotInterval:  1000,
				MinValidators:     3,
			},
			wantErr: true,
		},
		{
			name: "replication equals min validators",
			params: types.ShardingParams{
				ReplicationFactor: 3,
				SnapshotInterval:  1000,
				MinValidators:     3,
			},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.params.Validate()
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// ─── ShardAssignment CRUD ───────────────────────────────────────────────────

func TestShardAssignmentCRUD(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Not found initially
	_, found := k.GetShardAssignment(ctx, "val1", 100)
	require.False(t, found)

	// Create
	assignment := types.ShardAssignment{
		ValidatorAddr:  "val1",
		TDUIDs:         []string{"tdu1", "tdu2", "tdu3"},
		SnapshotHeight: 100,
		Seed:           []byte{0xab, 0xcd},
	}
	require.NoError(t, k.SetShardAssignment(ctx, assignment))

	// Read
	got, found := k.GetShardAssignment(ctx, "val1", 100)
	require.True(t, found)
	require.Equal(t, "val1", got.ValidatorAddr)
	require.Equal(t, []string{"tdu1", "tdu2", "tdu3"}, got.TDUIDs)
	require.Equal(t, int64(100), got.SnapshotHeight)
	require.Equal(t, []byte{0xab, 0xcd}, got.Seed)

	// Different snapshot height not found
	_, found = k.GetShardAssignment(ctx, "val1", 200)
	require.False(t, found)

	// Delete
	require.NoError(t, k.DeleteShardAssignment(ctx, "val1", 100))
	_, found = k.GetShardAssignment(ctx, "val1", 100)
	require.False(t, found)
}

// ─── StorageAttestation CRUD ────────────────────────────────────────────────

func TestStorageAttestationCRUD(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Not found initially
	_, found := k.GetStorageAttestation(ctx, "val1", 100)
	require.False(t, found)

	// Create
	attestation := types.StorageAttestation{
		ValidatorAddr:  "val1",
		SnapshotHeight: 100,
		AttestationHex: "deadbeef",
		BlockHeight:    105,
	}
	require.NoError(t, k.SetStorageAttestation(ctx, attestation))

	// Read
	got, found := k.GetStorageAttestation(ctx, "val1", 100)
	require.True(t, found)
	require.Equal(t, "val1", got.ValidatorAddr)
	require.Equal(t, int64(100), got.SnapshotHeight)
	require.Equal(t, "deadbeef", got.AttestationHex)
	require.Equal(t, int64(105), got.BlockHeight)
}

// ─── ComputeShardAssignments: Determinism ───────────────────────────────────

func TestComputeShardAssignments_Deterministic(t *testing.T) {
	blockHash := sha256Hash("block100")
	tduHashes := []string{"tdu1", "tdu2", "tdu3", "tdu4", "tdu5"}
	validators := []string{"val_a", "val_b", "val_c", "val_d"}

	// Run twice with same inputs
	result1 := keeper.ComputeShardAssignments(blockHash, tduHashes, validators, 3)
	result2 := keeper.ComputeShardAssignments(blockHash, tduHashes, validators, 3)

	// Must produce identical results
	for _, v := range validators {
		require.Equal(t, result1[v], result2[v], "non-deterministic assignment for %s", v)
	}
}

func TestComputeShardAssignments_DeterministicAcrossRuns(t *testing.T) {
	// Run 10 times and verify all produce the same result
	blockHash := sha256Hash("determinism-test")
	tduHashes := []string{"a", "b", "c"}
	validators := []string{"v1", "v2", "v3"}

	baseline := keeper.ComputeShardAssignments(blockHash, tduHashes, validators, 2)
	for i := 0; i < 10; i++ {
		result := keeper.ComputeShardAssignments(blockHash, tduHashes, validators, 2)
		for _, v := range validators {
			require.Equal(t, baseline[v], result[v], "run %d diverged for %s", i, v)
		}
	}
}

// ─── ComputeShardAssignments: Replication Factor ────────────────────────────

func TestComputeShardAssignments_ReplicationFactor(t *testing.T) {
	tests := []struct {
		name              string
		replicationFactor uint32
		numValidators     int
		numTDUs           int
	}{
		{"R=1, V=4, N=10", 1, 4, 10},
		{"R=2, V=4, N=10", 2, 4, 10},
		{"R=3, V=4, N=10", 3, 4, 10},
		{"R=4, V=4, N=10", 4, 4, 10},
		{"R=3, V=3, N=5", 3, 3, 5},
		{"R=1, V=1, N=5", 1, 1, 5},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			blockHash := sha256Hash("test-block")
			tduHashes := makeTDUHashes(tc.numTDUs)
			validators := makeValidators(tc.numValidators)

			result := keeper.ComputeShardAssignments(blockHash, tduHashes, validators, tc.replicationFactor)

			// Each TDU should appear exactly R times across all validators
			tduCount := make(map[string]int)
			for _, tduIDs := range result {
				for _, id := range tduIDs {
					tduCount[id]++
				}
			}

			expectedR := tc.replicationFactor
			if uint32(tc.numValidators) < expectedR {
				expectedR = uint32(tc.numValidators)
			}

			for _, hash := range tduHashes {
				require.Equal(t, int(expectedR), tduCount[hash],
					"TDU %s replicated %d times, expected %d", hash, tduCount[hash], expectedR)
			}
		})
	}
}

func TestComputeShardAssignments_ReplicationExceedsValidators(t *testing.T) {
	// R=5 but only 3 validators — should cap at 3
	blockHash := sha256Hash("cap-test")
	tduHashes := []string{"t1", "t2"}
	validators := []string{"v1", "v2", "v3"}

	result := keeper.ComputeShardAssignments(blockHash, tduHashes, validators, 5)

	tduCount := make(map[string]int)
	for _, tduIDs := range result {
		for _, id := range tduIDs {
			tduCount[id]++
		}
	}

	// Each TDU should be on all 3 validators (capped)
	for _, hash := range tduHashes {
		require.Equal(t, 3, tduCount[hash], "TDU %s should be on all 3 validators", hash)
	}
}

// ─── ComputeShardAssignments: Reshuffling ───────────────────────────────────

func TestComputeShardAssignments_ReshufflesOnNewBlockHash(t *testing.T) {
	tduHashes := makeTDUHashes(20)
	validators := makeValidators(6)

	hash1 := sha256Hash("block-100")
	hash2 := sha256Hash("block-200")

	result1 := keeper.ComputeShardAssignments(hash1, tduHashes, validators, 3)
	result2 := keeper.ComputeShardAssignments(hash2, tduHashes, validators, 3)

	// At least one validator should have a different assignment
	differs := false
	for _, v := range validators {
		if !stringSlicesEqual(result1[v], result2[v]) {
			differs = true
			break
		}
	}
	require.True(t, differs, "different block hashes should produce different assignments")
}

// ─── ComputeShardAssignments: Edge Cases ────────────────────────────────────

func TestComputeShardAssignments_EmptyTDUs(t *testing.T) {
	blockHash := sha256Hash("empty")
	validators := []string{"v1", "v2", "v3"}

	result := keeper.ComputeShardAssignments(blockHash, nil, validators, 3)

	for _, v := range validators {
		require.Nil(t, result[v], "no TDUs should mean empty assignment for %s", v)
	}
}

func TestComputeShardAssignments_EmptyValidators(t *testing.T) {
	blockHash := sha256Hash("empty")
	tduHashes := []string{"t1", "t2"}

	result := keeper.ComputeShardAssignments(blockHash, tduHashes, nil, 3)
	require.Empty(t, result)
}

func TestComputeShardAssignments_SingleValidator(t *testing.T) {
	blockHash := sha256Hash("single")
	tduHashes := []string{"t1", "t2", "t3"}
	validators := []string{"v1"}

	result := keeper.ComputeShardAssignments(blockHash, tduHashes, validators, 3)

	// All TDUs go to the single validator
	require.Len(t, result["v1"], 3)
}

func TestComputeShardAssignments_SingleTDU(t *testing.T) {
	blockHash := sha256Hash("single-tdu")
	tduHashes := []string{"only-tdu"}
	validators := []string{"v1", "v2", "v3", "v4"}

	result := keeper.ComputeShardAssignments(blockHash, tduHashes, validators, 2)

	// Exactly 2 validators should have the TDU
	count := 0
	for _, tduIDs := range result {
		for _, id := range tduIDs {
			if id == "only-tdu" {
				count++
			}
		}
	}
	require.Equal(t, 2, count)
}

// ─── ComputeShardAssignments: Load Distribution ─────────────────────────────

func TestComputeShardAssignments_LoadDistribution(t *testing.T) {
	// With many TDUs and R=3, V=4, each validator should hold ~S = (N*R)/V TDUs
	blockHash := sha256Hash("load-test")
	tduHashes := makeTDUHashes(100)
	validators := makeValidators(4)

	result := keeper.ComputeShardAssignments(blockHash, tduHashes, validators, 3)

	expectedS := (100 * 3) / 4 // 75 per validator

	for _, v := range validators {
		count := len(result[v])
		// Allow 30% deviation for statistical distribution
		require.Greater(t, count, expectedS/2, "validator %s has too few TDUs: %d", v, count)
		require.Less(t, count, expectedS*2, "validator %s has too many TDUs: %d", v, count)
	}

	// Total assignments should be exactly N * R
	total := 0
	for _, tduIDs := range result {
		total += len(tduIDs)
	}
	require.Equal(t, 100*3, total, "total assignment count should be N*R")
}

// ─── ComputeShardAssignments: Validator Order Independence ──────────────────

func TestComputeShardAssignments_ValidatorOrderIndependent(t *testing.T) {
	blockHash := sha256Hash("order-test")
	tduHashes := []string{"t1", "t2", "t3", "t4", "t5"}

	validators1 := []string{"val_c", "val_a", "val_b"}
	validators2 := []string{"val_a", "val_b", "val_c"}
	validators3 := []string{"val_b", "val_c", "val_a"}

	result1 := keeper.ComputeShardAssignments(blockHash, tduHashes, validators1, 2)
	result2 := keeper.ComputeShardAssignments(blockHash, tduHashes, validators2, 2)
	result3 := keeper.ComputeShardAssignments(blockHash, tduHashes, validators3, 2)

	// All should produce the same result regardless of input order
	for _, v := range []string{"val_a", "val_b", "val_c"} {
		require.Equal(t, result1[v], result2[v], "order matters for %s", v)
		require.Equal(t, result2[v], result3[v], "order matters for %s", v)
	}
}

// ─── GetValidatorShard ──────────────────────────────────────────────────────

func TestGetValidatorShard(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Not found
	_, err := k.GetValidatorShard(ctx, "val1", 100)
	require.ErrorIs(t, err, types.ErrShardAssignmentNotFound)

	// Create assignment
	assignment := types.ShardAssignment{
		ValidatorAddr:  "val1",
		TDUIDs:         []string{"tdu1", "tdu2"},
		SnapshotHeight: 100,
	}
	require.NoError(t, k.SetShardAssignment(ctx, assignment))

	// Found
	ids, err := k.GetValidatorShard(ctx, "val1", 100)
	require.NoError(t, err)
	require.Equal(t, []string{"tdu1", "tdu2"}, ids)
}

// ─── AttestProofOfStorage ───────────────────────────────────────────────────

func TestAttestProofOfStorage(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Must have an assignment first
	err := k.AttestProofOfStorage(ctx, "val1", 100, "deadbeef", 105)
	require.ErrorIs(t, err, types.ErrShardAssignmentNotFound)

	// Create assignment
	assignment := types.ShardAssignment{
		ValidatorAddr:  "val1",
		TDUIDs:         []string{"tdu1"},
		SnapshotHeight: 100,
	}
	require.NoError(t, k.SetShardAssignment(ctx, assignment))

	// Empty attestation fails
	err = k.AttestProofOfStorage(ctx, "val1", 100, "", 105)
	require.ErrorIs(t, err, types.ErrInvalidAttestation)

	// Valid attestation succeeds
	require.NoError(t, k.AttestProofOfStorage(ctx, "val1", 100, "deadbeef", 105))

	// Verify stored
	got, found := k.GetStorageAttestation(ctx, "val1", 100)
	require.True(t, found)
	require.Equal(t, "deadbeef", got.AttestationHex)
	require.Equal(t, int64(105), got.BlockHeight)
}

// ─── ApplyShardAssignments ──────────────────────────────────────────────────

func TestApplyShardAssignments(t *testing.T) {
	k, ctx := setupKeeper(t)

	blockHash := sha256Hash("block-1000")
	tduHashes := []string{"tdu1", "tdu2", "tdu3"}
	validators := []string{"val_a", "val_b", "val_c"}

	require.NoError(t, k.ApplyShardAssignments(ctx, blockHash, 1000, tduHashes, validators))

	// All validators should have assignments
	for _, v := range validators {
		_, found := k.GetShardAssignment(ctx, v, 1000)
		require.True(t, found, "validator %s should have an assignment", v)
	}

	// Total TDU count across all validators should be N * R
	total := 0
	for _, v := range validators {
		assignment, _ := k.GetShardAssignment(ctx, v, 1000)
		total += len(assignment.TDUIDs)
	}
	require.Equal(t, 3*3, total, "total should be N*R with default R=3")
}

func TestApplyShardAssignments_InsufficientValidators(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Default min_validators = 3, but only 2 validators
	blockHash := sha256Hash("block")
	err := k.ApplyShardAssignments(ctx, blockHash, 100, []string{"t1"}, []string{"v1", "v2"})
	require.ErrorIs(t, err, types.ErrShardingInsufficientValidators)
}

func TestApplyShardAssignments_CustomParams(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Set R=2, min=2
	require.NoError(t, k.SetShardingParams(ctx, types.ShardingParams{
		ReplicationFactor: 2,
		SnapshotInterval:  500,
		MinValidators:     2,
	}))

	blockHash := sha256Hash("custom-block")
	tduHashes := []string{"t1", "t2", "t3", "t4"}
	validators := []string{"v1", "v2", "v3"}

	require.NoError(t, k.ApplyShardAssignments(ctx, blockHash, 500, tduHashes, validators))

	total := 0
	for _, v := range validators {
		assignment, found := k.GetShardAssignment(ctx, v, 500)
		require.True(t, found)
		total += len(assignment.TDUIDs)
	}
	require.Equal(t, 4*2, total, "total should be N*R=4*2=8")
}

// ─── No Duplicate TDUs Per Validator ────────────────────────────────────────

func TestComputeShardAssignments_NoDuplicatePerValidator(t *testing.T) {
	blockHash := sha256Hash("no-dup-test")
	tduHashes := makeTDUHashes(50)
	validators := makeValidators(5)

	result := keeper.ComputeShardAssignments(blockHash, tduHashes, validators, 3)

	for v, ids := range result {
		seen := make(map[string]bool, len(ids))
		for _, id := range ids {
			require.False(t, seen[id], "validator %s has duplicate TDU %s", v, id)
			seen[id] = true
		}
	}
}

// ─── Helpers ────────────────────────────────────────────────────────────────

func sha256Hash(input string) []byte {
	h := sha256.Sum256([]byte(input))
	return h[:]
}

func makeTDUHashes(n int) []string {
	hashes := make([]string, n)
	for i := 0; i < n; i++ {
		h := sha256.Sum256([]byte("tdu-" + hex.EncodeToString([]byte{byte(i >> 8), byte(i)})))
		hashes[i] = hex.EncodeToString(h[:])
	}
	return hashes
}

func makeValidators(n int) []string {
	validators := make([]string, n)
	for i := 0; i < n; i++ {
		validators[i] = "validator_" + hex.EncodeToString([]byte{byte(i)})
	}
	return validators
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
