package keeper_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/keeper"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

func TestEpistemicState_KeyConstruction(t *testing.T) {
	key := types.EpistemicStateKey("mathematics")
	require.Equal(t, byte(0x53), key[0])
	require.Contains(t, string(key[1:]), "mathematics")
}

func TestEpistemicParams_Defaults(t *testing.T) {
	params := types.DefaultParams()
	require.Equal(t, uint64(995_000), params.EpistemicTemperatureDecayBps)
	require.Equal(t, uint64(50_000), params.EpistemicConformityCoolingBps)
	require.Equal(t, uint64(100_000), params.EpistemicVindicationHeatingBps)
	require.Equal(t, uint64(600_000), params.EpistemicColdConfidenceCapBps)
	require.Equal(t, uint64(1_500_000), params.EpistemicHotConfidenceGrowthBps)
	require.Equal(t, uint64(10_000), params.EpistemicTemperatureWindowBlocks)
}

func TestEpistemicState_SetGetRoundTrip(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	state := &types.DomainEpistemicState{
		Domain:                "mathematics",
		Temperature:           500_000,
		ConformityStreak:      3,
		VindicationCount:      2,
		LastTemperatureUpdate: 100,
	}
	require.NoError(t, k.SetDomainEpistemicState(ctx, state))

	got, found, err := k.GetDomainEpistemicState(ctx, "mathematics")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, uint64(500_000), got.Temperature)
	require.Equal(t, uint64(3), got.ConformityStreak)
	require.Equal(t, uint64(2), got.VindicationCount)
	require.Equal(t, uint64(100), got.LastTemperatureUpdate)
}

func TestEpistemicState_NotFound(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	_, found, err := k.GetDomainEpistemicState(ctx, "nonexistent")
	require.NoError(t, err)
	require.False(t, found)
}

func TestEpistemicState_GetOrInit(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// No existing state — should return neutral
	state, err := k.GetOrInitDomainEpistemicState(ctx, "new_domain")
	require.NoError(t, err)
	require.Equal(t, "new_domain", state.Domain)
	require.Equal(t, uint64(500_000), state.Temperature)

	// Set a state, then GetOrInit should return it
	require.NoError(t, k.SetDomainEpistemicState(ctx, &types.DomainEpistemicState{
		Domain:      "existing",
		Temperature: 800_000,
	}))
	state, err = k.GetOrInitDomainEpistemicState(ctx, "existing")
	require.NoError(t, err)
	require.Equal(t, uint64(800_000), state.Temperature)
}

func TestCountVindicationsInWindow(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Create facts in different domains using the existing makeTestFact helper
	makeTestFact(t, k, ctx, "f1", "fact one", "physics", "general", "zrn1submitter1", 700_000)
	makeTestFact(t, k, ctx, "f2", "fact two", "physics", "general", "zrn1submitter1", 700_000)
	makeTestFact(t, k, ctx, "f3", "fact three", "mathematics", "general", "zrn1submitter1", 700_000)

	// Add vindication records for physics domain facts
	require.NoError(t, k.SetVindicationRecord(ctx, "f1", types.VindicationRecord{
		Verifier: "v1", FactId: "f1", VindicatedAt: 5000,
	}))
	require.NoError(t, k.SetVindicationRecord(ctx, "f1", types.VindicationRecord{
		Verifier: "v2", FactId: "f1", VindicatedAt: 5000,
	}))
	require.NoError(t, k.SetVindicationRecord(ctx, "f2", types.VindicationRecord{
		Verifier: "v3", FactId: "f2", VindicatedAt: 9000,
	}))

	// Record for mathematics domain (should not count for physics)
	require.NoError(t, k.SetVindicationRecord(ctx, "f3", types.VindicationRecord{
		Verifier: "v4", FactId: "f3", VindicatedAt: 8000,
	}))

	// Count vindications for physics within window [0, 10000]
	count := k.CountVindicationsInWindow(ctx, "physics", 10000, 10000)
	require.Equal(t, uint64(2), count) // f1 and f2 are two distinct facts (events)

	// Narrower window that excludes f1's vindication
	count = k.CountVindicationsInWindow(ctx, "physics", 10000, 2000)
	require.Equal(t, uint64(1), count) // Only f2 at height 9000 within [8000, 10000]

	// Empty domain
	count = k.CountVindicationsInWindow(ctx, "logic", 10000, 10000)
	require.Equal(t, uint64(0), count)
}

