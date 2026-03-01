package keeper_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/discovery/types"
)

// ---------- Mock PacingKeeper ----------

type mockPacingKeeper struct {
	creationBps uint64
	analysisBps uint64
}

func (m *mockPacingKeeper) GetGlobalPacingMultiplier(_ context.Context) (uint64, uint64) {
	return m.creationBps, m.analysisBps
}

// ctxAtHeight returns a new context at the given block height.
func ctxAtHeight(ctx sdk.Context, height int64) sdk.Context {
	return ctx.WithBlockHeight(height)
}

// ---------- Tests: Adaptive Expiry Check Interval (R29-6) ----------

func TestPacing_NoPacingKeeper_BaseExpiryInterval(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	// Default ProfileExpiryBlocks = 100000
	params := k.GetParams(ctx)
	require.Equal(t, uint64(100000), params.ProfileExpiryBlocks)

	// Use short expiry for test
	params.ProfileExpiryBlocks = 500
	k.SetParams(ctx, params)

	k.SetProfile(ctx, &types.AgentProfile{
		Address:         "zrn1expiredaddr0000000000000000000",
		Domains:         []string{"test"},
		Status:          "active",
		LastActiveBlock: 1,
	})

	// No pacing keeper set — should use base interval 100.
	// Block 100: trigger (100 % 100 == 0), but 1 + 500 = 501 > 100, not expired
	ctx100 := ctxAtHeight(ctx, 100)
	err := k.BeginBlocker(ctx100)
	require.NoError(t, err)

	profile, found := k.GetProfile(ctx100, "zrn1expiredaddr0000000000000000000")
	require.True(t, found)
	assert.Equal(t, "active", profile.Status, "profile should still be active at block 100")

	// Block 600: trigger (600 % 100 == 0), and 1 + 500 = 501 < 600, so expires
	ctx600 := ctxAtHeight(ctx, 600)
	err = k.BeginBlocker(ctx600)
	require.NoError(t, err)

	profile, found = k.GetProfile(ctx600, "zrn1expiredaddr0000000000000000000")
	require.True(t, found)
	assert.Equal(t, "expired", profile.Status, "profile should be expired at block 600 (base interval)")
}

func TestPacing_NoPacingKeeper_NonIntervalBlock(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	params := k.GetParams(ctx)
	params.ProfileExpiryBlocks = 500
	k.SetParams(ctx, params)

	k.SetProfile(ctx, &types.AgentProfile{
		Address:         "zrn1expiredaddr0000000000000000000",
		Domains:         []string{"test"},
		Status:          "active",
		LastActiveBlock: 1,
	})

	// Block 599: NOT on interval (599 % 100 != 0), no expiry check
	ctx599 := ctxAtHeight(ctx, 599)
	err := k.BeginBlocker(ctx599)
	require.NoError(t, err)

	profile, found := k.GetProfile(ctx599, "zrn1expiredaddr0000000000000000000")
	require.True(t, found)
	assert.Equal(t, "active", profile.Status, "should NOT expire at non-interval block 599")
}

func TestPacing_Degraded_CreationBps750000(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	params := k.GetParams(ctx)
	params.ProfileExpiryBlocks = 500
	k.SetParams(ctx, params)

	k.SetProfile(ctx, &types.AgentProfile{
		Address:         "zrn1expiredaddr0000000000000000000",
		Domains:         []string{"test"},
		Status:          "active",
		LastActiveBlock: 1,
	})

	// Base interval = 100, creationBps = 750_000 (75%)
	// effectiveInterval = 100 * 1_000_000 / 750_000 = 133
	pk := &mockPacingKeeper{creationBps: 750_000, analysisBps: 1_000_000}
	k.SetPacingKeeper(pk)

	// Block 600: should NOT trigger (600 % 133 = 68 != 0)
	ctx600 := ctxAtHeight(ctx, 600)
	err := k.BeginBlocker(ctx600)
	require.NoError(t, err)

	profile, found := k.GetProfile(ctx600, "zrn1expiredaddr0000000000000000000")
	require.True(t, found)
	assert.Equal(t, "active", profile.Status, "should NOT expire at block 600 with degraded pacing (effective=133)")

	// Block 665 (= 133*5): should trigger, and profile should be expired
	// (LastActiveBlock=1 + expiryBlocks=500 = 501 < 665)
	ctx665 := ctxAtHeight(ctx, 665)
	err = k.BeginBlocker(ctx665)
	require.NoError(t, err)

	profile, found = k.GetProfile(ctx665, "zrn1expiredaddr0000000000000000000")
	require.True(t, found)
	assert.Equal(t, "expired", profile.Status, "should expire at block 665 (133*5) with degraded pacing")
}

func TestPacing_Critical_CreationBps500000(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	params := k.GetParams(ctx)
	params.ProfileExpiryBlocks = 500
	k.SetParams(ctx, params)

	k.SetProfile(ctx, &types.AgentProfile{
		Address:         "zrn1expiredaddr0000000000000000000",
		Domains:         []string{"test"},
		Status:          "active",
		LastActiveBlock: 1,
	})

	// Base interval = 100, creationBps = 500_000 (50%)
	// effectiveInterval = 100 * 1_000_000 / 500_000 = 200
	pk := &mockPacingKeeper{creationBps: 500_000, analysisBps: 1_000_000}
	k.SetPacingKeeper(pk)

	// Block 500: should NOT trigger (500 % 200 = 100 != 0)
	ctx500 := ctxAtHeight(ctx, 500)
	err := k.BeginBlocker(ctx500)
	require.NoError(t, err)

	profile, found := k.GetProfile(ctx500, "zrn1expiredaddr0000000000000000000")
	require.True(t, found)
	assert.Equal(t, "active", profile.Status, "should NOT expire at block 500 (not on 200 interval)")

	// Block 600 (= 200*3): should trigger, and profile should expire
	// (LastActiveBlock=1 + expiryBlocks=500 = 501 < 600)
	ctx600 := ctxAtHeight(ctx, 600)
	err = k.BeginBlocker(ctx600)
	require.NoError(t, err)

	profile, found = k.GetProfile(ctx600, "zrn1expiredaddr0000000000000000000")
	require.True(t, found)
	assert.Equal(t, "expired", profile.Status, "should expire at block 600 (200*3) with critical pacing")
}

func TestPacing_Neutral_CreationBps1000000(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	params := k.GetParams(ctx)
	params.ProfileExpiryBlocks = 500
	k.SetParams(ctx, params)

	k.SetProfile(ctx, &types.AgentProfile{
		Address:         "zrn1expiredaddr0000000000000000000",
		Domains:         []string{"test"},
		Status:          "active",
		LastActiveBlock: 1,
	})

	// creationBps = 1_000_000 means no adjustment (guard: != 1_000_000)
	// effectiveInterval stays at 100
	pk := &mockPacingKeeper{creationBps: 1_000_000, analysisBps: 1_000_000}
	k.SetPacingKeeper(pk)

	// Block 600: should trigger at base interval (600 % 100 == 0), and expire
	ctx600 := ctxAtHeight(ctx, 600)
	err := k.BeginBlocker(ctx600)
	require.NoError(t, err)

	profile, found := k.GetProfile(ctx600, "zrn1expiredaddr0000000000000000000")
	require.True(t, found)
	assert.Equal(t, "expired", profile.Status, "should expire at block 600 with neutral pacing (base interval)")
}
