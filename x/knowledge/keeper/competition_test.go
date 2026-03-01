package keeper_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Helpers ─────────────────────────────────────────────────────────────────

// makeNicheFact creates a fact with structured fields suitable for niche competition.
func makeNicheFact(id, domain, subject, scope string, claimType types.ClaimType, fitness, energy uint64, status types.FactStatus) *types.Fact {
	return &types.Fact{
		Id:        id,
		Content:   "test content for " + id,
		Domain:    domain,
		Status:    status,
		ClaimType: claimType,
		Structure: &types.ClaimStructure{
			Subject: subject,
			Scope:   scope,
		},
		FitnessScore: fitness,
		Energy:       energy,
		EnergyCap:    10_000,
		Submitter:    "zrn1test",
	}
}

// setupCompetitionTest creates a keeper with competition params and returns it at a fitness epoch boundary.
func setupCompetitionTest(t *testing.T) (k interface{ // workaround: use concrete keeper
}, ctx sdk.Context) {
	t.Helper()
	kk, c := setupKnowledgeTest(t)
	return kk, c
}

// ─── Tests ───────────────────────────────────────────────────────────────────

func TestNicheKey_SameSubjectSameDomain(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	fact1 := makeNicheFact("fact1", "physics", "water boiling point", "", types.ClaimType_CLAIM_TYPE_ASSERTION, 500_000, 5000, types.FactStatus_FACT_STATUS_VERIFIED)
	fact2 := makeNicheFact("fact2", "physics", "water boiling point", "", types.ClaimType_CLAIM_TYPE_ASSERTION, 400_000, 5000, types.FactStatus_FACT_STATUS_VERIFIED)

	require.NoError(t, k.SetFact(ctx, fact1))
	require.NoError(t, k.SetFact(ctx, fact2))

	key1 := k.ComputeNicheKey(fact1)
	key2 := k.ComputeNicheKey(fact2)

	require.Equal(t, key1, key2, "same domain + subject + claim_type should produce same niche key")
	require.NotEmpty(t, key1)
}

func TestNicheKey_DifferentScope(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	fact1 := makeNicheFact("fact1", "physics", "water boiling point", "", types.ClaimType_CLAIM_TYPE_ASSERTION, 500_000, 5000, types.FactStatus_FACT_STATUS_VERIFIED)
	fact2 := makeNicheFact("fact2", "physics", "water boiling point", "at altitude", types.ClaimType_CLAIM_TYPE_ASSERTION, 400_000, 5000, types.FactStatus_FACT_STATUS_VERIFIED)

	require.NoError(t, k.SetFact(ctx, fact1))
	require.NoError(t, k.SetFact(ctx, fact2))

	key1 := k.ComputeNicheKey(fact1)
	key2 := k.ComputeNicheKey(fact2)

	require.NotEqual(t, key1, key2, "different scope = different niche (sub-niche)")
}

func TestCompetition_LeaderGetsBonus(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Create two facts in the same niche with different fitness
	leader := makeNicheFact("leader1", "physics", "gravity", "", types.ClaimType_CLAIM_TYPE_ASSERTION, 800_000, 5000, types.FactStatus_FACT_STATUS_VERIFIED)
	follower := makeNicheFact("follower1", "physics", "gravity", "", types.ClaimType_CLAIM_TYPE_ASSERTION, 400_000, 5000, types.FactStatus_FACT_STATUS_VERIFIED)

	require.NoError(t, k.SetFact(ctx, leader))
	require.NoError(t, k.SetFact(ctx, follower))
	require.NoError(t, k.UpdateNicheIndex(ctx, leader))
	require.NoError(t, k.UpdateNicheIndex(ctx, follower))

	// Process competition
	err := k.ProcessCompetition(ctx, 1)
	require.NoError(t, err)

	// Verify leader status
	updatedLeader, found := k.GetFact(ctx, "leader1")
	require.True(t, found)
	require.True(t, updatedLeader.NicheLeader, "highest fitness fact should be niche leader")
	require.Equal(t, uint64(1), updatedLeader.NicheRank)
	require.Equal(t, uint64(0), updatedLeader.CompetitionTax, "leader should have no competition tax")

	// Verify follower status
	updatedFollower, found := k.GetFact(ctx, "follower1")
	require.True(t, found)
	require.False(t, updatedFollower.NicheLeader)
	require.Equal(t, uint64(2), updatedFollower.NicheRank)
}

