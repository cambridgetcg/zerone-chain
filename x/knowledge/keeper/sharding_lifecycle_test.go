package keeper_test

import (
	"context"
	"crypto/sha256"
	"testing"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/keeper"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── BeginBlocker triggers reshuffle at correct interval ─────────────────────

func TestBeginBlocker_ShardReshuffle_AtSnapshotInterval(t *testing.T) {
	k, ctx := setupKeeperWithStaking(t)
	setupShardingDefaults(t, k, ctx)

	// Add fitness records for TDUs (fitness >= 0.1)
	for _, id := range []string{"tdu1", "tdu2", "tdu3"} {
		record := types.NewTDUFitnessRecord(id, sdkmath.NewInt(1_000_000), 0)
		record.SetFitnessScore(sdkmath.LegacyNewDecWithPrec(5, 1)) // 0.5 Active
		require.NoError(t, k.SetFitnessRecord(ctx, record))
	}

	// Block 500: not at snapshot interval (default 1000) → no reshuffle
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ctx = sdkCtx.WithBlockHeight(500).WithEventManager(sdk.NewEventManager())
	require.NoError(t, k.BeginBlocker(ctx))

	sdkCtx = sdk.UnwrapSDKContext(ctx)
	events := sdkCtx.EventManager().Events()
	hasReshuffle := hasEvent(events, types.EventShardReshuffle)
	require.False(t, hasReshuffle, "should not reshuffle at non-interval block")

	// Block 1000: at snapshot interval → reshuffle
	ctx = sdkCtx.WithBlockHeight(1000).WithEventManager(sdk.NewEventManager())
	require.NoError(t, k.BeginBlocker(ctx))

	sdkCtx = sdk.UnwrapSDKContext(ctx)
	events = sdkCtx.EventManager().Events()
	hasReshuffle = hasEvent(events, types.EventShardReshuffle)
	require.True(t, hasReshuffle, "should reshuffle at snapshot interval")
}

// ─── Shard assignments are deterministic ─────────────────────────────────────

func TestShardAssignments_Deterministic(t *testing.T) {
	blockHash := sha256Sum("block100")
	tdus := []string{"tdu1", "tdu2", "tdu3", "tdu4"}
	validators := []string{"val1", "val2", "val3"}

	result1 := keeper.ComputeShardAssignments(blockHash, tdus, validators, 3)
	result2 := keeper.ComputeShardAssignments(blockHash, tdus, validators, 3)

	// Same inputs → same outputs
	for _, v := range validators {
		require.Equal(t, result1[v], result2[v], "assignments should be deterministic for validator %s", v)
	}
}

// ─── Different block hash → different assignments ────────────────────────────

func TestShardAssignments_DifferentBlockHash(t *testing.T) {
	hash1 := sha256Sum("block100")
	hash2 := sha256Sum("block200")
	tdus := []string{"tdu1", "tdu2", "tdu3", "tdu4", "tdu5", "tdu6", "tdu7", "tdu8"}
	validators := []string{"val1", "val2", "val3", "val4"}

	result1 := keeper.ComputeShardAssignments(hash1, tdus, validators, 2)
	result2 := keeper.ComputeShardAssignments(hash2, tdus, validators, 2)

	// With enough TDUs and different hashes, assignments should differ
	differ := false
	for _, v := range validators {
		if len(result1[v]) != len(result2[v]) {
			differ = true
			break
		}
		for i := range result1[v] {
			if result1[v][i] != result2[v][i] {
				differ = true
				break
			}
		}
		if differ {
			break
		}
	}
	require.True(t, differ, "different block hashes should produce different assignments")
}

// ─── Each TDU assigned to exactly R validators ──────────────────────────────

func TestShardAssignments_EachTDU_ExactlyR(t *testing.T) {
	blockHash := sha256Sum("block42")
	tdus := []string{"tdu1", "tdu2", "tdu3"}
	validators := []string{"val1", "val2", "val3", "val4", "val5"}
	r := uint32(3)

	result := keeper.ComputeShardAssignments(blockHash, tdus, validators, r)

	// Count how many validators have each TDU
	tduCount := make(map[string]int)
	for _, tduIDs := range result {
		for _, tduID := range tduIDs {
			tduCount[tduID]++
		}
	}

	for _, tdu := range tdus {
		require.Equal(t, int(r), tduCount[tdu], "TDU %s should be assigned to exactly %d validators", tdu, r)
	}
}

// ─── Genesis export → import round-trips all state ──────────────────────────

func TestShardingGenesis_RoundTrip(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Set params
	params := types.ShardingParams{
		ReplicationFactor: 2,
		SnapshotInterval:  500,
		MinValidators:     2,
	}
	require.NoError(t, k.SetShardingParams(ctx, params))

	// Set assignments
	a1 := types.ShardAssignment{
		ValidatorAddr:  "val1",
		TDUIDs:         []string{"tdu1", "tdu2"},
		SnapshotHeight: 1000,
		Seed:           []byte("seed1"),
	}
	a2 := types.ShardAssignment{
		ValidatorAddr:  "val2",
		TDUIDs:         []string{"tdu2", "tdu3"},
		SnapshotHeight: 1000,
		Seed:           []byte("seed1"),
	}
	require.NoError(t, k.SetShardAssignment(ctx, a1))
	require.NoError(t, k.SetShardAssignment(ctx, a2))

	// Set attestation
	att := types.StorageAttestation{
		ValidatorAddr:  "val1",
		SnapshotHeight: 1000,
		AttestationHex: "deadbeef",
		BlockHeight:    1050,
	}
	require.NoError(t, k.SetStorageAttestation(ctx, att))

	// Export
	exported := k.ExportShardingGenesis(ctx)
	require.Equal(t, params.ReplicationFactor, exported.Params.ReplicationFactor)
	require.Equal(t, params.SnapshotInterval, exported.Params.SnapshotInterval)
	require.Len(t, exported.Assignments, 2)
	require.Len(t, exported.Attestations, 1)

	// Import into fresh keeper
	k2, ctx2 := setupKeeper(t)
	require.NoError(t, k2.ImportShardingGenesis(ctx2, exported))

	// Verify params
	imported := k2.GetShardingParams(ctx2)
	require.Equal(t, params.ReplicationFactor, imported.ReplicationFactor)
	require.Equal(t, params.SnapshotInterval, imported.SnapshotInterval)
	require.Equal(t, params.MinValidators, imported.MinValidators)

	// Verify assignments
	got1, found := k2.GetShardAssignment(ctx2, "val1", 1000)
	require.True(t, found)
	require.Equal(t, []string{"tdu1", "tdu2"}, got1.TDUIDs)

	got2, found := k2.GetShardAssignment(ctx2, "val2", 1000)
	require.True(t, found)
	require.Equal(t, []string{"tdu2", "tdu3"}, got2.TDUIDs)

	// Verify attestation
	gotAtt, found := k2.GetStorageAttestation(ctx2, "val1", 1000)
	require.True(t, found)
	require.Equal(t, "deadbeef", gotAtt.AttestationHex)
	require.Equal(t, int64(1050), gotAtt.BlockHeight)
}

// ─── Pruned TDU excluded from next reshuffle ─────────────────────────────────

func TestShardReshuffle_PrunedTDUExcluded(t *testing.T) {
	k, ctx := setupKeeperWithStaking(t)
	setupShardingDefaults(t, k, ctx)

	// Active TDU: fitness 0.5
	rec1 := types.NewTDUFitnessRecord("active1", sdkmath.NewInt(1_000_000), 0)
	rec1.SetFitnessScore(sdkmath.LegacyNewDecWithPrec(5, 1))
	require.NoError(t, k.SetFitnessRecord(ctx, rec1))

	// Pruned TDU: fitness 0.05 → lifecycle Pruned
	rec2 := types.NewTDUFitnessRecord("pruned1", sdkmath.NewInt(1_000_000), 0)
	rec2.SetFitnessScore(sdkmath.LegacyNewDecWithPrec(5, 2)) // 0.05
	require.NoError(t, k.SetFitnessRecord(ctx, rec2))

	hashes := k.GetActiveTDUHashes(ctx)
	require.Contains(t, hashes, "active1")
	require.NotContains(t, hashes, "pruned1")
}

// ─── Fewer than MinValidators → skip reshuffle with warning ─────────────────

func TestShardReshuffle_InsufficientValidators_Skipped(t *testing.T) {
	k, ctx := setupKeeperWithInsufficientStaking(t)
	setupShardingDefaults(t, k, ctx)

	// Add a TDU
	rec := types.NewTDUFitnessRecord("tdu1", sdkmath.NewInt(1_000_000), 0)
	rec.SetFitnessScore(sdkmath.LegacyNewDecWithPrec(5, 1))
	require.NoError(t, k.SetFitnessRecord(ctx, rec))

	// At snapshot interval
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ctx = sdkCtx.WithBlockHeight(1000).WithEventManager(sdk.NewEventManager())
	require.NoError(t, k.BeginBlocker(ctx))

	sdkCtx = sdk.UnwrapSDKContext(ctx)
	events := sdkCtx.EventManager().Events()
	require.True(t, hasEvent(events, types.EventShardReshuffleSkipped),
		"should emit skip event when insufficient validators")
	require.False(t, hasEvent(events, types.EventShardReshuffle),
		"should not reshuffle when insufficient validators")
}

// ─── MsgAttestStorage handler ───────────────────────────────────────────────

func TestHandleMsgAttestStorage(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Set up assignment first
	a := types.ShardAssignment{
		ValidatorAddr:  "val1",
		TDUIDs:         []string{"tdu1"},
		SnapshotHeight: 1000,
		Seed:           []byte("seed"),
	}
	require.NoError(t, k.SetShardAssignment(ctx, a))

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ctx = sdkCtx.WithBlockHeight(1050).WithEventManager(sdk.NewEventManager())

	msg := &types.MsgAttestStorage{
		ValidatorAddr:  "val1",
		SnapshotHeight: 1000,
		AttestationHex: "abcdef1234",
	}

	require.NoError(t, k.HandleMsgAttestStorage(ctx, msg))

	// Verify attestation recorded
	att, found := k.GetStorageAttestation(ctx, "val1", 1000)
	require.True(t, found)
	require.Equal(t, "abcdef1234", att.AttestationHex)
	require.Equal(t, int64(1050), att.BlockHeight)

	// Verify event emitted
	sdkCtx = sdk.UnwrapSDKContext(ctx)
	require.True(t, hasEvent(sdkCtx.EventManager().Events(), types.EventStorageAttested))
}

func TestHandleMsgAttestStorage_InvalidInputs(t *testing.T) {
	k, ctx := setupKeeper(t)

	tests := []struct {
		name string
		msg  *types.MsgAttestStorage
	}{
		{"empty validator", &types.MsgAttestStorage{ValidatorAddr: "", SnapshotHeight: 1000, AttestationHex: "abc"}},
		{"empty attestation", &types.MsgAttestStorage{ValidatorAddr: "val1", SnapshotHeight: 1000, AttestationHex: ""}},
		{"zero snapshot", &types.MsgAttestStorage{ValidatorAddr: "val1", SnapshotHeight: 0, AttestationHex: "abc"}},
		{"no assignment", &types.MsgAttestStorage{ValidatorAddr: "val1", SnapshotHeight: 999, AttestationHex: "abc"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := k.HandleMsgAttestStorage(ctx, tc.msg)
			require.Error(t, err)
		})
	}
}

// ─── Missing attestation check ──────────────────────────────────────────────

func TestMissingAttestationCheck_AtCorrectInterval(t *testing.T) {
	k, ctx := setupKeeperWithStaking(t)

	params := types.ShardingParams{
		ReplicationFactor: 1,
		SnapshotInterval:  100,
		MinValidators:     3,
	}
	require.NoError(t, k.SetShardingParams(ctx, params))

	// Assignments at snapshot 100
	for _, v := range []string{"val_a", "val_b", "val_c"} {
		a := types.ShardAssignment{
			ValidatorAddr:  v,
			TDUIDs:         []string{"tdu1"},
			SnapshotHeight: 100,
			Seed:           []byte("seed"),
		}
		require.NoError(t, k.SetShardAssignment(ctx, a))
	}

	// val_a attests, val_b and val_c do not
	require.NoError(t, k.SetStorageAttestation(ctx, types.StorageAttestation{
		ValidatorAddr:  "val_a",
		SnapshotHeight: 100,
		AttestationHex: "abc",
		BlockHeight:    150,
	}))

	// Add fitness records
	rec := types.NewTDUFitnessRecord("tdu1", sdkmath.NewInt(100), 0)
	rec.SetFitnessScore(sdkmath.LegacyNewDecWithPrec(5, 1))
	require.NoError(t, k.SetFitnessRecord(ctx, rec))

	// Block 300: checks attestations for snapshot 200 (prevSnapshot = 300-100 = 200)
	// graceCutoff = 300 - 200 = 100. But we set assignments at 100, not 200.
	// So block 300 checks snapshot 200 — no assignments there.

	// Block 200: checks snapshot 100 (prevSnapshot = 200-100 = 100, graceCutoff = 200-200 = 0 > 0 false)
	// graceCutoff = 0, not > 0, so no check. That's correct — not enough time passed.

	// Block 300: prevSnapshot = 200, graceCutoff = 100 > 0.
	// But assignments are at 100, not 200. So no missing attestation events for snapshot 200.

	// The proper test: assignments at snapshot 100, check runs when prevSnapshot=100 and graceCutoff>0.
	// That happens at block 200 when graceCutoff = 200-200 = 0 (not > 0) — doesn't trigger.
	// At block 300: prevSnapshot=200, graceCutoff=100 — checks snapshot 200, no assignments there.

	// To properly test: we need SnapshotInterval small enough and set assignments at the right height.
	// Let me re-approach: SnapshotInterval=100, assignments at snapshot 200.
	// Check runs at block 400: prevSnapshot=300, graceCutoff=200>0.
	// Hmm, that checks snapshot 300 not 200. The check looks at prevSnapshotHeight only.
	// The design: at each snapshot we check the PREVIOUS snapshot's attestations.
	// If I put assignments at snapshot 100, the check at block 200 (prevSnapshot=100, graceCutoff=0) → skipped.
	// Check at block 300: prevSnapshot=200, graceCutoff=100>0 → checks snapshot 200, no assignments.
	// I need assignments at the prevSnapshotHeight for the check to find them.

	// Reset: add assignments at snapshot 200
	for _, v := range []string{"val_a", "val_b", "val_c"} {
		a := types.ShardAssignment{
			ValidatorAddr:  v,
			TDUIDs:         []string{"tdu1"},
			SnapshotHeight: 200,
			Seed:           []byte("seed"),
		}
		require.NoError(t, k.SetShardAssignment(ctx, a))
	}
	// val_a attests for snapshot 200
	require.NoError(t, k.SetStorageAttestation(ctx, types.StorageAttestation{
		ValidatorAddr:  "val_a",
		SnapshotHeight: 200,
		AttestationHex: "abc",
		BlockHeight:    250,
	}))

	// Block 300: prevSnapshot=200, graceCutoff=100>0 → checks snapshot 200
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ctx = sdkCtx.WithBlockHeight(300).WithEventManager(sdk.NewEventManager())
	require.NoError(t, k.BeginBlocker(ctx))

	sdkCtx = sdk.UnwrapSDKContext(ctx)

	// Count missing attestation events
	missingCount := countEvents(sdkCtx.EventManager().Events(), types.EventMissingStorageAttestation)
	require.Equal(t, 2, missingCount, "val_b and val_c should have missing attestation events")
}

// ─── Custom SnapshotInterval triggers reshuffle ─────────────────────────────

func TestShardReshuffle_CustomInterval(t *testing.T) {
	k, ctx := setupKeeperWithStaking(t)

	params := types.ShardingParams{
		ReplicationFactor: 2,
		SnapshotInterval:  200,
		MinValidators:     3,
	}
	require.NoError(t, k.SetShardingParams(ctx, params))

	// Add TDUs
	rec := types.NewTDUFitnessRecord("tdu1", sdkmath.NewInt(100), 0)
	rec.SetFitnessScore(sdkmath.LegacyNewDecWithPrec(5, 1))
	require.NoError(t, k.SetFitnessRecord(ctx, rec))

	// Block 200: at interval → reshuffle
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ctx = sdkCtx.WithBlockHeight(200).WithEventManager(sdk.NewEventManager())
	require.NoError(t, k.BeginBlocker(ctx))

	sdkCtx = sdk.UnwrapSDKContext(ctx)
	require.True(t, hasEvent(sdkCtx.EventManager().Events(), types.EventShardReshuffle))
}

// ─── Reshuffle creates assignments ──────────────────────────────────────────

func TestShardReshuffle_CreatesAssignments(t *testing.T) {
	k, ctx := setupKeeperWithStaking(t)
	setupShardingDefaults(t, k, ctx)

	// Add TDUs
	for _, id := range []string{"tdu1", "tdu2"} {
		rec := types.NewTDUFitnessRecord(id, sdkmath.NewInt(100), 0)
		rec.SetFitnessScore(sdkmath.LegacyNewDecWithPrec(5, 1))
		require.NoError(t, k.SetFitnessRecord(ctx, rec))
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ctx = sdkCtx.WithBlockHeight(1000).WithEventManager(sdk.NewEventManager())
	require.NoError(t, k.BeginBlocker(ctx))

	// Verify assignments were created for all 3 mock validators
	for _, v := range []string{"val1", "val2", "val3"} {
		a, found := k.GetShardAssignment(ctx, v, 1000)
		require.True(t, found, "assignment should exist for %s", v)
		require.Greater(t, len(a.TDUIDs), 0, "validator %s should have TDUs assigned", v)
	}
}

// ─── Helpers ────────────────────────────────────────────────────────────────

// mockStakingKeeper provides a mock staking keeper for sharding tests.
type mockStakingKeeper struct {
	validators []types.ValidatorInfo
}

func (m *mockStakingKeeper) GetActiveValidatorInfos(_ context.Context) ([]types.ValidatorInfo, error) {
	return m.validators, nil
}

func (m *mockStakingKeeper) GetValidatorInfo(_ context.Context, addr string) (*types.ValidatorInfo, error) {
	for _, v := range m.validators {
		if v.Address == addr {
			return &v, nil
		}
	}
	return nil, nil
}

func (m *mockStakingKeeper) GetEffectiveStake(_ context.Context, _ string) (uint64, error) {
	return 1_000_000, nil
}

func (m *mockStakingKeeper) GetTotalStake(_ context.Context) (uint64, error) {
	return 3_000_000, nil
}

func (m *mockStakingKeeper) SlashValidator(_ context.Context, _ string, _ uint64) error {
	return nil
}

func (m *mockStakingKeeper) SlashValidatorToModule(_ context.Context, _ string, _ uint64, _ string) (sdkmath.Int, error) {
	return sdkmath.ZeroInt(), nil
}

func setupKeeperWithStaking(t *testing.T) (keeper.Keeper, context.Context) {
	t.Helper()
	ss := newMockStoreService()
	bk := newMockBankKeeper()
	sk := &mockStakingKeeper{
		validators: []types.ValidatorInfo{
			{Address: "val1", Stake: 1_000_000, Tier: "bonded"},
			{Address: "val2", Stake: 1_000_000, Tier: "bonded"},
			{Address: "val3", Stake: 1_000_000, Tier: "bonded"},
		},
	}
	k := keeper.NewKeeper(ss, nil, "authority", bk, sk)
	ctx := sdk.Context{}.
		WithBlockHeight(100).
		WithEventManager(sdk.NewEventManager()).
		WithMultiStore(&mockCacheMultiStore{})
	return k, ctx
}

func setupKeeperWithInsufficientStaking(t *testing.T) (keeper.Keeper, context.Context) {
	t.Helper()
	ss := newMockStoreService()
	bk := newMockBankKeeper()
	sk := &mockStakingKeeper{
		validators: []types.ValidatorInfo{
			{Address: "val1", Stake: 1_000_000, Tier: "bonded"},
		},
	}
	k := keeper.NewKeeper(ss, nil, "authority", bk, sk)
	ctx := sdk.Context{}.
		WithBlockHeight(100).
		WithEventManager(sdk.NewEventManager()).
		WithMultiStore(&mockCacheMultiStore{})
	return k, ctx
}

func setupShardingDefaults(t *testing.T, k keeper.Keeper, ctx context.Context) {
	t.Helper()
	params := types.DefaultShardingParams()
	require.NoError(t, k.SetShardingParams(ctx, params))

	// Set default module params too (needed for BeginBlocker)
	p := types.DefaultParams()
	_ = k.SetParams(ctx, &p)
}

func sha256Sum(s string) []byte {
	h := sha256.Sum256([]byte(s))
	return h[:]
}

func countEvents(events sdk.Events, eventType string) int {
	count := 0
	for _, e := range events {
		if e.Type == eventType {
			count++
		}
	}
	return count
}

func hasEvent(events sdk.Events, eventType string) bool {
	for _, e := range events {
		if e.Type == eventType {
			return true
		}
	}
	return false
}