func TestTemperatureCategory(t *testing.T) {
	tests := []struct {
		temp     uint64
		expected string
	}{
		{0, "cold"},
		{200_000, "cold"},
		{299_999, "cold"},
		{300_000, "cool"},
		{400_000, "cool"},
		{499_999, "cool"},
		{500_000, "neutral"},
		{600_000, "neutral"},
		{700_000, "neutral"},
		{700_001, "warm"},
		{750_000, "warm"},
		{799_999, "warm"},
		{800_000, "hot"},
		{1_000_000, "hot"},
	}
	for _, tt := range tests {
		require.Equal(t, tt.expected, keeper.TemperatureCategory(tt.temp), "temp=%d", tt.temp)
	}
}

func TestUpdateEpistemicTemperature_DecayToNeutral(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	ctx = advanceBlocks(ctx, 9_900) // height 100 + 9900 = 10,000 (first fitness epoch)

	// Start hot
	require.NoError(t, k.SetDomainEpistemicState(ctx, &types.DomainEpistemicState{
		Domain:      "physics",
		Temperature: 800_000,
	}))

	require.NoError(t, k.UpdateEpistemicTemperature(ctx, "physics"))

	state, found, err := k.GetDomainEpistemicState(ctx, "physics")
	require.NoError(t, err)
	require.True(t, found)
	// Decay: neutral + (800,000 - 500,000) * 995,000 / 1,000,000
	// = 500,000 + 298,500 = 798,500
	require.Equal(t, uint64(798_500), state.Temperature)
}

func TestUpdateEpistemicTemperature_ConformityCooling(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	ctx = advanceBlocks(ctx, 9_900) // height = 10,000, epoch 1

	require.NoError(t, k.SetDomainEpistemicState(ctx, &types.DomainEpistemicState{
		Domain:      "physics",
		Temperature: 500_000, // neutral
	}))

	// Create low-diversity epoch data
	require.NoError(t, k.SetDomainDiversity(ctx, "physics", 1, keeper.DomainDiversityRecord{
		Domain:     "physics",
		Epoch:      1,
		AvgEntropy: 10_000, // Very low (below 50,000 threshold)
		RoundCount: 5,
	}))

	require.NoError(t, k.UpdateEpistemicTemperature(ctx, "physics"))

	state, found, err := k.GetDomainEpistemicState(ctx, "physics")
	require.NoError(t, err)
	require.True(t, found)
	require.Less(t, state.Temperature, uint64(500_000)) // Cooled below neutral
	require.Equal(t, uint64(1), state.ConformityStreak)
}

func TestUpdateEpistemicTemperature_NewDomainStartsNeutral(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	ctx = advanceBlocks(ctx, 9_900) // height = 10,000

	require.NoError(t, k.UpdateEpistemicTemperature(ctx, "new_domain"))

	state, found, err := k.GetDomainEpistemicState(ctx, "new_domain")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, uint64(500_000), state.Temperature) // neutral, no decay for neutral
}

func TestClampConfidence_ColdDomainCap(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Set cold temperature for physics
	require.NoError(t, k.SetDomainEpistemicState(ctx, &types.DomainEpistemicState{
		Domain:      "physics",
		Temperature: 200_000, // Cold (< 300,000)
	}))

	// Default MaxConfidence=880,000, but cold cap=600,000
	clamped := k.ClampConfidence(ctx, 750_000, "physics")
	require.Equal(t, uint64(600_000), clamped)
}

func TestClampConfidence_HotDomainAllowsHigher(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Set very hot temperature (> 800,000)
	require.NoError(t, k.SetDomainEpistemicState(ctx, &types.DomainEpistemicState{
		Domain:      "physics",
		Temperature: 850_000,
	}))

	// Hot domains: SurvivedChallengeConfidenceCap=880,000 — same as MaxConfidence
	// Value 860,000 <= 880,000, should pass through
	clamped := k.ClampConfidence(ctx, 860_000, "physics")
	require.Equal(t, uint64(860_000), clamped)
}

func TestClampConfidence_NeutralDomainUnchanged(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// No epistemic state set — falls through to global cap
	// Default MaxConfidence is 880,000, so 750,000 passes through
	clamped := k.ClampConfidence(ctx, 750_000, "physics")
	require.Equal(t, uint64(750_000), clamped)
}

func TestBeginBlocker_UpdatesTemperatureAtFitnessEpoch(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Set a domain with hot temperature ("physics" exists via DefaultGenesis domains)
	require.NoError(t, k.SetDomainEpistemicState(ctx, &types.DomainEpistemicState{
		Domain:      "physics",
		Temperature: 800_000,
	}))

	// Advance to a fitness epoch boundary (default FitnessEpochBlocks = 10,000)
	// setupKnowledgeTest starts at height 100, so +9,900 = 10,000
	ctx = advanceBlocks(ctx, 9_900)

	err := k.BeginBlocker(ctx)
	require.NoError(t, err)

	// Temperature should have been updated (decayed toward neutral)
	state, found, err := k.GetDomainEpistemicState(ctx, "physics")
	require.NoError(t, err)
	require.True(t, found)
	require.Less(t, state.Temperature, uint64(800_000))
}

