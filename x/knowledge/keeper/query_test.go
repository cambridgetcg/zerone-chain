package keeper_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/keeper"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── helpers ────────────────────────────────────────────────────────────────

func setupQueryServer(t *testing.T) (types.QueryServer, keeper.Keeper, context.Context) {
	t.Helper()
	k, ctx := setupKeeper(t)
	qs := keeper.NewQueryServerImpl(k)
	return qs, k, ctx
}

func createTestSample(t *testing.T, k keeper.Keeper, ctx context.Context, id, domain, submitter string, status types.SampleStatus, sampleType types.SampleType) {
	t.Helper()
	sample := &types.Sample{
		Id:         id,
		Domain:     domain,
		Submitter:  submitter,
		Status:     status,
		SampleType: sampleType,
		Content:    "test content for " + id,
		Energy:     500_000,
		EnergyCap:  1_000_000,
	}
	require.NoError(t, k.SetSample(ctx, sample))
	require.NoError(t, k.SetSampleDomainIndex(ctx, domain, id))
	require.NoError(t, k.SetSampleSubmitterIndex(ctx, submitter, id))
}

func createTestSampleWithThread(t *testing.T, k keeper.Keeper, ctx context.Context, id, domain, threadID string, position uint64) {
	t.Helper()
	sample := &types.Sample{
		Id:             id,
		Domain:         domain,
		Submitter:      testAddr,
		Status:         types.SampleStatus_SAMPLE_STATUS_GOLD,
		SampleType:     types.SampleType_SAMPLE_TYPE_DISCUSSION,
		Content:        "thread content " + id,
		ThreadId:       threadID,
		ThreadPosition: position,
		Energy:         500_000,
		EnergyCap:      1_000_000,
	}
	require.NoError(t, k.SetSample(ctx, sample))
	require.NoError(t, k.SetSampleDomainIndex(ctx, domain, id))
	require.NoError(t, k.SetSampleThreadIndex(ctx, threadID, id))
	require.NoError(t, k.SetSampleSubmitterIndex(ctx, testAddr, id))
}

// ─── Sample (single) ────────────────────────────────────────────────────────

func TestQuery_Sample_Found(t *testing.T) {
	qs, k, ctx := setupQueryServer(t)
	createTestSample(t, k, ctx, "s1", "science", testAddr, types.SampleStatus_SAMPLE_STATUS_GOLD, types.SampleType_SAMPLE_TYPE_DISCUSSION)

	resp, err := qs.Sample(ctx, &types.QuerySampleRequest{Id: "s1"})
	require.NoError(t, err)
	require.Equal(t, "s1", resp.Sample.Id)
	require.Equal(t, "science", resp.Sample.Domain)
}

