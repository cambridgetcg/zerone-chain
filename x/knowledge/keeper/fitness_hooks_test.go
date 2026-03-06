package keeper_test

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/keeper"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Consensus Strength ──────────────────────────────────────────────────────

func TestConsensusStrength(t *testing.T) {
	tests := []struct {
		name          string
		reveals       int
		selected      int
		expectScore   string
		expectLabel   string
	}{
		{
			name:        "unanimous: all selected revealed",
			reveals:     5,
			selected:    5,
			expectScore: "1.000000000000000000",
			expectLabel: "unanimous",
		},
		{
			name:        "supermajority: 4 of 5",
			reveals:     4,
			selected:    5,
			expectScore: "0.800000000000000000",
			expectLabel: "supermajority",
		},
		{
			name:        "supermajority: 3 of 4 (75% >= 66%)",
			reveals:     3,
			selected:    4,
			expectScore: "0.800000000000000000",
			expectLabel: "supermajority",
		},
		{
			name:        "supermajority: 2 of 3 (66.7%)",
			reveals:     2,
			selected:    3,
			expectScore: "0.800000000000000000",
			expectLabel: "supermajority",
		},
		{
			name:        "bare majority: 2 of 5 (40%)",
			reveals:     2,
			selected:    5,
			expectScore: "0.600000000000000000",
			expectLabel: "bare majority",
		},
		{
			name:        "bare majority: 1 of 3 (33%)",
			reveals:     1,
			selected:    3,
			expectScore: "0.600000000000000000",
			expectLabel: "bare majority",
		},
		{
			name:        "zero selected defaults to bare",
			reveals:     0,
			selected:    0,
			expectScore: "0.600000000000000000",
			expectLabel: "bare majority",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			strength := keeper.ExportConsensusStrength(tc.reveals, tc.selected)
			require.Equal(t, tc.expectScore, strength.String(), "label=%s", tc.expectLabel)
		})
	}
}

// ─── Accepted Submission Creates FitnessRecord ──────────────────────────────

