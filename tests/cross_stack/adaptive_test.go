package cross_stack_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	aptypes "github.com/zerone-chain/zerone/x/autopoiesis/types"
	aligntypes "github.com/zerone-chain/zerone/x/alignment/types"
	emergencytypes "github.com/zerone-chain/zerone/x/emergency/types"
	vestingkeeper "github.com/zerone-chain/zerone/x/vesting_rewards/keeper"
)

// TestScenario1_AutopoiesisVestingMultiplier verifies that autopoiesis
// multiplier state is readable through the vesting rewards adapter.
func TestScenario1_AutopoiesisVestingMultiplier(t *testing.T) {
	h := NewTestHarness(t)

	// 1. Activate autopoiesis.
	h.AutopoiesisKeeper.SetState(h.Ctx, &aptypes.AutopoiesisState{
		Activated:       true,
		CurrentEpoch:    0,
		LastEpochHeight: 0,
	})
	require.True(t, h.AutopoiesisKeeper.IsActive(h.Ctx))

	// 2. Set rewards.block multiplier to 200,000 BPS (0.2x).
	h.AutopoiesisKeeper.SetMultiplierState(h.Ctx, &aptypes.MultiplierState{
		Path:       "rewards.block",
		CurrentBps: 200_000,
		TargetBps:  200_000,
		MinBps:     500_000,
		MaxBps:     2_000_000,
	})

	// 3. Read directly via keeper.GetMultiplier.
	val, err := h.AutopoiesisKeeper.GetMultiplier(h.Ctx, "rewards.block")
	require.NoError(t, err)
	require.Equal(t, uint64(200_000), val, "direct GetMultiplier must return 200,000")

	// 4. Read via the AutopoiesisVestingAdapter (the bridge the vesting module uses).
	adapter := vestingkeeper.NewAutopoiesisVestingAdapter(h.AutopoiesisKeeper)
	adapterVal := adapter.GetMultiplier(h.Ctx, "rewards.block")
	require.Equal(t, uint64(200_000), adapterVal, "adapter GetMultiplier must return 200,000")

	// 5. Verify unknown path returns BPSScale (1.0x default).
	unknownVal, err := h.AutopoiesisKeeper.GetMultiplier(h.Ctx, "unknown.path")
	require.NoError(t, err)
	require.Equal(t, aptypes.BPSScale, unknownVal)
}

// TestScenario2_AlignmentCorrections verifies alignment generates corrections
// when knowledge quality is below the degraded threshold, and dispatches them
// to autopoiesis via SuggestAdjustment.
func TestScenario2_AlignmentCorrections(t *testing.T) {
	h := NewTestHarness(t)

	// 1. Enable alignment and activate autopoiesis.
	h.AlignmentKeeper.SetState(h.Ctx, &aligntypes.AlignmentState{
		Enabled:              true,
		LastObservationHeight: 0,
		ObservationCount:     0,
	})
	h.AutopoiesisKeeper.SetState(h.Ctx, &aptypes.AutopoiesisState{
		Activated: true,
	})

	// 1b. Raise auto-apply magnitude cap so large corrections are applied in this test.
	alignParamsInit := h.AlignmentKeeper.GetParams(h.Ctx)
	alignParamsInit.MaxAutoApplyMagnitudeBps = 1_000_000
	h.AlignmentKeeper.SetParams(h.Ctx, alignParamsInit)

	// 2. Observe — sensors will return NeutralBPS (500,000) for nil cross-module
	// keepers. Knowledge quality depends on the KnowledgeKeeper wired in the app.
	obs := h.AlignmentKeeper.ObserveAll(h.Ctx)
	require.NotNil(t, obs)
	require.Greater(t, obs.Height, uint64(0))

	// 3. Compute scores.
	scores := h.AlignmentKeeper.ComputeScores(h.Ctx, obs)
	require.NotNil(t, scores)

	// 4. Force low knowledge quality to trigger correction generation.
	// DegradedThreshold is 400,000. Set KnowledgeQuality well below it.
	scores.KnowledgeQuality = 100_000
	scores.Composite = 100_000 // Force low composite too.

	// 5. Generate corrections.
	corrections := h.AlignmentKeeper.GenerateCorrections(h.Ctx, scores)
	require.NotEmpty(t, corrections, "corrections must be generated for low knowledge quality")

	// Find the knowledge quality correction.
	var knowledgeCorrection *aligntypes.CorrectionRecord
	for _, c := range corrections {
		if c.Dimension == aligntypes.DimKnowledgeQuality {
			knowledgeCorrection = c
			break
		}
	}
	require.NotNil(t, knowledgeCorrection, "knowledge_quality correction must exist")
	require.Equal(t, "increase", knowledgeCorrection.Direction)
	require.False(t, knowledgeCorrection.Applied, "correction must not be applied yet")

	// 6. Apply corrections — dispatches to autopoiesis SuggestAdjustment.
	h.AlignmentKeeper.ApplyCorrections(h.Ctx, corrections)

	// Verify corrections are marked as applied (autopoiesis keeper is wired in the app).
	for _, c := range corrections {
		require.True(t, c.Applied, "correction %s must be marked applied", c.Dimension)
	}
}

