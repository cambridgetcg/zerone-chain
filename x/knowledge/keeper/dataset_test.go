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

func createActiveSample(t *testing.T, k keeper.Keeper, ctx context.Context, id, domain, lang string, sampleType types.SampleType, quality uint64, tags []string, content string, status types.SampleStatus) {
	t.Helper()
	sample := &types.Sample{
		Id:           id,
		Domain:       domain,
		Language:     lang,
		SampleType:   sampleType,
		QualityScore: quality,
		Tags:         tags,
		Content:      content,
		Status:       status,
		Submitter:    testAddr,
	}
	require.NoError(t, k.SetSample(ctx, sample))
	if domain != "" {
		require.NoError(t, k.SetSampleDomainIndex(ctx, domain, id))
	}
}

// ─── CreateDataset tests ────────────────────────────────────────────────────

func TestCreateDataset_Basic(t *testing.T) {
	k, ctx := setupKeeper(t)

	resp, err := k.CreateDataset(ctx, &types.MsgCreateDataset{
		Curator:     testAddr,
		Name:        "ML Training Set",
		Description: "High quality ML data",
		Domain:      "technology",
		License:     "CC-BY-4.0",
		MinQuality:  500_000,
	})
	require.NoError(t, err)
	require.NotEmpty(t, resp.DatasetId)

	ds, found := k.GetDataset(ctx, resp.DatasetId)
	require.True(t, found)
	require.Equal(t, "ML Training Set", ds.Name)
	require.Equal(t, "technology", ds.Domain)
	require.Equal(t, testAddr, ds.Curator)
	require.Equal(t, uint64(100), ds.CreatedAtBlock) // block height from setupKeeper
}

func TestCreateDataset_SequentialIDs(t *testing.T) {
	k, ctx := setupKeeper(t)

	msg := &types.MsgCreateDataset{Curator: testAddr, Name: "DS1"}
	resp1, err := k.CreateDataset(ctx, msg)
	require.NoError(t, err)

	msg.Name = "DS2"
	resp2, err := k.CreateDataset(ctx, msg)
	require.NoError(t, err)

	require.NotEqual(t, resp1.DatasetId, resp2.DatasetId)
}

func TestCreateDataset_InitialSampleCount(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Create 2 gold samples in "science" domain
	createActiveSample(t, k, ctx, "s1", "science", "en", types.SampleType_SAMPLE_TYPE_DISCUSSION, 700_000, nil, "hello world content", types.SampleStatus_SAMPLE_STATUS_GOLD)
	createActiveSample(t, k, ctx, "s2", "science", "en", types.SampleType_SAMPLE_TYPE_DEBATE, 800_000, nil, "more content here", types.SampleStatus_SAMPLE_STATUS_SILVER)
	// Create 1 rejected sample (should not count)
	createActiveSample(t, k, ctx, "s3", "science", "en", types.SampleType_SAMPLE_TYPE_DISCUSSION, 200_000, nil, "rejected", types.SampleStatus_SAMPLE_STATUS_REJECTED)

	resp, err := k.CreateDataset(ctx, &types.MsgCreateDataset{
		Curator: testAddr,
		Name:    "Science Set",
		Domain:  "science",
	})
	require.NoError(t, err)

	ds, found := k.GetDataset(ctx, resp.DatasetId)
	require.True(t, found)
	require.Equal(t, uint64(2), ds.SampleCount)
	require.True(t, ds.TotalTokens > 0)
}

// ─── Filter tests ───────────────────────────────────────────────────────────

func TestFilter_DomainOnly(t *testing.T) {
	k, ctx := setupKeeper(t)

	createActiveSample(t, k, ctx, "s1", "science", "en", 0, 500_000, nil, "content", types.SampleStatus_SAMPLE_STATUS_GOLD)
	createActiveSample(t, k, ctx, "s2", "tech", "en", 0, 500_000, nil, "content", types.SampleStatus_SAMPLE_STATUS_GOLD)

	resp, err := k.CreateDataset(ctx, &types.MsgCreateDataset{
		Curator: testAddr, Name: "Science Only", Domain: "science",
	})
	require.NoError(t, err)
	ds, _ := k.GetDataset(ctx, resp.DatasetId)
	require.Equal(t, uint64(1), ds.SampleCount)
}

