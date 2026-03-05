package keeper_test

import (
	"context"
	"testing"

	sdkmath "cosmossdk.io/math"
	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/keeper"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── helpers ────────────────────────────────────────────────────────────────

func createSampleForAccess(t *testing.T, k keeper.Keeper, ctx context.Context, id, domain, tier string, status types.SampleStatus) {
	t.Helper()
	sample := &types.Sample{
		Id:           id,
		Domain:       domain,
		QualityTier:  tier,
		QualityScore: 700_000,
		Status:       status,
		Submitter:    testAddr,
		Content:      "sample content for testing",
		Language:     "en",
		Energy:       500_000,
		EnergyCap:    1_000_000,
	}
	require.NoError(t, k.SetSample(ctx, sample))
	if domain != "" {
		require.NoError(t, k.SetSampleDomainIndex(ctx, domain, id))
	}
}

func setDefaultParams(t *testing.T, k keeper.Keeper, ctx context.Context) {
	t.Helper()
	params := types.DefaultParams()
	require.NoError(t, k.SetParams(ctx, &params))
}

// ─── AccessSample: Price calculation ────────────────────────────────────────

func TestAccessSample_GoldPricing(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)
	setDefaultParams(t, k, ctx)
	createSampleForAccess(t, k, ctx, "s1", "science", "gold", types.SampleStatus_SAMPLE_STATUS_GOLD)

	resp, err := k.AccessSample(ctx, &types.MsgAccessSample{
		Consumer: testAddr,
		SampleId: "s1",
	})
	require.NoError(t, err)
	// base=100000, gold=30000/10000=3x → 300000
	require.Equal(t, "300000", resp.Payment)
	require.Len(t, bk.accountToModuleCalls, 1)
}

func TestAccessSample_SilverPricing(t *testing.T) {
	k, ctx, _ := setupKeeperWithBank(t)
	setDefaultParams(t, k, ctx)
	createSampleForAccess(t, k, ctx, "s1", "science", "silver", types.SampleStatus_SAMPLE_STATUS_SILVER)

	resp, err := k.AccessSample(ctx, &types.MsgAccessSample{
		Consumer: testAddr,
		SampleId: "s1",
	})
	require.NoError(t, err)
	// base=100000, silver=20000/10000=2x → 200000
	require.Equal(t, "200000", resp.Payment)
}

func TestAccessSample_BronzePricing(t *testing.T) {
	k, ctx, _ := setupKeeperWithBank(t)
	setDefaultParams(t, k, ctx)
	createSampleForAccess(t, k, ctx, "s1", "science", "bronze", types.SampleStatus_SAMPLE_STATUS_BRONZE)

	resp, err := k.AccessSample(ctx, &types.MsgAccessSample{
		Consumer: testAddr,
		SampleId: "s1",
	})
	require.NoError(t, err)
	// base=100000, bronze=10000/10000=1x → 100000
	require.Equal(t, "100000", resp.Payment)
}

// ─── AccessSample: State updates ────────────────────────────────────────────

func TestAccessSample_CounterIncremented(t *testing.T) {
	k, ctx, _ := setupKeeperWithBank(t)
	setDefaultParams(t, k, ctx)
	createSampleForAccess(t, k, ctx, "s1", "science", "gold", types.SampleStatus_SAMPLE_STATUS_GOLD)

	_, err := k.AccessSample(ctx, &types.MsgAccessSample{Consumer: testAddr, SampleId: "s1"})
	require.NoError(t, err)

	sample, found := k.GetSample(ctx, "s1")
	require.True(t, found)
	require.Equal(t, uint64(1), sample.AccessCount)
	require.Equal(t, uint64(100), sample.LastAccessedBlock)
}

func TestAccessSample_MultipleAccessesAccumulate(t *testing.T) {
	k, ctx, _ := setupKeeperWithBank(t)
	setDefaultParams(t, k, ctx)
	createSampleForAccess(t, k, ctx, "s1", "science", "gold", types.SampleStatus_SAMPLE_STATUS_GOLD)

	for i := 0; i < 3; i++ {
		_, err := k.AccessSample(ctx, &types.MsgAccessSample{Consumer: testAddr, SampleId: "s1"})
		require.NoError(t, err)
	}

	sample, found := k.GetSample(ctx, "s1")
	require.True(t, found)
	require.Equal(t, uint64(3), sample.AccessCount)
	// total_revenue = 300000 * 3 = 900000
	require.Equal(t, "900000", sample.TotalRevenue)
}

