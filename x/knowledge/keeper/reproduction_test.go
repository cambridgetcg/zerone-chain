package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/keeper"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// makeLineageFact creates a fact with lineage and metabolism fields for testing.
func makeLineageFact(id, content, domain, submitter string, fitnessScore, energy uint64) *types.Fact {
	return &types.Fact{
		Id:            id,
		Content:       content,
		Domain:        domain,
		Category:      "empirical",
		Confidence:    700_000,
		Submitter:     submitter,
		Status:        types.FactStatus_FACT_STATUS_VERIFIED,
		FitnessScore:  fitnessScore,
		Energy:        energy,
		EnergyCap:     10_000,
	}
}

// TestReproduction_ParentChildLink verifies child fact links to parent correctly.
func TestReproduction_ParentChildLink(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	parent := makeLineageFact("parent-1", "Parent fact", "physics", makeValidBech32Addr("submitter1"), 600_000, 5000)
	require.NoError(t, k.SetFact(ctx, parent))

	// Create child claim with REFINES relation
	claim := &types.Claim{
		Id:               "claim-child-1",
		FactContent:      "Child fact refining parent fact about physics",
		Domain:           "physics",
		Category:         "empirical",
		Submitter:        makeValidBech32Addr("submitter2"),
		SubmittedAtBlock: 100,
		Status:           types.ClaimStatus_CLAIM_STATUS_PENDING,
		Stake:            "1000000",
		Relations: []*types.ClaimRelation{
			{TargetFactId: "parent-1", Relation: types.RelationType_RELATION_TYPE_REFINES},
		},
	}
	require.NoError(t, k.SetClaim(ctx, claim))

	round := makeRoundInPhase("round-child-1", claim.Id, types.VerificationPhase_VERIFICATION_PHASE_AGGREGATION, 100)
	require.NoError(t, k.SetVerificationRound(ctx, round))

	result := &keeper.VerificationResult{
		Verdict:    types.Verdict_VERDICT_ACCEPT,
		Confidence: 700_000,
	}
	require.NoError(t, k.CompleteRound(ctx, round, result))

	// Find the created fact (created from the claim)
	var childFact *types.Fact
	k.IterateFacts(ctx, func(f *types.Fact) bool {
		if f.ClaimId == "claim-child-1" {
			childFact = f
			return true
		}
		return false
	})
	require.NotNil(t, childFact, "child fact should have been created")
	require.Equal(t, "parent-1", childFact.ParentFactId)
	require.Equal(t, uint64(1), childFact.LineageDepth)

	// Verify parent has child listed
	updatedParent, found := k.GetFact(ctx, "parent-1")
	require.True(t, found)
	require.Contains(t, updatedParent.ChildFactIds, childFact.Id)
}

// TestReproduction_LineageDepth verifies depth increments through generations.
func TestReproduction_LineageDepth(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Create grandparent (depth 0)
	grandparent := makeLineageFact("gp-1", "Grandparent fact", "physics", makeValidBech32Addr("sub1"), 600_000, 5000)
	require.NoError(t, k.SetFact(ctx, grandparent))

	// Create parent (depth 1) with parent link
	parent := makeLineageFact("parent-2", "Parent fact", "physics", makeValidBech32Addr("sub2"), 500_000, 5000)
	parent.ParentFactId = "gp-1"
	parent.LineageDepth = 1
	parent.LineageRootId = "gp-1"
	require.NoError(t, k.SetFact(ctx, parent))
	// Update grandparent's children
	grandparent.ChildFactIds = []string{"parent-2"}
	require.NoError(t, k.SetFact(ctx, grandparent))

	// Create child (should become depth 2) via round completion
	claim := &types.Claim{
		Id:               "claim-depth-test",
		FactContent:      "Grandchild refining parent two with more info",
		Domain:           "physics",
		Category:         "empirical",
		Submitter:        makeValidBech32Addr("sub3"),
		SubmittedAtBlock: 100,
		Status:           types.ClaimStatus_CLAIM_STATUS_PENDING,
		Stake:            "1000000",
		Relations: []*types.ClaimRelation{
			{TargetFactId: "parent-2", Relation: types.RelationType_RELATION_TYPE_REFINES},
		},
	}
	require.NoError(t, k.SetClaim(ctx, claim))
	round := makeRoundInPhase("round-depth", claim.Id, types.VerificationPhase_VERIFICATION_PHASE_AGGREGATION, 100)
	require.NoError(t, k.SetVerificationRound(ctx, round))

	result := &keeper.VerificationResult{Verdict: types.Verdict_VERDICT_ACCEPT, Confidence: 700_000}
	require.NoError(t, k.CompleteRound(ctx, round, result))

	var childFact *types.Fact
	k.IterateFacts(ctx, func(f *types.Fact) bool {
		if f.ClaimId == "claim-depth-test" {
			childFact = f
			return true
		}
		return false
	})
	require.NotNil(t, childFact)
	require.Equal(t, uint64(2), childFact.LineageDepth)
	require.Equal(t, "gp-1", childFact.LineageRootId)
}