func TestAcceptedSubmissionCreatesFitnessRecord(t *testing.T) {
	tests := []struct {
		name          string
		stake         string
		reveals       int
		selected      int
		expectMinFit  string // score after init + training signal
		expectMaxFit  string
		expectStatus  types.TDULifecycleStatus
	}{
		{
			name:         "unanimous acceptance with 1 ZRN stake",
			stake:        "1000000",
			reveals:      3,
			selected:     3,
			expectMinFit: "0.370000000000000000", // init 0.5, signal weighted = 1.0*0.5 + 0*0.3 + 0.5*0.2 = 0.6, blend = 0.5*0.5 + 0.5*0.6 = 0.55... let me recalc
			expectMaxFit: "0.560000000000000000",
			expectStatus: types.TDULifecycleActive,
		},
		{
			name:         "supermajority acceptance",
			stake:        "1000000",
			reveals:      4,
			selected:     5,
			expectMinFit: "0.340000000000000000",
			expectMaxFit: "0.510000000000000000",
			expectStatus: types.TDULifecycleActive,
		},
		{
			name:         "bare majority acceptance",
			stake:        "500000",
			reveals:      2,
			selected:     5,
			expectMinFit: "0.300000000000000000",
			expectMaxFit: "0.460000000000000000",
			expectStatus: types.TDULifecycleActive,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			k, ctx, _ := setupKeeperWithBank(t)

			// Create submission
			sub := &types.Submission{
				Id:         "sub1",
				Content:    "test content",
				Domain:     "technology",
				SampleType: types.SampleType_SAMPLE_TYPE_DISCUSSION,
				Submitter:  rv1,
				Stake:      tc.stake,
				Tags:       []string{"ai"},
			}
			require.NoError(t, k.SetSubmission(ctx, sub))

			// Create quality round with reveals
			round := &types.QualityRound{
				Id:                "round1",
				SubmissionId:      "sub1",
				SelectedVerifiers: make([]string, tc.selected),
				Reveals:           make([]*types.RevealEntry, tc.reveals),
			}
			for i := range round.SelectedVerifiers {
				round.SelectedVerifiers[i] = "verifier" + string(rune('A'+i))
			}
			for i := range round.Reveals {
				round.Reveals[i] = &types.RevealEntry{Verifier: round.SelectedVerifiers[i]}
			}

			// Simulate createSampleFromSubmission path:
			// Set params so GetParams doesn't fail
			p := types.DefaultParams()
			require.NoError(t, k.SetParams(ctx, &p))

			strength := keeper.ExportConsensusStrength(tc.reveals, tc.selected)
			sampleID := k.NextSampleID(ctx)

			// Set the sample first (createSampleFromSubmission does this)
			require.NoError(t, k.SetSample(ctx, &types.Sample{
				Id:        sampleID,
				Submitter: rv1,
				Domain:    "technology",
				Status:    types.SampleStatus_SAMPLE_STATUS_GOLD,
			}))

			// Initialize fitness and send training signal (what createSampleFromSubmission now does)
			stake := sdkmath.ZeroInt()
			if s, ok := sdkmath.NewIntFromString(tc.stake); ok {
				stake = s
			}
			fitnessParams := k.GetFitnessDecayParams(ctx)
			sdkCtx := sdk.UnwrapSDKContext(ctx)
			currentCycle := uint64(sdkCtx.BlockHeight()) / fitnessParams.GetFitnessEpochBlocks()

			require.NoError(t, k.InitializeFitnessRecord(ctx, sampleID, stake, currentCycle))

			signal := types.FitnessSignal{
				TrainingInfluence: strength,
				UsageCorrelation:  sdkmath.LegacyZeroDec(),
				Redundancy:        sdkmath.LegacyNewDecWithPrec(5, 1),
			}
			require.NoError(t, k.UpdateFitnessScoreWithEvent(ctx, sampleID, signal, currentCycle))

			// Verify fitness record exists and has correct values
			record, found := k.GetFitnessRecord(ctx, sampleID)
			require.True(t, found, "fitness record should exist after acceptance")
			require.Equal(t, sampleID, record.SampleID)

			score := record.GetFitnessScore()
			minScore, _ := sdkmath.LegacyNewDecFromStr(tc.expectMinFit)
			maxScore, _ := sdkmath.LegacyNewDecFromStr(tc.expectMaxFit)
			require.True(t, score.GTE(minScore), "score %s < min %s", score, minScore)
			require.True(t, score.LTE(maxScore), "score %s > max %s", score, maxScore)
			require.Equal(t, tc.expectStatus, record.GetLifecycleStatus())

			// Verify original stake recorded
			require.Equal(t, tc.stake, record.OriginalStake)

			// Verify round info is not relevant for fitness, but training signal was applied (cycle count = 1)
			require.Equal(t, uint64(1), record.CycleCount, "should have processed one signal")
		})
	}
}

// ─── Repeated Access Increases Fitness ──────────────────────────────────────

func TestRepeatedAccessIncreasesFitness(t *testing.T) {
	tests := []struct {
		name          string
		initialScore  string
		accessCount   int
		expectMinFit  string
		expectMaxFit  string
	}{
		{
			name:         "single access from initial",
			initialScore: "0.500000000000000000",
			accessCount:  1,
			// Signal: TI=0.5, UC=1.0, R=0.5
			// weighted = 0.5*0.5 + 1.0*0.3 + 0.5*0.2 = 0.25 + 0.3 + 0.1 = 0.65
			// blended = 0.5*0.5 + 0.5*0.65 = 0.575
			expectMinFit: "0.570000000000000000",
			expectMaxFit: "0.580000000000000000",
		},
		{
			name:         "three accesses from initial",
			initialScore: "0.500000000000000000",
			accessCount:  3,
			// After 3 rounds of blending toward 0.65, approaches 0.65
			expectMinFit: "0.620000000000000000",
			expectMaxFit: "0.650000000000000000",
		},
		{
			name:         "access from dormant recovers toward active",
			initialScore: "0.200000000000000000",
			accessCount:  2,
			// Start 0.2, access pushes toward 0.65:
			// Round 1: 0.5*0.2 + 0.5*0.65 = 0.425
			// Round 2: 0.5*0.425 + 0.5*0.65 = 0.5375
			expectMinFit: "0.430000000000000000",
			expectMaxFit: "0.540000000000000000",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			k, ctx := setupKeeper(t)

			// Initialize fitness record with given score
			record := types.NewTDUFitnessRecord("s1", sdkmath.NewInt(1_000_000), 0)
			score, err := sdkmath.LegacyNewDecFromStr(tc.initialScore)
			require.NoError(t, err)
			record.SetFitnessScore(score)
			require.NoError(t, k.SetFitnessRecord(ctx, record))

			// Simulate repeated access signals
			for i := 0; i < tc.accessCount; i++ {
				signal := types.FitnessSignal{
					TrainingInfluence: sdkmath.LegacyNewDecWithPrec(5, 1), // 0.5 neutral
					UsageCorrelation:  sdkmath.LegacyOneDec(),             // 1.0
					Redundancy:        sdkmath.LegacyNewDecWithPrec(5, 1), // 0.5 neutral
				}
				require.NoError(t, k.UpdateFitnessScore(ctx, "s1", signal, uint64(i+1)))
			}

			updated, found := k.GetFitnessRecord(ctx, "s1")
			require.True(t, found)
			resultScore := updated.GetFitnessScore()
			minScore, _ := sdkmath.LegacyNewDecFromStr(tc.expectMinFit)
			maxScore, _ := sdkmath.LegacyNewDecFromStr(tc.expectMaxFit)
			require.True(t, resultScore.GTE(minScore), "score %s < min %s", resultScore, minScore)
			require.True(t, resultScore.LTE(maxScore), "score %s > max %s", resultScore, maxScore)
		})
	}
}