func TestFilter_SampleType(t *testing.T) {
	k, ctx := setupKeeper(t)

	createActiveSample(t, k, ctx, "s1", "science", "en", types.SampleType_SAMPLE_TYPE_TUTORIAL, 500_000, nil, "tutorial", types.SampleStatus_SAMPLE_STATUS_GOLD)
	createActiveSample(t, k, ctx, "s2", "science", "en", types.SampleType_SAMPLE_TYPE_DEBATE, 500_000, nil, "debate", types.SampleStatus_SAMPLE_STATUS_GOLD)

	resp, err := k.CreateDataset(ctx, &types.MsgCreateDataset{
		Curator:    testAddr,
		Name:       "Tutorials Only",
		Domain:     "science",
		FilterType: types.SampleType_SAMPLE_TYPE_TUTORIAL,
	})
	require.NoError(t, err)
	ds, _ := k.GetDataset(ctx, resp.DatasetId)
	require.Equal(t, uint64(1), ds.SampleCount)
}

func TestFilter_Language(t *testing.T) {
	k, ctx := setupKeeper(t)

	createActiveSample(t, k, ctx, "s1", "science", "en", 0, 500_000, nil, "english", types.SampleStatus_SAMPLE_STATUS_GOLD)
	createActiveSample(t, k, ctx, "s2", "science", "fr", 0, 500_000, nil, "french", types.SampleStatus_SAMPLE_STATUS_GOLD)

	resp, err := k.CreateDataset(ctx, &types.MsgCreateDataset{
		Curator:        testAddr,
		Name:           "French Only",
		Domain:         "science",
		FilterLanguage: "fr",
	})
	require.NoError(t, err)
	ds, _ := k.GetDataset(ctx, resp.DatasetId)
	require.Equal(t, uint64(1), ds.SampleCount)
}

func TestFilter_Tags(t *testing.T) {
	k, ctx := setupKeeper(t)

	createActiveSample(t, k, ctx, "s1", "tech", "en", 0, 500_000, []string{"ml", "python"}, "content", types.SampleStatus_SAMPLE_STATUS_GOLD)
	createActiveSample(t, k, ctx, "s2", "tech", "en", 0, 500_000, []string{"rust", "systems"}, "content", types.SampleStatus_SAMPLE_STATUS_GOLD)
	createActiveSample(t, k, ctx, "s3", "tech", "en", 0, 500_000, []string{"ml", "rust"}, "content", types.SampleStatus_SAMPLE_STATUS_SILVER)

	resp, err := k.CreateDataset(ctx, &types.MsgCreateDataset{
		Curator:    testAddr,
		Name:       "ML Dataset",
		Domain:     "tech",
		FilterTags: []string{"ml"},
	})
	require.NoError(t, err)
	ds, _ := k.GetDataset(ctx, resp.DatasetId)
	require.Equal(t, uint64(2), ds.SampleCount) // s1 and s3
}

func TestFilter_MinQuality(t *testing.T) {
	k, ctx := setupKeeper(t)

	createActiveSample(t, k, ctx, "s1", "science", "en", 0, 300_000, nil, "low", types.SampleStatus_SAMPLE_STATUS_BRONZE)
	createActiveSample(t, k, ctx, "s2", "science", "en", 0, 700_000, nil, "high", types.SampleStatus_SAMPLE_STATUS_GOLD)
	createActiveSample(t, k, ctx, "s3", "science", "en", 0, 900_000, nil, "premium", types.SampleStatus_SAMPLE_STATUS_GOLD)

	resp, err := k.CreateDataset(ctx, &types.MsgCreateDataset{
		Curator:    testAddr,
		Name:       "High Quality",
		Domain:     "science",
		MinQuality: 600_000,
	})
	require.NoError(t, err)
	ds, _ := k.GetDataset(ctx, resp.DatasetId)
	require.Equal(t, uint64(2), ds.SampleCount) // s2 and s3
}