// TestReproduction_FitnessInheritance verifies child starts with 20% of parent fitness.
func TestReproduction_FitnessInheritance(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	parent := makeLineageFact("parent-fit", "High fitness parent fact about physics", "physics", makeValidBech32Addr("sub1"), 800_000, 5000)
	require.NoError(t, k.SetFact(ctx, parent))

	claim := &types.Claim{
		Id:               "claim-fitness-inherit",
		FactContent:      "Child inheriting fitness from parent about physics",
		Domain:           "physics",
		Category:         "empirical",
		Submitter:        makeValidBech32Addr("sub2"),
		SubmittedAtBlock: 100,
		Status:           types.ClaimStatus_CLAIM_STATUS_PENDING,
		Stake:            "1000000",
		Relations: []*types.ClaimRelation{
			{TargetFactId: "parent-fit", Relation: types.RelationType_RELATION_TYPE_REFINES},
		},
	}
	require.NoError(t, k.SetClaim(ctx, claim))
	round := makeRoundInPhase("round-fitness", claim.Id, types.VerificationPhase_VERIFICATION_PHASE_AGGREGATION, 100)
	require.NoError(t, k.SetVerificationRound(ctx, round))

	result := &keeper.VerificationResult{Verdict: types.Verdict_VERDICT_ACCEPT, Confidence: 700_000}
	require.NoError(t, k.CompleteRound(ctx, round, result))

	var childFact *types.Fact
	k.IterateFacts(ctx, func(f *types.Fact) bool {
		if f.ClaimId == "claim-fitness-inherit" {
			childFact = f
			return true
		}
		return false
	})
	require.NotNil(t, childFact)
	// 20% of 800,000 = 160,000
	require.Equal(t, uint64(160_000), childFact.FitnessScore)
}

// TestReproduction_ParentEnergyBonus verifies parent gains energy when child is created.
func TestReproduction_ParentEnergyBonus(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	parent := makeLineageFact("parent-energy", "Parent for energy bonus test in physics", "physics", makeValidBech32Addr("sub1"), 500_000, 4000)
	require.NoError(t, k.SetFact(ctx, parent))

	claim := &types.Claim{
		Id:               "claim-energy-bonus",
		FactContent:      "Child claim triggering energy bonus for parent",
		Domain:           "physics",
		Category:         "empirical",
		Submitter:        makeValidBech32Addr("sub2"),
		SubmittedAtBlock: 100,
		Status:           types.ClaimStatus_CLAIM_STATUS_PENDING,
		Stake:            "1000000",
		Relations: []*types.ClaimRelation{
			{TargetFactId: "parent-energy", Relation: types.RelationType_RELATION_TYPE_GENERALIZES},
		},
	}
	require.NoError(t, k.SetClaim(ctx, claim))
	round := makeRoundInPhase("round-energy", claim.Id, types.VerificationPhase_VERIFICATION_PHASE_AGGREGATION, 100)
	require.NoError(t, k.SetVerificationRound(ctx, round))

	result := &keeper.VerificationResult{Verdict: types.Verdict_VERDICT_ACCEPT, Confidence: 700_000}
	require.NoError(t, k.CompleteRound(ctx, round, result))

	updatedParent, found := k.GetFact(ctx, "parent-energy")
	require.True(t, found)
	// Parent started at 4000 energy, bonus is 300 → 4300
	require.Equal(t, uint64(4300), updatedParent.Energy)
}