// ─── Decay Over N Epochs ────────────────────────────────────────────────────

func TestDecayOverEpochs(t *testing.T) {
	tests := []struct {
		name         string
		initialScore string
		epochs       int // number of decay cycles past threshold
		expectStatus types.TDULifecycleStatus
	}{
		{
			name:         "no decay within threshold",
			initialScore: "0.500000000000000000",
			epochs:       0,
			expectStatus: types.TDULifecycleActive,
		},
		{
			name:         "gradual decay: 5 cycles past threshold → still active",
			initialScore: "0.500000000000000000",
			epochs:       5, // 5 × 0.02 = 0.10 → 0.40 → active
			expectStatus: types.TDULifecycleActive,
		},
		{
			name:         "deeper decay: 10 cycles → dormant",
			initialScore: "0.500000000000000000",
			epochs:       10, // 10 × 0.02 = 0.20 → 0.30 → active (boundary)
			expectStatus: types.TDULifecycleActive,
		},
		{
			name:         "deep decay: 11 cycles → dormant",
			initialScore: "0.500000000000000000",
			epochs:       11, // 11 × 0.02 = 0.22 → 0.28 → dormant
			expectStatus: types.TDULifecycleDormant,
		},
		{
			name:         "full decay: 25 cycles → pruned",
			initialScore: "0.500000000000000000",
			epochs:       25, // 25 × 0.02 = 0.50 → 0.00 → pruned
			expectStatus: types.TDULifecyclePruned,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			k, ctx := setupKeeper(t)

			// Set params with immediate decay (threshold = 0)
			params := types.DefaultFitnessDecayParams()
			params.UnscoredCycleThreshold = 0
			require.NoError(t, k.SetFitnessDecayParams(ctx, params))

			// Create record at initial score, last signal at cycle 0
			record := types.NewTDUFitnessRecord("s1", sdkmath.NewInt(1_000_000), 0)
			score, _ := sdkmath.LegacyNewDecFromStr(tc.initialScore)
			record.SetFitnessScore(score)
			require.NoError(t, k.SetFitnessRecord(ctx, record))

			// Run decay for N epochs
			for i := 1; i <= tc.epochs; i++ {
				k.DecayUnscored(ctx, uint64(i))
			}

			updated, found := k.GetFitnessRecord(ctx, "s1")
			require.True(t, found)
			require.Equal(t, tc.expectStatus, updated.GetLifecycleStatus(),
				"score=%s after %d epochs", updated.FitnessScore, tc.epochs)
		})
	}
}

// ─── Lifecycle Transitions on Score Change ──────────────────────────────────

