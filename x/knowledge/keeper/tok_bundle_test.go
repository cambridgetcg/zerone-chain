package keeper_test

import (
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
