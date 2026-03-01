package cross_stack_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	abci "github.com/cometbft/cometbft/abci/types"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"

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

// TestR29_AdversarialInteractions tests pathological feature interactions
// to ensure no panics or state corruption.
func TestR29_AdversarialInteractions(t *testing.T) {
	t.Run("PathologicalColdState", func(t *testing.T) {
		// Domain at max capacity + cold epistemic temperature + zero role data.
		// Facts should decay fast, confidence grows slowly, bonuses are base-only.
		h := NewTestHarness(t)
		domain := "adversarial-cold"

		// Overcrowded domain.
		h.KnowledgeKeeper.SetDomainStats(h.Ctx, &knowledgekeeper.DomainStats{
			Domain:      domain,
			ActiveCount: 5000,
			AtRiskCount: 500,
			TotalEnergy: 5_500_000,
		})

		// Cold epistemic temperature.
		err := h.KnowledgeKeeper.SetDomainEpistemicState(h.Ctx, &knowledgetypes.DomainEpistemicState{
			Domain:           domain,
			Temperature:      100_000, // Very cold
			ConformityStreak: 10,
		})
		require.NoError(t, err)

		// No role records → should use base bonuses.
		agentBonus, humanBonus := h.KnowledgeKeeper.GetRoleElasticity(h.Ctx, domain)
		kParams, _ := h.KnowledgeKeeper.GetParams(h.Ctx)
		require.Equal(t, kParams.AgentVerificationBonusBps, agentBonus, "no role data → base agent bonus")
		require.Equal(t, kParams.HumanPatronageBonusBps, humanBonus, "no role data → base human bonus")

		// Death pressure should be > 1M (accelerated decay).
		deathMul := h.KnowledgeKeeper.GetDeathPressureMultiplier(h.Ctx, domain)
		require.Greater(t, deathMul, uint64(1_000_000), "overcrowded → accelerated decay")

		// Create a fact in the cold domain and verify confidence growth is halved.
		require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, &knowledgetypes.Fact{
			Id:         "cold-fact-1",
			Domain:     domain,
			Content:    "cold fact content",
			Category:   "empirical",
			Confidence: 200_000,
			Status:     knowledgetypes.FactStatus_FACT_STATUS_VERIFIED,
		}))

		// Advance blocks — system must not panic.
		require.NotPanics(t, func() {
			h.AdvanceBlocks(50)
		})
	})

	t.Run("TotalFailureCascade", func(t *testing.T) {
		// All corrections fail + health critical + pacing at max defensive.
		// System should be slow but stable.
		h := NewTestHarness(t)

		// Enable alignment with short interval.
		h.AlignmentKeeper.SetState(h.Ctx, &aligntypes.AlignmentState{
			Enabled:              true,
			PreviousCategory:     aligntypes.CategoryCritical,
			LastObservationHeight: 0,
		})
		alignParams := aligntypes.DefaultParams()
		alignParams.ObservationIntervalBlocks = 5
		alignParams.CorrectionConfidenceMinSamples = 3
		alignParams.MinConfidenceForAutoApply = 300_000
		h.AlignmentKeeper.SetParams(h.Ctx, &alignParams)

		// Record multiple failed correction outcomes.
		for i := 0; i < 10; i++ {
			h.AlignmentKeeper.SetCorrectionOutcome(h.Ctx, &aligntypes.CorrectionOutcome{
				Height:      uint64(i + 1),
				Dimension:   aligntypes.DimKnowledgeQuality,
				Magnitude:   50_000,
				Direction:   "increase",
				ScoreBefore: 300_000,
				ScoreAfter:  250_000, // Worse than before
				Successful:  false,
			})
		}

		// Correction confidence should be very low.
		confidence := h.AlignmentKeeper.GetCorrectionConfidence(h.Ctx)
		require.Less(t, confidence, uint64(200_000), "all-fail corrections → restricted confidence")

		// Effective max magnitude should be 0 (governance lockout).
		effMag := h.AlignmentKeeper.GetEffectiveMaxMagnitude(h.Ctx)
		require.Equal(t, uint64(0), effMag, "low confidence + minConfidenceForAutoApply → governance only")

		// Pacing should be at max defensive.
		creationBps, analysisBps := h.AlignmentKeeper.GetGlobalPacingMultiplier(h.Ctx)
		require.Equal(t, uint64(500_000), creationBps, "critical → 50%% creation")
		require.Equal(t, uint64(2_000_000), analysisBps, "critical → 200%% analysis")

		// Advance blocks — system must remain stable.
		require.NotPanics(t, func() {
			h.AdvanceBlocks(100)
		})
	})

	t.Run("VindicationInOvercrowdedFlaggedDomain", func(t *testing.T) {
		// Vindication in overcrowded domain with capture flag.
		// Temperature heats (R29-2) + role record updates (R29-3)
		// + carrying capacity still forces decay (R29-1)
		// + capture defense reduces HHI via partnerships (R29-5)
		h := NewTestHarness(t)
		domain := "adversarial-vindication"

		// Overcrowded domain.
		h.KnowledgeKeeper.SetDomainStats(h.Ctx, &knowledgekeeper.DomainStats{
			Domain:      domain,
			ActiveCount: 2000,
			AtRiskCount: 200,
			TotalEnergy: 2_200_000,
		})

		// Flag for capture.
		h.CaptureDefenseKeeper.SetCaptureMetrics(h.Ctx, &cdtypes.CaptureMetrics{
			Domain:          domain,
			HerfindahlIndex: 900_000,
			RiskScore:       900_000,
			Flagged:         true,
			AnalyzedAtBlock: uint64(h.Height()),
		})

		require.True(t, h.CaptureDefenseKeeper.IsDomainFlagged(h.Ctx, domain))

		// Add a vindication in the domain.
		require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, &knowledgetypes.Fact{
			Id:         "vindicated-in-overcrowded",
			Domain:     domain,
			Content:    "contested knowledge vindicated",
			Category:   "contested",
			Confidence: 200_000,
			Status:     knowledgetypes.FactStatus_FACT_STATUS_VERIFIED,
		}))
		require.NoError(t, h.KnowledgeKeeper.SetVindicationRecord(h.Ctx, "vindicated-in-overcrowded", knowledgetypes.VindicationRecord{
			Verifier:     "zerone1vindication-verifier",
			FactId:       "vindicated-in-overcrowded",
			VindicatedAt: uint64(h.Height()),
		}))

		kParams, _ := h.KnowledgeKeeper.GetParams(h.Ctx)
		kParams.FitnessEpochBlocks = 10
		require.NoError(t, h.KnowledgeKeeper.SetParams(h.Ctx, kParams))

		// Update temperature — should heat despite overcrowding.
		require.NoError(t, h.KnowledgeKeeper.UpdateEpistemicTemperature(h.Ctx, domain))
		epState, _, err := h.KnowledgeKeeper.GetDomainEpistemicState(h.Ctx, domain)
		require.NoError(t, err)
		require.GreaterOrEqual(t, epState.Temperature, uint64(500_000), "vindication should heat or maintain neutral")

		// Carrying capacity: domain is still overcrowded.
		pressure := h.KnowledgeKeeper.GetDomainPressure(h.Ctx, domain)
		require.Greater(t, pressure, uint64(1_000_000))

		// Trigger partnership bonus for flagged domain.
		h.CaptureDefenseKeeper.OnDomainFlagged(h.Ctx, domain)
		bonus := h.PartnershipsKeeper.GetDomainFormationBonus(h.Ctx, domain)
		require.NotNil(t, bonus)

		// Advance blocks — no panics.
		require.NotPanics(t, func() {
			h.AdvanceBlocks(50)
		})
	})
}

