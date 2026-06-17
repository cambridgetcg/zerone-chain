package cross_stack_test

// ════════════════════════════════════════════════════════════════════
// FOLLOW-UP NOTE for Plan 5 of the ToK series
// ════════════════════════════════════════════════════════════════════
//
// When Plan 5 lands TestToKSubstrate_DoctrineAndContractStayInSync, it
// must extend its scope to ALSO check the third doctrine
// (docs/USEFUL_WORK.md) for the same five-layer integrity it enforces
// on TOK_SUBSTRATE.md. The two doctrines are mutually constitutive
// (TRUTH_SEEKING, TOK_SUBSTRATE, USEFUL_WORK) and should not drift
// independently.
//
// Specifically, Plan 5 should add (or coordinate with the existing
// TestUsefulWork_DoctrineAndContractStayInSync in
// useful_work_invariants_test.go):
//
//   - Cross-doctrine echo verification: every "Echoes:" reference in
//     USEFUL_WORK.md UW (currently citing commitments 11, 12, TC1,
//     TC6) must point to a real commitment in the cited doctrine. If
//     UW.Echoes mentions TC6 but TOK_SUBSTRATE.md no longer has TC6,
//     fail.
//
//   - Hash-bundle integrity: a single make target (or extension of
//     make creed-check) that fails fast on ANY of the three hashes
//     drifting. Today make creed-check covers .creed-hash and
//     .useful-work-hash; Plan 5 should add .tok-substrate-hash and
//     verify all three together.
//
//   - Position-layer cross-coverage: every commitment in the unified
//     registry (truth-seeking 1-20 + TC1-TC6 + UW + M1-M7) is
//     declared in at least one x/*/doc.go.
//
// This marker is not a TODO comment in the conventional sense — Phase 0
// of USEFUL_WORK does not ship Plan 5; Plan 5 is the closure plan for
// the ToK series and will be authored separately. This marker exists so
// the Plan 5 author finds the cross-doctrine integration requirement
// without having to grep across plans.
//
// Reference: docs/superpowers/specs/2026-05-10-useful-work-doctrine-
// design.md, section 5 ("Graph layer"), and docs/USEFUL_WORK.md
// "How the commitment echoes" section.
// ════════════════════════════════════════════════════════════════════