func TestFilter_CombinedFilters(t *testing.T) {
	k, ctx := setupKeeper(t)

	createActiveSample(t, k, ctx, "s1", "science", "en", types.SampleType_SAMPLE_TYPE_TUTORIAL, 800_000, []string{"ml"}, "content", types.SampleStatus_SAMPLE_STATUS_GOLD)
	createActiveSample(t, k, ctx, "s2", "science", "en", types.SampleType_SAMPLE_TYPE_DEBATE, 800_000, []string{"ml"}, "content", types.SampleStatus_SAMPLE_STATUS_GOLD)
	createActiveSample(t, k, ctx, "s3", "science", "fr", types.SampleType_SAMPLE_TYPE_TUTORIAL, 800_000, []string{"ml"}, "content", types.SampleStatus_SAMPLE_STATUS_GOLD)
	createActiveSample(t, k, ctx, "s4", "science", "en", types.SampleType_SAMPLE_TYPE_TUTORIAL, 200_000, []string{"ml"}, "content", types.SampleStatus_SAMPLE_STATUS_BRONZE)

	resp, err := k.CreateDataset(ctx, &types.MsgCreateDataset{
		Curator:        testAddr,
		Name:           "Specific",
		Domain:         "science",
		FilterType:     types.SampleType_SAMPLE_TYPE_TUTORIAL,
		FilterLanguage: "en",
		FilterTags:     []string{"ml"},
		MinQuality:     500_000,
	})
	require.NoError(t, err)
	ds, _ := k.GetDataset(ctx, resp.DatasetId)
	require.Equal(t, uint64(1), ds.SampleCount) // only s1 matches all criteria
}

func TestFilter_ActiveStatusOnly(t *testing.T) {
	k, ctx := setupKeeper(t)

	createActiveSample(t, k, ctx, "s1", "tech", "en", 0, 500_000, nil, "content", types.SampleStatus_SAMPLE_STATUS_GOLD)
	createActiveSample(t, k, ctx, "s2", "tech", "en", 0, 500_000, nil, "content", types.SampleStatus_SAMPLE_STATUS_PENDING)
	createActiveSample(t, k, ctx, "s3", "tech", "en", 0, 500_000, nil, "content", types.SampleStatus_SAMPLE_STATUS_EXPIRED)
	createActiveSample(t, k, ctx, "s4", "tech", "en", 0, 500_000, nil, "content", types.SampleStatus_SAMPLE_STATUS_PRUNED)
	createActiveSample(t, k, ctx, "s5", "tech", "en", 0, 500_000, nil, "content", types.SampleStatus_SAMPLE_STATUS_SILVER)
	createActiveSample(t, k, ctx, "s6", "tech", "en", 0, 500_000, nil, "content", types.SampleStatus_SAMPLE_STATUS_BRONZE)

	resp, err := k.CreateDataset(ctx, &types.MsgCreateDataset{
		Curator: testAddr, Name: "Active Only", Domain: "tech",
	})
	require.NoError(t, err)
	ds, _ := k.GetDataset(ctx, resp.DatasetId)
	require.Equal(t, uint64(3), ds.SampleCount) // gold, silver, bronze
}

func TestFilter_NoMatches(t *testing.T) {
	k, ctx := setupKeeper(t)

	createActiveSample(t, k, ctx, "s1", "science", "en", 0, 500_000, nil, "content", types.SampleStatus_SAMPLE_STATUS_GOLD)

	resp, err := k.CreateDataset(ctx, &types.MsgCreateDataset{
		Curator:    testAddr,
		Name:       "Empty",
		Domain:     "nonexistent_domain",
		MinQuality: 999_999,
	})
	require.NoError(t, err)
	ds, _ := k.GetDataset(ctx, resp.DatasetId)
	require.Equal(t, uint64(0), ds.SampleCount)
}

