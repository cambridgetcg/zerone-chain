package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/keeper"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// TestNovelty_CommonKnowledgeMatch verifies that a fact matching a common knowledge entry
// receives a novelty penalty.
func TestNovelty_CommonKnowledgeMatch(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// "water boiling point" is seeded in common knowledge registry for physics domain
	fact := &types.Fact{
		Id:     "fact-water",
		Domain: "physics",
		Status: types.FactStatus_FACT_STATUS_VERIFIED,
		Structure: &types.ClaimStructure{
			Subject:   "water boiling point",
			Predicate: "is 100 degrees celsius",
		},
	}
	require.NoError(t, k.SetFact(ctx, fact))

	novelty := k.CalculateNovelty(ctx, fact)

	// Should be penalized — 1,000,000 - 800,000 = 200,000
	require.Less(t, novelty, uint64(500_000), "common knowledge match should significantly reduce novelty")
	require.True(t, fact.CommonKnowledgeMatch, "should be flagged as common knowledge match")
}

// TestNovelty_NoMatch verifies that a novel subject gets full novelty score.
func TestNovelty_NoMatch(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	fact := &types.Fact{
		Id:     "fact-novel",
		Domain: "physics",
		Status: types.FactStatus_FACT_STATUS_VERIFIED,
		Structure: &types.ClaimStructure{
			Subject:   "quantum entanglement decoherence rates in topological qubits",
			Predicate: "scale logarithmically with qubit count",
		},
	}
	require.NoError(t, k.SetFact(ctx, fact))

	novelty := k.CalculateNovelty(ctx, fact)

	// Should get full score — not in common knowledge registry, no overlaps
	require.Equal(t, uint64(1_000_000), novelty, "novel subject should get maximum novelty")
	require.False(t, fact.CommonKnowledgeMatch, "should not be flagged as common knowledge match")
}

// TestNovelty_FuzzyMatch verifies that substring matching works.
// "water boiling point at altitude" should match "water boiling point".
func TestNovelty_FuzzyMatch(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	fact := &types.Fact{
		Id:     "fact-fuzzy",
		Domain: "physics",
		Status: types.FactStatus_FACT_STATUS_VERIFIED,
		Structure: &types.ClaimStructure{
			Subject:   "water boiling point at altitude",
			Predicate: "decreases with altitude",
		},
	}
	require.NoError(t, k.SetFact(ctx, fact))

	novelty := k.CalculateNovelty(ctx, fact)

	// Should match "water boiling point" via substring containment
	require.Less(t, novelty, uint64(500_000), "fuzzy match should penalize novelty")
	require.True(t, fact.CommonKnowledgeMatch, "fuzzy match should flag common knowledge")
}

// TestNovelty_SubjectOverlap verifies that multiple facts with the same subject reduce novelty.
func TestNovelty_SubjectOverlap(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Create 3 facts with the same subject in a domain NOT covered by common knowledge
	for i := 0; i < 3; i++ {
		f := &types.Fact{
			Id:     makeSubmitterAddr(i), // unique IDs
			Domain: "cosmology",
			Status: types.FactStatus_FACT_STATUS_VERIFIED,
			Structure: &types.ClaimStructure{
				Subject:   "dark matter distribution",
				Predicate: "varies by galaxy type",
			},
		}
		require.NoError(t, k.SetFact(ctx, f))
	}

	// Now score the 4th fact with same subject
	newFact := &types.Fact{
		Id:     "fact-overlap-new",
		Domain: "cosmology",
		Status: types.FactStatus_FACT_STATUS_VERIFIED,
		Structure: &types.ClaimStructure{
			Subject:   "dark matter distribution",
			Predicate: "is approximately 27% of total mass-energy",
		},
	}
	require.NoError(t, k.SetFact(ctx, newFact))

	novelty := k.CalculateNovelty(ctx, newFact)

	// Should be penalized: 3 overlaps * 100,000 = 300,000 penalty → 700,000
	require.Less(t, novelty, uint64(1_000_000), "subject overlap should reduce novelty")
	require.Greater(t, novelty, uint64(0), "should still have some novelty")
}

// TestNovelty_PrecisionBonus verifies that more specific scope earns a bonus.
func TestNovelty_PrecisionBonus(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Existing fact with no scope (less specific)
	existing := &types.Fact{
		Id:     "fact-vague",
		Domain: "cosmology",
		Status: types.FactStatus_FACT_STATUS_VERIFIED,
		Structure: &types.ClaimStructure{
			Subject:   "dark energy equation of state",
			Predicate: "is approximately w=-1",
		},
	}
	require.NoError(t, k.SetFact(ctx, existing))

	// New fact with more specific scope
	precise := &types.Fact{
		Id:     "fact-precise",
		Domain: "cosmology",
		Status: types.FactStatus_FACT_STATUS_VERIFIED,
		Structure: &types.ClaimStructure{
			Subject:   "dark energy equation of state",
			Predicate: "is w=-1.03 +/- 0.03",
			Scope:     "measured by DESI BAO 2024 combined with CMB and SNIa data",
		},
	}
	require.NoError(t, k.SetFact(ctx, precise))

	novelty := k.CalculateNovelty(ctx, precise)

	// Should get precision bonus (100,000) minus overlap penalty (100,000) = net even
	// But the precision bonus exists, so score should reflect it
	// Score = 1,000,000 - 100,000 (1 overlap) + 100,000 (precision) = 1,000,000
	require.Equal(t, uint64(1_000_000), novelty, "precision bonus should offset overlap penalty")
}

