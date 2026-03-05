package keeper_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/keeper"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── CreateProjectBounty ────────────────────────────────────────────────────

func TestCreateProjectBounty_Success(t *testing.T) {
	k, ctx := setupKeeper(t)

	budget := sdk.NewCoins(sdk.NewInt64Coin("uzrn", 1_000_000))
	err := k.CreateProjectBounty(ctx, "science", 50, 500_000, budget, "proj-1")
	require.NoError(t, err)

	// Verify bounty was created
	var found bool
	k.IterateDataBounties(ctx, func(b *types.DataBounty) bool {
		if strings.Contains(b.Subject, "project:proj-1:") {
			found = true
			require.Equal(t, "science", b.Domain)
			require.Equal(t, "1000000", b.RewardAmount)
			require.Equal(t, uint64(50), b.DemandCount)
			require.False(t, b.Claimed)
			return true
		}
		return false
	})
	require.True(t, found, "project bounty should be created")
}

func TestCreateProjectBounty_ZeroBudget(t *testing.T) {
	k, ctx := setupKeeper(t)

	err := k.CreateProjectBounty(ctx, "science", 10, 200_000, sdk.NewCoins(), "proj-2")
	require.NoError(t, err)

	var found bool
	k.IterateDataBounties(ctx, func(b *types.DataBounty) bool {
		if strings.Contains(b.Subject, "project:proj-2:") {
			found = true
			require.Equal(t, "0", b.RewardAmount)
			return true
		}
		return false
	})
	require.True(t, found)
}

func TestCreateProjectBounty_MultipleBounties(t *testing.T) {
	k, ctx := setupKeeper(t)

	budget := sdk.NewCoins(sdk.NewInt64Coin("uzrn", 500_000))
	require.NoError(t, k.CreateProjectBounty(ctx, "science", 20, 300_000, budget, "proj-a"))
	require.NoError(t, k.CreateProjectBounty(ctx, "math", 30, 400_000, budget, "proj-b"))

	count := 0
	k.IterateDataBounties(ctx, func(b *types.DataBounty) bool {
		count++
		return false
	})
	require.Equal(t, 2, count)
}

// ─── GetBountyProgress ──────────────────────────────────────────────────────

func TestGetBountyProgress_NotFound(t *testing.T) {
	k, ctx := setupKeeper(t)

	_, _, found := k.GetBountyProgress(ctx, "nonexistent")
	require.False(t, found)
}

func TestGetBountyProgress_NoSamplesYet(t *testing.T) {
	k, ctx := setupKeeper(t)

	budget := sdk.NewCoins(sdk.NewInt64Coin("uzrn", 1_000_000))
	require.NoError(t, k.CreateProjectBounty(ctx, "physics", 25, 500_000, budget, "proj-progress"))

	current, target, found := k.GetBountyProgress(ctx, "proj-progress")
	require.True(t, found)
	require.Equal(t, uint64(25), target)
	require.Equal(t, uint64(0), current)
}

func TestGetBountyProgress_WithAcceptedSamples(t *testing.T) {
	k, ctx := setupKeeper(t)

	budget := sdk.NewCoins(sdk.NewInt64Coin("uzrn", 1_000_000))
	require.NoError(t, k.CreateProjectBounty(ctx, "physics", 10, 500_000, budget, "proj-prog2"))

	// Add some samples in the domain
	storeSampleWithContent(t, k, ctx, "ps1", "physics content 1", "physics", types.SampleStatus_SAMPLE_STATUS_GOLD)
	require.NoError(t, k.SetSampleDomainIndex(ctx, "physics", "ps1"))
	storeSampleWithContent(t, k, ctx, "ps2", "physics content 2", "physics", types.SampleStatus_SAMPLE_STATUS_SILVER)
	require.NoError(t, k.SetSampleDomainIndex(ctx, "physics", "ps2"))
	// Pending should not count
	storeSampleWithContent(t, k, ctx, "ps3", "physics content 3", "physics", types.SampleStatus_SAMPLE_STATUS_PENDING)
	require.NoError(t, k.SetSampleDomainIndex(ctx, "physics", "ps3"))

	current, target, found := k.GetBountyProgress(ctx, "proj-prog2")
	require.True(t, found)
	require.Equal(t, uint64(10), target)
	require.Equal(t, uint64(2), current) // only gold + silver count
}

