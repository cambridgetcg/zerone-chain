package keeper_test

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── ReputationDecayParams CRUD ─────────────────────────────────────────────

func TestReputationDecayParamsCRUD(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Default when unset
	got := k.GetReputationDecayParams(ctx)
	require.Equal(t, types.DefaultReputationDecayParams(), got)

	// Set custom
	custom := types.ReputationDecayParams{
		DecayRateBps:        1000,  // 10%
		DecayIntervalBlocks: 86400, // ~6 days
		FloorRatioBps:       5000,  // 50%
	}
	require.NoError(t, k.SetReputationDecayParams(ctx, custom))
	got = k.GetReputationDecayParams(ctx)
	require.Equal(t, custom, got)
}

func TestReputationDecayParamsValidation(t *testing.T) {
	tests := []struct {
		name    string
		params  types.ReputationDecayParams
		wantErr bool
	}{
		{"valid defaults", types.DefaultReputationDecayParams(), false},
		{"decay too high", types.ReputationDecayParams{DecayRateBps: 10001, DecayIntervalBlocks: 100, FloorRatioBps: 2500}, true},
		{"zero interval", types.ReputationDecayParams{DecayRateBps: 500, DecayIntervalBlocks: 0, FloorRatioBps: 2500}, true},
		{"floor too high", types.ReputationDecayParams{DecayRateBps: 500, DecayIntervalBlocks: 100, FloorRatioBps: 10001}, true},
		{"zero decay rate", types.ReputationDecayParams{DecayRateBps: 0, DecayIntervalBlocks: 100, FloorRatioBps: 2500}, false},
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

func TestReputationDecayParamsGetters(t *testing.T) {
	p := types.ReputationDecayParams{DecayRateBps: 500, DecayIntervalBlocks: 100, FloorRatioBps: 2500}
	require.Equal(t, "0.050000000000000000", p.GetDecayRate().String())
	require.Equal(t, "0.250000000000000000", p.GetFloorRatio().String())
}

// ─── AgentDomainReputation Type ─────────────────────────────────────────────

func TestAgentDomainReputation_GettersSetters(t *testing.T) {
	r := types.AgentDomainReputation{}

	// Empty returns zero
	require.True(t, r.GetScore().IsZero())
	require.True(t, r.GetPeakScore().IsZero())

	// Set and get
	r.SetScore(sdkmath.LegacyNewDec(100))
	require.Equal(t, "100.000000000000000000", r.Score)
	require.Equal(t, sdkmath.LegacyNewDec(100), r.GetScore())

	r.SetPeakScore(sdkmath.LegacyNewDec(200))
	require.Equal(t, "200.000000000000000000", r.PeakScore)

	// Negative clamps to zero
	r.SetScore(sdkmath.LegacyNewDec(-5))
	require.True(t, r.GetScore().IsZero())
	r.SetPeakScore(sdkmath.LegacyNewDec(-5))
	require.True(t, r.GetPeakScore().IsZero())
}

func TestNewAgentDomainReputation(t *testing.T) {
	rep := types.NewAgentDomainReputation("agent1", "tech", sdkmath.LegacyNewDec(50), 1000)
	require.Equal(t, "agent1", rep.AgentAddr)
	require.Equal(t, "tech", rep.DomainID)
	require.Equal(t, sdkmath.LegacyNewDec(50), rep.GetScore())
	require.Equal(t, sdkmath.LegacyNewDec(50), rep.GetPeakScore())
	require.Equal(t, int64(1000), rep.LastActiveHeight)
}

// ─── AgentDomainReputation Keeper CRUD ──────────────────────────────────────

func TestAgentDomainReputationCRUD(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Not found initially
	_, found := k.GetAgentDomainReputation(ctx, "agent1", "tech")
	require.False(t, found)

	// Create
	rep := types.NewAgentDomainReputation("agent1", "tech", sdkmath.LegacyNewDec(50), 100)
	require.NoError(t, k.SetAgentDomainReputation(ctx, rep))

	// Read
	got, found := k.GetAgentDomainReputation(ctx, "agent1", "tech")
	require.True(t, found)
	require.Equal(t, "agent1", got.AgentAddr)
	require.Equal(t, "tech", got.DomainID)
	require.Equal(t, sdkmath.LegacyNewDec(50), got.GetScore())

	// Update
	got.SetScore(sdkmath.LegacyNewDec(75))
	require.NoError(t, k.SetAgentDomainReputation(ctx, got))
	updated, found := k.GetAgentDomainReputation(ctx, "agent1", "tech")
	require.True(t, found)
	require.Equal(t, sdkmath.LegacyNewDec(75), updated.GetScore())

	// Delete
	require.NoError(t, k.DeleteAgentDomainReputation(ctx, "agent1", "tech"))
	_, found = k.GetAgentDomainReputation(ctx, "agent1", "tech")
	require.False(t, found)
}

func TestAgentDomainReputationIteration(t *testing.T) {
	k, ctx := setupKeeper(t)

	pairs := []struct{ agent, domain string }{
		{"agent1", "tech"},
		{"agent1", "science"},
		{"agent2", "tech"},
	}
	for _, p := range pairs {
		require.NoError(t, k.SetAgentDomainReputation(ctx, types.NewAgentDomainReputation(
			p.agent, p.domain, sdkmath.LegacyNewDec(10), 100)))
	}

	var collected []string
	k.IterateAgentDomainReputations(ctx, func(r types.AgentDomainReputation) bool {
		collected = append(collected, r.AgentAddr+"/"+r.DomainID)
		return false
	})
	require.Len(t, collected, 3)
}

// ─── UpdateReputation ───────────────────────────────────────────────────────

func TestUpdateReputation(t *testing.T) {
	tests := []struct {
		name        string
		setup       *types.AgentDomainReputation // nil = no existing record
		delta       string
		height      int64
		expectScore string
		expectPeak  string
	}{
		{
			name:        "new record with positive delta",
			setup:       nil,
			delta:       "50.000000000000000000",
			height:      100,
			expectScore: "50.000000000000000000",
			expectPeak:  "50.000000000000000000",
		},
		{
			name:        "new record with negative delta clamps to zero",
			setup:       nil,
			delta:       "-10.000000000000000000",
			height:      100,
			expectScore: "0.000000000000000000",
			expectPeak:  "0.000000000000000000",
		},
		{
			name: "add to existing",
			setup: func() *types.AgentDomainReputation {
				r := types.NewAgentDomainReputation("agent1", "tech", sdkmath.LegacyNewDec(50), 50)
				return &r
			}(),
			delta:       "25.000000000000000000",
			height:      200,
			expectScore: "75.000000000000000000",
			expectPeak:  "75.000000000000000000", // new peak
		},
		{
			name: "subtract from existing, peak unchanged",
			setup: func() *types.AgentDomainReputation {
				r := types.NewAgentDomainReputation("agent1", "tech", sdkmath.LegacyNewDec(100), 50)
				return &r
			}(),
			delta:       "-30.000000000000000000",
			height:      200,
			expectScore: "70.000000000000000000",
			expectPeak:  "100.000000000000000000", // peak stays at 100
		},
		{
			name: "subtract past zero clamps",
			setup: func() *types.AgentDomainReputation {
				r := types.NewAgentDomainReputation("agent1", "tech", sdkmath.LegacyNewDec(10), 50)
				return &r
			}(),
			delta:       "-50.000000000000000000",
			height:      200,
			expectScore: "0.000000000000000000",
			expectPeak:  "10.000000000000000000",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			k, ctx := setupKeeper(t)

			if tc.setup != nil {
				require.NoError(t, k.SetAgentDomainReputation(ctx, *tc.setup))
			}

			delta, err := sdkmath.LegacyNewDecFromStr(tc.delta)
			require.NoError(t, err)
			require.NoError(t, k.UpdateReputation(ctx, "agent1", "tech", delta, tc.height))

			got, found := k.GetAgentDomainReputation(ctx, "agent1", "tech")
			require.True(t, found)
			require.Equal(t, tc.expectScore, got.Score)
			require.Equal(t, tc.expectPeak, got.PeakScore)
			require.Equal(t, tc.height, got.LastActiveHeight)
		})
	}
}

// ─── ResetInactivityTimer ───────────────────────────────────────────────────

func TestResetInactivityTimer(t *testing.T) {
	k, ctx := setupKeeper(t)

	rep := types.NewAgentDomainReputation("agent1", "tech", sdkmath.LegacyNewDec(50), 100)
	require.NoError(t, k.SetAgentDomainReputation(ctx, rep))

	// Reset timer to new height
	k.ResetInactivityTimer(ctx, "agent1", "tech", 500)

	got, found := k.GetAgentDomainReputation(ctx, "agent1", "tech")
	require.True(t, found)
	require.Equal(t, int64(500), got.LastActiveHeight)
	// Score unchanged
	require.Equal(t, sdkmath.LegacyNewDec(50), got.GetScore())
}

func TestResetInactivityTimer_NonExistent(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Should not panic on non-existent record
	k.ResetInactivityTimer(ctx, "agent1", "tech", 500)

	_, found := k.GetAgentDomainReputation(ctx, "agent1", "tech")
	require.False(t, found)
}

// ─── ApplyReputationDecay ───────────────────────────────────────────────────

func TestApplyReputationDecay(t *testing.T) {
	tests := []struct {
		name            string
		score           string
		peak            string
		lastActive      int64
		currentHeight   int64
		decayRateBps    uint32
		intervalBlocks  uint64
		floorRatioBps   uint32
		expectMinScore  string
		expectMaxScore  string
		expectDecayed   bool
	}{
		{
			name:           "within interval - no decay",
			score:          "100.000000000000000000",
			peak:           "100.000000000000000000",
			lastActive:     900,
			currentHeight:  1000,
			decayRateBps:   500,
			intervalBlocks: 200,
			floorRatioBps:  2500,
			expectMinScore: "100.000000000000000000",
			expectMaxScore: "100.000000000000000000",
			expectDecayed:  false,
		},
		{
			name:           "at interval boundary - no decay",
			score:          "100.000000000000000000",
			peak:           "100.000000000000000000",
			lastActive:     800,
			currentHeight:  1000,
			decayRateBps:   500,
			intervalBlocks: 200,
			floorRatioBps:  2500,
			expectMinScore: "100.000000000000000000",
			expectMaxScore: "100.000000000000000000",
			expectDecayed:  false,
		},
		{
			name:           "one interval past - 5% decay",
			score:          "100.000000000000000000",
			peak:           "100.000000000000000000",
			lastActive:     500,
			currentHeight:  1000,
			decayRateBps:   500,
			intervalBlocks: 200,
			floorRatioBps:  2500,
			// 500 blocks inactive / 200 interval = 2 periods
			// 100 * 0.95^2 = 90.25
			expectMinScore: "90.000000000000000000",
			expectMaxScore: "91.000000000000000000",
			expectDecayed:  true,
		},
		{
			name:           "single period decay",
			score:          "100.000000000000000000",
			peak:           "100.000000000000000000",
			lastActive:     700,
			currentHeight:  1000,
			decayRateBps:   500,
			intervalBlocks: 200,
			floorRatioBps:  2500,
			// 300 blocks / 200 = 1 period
			// 100 * 0.95 = 95
			expectMinScore: "95.000000000000000000",
			expectMaxScore: "95.000000000000000000",
			expectDecayed:  true,
		},
		{
			name:           "many periods - floor enforced",
			score:          "100.000000000000000000",
			peak:           "100.000000000000000000",
			lastActive:     0,
			currentHeight:  100000,
			decayRateBps:   500,
			intervalBlocks: 200,
			floorRatioBps:  2500,
			// 100000/200 = 500 periods → floor at 25
			expectMinScore: "25.000000000000000000",
			expectMaxScore: "25.000000000000000000",
			expectDecayed:  true,
		},
		{
			name:           "already at floor - no change",
			score:          "25.000000000000000000",
			peak:           "100.000000000000000000",
			lastActive:     0,
			currentHeight:  10000,
			decayRateBps:   500,
			intervalBlocks: 200,
			floorRatioBps:  2500,
			expectMinScore: "25.000000000000000000",
			expectMaxScore: "25.000000000000000000",
			expectDecayed:  false,
		},
		{
			name:           "below floor - no change",
			score:          "20.000000000000000000",
			peak:           "100.000000000000000000",
			lastActive:     0,
			currentHeight:  10000,
			decayRateBps:   500,
			intervalBlocks: 200,
			floorRatioBps:  2500,
			expectMinScore: "20.000000000000000000",
			expectMaxScore: "20.000000000000000000",
			expectDecayed:  false,
		},
		{
			name:           "zero score - no change",
			score:          "0.000000000000000000",
			peak:           "100.000000000000000000",
			lastActive:     0,
			currentHeight:  10000,
			decayRateBps:   500,
			intervalBlocks: 200,
			floorRatioBps:  2500,
			expectMinScore: "0.000000000000000000",
			expectMaxScore: "0.000000000000000000",
			expectDecayed:  false,
		},
		{
			name:           "10% decay rate - heavier decay",
			score:          "100.000000000000000000",
			peak:           "100.000000000000000000",
			lastActive:     600,
			currentHeight:  1000,
			decayRateBps:   1000,
			intervalBlocks: 200,
			floorRatioBps:  2500,
			// 400/200 = 2 periods, 100 * 0.9^2 = 81
			expectMinScore: "81.000000000000000000",
			expectMaxScore: "81.000000000000000000",
			expectDecayed:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			k, ctx := setupKeeper(t)

			// Set params
			params := types.ReputationDecayParams{
				DecayRateBps:        tc.decayRateBps,
				DecayIntervalBlocks: tc.intervalBlocks,
				FloorRatioBps:       tc.floorRatioBps,
			}
			require.NoError(t, k.SetReputationDecayParams(ctx, params))

			// Create record
			score, _ := sdkmath.LegacyNewDecFromStr(tc.score)
			peak, _ := sdkmath.LegacyNewDecFromStr(tc.peak)
			rep := types.AgentDomainReputation{
				AgentAddr:        "agent1",
				DomainID:         "tech",
				Score:            score.String(),
				PeakScore:        peak.String(),
				LastActiveHeight: tc.lastActive,
			}
			require.NoError(t, k.SetAgentDomainReputation(ctx, rep))

			// Apply decay
			k.ApplyReputationDecay(ctx, tc.currentHeight)

			// Verify
			got, found := k.GetAgentDomainReputation(ctx, "agent1", "tech")
			require.True(t, found)

			resultScore := got.GetScore()
			minScore, _ := sdkmath.LegacyNewDecFromStr(tc.expectMinScore)
			maxScore, _ := sdkmath.LegacyNewDecFromStr(tc.expectMaxScore)
			require.True(t, resultScore.GTE(minScore), "score %s < min %s", resultScore, minScore)
			require.True(t, resultScore.LTE(maxScore), "score %s > max %s", resultScore, maxScore)

			// Peak should never change from decay
			require.Equal(t, tc.peak, got.PeakScore, "peak should not change from decay")
		})
	}
}