func TestAccessSample_EnergyRestored(t *testing.T) {
	k, ctx, _ := setupKeeperWithBank(t)
	setDefaultParams(t, k, ctx)
	createSampleForAccess(t, k, ctx, "s1", "science", "gold", types.SampleStatus_SAMPLE_STATUS_GOLD)

	_, err := k.AccessSample(ctx, &types.MsgAccessSample{Consumer: testAddr, SampleId: "s1"})
	require.NoError(t, err)

	sample, found := k.GetSample(ctx, "s1")
	require.True(t, found)
	// Energy was 500000, params.EnergyPerAccess=1000 → 501000
	require.Equal(t, uint64(501_000), sample.Energy)
}

func TestAccessSample_RevenueQueued(t *testing.T) {
	k, ctx, _ := setupKeeperWithBank(t)
	setDefaultParams(t, k, ctx)
	createSampleForAccess(t, k, ctx, "s1", "science", "gold", types.SampleStatus_SAMPLE_STATUS_GOLD)

	_, err := k.AccessSample(ctx, &types.MsgAccessSample{Consumer: testAddr, SampleId: "s1"})
	require.NoError(t, err)

	pending := k.GetPendingRevenue(ctx, "s1")
	require.Equal(t, uint64(300_000), pending) // gold price
}

func TestAccessSample_MultipleAccessesAccumulateRevenue(t *testing.T) {
	k, ctx, _ := setupKeeperWithBank(t)
	setDefaultParams(t, k, ctx)
	createSampleForAccess(t, k, ctx, "s1", "science", "bronze", types.SampleStatus_SAMPLE_STATUS_BRONZE)

	for i := 0; i < 5; i++ {
		_, err := k.AccessSample(ctx, &types.MsgAccessSample{Consumer: testAddr, SampleId: "s1"})
		require.NoError(t, err)
	}

	pending := k.GetPendingRevenue(ctx, "s1")
	require.Equal(t, uint64(500_000), pending) // 100000 * 5
}

// ─── AccessSample: Error cases ──────────────────────────────────────────────

func TestAccessSample_NotFound(t *testing.T) {
	k, ctx := setupKeeper(t)
	setDefaultParams(t, k, ctx)

	_, err := k.AccessSample(ctx, &types.MsgAccessSample{
		Consumer: testAddr,
		SampleId: "nonexistent",
	})
	require.ErrorIs(t, err, types.ErrSampleNotFound)
}

func TestAccessSample_InactiveSample(t *testing.T) {
	k, ctx, _ := setupKeeperWithBank(t)
	setDefaultParams(t, k, ctx)
	createSampleForAccess(t, k, ctx, "s1", "science", "gold", types.SampleStatus_SAMPLE_STATUS_REJECTED)

	_, err := k.AccessSample(ctx, &types.MsgAccessSample{
		Consumer: testAddr,
		SampleId: "s1",
	})
	require.ErrorIs(t, err, types.ErrSampleNotFound)
}

func TestAccessSample_MaxPaymentExceeded(t *testing.T) {
	k, ctx, _ := setupKeeperWithBank(t)
	setDefaultParams(t, k, ctx)
	createSampleForAccess(t, k, ctx, "s1", "science", "gold", types.SampleStatus_SAMPLE_STATUS_GOLD)

	_, err := k.AccessSample(ctx, &types.MsgAccessSample{
		Consumer:   testAddr,
		SampleId:   "s1",
		MaxPayment: "100000", // gold costs 300000
	})
	require.ErrorIs(t, err, types.ErrInsufficientPayment)
}

func TestAccessSample_InsufficientFunds(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)
	setDefaultParams(t, k, ctx)
	bk.failNextSend = true
	createSampleForAccess(t, k, ctx, "s1", "science", "gold", types.SampleStatus_SAMPLE_STATUS_GOLD)

	_, err := k.AccessSample(ctx, &types.MsgAccessSample{
		Consumer: testAddr,
		SampleId: "s1",
	})
	require.ErrorIs(t, err, types.ErrInsufficientPayment)
}

// ─── AccessDataset: Bulk pricing ────────────────────────────────────────────

func TestAccessDataset_BulkPriceUsed(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)
	setDefaultParams(t, k, ctx)
	createSampleForAccess(t, k, ctx, "s1", "science", "gold", types.SampleStatus_SAMPLE_STATUS_GOLD)

	dsResp, err := k.CreateDataset(ctx, &types.MsgCreateDataset{
		Curator:   testAddr,
		Name:      "DS",
		Domain:    "science",
		BulkPrice: "500000",
	})
	require.NoError(t, err)

	resp, err := k.AccessDataset(ctx, &types.MsgAccessDataset{
		Consumer:  testAddr,
		DatasetId: dsResp.DatasetId,
	})
	require.NoError(t, err)
	require.Equal(t, "500000", resp.Payment)
	require.Len(t, bk.accountToModuleCalls, 1)
}