func TestCompetition_TaxProportionalToGap(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	params, err := k.GetParams(ctx)
	require.NoError(t, err)

	// Leader at 1,000,000 fitness, follower at 500,000 (50% ratio → 50% gap)
	leader := makeNicheFact("leader_tax", "physics", "tax test", "", types.ClaimType_CLAIM_TYPE_ASSERTION, 1_000_000, 5000, types.FactStatus_FACT_STATUS_VERIFIED)
	follower := makeNicheFact("follower_tax", "physics", "tax test", "", types.ClaimType_CLAIM_TYPE_ASSERTION, 500_000, 5000, types.FactStatus_FACT_STATUS_VERIFIED)

	require.NoError(t, k.SetFact(ctx, leader))
	require.NoError(t, k.SetFact(ctx, follower))
	require.NoError(t, k.UpdateNicheIndex(ctx, leader))
	require.NoError(t, k.UpdateNicheIndex(ctx, follower))

	err = k.ProcessCompetition(ctx, 1)
	require.NoError(t, err)

	updatedFollower, found := k.GetFact(ctx, "follower_tax")
	require.True(t, found)

	// 50% gap → competition_tax = base_cost * 0.5
	expectedTax := params.MetabolismBaseCost / 2 // safeMulDiv(100, 500000, 1000000) = 50
	require.Equal(t, expectedTax, updatedFollower.CompetitionTax,
		"competition tax should be proportional to fitness gap (50%% gap → %d)", expectedTax)
}

func TestCompetition_RedundantTripleTax(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	params, err := k.GetParams(ctx)
	require.NoError(t, err)

	// Leader at 1,000,000, redundant fact at 100,000 (10% ratio < 20% threshold)
	leader := makeNicheFact("leader_red", "physics", "redundancy test", "", types.ClaimType_CLAIM_TYPE_ASSERTION, 1_000_000, 5000, types.FactStatus_FACT_STATUS_VERIFIED)
	redundant := makeNicheFact("redundant1", "physics", "redundancy test", "", types.ClaimType_CLAIM_TYPE_ASSERTION, 100_000, 5000, types.FactStatus_FACT_STATUS_VERIFIED)

	require.NoError(t, k.SetFact(ctx, leader))
	require.NoError(t, k.SetFact(ctx, redundant))
	require.NoError(t, k.UpdateNicheIndex(ctx, leader))
	require.NoError(t, k.UpdateNicheIndex(ctx, redundant))

	err = k.ProcessCompetition(ctx, 1)
	require.NoError(t, err)

	updatedRedundant, found := k.GetFact(ctx, "redundant1")
	require.True(t, found)

	// 90% gap → base tax = baseCost * 0.9 = 90
	// 100k/1M = 10% ratio < 20% threshold → triple: 90 * 3 = 270
	baseTax := params.MetabolismBaseCost * 900_000 / 1_000_000 // 90
	expectedTax := baseTax * 3                                  // 270

	require.Equal(t, expectedTax, updatedRedundant.CompetitionTax,
		"redundant fact should pay 3x competition tax")
}