// ─── Floor Enforcement ──────────────────────────────────────────────────────

func TestFloorEnforcement(t *testing.T) {
	tests := []struct {
		name          string
		peak          string
		floorBps      uint32
		expectFloor   string
	}{
		{"25% of 100", "100.000000000000000000", 2500, "25.000000000000000000"},
		{"25% of 200", "200.000000000000000000", 2500, "50.000000000000000000"},
		{"50% of 100", "100.000000000000000000", 5000, "50.000000000000000000"},
		{"0% floor", "100.000000000000000000", 0, "0.000000000000000000"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			k, ctx := setupKeeper(t)

			params := types.ReputationDecayParams{
				DecayRateBps:        5000, // 50% - aggressive to hit floor fast
				DecayIntervalBlocks: 100,
				FloorRatioBps:       tc.floorBps,
			}
			require.NoError(t, k.SetReputationDecayParams(ctx, params))

			peak, _ := sdkmath.LegacyNewDecFromStr(tc.peak)
			rep := types.AgentDomainReputation{
				AgentAddr:        "agent1",
				DomainID:         "tech",
				Score:            peak.String(), // start at peak
				PeakScore:        peak.String(),
				LastActiveHeight: 0,
			}
			require.NoError(t, k.SetAgentDomainReputation(ctx, rep))

			// Apply many decay periods to hit floor
			k.ApplyReputationDecay(ctx, 100000)

			got, _ := k.GetAgentDomainReputation(ctx, "agent1", "tech")
			expectFloor, _ := sdkmath.LegacyNewDecFromStr(tc.expectFloor)
			require.True(t, got.GetScore().GTE(expectFloor),
				"score %s should be >= floor %s", got.GetScore(), expectFloor)
		})
	}
}