func TestQuery_Sample_NotFound(t *testing.T) {
	qs, _, ctx := setupQueryServer(t)

	_, err := qs.Sample(ctx, &types.QuerySampleRequest{Id: "nonexistent"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

func TestQuery_Sample_MissingID(t *testing.T) {
	qs, _, ctx := setupQueryServer(t)

	_, err := qs.Sample(ctx, &types.QuerySampleRequest{Id: ""})
	require.Error(t, err)
	require.Contains(t, err.Error(), "required")
}

// ─── Samples (filtered) ─────────────────────────────────────────────────────

func TestQuery_Samples_FilterByDomain(t *testing.T) {
	qs, k, ctx := setupQueryServer(t)
	createTestSample(t, k, ctx, "s1", "science", testAddr, types.SampleStatus_SAMPLE_STATUS_GOLD, types.SampleType_SAMPLE_TYPE_DISCUSSION)
	createTestSample(t, k, ctx, "s2", "technology", testAddr, types.SampleStatus_SAMPLE_STATUS_GOLD, types.SampleType_SAMPLE_TYPE_DISCUSSION)
	createTestSample(t, k, ctx, "s3", "science", testAddr, types.SampleStatus_SAMPLE_STATUS_SILVER, types.SampleType_SAMPLE_TYPE_DISCUSSION)

	resp, err := qs.Samples(ctx, &types.QuerySamplesRequest{Domain: "science"})
	require.NoError(t, err)
	require.Len(t, resp.Samples, 2)
	for _, s := range resp.Samples {
		require.Equal(t, "science", s.Domain)
	}
}

func TestQuery_Samples_FilterByStatus(t *testing.T) {
	qs, k, ctx := setupQueryServer(t)
	createTestSample(t, k, ctx, "s1", "science", testAddr, types.SampleStatus_SAMPLE_STATUS_GOLD, types.SampleType_SAMPLE_TYPE_DISCUSSION)
	createTestSample(t, k, ctx, "s2", "science", testAddr, types.SampleStatus_SAMPLE_STATUS_PENDING, types.SampleType_SAMPLE_TYPE_DISCUSSION)
	createTestSample(t, k, ctx, "s3", "science", testAddr, types.SampleStatus_SAMPLE_STATUS_GOLD, types.SampleType_SAMPLE_TYPE_DISCUSSION)

	resp, err := qs.Samples(ctx, &types.QuerySamplesRequest{
		Status: "SAMPLE_STATUS_GOLD",
	})
	require.NoError(t, err)
	require.Len(t, resp.Samples, 2)
	for _, s := range resp.Samples {
		require.Equal(t, types.SampleStatus_SAMPLE_STATUS_GOLD, s.Status)
	}
}

func TestQuery_Samples_FilterBySampleType(t *testing.T) {
	qs, k, ctx := setupQueryServer(t)
	createTestSample(t, k, ctx, "s1", "science", testAddr, types.SampleStatus_SAMPLE_STATUS_GOLD, types.SampleType_SAMPLE_TYPE_DISCUSSION)
	createTestSample(t, k, ctx, "s2", "science", testAddr, types.SampleStatus_SAMPLE_STATUS_GOLD, types.SampleType_SAMPLE_TYPE_EXPLANATION)
	createTestSample(t, k, ctx, "s3", "science", testAddr, types.SampleStatus_SAMPLE_STATUS_GOLD, types.SampleType_SAMPLE_TYPE_DISCUSSION)

	resp, err := qs.Samples(ctx, &types.QuerySamplesRequest{
		SampleType: types.SampleType_SAMPLE_TYPE_EXPLANATION,
	})
	require.NoError(t, err)
	require.Len(t, resp.Samples, 1)
	require.Equal(t, "s2", resp.Samples[0].Id)
}

func TestQuery_Samples_EmptyResult(t *testing.T) {
	qs, _, ctx := setupQueryServer(t)

	resp, err := qs.Samples(ctx, &types.QuerySamplesRequest{Domain: "nonexistent"})
	require.NoError(t, err)
	require.Empty(t, resp.Samples)
}

// ─── SamplesByDomain ─────────────────────────────────────────────────────────

func TestQuery_SamplesByDomain(t *testing.T) {
	qs, k, ctx := setupQueryServer(t)
	createTestSample(t, k, ctx, "s1", "science", testAddr, types.SampleStatus_SAMPLE_STATUS_GOLD, types.SampleType_SAMPLE_TYPE_DISCUSSION)
	createTestSample(t, k, ctx, "s2", "technology", testAddr, types.SampleStatus_SAMPLE_STATUS_GOLD, types.SampleType_SAMPLE_TYPE_DISCUSSION)

	resp, err := qs.SamplesByDomain(ctx, &types.QuerySamplesByDomainRequest{Domain: "science"})
	require.NoError(t, err)
	require.Len(t, resp.Samples, 1)
	require.Equal(t, "s1", resp.Samples[0].Id)
}

// ─── SamplesByThread (ordered by position) ──────────────────────────────────

func TestQuery_SamplesByThread_OrderedByPosition(t *testing.T) {
	qs, k, ctx := setupQueryServer(t)
	// Insert out of order
	createTestSampleWithThread(t, k, ctx, "s3", "science", "thread1", 3)
	createTestSampleWithThread(t, k, ctx, "s1", "science", "thread1", 1)
	createTestSampleWithThread(t, k, ctx, "s2", "science", "thread1", 2)

	resp, err := qs.SamplesByThread(ctx, &types.QuerySamplesByThreadRequest{ThreadId: "thread1"})
	require.NoError(t, err)
	require.Len(t, resp.Samples, 3)
	// Verify order by position
	require.Equal(t, uint64(1), resp.Samples[0].ThreadPosition)
	require.Equal(t, uint64(2), resp.Samples[1].ThreadPosition)
	require.Equal(t, uint64(3), resp.Samples[2].ThreadPosition)
}

// ─── SamplesBySubmitter ──────────────────────────────────────────────────────

func TestQuery_SamplesBySubmitter(t *testing.T) {
	qs, k, ctx := setupQueryServer(t)
	sub2 := "zrn1qcxce9c4thzxnfmpr2dqnnlqea9ey35ydj769h"
	createTestSample(t, k, ctx, "s1", "science", testAddr, types.SampleStatus_SAMPLE_STATUS_GOLD, types.SampleType_SAMPLE_TYPE_DISCUSSION)
	createTestSample(t, k, ctx, "s2", "science", sub2, types.SampleStatus_SAMPLE_STATUS_GOLD, types.SampleType_SAMPLE_TYPE_DISCUSSION)
	createTestSample(t, k, ctx, "s3", "science", testAddr, types.SampleStatus_SAMPLE_STATUS_GOLD, types.SampleType_SAMPLE_TYPE_DISCUSSION)

	resp, err := qs.SamplesBySubmitter(ctx, &types.QuerySamplesBySubmitterRequest{Submitter: testAddr})
	require.NoError(t, err)
	require.Len(t, resp.Samples, 2)
	for _, s := range resp.Samples {
		require.Equal(t, testAddr, s.Submitter)
	}
}

// ─── Submission / PendingSubmissions ─────────────────────────────────────────

func TestQuery_Submission_Found(t *testing.T) {
	qs, k, ctx := setupQueryServer(t)
	sub := &types.Submission{
		Id:        "sub1",
		Submitter: testAddr,
		Domain:    "science",
		Status:    types.SubmissionStatus_SUBMISSION_STATUS_PENDING,
	}
	require.NoError(t, k.SetSubmission(ctx, sub))

	resp, err := qs.Submission(ctx, &types.QuerySubmissionRequest{Id: "sub1"})
	require.NoError(t, err)
	require.Equal(t, "sub1", resp.Submission.Id)
}

func TestQuery_PendingSubmissions(t *testing.T) {
	qs, k, ctx := setupQueryServer(t)
	require.NoError(t, k.SetSubmission(ctx, &types.Submission{
		Id: "sub1", Status: types.SubmissionStatus_SUBMISSION_STATUS_PENDING,
	}))
	require.NoError(t, k.SetSubmission(ctx, &types.Submission{
		Id: "sub2", Status: types.SubmissionStatus_SUBMISSION_STATUS_REVIEWED,
	}))
	require.NoError(t, k.SetSubmission(ctx, &types.Submission{
		Id: "sub3", Status: types.SubmissionStatus_SUBMISSION_STATUS_PENDING,
	}))

	resp, err := qs.PendingSubmissions(ctx, &types.QueryPendingSubmissionsRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Submissions, 2)
}

// ─── QualityRound ────────────────────────────────────────────────────────────

func TestQuery_QualityRound_Found(t *testing.T) {
	qs, k, ctx := setupQueryServer(t)
	round := &types.QualityRound{
		Id:           "r1",
		SubmissionId: "sub1",
	}
	require.NoError(t, k.SetQualityRound(ctx, round))

	resp, err := qs.QualityRound(ctx, &types.QueryQualityRoundRequest{Id: "r1"})
	require.NoError(t, err)
	require.Equal(t, "r1", resp.Round.Id)
}

// ─── Domain / Domains ────────────────────────────────────────────────────────

func TestQuery_Domain_Found(t *testing.T) {
	qs, k, ctx := setupQueryServer(t)
	setupDefaultDomains(t, k, ctx)

	resp, err := qs.Domain(ctx, &types.QueryDomainRequest{Name: "science"})
	require.NoError(t, err)
	require.Equal(t, "science", resp.Domain.Name)
}

func TestQuery_Domains_ListAll(t *testing.T) {
	qs, k, ctx := setupQueryServer(t)
	setupDefaultDomains(t, k, ctx)

	resp, err := qs.Domains(ctx, &types.QueryDomainsRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Domains, 4)
}

// ─── DomainStats ─────────────────────────────────────────────────────────────

func TestQuery_DomainStats(t *testing.T) {
	qs, k, ctx := setupQueryServer(t)
	createTestSample(t, k, ctx, "s1", "science", testAddr, types.SampleStatus_SAMPLE_STATUS_GOLD, types.SampleType_SAMPLE_TYPE_DISCUSSION)
	createTestSample(t, k, ctx, "s2", "science", testAddr, types.SampleStatus_SAMPLE_STATUS_SILVER, types.SampleType_SAMPLE_TYPE_EXPLANATION)
	createTestSample(t, k, ctx, "s3", "science", testAddr, types.SampleStatus_SAMPLE_STATUS_PENDING, types.SampleType_SAMPLE_TYPE_DISCUSSION)
	createTestSample(t, k, ctx, "s4", "technology", testAddr, types.SampleStatus_SAMPLE_STATUS_GOLD, types.SampleType_SAMPLE_TYPE_DISCUSSION)

	resp, err := qs.DomainStats(ctx, &types.QueryDomainStatsRequest{Domain: "science"})
	require.NoError(t, err)
	require.Equal(t, "science", resp.Domain)
	require.Equal(t, uint64(3), resp.SampleCount)
	require.Equal(t, uint64(1), resp.GoldCount)
	require.Equal(t, uint64(1), resp.SilverCount)
	require.Equal(t, uint64(1), resp.PendingCount)
	require.Equal(t, uint64(0), resp.BronzeCount)
}

// ─── ProtocolStats ───────────────────────────────────────────────────────────

func TestQuery_ProtocolStats(t *testing.T) {
	qs, k, ctx := setupQueryServer(t)
	createTestSample(t, k, ctx, "s1", "science", testAddr, types.SampleStatus_SAMPLE_STATUS_GOLD, types.SampleType_SAMPLE_TYPE_DISCUSSION)
	createTestSample(t, k, ctx, "s2", "technology", testAddr, types.SampleStatus_SAMPLE_STATUS_GOLD, types.SampleType_SAMPLE_TYPE_EXPLANATION)

	require.NoError(t, k.SetSubmission(ctx, &types.Submission{Id: "sub1"}))

	resp, err := qs.ProtocolStats(ctx, &types.QueryProtocolStatsRequest{})
	require.NoError(t, err)
	require.Equal(t, uint64(2), resp.TotalSamples)
	require.Equal(t, uint64(1), resp.TotalSubmissions)
	require.Equal(t, uint64(2), resp.TotalDomains) // science + technology
}

// ─── Params ──────────────────────────────────────────────────────────────────

func TestQuery_Params(t *testing.T) {
	qs, k, ctx := setupQueryServer(t)
	setDefaultParams(t, k, ctx)

	resp, err := qs.Params(ctx, &types.QueryParamsRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp.Params)
}

// ─── DataBounties ────────────────────────────────────────────────────────────

func TestQuery_DataBounties_FilterByDomain(t *testing.T) {
	qs, k, ctx := setupQueryServer(t)
	require.NoError(t, k.SetDataBounty(ctx, &types.DataBounty{Id: "b1", Domain: "science"}))
	require.NoError(t, k.SetDataBounty(ctx, &types.DataBounty{Id: "b2", Domain: "technology"}))
	require.NoError(t, k.SetDataBounty(ctx, &types.DataBounty{Id: "b3", Domain: "science"}))

	resp, err := qs.DataBounties(ctx, &types.QueryDataBountiesRequest{Domain: "science"})
	require.NoError(t, err)
	require.Len(t, resp.Bounties, 2)
}