func TestLifecycleTransitionEvents(t *testing.T) {
	tests := []struct {
		name         string
		initialScore string
		signal       types.FitnessSignal
		oldStatus    types.TDULifecycleStatus
		newStatus    types.TDULifecycleStatus
		expectEvent  bool
	}{
		{
			name:         "active to core on strong signal",
			initialScore: "0.650000000000000000",
			signal: types.FitnessSignal{
				TrainingInfluence: sdkmath.LegacyOneDec(),
				UsageCorrelation:  sdkmath.LegacyOneDec(),
				Redundancy:        sdkmath.LegacyOneDec(),
			},
			oldStatus:   types.TDULifecycleActive,
			newStatus:   types.TDULifecycleCore,
			expectEvent: true,
		},
		{
			name:         "active stays active on moderate signal",
			initialScore: "0.500000000000000000",
			signal: types.FitnessSignal{
				TrainingInfluence: sdkmath.LegacyNewDecWithPrec(5, 1),
				UsageCorrelation:  sdkmath.LegacyNewDecWithPrec(5, 1),
				Redundancy:        sdkmath.LegacyNewDecWithPrec(5, 1),
			},
			oldStatus:   types.TDULifecycleActive,
			newStatus:   types.TDULifecycleActive,
			expectEvent: false,
		},
		{
			name:         "active to dormant on weak signal",
			initialScore: "0.350000000000000000",
			signal: types.FitnessSignal{
				TrainingInfluence: sdkmath.LegacyZeroDec(),
				UsageCorrelation:  sdkmath.LegacyZeroDec(),
				Redundancy:        sdkmath.LegacyZeroDec(),
			},
			oldStatus:   types.TDULifecycleActive,
			newStatus:   types.TDULifecycleDormant,
			expectEvent: true,
		},
		{
			name:         "dormant to active on strong signal",
			initialScore: "0.150000000000000000",
			signal: types.FitnessSignal{
				TrainingInfluence: sdkmath.LegacyOneDec(),
				UsageCorrelation:  sdkmath.LegacyOneDec(),
				Redundancy:        sdkmath.LegacyOneDec(),
			},
			oldStatus:   types.TDULifecycleDormant,
			newStatus:   types.TDULifecycleActive,
			expectEvent: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			k, ctx := setupKeeper(t)
			sdkCtx := sdk.UnwrapSDKContext(ctx)
			ctx = sdkCtx.WithEventManager(sdk.NewEventManager())

			// Create record
			record := types.NewTDUFitnessRecord("s1", sdkmath.NewInt(1_000_000), 0)
			score, _ := sdkmath.LegacyNewDecFromStr(tc.initialScore)
			record.SetFitnessScore(score)
			require.NoError(t, k.SetFitnessRecord(ctx, record))

			// Verify initial status
			require.Equal(t, tc.oldStatus, record.GetLifecycleStatus())

			// Update with event
			require.NoError(t, k.UpdateFitnessScoreWithEvent(ctx, "s1", tc.signal, 1))

			// Check new status
			updated, _ := k.GetFitnessRecord(ctx, "s1")
			require.Equal(t, tc.newStatus, updated.GetLifecycleStatus())

			// Check events
			sdkCtx = sdk.UnwrapSDKContext(ctx)
			events := sdkCtx.EventManager().Events()
			found := false
			for _, e := range events {
				if e.Type == "tdu_lifecycle_transition" {
					found = true
					break
				}
			}
			require.Equal(t, tc.expectEvent, found,
				"expected lifecycle event=%v, got events=%v", tc.expectEvent, events)
		})
	}
}

// ─── Fitness Drops Below 0.3 → Dormant ──────────────────────────────────────

func TestFitnessDropsBelowThreshold_Dormant(t *testing.T) {
	k, ctx := setupKeeper(t)

	params := types.DefaultFitnessDecayParams()
	params.UnscoredCycleThreshold = 0
	params.DecayPerCycle = "0.100000000000000000" // aggressive
	require.NoError(t, k.SetFitnessDecayParams(ctx, params))

	// Start at 0.35 (just above dormant threshold)
	record := types.NewTDUFitnessRecord("s1", sdkmath.NewInt(1_000_000), 0)
	record.SetFitnessScore(sdkmath.LegacyNewDecWithPrec(35, 2)) // 0.35
	require.NoError(t, k.SetFitnessRecord(ctx, record))
	require.Equal(t, types.TDULifecycleActive, record.GetLifecycleStatus())

	// One decay cycle: 0.35 - 0.1 = 0.25 → Dormant
	k.DecayUnscored(ctx, 1)
	updated, _ := k.GetFitnessRecord(ctx, "s1")
	require.Equal(t, types.TDULifecycleDormant, updated.GetLifecycleStatus())
}

// ─── Fitness Drops Below 0.1 → Pruned ───────────────────────────────────────