func TestCompetition_ForcedPruning(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Set max niche size to 3 for testing
	params, err := k.GetParams(ctx)
	require.NoError(t, err)
	params.CompetitionMaxNicheSize = 3
	require.NoError(t, k.SetParams(ctx, params))

	// Create 5 facts in the same niche
	for i := 0; i < 5; i++ {
		fact := makeNicheFact(
			"prune_"+string(rune('a'+i)),
			"physics", "forced pruning test", "",
			types.ClaimType_CLAIM_TYPE_ASSERTION,
			uint64(500_000-i*100_000), // decreasing fitness
			5000,
			types.FactStatus_FACT_STATUS_VERIFIED,
		)
		require.NoError(t, k.SetFact(ctx, fact))
		require.NoError(t, k.UpdateNicheIndex(ctx, fact))
	}

	err = k.ProcessCompetition(ctx, 1)
	require.NoError(t, err)

	// Facts ranked 4 and 5 (indices 3 and 4) should be pruned
	for i := 0; i < 5; i++ {
		factID := "prune_" + string(rune('a'+i))
		fact, found := k.GetFact(ctx, factID)
		require.True(t, found, "fact %s should exist", factID)

		if i < 3 {
			require.NotEqual(t, types.FactStatus_FACT_STATUS_PRUNED, fact.Status,
				"fact %s (rank %d) should NOT be pruned", factID, i+1)
		} else {
			require.Equal(t, types.FactStatus_FACT_STATUS_PRUNED, fact.Status,
				"fact %s (rank %d) should be pruned (exceeds max niche size)", factID, i+1)
			require.Equal(t, uint64(0), fact.Energy, "pruned fact should have 0 energy")
		}
	}
}

func TestNicheDisplacement(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Create an existing niche leader
	leader := makeNicheFact("old_leader", "physics", "displacement test", "", types.ClaimType_CLAIM_TYPE_ASSERTION, 600_000, 5000, types.FactStatus_FACT_STATUS_VERIFIED)
	require.NoError(t, k.SetFact(ctx, leader))
	require.NoError(t, k.UpdateNicheIndex(ctx, leader))

	// Process competition to establish leadership
	err := k.ProcessCompetition(ctx, 1)
	require.NoError(t, err)

	// Add a new fact with higher fitness
	challenger := makeNicheFact("new_leader", "physics", "displacement test", "", types.ClaimType_CLAIM_TYPE_ASSERTION, 900_000, 5000, types.FactStatus_FACT_STATUS_VERIFIED)
	require.NoError(t, k.SetFact(ctx, challenger))
	require.NoError(t, k.UpdateNicheIndex(ctx, challenger))

	// Process competition again
	err = k.ProcessCompetition(ctx, 2)
	require.NoError(t, err)

	// Verify new leader
	updatedChallenger, found := k.GetFact(ctx, "new_leader")
	require.True(t, found)
	require.True(t, updatedChallenger.NicheLeader, "higher fitness fact should become leader")
	require.Equal(t, uint64(1), updatedChallenger.NicheRank)

	// Old leader should be demoted
	updatedOld, found := k.GetFact(ctx, "old_leader")
	require.True(t, found)
	require.False(t, updatedOld.NicheLeader, "old leader should lose leadership")
	require.Equal(t, uint64(2), updatedOld.NicheRank)
}

func TestNicheSuccession(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Create niche with leader and successor
	leader := makeNicheFact("dying_leader", "physics", "succession test", "", types.ClaimType_CLAIM_TYPE_ASSERTION, 800_000, 5000, types.FactStatus_FACT_STATUS_VERIFIED)
	successor := makeNicheFact("successor1", "physics", "succession test", "", types.ClaimType_CLAIM_TYPE_ASSERTION, 600_000, 5000, types.FactStatus_FACT_STATUS_VERIFIED)

	require.NoError(t, k.SetFact(ctx, leader))
	require.NoError(t, k.SetFact(ctx, successor))
	require.NoError(t, k.UpdateNicheIndex(ctx, leader))
	require.NoError(t, k.UpdateNicheIndex(ctx, successor))

	// Process competition to establish leadership
	err := k.ProcessCompetition(ctx, 1)
	require.NoError(t, err)

	// Verify leader is established
	updatedLeader, _ := k.GetFact(ctx, "dying_leader")
	require.True(t, updatedLeader.NicheLeader)

	// Kill the leader (prune it)
	updatedLeader.Status = types.FactStatus_FACT_STATUS_PRUNED
	require.NoError(t, k.SetFact(ctx, updatedLeader))

	// Process competition again — successor should inherit
	err = k.ProcessCompetition(ctx, 2)
	require.NoError(t, err)

	// Successor should now be leader
	updatedSuccessor, found := k.GetFact(ctx, "successor1")
	require.True(t, found)
	require.True(t, updatedSuccessor.NicheLeader, "successor should inherit niche leadership when leader dies")
	require.Equal(t, uint64(1), updatedSuccessor.NicheRank)
	require.Equal(t, uint64(1), updatedSuccessor.NicheSize, "niche size should reflect only living members")
}

