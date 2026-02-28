package keeper_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/partnerships/types"
)

// ---------- Mock PacingKeeper ----------

type mockPacingKeeper struct {
	creationBps uint64
	analysisBps uint64
}

func (m *mockPacingKeeper) GetGlobalPacingMultiplier(_ context.Context) (uint64, uint64) {
	return m.creationBps, m.analysisBps
}

// ---------- Tests: Adaptive Formation Interval (R29-6) ----------

func TestPacing_NoPacingKeeper_BaseInterval(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	// Default FormationMatchIntervalBlocks = 100
	params := k.GetParams(ctx)
	require.Equal(t, uint64(100), params.FormationMatchIntervalBlocks)

	// Set up two pool entries for matching
	params.MatchAcceptanceBlocks = 1000
	k.SetParams(ctx, params)

	k.SetPoolEntry(ctx, &types.PoolEntry{
		Address:       humanAddr,
		Domains:       []string{"test"},
		PreferredRole: "human",
		Status:        "active",
		RegisteredAt:  1,
	})
	k.SetPoolEntry(ctx, &types.PoolEntry{
		Address:       agentAddr,
		Domains:       []string{"test"},
		PreferredRole: "agent",
		Status:        "active",
		RegisteredAt:  1,
	})

	// No pacing keeper set — should use base interval 100.
	// Block 100: should trigger matching (100 % 100 == 0)
	ctx100 := ctxAtHeight(ctx, 100)
	k.RunFormationMatching(ctx100)
	matches := k.GetAllFormationMatches(ctx100)
	assert.Len(t, matches, 1, "should match at block 100 (base interval)")
}

func TestPacing_NoPacingKeeper_NonIntervalBlock(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	params := k.GetParams(ctx)
	params.MatchAcceptanceBlocks = 1000
	k.SetParams(ctx, params)

	k.SetPoolEntry(ctx, &types.PoolEntry{
		Address:       humanAddr,
		Domains:       []string{"test"},
		PreferredRole: "human",
		Status:        "active",
		RegisteredAt:  1,
	})
	k.SetPoolEntry(ctx, &types.PoolEntry{
		Address:       agentAddr,
		Domains:       []string{"test"},
		PreferredRole: "agent",
		Status:        "active",
		RegisteredAt:  1,
	})

	// Block 99: should NOT trigger matching (99 % 100 != 0)
	ctx99 := ctxAtHeight(ctx, 99)
	k.RunFormationMatching(ctx99)
	matches := k.GetAllFormationMatches(ctx99)
	assert.Len(t, matches, 0, "should NOT match at block 99 (not on interval)")
}

func TestPacing_Degraded_CreationBps750000(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	// Base interval = 100, creationBps = 750_000 (75%)
	// effectiveInterval = 100 * 1_000_000 / 750_000 = 133
	params := k.GetParams(ctx)
	params.MatchAcceptanceBlocks = 1000
	k.SetParams(ctx, params)

	k.SetPoolEntry(ctx, &types.PoolEntry{
		Address:       humanAddr,
		Domains:       []string{"test"},
		PreferredRole: "human",
		Status:        "active",
		RegisteredAt:  1,
	})
	k.SetPoolEntry(ctx, &types.PoolEntry{
		Address:       agentAddr,
		Domains:       []string{"test"},
		PreferredRole: "agent",
		Status:        "active",
		RegisteredAt:  1,
	})

	pk := &mockPacingKeeper{creationBps: 750_000, analysisBps: 1_000_000}
	k.SetPacingKeeper(pk)

	// Block 100: should NOT trigger (100 % 133 != 0)
	ctx100 := ctxAtHeight(ctx, 100)
	k.RunFormationMatching(ctx100)
	matches := k.GetAllFormationMatches(ctx100)
	assert.Len(t, matches, 0, "should NOT match at block 100 with degraded pacing (effective=133)")

	// Block 133: should trigger (133 % 133 == 0)
	ctx133 := ctxAtHeight(ctx, 133)
	k.RunFormationMatching(ctx133)
	matches = k.GetAllFormationMatches(ctx133)
	assert.Len(t, matches, 1, "should match at block 133 with degraded pacing (effective=133)")
}

func TestPacing_Critical_CreationBps500000(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	// Base interval = 100, creationBps = 500_000 (50%)
	// effectiveInterval = 100 * 1_000_000 / 500_000 = 200
	params := k.GetParams(ctx)
	params.MatchAcceptanceBlocks = 1000
	k.SetParams(ctx, params)

	k.SetPoolEntry(ctx, &types.PoolEntry{
		Address:       humanAddr,
		Domains:       []string{"test"},
		PreferredRole: "human",
		Status:        "active",
		RegisteredAt:  1,
	})
	k.SetPoolEntry(ctx, &types.PoolEntry{
		Address:       agentAddr,
		Domains:       []string{"test"},
		PreferredRole: "agent",
		Status:        "active",
		RegisteredAt:  1,
	})

	pk := &mockPacingKeeper{creationBps: 500_000, analysisBps: 1_000_000}
	k.SetPacingKeeper(pk)

	// Block 100: should NOT trigger (100 % 200 != 0)
	ctx100 := ctxAtHeight(ctx, 100)
	k.RunFormationMatching(ctx100)
	matches := k.GetAllFormationMatches(ctx100)
	assert.Len(t, matches, 0, "should NOT match at block 100 with critical pacing (effective=200)")

	// Block 200: should trigger (200 % 200 == 0)
	ctx200 := ctxAtHeight(ctx, 200)
	k.RunFormationMatching(ctx200)
	matches = k.GetAllFormationMatches(ctx200)
	assert.Len(t, matches, 1, "should match at block 200 with critical pacing (effective=200)")
}

func TestPacing_Neutral_CreationBps1000000(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	// creationBps = 1_000_000 means no adjustment (guard: != 1_000_000)
	params := k.GetParams(ctx)
	params.MatchAcceptanceBlocks = 1000
	k.SetParams(ctx, params)

	k.SetPoolEntry(ctx, &types.PoolEntry{
		Address:       humanAddr,
		Domains:       []string{"test"},
		PreferredRole: "human",
		Status:        "active",
		RegisteredAt:  1,
	})
	k.SetPoolEntry(ctx, &types.PoolEntry{
		Address:       agentAddr,
		Domains:       []string{"test"},
		PreferredRole: "agent",
		Status:        "active",
		RegisteredAt:  1,
	})

	pk := &mockPacingKeeper{creationBps: 1_000_000, analysisBps: 1_000_000}
	k.SetPacingKeeper(pk)

	// Block 100: should trigger at base interval (neutral pacing)
	ctx100 := ctxAtHeight(ctx, 100)
	k.RunFormationMatching(ctx100)
	matches := k.GetAllFormationMatches(ctx100)
	assert.Len(t, matches, 1, "should match at block 100 with neutral pacing (base interval)")
}