func TestAdvanceConfidence_NeutralDomain(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Create a verified fact
	makeTestFact(t, k, ctx, "f1", "verified fact", "physics", "general", "zrn1submitter1", 500_000)
	fact, _ := k.GetFact(ctx, "f1")
	fact.Status = types.FactStatus_FACT_STATUS_VERIFIED
	require.NoError(t, k.SetFact(ctx, fact))

	// No epistemic state → neutral → normal growth
	require.NoError(t, k.AdvanceConfidence(ctx))

	updated, _ := k.GetFact(ctx, "f1")
	// Default growth: 11,000 BPS (1.1%) of 500,000 = 5,500
	// New confidence = 500,000 + 5,500 = 505,500
	require.Equal(t, uint64(505_500), updated.Confidence)
}

func TestAdvanceConfidence_HotDomainFasterGrowth(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	makeTestFact(t, k, ctx, "f1", "verified fact", "physics", "general", "zrn1submitter1", 500_000)
	fact, _ := k.GetFact(ctx, "f1")
	fact.Status = types.FactStatus_FACT_STATUS_VERIFIED
	require.NoError(t, k.SetFact(ctx, fact))

	// Set hot temperature
	require.NoError(t, k.SetDomainEpistemicState(ctx, &types.DomainEpistemicState{
		Domain:      "physics",
		Temperature: 800_000, // Hot (> 700,000)
	}))

	require.NoError(t, k.AdvanceConfidence(ctx))

	updated, _ := k.GetFact(ctx, "f1")
	// Hot growth: 11,000 * 1,500,000 / 1,000,000 = 16,500 BPS
	// Growth amount: 500,000 * 16,500 / 1,000,000 = 8,250
	// New confidence = 500,000 + 8,250 = 508,250
	require.Equal(t, uint64(508_250), updated.Confidence)
}

func TestAdvanceConfidence_ColdDomainSlowerGrowth(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	makeTestFact(t, k, ctx, "f1", "verified fact", "physics", "general", "zrn1submitter1", 500_000)
	fact, _ := k.GetFact(ctx, "f1")
	fact.Status = types.FactStatus_FACT_STATUS_VERIFIED
	require.NoError(t, k.SetFact(ctx, fact))

	// Set cold temperature
	require.NoError(t, k.SetDomainEpistemicState(ctx, &types.DomainEpistemicState{
		Domain:      "physics",
		Temperature: 200_000, // Cold (< 300,000)
	}))

	require.NoError(t, k.AdvanceConfidence(ctx))

	updated, _ := k.GetFact(ctx, "f1")
	// Cold growth: 11,000 * 500,000 / 1,000,000 = 5,500 BPS (50% rate)
	// Growth amount: 500,000 * 5,500 / 1,000,000 = 2,750
	// New confidence = 500,000 + 2,750 = 502,750
	require.Equal(t, uint64(502_750), updated.Confidence)
}

func TestAdvanceConfidence_SkipsNonActiveFacts(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Create a fact (defaults to VERIFIED), then change to EXPIRED
	makeTestFact(t, k, ctx, "f1", "rejected fact", "physics", "general", "zrn1submitter1", 500_000)
	fact, _ := k.GetFact(ctx, "f1")
	fact.Status = types.FactStatus_FACT_STATUS_EXPIRED
	require.NoError(t, k.SetFact(ctx, fact))

	require.NoError(t, k.AdvanceConfidence(ctx))

	// Should not have changed
	updated, _ := k.GetFact(ctx, "f1")
	require.Equal(t, uint64(500_000), updated.Confidence)
}