func TestSymbiosis_SupportsBonus(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	params, err := k.GetParams(ctx)
	require.NoError(t, err)

	// Create a healthy target fact (fitness > 500k)
	target := makeNicheFact("target_healthy", "physics", "target subject", "", types.ClaimType_CLAIM_TYPE_ASSERTION, 700_000, 5000, types.FactStatus_FACT_STATUS_VERIFIED)
	require.NoError(t, k.SetFact(ctx, target))

	// Create a supporting fact
	supporter := makeNicheFact("supporter1", "physics", "supporter subject", "", types.ClaimType_CLAIM_TYPE_ASSERTION, 400_000, 5000, types.FactStatus_FACT_STATUS_VERIFIED)
	require.NoError(t, k.SetFact(ctx, supporter))

	// Create SUPPORTS relation from supporter to target
	rel := &types.FactRelation{
		SourceFactId:   "supporter1",
		TargetFactId:   "target_healthy",
		Relation:       types.RelationType_RELATION_TYPE_SUPPORTS,
		CreatedAtBlock: 100,
		Creator:        "zrn1test",
	}
	require.NoError(t, k.SetFactRelation(ctx, rel))

	// Process symbiosis
	k.ProcessSymbiosis(ctx, params)

	// Supporter should get a fitness bonus
	updatedSupporter, found := k.GetFact(ctx, "supporter1")
	require.True(t, found)
	require.Greater(t, updatedSupporter.FitnessScore, uint64(400_000),
		"supporter of healthy fact should get symbiosis fitness bonus")
}

func TestSymbiosis_NoBonus_UnhealthyTarget(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	params, err := k.GetParams(ctx)
	require.NoError(t, err)

	// Create an unhealthy target fact (fitness <= 500k)
	target := makeNicheFact("target_unhealthy", "physics", "unhealthy target", "", types.ClaimType_CLAIM_TYPE_ASSERTION, 300_000, 5000, types.FactStatus_FACT_STATUS_VERIFIED)
	require.NoError(t, k.SetFact(ctx, target))

	// Create a supporting fact
	supporter := makeNicheFact("supporter2", "physics", "supporter2 subject", "", types.ClaimType_CLAIM_TYPE_ASSERTION, 400_000, 5000, types.FactStatus_FACT_STATUS_VERIFIED)
	require.NoError(t, k.SetFact(ctx, supporter))

	// Create SUPPORTS relation
	rel := &types.FactRelation{
		SourceFactId:   "supporter2",
		TargetFactId:   "target_unhealthy",
		Relation:       types.RelationType_RELATION_TYPE_SUPPORTS,
		CreatedAtBlock: 100,
		Creator:        "zrn1test",
	}
	require.NoError(t, k.SetFactRelation(ctx, rel))

	// Process symbiosis
	k.ProcessSymbiosis(ctx, params)

	// Supporter should NOT get a bonus (target is unhealthy)
	updatedSupporter, found := k.GetFact(ctx, "supporter2")
	require.True(t, found)
	require.Equal(t, uint64(400_000), updatedSupporter.FitnessScore,
		"supporter of unhealthy fact should get no symbiosis bonus")
}

