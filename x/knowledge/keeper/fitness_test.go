package keeper_test

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Lifecycle Status Derivation ────────────────────────────────────────────

func TestLifecycleStatusFromScore(t *testing.T) {
	tests := []struct {
		name   string
		score  string // LegacyDec as string
		expect types.TDULifecycleStatus
	}{
		{"core at 1.0", "1.000000000000000000", types.TDULifecycleCore},
		{"core at 0.7", "0.700000000000000000", types.TDULifecycleCore},
		{"core at 0.85", "0.850000000000000000", types.TDULifecycleCore},
		{"active at 0.69", "0.690000000000000000", types.TDULifecycleActive},
		{"active at 0.5 (initial)", "0.500000000000000000", types.TDULifecycleActive},
		{"active at 0.3", "0.300000000000000000", types.TDULifecycleActive},
		{"dormant at 0.29", "0.290000000000000000", types.TDULifecycleDormant},
		{"dormant at 0.1", "0.100000000000000000", types.TDULifecycleDormant},
		{"pruned at 0.09", "0.090000000000000000", types.TDULifecyclePruned},
		{"pruned at 0.0", "0.000000000000000000", types.TDULifecyclePruned},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			score, err := sdkmath.LegacyNewDecFromStr(tc.score)
			require.NoError(t, err)
			got := types.LifecycleStatusFromScore(score)
			require.Equal(t, tc.expect, got, "score=%s", tc.score)
		})
	}
}

func TestLifecycleStatusString(t *testing.T) {
	tests := []struct {
		status types.TDULifecycleStatus
		expect string
	}{
		{types.TDULifecycleCore, "core"},
		{types.TDULifecycleActive, "active"},
		{types.TDULifecycleDormant, "dormant"},
		{types.TDULifecyclePruned, "pruned"},
		{types.TDULifecycleStatus(99), "unknown"},
	}

	for _, tc := range tests {
		t.Run(tc.expect, func(t *testing.T) {
			require.Equal(t, tc.expect, tc.status.String())
		})
	}
}

// ─── Fitness Record CRUD ────────────────────────────────────────────────────

func TestFitnessRecordCRUD(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Not found initially
	_, found := k.GetFitnessRecord(ctx, "sample1")
	require.False(t, found)

	// Create
	record := types.NewTDUFitnessRecord("sample1", sdkmath.NewInt(1_000_000), 10)
	require.NoError(t, k.SetFitnessRecord(ctx, record))

	// Read
	got, found := k.GetFitnessRecord(ctx, "sample1")
	require.True(t, found)
	require.Equal(t, "sample1", got.SampleID)
	require.Equal(t, types.FitnessInitialScore.String(), got.FitnessScore)
	require.Equal(t, "1000000", got.OriginalStake)
	require.Equal(t, uint64(10), got.LastSignalCycle)

	// Update
	got.SetFitnessScore(sdkmath.LegacyNewDecWithPrec(8, 1)) // 0.8
	require.NoError(t, k.SetFitnessRecord(ctx, got))
	updated, found := k.GetFitnessRecord(ctx, "sample1")
	require.True(t, found)
	require.Equal(t, "0.800000000000000000", updated.FitnessScore)

	// Delete
	require.NoError(t, k.DeleteFitnessRecord(ctx, "sample1"))
	_, found = k.GetFitnessRecord(ctx, "sample1")
	require.False(t, found)
}

func TestFitnessRecordIteration(t *testing.T) {
	k, ctx := setupKeeper(t)

	ids := []string{"aaa", "bbb", "ccc"}
	for _, id := range ids {
		require.NoError(t, k.SetFitnessRecord(ctx, types.NewTDUFitnessRecord(id, sdkmath.NewInt(100), 1)))
	}

	var collected []string
	k.IterateFitnessRecords(ctx, func(r types.TDUFitnessRecord) bool {
		collected = append(collected, r.SampleID)
		return false
	})
	require.Len(t, collected, 3)
}