// TestNovelty_CrossDomainBonus verifies that bridge facts get a novelty bonus.
func TestNovelty_CrossDomainBonus(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	fact := &types.Fact{
		Id:          "fact-bridge",
		Domain:      "information_theory",
		BridgeScore: 500_000, // Cross-domain bridge value
		Status:      types.FactStatus_FACT_STATUS_VERIFIED,
		Structure: &types.ClaimStructure{
			Subject:   "kolmogorov complexity and thermodynamic entropy equivalence",
			Predicate: "are related via the Zurek-Landauer principle",
		},
	}
	require.NoError(t, k.SetFact(ctx, fact))

	novelty := k.CalculateNovelty(ctx, fact)

	// Should get cross-domain bonus: 1,000,000 + 100,000 = capped at 1,000,000
	require.Equal(t, uint64(1_000_000), novelty, "cross-domain bonus should boost (capped at max)")
}

// TestNovelty_Cap verifies that novelty score is capped at 1,000,000.
func TestNovelty_Cap(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Set generous params
	params, _ := k.GetParams(ctx)
	params.NoveltyPrecisionBonusBps = 500_000
	params.NoveltyCrossDomainBonusBps = 500_000
	require.NoError(t, k.SetParams(ctx, params))

	// Existing less-specific fact
	existing := &types.Fact{
		Id:     "fact-existing-cap",
		Domain: "cosmology",
		Status: types.FactStatus_FACT_STATUS_VERIFIED,
		Structure: &types.ClaimStructure{
			Subject:   "hubble constant",
			Predicate: "is approximately 70 km/s/Mpc",
		},
	}
	require.NoError(t, k.SetFact(ctx, existing))

	// Precise, cross-domain fact
	fact := &types.Fact{
		Id:          "fact-max-bonus",
		Domain:      "cosmology",
		BridgeScore: 500_000,
		Status:      types.FactStatus_FACT_STATUS_VERIFIED,
		Structure: &types.ClaimStructure{
			Subject:   "hubble constant",
			Predicate: "is 73.04 +/- 1.04 km/s/Mpc",
			Scope:     "SH0ES 2022 Cepheid-calibrated SNIa distance ladder",
		},
	}
	require.NoError(t, k.SetFact(ctx, fact))

	novelty := k.CalculateNovelty(ctx, fact)

	// Even with huge bonuses, should be capped at 1,000,000
	require.LessOrEqual(t, novelty, uint64(1_000_000), "novelty must be capped at 1,000,000")
}

// TestCheckNovelty_PreSubmission verifies the query endpoint returns correct preview.
func TestCheckNovelty_PreSubmission(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Check a common knowledge subject
	score, match, matchedEntry, overlap := k.CheckNoveltyPreSubmission(ctx, "physics", "water boiling point", "Water boils at 100C")

	require.True(t, match, "should match common knowledge")
	require.NotEmpty(t, matchedEntry, "should return matched entry ID")
	require.Less(t, score, uint64(500_000), "common knowledge should have low novelty")
	require.Equal(t, uint64(0), overlap, "no on-chain facts with same subject")

	// Check a novel subject
	score2, match2, _, _ := k.CheckNoveltyPreSubmission(ctx, "physics", "quantum chromodynamic confinement mechanism", "Novel QCD research")

	require.False(t, match2, "novel subject should not match")
	require.Equal(t, uint64(1_000_000), score2, "novel subject should get max score")
}

// TestCommonKnowledge_GovernanceAdd verifies that authority can add entries.
func TestCommonKnowledge_GovernanceAdd(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	msgServer := keeper.NewMsgServerImpl(k)

	// Add a new common knowledge entry
	resp, err := msgServer.AddCommonKnowledge(ctx, &types.MsgAddCommonKnowledge{
		Authority:   "zrn1authority",
		Domain:      "computer_science",
		Subject:     "hello world program",
		Description: "First program in any language tutorial",
		PenaltyBps:  800_000,
	})
	require.NoError(t, err)
	require.NotEmpty(t, resp.Id)

	// Verify it's stored
	entry, found := k.FindCommonKnowledge(ctx, "computer_science", "hello world program")
	require.True(t, found)
	require.Equal(t, uint64(800_000), entry.PenaltyBps)
	require.Equal(t, "hello world program", entry.Subject)

	// Non-authority should fail
	_, err = msgServer.AddCommonKnowledge(ctx, &types.MsgAddCommonKnowledge{
		Authority:  "zrn1random",
		Domain:     "computer_science",
		Subject:    "fizzbuzz",
		PenaltyBps: 500_000,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unauthorized")
}

// TestCommonKnowledge_GovernanceRemove verifies that authority can remove entries.
func TestCommonKnowledge_GovernanceRemove(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	msgServer := keeper.NewMsgServerImpl(k)

	// Verify a seeded entry exists
	entry, found := k.FindCommonKnowledge(ctx, "physics", "water boiling point")
	require.True(t, found, "seeded entry should exist")

	// Remove it
	_, err := msgServer.RemoveCommonKnowledge(ctx, &types.MsgRemoveCommonKnowledge{
		Authority: "zrn1authority",
		Id:        entry.Id,
	})
	require.NoError(t, err)

	// Verify it's gone
	_, found = k.FindCommonKnowledge(ctx, "physics", "water boiling point")
	require.False(t, found, "removed entry should not be found")

	// Now a fact about water boiling point should get full novelty
	fact := &types.Fact{
		Id:     "fact-water-after-remove",
		Domain: "physics",
		Status: types.FactStatus_FACT_STATUS_VERIFIED,
		Structure: &types.ClaimStructure{
			Subject:   "water boiling point",
			Predicate: "is 100 degrees celsius",
		},
	}
	require.NoError(t, k.SetFact(ctx, fact))
	novelty := k.CalculateNovelty(ctx, fact)
	require.Equal(t, uint64(1_000_000), novelty, "after removal, subject should have full novelty")
}
