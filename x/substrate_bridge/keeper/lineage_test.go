package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/substrate_bridge/types"
)

func TestLineage_CreateEdgeValid(t *testing.T) {
	k, ctx := setupSubstrateBridgeKeeper(t)

	require.NoError(t, k.WriteAttestation(ctx, &types.ExternalAttestation{
		AttestationId: "att-upstream", Submitter: "alice", SubmittedAtBlock: 10,
		Status: types.AttestationStatus_ATTESTATION_STATUS_SETTLED,
	}))
	require.NoError(t, k.WriteAttestation(ctx, &types.ExternalAttestation{
		AttestationId: "att-downstream", Submitter: "bob", SubmittedAtBlock: 20,
		Status: types.AttestationStatus_ATTESTATION_STATUS_SETTLED,
	}))

	err := k.CreateLineageEdge(ctx, &types.LineageEdge{
		UpstreamAttestationId:   "att-upstream",
		DownstreamAttestationId: "att-downstream",
		CitationType:            types.CitationType_CITATION_TYPE_SUPPORTS,
		ContributionShareBps:    10000,
	})
	require.NoError(t, err)

	edge, found := k.GetLineageEdge(ctx, types.EdgeID("att-upstream", "att-downstream"))
	require.True(t, found)
	require.Equal(t, "att-upstream", edge.UpstreamAttestationId)
}

func TestLineage_RejectsTimestampCycle(t *testing.T) {
	k, ctx := setupSubstrateBridgeKeeper(t)
	require.NoError(t, k.WriteAttestation(ctx, &types.ExternalAttestation{
		AttestationId: "later", SubmittedAtBlock: 30,
	}))
	require.NoError(t, k.WriteAttestation(ctx, &types.ExternalAttestation{
		AttestationId: "earlier", SubmittedAtBlock: 10,
	}))
	// Try to create later→earlier (cycle).
	err := k.CreateLineageEdge(ctx, &types.LineageEdge{
		UpstreamAttestationId:   "later",
		DownstreamAttestationId: "earlier",
		CitationType:            types.CitationType_CITATION_TYPE_SUPPORTS,
		ContributionShareBps:    10000,
	})
	require.ErrorIs(t, err, types.ErrLineageCycle)
}

func TestLineage_ForwardBackwardWalks(t *testing.T) {
	k, ctx := setupSubstrateBridgeKeeper(t)
	require.NoError(t, k.WriteAttestation(ctx, &types.ExternalAttestation{
		AttestationId: "att-upstream", Submitter: "alice", SubmittedAtBlock: 10,
		Status: types.AttestationStatus_ATTESTATION_STATUS_SETTLED,
	}))
	require.NoError(t, k.WriteAttestation(ctx, &types.ExternalAttestation{
		AttestationId: "att-downstream", Submitter: "bob", SubmittedAtBlock: 20,
		Status: types.AttestationStatus_ATTESTATION_STATUS_SETTLED,
	}))
	require.NoError(t, k.CreateLineageEdge(ctx, &types.LineageEdge{
		UpstreamAttestationId:   "att-upstream",
		DownstreamAttestationId: "att-downstream",
		CitationType:            types.CitationType_CITATION_TYPE_SUPPORTS,
		ContributionShareBps:    10000,
	}))

	var forward []*types.LineageEdge
	k.IterateForwardLineage(ctx, "att-upstream", func(e *types.LineageEdge) bool {
		forward = append(forward, e)
		return false
	})
	require.Len(t, forward, 1)
	require.Equal(t, "att-downstream", forward[0].DownstreamAttestationId)

	var backward []*types.LineageEdge
	k.IterateBackwardLineage(ctx, "att-downstream", func(e *types.LineageEdge) bool {
		backward = append(backward, e)
		return false
	})
	require.Len(t, backward, 1)
}

func TestLineage_SelfCitationCap(t *testing.T) {
	k, ctx := setupSubstrateBridgeKeeper(t)
	require.NoError(t, k.WriteAttestation(ctx, &types.ExternalAttestation{
		AttestationId: "att-up", Submitter: "alice", SubmittedAtBlock: 10,
	}))
	require.NoError(t, k.WriteAttestation(ctx, &types.ExternalAttestation{
		AttestationId: "att-down", Submitter: "alice", SubmittedAtBlock: 20,
	}))
	// Same submitter ("alice") + 8000 bps share → exceeds 5000 cap.
	err := k.CreateLineageEdge(ctx, &types.LineageEdge{
		UpstreamAttestationId:   "att-up",
		DownstreamAttestationId: "att-down",
		CitationType:            types.CitationType_CITATION_TYPE_SUPPORTS,
		ContributionShareBps:    8000,
	})
	require.ErrorIs(t, err, types.ErrSelfCitationCapExceeded)
}
