package cross_stack_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	knowledgekeeper "github.com/zerone-chain/zerone/x/knowledge/keeper"
	knowledgetypes "github.com/zerone-chain/zerone/x/knowledge/types"
)

// TC1: the graph is the headline.
// Verified by: RouteBCapabilities advertising tok_capabilities, and
// BundleToK accepting and returning a well-formed bundle.
func TestToKSubstrate_TC1_GraphIsTheHeadline(t *testing.T) {
	h := NewTestHarness(t)

	// Capability advertisement.
	q := knowledgekeeper.NewQueryServerImpl(h.KnowledgeKeeper)
	caps, err := q.RouteBCapabilities(h.Ctx, &knowledgetypes.QueryRouteBCapabilitiesRequest{})
	require.NoError(t, err)
	require.NotNil(t, caps.TokCapabilities, "TC1: tok_capabilities must be advertised")
	require.Contains(t, caps.TokCapabilities.SupportedSelectors, "rooted_subtree")

	// Headline endpoint roundtrip.
	seedTokFact(t, h, "physics", "axiom-tc1")
	resp, err := q.BundleToK(h.Ctx, &knowledgetypes.QueryBundleToKRequest{
		Selector: &knowledgetypes.ToKSelector{Variant: &knowledgetypes.ToKSelector_RootedSubtree{
			RootedSubtree: &knowledgetypes.RootedSubtreeSelector{RootFactId: "axiom-tc1", MaxDepth: 1},
		}},
	})
	require.NoError(t, err)
	require.NotNil(t, resp.Bundle, "TC1: BundleToK is the headline; it must return a graph bundle")
	require.NotEmpty(t, resp.Bundle.SnapshotRoot)
}

// TC2: every view is graph-pinned.
// Verified by: bundle ships snapshot_root + snapshot_block, and the
// root is re-derivable from IDs alone (trust-minimised verification).
func TestToKSubstrate_TC2_EveryViewIsGraphPinned(t *testing.T) {
	h := NewTestHarness(t)
	seedTokFact(t, h, "physics", "axiom-tc2")
	q := knowledgekeeper.NewQueryServerImpl(h.KnowledgeKeeper)
	resp, err := q.BundleToK(h.Ctx, &knowledgetypes.QueryBundleToKRequest{
		Selector: &knowledgetypes.ToKSelector{Variant: &knowledgetypes.ToKSelector_RootedSubtree{
			RootedSubtree: &knowledgetypes.RootedSubtreeSelector{RootFactId: "axiom-tc2", MaxDepth: 1},
		}},
	})
	require.NoError(t, err)
	require.Len(t, resp.Bundle.SnapshotRoot, 32, "TC2: snapshot root must be 32 bytes")
	require.Greater(t, resp.Bundle.SnapshotBlock, uint64(0), "TC2: snapshot_block must be set")
	// Re-derivability — trust-minimised verification.
	rederived := knowledgekeeper.ComputeToKSnapshotRoot(resp.Bundle.IncludedNodeIds, resp.Bundle.IncludedEdges)
	require.Equal(t, resp.Bundle.SnapshotRoot, rederived, "TC2: root must be re-derivable from IDs")
}

// seedTokFact registers a fact + its domain so it can be bundled.
func seedTokFact(t *testing.T, h *TestHarness, domain, factID string) {
	t.Helper()
	require.NoError(t, h.KnowledgeKeeper.SetDomain(h.Ctx, &knowledgetypes.Domain{
		Name:   domain,
		Status: knowledgetypes.DomainStatus_DOMAIN_STATUS_ACTIVE,
	}))
	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, &knowledgetypes.Fact{
		Id:              factID,
		Domain:          domain,
		Status:          knowledgetypes.FactStatus_FACT_STATUS_VERIFIED,
		VerifiedAtBlock: 1,
	}))
}