func TestFilter_NoDomainIteratesAll(t *testing.T) {
	k, ctx := setupKeeper(t)

	createActiveSample(t, k, ctx, "s1", "science", "en", 0, 500_000, nil, "content", types.SampleStatus_SAMPLE_STATUS_GOLD)
	createActiveSample(t, k, ctx, "s2", "tech", "en", 0, 500_000, nil, "content", types.SampleStatus_SAMPLE_STATUS_SILVER)

	resp, err := k.CreateDataset(ctx, &types.MsgCreateDataset{
		Curator: testAddr, Name: "All Domains",
		// No domain filter
	})
	require.NoError(t, err)
	ds, _ := k.GetDataset(ctx, resp.DatasetId)
	require.Equal(t, uint64(2), ds.SampleCount)
}

// ─── Overlap tests ──────────────────────────────────────────────────────────

func TestMultipleDatasets_OverlappingSamples(t *testing.T) {
	k, ctx := setupKeeper(t)

	createActiveSample(t, k, ctx, "s1", "science", "en", types.SampleType_SAMPLE_TYPE_TUTORIAL, 700_000, []string{"ml"}, "content", types.SampleStatus_SAMPLE_STATUS_GOLD)

	resp1, err := k.CreateDataset(ctx, &types.MsgCreateDataset{
		Curator: testAddr, Name: "DS1", Domain: "science",
	})
	require.NoError(t, err)

	resp2, err := k.CreateDataset(ctx, &types.MsgCreateDataset{
		Curator:    testAddr,
		Name:       "DS2",
		Domain:     "science",
		FilterTags: []string{"ml"},
	})
	require.NoError(t, err)

	ds1, _ := k.GetDataset(ctx, resp1.DatasetId)
	ds2, _ := k.GetDataset(ctx, resp2.DatasetId)
	require.Equal(t, uint64(1), ds1.SampleCount)
	require.Equal(t, uint64(1), ds2.SampleCount)
}

// ─── CRUD tests ─────────────────────────────────────────────────────────────

func TestDataset_GetNotFound(t *testing.T) {
	k, ctx := setupKeeper(t)

	_, found := k.GetDataset(ctx, "nonexistent")
	require.False(t, found)
}

func TestDataset_Delete(t *testing.T) {
	k, ctx := setupKeeper(t)

	resp, err := k.CreateDataset(ctx, &types.MsgCreateDataset{
		Curator: testAddr, Name: "Deletable",
	})
	require.NoError(t, err)

	_, found := k.GetDataset(ctx, resp.DatasetId)
	require.True(t, found)

	require.NoError(t, k.DeleteDataset(ctx, resp.DatasetId))

	_, found = k.GetDataset(ctx, resp.DatasetId)
	require.False(t, found)
}

func TestDataset_IterateDatasets(t *testing.T) {
	k, ctx := setupKeeper(t)

	for i := 0; i < 3; i++ {
		_, err := k.CreateDataset(ctx, &types.MsgCreateDataset{
			Curator: testAddr, Name: "DS",
		})
		require.NoError(t, err)
	}

	var count int
	k.IterateDatasets(ctx, func(ds *types.Dataset) bool {
		count++
		return false
	})
	require.Equal(t, 3, count)
}

// ─── AccessDataset tests ────────────────────────────────────────────────────

