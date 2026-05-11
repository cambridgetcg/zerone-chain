package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/substrate_bridge/types"
)

func TestAdapterRegistry_WriteAndGet(t *testing.T) {
	k, ctx := setupSubstrateBridgeKeeper(t)
	adapter := &types.AdapterRegistration{
		AdapterId:              "wikipedia-en-v1",
		SourceType:             "wikipedia",
		Version:                "1.0.0",
		CompilerBinaryHash:     []byte{0xde, 0xad, 0xbe, 0xef},
		MinAttestationBondUzrn: "222000",
		MinPerClaimBondUzrn:    "222",
		Status:                 types.AdapterStatus_ADAPTER_STATUS_ACTIVE,
		RegisteredViaLipId:     "LIP-0001",
		RegisteredAtBlock:      100,
	}
	require.NoError(t, k.WriteAdapter(ctx, adapter))
	got, found := k.GetAdapter(ctx, "wikipedia-en-v1")
	require.True(t, found)
	require.Equal(t, adapter.AdapterId, got.AdapterId)
	require.Equal(t, adapter.Status, got.Status)
}

func TestAdapterRegistry_GetMissing(t *testing.T) {
	k, ctx := setupSubstrateBridgeKeeper(t)
	_, found := k.GetAdapter(ctx, "missing")
	require.False(t, found)
}

func TestAdapterRegistry_SuspendChangesStatus(t *testing.T) {
	k, ctx := setupSubstrateBridgeKeeper(t)
	require.NoError(t, k.WriteAdapter(ctx, &types.AdapterRegistration{
		AdapterId: "test-adapter",
		Status:    types.AdapterStatus_ADAPTER_STATUS_ACTIVE,
	}))
	require.NoError(t, k.SuspendAdapter(ctx, "test-adapter", "incident"))
	got, _ := k.GetAdapter(ctx, "test-adapter")
	require.Equal(t, types.AdapterStatus_ADAPTER_STATUS_SUSPENDED, got.Status)
}

func TestAdapterRegistry_TombstoneIsForwardOnly(t *testing.T) {
	k, ctx := setupSubstrateBridgeKeeper(t)
	require.NoError(t, k.WriteAdapter(ctx, &types.AdapterRegistration{
		AdapterId: "doomed-adapter",
		Status:    types.AdapterStatus_ADAPTER_STATUS_ACTIVE,
	}))
	require.NoError(t, k.TombstoneAdapter(ctx, "doomed-adapter"))
	got, _ := k.GetAdapter(ctx, "doomed-adapter")
	require.Equal(t, types.AdapterStatus_ADAPTER_STATUS_TOMBSTONED, got.Status)
	require.Greater(t, got.TombstonedAtBlock, uint64(0))

	err := k.WriteAdapter(ctx, &types.AdapterRegistration{
		AdapterId: "doomed-adapter",
		Status:    types.AdapterStatus_ADAPTER_STATUS_ACTIVE,
	})
	require.ErrorIs(t, err, types.ErrAdapterTombstoned)
}
