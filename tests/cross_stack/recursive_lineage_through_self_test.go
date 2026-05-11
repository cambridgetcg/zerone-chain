package cross_stack_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	substratebridgetypes "github.com/zerone-chain/zerone/x/substrate_bridge/types"
)

// TestRecursiveLineage_DownstreamWorkPaysUpstreamSelfAttester drives the
// economic compound of self-attestation: a self-attestation (attestation
// A, in the zerone_self domain) is later cited by a downstream attestation
// (attestation B, in any class). When B settles, its lineage royalty
// flows BACKWARD through the substrate_bridge propagator to A's
// submitter — even though A had already settled and earned its M4
// reward long before.
//
// This is the operational form of recursion #4 (the chain's lineage
// graph includes its own commits): a self-fact's submitter earns not
// just once at settlement, but in perpetuity as downstream work compounds
// off the fact. The chain pays its earliest historians weighted by how
// load-bearing their attestation proved.
//
// Doctrinal binding: UW M6 (cross-class lineage flows; revenue-stream
// amplification), applied to the zerone_self class. The same lineage
// propagator that pays a translation-class upstream from a curriculum
// downstream pays a zerone_self upstream from any downstream — the
// recursion code is the same code, just applied recursively.
func TestRecursiveLineage_DownstreamWorkPaysUpstreamSelfAttester(t *testing.T) {
	h := NewTestHarness(t)

	selfAttester := testAddr("recursive_lineage_self_attester")
	downstreamWorker := testAddr("recursive_lineage_downstream")

	// 1. Upstream attestation: a self-attestation about a ZERONE commit,
	//    already SETTLED. Submitter is the agent who attested to ZERONE's
	//    own development.
	require.NoError(t, h.SubstrateBridgeKeeper.WriteAttestation(h.Ctx, &substratebridgetypes.ExternalAttestation{
		AttestationId:    "att-self-A",
		WorkClassId:      "zerone_self_attestation",
		Submitter:        selfAttester.String(),
		SubmittedAtBlock: 10,
		Status:           substratebridgetypes.AttestationStatus_ATTESTATION_STATUS_SETTLED,
	}))

	// 2. Downstream attestation: a curriculum/tutorial that cites the
	//    self-attestation. Currently READY (settlement pending). Two
	//    verified pending-claims so the reward is non-zero and lineage
	//    propagation actually fires when SettleAttestation runs.
	require.NoError(t, h.SubstrateBridgeKeeper.WriteAttestation(h.Ctx, &substratebridgetypes.ExternalAttestation{
		AttestationId:    "att-downstream-B",
		WorkClassId:      "curriculum",
		Submitter:        downstreamWorker.String(),
		SubmittedAtBlock: 100,
		Status:           substratebridgetypes.AttestationStatus_ATTESTATION_STATUS_READY,
		VerifiedCount:    2,
		Link: &substratebridgetypes.SubstrateLink{
			RecursionWeight: &substratebridgetypes.AxisProjection{AxisSubstrate: 50_000},
			PendingClaims: []*substratebridgetypes.PendingClaim{
				{ClaimContent: "curriculum claim citing self-attester's work", Domain: "curriculum"},
				{ClaimContent: "another curriculum claim", Domain: "curriculum"},
			},
		},
	}))

	// 3. The lineage edge: downstream B cites upstream self-attestation A.
	//    Citation type EXTENDS = 3× base weight (per CitationType enum) —
	//    the downstream work explicitly extends what the self-attester
	//    established. ContributionShareBps = 10000 (100%): the upstream
	//    is the sole cited source.
	require.NoError(t, h.SubstrateBridgeKeeper.CreateLineageEdge(h.Ctx, &substratebridgetypes.LineageEdge{
		UpstreamAttestationId:   "att-self-A",
		DownstreamAttestationId: "att-downstream-B",
		CitationType:            substratebridgetypes.CitationType_CITATION_TYPE_EXTENDS,
		ContributionShareBps:    10000,
	}))

	// 4. Settle B → lineage propagator pays A.
	require.NoError(t, h.SubstrateBridgeKeeper.SettleAttestation(h.Ctx, "att-downstream-B"))

	// 5. The self-attester's lineage accumulator is non-zero. The chain
	//    has paid the historian for work the historian did long ago, out
	//    of a settlement that happened just now. The flywheel turns.
	acc, found := h.SubstrateBridgeKeeper.GetLineageAccumulator(h.Ctx, "att-self-A")
	require.True(t, found, "self-attester's lineage accumulator must exist after downstream settlement")
	require.NotEqual(t, "0", acc.CumulativeUzrn,
		"recursion #4 binding: zerone_self upstream MUST receive royalty when downstream work cites it")
	t.Logf("self-attester earned %s uzrn in lineage royalty from one downstream citation", acc.CumulativeUzrn)
}