// ─── SendProtocolRevenue ────────────────────────────────────────────────────

func TestSendProtocolRevenue_NoVestingKeeper(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Without vesting keeper wired, should be no-op
	amount := sdk.NewCoins(sdk.NewInt64Coin("uzrn", 100_000))
	err := k.SendProtocolRevenue(ctx, amount)
	require.NoError(t, err) // no-op, no error
}

// ─── ToolboxKnowledgeAdapter ────────────────────────────────────────────────

func newToolboxAdapter(t *testing.T) (*keeper.ToolboxKnowledgeAdapter, keeper.Keeper, context.Context) {
	t.Helper()
	k, ctx := setupKeeper(t)
	adapter := keeper.NewToolboxKnowledgeAdapter(k)
	return adapter, k, ctx
}

func TestToolboxAdapter_GetFactConfidence(t *testing.T) {
	adapter, k, ctx := newToolboxAdapter(t)

	sample := &types.Sample{
		Id:           "s1",
		Content:      "test content",
		Domain:       "science",
		Status:       types.SampleStatus_SAMPLE_STATUS_GOLD,
		QualityScore: 850_000,
		Submitter:    testAddr,
	}
	require.NoError(t, k.SetSample(ctx, sample))

	conf, found := adapter.GetFactConfidence(ctx, "s1")
	require.True(t, found)
	require.Equal(t, uint64(850_000), conf)
}

func TestToolboxAdapter_GetFactConfidence_NotFound(t *testing.T) {
	adapter, _, ctx := newToolboxAdapter(t)

	_, found := adapter.GetFactConfidence(ctx, "nonexistent")
	require.False(t, found)
}

func TestToolboxAdapter_SearchFactsByContent(t *testing.T) {
	adapter, k, ctx := newToolboxAdapter(t)

	// Add samples with content
	s1 := &types.Sample{Id: "s1", Content: "quantum mechanics and wave functions", Domain: "physics", Status: types.SampleStatus_SAMPLE_STATUS_GOLD, Submitter: testAddr}
	s2 := &types.Sample{Id: "s2", Content: "classical mechanics and newton laws", Domain: "physics", Status: types.SampleStatus_SAMPLE_STATUS_SILVER, Submitter: testAddr}
	s3 := &types.Sample{Id: "s3", Content: "organic chemistry reactions", Domain: "physics", Status: types.SampleStatus_SAMPLE_STATUS_GOLD, Submitter: testAddr}
	require.NoError(t, k.SetSample(ctx, s1))
	require.NoError(t, k.SetSample(ctx, s2))
	require.NoError(t, k.SetSample(ctx, s3))
	require.NoError(t, k.SetSampleDomainIndex(ctx, "physics", "s1"))
	require.NoError(t, k.SetSampleDomainIndex(ctx, "physics", "s2"))
	require.NoError(t, k.SetSampleDomainIndex(ctx, "physics", "s3"))

	// Search for "mechanics"
	results, err := adapter.SearchFactsByContent(ctx, "physics", []string{"mechanics"}, 10)
	require.NoError(t, err)
	require.Len(t, results, 2) // s1 and s2 match

	// Search for "quantum"
	results, err = adapter.SearchFactsByContent(ctx, "physics", []string{"quantum"}, 10)
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, "s1", results[0])
}

