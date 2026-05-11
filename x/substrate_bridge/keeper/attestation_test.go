package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/substrate_bridge/types"
)

func TestAttestation_WriteGet(t *testing.T) {
	k, ctx := setupSubstrateBridgeKeeper(t)
	att := &types.ExternalAttestation{
		AttestationId: "att-1", AdapterId: "wiki-v1", Submitter: "zrn1xxx",
		Status: types.AttestationStatus_ATTESTATION_STATUS_SUBMITTED, SubmittedAtBlock: 100,
	}
	require.NoError(t, k.WriteAttestation(ctx, att))
	got, found := k.GetAttestation(ctx, "att-1")
	require.True(t, found)
	require.Equal(t, att.AttestationId, got.AttestationId)
}

func TestAttestation_StatusIndex(t *testing.T) {
	k, ctx := setupSubstrateBridgeKeeper(t)
	require.NoError(t, k.WriteAttestation(ctx, &types.ExternalAttestation{
		AttestationId: "a", Status: types.AttestationStatus_ATTESTATION_STATUS_AWAITING_RESOLUTION,
	}))
	require.NoError(t, k.WriteAttestation(ctx, &types.ExternalAttestation{
		AttestationId: "b", Status: types.AttestationStatus_ATTESTATION_STATUS_AWAITING_RESOLUTION,
	}))
	require.NoError(t, k.WriteAttestation(ctx, &types.ExternalAttestation{
		AttestationId: "c", Status: types.AttestationStatus_ATTESTATION_STATUS_SETTLED,
	}))

	var awaiting []string
	k.IterateAttestationsByStatus(ctx, types.AttestationStatus_ATTESTATION_STATUS_AWAITING_RESOLUTION, func(id string) bool {
		awaiting = append(awaiting, id); return false
	})
	require.Len(t, awaiting, 2)
}

func TestAttestation_TransitionDeletesOldIndex(t *testing.T) {
	k, ctx := setupSubstrateBridgeKeeper(t)
	require.NoError(t, k.WriteAttestation(ctx, &types.ExternalAttestation{
		AttestationId: "a", Status: types.AttestationStatus_ATTESTATION_STATUS_AWAITING_RESOLUTION,
	}))
	att, _ := k.GetAttestation(ctx, "a")
	att.Status = types.AttestationStatus_ATTESTATION_STATUS_SETTLED
	require.NoError(t, k.WriteAttestation(ctx, att))
	var awaiting []string
	k.IterateAttestationsByStatus(ctx, types.AttestationStatus_ATTESTATION_STATUS_AWAITING_RESOLUTION, func(id string) bool {
		awaiting = append(awaiting, id); return false
	})
	require.Empty(t, awaiting)
}

func TestAttestation_NextIDMonotonic(t *testing.T) {
	k, ctx := setupSubstrateBridgeKeeper(t)
	id1 := k.NextAttestationID(ctx)
	id2 := k.NextAttestationID(ctx)
	require.NotEqual(t, id1, id2)
}