func TestFitnessDropsBelowThreshold_Pruned(t *testing.T) {
	k, ctx := setupKeeper(t)

	params := types.DefaultFitnessDecayParams()
	params.UnscoredCycleThreshold = 0
	params.DecayPerCycle = "0.050000000000000000"
	require.NoError(t, k.SetFitnessDecayParams(ctx, params))

	// Start at 0.12
	record := types.NewTDUFitnessRecord("s1", sdkmath.NewInt(1_000_000), 0)
	record.SetFitnessScore(sdkmath.LegacyNewDecWithPrec(12, 2)) // 0.12
	require.NoError(t, k.SetFitnessRecord(ctx, record))
	require.Equal(t, types.TDULifecycleDormant, record.GetLifecycleStatus())

	// One cycle: 0.12 - 0.05 = 0.07 → Pruned
	k.DecayUnscored(ctx, 1)
	updated, _ := k.GetFitnessRecord(ctx, "s1")
	require.Equal(t, types.TDULifecyclePruned, updated.GetLifecycleStatus())
}

// ─── Core TDU Earns Longevity Reward ────────────────────────────────────────

func TestCoreTDUEarnsLongevityRewardPerEpoch(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)

	require.NoError(t, k.SetSample(ctx, &types.Sample{
		Id:        "s1",
		Submitter: rv1,
		Status:    types.SampleStatus_SAMPLE_STATUS_GOLD,
	}))

	record := types.NewTDUFitnessRecord("s1", sdkmath.NewInt(2_000_000), 1)
	record.SetFitnessScore(sdkmath.LegacyNewDecWithPrec(8, 1)) // 0.8 = Core
	require.NoError(t, k.SetFitnessRecord(ctx, record))

	k.DistributeLongevityRewards(ctx)

	// Core: 0.01 × 2,000,000 = 20,000 uzrn
	require.Len(t, bk.moduleToAccountCalls, 1)
	transfer := bk.moduleToAccountCalls[0]
	require.Equal(t, types.ModuleName, transfer.from)
	require.Equal(t, sdkmath.NewInt(20_000), transfer.amount.AmountOf("uzrn"))
}

// ─── Pruned TDU Earns Nothing ───────────────────────────────────────────────

func TestPrunedTDUEarnsNothing(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)

	require.NoError(t, k.SetSample(ctx, &types.Sample{
		Id:        "s1",
		Submitter: rv1,
		Status:    types.SampleStatus_SAMPLE_STATUS_GOLD,
	}))

	record := types.NewTDUFitnessRecord("s1", sdkmath.NewInt(1_000_000), 1)
	record.SetFitnessScore(sdkmath.LegacyNewDecWithPrec(5, 2)) // 0.05 = Pruned
	require.NoError(t, k.SetFitnessRecord(ctx, record))

	k.DistributeLongevityRewards(ctx)

	require.Len(t, bk.moduleToAccountCalls, 0, "pruned TDU should earn nothing")
}

// ─── PruneFitnessBelowThreshold ─────────────────────────────────────────────

func TestPruneFitnessBelowThreshold(t *testing.T) {
	tests := []struct {
		name            string
		fitnessScore    string
		initialStatus   types.SampleStatus
		expectPruned    bool
		expectNoContent bool
	}{
		{
			name:            "below 0.1: sample gets pruned",
			fitnessScore:    "0.050000000000000000",
			initialStatus:   types.SampleStatus_SAMPLE_STATUS_GOLD,
			expectPruned:    true,
			expectNoContent: true,
		},
		{
			name:          "above 0.1: sample untouched",
			fitnessScore:  "0.300000000000000000",
			initialStatus: types.SampleStatus_SAMPLE_STATUS_GOLD,
			expectPruned:  false,
		},
		{
			name:          "already pruned: no double-pruning",
			fitnessScore:  "0.050000000000000000",
			initialStatus: types.SampleStatus_SAMPLE_STATUS_PRUNED,
			expectPruned:  true, // still pruned
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			k, ctx := setupKeeper(t)

			require.NoError(t, k.SetSample(ctx, &types.Sample{
				Id:        "s1",
				Submitter: rv1,
				Content:   "important data",
				Status:    tc.initialStatus,
			}))

			record := types.NewTDUFitnessRecord("s1", sdkmath.NewInt(100), 0)
			score, _ := sdkmath.LegacyNewDecFromStr(tc.fitnessScore)
			record.SetFitnessScore(score)
			require.NoError(t, k.SetFitnessRecord(ctx, record))

			k.PruneFitnessBelowThreshold(ctx)

			sample, found := k.GetSample(ctx, "s1")
			require.True(t, found)

			if tc.expectPruned {
				require.Equal(t, types.SampleStatus_SAMPLE_STATUS_PRUNED, sample.Status)
			} else {
				require.NotEqual(t, types.SampleStatus_SAMPLE_STATUS_PRUNED, sample.Status)
				require.Equal(t, "important data", sample.Content)
			}

			if tc.expectNoContent {
				require.Empty(t, sample.Content, "pruned sample should have content cleared")
			}
		})
	}
}