// ─── Initialize Fitness ─────────────────────────────────────────────────────

func TestInitializeFitnessRecord(t *testing.T) {
	k, ctx := setupKeeper(t)

	require.NoError(t, k.InitializeFitnessRecord(ctx, "s1", sdkmath.NewInt(5_000_000), 42))
	r, found := k.GetFitnessRecord(ctx, "s1")
	require.True(t, found)
	require.Equal(t, "0.500000000000000000", r.FitnessScore)
	require.Equal(t, "5000000", r.OriginalStake)
	require.Equal(t, uint64(42), r.LastSignalCycle)
	require.Equal(t, uint64(0), r.CycleCount)
	require.Equal(t, types.TDULifecycleActive, r.GetLifecycleStatus())
}

// ─── Fitness Score Clamping ─────────────────────────────────────────────────

func TestFitnessScoreClamping(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{"normal value", "0.500000000000000000", "0.500000000000000000"},
		{"clamped above 1.0", "1.500000000000000000", "1.000000000000000000"},
		{"clamped below 0.0", "-0.100000000000000000", "0.000000000000000000"},
		{"exact zero", "0.000000000000000000", "0.000000000000000000"},
		{"exact one", "1.000000000000000000", "1.000000000000000000"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := types.TDUFitnessRecord{}
			score, err := sdkmath.LegacyNewDecFromStr(tc.input)
			require.NoError(t, err)
			r.SetFitnessScore(score)
			require.Equal(t, tc.expect, r.FitnessScore)
		})
	}
}

// ─── Update Fitness Score (Signal Aggregation) ──────────────────────────────

func TestUpdateFitnessScore(t *testing.T) {
	tests := []struct {
		name           string
		initialScore   string
		signal         types.FitnessSignal
		expectMinScore string // score should be >= this
		expectMaxScore string // score should be <= this
		expectStatus   types.TDULifecycleStatus
	}{
		{
			name:         "strong signals push toward core",
			initialScore: "0.500000000000000000",
			signal: types.FitnessSignal{
				TrainingInfluence: sdkmath.LegacyOneDec(),
				UsageCorrelation:  sdkmath.LegacyOneDec(),
				Redundancy:        sdkmath.LegacyOneDec(),
			},
			// weighted = 1.0*0.5 + 1.0*0.3 + 1.0*0.2 = 1.0
			// blended = 0.5*0.5 + 0.5*1.0 = 0.75
			expectMinScore: "0.740000000000000000",
			expectMaxScore: "0.760000000000000000",
			expectStatus:   types.TDULifecycleCore,
		},
		{
			name:         "weak signals push toward dormant",
			initialScore: "0.500000000000000000",
			signal: types.FitnessSignal{
				TrainingInfluence: sdkmath.LegacyZeroDec(),
				UsageCorrelation:  sdkmath.LegacyZeroDec(),
				Redundancy:        sdkmath.LegacyZeroDec(),
			},
			// weighted = 0, blended = 0.5*0.5 + 0.5*0 = 0.25
			expectMinScore: "0.240000000000000000",
			expectMaxScore: "0.260000000000000000",
			expectStatus:   types.TDULifecycleDormant,
		},
		{
			name:         "mixed signals stay active",
			initialScore: "0.500000000000000000",
			signal: types.FitnessSignal{
				TrainingInfluence: sdkmath.LegacyNewDecWithPrec(6, 1), // 0.6
				UsageCorrelation:  sdkmath.LegacyNewDecWithPrec(4, 1), // 0.4
				Redundancy:        sdkmath.LegacyNewDecWithPrec(5, 1), // 0.5
			},
			// weighted = 0.6*0.5 + 0.4*0.3 + 0.5*0.2 = 0.30 + 0.12 + 0.10 = 0.52
			// blended = 0.5*0.5 + 0.5*0.52 = 0.51
			expectMinScore: "0.500000000000000000",
			expectMaxScore: "0.520000000000000000",
			expectStatus:   types.TDULifecycleActive,
		},
		{
			name:         "already core stays core with strong signals",
			initialScore: "0.900000000000000000",
			signal: types.FitnessSignal{
				TrainingInfluence: sdkmath.LegacyNewDecWithPrec(9, 1), // 0.9
				UsageCorrelation:  sdkmath.LegacyNewDecWithPrec(8, 1), // 0.8
				Redundancy:        sdkmath.LegacyNewDecWithPrec(7, 1), // 0.7
			},
			// weighted = 0.9*0.5 + 0.8*0.3 + 0.7*0.2 = 0.45 + 0.24 + 0.14 = 0.83
			// blended = 0.5*0.9 + 0.5*0.83 = 0.865
			expectMinScore: "0.860000000000000000",
			expectMaxScore: "0.870000000000000000",
			expectStatus:   types.TDULifecycleCore,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			k, ctx := setupKeeper(t)

			// Create record with initial score
			record := types.NewTDUFitnessRecord("s1", sdkmath.NewInt(1_000_000), 1)
			score, err := sdkmath.LegacyNewDecFromStr(tc.initialScore)
			require.NoError(t, err)
			record.SetFitnessScore(score)
			require.NoError(t, k.SetFitnessRecord(ctx, record))

			// Update with signal
			require.NoError(t, k.UpdateFitnessScore(ctx, "s1", tc.signal, 5))

			// Verify
			updated, found := k.GetFitnessRecord(ctx, "s1")
			require.True(t, found)

			resultScore := updated.GetFitnessScore()
			minScore, _ := sdkmath.LegacyNewDecFromStr(tc.expectMinScore)
			maxScore, _ := sdkmath.LegacyNewDecFromStr(tc.expectMaxScore)
			require.True(t, resultScore.GTE(minScore), "score %s < min %s", resultScore, minScore)
			require.True(t, resultScore.LTE(maxScore), "score %s > max %s", resultScore, maxScore)
			require.Equal(t, tc.expectStatus, updated.GetLifecycleStatus())
			require.Equal(t, uint64(5), updated.LastSignalCycle)
			require.Equal(t, uint64(1), updated.CycleCount)
		})
	}
}