func TestToolboxAdapter_SearchFactsByContent_MaxResults(t *testing.T) {
	adapter, k, ctx := newToolboxAdapter(t)

	for i := 0; i < 5; i++ {
		s := &types.Sample{
			Id:        "sm" + string(rune('0'+i)),
			Content:   "common term in all samples",
			Domain:    "test",
			Status:    types.SampleStatus_SAMPLE_STATUS_GOLD,
			Submitter: testAddr,
		}
		require.NoError(t, k.SetSample(ctx, s))
		require.NoError(t, k.SetSampleDomainIndex(ctx, "test", s.Id))
	}

	results, err := adapter.SearchFactsByContent(ctx, "test", []string{"common"}, 3)
	require.NoError(t, err)
	require.LessOrEqual(t, len(results), 3)
}

func TestToolboxAdapter_SearchFactsByContent_SkipsRevoked(t *testing.T) {
	adapter, k, ctx := newToolboxAdapter(t)

	s1 := &types.Sample{Id: "sr1", Content: "[consent revoked]", Domain: "test", Status: types.SampleStatus_SAMPLE_STATUS_PRUNED, Submitter: testAddr}
	s2 := &types.Sample{Id: "sr2", Content: "valid content with term", Domain: "test", Status: types.SampleStatus_SAMPLE_STATUS_GOLD, Submitter: testAddr}
	require.NoError(t, k.SetSample(ctx, s1))
	require.NoError(t, k.SetSample(ctx, s2))
	require.NoError(t, k.SetSampleDomainIndex(ctx, "test", "sr1"))
	require.NoError(t, k.SetSampleDomainIndex(ctx, "test", "sr2"))

	results, err := adapter.SearchFactsByContent(ctx, "test", []string{"content"}, 10)
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, "sr2", results[0])
}

func TestToolboxAdapter_GetFactDetails(t *testing.T) {
	adapter, k, ctx := newToolboxAdapter(t)

	sample := &types.Sample{
		Id:           "sd1",
		Content:      "detailed fact content",
		Domain:       "science",
		Status:       types.SampleStatus_SAMPLE_STATUS_GOLD,
		QualityScore: 750_000,
		AccessCount:  42,
		Submitter:    testAddr,
	}
	require.NoError(t, k.SetSample(ctx, sample))

	content, confidence, citations, err := adapter.GetFactDetails(ctx, "sd1")
	require.NoError(t, err)
	require.Equal(t, "detailed fact content", content)
	require.Equal(t, uint64(750_000), confidence)
	require.Equal(t, uint64(42), citations)
}

func TestToolboxAdapter_GetFactDetails_NotFound(t *testing.T) {
	adapter, _, ctx := newToolboxAdapter(t)

	_, _, _, err := adapter.GetFactDetails(ctx, "nonexistent")
	require.Error(t, err)
}

func TestToolboxAdapter_RecordFactCitation(t *testing.T) {
	adapter, k, ctx := newToolboxAdapter(t)

	sample := &types.Sample{
		Id:          "sc1",
		Content:     "citable content",
		Domain:      "science",
		Status:      types.SampleStatus_SAMPLE_STATUS_GOLD,
		AccessCount: 10,
		Submitter:   testAddr,
	}
	require.NoError(t, k.SetSample(ctx, sample))

	err := adapter.RecordFactCitation(ctx, "sc1", "tool-123")
	require.NoError(t, err)

	// Verify access count incremented
	updated, found := k.GetSample(ctx, "sc1")
	require.True(t, found)
	require.Equal(t, uint64(11), updated.AccessCount)
}

func TestToolboxAdapter_RecordFactCitation_NotFound(t *testing.T) {
	adapter, _, ctx := newToolboxAdapter(t)

	err := adapter.RecordFactCitation(ctx, "nonexistent", "tool-1")
	require.Error(t, err)
}

// ─── TreeKnowledgeAdapter compile-time check ────────────────────────────────

func TestTreeKnowledgeAdapter_InterfaceCompliance(t *testing.T) {
	k, _ := setupKeeper(t)
	adapter := keeper.NewTreeKnowledgeAdapter(k)
	require.NotNil(t, adapter)
}