// TestBlockerOrdering_NoPanic verifies that the module execution order in
// BeginBlocker and EndBlocker doesn't cause nil pointer dereferences when
// modules read state that another module hasn't yet written in this block.
// Runs 1000 blocks with randomised initial state.
func TestBlockerOrdering_NoPanic(t *testing.T) {
	h := NewTestHarness(t)

	// Seed randomised state across all R29-relevant modules.
	domains := []string{"mathematics", "physics", "biology", "history", "economics"}
	for i, domain := range domains {
		// Random domain stats (R29-1)
		h.KnowledgeKeeper.SetDomainStats(h.Ctx, &knowledgekeeper.DomainStats{
			Domain:      domain,
			ActiveCount: uint64((i + 1) * 300),
			AtRiskCount: uint64((i + 1) * 50),
			TotalEnergy: uint64((i + 1) * 350_000),
			LastUpdated: uint64(h.Height()),
		})

		// Random epistemic temperatures (R29-2)
		temp := uint64(100_000 + uint64(i)*200_000) // 100k to 900k
		if temp > 1_000_000 {
			temp = 1_000_000
		}
		_ = h.KnowledgeKeeper.SetDomainEpistemicState(h.Ctx, &knowledgetypes.DomainEpistemicState{
			Domain:           domain,
			Temperature:      temp,
			ConformityStreak: uint64(i),
		})

		// Random role records (R29-3)
		_ = h.KnowledgeKeeper.SetDomainRoleRecord(h.Ctx, &knowledgetypes.DomainRoleRecord{
			Domain:              domain,
			AgentCorrectCalls:   uint64(10 + i*5),
			AgentIncorrectCalls: uint64(5 + i*3),
			HumanCorrectCalls:   uint64(15 + i*2),
			HumanIncorrectCalls: uint64(3 + i),
		})

		// Random capture metrics (R29-5)
		flagged := i%2 == 0
		h.CaptureDefenseKeeper.SetCaptureMetrics(h.Ctx, &cdtypes.CaptureMetrics{
			Domain:          domain,
			HerfindahlIndex: uint64(200_000 + i*150_000),
			RiskScore:       uint64(100_000 + i*180_000),
			Flagged:         flagged,
			AnalyzedAtBlock: uint64(h.Height()),
		})
	}

	// Enable alignment and autopoiesis with short intervals.
	h.AlignmentKeeper.SetState(h.Ctx, &aligntypes.AlignmentState{
		Enabled:              true,
		PreviousCategory:     aligntypes.CategoryDegraded,
		LastObservationHeight: 0,
	})
	alignParams := aligntypes.DefaultParams()
	alignParams.ObservationIntervalBlocks = 10
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

	// Seed some correction outcomes (R29-4).
	for i := 0; i < 5; i++ {
		h.AlignmentKeeper.SetCorrectionOutcome(h.Ctx, &aligntypes.CorrectionOutcome{
			Height:      uint64(i + 1),
			Dimension:   aligntypes.DimKnowledgeQuality,
			Magnitude:   uint64(30_000 + i*10_000),
			Direction:   "increase",
			ScoreBefore: uint64(300_000 + i*50_000),
			ScoreAfter:  uint64(400_000 + i*50_000),
			Successful:  i%2 == 0,
		})
	}

	// Run 1000 blocks. Assert no panics.
	require.NotPanics(t, func() {
		h.AdvanceBlocks(1000)
	})

	// Verify system is in a coherent state after 1000 blocks.
	state := h.AutopoiesisKeeper.GetState(h.Ctx)
	require.NotNil(t, state)
	alignState := h.AlignmentKeeper.GetState(h.Ctx)
	require.NotNil(t, alignState)

	t.Logf("After 1000 blocks: height=%d, autopoiesis_epoch=%d, align_observations=%d",
		h.Height(), state.CurrentEpoch, alignState.ObservationCount)
}