func TestUpdateFitnessScore_InvalidSignal(t *testing.T) {
	tests := []struct {
		name   string
		signal types.FitnessSignal
	}{
		{
			name: "negative training influence",
			signal: types.FitnessSignal{
				TrainingInfluence: sdkmath.LegacyNewDec(-1),
				UsageCorrelation:  sdkmath.LegacyZeroDec(),
				Redundancy:        sdkmath.LegacyZeroDec(),
			},
		},
		{
			name: "usage above 1.0",
			signal: types.FitnessSignal{
				TrainingInfluence: sdkmath.LegacyZeroDec(),
				UsageCorrelation:  sdkmath.LegacyNewDecWithPrec(15, 1), // 1.5
				Redundancy:        sdkmath.LegacyZeroDec(),
			},
		},
		{
			name: "redundancy negative",
			signal: types.FitnessSignal{
				TrainingInfluence: sdkmath.LegacyZeroDec(),
				UsageCorrelation:  sdkmath.LegacyZeroDec(),
				Redundancy:        sdkmath.LegacyNewDec(-1),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			k, ctx := setupKeeper(t)
			require.NoError(t, k.InitializeFitnessRecord(ctx, "s1", sdkmath.NewInt(1_000_000), 1))
			err := k.UpdateFitnessScore(ctx, "s1", tc.signal, 2)
			require.ErrorIs(t, err, types.ErrInvalidFitnessSignal)
		})
	}
}