func TestEpistemicTemperature_FullCycle(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	domain := "physics"

	// 1. New domain starts at neutral (500,000)
	state, err := k.GetOrInitDomainEpistemicState(ctx, domain)
	require.NoError(t, err)
	require.Equal(t, uint64(500_000), state.Temperature)

	// 2. Simulate conformity cooling over 10 epochs.
	//    Each epoch: decay toward neutral + conformity cooling (streak-scaled).
	//    After 10 epochs with low diversity the temperature should drop well below 300,000.
	params, err := k.GetParams(ctx)
	require.NoError(t, err)
	epochBlocks := int64(params.FitnessEpochBlocks) // 10,000

	// Advance from starting height 100 to the first epoch boundary (height 10,000).
	ctx = advanceBlocks(ctx, epochBlocks-100)

	for i := uint64(1); i <= 10; i++ {
		if i > 1 {
			ctx = advanceBlocks(ctx, epochBlocks)
		}

		sdkCtx := sdk.UnwrapSDKContext(ctx)
		epoch := uint64(sdkCtx.BlockHeight()) / params.FitnessEpochBlocks

		// Record low-diversity epoch data (very low entropy triggers conformity cooling)
		require.NoError(t, k.SetDomainDiversity(ctx, domain, epoch, keeper.DomainDiversityRecord{
			Domain:     domain,
			Epoch:      epoch,
			AvgEntropy: 10_000, // Well below 50,000 conformity threshold
			RoundCount: 3,
		}))

		require.NoError(t, k.UpdateEpistemicTemperature(ctx, domain))
	}

	state, _, err = k.GetDomainEpistemicState(ctx, domain)
	require.NoError(t, err)
	require.Less(t, state.Temperature, uint64(300_000), "Should be cold after 10 conformity epochs")
	t.Logf("After 10 conformity epochs: temperature=%d, category=%s",
		state.Temperature, keeper.TemperatureCategory(state.Temperature))

	// 3. Cold domain: confidence capped at 600,000 (EpistemicColdConfidenceCapBps)
	capped := k.ClampConfidence(ctx, 750_000, domain)
	require.Equal(t, uint64(600_000), capped)

	// 4. Simulate a vindication event — disproven fact heats the domain.
	makeTestFact(t, k, ctx, "f-vind", "disproven fact", domain, "general", "zrn1submitter1", 700_000)
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	vindHeight := uint64(sdkCtx.BlockHeight())
	require.NoError(t, k.SetVindicationRecord(ctx, "f-vind", types.VindicationRecord{
		Verifier:     "v1",
		FactId:       "f-vind",
		VindicatedAt: vindHeight,
		DisprovenBy:  "f-new",
	}))

	// Record the temperature before vindication update for comparison
	tempBeforeVindication := state.Temperature

	// Advance to next epoch boundary and update with healthy diversity
	ctx = advanceBlocks(ctx, epochBlocks)
	sdkCtx = sdk.UnwrapSDKContext(ctx)
	nextEpoch := uint64(sdkCtx.BlockHeight()) / params.FitnessEpochBlocks

	// Healthy diversity data prevents further conformity cooling
	require.NoError(t, k.SetDomainDiversity(ctx, domain, nextEpoch, keeper.DomainDiversityRecord{
		Domain:     domain,
		Epoch:      nextEpoch,
		AvgEntropy: 500_000, // Healthy diversity
		RoundCount: 3,
	}))
	require.NoError(t, k.UpdateEpistemicTemperature(ctx, domain))

	state, _, err = k.GetDomainEpistemicState(ctx, domain)
	require.NoError(t, err)
	t.Logf("After vindication: temperature=%d (was %d), category=%s",
		state.Temperature, tempBeforeVindication, keeper.TemperatureCategory(state.Temperature))
	// Temperature should have risen due to vindication heating (+100,000 per new vindication)
	// combined with decay toward neutral (which also pushes up from cold).
	require.Greater(t, state.Temperature, tempBeforeVindication,
		"Vindication should heat a cold domain")

	// 5. After many quiet epochs with healthy diversity, temperature drifts back to neutral (500,000).
	//    Decay rate is 0.5% per epoch (995,000/1,000,000), so deviation halves roughly every
	//    139 epochs. 500 epochs should reduce any deviation to negligible levels.
	for i := 0; i < 500; i++ {
		ctx = advanceBlocks(ctx, epochBlocks)
		sdkCtx2 := sdk.UnwrapSDKContext(ctx)
		ep := uint64(sdkCtx2.BlockHeight()) / params.FitnessEpochBlocks
		require.NoError(t, k.SetDomainDiversity(ctx, domain, ep, keeper.DomainDiversityRecord{
			Domain:     domain,
			Epoch:      ep,
			AvgEntropy: 500_000, // Healthy
			RoundCount: 3,
		}))
		require.NoError(t, k.UpdateEpistemicTemperature(ctx, domain))
	}

	state, _, err = k.GetDomainEpistemicState(ctx, domain)
	require.NoError(t, err)
	t.Logf("After 500 quiet epochs: temperature=%d", state.Temperature)
	diff := int64(state.Temperature) - 500_000
	if diff < 0 {
		diff = -diff
	}
	require.Less(t, diff, int64(15_000),
		"Temperature should be near neutral after many quiet epochs")
}
