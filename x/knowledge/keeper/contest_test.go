package keeper_test

import (
	"bytes"
	"context"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/keeper"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// challengerAddr returns a valid bech32 address with "zrn" prefix distinct from testAddr.
// Must be called after TestMain sets the bech32 prefix.
func challengerAddr() string {
	return sdk.AccAddress(bytes.Repeat([]byte{1}, 20)).String()
}

// createGoldSample is a test helper that sets up a gold-tier sample owned by testAddr.
func createGoldSample(t *testing.T, k keeper.Keeper, ctx context.Context, sampleID string) *types.Sample {
	t.Helper()
	sample := &types.Sample{
		Id:             sampleID,
		Content:        "gold sample content",
		SampleType:     types.SampleType_SAMPLE_TYPE_DISCUSSION,
		Domain:         "technology",
		SourceUri:      "https://example.com/sample",
		SourcePlatform: "web",
		Submitter:      testAddr,
		OriginalAuthor: "author",
		License:        "MIT",
		Tags:           []string{"golang"},
		Language:       "en",
		Status:         types.SampleStatus_SAMPLE_STATUS_GOLD,
	}
	require.NoError(t, k.SetSample(ctx, sample))
	return sample
}

func TestContestSample_Success(t *testing.T) {
	challenger := challengerAddr()
	k, ctx, bk := setupKeeperWithBank(t)
	setupDefaultDomains(t, k, ctx)

	createGoldSample(t, k, ctx, "sample-1")

	msg := &types.MsgContestSample{
		Challenger:  challenger,
		SampleId:    "sample-1",
		Stake:       "1000000",
		Reason:      "quality was mis-scored",
		ContestType: types.ContestType_CONTEST_TYPE_QUALITY,
	}

	resp, err := k.ContestSample(ctx, msg)
	require.NoError(t, err)
	require.NotEmpty(t, resp.RoundId)

	// Sample status should be CONTESTED
	updated, found := k.GetSample(ctx, "sample-1")
	require.True(t, found)
	require.Equal(t, types.SampleStatus_SAMPLE_STATUS_CONTESTED, updated.Status)

	// Stake should be locked via bank
	require.Len(t, bk.accountToModuleCalls, 1)
	require.Equal(t, challenger, bk.accountToModuleCalls[0].from)
	require.Equal(t, types.ModuleName, bk.accountToModuleCalls[0].to)

	// Quality round should exist
	round, found := k.GetQualityRound(ctx, resp.RoundId)
	require.True(t, found)
	require.NotEmpty(t, round.SubmissionId)

	// Contest index should be set
	gotRoundID, found := k.GetContestRound(ctx, "sample-1")
	require.True(t, found)
	require.Equal(t, resp.RoundId, gotRoundID)

	// Re-validation submission should exist
	sub, found := k.GetSubmission(ctx, round.SubmissionId)
	require.True(t, found)
	require.Equal(t, challenger, sub.Submitter)
	require.Equal(t, "gold sample content", sub.Content)
	require.Equal(t, "1000000", sub.Stake)
}

func TestContestSample_SampleNotFound(t *testing.T) {
	challenger := challengerAddr()
	k, ctx := setupKeeper(t)

	msg := &types.MsgContestSample{
		Challenger:  challenger,
		SampleId:    "nonexistent",
		Stake:       "1000000",
		ContestType: types.ContestType_CONTEST_TYPE_QUALITY,
	}

	_, err := k.ContestSample(ctx, msg)
	require.ErrorIs(t, err, types.ErrSampleNotFound)
}

func TestContestSample_CannotContestPruned(t *testing.T) {
	challenger := challengerAddr()
	k, ctx := setupKeeper(t)

	require.NoError(t, k.SetSample(ctx, &types.Sample{
		Id:        "pruned-1",
		Submitter: testAddr,
		Status:    types.SampleStatus_SAMPLE_STATUS_PRUNED,
	}))

	msg := &types.MsgContestSample{
		Challenger:  challenger,
		SampleId:    "pruned-1",
		Stake:       "1000000",
		ContestType: types.ContestType_CONTEST_TYPE_QUALITY,
	}

	_, err := k.ContestSample(ctx, msg)
	require.ErrorIs(t, err, types.ErrInvalidChallenge)
}

func TestContestSample_CannotContestRejected(t *testing.T) {
	challenger := challengerAddr()
	k, ctx := setupKeeper(t)

	require.NoError(t, k.SetSample(ctx, &types.Sample{
		Id:        "rejected-1",
		Submitter: testAddr,
		Status:    types.SampleStatus_SAMPLE_STATUS_REJECTED,
	}))

	msg := &types.MsgContestSample{
		Challenger:  challenger,
		SampleId:    "rejected-1",
		Stake:       "1000000",
		ContestType: types.ContestType_CONTEST_TYPE_QUALITY,
	}

	_, err := k.ContestSample(ctx, msg)
	require.ErrorIs(t, err, types.ErrInvalidChallenge)
}

func TestContestSample_AlreadyContested(t *testing.T) {
	challenger := challengerAddr()
	k, ctx := setupKeeper(t)

	createGoldSample(t, k, ctx, "sample-2")

	// Set contest index first (simulating an existing contest)
	require.NoError(t, k.SetContestIndex(ctx, "sample-2", "existing-round"))

	msg := &types.MsgContestSample{
		Challenger:  challenger,
		SampleId:    "sample-2",
		Stake:       "1000000",
		ContestType: types.ContestType_CONTEST_TYPE_QUALITY,
	}

	_, err := k.ContestSample(ctx, msg)
	require.ErrorIs(t, err, types.ErrDuplicateChallenge)
}

func TestContestSample_SelfChallenge(t *testing.T) {
	k, ctx := setupKeeper(t)

	createGoldSample(t, k, ctx, "sample-3")

	// testAddr is the sample submitter — using it as challenger should fail
	msg := &types.MsgContestSample{
		Challenger:  testAddr,
		SampleId:    "sample-3",
		Stake:       "1000000",
		ContestType: types.ContestType_CONTEST_TYPE_QUALITY,
	}

	_, err := k.ContestSample(ctx, msg)
	require.ErrorIs(t, err, types.ErrSelfChallenge)
}

func TestContestSample_InsufficientStake(t *testing.T) {
	challenger := challengerAddr()
	k, ctx := setupKeeper(t)

	createGoldSample(t, k, ctx, "sample-4")

	msg := &types.MsgContestSample{
		Challenger:  challenger,
		SampleId:    "sample-4",
		Stake:       "100", // Way below min_submission_stake of 1000000
		ContestType: types.ContestType_CONTEST_TYPE_QUALITY,
	}

	_, err := k.ContestSample(ctx, msg)
	require.ErrorIs(t, err, types.ErrInsufficientStake)
}

func TestContestSample_ConsentType_LowerStake(t *testing.T) {
	challenger := challengerAddr()
	k, ctx, bk := setupKeeperWithBank(t)
	setupDefaultDomains(t, k, ctx)

	createGoldSample(t, k, ctx, "sample-5")

	// Consent contest requires half stake (500000)
	msg := &types.MsgContestSample{
		Challenger:  challenger,
		SampleId:    "sample-5",
		Stake:       "500000",
		Reason:      "consent proof is invalid",
		ContestType: types.ContestType_CONTEST_TYPE_CONSENT,
	}

	resp, err := k.ContestSample(ctx, msg)
	require.NoError(t, err)
	require.NotEmpty(t, resp.RoundId)

	// Verify bank was called with 500000
	require.Len(t, bk.accountToModuleCalls, 1)
	require.Equal(t, sdk.NewInt64Coin("uzrn", 500000), bk.accountToModuleCalls[0].amount[0])
}

func TestContestSample_DuplicateType(t *testing.T) {
	challenger := challengerAddr()
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	createGoldSample(t, k, ctx, "sample-6")

	msg := &types.MsgContestSample{
		Challenger:  challenger,
		SampleId:    "sample-6",
		Stake:       "1000000",
		Reason:      "already exists in dataset",
		ContestType: types.ContestType_CONTEST_TYPE_DUPLICATE,
	}

	resp, err := k.ContestSample(ctx, msg)
	require.NoError(t, err)
	require.NotEmpty(t, resp.RoundId)

	// Verify round was created
	_, found := k.GetQualityRound(ctx, resp.RoundId)
	require.True(t, found)
}

func TestContestSample_StakeLockFails(t *testing.T) {
	challenger := challengerAddr()
	k, ctx, bk := setupKeeperWithBank(t)

	createGoldSample(t, k, ctx, "sample-7")

	// Tell mock bank to fail next send
	bk.failNextSend = true

	msg := &types.MsgContestSample{
		Challenger:  challenger,
		SampleId:    "sample-7",
		Stake:       "1000000",
		ContestType: types.ContestType_CONTEST_TYPE_QUALITY,
	}

	_, err := k.ContestSample(ctx, msg)
	require.ErrorIs(t, err, types.ErrInsufficientStake)

	// Sample status should NOT have changed
	sample, found := k.GetSample(ctx, "sample-7")
	require.True(t, found)
	require.Equal(t, types.SampleStatus_SAMPLE_STATUS_GOLD, sample.Status)
}

func TestContestSample_BronzeSample(t *testing.T) {
	challenger := challengerAddr()
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	require.NoError(t, k.SetSample(ctx, &types.Sample{
		Id:        "bronze-1",
		Content:   "bronze content",
		Domain:    "technology",
		Submitter: testAddr,
		Status:    types.SampleStatus_SAMPLE_STATUS_BRONZE,
	}))

	msg := &types.MsgContestSample{
		Challenger:  challenger,
		SampleId:    "bronze-1",
		Stake:       "1000000",
		ContestType: types.ContestType_CONTEST_TYPE_QUALITY,
	}

	resp, err := k.ContestSample(ctx, msg)
	require.NoError(t, err)
	require.NotEmpty(t, resp.RoundId)

	updated, found := k.GetSample(ctx, "bronze-1")
	require.True(t, found)
	require.Equal(t, types.SampleStatus_SAMPLE_STATUS_CONTESTED, updated.Status)
}

func TestContestSample_CopyrightType(t *testing.T) {
	challenger := challengerAddr()
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	createGoldSample(t, k, ctx, "sample-8")

	msg := &types.MsgContestSample{
		Challenger:  challenger,
		SampleId:    "sample-8",
		Stake:       "1000000",
		Reason:      "copyright violation",
		ContestType: types.ContestType_CONTEST_TYPE_COPYRIGHT,
	}

	resp, err := k.ContestSample(ctx, msg)
	require.NoError(t, err)
	require.NotEmpty(t, resp.RoundId)

	_, found := k.GetQualityRound(ctx, resp.RoundId)
	require.True(t, found)
}

func TestContestSample_EmitsEvent(t *testing.T) {
	challenger := challengerAddr()
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	createGoldSample(t, k, ctx, "sample-9")

	msg := &types.MsgContestSample{
		Challenger:  challenger,
		SampleId:    "sample-9",
		Stake:       "1000000",
		Reason:      "test reason",
		ContestType: types.ContestType_CONTEST_TYPE_QUALITY,
	}

	_, err := k.ContestSample(ctx, msg)
	require.NoError(t, err)

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	events := sdkCtx.EventManager().Events()

	found := false
	for _, e := range events {
		if e.Type == "sample_contested" {
			found = true
			// Verify key attributes
			attrMap := make(map[string]string)
			for _, attr := range e.Attributes {
				attrMap[attr.Key] = attr.Value
			}
			require.Equal(t, "sample-9", attrMap["sample_id"])
			require.Equal(t, challenger, attrMap["challenger"])
			require.Equal(t, types.ContestType_CONTEST_TYPE_QUALITY.String(), attrMap["contest_type"])
			require.Equal(t, "1000000", attrMap["stake"])
			require.Equal(t, types.SampleStatus_SAMPLE_STATUS_GOLD.String(), attrMap["previous_status"])
			require.Equal(t, "test reason", attrMap["reason"])
			require.Equal(t, "100", attrMap["block"])
			break
		}
	}
	require.True(t, found, "expected sample_contested event")
}