func TestUpdateFitnessScore_RecordNotFound(t *testing.T) {
	k, ctx := setupKeeper(t)
	err := k.UpdateFitnessScore(ctx, "nonexistent", types.FitnessSignal{
		TrainingInfluence: sdkmath.LegacyZeroDec(),
		UsageCorrelation:  sdkmath.LegacyZeroDec(),
		Redundancy:        sdkmath.LegacyZeroDec(),
	}, 1)
	require.ErrorIs(t, err, types.ErrFitnessRecordNotFound)
}

// ─── Decay Unscored ─────────────────────────────────────────────────────────

func TestDecayUnscored(t *testing.T) {
	tests := []struct {
		name             string
		lastSignalCycle  uint64
		currentCycle     uint64
		threshold        uint64
		initialScore     string
		expectedDecayed  bool
		expectedMinScore string
		expectedMaxScore string
	}{
		{
			name:             "within threshold - no decay",
			lastSignalCycle:  10,
			currentCycle:     14,
			threshold:        5,
			initialScore:     "0.500000000000000000",
			expectedDecayed:  false,
			expectedMinScore: "0.500000000000000000",
			expectedMaxScore: "0.500000000000000000",
		},
		{
			name:             "at threshold boundary - no decay",
			lastSignalCycle:  10,
			currentCycle:     15,
			threshold:        5,
			initialScore:     "0.500000000000000000",
			expectedDecayed:  false,
			expectedMinScore: "0.500000000000000000",
			expectedMaxScore: "0.500000000000000000",
		},
		{
			name:             "past threshold - decays",
			lastSignalCycle:  10,
			currentCycle:     16,
			threshold:        5,
			initialScore:     "0.500000000000000000",
			expectedDecayed:  true,
			expectedMinScore: "0.470000000000000000",
			expectedMaxScore: "0.490000000000000000",
		},
		{
			name:             "already zero - no change",
			lastSignalCycle:  1,
			currentCycle:     100,
			threshold:        5,
			initialScore:     "0.000000000000000000",
			expectedDecayed:  false,
			expectedMinScore: "0.000000000000000000",
			expectedMaxScore: "0.000000000000000000",
		},
		{
			name:             "near zero - clamps to zero",
			lastSignalCycle:  1,
			currentCycle:     100,
			threshold:        5,
			initialScore:     "0.010000000000000000",
			expectedDecayed:  true,
			expectedMinScore: "0.000000000000000000",
			expectedMaxScore: "0.000000000000000000",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			k, ctx := setupKeeper(t)

			// Set params
			params := types.DefaultFitnessDecayParams()
			params.UnscoredCycleThreshold = tc.threshold
			require.NoError(t, k.SetFitnessDecayParams(ctx, params))

			// Create record
			record := types.NewTDUFitnessRecord("s1", sdkmath.NewInt(1_000_000), tc.lastSignalCycle)
			score, _ := sdkmath.LegacyNewDecFromStr(tc.initialScore)
			record.SetFitnessScore(score)
			require.NoError(t, k.SetFitnessRecord(ctx, record))

			// Run decay
			k.DecayUnscored(ctx, tc.currentCycle)

			// Verify
			updated, found := k.GetFitnessRecord(ctx, "s1")
			require.True(t, found)

			resultScore := updated.GetFitnessScore()
			minScore, _ := sdkmath.LegacyNewDecFromStr(tc.expectedMinScore)
			maxScore, _ := sdkmath.LegacyNewDecFromStr(tc.expectedMaxScore)
			require.True(t, resultScore.GTE(minScore), "score %s < min %s", resultScore, minScore)
			require.True(t, resultScore.LTE(maxScore), "score %s > max %s", resultScore, maxScore)
		})
	}
}