func TestAccessDataset_PaymentAndCommission(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)

	createActiveSample(t, k, ctx, "s1", "science", "en", 0, 700_000, nil, "content bytes here", types.SampleStatus_SAMPLE_STATUS_GOLD)

	resp, err := k.CreateDataset(ctx, &types.MsgCreateDataset{
		Curator:   testAddr,
		Name:      "Paid DS",
		Domain:    "science",
		BulkPrice: "1000000",
	})
	require.NoError(t, err)

	accessResp, err := k.AccessDataset(ctx, &types.MsgAccessDataset{
		Consumer:   testAddr,
		DatasetId:  resp.DatasetId,
		MaxPayment: "2000000",
	})
	require.NoError(t, err)
	require.Equal(t, "1000000", accessResp.Payment)
	require.Equal(t, uint64(1), accessResp.SampleCount)

	// Verify bank calls
	require.Len(t, bk.accountToModuleCalls, 1)
	require.Equal(t, "knowledge", bk.accountToModuleCalls[0].to)

	// Curator should get 95%
	require.Len(t, bk.moduleToAccountCalls, 1)
	require.Equal(t, testAddr, bk.moduleToAccountCalls[0].to)
	curatorAmt := bk.moduleToAccountCalls[0].amount.AmountOf("uzrn")
	require.Equal(t, sdkmath.NewInt(950_000), curatorAmt) // 95% of 1_000_000
}

func TestAccessDataset_NotFound(t *testing.T) {
	k, ctx := setupKeeper(t)

	_, err := k.AccessDataset(ctx, &types.MsgAccessDataset{
		Consumer:  testAddr,
		DatasetId: "nonexistent",
	})
	require.ErrorIs(t, err, types.ErrDatasetNotFound)
}

func TestAccessDataset_MaxPaymentExceeded(t *testing.T) {
	k, ctx := setupKeeper(t)

	resp, err := k.CreateDataset(ctx, &types.MsgCreateDataset{
		Curator:   testAddr,
		Name:      "Expensive",
		BulkPrice: "1000000",
	})
	require.NoError(t, err)

	_, err = k.AccessDataset(ctx, &types.MsgAccessDataset{
		Consumer:   testAddr,
		DatasetId:  resp.DatasetId,
		MaxPayment: "500000",
	})
	require.ErrorIs(t, err, types.ErrInsufficientPayment)
}

func TestAccessDataset_InsufficientFunds(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)
	bk.failNextSend = true

	resp, err := k.CreateDataset(ctx, &types.MsgCreateDataset{
		Curator:   testAddr,
		Name:      "DS",
		BulkPrice: "1000000",
	})
	require.NoError(t, err)

	_, err = k.AccessDataset(ctx, &types.MsgAccessDataset{
		Consumer:  testAddr,
		DatasetId: resp.DatasetId,
	})
	require.ErrorIs(t, err, types.ErrInsufficientPayment)
}

// ─── Domain index tests ─────────────────────────────────────────────────────

func TestDataset_DomainIndex(t *testing.T) {
	k, ctx := setupKeeper(t)

	_, err := k.CreateDataset(ctx, &types.MsgCreateDataset{
		Curator: testAddr, Name: "Science DS", Domain: "science",
	})
	require.NoError(t, err)

	_, err = k.CreateDataset(ctx, &types.MsgCreateDataset{
		Curator: testAddr, Name: "Tech DS", Domain: "tech",
	})
	require.NoError(t, err)

	scienceIDs := k.GetDatasetsByDomain(ctx, "science")
	require.Len(t, scienceIDs, 1)

	techIDs := k.GetDatasetsByDomain(ctx, "tech")
	require.Len(t, techIDs, 1)

	emptyIDs := k.GetDatasetsByDomain(ctx, "nonexistent")
	require.Empty(t, emptyIDs)
}

// ─── Token estimation test ──────────────────────────────────────────────────

func TestDataset_TokenEstimation(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Create sample with 100 bytes of content → ~25 tokens
	content := make([]byte, 100)
	for i := range content {
		content[i] = 'a'
	}
	createActiveSample(t, k, ctx, "s1", "tech", "en", 0, 500_000, nil, string(content), types.SampleStatus_SAMPLE_STATUS_GOLD)

	resp, err := k.CreateDataset(ctx, &types.MsgCreateDataset{
		Curator: testAddr, Name: "Token DS", Domain: "tech",
	})
	require.NoError(t, err)

	ds, _ := k.GetDataset(ctx, resp.DatasetId)
	require.Equal(t, uint64(25), ds.TotalTokens) // 100 / 4
}
