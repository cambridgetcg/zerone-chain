package keeper_test

import (
	"crypto/sha256"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/substrate_bridge/keeper"
	"github.com/zerone-chain/zerone/x/substrate_bridge/types"
)

func TestComputeLinkHash_Deterministic(t *testing.T) {
	link := &types.SubstrateLink{
		AdapterId: "wiki-v1",
		CitedFacts: []*types.FactCitation{{FactId: "fact-1", CitationType: types.CitationType_CITATION_TYPE_SUPPORTS}},
		PendingClaims: []*types.PendingClaim{{ClaimContent: "X is Y", Domain: "history", MethodologyId: "wiki-cite"}},
		RecursionWeight: &types.AxisProjection{AxisSubstrate: 100},
		Source: &types.ExternalSource{SourceId: "Q42", ContentHash: []byte{0x01}},
	}
	h1 := keeper.ComputeLinkHash(link)
	h2 := keeper.ComputeLinkHash(link)
	require.Equal(t, h1, h2)
	require.Len(t, h1, sha256.Size)
}

func TestComputeLinkHash_FieldSensitivity(t *testing.T) {
	a := &types.SubstrateLink{AdapterId: "wiki-v1", CitedFacts: []*types.FactCitation{{FactId: "fact-1"}}}
	b := &types.SubstrateLink{AdapterId: "wiki-v1", CitedFacts: []*types.FactCitation{{FactId: "fact-2"}}}
	require.NotEqual(t, keeper.ComputeLinkHash(a), keeper.ComputeLinkHash(b))
}

func TestValidateLink_AdapterMustExist(t *testing.T) {
	k, ctx := setupSubstrateBridgeKeeper(t)
	link := &types.SubstrateLink{AdapterId: "unregistered"}
	err := k.ValidateLink(ctx, link, types.DefaultParams())
	require.ErrorIs(t, err, types.ErrAdapterNotFound)
}

func TestValidateLink_AdapterMustBeActive(t *testing.T) {
	k, ctx := setupSubstrateBridgeKeeper(t)
	require.NoError(t, k.WriteAdapter(ctx, &types.AdapterRegistration{
		AdapterId: "wiki-v1",
		Status:    types.AdapterStatus_ADAPTER_STATUS_SUSPENDED,
	}))
	err := k.ValidateLink(ctx, &types.SubstrateLink{AdapterId: "wiki-v1"}, types.DefaultParams())
	require.ErrorIs(t, err, types.ErrAdapterNotActive)
}

func TestValidateLink_TooManyPendingClaims(t *testing.T) {
	k, ctx := setupSubstrateBridgeKeeper(t)
	require.NoError(t, k.WriteAdapter(ctx, &types.AdapterRegistration{
		AdapterId: "wiki-v1", Status: types.AdapterStatus_ADAPTER_STATUS_ACTIVE,
	}))
	p := types.DefaultParams()
	p.MaxPendingClaimsPerAttestation = 2
	link := &types.SubstrateLink{
		AdapterId: "wiki-v1",
		PendingClaims: []*types.PendingClaim{
			{ClaimContent: "a"}, {ClaimContent: "b"}, {ClaimContent: "c"},
		},
	}
	require.ErrorIs(t, k.ValidateLink(ctx, link, p), types.ErrTooManyPendingClaims)
}

func TestValidateLink_AxisOverflow(t *testing.T) {
	k, ctx := setupSubstrateBridgeKeeper(t)
	require.NoError(t, k.WriteAdapter(ctx, &types.AdapterRegistration{
		AdapterId:  "wiki-v1",
		Status:     types.AdapterStatus_ADAPTER_STATUS_ACTIVE,
		AxisBounds: &types.AxisBounds{AxisSubstrateMax: 100},
	}))
	link := &types.SubstrateLink{
		AdapterId: "wiki-v1",
		RecursionWeight: &types.AxisProjection{AxisSubstrate: 200},
	}
	require.ErrorIs(t, k.ValidateLink(ctx, link, types.DefaultParams()), types.ErrAxisOverflow)
}