// TestReproduction_ProgenyCountPropagation verifies progeny count propagates up lineage.
func TestReproduction_ProgenyCountPropagation(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Create grandparent → parent chain
	gp := makeLineageFact("gp-prog", "Grandparent for propagation", "physics", makeValidBech32Addr("sub1"), 500_000, 5000)
	require.NoError(t, k.SetFact(ctx, gp))

	parent := makeLineageFact("parent-prog", "Parent for propagation test", "physics", makeValidBech32Addr("sub2"), 500_000, 5000)
	parent.ParentFactId = "gp-prog"
	parent.LineageDepth = 1
	parent.LineageRootId = "gp-prog"
	require.NoError(t, k.SetFact(ctx, parent))
	gp.ChildFactIds = []string{"parent-prog"}
	require.NoError(t, k.SetFact(ctx, gp))

	// Create child via round — should propagate progeny up
	claim := &types.Claim{
		Id:               "claim-progeny-prop",
		FactContent:      "Child triggering progeny propagation up to root",
		Domain:           "physics",
		Category:         "empirical",
		Submitter:        makeValidBech32Addr("sub3"),
		SubmittedAtBlock: 100,
		Status:           types.ClaimStatus_CLAIM_STATUS_PENDING,
		Stake:            "1000000",
		Relations: []*types.ClaimRelation{
			{TargetFactId: "parent-prog", Relation: types.RelationType_RELATION_TYPE_REFINES},
		},
	}
	require.NoError(t, k.SetClaim(ctx, claim))
	round := makeRoundInPhase("round-progeny", claim.Id, types.VerificationPhase_VERIFICATION_PHASE_AGGREGATION, 100)
	require.NoError(t, k.SetVerificationRound(ctx, round))

	result := &keeper.VerificationResult{Verdict: types.Verdict_VERDICT_ACCEPT, Confidence: 700_000}
	require.NoError(t, k.CompleteRound(ctx, round, result))

	// Parent should have progeny_count = 1 (direct child)
	updatedParent, found := k.GetFact(ctx, "parent-prog")
	require.True(t, found)
	require.Equal(t, uint64(1), updatedParent.ProgenyCount)

	// Grandparent should also have progeny_count = 1 (propagated)
	updatedGP, found := k.GetFact(ctx, "gp-prog")
	require.True(t, found)
	require.Equal(t, uint64(1), updatedGP.ProgenyCount)
}

// TestRoyalty_ParentGets5Percent verifies parent receives 5% of child reward.
func TestRoyalty_ParentGets5Percent(t *testing.T) {
	k, ctx, bk := setupKnowledgeTestWithBank(t)

	parentAddr := makeValidBech32Addr("royalty-parent1")
	parent := makeLineageFact("parent-royalty", "Parent for royalty test in physics", "physics", parentAddr, 500_000, 5000)
	require.NoError(t, k.SetFact(ctx, parent))

	// Create child with parent link
	child := makeLineageFact("child-royalty", "Child earning rewards from physics", "physics", makeValidBech32Addr("child-sub"), 100_000, 5000)
	child.ParentFactId = "parent-royalty"
	child.LineageDepth = 1
	require.NoError(t, k.SetFact(ctx, child))

	// Distribute royalties for a 1,000,000 uzrn reward
	err := k.DistributeLineageRoyalties(ctx, "child-royalty", 1_000_000)
	require.NoError(t, err)

	// Parent should receive 5% = 50,000 uzrn
	require.Len(t, bk.sendCalls, 1)
	require.Equal(t, parentAddr, bk.sendCalls[0].to)
	require.Equal(t, int64(50_000), bk.sendCalls[0].coins.AmountOf("uzrn").Int64())
}

// TestRoyalty_GrandparentGets2_5Percent verifies grandparent receives 2.5%.
func TestRoyalty_GrandparentGets2_5Percent(t *testing.T) {
	k, ctx, bk := setupKnowledgeTestWithBank(t)

	gpAddr := makeValidBech32Addr("royalty-gp-addr")
	parentAddr := makeValidBech32Addr("royalty-par-addr")

	gp := makeLineageFact("gp-royalty", "Grandparent for royalty test", "physics", gpAddr, 500_000, 5000)
	require.NoError(t, k.SetFact(ctx, gp))

	parent := makeLineageFact("parent-royalty-2", "Parent in royalty chain", "physics", parentAddr, 400_000, 5000)
	parent.ParentFactId = "gp-royalty"
	parent.LineageDepth = 1
	require.NoError(t, k.SetFact(ctx, parent))

	child := makeLineageFact("child-royalty-2", "Child earning rewards in chain", "physics", makeValidBech32Addr("child-sub2"), 100_000, 5000)
	child.ParentFactId = "parent-royalty-2"
	child.LineageDepth = 2
	require.NoError(t, k.SetFact(ctx, child))

	err := k.DistributeLineageRoyalties(ctx, "child-royalty-2", 1_000_000)
	require.NoError(t, err)

	// Two sends: parent gets 5% = 50,000; grandparent gets 2.5% = 25,000
	require.Len(t, bk.sendCalls, 2)

	// First send: parent (5%)
	require.Equal(t, parentAddr, bk.sendCalls[0].to)
	require.Equal(t, int64(50_000), bk.sendCalls[0].coins.AmountOf("uzrn").Int64())

	// Second send: grandparent (2.5%)
	require.Equal(t, gpAddr, bk.sendCalls[1].to)
	require.Equal(t, int64(25_000), bk.sendCalls[1].coins.AmountOf("uzrn").Int64())
}