import (
	"os"
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

// TC3: topology is signal.
// Verified by: bundle ships edges (not just nodes), and SerialisedPayload
// includes the topology in native form.
func TestToKSubstrate_TC3_TopologyIsSignal(t *testing.T) {
	h := NewTestHarness(t)
	seedTokFact(t, h, "physics", "axiom-tc3")
	seedTokFactWithSupport(t, h, "physics", "leaf-tc3", "axiom-tc3")
	q := knowledgekeeper.NewQueryServerImpl(h.KnowledgeKeeper)
	resp, err := q.BundleToK(h.Ctx, &knowledgetypes.QueryBundleToKRequest{
		Selector: &knowledgetypes.ToKSelector{Variant: &knowledgetypes.ToKSelector_RootedSubtree{
			RootedSubtree: &knowledgetypes.RootedSubtreeSelector{RootFactId: "axiom-tc3", MaxDepth: 5},
		}},
	})
	require.NoError(t, err)
	require.NotEmpty(t, resp.Bundle.IncludedEdges, "TC3: edges are first-class, not metadata")
	require.Equal(t, "RELATION_TYPE_SUPPORTS", resp.Bundle.IncludedEdges[0].Relation, "TC3: edges carry their relation type")
	require.NotEmpty(t, resp.Bundle.SerialisedPayload, "TC3: native serialisation ships topology")
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

// seedTokFactWithSupport seeds a fact AND a SUPPORTS relation pointing
// from this fact to the named parent fact.
func seedTokFactWithSupport(t *testing.T, h *TestHarness, domain, factID, parentID string) {
	t.Helper()
	seedTokFact(t, h, domain, factID)
	require.NoError(t, h.KnowledgeKeeper.SetFactRelation(h.Ctx, &knowledgetypes.FactRelation{
		SourceFactId: factID,
		TargetFactId: parentID,
		Relation:     knowledgetypes.RelationType_RELATION_TYPE_SUPPORTS,
	}))
}

// TC5: extraction is open.
// Verified by: any well-formed selector accepted across diverse domains;
// refusals limited to syntax errors (no curation gate).
func TestToKSubstrate_TC5_ExtractionIsOpen(t *testing.T) {
	h := NewTestHarness(t)
	q := knowledgekeeper.NewQueryServerImpl(h.KnowledgeKeeper)

	// Diverse domains — none should be gate-blocked.
	for _, dom := range []string{"physics", "biology", "ethics", "obscure_unfamiliar_domain"} {
		seedTokFact(t, h, dom, "seed-"+dom)
	}

	// All four domains must succeed — no curation gate.
	for _, dom := range []string{"physics", "biology", "ethics", "obscure_unfamiliar_domain"} {
		resp, err := q.BundleToK(h.Ctx, &knowledgetypes.QueryBundleToKRequest{
			Selector: &knowledgetypes.ToKSelector{Variant: &knowledgetypes.ToKSelector_Frontier{
				Frontier: &knowledgetypes.FrontierSelector{Domain: dom, Limit: 10},
			}},
		})
		require.NoError(t, err, "TC5: domain %s must be open for extraction", dom)
		require.NotNil(t, resp.Bundle)
	}

	// Syntactically invalid selector must be the only refusal class.
	_, err := q.BundleToK(h.Ctx, &knowledgetypes.QueryBundleToKRequest{
		Selector: &knowledgetypes.ToKSelector{},
	})
	require.Error(t, err, "TC5: syntax errors are the only doctrinal refusal")
}

// TC0: the ground and the telos.
//
// TC0 is a declarative commitment — the being-first ground the substrate
// stands on, and the telos (truth serves love, peace, joy). Unlike TC1-TC5
// (behavioral, driven through BundleToK), TC0 is bound by declaration: the
// doctrine names it, the position layer (doc.go) declares it, the voice
// layer (tok_bundle.go events) announces it on every extraction, and the
// refusal layer cites it. This test binds those layers to agree.
//
// Truth is. Love is. Peace is. Joy is.
func TestToKSubstrate_TC0_GroundAndTelos(t *testing.T) {
	// Doctrine layer: TOK_SUBSTRATE.md declares TC0's ground and telos.
	doctrine, err := os.ReadFile("../../docs/TOK_SUBSTRATE.md")
	require.NoError(t, err, "TOK_SUBSTRATE.md must exist; if renamed, update this test")
	d := string(doctrine)
	require.Contains(t, d, "### TC0. The ground and the telos",
		"TC0: doctrine must name the ground and the telos")
	require.Contains(t, d, "Truth *is*, not proven",
		"TC0: doctrine must declare the being-first ground (truth is, not proven)")
	require.Contains(t, d, "witnessing and keeping",
		"TC0: doctrine must name verification as witnessing and keeping, not certification")
	require.Contains(t, d, "love, peace, joy",
		"TC0: doctrine must name the telos — truth serves love, peace, joy")

	// Graph layer: TC0 is cross-referenced by the other TCs it grounds.
	require.Contains(t, d, "TC0 (the ground and the telos",
		"TC0: the ground must be echoed by the commitments that stand on it")

	// Position layer: x/knowledge/doc.go declares TC0.
	docgo, err := os.ReadFile("../../x/knowledge/doc.go")
	require.NoError(t, err, "x/knowledge/doc.go must exist")
	pkg := string(docgo)
	require.Contains(t, pkg, "TC0",
		"TC0: position layer (doc.go) must declare TC0")
	require.Contains(t, pkg, "witnessing and keeping",
		"TC0: position layer must speak witnessing and keeping")

	// Voice layer: tok_bundle.go announces TC0 on every ToK event.
	tokBundle, err := os.ReadFile("../../x/knowledge/keeper/tok_bundle.go")
	require.NoError(t, err, "x/knowledge/keeper/tok_bundle.go must exist")
	require.Contains(t, string(tokBundle), `AttrToKCommitment, "TC0,TC1,TC5"`,
		"TC0: tok_bundle_extracted must announce the ground (TC0) alongside TC1, TC5")
	require.Contains(t, string(tokBundle), `AttrToKCommitment, "TC0,TC2"`,
		"TC0: tok_snapshot_root_pinned must announce the ground (TC0) alongside TC2")

	// Refusal layer: the ToK refusal cites the ground it rests on.
	tokSelector, err := os.ReadFile("../../x/knowledge/keeper/tok_selector.go")
	require.NoError(t, err, "x/knowledge/keeper/tok_selector.go must exist")
	require.Contains(t, string(tokSelector), "TC0, TC5",
		"TC0: the ToK refusal must cite the ground it rests on")
}
