package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/substrate_bridge/types"
)

// TestBeginBlocker_TimeoutTransitionsToPartial verifies that an
// AWAITING_RESOLUTION attestation older than MaxPendingWindowBlocks is
// settled by BeginBlocker. With some verified and some rejected (below the
// rejection-threshold), the outcome is PARTIAL.
func TestBeginBlocker_TimeoutTransitionsToPartial(t *testing.T) {
	k, ctx := setupSubstrateBridgeKeeperWithBank(t)
	require.NoError(t, k.WriteAdapter(ctx, &types.AdapterRegistration{
		AdapterId: "wiki-v1", Status: types.AdapterStatus_ADAPTER_STATUS_ACTIVE,
	}))

	// Attestation submitted at block 0, with 4 pending claims.
	// VerifiedCount=3, RejectedCount=1 → rejectionRatioBps = 1*10000/4 = 2500 < 5000 (threshold).
	// verifiedNumerator = 3 (no cited_facts), totalCount = 4, verifiedRatioBps = 7500 >= 1000.
	// RejectedCount > 0 → outcome is PARTIAL (not SETTLED, not REJECTED).
	att := &types.ExternalAttestation{
		AttestationId:    "old-att",
		AdapterId:        "wiki-v1",
		Status:           types.AttestationStatus_ATTESTATION_STATUS_AWAITING_RESOLUTION,
		SubmittedAtBlock: 0,
		Link: &types.SubstrateLink{
			PendingClaims: []*types.PendingClaim{{}, {}, {}, {}},
		},
		VerifiedCount: 3,
		RejectedCount: 1,
	}
	require.NoError(t, k.WriteAttestation(ctx, att))

	// Move chain past timeout window (default 6_220_800 blocks).
	ctx = ctx.WithBlockHeight(7_000_000)

	require.NoError(t, k.BeginBlocker(ctx))

	settled, found := k.GetAttestation(ctx, "old-att")
	require.True(t, found)
	// 25% rejected < 50% threshold and > 0 rejections → PARTIAL.
	require.Equal(t, types.AttestationStatus_ATTESTATION_STATUS_PARTIAL, settled.Status)
}

// TestBeginBlocker_NoTimeoutBeforeWindow verifies that an attestation NOT
// past its timeout window is left untouched.
func TestBeginBlocker_NoTimeoutBeforeWindow(t *testing.T) {
	k, ctx := setupSubstrateBridgeKeeperWithBank(t)
	require.NoError(t, k.WriteAdapter(ctx, &types.AdapterRegistration{
		AdapterId: "wiki-v1", Status: types.AdapterStatus_ADAPTER_STATUS_ACTIVE,
	}))

	att := &types.ExternalAttestation{
		AttestationId:    "fresh-att",
		AdapterId:        "wiki-v1",
		Status:           types.AttestationStatus_ATTESTATION_STATUS_AWAITING_RESOLUTION,
		SubmittedAtBlock: 100,
		Link: &types.SubstrateLink{
			PendingClaims: []*types.PendingClaim{{}, {}},
		},
		VerifiedCount: 0,
		RejectedCount: 0,
	}
	require.NoError(t, k.WriteAttestation(ctx, att))

	// Only at block 200 — well within the 6.2M window.
	ctx = ctx.WithBlockHeight(200)

	require.NoError(t, k.BeginBlocker(ctx))

	untouched, found := k.GetAttestation(ctx, "fresh-att")
	require.True(t, found)
	require.Equal(t, types.AttestationStatus_ATTESTATION_STATUS_AWAITING_RESOLUTION, untouched.Status)
}

// TestBeginBlocker_DrainsReadyQueue verifies that READY attestations are
// settled by BeginBlocker.
func TestBeginBlocker_DrainsReadyQueue(t *testing.T) {
	k, ctx := setupSubstrateBridgeKeeperWithBank(t)

	att := &types.ExternalAttestation{
		AttestationId: "ready-att",
		AdapterId:     "wiki-v1",
		Status:        types.AttestationStatus_ATTESTATION_STATUS_READY,
		Submitter:     "cosmos1qypqxpq9qcrsszgszyfpx9q4zct3sxfq0fzduj",
		Link: &types.SubstrateLink{
			PendingClaims: []*types.PendingClaim{{}},
		},
		VerifiedCount: 1,
		RejectedCount: 0,
	}
	require.NoError(t, k.WriteAttestation(ctx, att))

	require.NoError(t, k.BeginBlocker(ctx))

	drained, found := k.GetAttestation(ctx, "ready-att")
	require.True(t, found)
	require.Equal(t, types.AttestationStatus_ATTESTATION_STATUS_SETTLED, drained.Status)
}