// ─── Peak Tracking ──────────────────────────────────────────────────────────

func TestPeakTracking(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Build up reputation
	require.NoError(t, k.UpdateReputation(ctx, "agent1", "tech", sdkmath.LegacyNewDec(50), 100))
	got, _ := k.GetAgentDomainReputation(ctx, "agent1", "tech")
	require.Equal(t, sdkmath.LegacyNewDec(50), got.GetPeakScore())

	// Increase
	require.NoError(t, k.UpdateReputation(ctx, "agent1", "tech", sdkmath.LegacyNewDec(30), 200))
	got, _ = k.GetAgentDomainReputation(ctx, "agent1", "tech")
	require.Equal(t, sdkmath.LegacyNewDec(80), got.GetScore())
	require.Equal(t, sdkmath.LegacyNewDec(80), got.GetPeakScore())

	// Decrease - peak stays
	require.NoError(t, k.UpdateReputation(ctx, "agent1", "tech", sdkmath.LegacyNewDec(-20), 300))
	got, _ = k.GetAgentDomainReputation(ctx, "agent1", "tech")
	require.Equal(t, sdkmath.LegacyNewDec(60), got.GetScore())
	require.Equal(t, sdkmath.LegacyNewDec(80), got.GetPeakScore()) // peak unchanged

	// Exceed previous peak
	require.NoError(t, k.UpdateReputation(ctx, "agent1", "tech", sdkmath.LegacyNewDec(50), 400))
	got, _ = k.GetAgentDomainReputation(ctx, "agent1", "tech")
	require.Equal(t, sdkmath.LegacyNewDec(110), got.GetScore())
	require.Equal(t, sdkmath.LegacyNewDec(110), got.GetPeakScore()) // new peak
}