func TestAccessDataset_BulkDiscountApplied(t *testing.T) {
	k, ctx, _ := setupKeeperWithBank(t)
	setDefaultParams(t, k, ctx)

	// Create 2 gold samples → 300000 each = 600000 total, 20% discount → 480000
	createSampleForAccess(t, k, ctx, "s1", "science", "gold", types.SampleStatus_SAMPLE_STATUS_GOLD)
	createSampleForAccess(t, k, ctx, "s2", "science", "gold", types.SampleStatus_SAMPLE_STATUS_GOLD)

	dsResp, err := k.CreateDataset(ctx, &types.MsgCreateDataset{
		Curator: testAddr,
		Name:    "DS",
		Domain:  "science",
		// No BulkPrice → auto-calculated with discount
	})
	require.NoError(t, err)

	resp, err := k.AccessDataset(ctx, &types.MsgAccessDataset{
		Consumer:  testAddr,
		DatasetId: dsResp.DatasetId,
	})
	require.NoError(t, err)
	// 2 gold samples × 300000 = 600000, 20% off = 480000
	require.Equal(t, "480000", resp.Payment)
	require.Equal(t, uint64(2), resp.SampleCount)
}

func TestAccessDataset_SamplesUpdated(t *testing.T) {
	k, ctx, _ := setupKeeperWithBank(t)
	setDefaultParams(t, k, ctx)
	createSampleForAccess(t, k, ctx, "s1", "science", "gold", types.SampleStatus_SAMPLE_STATUS_GOLD)

	dsResp, err := k.CreateDataset(ctx, &types.MsgCreateDataset{
		Curator:   testAddr,
		Name:      "DS",
		Domain:    "science",
		BulkPrice: "500000",
	})
	require.NoError(t, err)

	_, err = k.AccessDataset(ctx, &types.MsgAccessDataset{
		Consumer:  testAddr,
		DatasetId: dsResp.DatasetId,
	})
	require.NoError(t, err)

	// Sample should have access_count incremented and energy restored
	sample, found := k.GetSample(ctx, "s1")
	require.True(t, found)
	require.Equal(t, uint64(1), sample.AccessCount)
	require.Equal(t, uint64(501_000), sample.Energy) // 500000 + 1000
}

func TestAccessDataset_CuratorCommission(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)
	setDefaultParams(t, k, ctx)
	createSampleForAccess(t, k, ctx, "s1", "science", "gold", types.SampleStatus_SAMPLE_STATUS_GOLD)

	dsResp, err := k.CreateDataset(ctx, &types.MsgCreateDataset{
		Curator:   testAddr,
		Name:      "DS",
		Domain:    "science",
		BulkPrice: "1000000",
	})
	require.NoError(t, err)

	_, err = k.AccessDataset(ctx, &types.MsgAccessDataset{
		Consumer:  testAddr,
		DatasetId: dsResp.DatasetId,
	})
	require.NoError(t, err)

	// Curator gets 95%
	require.Len(t, bk.moduleToAccountCalls, 1)
	curatorAmt := bk.moduleToAccountCalls[0].amount.AmountOf("uzrn")
	require.Equal(t, sdkmath.NewInt(950_000), curatorAmt)
}

// ─── PendingRevenue CRUD ────────────────────────────────────────────────────

func TestPendingRevenue_GetSetDelete(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Initially zero
	require.Equal(t, uint64(0), k.GetPendingRevenue(ctx, "s1"))

	// Set
	require.NoError(t, k.SetPendingRevenue(ctx, "s1", 500_000))
	require.Equal(t, uint64(500_000), k.GetPendingRevenue(ctx, "s1"))

	// Delete
	require.NoError(t, k.DeletePendingRevenue(ctx, "s1"))
	require.Equal(t, uint64(0), k.GetPendingRevenue(ctx, "s1"))
}

func TestPendingRevenue_IterateMultiple(t *testing.T) {
	k, ctx := setupKeeper(t)

	require.NoError(t, k.SetPendingRevenue(ctx, "s1", 100))
	require.NoError(t, k.SetPendingRevenue(ctx, "s2", 200))
	require.NoError(t, k.SetPendingRevenue(ctx, "s3", 300))

	var total uint64
	var count int
	k.IteratePendingRevenue(ctx, func(sampleID string, amount uint64) bool {
		total += amount
		count++
		return false
	})
	require.Equal(t, 3, count)
	require.Equal(t, uint64(600), total)
}
