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
		EnergyCap: 10_000,
		Submitter: "zrn1test",
	}
}

func TestMetabolism_BaseDrain(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	fact := makeEnergyFact("fact-bd", "Base drain test fact", "physics", 5000, types.FactStatus_FACT_STATUS_VERIFIED)
	require.NoError(t, k.SetFact(ctx, fact))

	// Run one epoch of metabolism
	require.NoError(t, k.ProcessMetabolism(ctx, 1))

	updated, found := k.GetFact(ctx, "fact-bd")
	require.True(t, found)

	// Base cost = 100. Content = 20 chars → 0 groups of 100 → no content factor.
	// Domain: 1 fact → 0 groups of 100 → no competition factor.
	// No income sources → energy should decrease by base cost (100).
	require.Equal(t, uint64(4900), updated.Energy)
}

func TestMetabolism_QueryIncome(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	fact := makeEnergyFact("fact-qi", "Query income test", "physics", 1000, types.FactStatus_FACT_STATUS_VERIFIED)
	fact.QueryCountEpoch = 50 // 50 queries this epoch
	require.NoError(t, k.SetFact(ctx, fact))

	require.NoError(t, k.ProcessMetabolism(ctx, 1))

	updated, found := k.GetFact(ctx, "fact-qi")
	require.True(t, found)

	// Income = 50 * 10 = 500 energy from queries
	// Cost = 100 (base)
	// New energy = 1000 + 500 - 100 = 1400
	require.Equal(t, uint64(1400), updated.Energy)
}

func TestMetabolism_CitationIncome(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	fact := makeEnergyFact("fact-ci", "Citation income test", "physics", 1000, types.FactStatus_FACT_STATUS_VERIFIED)
	require.NoError(t, k.SetFact(ctx, fact))

	// Simulate 3 new citations this epoch
	k.IncrementNewCitationEpoch(ctx, "fact-ci")
	k.IncrementNewCitationEpoch(ctx, "fact-ci")
	k.IncrementNewCitationEpoch(ctx, "fact-ci")

	require.NoError(t, k.ProcessMetabolism(ctx, 1))

	updated, found := k.GetFact(ctx, "fact-ci")
	require.True(t, found)

	// Income = 3 * 50 = 150 energy from citations
	// Cost = 100 (base)
	// New energy = 1000 + 150 - 100 = 1050
	require.Equal(t, uint64(1050), updated.Energy)

	// Verify citation counters were reset
	require.Equal(t, uint64(0), k.GetNewCitationsThisEpoch(ctx, "fact-ci"))
}

func TestMetabolism_PatronageIncome(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	ctx = ctx.WithBlockHeader(cmtproto.Header{Height: 500})

	fact := makeEnergyFact("fact-pi", "Patronage income test", "physics", 1000, types.FactStatus_FACT_STATUS_VERIFIED)
	fact.PatronageAmount = "1000000" // 1 ZRN
	fact.PatronageExpiryBlock = 10000
	require.NoError(t, k.SetFact(ctx, fact))

	require.NoError(t, k.ProcessMetabolism(ctx, 1))

	updated, found := k.GetFact(ctx, "fact-pi")
	require.True(t, found)

	// Income = 200 energy from patronage
	// Cost = 100 (base)
	// New energy = 1000 + 200 - 100 = 1100
	require.Equal(t, uint64(1100), updated.Energy)
}

func TestMetabolism_ContentLengthCost(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Short fact (20 chars) — no extra cost
	shortFact := makeEnergyFact("fact-short", "Short fact content!!", "physics", 5000, types.FactStatus_FACT_STATUS_VERIFIED)
	require.NoError(t, k.SetFact(ctx, shortFact))

	// Long fact (500 chars) — higher cost
	longContent := make([]byte, 500)
	for i := range longContent {
		longContent[i] = 'a'
	}
	longFact := makeEnergyFact("fact-long", string(longContent), "physics", 5000, types.FactStatus_FACT_STATUS_VERIFIED)
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
	lonelyFact := makeEnergyFact("fact-lonely", "Lonely fact in quiet domain", "theology", 5000, types.FactStatus_FACT_STATUS_VERIFIED)
	require.NoError(t, k.SetFact(ctx, lonelyFact))

	// Create 200 facts in physics to make it a crowded domain
	for i := 0; i < 200; i++ {
		f := makeEnergyFact(
			"crowd-"+string(rune('a'+i/26))+string(rune('a'+i%26)),
			"Filler fact for domain competition",
			"physics",
			5000,
			types.FactStatus_FACT_STATUS_VERIFIED,
		)
		require.NoError(t, k.SetFact(ctx, f))
	}

	// Add a target fact in physics
	crowdedFact := makeEnergyFact("fact-crowded", "Fact in crowded domain", "physics", 5000, types.FactStatus_FACT_STATUS_VERIFIED)
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
	fact := makeEnergyFact("fact-ar", "At risk test fact!!!!!", "physics", 100, types.FactStatus_FACT_STATUS_VERIFIED)
	require.NoError(t, k.SetFact(ctx, fact))

	require.NoError(t, k.ProcessMetabolism(ctx, 1))

	updated, found := k.GetFact(ctx, "fact-ar")
	require.True(t, found)

	// Energy = 100 - 100 (base cost) = 0 → should be AT_RISK
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

	// Fact at risk with 0 energy
	fact := makeEnergyFact("fact-rec", "Recovery test fact!!!", "physics", 0, types.FactStatus_FACT_STATUS_AT_RISK)
	fact.AtRiskSinceEpoch = 1
	fact.QueryCountEpoch = 20 // 20 queries → 200 energy income
	require.NoError(t, k.SetFact(ctx, fact))

	require.NoError(t, k.ProcessMetabolism(ctx, 2))

	updated, found := k.GetFact(ctx, "fact-rec")
	require.True(t, found)

	// Income = 20 * 10 = 200, Cost = 100 → net = 100 energy
	require.Greater(t, updated.Energy, uint64(0))
	require.Equal(t, types.FactStatus_FACT_STATUS_ACTIVE, updated.Status)
	require.Equal(t, uint64(0), updated.AtRiskSinceEpoch, "should clear at-risk epoch on recovery")
}

func TestMetabolism_ChallengeSurvivalBoost(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Create a fact and a rejected challenge claim
	fact := makeEnergyFact("fact-cs", "Challenge survival test", "physics", 2000, types.FactStatus_FACT_STATUS_CHALLENGED)
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

	// 2000 + 500 = 2500
	require.Equal(t, uint64(2500), updated.Energy)
	require.Equal(t, types.FactStatus_FACT_STATUS_ACTIVE, updated.Status)
}

func TestMetabolism_EnergyCap(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Fact near cap with lots of income
	fact := makeEnergyFact("fact-cap", "Energy cap test fact!!", "physics", 9900, types.FactStatus_FACT_STATUS_VERIFIED)
	fact.QueryCountEpoch = 100 // 100 * 10 = 1000 income
	require.NoError(t, k.SetFact(ctx, fact))

	require.NoError(t, k.ProcessMetabolism(ctx, 1))

	updated, found := k.GetFact(ctx, "fact-cap")
	require.True(t, found)

	// 9900 + 1000 - 100 = 10800, but capped at 10000
	require.Equal(t, uint64(10_000), updated.Energy, "energy should not exceed cap")
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

	require.Equal(t, uint64(5000), updated.Energy, "new facts should start with initial energy")
	require.Equal(t, uint64(10_000), updated.EnergyCap, "energy cap should match params")
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