// ─── Multiple Domains Per Agent ─────────────────────────────────────────────

func TestMultipleDomainsPerAgent(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Agent has reputation in two domains
	require.NoError(t, k.UpdateReputation(ctx, "agent1", "tech", sdkmath.LegacyNewDec(100), 100))
	require.NoError(t, k.UpdateReputation(ctx, "agent1", "science", sdkmath.LegacyNewDec(80), 100))

	// Decay params: only tech should decay (agent active in science recently)
	params := types.ReputationDecayParams{
		DecayRateBps:        500,
		DecayIntervalBlocks: 100,
		FloorRatioBps:       2500,
	}
	require.NoError(t, k.SetReputationDecayParams(ctx, params))

	// Make science domain recently active, tech domain inactive
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ctx = sdkCtx.WithBlockHeight(500)
	k.ResetInactivityTimer(ctx, "agent1", "science", 450)

	// Apply decay at height 500 — tech inactive since 100 (400 blocks > 100 interval)
	k.ApplyReputationDecay(ctx, 500)

	techRep, _ := k.GetAgentDomainReputation(ctx, "agent1", "tech")
	sciRep, _ := k.GetAgentDomainReputation(ctx, "agent1", "science")

	// Tech should have decayed (100 * 0.95^4 ≈ 81.45)
	require.True(t, techRep.GetScore().LT(sdkmath.LegacyNewDec(100)), "tech should have decayed")

	// Science should NOT have decayed (450 last active, 500 current, 50 < 100 interval)
	require.Equal(t, sdkmath.LegacyNewDec(80), sciRep.GetScore(), "science should not decay")
}