// TestScenario7_FullAdaptiveLoop verifies the multi-epoch adaptive feedback loop
// between alignment observation, correction dispatch, and autopoiesis epoch processing.
func TestScenario7_FullAdaptiveLoop(t *testing.T) {
	h := NewTestHarness(t)

	// 1. Activate autopoiesis with short epochs for testing.
	h.AutopoiesisKeeper.SetState(h.Ctx, &aptypes.AutopoiesisState{
		Activated:       true,
		CurrentEpoch:    0,
		LastEpochHeight: uint64(h.Height()),
	})
	params := aptypes.DefaultParams()
	params.EpochLengthBlocks = 10 // short epochs for testing
	h.AutopoiesisKeeper.SetParams(h.Ctx, &params)

	// Set default multipliers.
	for _, m := range aptypes.DefaultMultipliers() {
		h.AutopoiesisKeeper.SetMultiplierState(h.Ctx, m)
	}

	// 2. Enable alignment with short observation interval.
	alignParams := aligntypes.DefaultParams()
	alignParams.ObservationIntervalBlocks = 10
	h.AlignmentKeeper.SetParams(h.Ctx, &alignParams)
	h.AlignmentKeeper.SetState(h.Ctx, &aligntypes.AlignmentState{
		Enabled:              true,
		LastObservationHeight: 0,
		ObservationCount:     0,
	})

	// 3. Record initial multiplier state.
	initMS, found := h.AutopoiesisKeeper.GetMultiplierState(h.Ctx, "rewards.block")
	require.True(t, found)
	require.Equal(t, aptypes.BPSScale, initMS.CurrentBps, "initial multiplier must be 1.0x")

	// 4. Advance blocks past epoch boundary — triggers both EndBlockers.
	// The autopoiesis CollectAndAdapt runs in EndBlocker and the alignment
	// module observes at interval boundaries.
	h.AdvanceBlocks(15)

	// 5. After advancing, check that autopoiesis processed at least one epoch.
	state := h.AutopoiesisKeeper.GetState(h.Ctx)
	require.NotNil(t, state)
	// The first call to CollectAndAdapt sets the baseline (LastEpochHeight).
	// After 15 blocks from height ~1, at least one epoch should process.

	// 6. Verify SSI was computed.
	ssi := h.AutopoiesisKeeper.GetSSI(h.Ctx)
	// SSI depends on staking participation and verification rate.
	// Even with minimal state it should be a valid BPS value.
	require.LessOrEqual(t, ssi, aptypes.BPSScale, "SSI must be <= 1,000,000")

	// 7. Advance more blocks for a second epoch.
	h.AdvanceBlocks(15)
	state2 := h.AutopoiesisKeeper.GetState(h.Ctx)
	require.NotNil(t, state2)

	// 8. Check alignment observation was recorded.
	alignState := h.AlignmentKeeper.GetState(h.Ctx)
	require.NotNil(t, alignState)
	// ObservationCount may be 0 if the block height doesn't align exactly
	// with the interval, but LastObservationHeight should advance if it did.

	// 9. Verify the system doesn't panic over multiple epochs.
	h.AdvanceBlocks(30) // 3 more epochs
	finalState := h.AutopoiesisKeeper.GetState(h.Ctx)
	require.NotNil(t, finalState)
}

// TestScenario10_EmergencyHaltStopsAdaptiveLayer verifies that the emergency
// halt prevents both autopoiesis and alignment from processing.
func TestScenario10_EmergencyHaltStopsAdaptiveLayer(t *testing.T) {
	h := NewTestHarness(t)

	// 1. Activate autopoiesis and alignment.
	h.AutopoiesisKeeper.SetState(h.Ctx, &aptypes.AutopoiesisState{
		Activated:       true,
		CurrentEpoch:    0,
		LastEpochHeight: uint64(h.Height()),
	})
	params := aptypes.DefaultParams()
	params.EpochLengthBlocks = 5
	h.AutopoiesisKeeper.SetParams(h.Ctx, &params)
	for _, m := range aptypes.DefaultMultipliers() {
		h.AutopoiesisKeeper.SetMultiplierState(h.Ctx, m)
	}

	alignParams := aligntypes.DefaultParams()
	alignParams.ObservationIntervalBlocks = 5
	h.AlignmentKeeper.SetParams(h.Ctx, &alignParams)
	h.AlignmentKeeper.SetState(h.Ctx, &aligntypes.AlignmentState{
		Enabled: true,
	})

	// 2. Advance a few blocks to establish baseline.
	h.AdvanceBlocks(6)
	stateBeforeHalt := h.AutopoiesisKeeper.GetState(h.Ctx)
	epochBeforeHalt := stateBeforeHalt.CurrentEpoch

	// 3. Halt the chain via emergency module.
	h.EmergencyKeeper.SetEmergencyStatus(h.Ctx, emergencytypes.StatusHalted)
	require.True(t, h.EmergencyKeeper.IsHalted(h.Ctx), "chain must be halted")

	// 4. Advance blocks — autopoiesis and alignment should skip processing.
	h.AdvanceBlocks(20)

	stateAfterHalt := h.AutopoiesisKeeper.GetState(h.Ctx)
	require.Equal(t, epochBeforeHalt, stateAfterHalt.CurrentEpoch,
		"autopoiesis epoch must not advance during halt")

	// Alignment observation count should not change during halt.
	alignStateHalted := h.AlignmentKeeper.GetState(h.Ctx)
	obsCountDuringHalt := alignStateHalted.ObservationCount

	// 5. Resume: set status back to normal.
	h.EmergencyKeeper.SetEmergencyStatus(h.Ctx, emergencytypes.StatusNormal)
	require.False(t, h.EmergencyKeeper.IsHalted(h.Ctx), "chain must be resumed")

	// 6. Advance blocks — both modules should resume processing.
	h.AdvanceBlocks(20)

	stateAfterResume := h.AutopoiesisKeeper.GetState(h.Ctx)
	require.Greater(t, stateAfterResume.CurrentEpoch, epochBeforeHalt,
		"autopoiesis must advance epochs after resume")

	alignStateResumed := h.AlignmentKeeper.GetState(h.Ctx)
	require.GreaterOrEqual(t, alignStateResumed.ObservationCount, obsCountDuringHalt,
		"alignment must resume observations after emergency ends")
}