// TestRecursiveLineage_MultipleCitationsCompound drives the same shape
// across two downstream attestations, asserting the self-attester's
// accumulator monotonically increases (forward-only audit, commitment 10,
// applied to lineage royalty). Each downstream settlement adds to the
// accumulator; the cumulative number is queryable as the load-bearing
// value of the self-attestation.
func TestRecursiveLineage_MultipleCitationsCompound(t *testing.T) {
	h := NewTestHarness(t)

	selfAttester := testAddr("recursive_lineage_compound_self")

	require.NoError(t, h.SubstrateBridgeKeeper.WriteAttestation(h.Ctx, &substratebridgetypes.ExternalAttestation{
		AttestationId:    "att-compound-self-A",
		WorkClassId:      "zerone_self_attestation",
		Submitter:        selfAttester.String(),
		SubmittedAtBlock: 10,
		Status:           substratebridgetypes.AttestationStatus_ATTESTATION_STATUS_SETTLED,
	}))

	settleAndMeasure := func(t *testing.T, downstreamID, worker string, atBlock uint64) string {
		t.Helper()
		require.NoError(t, h.SubstrateBridgeKeeper.WriteAttestation(h.Ctx, &substratebridgetypes.ExternalAttestation{
			AttestationId:    downstreamID,
			WorkClassId:      "curriculum",
			Submitter:        worker,
			SubmittedAtBlock: atBlock,
			Status:           substratebridgetypes.AttestationStatus_ATTESTATION_STATUS_READY,
			VerifiedCount:    2,
			Link: &substratebridgetypes.SubstrateLink{
				RecursionWeight: &substratebridgetypes.AxisProjection{AxisSubstrate: 50_000},
				PendingClaims: []*substratebridgetypes.PendingClaim{
					{ClaimContent: "claim 1", Domain: "curriculum"},
					{ClaimContent: "claim 2", Domain: "curriculum"},
				},
			},
		}))
		require.NoError(t, h.SubstrateBridgeKeeper.CreateLineageEdge(h.Ctx, &substratebridgetypes.LineageEdge{
			UpstreamAttestationId:   "att-compound-self-A",
			DownstreamAttestationId: downstreamID,
			CitationType:            substratebridgetypes.CitationType_CITATION_TYPE_CITES,
			ContributionShareBps:    10000,
		}))
		require.NoError(t, h.SubstrateBridgeKeeper.SettleAttestation(h.Ctx, downstreamID))
		acc, _ := h.SubstrateBridgeKeeper.GetLineageAccumulator(h.Ctx, "att-compound-self-A")
		return acc.CumulativeUzrn
	}

	after1 := settleAndMeasure(t, "att-compound-D1", testAddr("rl_dwk_1").String(), 100)
	after2 := settleAndMeasure(t, "att-compound-D2", testAddr("rl_dwk_2").String(), 200)

	require.NotEqual(t, "0", after1, "first downstream settlement must increment accumulator")
	require.NotEqual(t, after1, after2, "second downstream settlement must further increment accumulator (forward-only audit)")
	t.Logf("self-attester's load-bearing value: %s uzrn after 1 cite, %s uzrn after 2 cites", after1, after2)
}