// TestRoyalty_MaxDepth verifies royalties stop at 5 generations.
func TestRoyalty_MaxDepth(t *testing.T) {
	k, ctx, bk := setupKnowledgeTestWithBank(t)

	// Create a chain of 7 facts (0..6), with child at index 6
	facts := make([]*types.Fact, 7)
	for i := 0; i < 7; i++ {
		addr := makeValidBech32Addr("depth-" + string(rune('a'+i)))
		f := makeLineageFact(
			"depth-"+string(rune('a'+i)),
			"Fact at depth "+string(rune('0'+i))+" in royalty chain",
			"physics",
			addr,
			500_000,
			5000,
		)
		if i > 0 {
			f.ParentFactId = facts[i-1].Id
			f.LineageDepth = uint64(i)
			f.LineageRootId = "depth-a"
		}
		require.NoError(t, k.SetFact(ctx, f))
		facts[i] = f
	}

	err := k.DistributeLineageRoyalties(ctx, "depth-g", 1_000_000)
	require.NoError(t, err)

	// Max depth is 5, so only 5 sends (to parents at depth 5,4,3,2,1)
	require.Len(t, bk.sendCalls, 5)
}

// TestDisprovenCascade_ChildrenAtRisk verifies disproven parent puts children at risk.
func TestDisprovenCascade_ChildrenAtRisk(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	parent := makeLineageFact("parent-disproven", "Parent that will be disproven", "physics", makeValidBech32Addr("sub1"), 500_000, 5000)
	parent.ChildFactIds = []string{"child-at-risk-1", "child-at-risk-2"}
	require.NoError(t, k.SetFact(ctx, parent))

	child1 := makeLineageFact("child-at-risk-1", "Child one of disproven parent", "physics", makeValidBech32Addr("sub2"), 400_000, 6000)
	child1.ParentFactId = "parent-disproven"
	child1.Status = types.FactStatus_FACT_STATUS_ACTIVE
	require.NoError(t, k.SetFact(ctx, child1))

	child2 := makeLineageFact("child-at-risk-2", "Child two of disproven parent", "physics", makeValidBech32Addr("sub3"), 300_000, 4000)
	child2.ParentFactId = "parent-disproven"
	child2.Status = types.FactStatus_FACT_STATUS_VERIFIED
	require.NoError(t, k.SetFact(ctx, child2))

	k.CascadeDisproven(ctx, "parent-disproven")

	updated1, found := k.GetFact(ctx, "child-at-risk-1")
	require.True(t, found)
	require.Equal(t, types.FactStatus_FACT_STATUS_AT_RISK, updated1.Status)
	require.Equal(t, uint64(3000), updated1.Energy) // 6000 / 2

	updated2, found := k.GetFact(ctx, "child-at-risk-2")
	require.True(t, found)
	require.Equal(t, types.FactStatus_FACT_STATUS_AT_RISK, updated2.Status)
	require.Equal(t, uint64(2000), updated2.Energy) // 4000 / 2
}

// TestMaxChildren_Enforced verifies parent at 20 children rejects new child links.
func TestMaxChildren_Enforced(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Create parent with 20 children already
	parent := makeLineageFact("parent-maxchild", "Parent at max children capacity", "physics", makeValidBech32Addr("sub1"), 500_000, 5000)
	for i := 0; i < 20; i++ {
		childID := "existing-child-" + string(rune('a'+i))
		parent.ChildFactIds = append(parent.ChildFactIds, childID)
	}
	require.NoError(t, k.SetFact(ctx, parent))

	// Attempt to create a 21st child
	claim := &types.Claim{
		Id:               "claim-maxchild",
		FactContent:      "Twenty first child trying to link to full parent",
		Domain:           "physics",
		Category:         "empirical",
		Submitter:        makeValidBech32Addr("sub2"),
		SubmittedAtBlock: 100,
		Status:           types.ClaimStatus_CLAIM_STATUS_PENDING,
		Stake:            "1000000",
		Relations: []*types.ClaimRelation{
			{TargetFactId: "parent-maxchild", Relation: types.RelationType_RELATION_TYPE_REFINES},
		},
	}
	require.NoError(t, k.SetClaim(ctx, claim))
	round := makeRoundInPhase("round-maxchild", claim.Id, types.VerificationPhase_VERIFICATION_PHASE_AGGREGATION, 100)
	require.NoError(t, k.SetVerificationRound(ctx, round))

	result := &keeper.VerificationResult{Verdict: types.Verdict_VERDICT_ACCEPT, Confidence: 700_000}
	require.NoError(t, k.CompleteRound(ctx, round, result))

	// The new fact should NOT have a parent link (parent was at max)
	var childFact *types.Fact
	k.IterateFacts(ctx, func(f *types.Fact) bool {
		if f.ClaimId == "claim-maxchild" {
			childFact = f
			return true
		}
		return false
	})
	require.NotNil(t, childFact)
	require.Empty(t, childFact.ParentFactId, "child should not link to parent at max children")

	// Parent should still have exactly 20 children
	updatedParent, found := k.GetFact(ctx, "parent-maxchild")
	require.True(t, found)
	require.Len(t, updatedParent.ChildFactIds, 20)
}