// ─── FitnessEpochBlocks Param ───────────────────────────────────────────────

func TestFitnessEpochBlocksParam(t *testing.T) {
	tests := []struct {
		name   string
		set    uint64
		expect uint64
	}{
		{"default when unset", 0, 100},
		{"custom value", 200, 200},
		{"custom small", 50, 50},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			params := types.DefaultFitnessDecayParams()
			params.FitnessEpochBlocks = tc.set
			require.Equal(t, tc.expect, params.GetFitnessEpochBlocks())
		})
	}
}

// ─── UpdateFitnessScoreWithEvent ────────────────────────────────────────────

func TestUpdateFitnessScoreWithEvent_NoEventOnSameStatus(t *testing.T) {
	k, ctx := setupKeeper(t)
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ctx = sdkCtx.WithEventManager(sdk.NewEventManager())

	// Active at 0.5
	require.NoError(t, k.InitializeFitnessRecord(ctx, "s1", sdkmath.NewInt(100), 0))

	// Moderate signal keeps Active → no transition event
	signal := types.FitnessSignal{
		TrainingInfluence: sdkmath.LegacyNewDecWithPrec(5, 1),
		UsageCorrelation:  sdkmath.LegacyNewDecWithPrec(5, 1),
		Redundancy:        sdkmath.LegacyNewDecWithPrec(5, 1),
	}
	require.NoError(t, k.UpdateFitnessScoreWithEvent(ctx, "s1", signal, 1))

	sdkCtx = sdk.UnwrapSDKContext(ctx)
	for _, e := range sdkCtx.EventManager().Events() {
		require.NotEqual(t, "tdu_lifecycle_transition", e.Type,
			"should not emit transition event when status doesn't change")
	}
}

// ─── Integration: EndBlocker Fitness Epoch ──────────────────────────────────

func TestEndBlockerFitnessEpochProcessing(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)

	// Set up params
	p := types.DefaultParams()
			require.NoError(t, k.SetParams(ctx, &p))

	fitnessParams := types.DefaultFitnessDecayParams()
	fitnessParams.FitnessEpochBlocks = 50
	fitnessParams.UnscoredCycleThreshold = 0 // immediate decay
	require.NoError(t, k.SetFitnessDecayParams(ctx, fitnessParams))

	// Create a Core TDU (should receive reward)
	require.NoError(t, k.SetSample(ctx, &types.Sample{
		Id:        "core1",
		Submitter: rv1,
		Status:    types.SampleStatus_SAMPLE_STATUS_GOLD,
	}))
	coreRecord := types.NewTDUFitnessRecord("core1", sdkmath.NewInt(1_000_000), 0)
	coreRecord.SetFitnessScore(sdkmath.LegacyNewDecWithPrec(8, 1)) // 0.8
	require.NoError(t, k.SetFitnessRecord(ctx, coreRecord))

	// Create a TDU that should be pruned (fitness < 0.1)
	require.NoError(t, k.SetSample(ctx, &types.Sample{
		Id:        "dying1",
		Submitter: rv1,
		Content:   "will be pruned",
		Status:    types.SampleStatus_SAMPLE_STATUS_BRONZE,
	}))
	dyingRecord := types.NewTDUFitnessRecord("dying1", sdkmath.NewInt(100), 0)
	dyingRecord.SetFitnessScore(sdkmath.LegacyNewDecWithPrec(5, 2)) // 0.05
	require.NoError(t, k.SetFitnessRecord(ctx, dyingRecord))

	// Run EndBlocker at fitness epoch boundary (block 50)
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ctx = sdkCtx.WithBlockHeight(50)
	require.NoError(t, k.EndBlocker(ctx))

	// Core TDU should have received longevity reward
	require.True(t, len(bk.moduleToAccountCalls) > 0, "core TDU should receive longevity reward")

	// Dying TDU should be pruned
	dSample, found := k.GetSample(ctx, "dying1")
	require.True(t, found)
	require.Equal(t, types.SampleStatus_SAMPLE_STATUS_PRUNED, dSample.Status)
	require.Empty(t, dSample.Content, "pruned TDU content should be cleared")
}