func TestDecayUnscored_MultipleRecords(t *testing.T) {
	k, ctx := setupKeeper(t)

	params := types.DefaultFitnessDecayParams()
	params.UnscoredCycleThreshold = 3
	require.NoError(t, k.SetFitnessDecayParams(ctx, params))

	// s1: should decay (last signal cycle 1, current 10, threshold 3)
	r1 := types.NewTDUFitnessRecord("s1", sdkmath.NewInt(100), 1)
	require.NoError(t, k.SetFitnessRecord(ctx, r1))

	// s2: should NOT decay (last signal cycle 8, current 10, threshold 3)
	r2 := types.NewTDUFitnessRecord("s2", sdkmath.NewInt(100), 8)
	require.NoError(t, k.SetFitnessRecord(ctx, r2))

	k.DecayUnscored(ctx, 10)

	u1, _ := k.GetFitnessRecord(ctx, "s1")
	u2, _ := k.GetFitnessRecord(ctx, "s2")

	// s1 should have decayed from 0.5
	require.True(t, u1.GetFitnessScore().LT(types.FitnessInitialScore))

	// s2 should remain at 0.5
	require.True(t, u2.GetFitnessScore().Equal(types.FitnessInitialScore))
}

// ─── Longevity Rewards ──────────────────────────────────────────────────────

func TestComputeLongevityReward(t *testing.T) {
	tests := []struct {
		name          string
		fitnessScore  string
		originalStake int64
		expectReward  int64
	}{
		{
			name:          "core TDU earns 0.01x stake",
			fitnessScore:  "0.800000000000000000",
			originalStake: 1_000_000,
			expectReward:  10_000, // 1M * 0.01
		},
		{
			name:          "active TDU earns 0.005x stake",
			fitnessScore:  "0.500000000000000000",
			originalStake: 1_000_000,
			expectReward:  5_000, // 1M * 0.005
		},
		{
			name:          "dormant TDU earns nothing",
			fitnessScore:  "0.200000000000000000",
			originalStake: 1_000_000,
			expectReward:  0,
		},
		{
			name:          "pruned TDU earns nothing",
			fitnessScore:  "0.050000000000000000",
			originalStake: 1_000_000,
			expectReward:  0,
		},
		{
			name:          "core with small stake",
			fitnessScore:  "0.900000000000000000",
			originalStake: 100,
			expectReward:  1, // 100 * 0.01 = 1
		},
		{
			name:          "zero stake earns nothing",
			fitnessScore:  "0.900000000000000000",
			originalStake: 0,
			expectReward:  0,
		},
		{
			name:          "exactly at core boundary",
			fitnessScore:  "0.700000000000000000",
			originalStake: 1_000_000,
			expectReward:  10_000,
		},
		{
			name:          "exactly at active boundary",
			fitnessScore:  "0.300000000000000000",
			originalStake: 1_000_000,
			expectReward:  5_000,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			k, ctx := setupKeeper(t)

			record := types.NewTDUFitnessRecord("s1", sdkmath.NewInt(tc.originalStake), 1)
			score, _ := sdkmath.LegacyNewDecFromStr(tc.fitnessScore)
			record.SetFitnessScore(score)
			require.NoError(t, k.SetFitnessRecord(ctx, record))

			reward := k.ComputeLongevityReward(ctx, "s1")
			require.Equal(t, sdkmath.NewInt(tc.expectReward), reward, "reward mismatch")
		})
	}
}

func TestComputeLongevityReward_RecordNotFound(t *testing.T) {
	k, ctx := setupKeeper(t)
	reward := k.ComputeLongevityReward(ctx, "nonexistent")
	require.True(t, reward.IsZero())
}

// ─── Fitness Decay Params CRUD ──────────────────────────────────────────────

func TestFitnessDecayParamsCRUD(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Default when unset
	got := k.GetFitnessDecayParams(ctx)
	require.Equal(t, types.DefaultFitnessDecayParams(), got)

	// Set custom
	custom := types.FitnessDecayParams{
		DecayPerCycle:          "0.050000000000000000",
		UnscoredCycleThreshold: 10,
		CoreRewardRate:         "0.020000000000000000",
		ActiveRewardRate:       "0.010000000000000000",
	}
	require.NoError(t, k.SetFitnessDecayParams(ctx, custom))

	got = k.GetFitnessDecayParams(ctx)
	require.Equal(t, custom, got)
}