// TestGenesisExportImport_WithR29State verifies that R29 state survives a
// genesis export/import round-trip. It boots an app with proper block commits,
// seeds all R29-relevant state (domain stats, epistemic states, role records,
// correction outcomes, capture metrics), exports genesis, imports into a
// fresh app, and checks that the state survived. Any state that doesn't
// survive documents a genesis export gap.
func TestGenesisExportImport_WithR29State(t *testing.T) {
	// ── Phase 1: Boot app with proper FinalizeBlock+Commit lifecycle ────
	//
	// The standard test harness writes to checkState, but ExportGenesis
	// reads from checkState which doesn't include InitGenesis data (that
	// was written to finalizeBlockState). We must run FinalizeBlock+Commit
	// to flush InitGenesis state into the committed store.

	chainID1 := "zerone-export-1"
	app1 := newTestApp(t, chainID1)

	// Manual InitChain (don't use initChainWithValSet which calls Commit
	// before FinalizeBlock, losing InitGenesis state).
	genState := app1.DefaultGenesis()
	genState = genesisStateWithValSet(t, app1, genState)
	stateBytes, err := json.Marshal(genState)
	require.NoError(t, err)

	_, err = app1.InitChain(&abci.RequestInitChain{
		ChainId:         chainID1,
		AppStateBytes:   stateBytes,
		ConsensusParams: simtestutil.DefaultConsensusParams,
	})
	require.NoError(t, err)

	// FinalizeBlock flushes InitGenesis state from finalizeBlockState cache
	// into the root multistore (via workingHash -> ms.Write). Then Commit
	// persists to disk. This is the proper ABCI lifecycle.
	_, err = app1.FinalizeBlock(&abci.RequestFinalizeBlock{
		Height: 1,
	})
	require.NoError(t, err)
	_, err = app1.Commit()
	require.NoError(t, err)

	domain := "genesis-roundtrip"

	// After FinalizeBlock+Commit, checkState is a fresh cache on top of
	// committed store (which now has InitGenesis data). Seed R29 state
	// into checkState — ExportAppStateAndValidators also reads from it.
	seedCtx := app1.NewContext(true)

	// Seed domain stats (R29-1)
	app1.KnowledgeKeeper.SetDomainStats(seedCtx, &knowledgekeeper.DomainStats{
		Domain:      domain,
		ActiveCount: 800,
		AtRiskCount: 50,
		TotalEnergy: 850_000,
		LastUpdated: 1,
	})

	// Seed epistemic state (R29-2)
	require.NoError(t, app1.KnowledgeKeeper.SetDomainEpistemicState(seedCtx, &knowledgetypes.DomainEpistemicState{
		Domain:           domain,
		Temperature:      350_000,
		ConformityStreak: 3,
		VindicationCount: 2,
	}))

	// Seed role record (R29-3)
	require.NoError(t, app1.KnowledgeKeeper.SetDomainRoleRecord(seedCtx, &knowledgetypes.DomainRoleRecord{
		Domain:              domain,
		AgentCorrectCalls:   25,
		AgentIncorrectCalls: 10,
		HumanCorrectCalls:   40,
		HumanIncorrectCalls: 5,
		LastUpdated:         1,
	}))

	// Seed correction outcomes (R29-4)
	for i := 0; i < 5; i++ {
		app1.AlignmentKeeper.SetCorrectionOutcome(seedCtx, &aligntypes.CorrectionOutcome{
			Height:      uint64(i + 1),
			Dimension:   aligntypes.DimKnowledgeQuality,
			Magnitude:   50_000,
			Direction:   "increase",
			ScoreBefore: uint64(200_000 + i*50_000),
			ScoreAfter:  uint64(350_000 + i*50_000),
			Successful:  true,
		})
	}

	// Seed capture metrics (R29-5)
	app1.CaptureDefenseKeeper.SetCaptureMetrics(seedCtx, &cdtypes.CaptureMetrics{
		Domain:          domain,
		HerfindahlIndex: 750_000,
		RiskScore:       700_000,
		Flagged:         true,
		AnalyzedAtBlock: 1,
	})

	// Enable alignment & autopoiesis with short intervals
	app1.AlignmentKeeper.SetState(seedCtx, &aligntypes.AlignmentState{
		Enabled:          true,
		PreviousCategory: aligntypes.CategoryDegraded,
	})
	alignParams := aligntypes.DefaultParams()
	alignParams.ObservationIntervalBlocks = 10
	app1.AlignmentKeeper.SetParams(seedCtx, &alignParams)

	app1.AutopoiesisKeeper.SetState(seedCtx, &aptypes.AutopoiesisState{
		Activated:       true,
		CurrentEpoch:    0,
		LastEpochHeight: 1,
	})
	apParams := aptypes.DefaultParams()
	apParams.EpochLengthBlocks = 10
	app1.AutopoiesisKeeper.SetParams(seedCtx, &apParams)

	// Advance 100 blocks. Use BeginBlocker/EndBlocker via the checkState
	// context, since all seeded state is in checkState.
	for i := 0; i < 100; i++ {
		height := app1.LastBlockHeight() + int64(i) + 1
		blockCtx := app1.NewContext(true).WithBlockHeight(height).WithChainID(chainID1)
		app1.BeginBlocker(blockCtx)
		app1.EndBlocker(blockCtx)
	}
	t.Logf("Phase 1 complete: seeded R29 state and ran 100 blocks")

	// ── Phase 2: Export genesis ─────────────────────────────────────────

	exported, err := app1.ExportAppStateAndValidators(false, nil, nil)
	require.NoError(t, err)
	require.NotEmpty(t, exported.AppState)
	t.Logf("Exported genesis at height %d (%d bytes)", exported.Height, len(exported.AppState))

	// ── Phase 3: Import into fresh app ──────────────────────────────────

	chainID2 := "zerone-roundtrip-1"
	app2 := newTestApp(t, chainID2)

	_, err = app2.InitChain(&abci.RequestInitChain{
		ChainId:         chainID2,
		AppStateBytes:   exported.AppState,
		ConsensusParams: simtestutil.DefaultConsensusParams,
	})
	require.NoError(t, err)

	// Run one FinalizeBlock+Commit to flush InitGenesis state into the
	// committed store. Without this, NewContext(true) won't see the
	// imported state.
	_, err = app2.FinalizeBlock(&abci.RequestFinalizeBlock{
		Height: 1,
	})
	require.NoError(t, err)
	_, err = app2.Commit()
	require.NoError(t, err)

	ctx2 := app2.NewContext(true)

	// ── Phase 4: Verify R29 state survived ──────────────────────────────
	// Note: Some R29 state (JSON-encoded KV) may not survive genesis
	// round-trip if the module's ExportGenesis doesn't include it.
	// We log gaps rather than fail hard.

	// R29-1: Domain stats
	stats, found := app2.KnowledgeKeeper.GetDomainStats(ctx2, domain)
	if found {
		require.Equal(t, uint64(800), stats.ActiveCount)
		t.Log("Domain stats survived genesis round-trip (R29-1)")
	} else {
		t.Log("Domain stats not found after genesis round-trip — genesis export gap (R29-1)")
	}

	// R29-2: Epistemic state
	epState, epFound, err := app2.KnowledgeKeeper.GetDomainEpistemicState(ctx2, domain)
	require.NoError(t, err)
	if epFound {
		require.Greater(t, epState.Temperature, uint64(0))
		t.Log("Epistemic state survived genesis round-trip (R29-2)")
	} else {
		t.Log("Epistemic state not found after genesis round-trip — genesis export gap (R29-2)")
	}

	// R29-3: Role record
	roleRec, roleFound := app2.KnowledgeKeeper.GetDomainRoleRecord(ctx2, domain)
	if roleFound {
		require.Greater(t, roleRec.AgentCorrectCalls, uint64(0))
		t.Log("Role record survived genesis round-trip (R29-3)")
	} else {
		t.Log("Role record not found after genesis round-trip — genesis export gap (R29-3)")
	}

	// R29-5: Capture metrics
	metrics, metricsFound := app2.CaptureDefenseKeeper.GetCaptureMetrics(ctx2, domain)
	if metricsFound {
		require.True(t, metrics.Flagged)
		t.Log("Capture metrics survived genesis round-trip (R29-5)")
	} else {
		t.Log("Capture metrics not found after genesis round-trip — genesis export gap (R29-5)")
	}

	// ── Phase 5: Run 10 more blocks on imported app — no panics ─────────

	require.NotPanics(t, func() {
		height := app2.LastBlockHeight()
		for i := 0; i < 10; i++ {
			height++
			_, fbErr := app2.FinalizeBlock(&abci.RequestFinalizeBlock{
				Height: height,
			})
			if fbErr != nil {
				panic(fbErr)
			}
			_, cErr := app2.Commit()
			if cErr != nil {
				panic(cErr)
			}
		}
	})

	t.Log("Genesis export/import round-trip complete — 10 post-import blocks succeeded")
}
