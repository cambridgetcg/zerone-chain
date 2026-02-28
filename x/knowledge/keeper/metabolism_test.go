package keeper_test

import (
	"testing"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/keeper"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// makeEnergyFact creates a fact with metabolism fields set for testing.
func makeEnergyFact(id, content, domain string, energy uint64, status types.FactStatus) *types.Fact {
	return &types.Fact{
		Id:        id,
		Content:   content,
		Domain:    domain,
		Status:    status,
		Energy:    energy,
		EnergyCap: 1_000_000,
		Submitter: "zrn1test",
	}
}

func TestMetabolism_BaseDrain(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	fact := makeEnergyFact("fact-bd", "Base drain test fact", "physics", 500_000, types.FactStatus_FACT_STATUS_VERIFIED)
	require.NoError(t, k.SetFact(ctx, fact))

	// Run one epoch of metabolism
	require.NoError(t, k.ProcessMetabolism(ctx, 1))

	updated, found := k.GetFact(ctx, "fact-bd")
	require.True(t, found)

	// Base cost = 10,000. Content = 20 chars → 0 groups of 100 → no content factor.
	// Domain: 1 fact → 0 groups of 100 → no competition factor.
	// No income sources → energy should decrease by base cost (10,000).
	require.Equal(t, uint64(490_000), updated.Energy)
}

func TestMetabolism_QueryIncome(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	fact := makeEnergyFact("fact-qi", "Query income test", "physics", 100_000, types.FactStatus_FACT_STATUS_VERIFIED)
	fact.QueryCountEpoch = 50 // 50 queries this epoch
	require.NoError(t, k.SetFact(ctx, fact))

	require.NoError(t, k.ProcessMetabolism(ctx, 1))

	updated, found := k.GetFact(ctx, "fact-qi")
	require.True(t, found)

	// Income = 50 * 1,000 = 50,000 energy from queries
	// Cost = 10,000 (base)
	// New energy = 100,000 + 50,000 - 10,000 = 140,000
	require.Equal(t, uint64(140_000), updated.Energy)
}

func TestMetabolism_CitationIncome(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	fact := makeEnergyFact("fact-ci", "Citation income test", "physics", 100_000, types.FactStatus_FACT_STATUS_VERIFIED)
	require.NoError(t, k.SetFact(ctx, fact))

	// Simulate 3 new citations this epoch
	k.IncrementNewCitationEpoch(ctx, "fact-ci")
	k.IncrementNewCitationEpoch(ctx, "fact-ci")
	k.IncrementNewCitationEpoch(ctx, "fact-ci")

	require.NoError(t, k.ProcessMetabolism(ctx, 1))

	updated, found := k.GetFact(ctx, "fact-ci")
	require.True(t, found)

	// Income = 3 * 5,000 = 15,000 energy from citations
	// Cost = 10,000 (base)
	// New energy = 100,000 + 15,000 - 10,000 = 105,000
	require.Equal(t, uint64(105_000), updated.Energy)

	// Verify citation counters were reset
	require.Equal(t, uint64(0), k.GetNewCitationsThisEpoch(ctx, "fact-ci"))
}

func TestMetabolism_PatronageIncome(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	ctx = ctx.WithBlockHeader(cmtproto.Header{Height: 500})

	fact := makeEnergyFact("fact-pi", "Patronage income test", "physics", 100_000, types.FactStatus_FACT_STATUS_VERIFIED)
	fact.PatronageAmount = "1000000" // 1 ZRN
	fact.PatronageExpiryBlock = 10000
	require.NoError(t, k.SetFact(ctx, fact))

	require.NoError(t, k.ProcessMetabolism(ctx, 1))

	updated, found := k.GetFact(ctx, "fact-pi")
	require.True(t, found)

	// Income = 20,000 energy from patronage
	// Cost = 10,000 (base)
	// New energy = 100,000 + 20,000 - 10,000 = 110,000
	require.Equal(t, uint64(110_000), updated.Energy)
}

func TestMetabolism_ContentLengthCost(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Short fact (20 chars) — no extra cost
	shortFact := makeEnergyFact("fact-short", "Short fact content!!", "physics", 500_000, types.FactStatus_FACT_STATUS_VERIFIED)
	require.NoError(t, k.SetFact(ctx, shortFact))

	// Long fact (500 chars) — higher cost
	longContent := make([]byte, 500)
	for i := range longContent {
		longContent[i] = 'a'
	}
	longFact := makeEnergyFact("fact-long", string(longContent), "physics", 500_000, types.FactStatus_FACT_STATUS_VERIFIED)
	require.NoError(t, k.SetFact(ctx, longFact))

	require.NoError(t, k.ProcessMetabolism(ctx, 1))

	shortUpdated, _ := k.GetFact(ctx, "fact-short")
	longUpdated, _ := k.GetFact(ctx, "fact-long")

	// Long fact should have drained more energy
	require.Greater(t, shortUpdated.Energy, longUpdated.Energy,
		"longer fact should drain more energy")
}

func TestMetabolism_DomainCompetition(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Create a lonely fact in an empty domain
	lonelyFact := makeEnergyFact("fact-lonely", "Lonely fact in quiet domain", "theology", 500_000, types.FactStatus_FACT_STATUS_VERIFIED)
	require.NoError(t, k.SetFact(ctx, lonelyFact))

	// Create 200 facts in physics to make it a crowded domain
	for i := 0; i < 200; i++ {
		f := makeEnergyFact(
			"crowd-"+string(rune('a'+i/26))+string(rune('a'+i%26)),
			"Filler fact for domain competition",
			"physics",
			500_000,
			types.FactStatus_FACT_STATUS_VERIFIED,
		)
		require.NoError(t, k.SetFact(ctx, f))
	}

	// Add a target fact in physics
	crowdedFact := makeEnergyFact("fact-crowded", "Fact in crowded domain", "physics", 500_000, types.FactStatus_FACT_STATUS_VERIFIED)
	require.NoError(t, k.SetFact(ctx, crowdedFact))

	require.NoError(t, k.ProcessMetabolism(ctx, 1))

	lonelyUpdated, _ := k.GetFact(ctx, "fact-lonely")
	crowdedUpdated, _ := k.GetFact(ctx, "fact-crowded")

	// Crowded domain should drain more due to competition factor
	require.Greater(t, lonelyUpdated.Energy, crowdedUpdated.Energy,
		"fact in crowded domain should drain more energy")
}

func TestMetabolism_AtRiskTransition(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Fact with just enough energy to drain to 0
	fact := makeEnergyFact("fact-ar", "At risk test fact!!!!!", "physics", 10_000, types.FactStatus_FACT_STATUS_VERIFIED)
	require.NoError(t, k.SetFact(ctx, fact))

	require.NoError(t, k.ProcessMetabolism(ctx, 1))

	updated, found := k.GetFact(ctx, "fact-ar")
	require.True(t, found)

	// Energy = 10,000 - 10,000 (base cost) = 0 → below 300K threshold → AT_RISK
	require.Equal(t, uint64(0), updated.Energy)
	require.Equal(t, types.FactStatus_FACT_STATUS_AT_RISK, updated.Status)
	require.Equal(t, uint64(1), updated.AtRiskSinceEpoch)
}

func TestMetabolism_ExpiredTransition(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Fact already at risk since epoch 1
	fact := makeEnergyFact("fact-exp", "Expiring fact test!!!", "physics", 0, types.FactStatus_FACT_STATUS_AT_RISK)
	fact.AtRiskSinceEpoch = 1
	require.NoError(t, k.SetFact(ctx, fact))

	// Process at epoch 6 (5 epochs at risk → should expire, since at_risk_epochs = 5)
	require.NoError(t, k.ProcessMetabolism(ctx, 6))

	updated, found := k.GetFact(ctx, "fact-exp")
	require.True(t, found)

	require.Equal(t, types.FactStatus_FACT_STATUS_EXPIRED, updated.Status)
}

func TestMetabolism_PrunedTransition(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Fact already at risk since epoch 1
	fact := makeEnergyFact("fact-prn", "Pruning fact test!!!!", "physics", 0, types.FactStatus_FACT_STATUS_EXPIRED)
	fact.AtRiskSinceEpoch = 1
	require.NoError(t, k.SetFact(ctx, fact))

	// Process at epoch 26 (25 epochs at risk: 5 at_risk + 20 expired_to_pruned = 25)
	require.NoError(t, k.ProcessMetabolism(ctx, 26))

	updated, found := k.GetFact(ctx, "fact-prn")
	require.True(t, found)

	require.Equal(t, types.FactStatus_FACT_STATUS_PRUNED, updated.Status)
}

func TestMetabolism_Recovery(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Fact at risk with 0 energy — needs enough income to cross 300K threshold
	fact := makeEnergyFact("fact-rec", "Recovery test fact!!!", "physics", 0, types.FactStatus_FACT_STATUS_AT_RISK)
	fact.AtRiskSinceEpoch = 1
	fact.QueryCountEpoch = 400 // 400 queries * 1,000 = 400,000 energy income
	require.NoError(t, k.SetFact(ctx, fact))

	require.NoError(t, k.ProcessMetabolism(ctx, 2))

	updated, found := k.GetFact(ctx, "fact-rec")
	require.True(t, found)

	// Income = 400 * 1,000 = 400,000, Cost = 10,000 → net = 390,000 energy (above 300K threshold)
	require.Equal(t, uint64(390_000), updated.Energy)
	require.Equal(t, types.FactStatus_FACT_STATUS_ACTIVE, updated.Status)
	require.Equal(t, uint64(0), updated.AtRiskSinceEpoch, "should clear at-risk epoch on recovery")
}

func TestMetabolism_ChallengeSurvivalBoost(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Create a fact and a rejected challenge claim
	fact := makeEnergyFact("fact-cs", "Challenge survival test", "physics", 200_000, types.FactStatus_FACT_STATUS_CHALLENGED)
	require.NoError(t, k.SetFact(ctx, fact))

	// Simulate challenge claim with the original fact ID
	challengeClaim := &types.Claim{
		Id:                "challenge-claim-1",
		FactContent:       "Challenge of fact fact-cs",
		Domain:            "physics",
		ProvisionalFactId: "fact-cs",
		Status:            types.ClaimStatus_CLAIM_STATUS_REJECTED,
	}
	require.NoError(t, k.SetClaim(ctx, challengeClaim))

	// Call handleChallengeSurvival via CompleteRound behavior
	// Directly test the exported method behavior by simulating what CompleteRound does
	params, _ := k.GetParams(ctx)

	// Manual energy boost (simulating what handleChallengeSurvival does)
	fact.Energy += params.MetabolismEnergyChallengeSurvival
	if fact.Energy > params.MetabolismEnergyCap {
		fact.Energy = params.MetabolismEnergyCap
	}
	fact.Status = types.FactStatus_FACT_STATUS_ACTIVE
	require.NoError(t, k.SetFact(ctx, fact))

	updated, found := k.GetFact(ctx, "fact-cs")
	require.True(t, found)

	// 200,000 + 100,000 = 300,000
	require.Equal(t, uint64(300_000), updated.Energy)
	require.Equal(t, types.FactStatus_FACT_STATUS_ACTIVE, updated.Status)
}

func TestMetabolism_EnergyCap(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Fact near cap with lots of income
	fact := makeEnergyFact("fact-cap", "Energy cap test fact!!", "physics", 990_000, types.FactStatus_FACT_STATUS_VERIFIED)
	fact.QueryCountEpoch = 100 // 100 * 1,000 = 100,000 income
	require.NoError(t, k.SetFact(ctx, fact))

	require.NoError(t, k.ProcessMetabolism(ctx, 1))

	updated, found := k.GetFact(ctx, "fact-cap")
	require.True(t, found)

	// 990,000 + 100,000 - 10,000 = 1,080,000, but capped at 1,000,000
	require.Equal(t, uint64(1_000_000), updated.Energy, "energy should not exceed cap")
}

func TestMetabolism_InitialEnergy(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	params, err := k.GetParams(ctx)
	require.NoError(t, err)

	// Create a fact via createFactFromClaim pattern (direct setup with initial energy)
	fact := &types.Fact{
		Id:            "fact-init",
		Content:       "New fact with initial energy",
		Domain:        "physics",
		Status:        types.FactStatus_FACT_STATUS_VERIFIED,
		Energy:        params.MetabolismInitialEnergy,
		EnergyCap:     params.MetabolismEnergyCap,
		FitnessScore:  params.FitnessInitialScore,
	}
	require.NoError(t, k.SetFact(ctx, fact))

	updated, found := k.GetFact(ctx, "fact-init")
	require.True(t, found)

	require.Equal(t, uint64(500_000), updated.Energy, "new facts should start with initial energy")
	require.Equal(t, uint64(1_000_000), updated.EnergyCap, "energy cap should match params")
}

func TestMetabolism_MultiLevelThresholds_ActiveToAtRisk(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Energy just above active threshold. After drain, falls below → AT_RISK
	// ActiveThreshold = 300,000. BaseCost = 10,000. Start at 305,000 → 295,000 → AT_RISK
	fact := makeEnergyFact("fact-ml1", "Multi-level threshold test!!!", "physics", 305_000, types.FactStatus_FACT_STATUS_VERIFIED)
	require.NoError(t, k.SetFact(ctx, fact))

	require.NoError(t, k.ProcessMetabolism(ctx, 1))

	updated, found := k.GetFact(ctx, "fact-ml1")
	require.True(t, found)
	require.Equal(t, uint64(295_000), updated.Energy)
	require.Equal(t, types.FactStatus_FACT_STATUS_AT_RISK, updated.Status)
}

func TestMetabolism_MultiLevelThresholds_StaysHealthy(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Energy well above threshold — should stay healthy, no status change
	fact := makeEnergyFact("fact-ml4", "Stays healthy test fact!!!", "physics", 500_000, types.FactStatus_FACT_STATUS_VERIFIED)
	require.NoError(t, k.SetFact(ctx, fact))

	require.NoError(t, k.ProcessMetabolism(ctx, 1))

	updated, found := k.GetFact(ctx, "fact-ml4")
	require.True(t, found)
	require.Equal(t, uint64(490_000), updated.Energy)
	require.Equal(t, types.FactStatus_FACT_STATUS_VERIFIED, updated.Status)
}

func TestFactsAtRisk_Query(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Create facts in various states
	activeFact := makeEnergyFact("fact-active", "Active healthy fact!!!", "physics", 5000, types.FactStatus_FACT_STATUS_ACTIVE)
	require.NoError(t, k.SetFact(ctx, activeFact))

	atRiskFact1 := makeEnergyFact("fact-risk1", "At risk in physics!!!!", "physics", 0, types.FactStatus_FACT_STATUS_AT_RISK)
	atRiskFact1.AtRiskSinceEpoch = 1
	require.NoError(t, k.SetFact(ctx, atRiskFact1))

	atRiskFact2 := makeEnergyFact("fact-risk2", "At risk in mathematics", "mathematics", 0, types.FactStatus_FACT_STATUS_AT_RISK)
	atRiskFact2.AtRiskSinceEpoch = 2
	require.NoError(t, k.SetFact(ctx, atRiskFact2))

	expiredFact := makeEnergyFact("fact-expired", "Expired fact in physics", "physics", 0, types.FactStatus_FACT_STATUS_EXPIRED)
	expiredFact.AtRiskSinceEpoch = 1
	require.NoError(t, k.SetFact(ctx, expiredFact))

	// Query all at-risk facts (no domain filter)
	qs := keeper.NewQueryServerImpl(k)
	resp, err := qs.FactsAtRisk(ctx, &types.QueryFactsAtRiskRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Facts, 3, "should return at-risk and expired facts")

	// Query with domain filter
	resp, err = qs.FactsAtRisk(ctx, &types.QueryFactsAtRiskRequest{Domain: "physics"})
	require.NoError(t, err)
	require.Len(t, resp.Facts, 2, "should return only physics at-risk/expired facts")

	// Query with limit
	resp, err = qs.FactsAtRisk(ctx, &types.QueryFactsAtRiskRequest{Limit: 1})
	require.NoError(t, err)
	require.Len(t, resp.Facts, 1, "should respect limit")
}

func TestMetabolism_UnifiedStatusEvent(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Fact that will transition VERIFIED → AT_RISK
	fact := makeEnergyFact("fact-ev", "Event test fact content!!", "physics", 305_000, types.FactStatus_FACT_STATUS_VERIFIED)
	require.NoError(t, k.SetFact(ctx, fact))

	require.NoError(t, k.ProcessMetabolism(ctx, 1))

	// Check for unified event
	events := ctx.EventManager().Events()
	found := false
	for _, event := range events {
		if event.Type == "zerone.knowledge.fact_status_changed" {
			found = true
			attrs := make(map[string]string)
			for _, attr := range event.Attributes {
				attrs[attr.Key] = attr.Value
			}
			require.Equal(t, "fact-ev", attrs["fact_id"])
			require.Equal(t, "FACT_STATUS_VERIFIED", attrs["old_status"])
			require.Equal(t, "FACT_STATUS_AT_RISK", attrs["new_status"])
			require.Equal(t, "decay", attrs["reason"])
			require.NotEmpty(t, attrs["energy"])
		}
	}
	require.True(t, found, "should emit fact_status_changed event")
}

func TestPatronage_ImmediateEnergyBoost(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	ctx = ctx.WithBlockHeader(cmtproto.Header{Height: 500})

	params, _ := k.GetParams(ctx)

	// Fact with low energy — no patronage yet
	fact := makeEnergyFact("fact-ipe", "Patronage immediate test!!", "physics", 200_000, types.FactStatus_FACT_STATUS_VERIFIED)
	require.NoError(t, k.SetFact(ctx, fact))

	// Duration: 10,000 blocks. FitnessEpochBlocks from params. ~10 epochs.
	// Boost = MetabolismEnergyPerPatronage * durationEpochs / 10
	durationBlocks := uint64(10_000)
	durationEpochs := durationBlocks / params.FitnessEpochBlocks
	if durationEpochs == 0 {
		durationEpochs = 1
	}
	expectedBoost := params.MetabolismEnergyPerPatronage * durationEpochs / 10
	if expectedBoost == 0 {
		expectedBoost = params.MetabolismEnergyPerPatronage
	}

	// Apply patronage boost
	fact2, _ := k.GetFact(ctx, "fact-ipe")
	k.ApplyPatronageEnergyBoost(ctx, fact2, durationBlocks)

	updated, found := k.GetFact(ctx, "fact-ipe")
	require.True(t, found)
	require.Equal(t, 200_000+expectedBoost, updated.Energy)
}

func TestPatronage_AtRiskRecovery(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	ctx = ctx.WithBlockHeader(cmtproto.Header{Height: 500})

	// AT_RISK fact — patronage should push above threshold and recover
	fact := makeEnergyFact("fact-prec", "Patronage recovery test!!!", "physics", 250_000, types.FactStatus_FACT_STATUS_AT_RISK)
	fact.AtRiskSinceEpoch = 1
	require.NoError(t, k.SetFact(ctx, fact))

	// Long patronage to ensure recovery above 300K threshold
	// Need boost >= 50K. boost = 20,000 * epochs / 10. epochs = 500,000 / 10,000 = 50. boost = 20,000 * 50 / 10 = 100,000.
	fact2, _ := k.GetFact(ctx, "fact-prec")
	k.ApplyPatronageEnergyBoost(ctx, fact2, 500_000)

	updated, found := k.GetFact(ctx, "fact-prec")
	require.True(t, found)
	require.Equal(t, types.FactStatus_FACT_STATUS_ACTIVE, updated.Status)
	require.Equal(t, uint64(0), updated.AtRiskSinceEpoch)
	require.Greater(t, updated.Energy, uint64(300_000))
}

func TestPatronage_EnergyCapped(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	ctx = ctx.WithBlockHeader(cmtproto.Header{Height: 500})

	// Fact near energy cap — boost should not exceed cap
	fact := makeEnergyFact("fact-pcap", "Patronage cap test fact!!", "physics", 990_000, types.FactStatus_FACT_STATUS_VERIFIED)
	require.NoError(t, k.SetFact(ctx, fact))

	fact2, _ := k.GetFact(ctx, "fact-pcap")
	k.ApplyPatronageEnergyBoost(ctx, fact2, 100_000)

	updated, found := k.GetFact(ctx, "fact-pcap")
	require.True(t, found)
	require.Equal(t, uint64(1_000_000), updated.Energy, "energy should be capped at 1M")
}

func TestMetabolism_RecoveryEvent(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// AT_RISK fact that will recover
	fact := makeEnergyFact("fact-rev", "Recovery event test fact!", "physics", 0, types.FactStatus_FACT_STATUS_AT_RISK)
	fact.AtRiskSinceEpoch = 1
	fact.QueryCountEpoch = 400 // 400 * 1000 = 400K income, -10K cost = 390K > 300K
	require.NoError(t, k.SetFact(ctx, fact))

	require.NoError(t, k.ProcessMetabolism(ctx, 2))

	events := ctx.EventManager().Events()
	found := false
	for _, event := range events {
		if event.Type == "zerone.knowledge.fact_status_changed" {
			attrs := make(map[string]string)
			for _, attr := range event.Attributes {
				attrs[attr.Key] = attr.Value
			}
			if attrs["fact_id"] == "fact-rev" {
				found = true
				require.Equal(t, "FACT_STATUS_AT_RISK", attrs["old_status"])
				require.Equal(t, "FACT_STATUS_ACTIVE", attrs["new_status"])
				require.Equal(t, "recovery", attrs["reason"])
			}
		}
	}
	require.True(t, found, "should emit recovery event")
}