// ─── Timer Reset Prevents Decay ─────────────────────────────────────────────

func TestTimerResetPreventsDecay(t *testing.T) {
	k, ctx := setupKeeper(t)

	params := types.ReputationDecayParams{
		DecayRateBps:        500,
		DecayIntervalBlocks: 100,
		FloorRatioBps:       2500,
	}
	require.NoError(t, k.SetReputationDecayParams(ctx, params))

	// Create reputation at height 0
	require.NoError(t, k.UpdateReputation(ctx, "agent1", "tech", sdkmath.LegacyNewDec(100), 0))

	// At height 150 (past interval), but reset timer at height 100
	k.ResetInactivityTimer(ctx, "agent1", "tech", 100)
	k.ApplyReputationDecay(ctx, 150) // 150 - 100 = 50 < 100 interval

	got, _ := k.GetAgentDomainReputation(ctx, "agent1", "tech")
	require.Equal(t, sdkmath.LegacyNewDec(100), got.GetScore(), "timer reset should prevent decay")
}

// ─── UpdateReputation Resets Timer ──────────────────────────────────────────

func TestUpdateReputationResetsTimer(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Create at height 100
	require.NoError(t, k.UpdateReputation(ctx, "agent1", "tech", sdkmath.LegacyNewDec(100), 100))

	// Update at height 500 — this should reset the timer
	require.NoError(t, k.UpdateReputation(ctx, "agent1", "tech", sdkmath.LegacyNewDec(10), 500))

	got, _ := k.GetAgentDomainReputation(ctx, "agent1", "tech")
	require.Equal(t, int64(500), got.LastActiveHeight)
}

// ─── Decay With Zero Interval Blocks ────────────────────────────────────────

func TestDecayWithZeroIntervalBlocks(t *testing.T) {
	k, ctx := setupKeeper(t)

	params := types.ReputationDecayParams{
		DecayRateBps:        500,
		DecayIntervalBlocks: 0, // edge case — should not panic
		FloorRatioBps:       2500,
	}
	// Note: Validate() would catch this, but the keeper should handle gracefully
	require.NoError(t, k.SetReputationDecayParams(ctx, params))
	require.NoError(t, k.UpdateReputation(ctx, "agent1", "tech", sdkmath.LegacyNewDec(100), 0))

	// Should not panic or loop
	k.ApplyReputationDecay(ctx, 1000)

	got, _ := k.GetAgentDomainReputation(ctx, "agent1", "tech")
	require.Equal(t, sdkmath.LegacyNewDec(100), got.GetScore(), "zero interval should skip decay")
}
