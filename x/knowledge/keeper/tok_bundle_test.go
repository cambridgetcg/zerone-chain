package keeper_test

import (
	"fmt"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
	"github.com/zerone-chain/zerone/x/knowledge/keeper"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

func TestValidateToKSelector_RejectsEmptyVariant(t *testing.T) {
	err := keeper.ValidateToKSelector(&types.ToKSelector{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "selector variant required")
}

func TestValidateToKSelector_RootedSubtree_RequiresRootFactId(t *testing.T) {
	sel := &types.ToKSelector{Variant: &types.ToKSelector_RootedSubtree{
		RootedSubtree: &types.RootedSubtreeSelector{},
	}}
	err := keeper.ValidateToKSelector(sel)
	require.Error(t, err)
	require.Contains(t, err.Error(), "root_fact_id")
}

func TestValidateToKSelector_RootedSubtree_CapsMaxDepth(t *testing.T) {
	sel := &types.ToKSelector{Variant: &types.ToKSelector_RootedSubtree{
		RootedSubtree: &types.RootedSubtreeSelector{
			RootFactId: "fact-1",
			MaxDepth:   100, // > cap 32
		},
	}}
	capped, err := keeper.ValidateAndCapToKSelector(sel)
	require.NoError(t, err)
	require.Equal(t, uint32(32), capped.GetRootedSubtree().MaxDepth)
}

func TestValidateToKSelector_Frontier_RequiresDomain(t *testing.T) {
	sel := &types.ToKSelector{Variant: &types.ToKSelector_Frontier{
		Frontier: &types.FrontierSelector{},
	}}
	err := keeper.ValidateToKSelector(sel)
	require.Error(t, err)
	require.Contains(t, err.Error(), "domain")
}

func TestGatherRootedSubtree_LinearChain(t *testing.T) {
	// Build: axiom ──SUPPORTS──> b ──SUPPORTS──> c
	k, ctx := setupKnowledgeWithFacts(t, []factSpec{
		{id: "axiom", domain: "physics"},
		{id: "b", domain: "physics", supports: []string{"axiom"}},
		{id: "c", domain: "physics", supports: []string{"b"}},
	})
	sel := &types.RootedSubtreeSelector{RootFactId: "axiom", MaxDepth: 5}
	nodeIDs, edges, err := k.GatherRootedSubtree(ctx, sel)
	require.NoError(t, err)
	require.Equal(t, []string{"axiom", "b", "c"}, nodeIDs) // sorted
	require.Len(t, edges, 2)
	// Assert edge shape (not just cardinality). sortToKEdges sorts by FromFactId,
	// so b < c lexicographically — edges[0] is b→axiom, edges[1] is c→b.
	require.Equal(t, "b", edges[0].FromFactId)
	require.Equal(t, "axiom", edges[0].ToFactId)
	require.Equal(t, "c", edges[1].FromFactId)
	require.Equal(t, "b", edges[1].ToFactId)
}

func TestGatherRootedSubtree_FiltersContradictsRelations(t *testing.T) {
	// Build: axiom ──SUPPORTS──> b SUPPORTS axiom
	//                            c CONTRADICTS axiom  (must be excluded)
	k, ctx := setupKnowledgeWithFacts(t, []factSpec{
		{id: "axiom", domain: "physics"},
		{id: "b", domain: "physics", supports: []string{"axiom"}},
		{id: "c", domain: "physics"}, // CONTRADICTS axiom — added manually below
	})
	// Add the CONTRADICTS relation directly (factSpec only supports SUPPORTS).
	require.NoError(t, k.SetFactRelation(ctx, &types.FactRelation{
		SourceFactId: "c",
		TargetFactId: "axiom",
		Relation:     types.RelationType_RELATION_TYPE_CONTRADICTS,
	}))
	sel := &types.RootedSubtreeSelector{RootFactId: "axiom", MaxDepth: 5}
	nodeIDs, _, err := k.GatherRootedSubtree(ctx, sel)
	require.NoError(t, err)
	// c must be excluded because CONTRADICTS is not a support-bearing relation.
	require.Equal(t, []string{"axiom", "b"}, nodeIDs)
}

func TestGatherAncestorCone_LinearChain(t *testing.T) {
	k, ctx := setupKnowledgeWithFacts(t, []factSpec{
		{id: "axiom", domain: "physics"},
		{id: "b", domain: "physics", supports: []string{"axiom"}},
		{id: "c", domain: "physics", supports: []string{"b"}},
	})
	sel := &types.AncestorConeSelector{LeafFactId: "c", MaxDepth: 5, MaxPaths: 10}
	nodeIDs, edges, err := k.GatherAncestorCone(ctx, sel)
	require.NoError(t, err)
	require.Equal(t, []string{"axiom", "b", "c"}, nodeIDs)
	require.Len(t, edges, 2)
	// sortToKEdges sorts by FromFactId: "b" < "c" lexicographically
	// edges[0]: b→axiom, edges[1]: c→b
	require.Equal(t, "b", edges[0].FromFactId)
	require.Equal(t, "axiom", edges[0].ToFactId)
	require.Equal(t, "c", edges[1].FromFactId)
	require.Equal(t, "b", edges[1].ToFactId)
}

func TestGatherAncestorCone_MaxPathsCapEnforced(t *testing.T) {
	// Build a chain longer than maxPaths: axiom → b → c → d → e (each supports the prior).
	// Relations: e SUPPORTS d, d SUPPORTS c, c SUPPORTS b, b SUPPORTS axiom.
	// Traversal from leaf "e" follows outgoing edges: e→d, d→c, c→b, b→axiom — 4 edges total.
	// With MaxPaths=2, the traversal must stop after recording exactly 2 edges.
	k, ctx := setupKnowledgeWithFacts(t, []factSpec{
		{id: "axiom", domain: "physics"},
		{id: "b", domain: "physics", supports: []string{"axiom"}},
		{id: "c", domain: "physics", supports: []string{"b"}},
		{id: "d", domain: "physics", supports: []string{"c"}},
		{id: "e", domain: "physics", supports: []string{"d"}},
	})
	sel := &types.AncestorConeSelector{LeafFactId: "e", MaxDepth: 10, MaxPaths: 2}
	nodeIDs, edges, err := k.GatherAncestorCone(ctx, sel)
	require.NoError(t, err)
	// Exactly 2 edges must be recorded (cap enforced).
	require.Len(t, edges, 2)
	// The leaf "e" is always in visited; 2 edges means 2 targets were added,
	// so visited = {e, d, c} — exactly 3 nodes.
	require.Len(t, nodeIDs, 3)
}

// ─── GatherFrontier tests ─────────────────────────────────────────────────────

func TestGatherFrontier_DomainScoped(t *testing.T) {
	k, ctx, _, _ := setupKnowledgeTestFull(t)

	// Store old facts with VerifiedAtBlock = 100.
	require.NoError(t, k.SetFact(ctx, &types.Fact{
		Id: "old1", Domain: "physics",
		Status: types.FactStatus_FACT_STATUS_VERIFIED, VerifiedAtBlock: 100,
	}))
	require.NoError(t, k.SetFact(ctx, &types.Fact{
		Id: "old2", Domain: "physics",
		Status: types.FactStatus_FACT_STATUS_VERIFIED, VerifiedAtBlock: 100,
	}))
	// Add a recent fact with VerifiedAtBlock = 200.
	require.NoError(t, k.SetFact(ctx, &types.Fact{
		Id: "new1", Domain: "physics",
		Status: types.FactStatus_FACT_STATUS_VERIFIED, VerifiedAtBlock: 200,
	}))

	sel := &types.FrontierSelector{Domain: "physics", SinceBlock: 150, Limit: 100}
	nodeIDs, _, err := k.GatherFrontier(ctx, sel)
	require.NoError(t, err)
	require.Equal(t, []string{"new1"}, nodeIDs)
}

func TestGatherFrontier_LimitCapped(t *testing.T) {
	k, ctx, _, _ := setupKnowledgeTestFull(t)

	for i := 0; i < 5; i++ {
		id := fmt.Sprintf("fact%d", i)
		require.NoError(t, k.SetFact(ctx, &types.Fact{
			Id: id, Domain: "math",
			Status: types.FactStatus_FACT_STATUS_VERIFIED, VerifiedAtBlock: 200,
		}))
	}

	sel := &types.FrontierSelector{Domain: "math", SinceBlock: 100, Limit: 3}
	nodeIDs, _, err := k.GatherFrontier(ctx, sel)
	require.NoError(t, err)
	require.Len(t, nodeIDs, 3)
}

func TestGatherFrontier_InterSetEdgesIncluded(t *testing.T) {
	k, ctx, _, _ := setupKnowledgeTestFull(t)

	// Two recent facts in same domain with a relation between them.
	require.NoError(t, k.SetFact(ctx, &types.Fact{
		Id: "f1", Domain: "bio",
		Status: types.FactStatus_FACT_STATUS_VERIFIED, VerifiedAtBlock: 300,
	}))
	require.NoError(t, k.SetFact(ctx, &types.Fact{
		Id: "f2", Domain: "bio",
		Status: types.FactStatus_FACT_STATUS_VERIFIED, VerifiedAtBlock: 300,
	}))
	require.NoError(t, k.SetFactRelation(ctx, &types.FactRelation{
		SourceFactId: "f2",
		TargetFactId: "f1",
		Relation:     types.RelationType_RELATION_TYPE_SUPPORTS,
	}))

	sel := &types.FrontierSelector{Domain: "bio", SinceBlock: 200, Limit: 100}
	nodeIDs, edges, err := k.GatherFrontier(ctx, sel)
	require.NoError(t, err)
	require.Equal(t, []string{"f1", "f2"}, nodeIDs)
	require.Len(t, edges, 1)
	require.Equal(t, "f2", edges[0].FromFactId)
	require.Equal(t, "f1", edges[0].ToFactId)
}

func TestGatherFrontier_ExcludesUnverifiedFacts(t *testing.T) {
	k, ctx, _, _ := setupKnowledgeTestFull(t)

	// One verified fact and one unverified fact in the same domain.
	require.NoError(t, k.SetFact(ctx, &types.Fact{
		Id: "verified1", Domain: "science",
		Status: types.FactStatus_FACT_STATUS_VERIFIED, VerifiedAtBlock: 100,
	}))
	require.NoError(t, k.SetFact(ctx, &types.Fact{
		Id: "unverified1", Domain: "science",
		Status: types.FactStatus_FACT_STATUS_PENDING, VerifiedAtBlock: 0,
	}))

	// SinceBlock=0: this is the case where the old filter (VerifiedAtBlock < 0)
	// was never true for uint64, causing unverified facts to leak into results.
	sel := &types.FrontierSelector{Domain: "science", SinceBlock: 0, Limit: 10}
	nodeIDs, _, err := k.GatherFrontier(ctx, sel)
	require.NoError(t, err)
	require.Equal(t, []string{"verified1"}, nodeIDs, "unverified1 (VerifiedAtBlock==0) must be excluded")
}

func TestGatherFrontier_ExcludesDifferentDomain(t *testing.T) {
	k, ctx, _, _ := setupKnowledgeTestFull(t)

	require.NoError(t, k.SetFact(ctx, &types.Fact{
		Id: "phys1", Domain: "physics",
		Status: types.FactStatus_FACT_STATUS_VERIFIED, VerifiedAtBlock: 300,
	}))
	require.NoError(t, k.SetFact(ctx, &types.Fact{
		Id: "chem1", Domain: "chemistry",
		Status: types.FactStatus_FACT_STATUS_VERIFIED, VerifiedAtBlock: 300,
	}))

	sel := &types.FrontierSelector{Domain: "physics", SinceBlock: 100, Limit: 100}
	nodeIDs, _, err := k.GatherFrontier(ctx, sel)
	require.NoError(t, err)
	require.Equal(t, []string{"phys1"}, nodeIDs)
}

// ─── ComputeToKSnapshotRoot tests ────────────────────────────────────────────

func TestComputeToKSnapshotRoot_Deterministic(t *testing.T) {
	nodeIDs := []string{"a", "b", "c"}
	edges := []*types.ToKEdge{
		{FromFactId: "b", ToFactId: "a", Relation: "SUPPORTS"},
		{FromFactId: "c", ToFactId: "b", Relation: "SUPPORTS"},
	}
	r1 := keeper.ComputeToKSnapshotRoot(nodeIDs, edges)
	r2 := keeper.ComputeToKSnapshotRoot(nodeIDs, edges)
	require.Equal(t, r1, r2)
	require.Len(t, r1, 32) // 32-byte sha256
}

func TestComputeToKSnapshotRoot_DomainSeparated(t *testing.T) {
	// Same IDs, but one set is "TOK_NODES" tagged and another is "TOK_EDGES" tagged.
	// Roots must differ — domain tags prevent set-swap collisions.
	r := keeper.ComputeToKSnapshotRoot([]string{"a", "b"}, nil)
	r2 := keeper.ComputeToKSnapshotRoot(nil, []*types.ToKEdge{
		{FromFactId: "a", ToFactId: "b", Relation: "SUPPORTS"},
	})
	require.NotEqual(t, r, r2)
}

func TestComputeToKSnapshotRoot_NoCanonCollision(t *testing.T) {
	// Two semantically-distinct edge sets that would collide under naive
	// pipe-concatenation. After the field-level length-prefix fix they
	// must produce different roots.
	setA := []*types.ToKEdge{{FromFactId: "a|b", ToFactId: "c", Relation: "X"}}
	setB := []*types.ToKEdge{{FromFactId: "a", ToFactId: "b|c", Relation: "X"}}
	rA := keeper.ComputeToKSnapshotRoot(nil, setA)
	rB := keeper.ComputeToKSnapshotRoot(nil, setB)
	require.NotEqual(t, rA, rB, "pipe in field must not collide with field separator")
}

// ─── AssembleToKBundle tests ─────────────────────────────────────────────────

func TestAssembleToKBundle_RootedSubtree(t *testing.T) {
	k, ctx := setupKnowledgeWithFacts(t, []factSpec{
		{id: "axiom", domain: "physics"},
		{id: "b", domain: "physics", supports: []string{"axiom"}},
	})
	sel := &types.ToKSelector{Variant: &types.ToKSelector_RootedSubtree{
		RootedSubtree: &types.RootedSubtreeSelector{RootFactId: "axiom", MaxDepth: 5},
	}}
	bundle, err := k.AssembleToKBundle(ctx, sel, 0 /* current block */)
	require.NoError(t, err)
	require.NotEmpty(t, bundle.SnapshotRoot)
	require.Equal(t, []string{"axiom", "b"}, bundle.IncludedNodeIds)
	require.Len(t, bundle.IncludedEdges, 1)
	require.Len(t, bundle.Nodes, 2)
	// Re-derivability: re-compute root from IDs — must match.
	rederived := keeper.ComputeToKSnapshotRoot(bundle.IncludedNodeIds, bundle.IncludedEdges)
	require.Equal(t, bundle.SnapshotRoot, rederived)
}

// ─── helpers ─────────────────────────────────────────────────────────────────

type factSpec struct {
	id       string
	domain   string
	supports []string // predecessor IDs (this fact SUPPORTS those facts)
}

func setupKnowledgeWithFacts(t *testing.T, specs []factSpec) (*keeper.Keeper, sdk.Context) {
	t.Helper()
	k, ctx, _, _ := setupKnowledgeTestFull(t)
	for _, s := range specs {
		require.NoError(t, k.SetFact(ctx, &types.Fact{
			Id:     s.id,
			Domain: s.domain,
			Status: types.FactStatus_FACT_STATUS_VERIFIED,
		}))
	}
	for _, s := range specs {
		for _, parent := range s.supports {
			require.NoError(t, k.SetFactRelation(ctx, &types.FactRelation{
				SourceFactId: s.id,
				TargetFactId: parent,
				Relation:     types.RelationType_RELATION_TYPE_SUPPORTS,
			}))
		}
	}
	return &k, ctx
}