func TestFitnessDecayParamsValidation(t *testing.T) {
	tests := []struct {
		name    string
		params  types.FitnessDecayParams
		wantErr bool
	}{
		{
			name:    "valid defaults",
			params:  types.DefaultFitnessDecayParams(),
			wantErr: false,
		},
		{
			name: "negative decay",
			params: types.FitnessDecayParams{
				DecayPerCycle:          "-0.010000000000000000",
				UnscoredCycleThreshold: 5,
				CoreRewardRate:         "0.010000000000000000",
				ActiveRewardRate:       "0.005000000000000000",
			},
			wantErr: true,
		},
		{
			name: "decay above 1.0",
			params: types.FitnessDecayParams{
				DecayPerCycle:          "1.500000000000000000",
				UnscoredCycleThreshold: 5,
				CoreRewardRate:         "0.010000000000000000",
				ActiveRewardRate:       "0.005000000000000000",
			},
			wantErr: true,
		},
		{
			name: "negative core reward",
			params: types.FitnessDecayParams{
				DecayPerCycle:          "0.020000000000000000",
				UnscoredCycleThreshold: 5,
				CoreRewardRate:         "-0.010000000000000000",
				ActiveRewardRate:       "0.005000000000000000",
			},
			wantErr: true,
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

// ─── GetTDULifecycleStatus ──────────────────────────────────────────────────

func TestGetTDULifecycleStatus(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Not found
	_, err := k.GetTDULifecycleStatus(ctx, "nonexistent")
	require.ErrorIs(t, err, types.ErrFitnessRecordNotFound)

	// Create record at initial score
	require.NoError(t, k.InitializeFitnessRecord(ctx, "s1", sdkmath.NewInt(100), 1))
	status, err := k.GetTDULifecycleStatus(ctx, "s1")
	require.NoError(t, err)
	require.Equal(t, types.TDULifecycleActive, status)
}

// ─── DistributeLongevityRewards ─────────────────────────────────────────────

func TestDistributeLongevityRewards(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)

	// Create a sample that the fitness record references
	require.NoError(t, k.SetSample(ctx, &types.Sample{
		Id:        "s1",
		Submitter: rv1,
		Status:    types.SampleStatus_SAMPLE_STATUS_GOLD,
	}))

	// Create a fitness record at Core level
	record := types.NewTDUFitnessRecord("s1", sdkmath.NewInt(1_000_000), 1)
	record.SetFitnessScore(sdkmath.LegacyNewDecWithPrec(8, 1)) // 0.8 = Core
	require.NoError(t, k.SetFitnessRecord(ctx, record))

	// Distribute
	k.DistributeLongevityRewards(ctx)

	// Should have sent 10,000 uzrn (0.01 × 1,000,000) to rv1
	require.Len(t, bk.moduleToAccountCalls, 1)
	transfer := bk.moduleToAccountCalls[0]
	require.Equal(t, types.ModuleName, transfer.from)
	require.Equal(t, sdkmath.NewInt(10_000), transfer.amount.AmountOf("uzrn"))
}

func TestDistributeLongevityRewards_DormantGetsNothing(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)

	require.NoError(t, k.SetSample(ctx, &types.Sample{
		Id:        "s1",
		Submitter: rv1,
		Status:    types.SampleStatus_SAMPLE_STATUS_BRONZE,
	}))

	record := types.NewTDUFitnessRecord("s1", sdkmath.NewInt(1_000_000), 1)
	record.SetFitnessScore(sdkmath.LegacyNewDecWithPrec(2, 1)) // 0.2 = Dormant
	require.NoError(t, k.SetFitnessRecord(ctx, record))

	k.DistributeLongevityRewards(ctx)

	require.Len(t, bk.moduleToAccountCalls, 0, "dormant TDU should not receive rewards")
}

// ─── Lifecycle Transitions ──────────────────────────────────────────────────

func TestLifecycleTransitionViaDecay(t *testing.T) {
	k, ctx := setupKeeper(t)

	params := types.FitnessDecayParams{
		DecayPerCycle:          "0.100000000000000000", // aggressive decay for testing
		UnscoredCycleThreshold: 0,                      // decay immediately
		CoreRewardRate:         "0.010000000000000000",
		ActiveRewardRate:       "0.005000000000000000",
	}
	require.NoError(t, k.SetFitnessDecayParams(ctx, params))

	// Start at Active (0.5)
	require.NoError(t, k.InitializeFitnessRecord(ctx, "s1", sdkmath.NewInt(100), 0))

	statusAt := func(cycle uint64) types.TDULifecycleStatus {
		k.DecayUnscored(ctx, cycle)
		s, _ := k.GetTDULifecycleStatus(ctx, "s1")
		return s
	}

	// Cycle 1: 0.5 - 0.1 = 0.4 → Active
	require.Equal(t, types.TDULifecycleActive, statusAt(1))

	// Cycle 2: 0.4 - 0.1 = 0.3 → Active (boundary)
	require.Equal(t, types.TDULifecycleActive, statusAt(2))

	// Cycle 3: 0.3 - 0.1 = 0.2 → Dormant
	require.Equal(t, types.TDULifecycleDormant, statusAt(3))

	// Cycle 4: 0.2 - 0.1 = 0.1 → Dormant (boundary)
	require.Equal(t, types.TDULifecycleDormant, statusAt(4))

	// Cycle 5: 0.1 - 0.1 = 0.0 → Pruned
	require.Equal(t, types.TDULifecyclePruned, statusAt(5))
}

func TestLifecycleRecoveryViaSignal(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Start dormant
	record := types.NewTDUFitnessRecord("s1", sdkmath.NewInt(100), 0)
	record.SetFitnessScore(sdkmath.LegacyNewDecWithPrec(15, 2)) // 0.15 = Dormant
	require.NoError(t, k.SetFitnessRecord(ctx, record))

	status, _ := k.GetTDULifecycleStatus(ctx, "s1")
	require.Equal(t, types.TDULifecycleDormant, status)

	// Strong signal should boost fitness
	signal := types.FitnessSignal{
		TrainingInfluence: sdkmath.LegacyOneDec(),
		UsageCorrelation:  sdkmath.LegacyOneDec(),
		Redundancy:        sdkmath.LegacyOneDec(),
	}
	// weighted = 1.0, blended = 0.5*0.15 + 0.5*1.0 = 0.575 → Active
	require.NoError(t, k.UpdateFitnessScore(ctx, "s1", signal, 5))

	status, _ = k.GetTDULifecycleStatus(ctx, "s1")
	require.Equal(t, types.TDULifecycleActive, status)
}

// ─── TDUFitnessRecord Getter Edge Cases ─────────────────────────────────────

func TestTDUFitnessRecord_EmptyFields(t *testing.T) {
	r := types.TDUFitnessRecord{}

	// Empty fitness score returns initial
	require.True(t, r.GetFitnessScore().Equal(types.FitnessInitialScore))

	// Empty original stake returns zero
	require.True(t, r.GetOriginalStake().IsZero())
}

// ─── Export setupKeeper for use by fitness tests ────────────────────────────

// Verify the test uses the correct sdk import for UnwrapSDKContext
func TestFitnessWithSDKContext(t *testing.T) {
	k, baseCtx := setupKeeper(t)
	sdkCtx := sdk.UnwrapSDKContext(baseCtx)
	ctx := sdkCtx.WithBlockHeight(500)

	require.NoError(t, k.InitializeFitnessRecord(ctx, "s1", sdkmath.NewInt(100), 5))
	r, found := k.GetFitnessRecord(ctx, "s1")
	require.True(t, found)
	require.Equal(t, uint64(5), r.LastSignalCycle)
}