func TestCompetition_UnstructuredFacts(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Create two unstructured facts (no Structure field)
	fact1 := &types.Fact{
		Id:           "unstruct1",
		Content:      "some unstructured content",
		Domain:       "physics",
		Status:       types.FactStatus_FACT_STATUS_VERIFIED,
		FitnessScore: 500_000,
		Energy:       5000,
		EnergyCap:    10_000,
		Submitter:    "zrn1test",
	}
	fact2 := &types.Fact{
		Id:           "unstruct2",
		Content:      "another unstructured content",
		Domain:       "physics",
		Status:       types.FactStatus_FACT_STATUS_VERIFIED,
		FitnessScore: 300_000,
		Energy:       5000,
		EnergyCap:    10_000,
		Submitter:    "zrn1test",
	}

	require.NoError(t, k.SetFact(ctx, fact1))
	require.NoError(t, k.SetFact(ctx, fact2))
	require.NoError(t, k.UpdateNicheIndex(ctx, fact1))
	require.NoError(t, k.UpdateNicheIndex(ctx, fact2))

	// Each should be in its own niche (solo:factID)
	key1 := k.ComputeNicheKey(fact1)
	key2 := k.ComputeNicheKey(fact2)
	require.NotEqual(t, key1, key2, "unstructured facts should each have unique niche keys")
	require.Contains(t, key1, "solo:", "unstructured fact niche key should start with solo:")
	require.Contains(t, key2, "solo:", "unstructured fact niche key should start with solo:")

	// Process competition — no competition between them
	err := k.ProcessCompetition(ctx, 1)
	require.NoError(t, err)

	// Both should be sole leaders in their niches
	updated1, found := k.GetFact(ctx, "unstruct1")
	require.True(t, found)
	require.True(t, updated1.NicheLeader)
	require.Equal(t, uint64(1), updated1.NicheSize)
	require.Equal(t, uint64(0), updated1.CompetitionTax)

	updated2, found := k.GetFact(ctx, "unstruct2")
	require.True(t, found)
	require.True(t, updated2.NicheLeader)
	require.Equal(t, uint64(1), updated2.NicheSize)
	require.Equal(t, uint64(0), updated2.CompetitionTax)
}

func TestWaterBoiling_Displacement(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	// Use a block height at a fitness epoch boundary
	ctx = ctx.WithBlockHeader(cmtproto.Header{Height: 10_000})

	// Create the imprecise fact: "water boils at 100°C"
	imprecise := makeNicheFact("boil_100c", "physics", "water boiling point", "",
		types.ClaimType_CLAIM_TYPE_ASSERTION, 500_000, 5000, types.FactStatus_FACT_STATUS_VERIFIED)
	imprecise.Content = "Water boils at 100°C"

	require.NoError(t, k.SetFact(ctx, imprecise))
	require.NoError(t, k.UpdateNicheIndex(ctx, imprecise))

	// Process competition — sole occupant becomes leader
	err := k.ProcessCompetition(ctx, 1)
	require.NoError(t, err)

	// Verify it's the leader
	updatedImprecise, _ := k.GetFact(ctx, "boil_100c")
	require.True(t, updatedImprecise.NicheLeader)

	// Now submit the more precise fact: "water boils at 99.97°C at 101.325 kPa"
	precise := makeNicheFact("boil_precise", "physics", "water boiling point", "",
		types.ClaimType_CLAIM_TYPE_ASSERTION, 800_000, 5000, types.FactStatus_FACT_STATUS_VERIFIED)
	precise.Content = "Water boils at 99.97°C at standard atmospheric pressure (101.325 kPa)"

	require.NoError(t, k.SetFact(ctx, precise))
	require.NoError(t, k.UpdateNicheIndex(ctx, precise))

	// Process competition again — precise fact should win
	err = k.ProcessCompetition(ctx, 2)
	require.NoError(t, err)

	// Verify the precise fact is now the leader
	updatedPrecise, found := k.GetFact(ctx, "boil_precise")
	require.True(t, found)
	require.True(t, updatedPrecise.NicheLeader, "more precise fact should become niche leader")
	require.Equal(t, uint64(1), updatedPrecise.NicheRank)

	// Verify the imprecise fact is demoted and pays competition tax
	updatedImprecise2, found := k.GetFact(ctx, "boil_100c")
	require.True(t, found)
	require.False(t, updatedImprecise2.NicheLeader, "imprecise fact should lose leadership")
	require.Equal(t, uint64(2), updatedImprecise2.NicheRank)
	require.Greater(t, updatedImprecise2.CompetitionTax, uint64(0),
		"displaced fact should pay competition tax")
}