// TestLineageQuery_TracesToRoot verifies lineage query returns full ancestor chain.
func TestLineageQuery_TracesToRoot(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Create a 3-level lineage: root → middle → leaf
	root := makeLineageFact("lineage-root", "Root fact in lineage chain", "physics", makeValidBech32Addr("sub1"), 500_000, 5000)
	require.NoError(t, k.SetFact(ctx, root))

	middle := makeLineageFact("lineage-middle", "Middle fact in lineage chain", "physics", makeValidBech32Addr("sub2"), 400_000, 5000)
	middle.ParentFactId = "lineage-root"
	middle.LineageDepth = 1
	middle.LineageRootId = "lineage-root"
	require.NoError(t, k.SetFact(ctx, middle))

	leaf := makeLineageFact("lineage-leaf", "Leaf fact in lineage chain", "physics", makeValidBech32Addr("sub3"), 300_000, 5000)
	leaf.ParentFactId = "lineage-middle"
	leaf.LineageDepth = 2
	leaf.LineageRootId = "lineage-root"
	require.NoError(t, k.SetFact(ctx, leaf))

	qs := keeper.NewQueryServerImpl(k)

	resp, err := qs.FactLineage(ctx, &types.QueryFactLineageRequest{FactId: "lineage-leaf"})
	require.NoError(t, err)
	require.Len(t, resp.Ancestors, 2)
	require.Equal(t, "lineage-middle", resp.Ancestors[0].Id)
	require.Equal(t, "lineage-root", resp.Ancestors[1].Id)
	require.Equal(t, "lineage-root", resp.RootId)
}

// TestProgenyQuery_ReturnsTree verifies progeny query returns descendant tree.
func TestProgenyQuery_ReturnsTree(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Create tree: root → [child1, child2], child1 → [grandchild1]
	root := makeLineageFact("progeny-root", "Root of progeny tree", "physics", makeValidBech32Addr("sub1"), 500_000, 5000)
	root.ChildFactIds = []string{"progeny-child1", "progeny-child2"}
	require.NoError(t, k.SetFact(ctx, root))

	child1 := makeLineageFact("progeny-child1", "First child in progeny tree", "physics", makeValidBech32Addr("sub2"), 400_000, 5000)
	child1.ParentFactId = "progeny-root"
	child1.ChildFactIds = []string{"progeny-gc1"}
	require.NoError(t, k.SetFact(ctx, child1))

	child2 := makeLineageFact("progeny-child2", "Second child in progeny tree", "physics", makeValidBech32Addr("sub3"), 350_000, 5000)
	child2.ParentFactId = "progeny-root"
	require.NoError(t, k.SetFact(ctx, child2))

	gc1 := makeLineageFact("progeny-gc1", "Grandchild in progeny tree", "physics", makeValidBech32Addr("sub4"), 300_000, 5000)
	gc1.ParentFactId = "progeny-child1"
	require.NoError(t, k.SetFact(ctx, gc1))

	qs := keeper.NewQueryServerImpl(k)

	resp, err := qs.FactProgeny(ctx, &types.QueryFactProgenyRequest{FactId: "progeny-root", Depth: 3})
	require.NoError(t, err)
	require.Equal(t, "progeny-root", resp.Root.Id)
	require.Len(t, resp.Tree, 2) // 2 direct children

	// Find child1 in tree — it should have 1 grandchild
	var child1Node *types.FactWithChildren
	for _, node := range resp.Tree {
		if node.Fact.Id == "progeny-child1" {
			child1Node = node
		}
	}
	require.NotNil(t, child1Node)
	require.Len(t, child1Node.Children, 1)
	require.Equal(t, "progeny-gc1", child1Node.Children[0].Fact.Id)
}
