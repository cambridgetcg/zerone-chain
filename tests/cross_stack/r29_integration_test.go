package cross_stack_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	aligntypes "github.com/zerone-chain/zerone/x/alignment/types"
	aptypes "github.com/zerone-chain/zerone/x/autopoiesis/types"
	cdtypes "github.com/zerone-chain/zerone/x/capture_defense/types"
	knowledgekeeper "github.com/zerone-chain/zerone/x/knowledge/keeper"
	knowledgetypes "github.com/zerone-chain/zerone/x/knowledge/types"
)

// TestR29_FullEcosystemCycle exercises all six R29 polarities in sequence:
// carrying capacity, epistemic temperature, role elasticity, correction confidence,
// structural immunity, and adaptive pacing — all interacting through a single test app.
func TestR29_FullEcosystemCycle(t *testing.T) {
	h := NewTestHarness(t)
	domain := "physics"

	// ── Setup: Enable alignment and autopoiesis with short intervals ─────

	h.AlignmentKeeper.SetState(h.Ctx, &aligntypes.AlignmentState{
		Enabled:              true,
		LastObservationHeight: 0,
		ObservationCount:     0,
		PreviousCategory:     aligntypes.CategoryHealthy,
	})
	alignParams := aligntypes.DefaultParams()
	alignParams.ObservationIntervalBlocks = 10
	alignParams.MaxAutoApplyMagnitudeBps = 1_000_000
	h.AlignmentKeeper.SetParams(h.Ctx, &alignParams)

	h.AutopoiesisKeeper.SetState(h.Ctx, &aptypes.AutopoiesisState{
		Activated:       true,
		CurrentEpoch:    0,
		LastEpochHeight: uint64(h.Height()),
	})
	apParams := aptypes.DefaultParams()
	apParams.EpochLengthBlocks = 10
	h.AutopoiesisKeeper.SetParams(h.Ctx, &apParams)
	for _, m := range aptypes.DefaultMultipliers() {
		h.AutopoiesisKeeper.SetMultiplierState(h.Ctx, m)
	}

	// ── Step 1: Populate domain past carrying capacity (R29-1) ───────────

	// Default DomainBaseCapacity = 1000. Set 1500 active facts → overcrowded.
	h.KnowledgeKeeper.SetDomainStats(h.Ctx, &knowledgekeeper.DomainStats{
		Domain:      domain,
		ActiveCount: 1500,
		AtRiskCount: 100,
		TotalEnergy: 1_600_000,
		LastUpdated: uint64(h.Height()),
	})

	pressure := h.KnowledgeKeeper.GetDomainPressure(h.Ctx, domain)
	require.Greater(t, pressure, uint64(1_000_000), "domain must be overcrowded (pressure > 1M BPS)")
	require.Equal(t, "overcrowded", knowledgekeeper.PressureCategory(pressure))

	// Death pressure should be accelerated for overcrowded domains.
	deathMul := h.KnowledgeKeeper.GetDeathPressureMultiplier(h.Ctx, domain)
	require.Greater(t, deathMul, uint64(1_000_000), "overcrowded domain must have accelerated decay")

	// Birth pressure: no bonus in overcrowded domain.
	boosted := h.KnowledgeKeeper.ApplyBirthPressure(h.Ctx, domain, 100_000)
	require.Equal(t, uint64(100_000), boosted, "overcrowded domain must give zero birth bonus")

	// ── Step 2: Verify epistemic temperature starts neutral (R29-2) ──────

	epState, err := h.KnowledgeKeeper.GetOrInitDomainEpistemicState(h.Ctx, domain)
	require.NoError(t, err)
	require.Equal(t, uint64(500_000), epState.Temperature, "initial temperature must be neutral (500,000)")

	// ── Step 3: Conformity cooling (R29-2) ───────────────────────────────

	// Set up knowledge params with short epochs for conformity detection.
	kParams, err := h.KnowledgeKeeper.GetParams(h.Ctx)
	require.NoError(t, err)
	kParams.FitnessEpochBlocks = 10
	require.NoError(t, h.KnowledgeKeeper.SetParams(h.Ctx, kParams))

	// Create a low-diversity record for the current epoch → triggers conformity cooling.
	currentEpoch := uint64(h.Height()) / kParams.FitnessEpochBlocks
	err = h.KnowledgeKeeper.SetDomainDiversity(h.Ctx, domain, currentEpoch, knowledgekeeper.DomainDiversityRecord{
		Domain:         domain,
		Epoch:          currentEpoch,
		AvgEntropy:     10_000, // Far below conformity alert threshold (50,000)
		RoundCount:     5,
		UnanimousCount: 5,
	})
	require.NoError(t, err)

	// Run temperature update — should cool the domain.
	err = h.KnowledgeKeeper.UpdateEpistemicTemperature(h.Ctx, domain)
	require.NoError(t, err)

	epState, _, err = h.KnowledgeKeeper.GetDomainEpistemicState(h.Ctx, domain)
	require.NoError(t, err)
	require.Less(t, epState.Temperature, uint64(500_000), "temperature must cool below neutral after conformity")
	require.Equal(t, uint64(1), epState.ConformityStreak)
	cooledTemp := epState.Temperature

	// ── Step 4: Vindication heating (R29-2) ──────────────────────────────

	// Create a fact in the domain and add a vindication record.
	factID := "fact-vindicated-1"
	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, &knowledgetypes.Fact{
		Id:               factID,
		Domain:           domain,
		Content:          "E=mc^2",
		Category:         "empirical",
		Confidence:       300_000,
		Status:           knowledgetypes.FactStatus_FACT_STATUS_VERIFIED,
		SubmittedAtBlock: 1,
	}))

	err = h.KnowledgeKeeper.SetVindicationRecord(h.Ctx, factID, knowledgetypes.VindicationRecord{
		Verifier:     "zerone1validator1",
		FactId:       factID,
		VindicatedAt: uint64(h.Height()),
	})
	require.NoError(t, err)

	// Run temperature update again — vindication should heat the domain.
	err = h.KnowledgeKeeper.UpdateEpistemicTemperature(h.Ctx, domain)
	require.NoError(t, err)

	epState, _, err = h.KnowledgeKeeper.GetDomainEpistemicState(h.Ctx, domain)
	require.NoError(t, err)
	require.Greater(t, epState.Temperature, cooledTemp, "temperature must heat after vindication")

	// ── Step 5: Role elasticity updated from vindication (R29-3) ─────────

	// Seed role records: agents were incorrect more than humans.
	err = h.KnowledgeKeeper.SetDomainRoleRecord(h.Ctx, &knowledgetypes.DomainRoleRecord{
		Domain:              domain,
		AgentCorrectCalls:   30,
		AgentIncorrectCalls: 20,
		HumanCorrectCalls:   45,
		HumanIncorrectCalls: 5,
		LastUpdated:         uint64(h.Height()),
	})
	require.NoError(t, err)

	agentBonus, humanBonus := h.KnowledgeKeeper.GetRoleElasticity(h.Ctx, domain)
	// Human accuracy (90%) > agent accuracy (60%), so agent bonus should be boosted
	// (weaker role gets more incentive).
	require.Greater(t, agentBonus, uint64(0), "agent bonus must be non-zero")
	require.Greater(t, humanBonus, uint64(0), "human bonus must be non-zero")

	// ── Step 6: Alignment observes and generates corrections (R28-7, R29-4) ─

	obs := h.AlignmentKeeper.ObserveAll(h.Ctx)
	require.NotNil(t, obs)
	scores := h.AlignmentKeeper.ComputeScores(h.Ctx, obs)
	require.NotNil(t, scores)

	// Force low knowledge quality to trigger correction generation.
	scores.KnowledgeQuality = 100_000
	scores.Composite = 100_000
	corrections := h.AlignmentKeeper.GenerateCorrections(h.Ctx, scores)
	require.NotEmpty(t, corrections, "corrections must be generated for low knowledge quality")

	// ── Step 7: Apply corrections → record outcomes → check confidence (R29-4)

	h.AlignmentKeeper.ApplyCorrections(h.Ctx, corrections)
	for _, c := range corrections {
		require.True(t, c.Applied, "correction %s must be applied", c.Dimension)
	}

	// Record a successful outcome to build correction confidence.
	h.AlignmentKeeper.SetCorrectionOutcome(h.Ctx, &aligntypes.CorrectionOutcome{
		Height:      uint64(h.Height()),
		Dimension:   aligntypes.DimKnowledgeQuality,
		Magnitude:   50_000,
		Direction:   "increase",
		ScoreBefore: 100_000,
		ScoreAfter:  400_000,
		Successful:  true,
	})

	// Confidence should still be neutral (needs more samples).
	confidence := h.AlignmentKeeper.GetCorrectionConfidence(h.Ctx)
	require.Greater(t, confidence, uint64(0), "correction confidence must be > 0")

	// ── Step 8: Degrade health → verify pacing changes (R29-6) ──────────

	h.AlignmentKeeper.SetState(h.Ctx, &aligntypes.AlignmentState{
		Enabled:               true,
		LastObservationHeight: uint64(h.Height()),
		ObservationCount:      1,
		PreviousCategory:      aligntypes.CategoryCritical,
	})

	creationBps, analysisBps := h.AlignmentKeeper.GetGlobalPacingMultiplier(h.Ctx)
	require.Equal(t, uint64(500_000), creationBps, "critical health → 50%% creation pacing")
	require.Equal(t, uint64(2_000_000), analysisBps, "critical health → 200%% analysis pacing")

	// ── Step 9: Flag domain for capture → verify partnership bonus (R29-5) ─

	h.CaptureDefenseKeeper.SetCaptureMetrics(h.Ctx, &cdtypes.CaptureMetrics{
		Domain:          domain,
		HerfindahlIndex: 800_000,
		RiskScore:       850_000,
		Flagged:         true,
		AnalyzedAtBlock: uint64(h.Height()),
	})

	require.True(t, h.CaptureDefenseKeeper.IsDomainFlagged(h.Ctx, domain))

	// OnDomainFlagged triggers partnership formation bonus.
	h.CaptureDefenseKeeper.OnDomainFlagged(h.Ctx, domain)
	bonus := h.PartnershipsKeeper.GetDomainFormationBonus(h.Ctx, domain)
	require.NotNil(t, bonus, "flagged domain must get formation bonus")
	require.Greater(t, bonus.BonusBps, uint64(0))

	// ── Step 10: Recovery → verify pacing normalises ─────────────────────

	h.AlignmentKeeper.SetState(h.Ctx, &aligntypes.AlignmentState{
		Enabled:               true,
		LastObservationHeight: uint64(h.Height()),
		ObservationCount:      2,
		PreviousCategory:      aligntypes.CategoryHealthy,
	})

	creationBps, analysisBps = h.AlignmentKeeper.GetGlobalPacingMultiplier(h.Ctx)
	require.Equal(t, uint64(1_000_000), creationBps, "healthy → 100%% creation pacing")
	require.Equal(t, uint64(1_000_000), analysisBps, "healthy → 100%% analysis pacing")

	// Advance blocks to confirm no panics with all this state present.
	h.AdvanceBlocks(20)
}
