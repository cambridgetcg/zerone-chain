package keeper_test

import (
	"context"
	"testing"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/keeper"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Helpers ────────────────────────────────────────────────────────────────

func setupBountyTest(t *testing.T) (keeper.Keeper, context.Context, *mockBankKeeper) {
	t.Helper()
	k, ctx, bk := setupKeeperWithBank(t)
	setupDefaultDomains(t, k, ctx)

	// Create a funded bounty.
	resp, err := k.FundBounty(ctx, &types.MsgFundBounty{
		Funder:        testAddr,
		Domain:        "technology",
		Topic:         "golang",
		Amount:        "10000000", // 10 ZRN
		ExpiresBlocks: 100_000,
	})
	require.NoError(t, err)
	require.NotEmpty(t, resp.BountyId)

	return k, ctx, bk
}

func getBountyID(t *testing.T, k keeper.Keeper, ctx context.Context) string {
	t.Helper()
	bounties := k.GetActiveBounties(ctx, "technology")
	require.NotEmpty(t, bounties)
	return bounties[0].Id
}

// seed a fitness record for a sample to enable scoring.
func seedFitness(t *testing.T, k keeper.Keeper, ctx context.Context, sampleID string, score string) {
	t.Helper()
	dec, err := sdkmath.LegacyNewDecFromStr(score)
	require.NoError(t, err)
	rec := types.TDUFitnessRecord{SampleID: sampleID}
	rec.SetFitnessScore(dec)
	require.NoError(t, k.SetFitnessRecord(ctx, rec))
}

// ─── Test: Submit to Bounty — Happy Path ────────────────────────────────────

func TestSubmitToBounty_HappyPath(t *testing.T) {
	k, ctx, _ := setupBountyTest(t)
	bountyID := getBountyID(t, k, ctx)

	sub, err := k.SubmitToBounty(ctx, bountyID, "sample-001", testAddr)
	require.NoError(t, err)
	require.NotEmpty(t, sub.SubmissionID)
	require.Equal(t, bountyID, sub.BountyID)
	require.Equal(t, "sample-001", sub.SampleID)
	require.Equal(t, testAddr, sub.Submitter)
	require.Equal(t, int64(100), sub.SubmittedAt) // block height from setup

	// Verify competitive bounty was created.
	comp, found := k.GetCompetitiveBounty(ctx, bountyID)
	require.True(t, found)
	require.Equal(t, keeper.BountyStatusCompeting, comp.Status)
	require.Equal(t, uint64(1), comp.SubmissionCount)
	require.Equal(t, "10000000", comp.TotalPool)
}

// ─── Test: Submit to Bounty — Multiple Submissions ──────────────────────────

func TestSubmitToBounty_MultipleSubmitters(t *testing.T) {
	k, ctx, _ := setupBountyTest(t)
	bountyID := getBountyID(t, k, ctx)

	addr2 := "zrn1qqqsyqcyq5rqwzqfpg9scrgwpugpzysnv2f5q0" // different

	_, err := k.SubmitToBounty(ctx, bountyID, "sample-001", testAddr)
	require.NoError(t, err)

	_, err = k.SubmitToBounty(ctx, bountyID, "sample-002", addr2)
	require.NoError(t, err)

	comp, found := k.GetCompetitiveBounty(ctx, bountyID)
	require.True(t, found)
	require.Equal(t, uint64(2), comp.SubmissionCount)

	// Verify submissions are retrievable.
	subs := k.GetBountySubmissions(ctx, bountyID)
	require.Len(t, subs, 2)
}

// ─── Test: Submit to Bounty — Duplicate Rejected ────────────────────────────

func TestSubmitToBounty_DuplicateRejected(t *testing.T) {
	k, ctx, _ := setupBountyTest(t)
	bountyID := getBountyID(t, k, ctx)

	_, err := k.SubmitToBounty(ctx, bountyID, "sample-001", testAddr)
	require.NoError(t, err)

	_, err = k.SubmitToBounty(ctx, bountyID, "sample-002", testAddr)
	require.ErrorIs(t, err, types.ErrBountyDuplicateSubmission)
}

// ─── Test: Submit to Bounty — Not Found ─────────────────────────────────────

func TestSubmitToBounty_NotFound(t *testing.T) {
	k, ctx, _ := setupBountyTest(t)

	_, err := k.SubmitToBounty(ctx, "nonexistent", "sample-001", testAddr)
	require.ErrorIs(t, err, types.ErrBountyNotFound)
}

// ─── Test: Submit to Bounty — Expired ───────────────────────────────────────

func TestSubmitToBounty_Expired(t *testing.T) {
	k, ctx, _ := setupBountyTest(t)
	bountyID := getBountyID(t, k, ctx)

	// Advance block past expiry.
	ctx = sdk.UnwrapSDKContext(ctx).WithBlockHeight(200_100)

	_, err := k.SubmitToBounty(ctx, bountyID, "sample-001", testAddr)
	require.ErrorIs(t, err, types.ErrBountyExpired)
}

// ─── Test: Submit to Bounty — Already Claimed ───────────────────────────────

func TestSubmitToBounty_AlreadyClaimed(t *testing.T) {
	k, ctx, _ := setupBountyTest(t)
	bountyID := getBountyID(t, k, ctx)

	// Manually mark as claimed.
	bounty, _ := k.GetDataBounty(ctx, bountyID)
	bounty.Claimed = true
	_ = k.SetDataBounty(ctx, bounty)

	_, err := k.SubmitToBounty(ctx, bountyID, "sample-001", testAddr)
	require.ErrorIs(t, err, types.ErrBountyAlreadyClaimed)
}

// ─── Test: Submit to Bounty — Past Submission Deadline ──────────────────────

func TestSubmitToBounty_PastDeadline(t *testing.T) {
	k, ctx, _ := setupBountyTest(t)
	bountyID := getBountyID(t, k, ctx)

	// First submission sets the deadline.
	_, err := k.SubmitToBounty(ctx, bountyID, "sample-001", testAddr)
	require.NoError(t, err)

	comp, _ := k.GetCompetitiveBounty(ctx, bountyID)
	require.True(t, comp.SubmissionDeadline > 0)

	// Advance past deadline.
	ctx = sdk.UnwrapSDKContext(ctx).WithBlockHeight(int64(comp.SubmissionDeadline + 1))

	addr2 := "zrn1qqqsyqcyq5rqwzqfpg9scrgwpugpzysnv2f5q0"
	_, err = k.SubmitToBounty(ctx, bountyID, "sample-002", addr2)
	require.ErrorIs(t, err, types.ErrBountySubmissionClosed)
}

// ─── Test: Resolve Bounty — Single Submission ───────────────────────────────

func TestResolveBounty_SingleSubmission(t *testing.T) {
	k, ctx, bk := setupBountyTest(t)
	bountyID := getBountyID(t, k, ctx)

	_, err := k.SubmitToBounty(ctx, bountyID, "sample-001", testAddr)
	require.NoError(t, err)

	// Seed fitness score.
	seedFitness(t, k, ctx, "sample-001", "0.75")

	// Resolve.
	resolution, err := k.ResolveBounty(ctx, bountyID)
	require.NoError(t, err)
	require.Len(t, resolution.Winners, 1)
	require.Equal(t, "sample-001", resolution.Winners[0].SampleID)
	require.Equal(t, uint64(1), resolution.Winners[0].Rank)

	// Winner gets 50% (first tier).
	require.Equal(t, "5000000", resolution.Winners[0].Reward) // 50% of 10M

	// Remainder is 50%.
	require.Equal(t, "5000000", resolution.Remainder)

	// Verify competitive bounty is resolved.
	comp, found := k.GetCompetitiveBounty(ctx, bountyID)
	require.True(t, found)
	require.Equal(t, keeper.BountyStatusResolved, comp.Status)
	require.Len(t, comp.WinnerIDs, 1)

	// Verify base bounty is claimed.
	baseBounty, _ := k.GetDataBounty(ctx, bountyID)
	require.True(t, baseBounty.Claimed)

	// Verify bank transfer was called.
	require.NotEmpty(t, bk.moduleToAccountCalls)
}

// ─── Test: Resolve Bounty — Ranked Rewards (3 Submissions) ─────────────────

func TestResolveBounty_RankedRewards(t *testing.T) {
	k, ctx, _ := setupBountyTest(t)
	bountyID := getBountyID(t, k, ctx)

	// Three different submitters.
	addrs := []string{
		testAddr,
		"zrn1qqqsyqcyq5rqwzqfpg9scrgwpugpzysnv2f5q0",
		"zrn1qcxce9c4thzxnfmpr2dqnnlqea9ey35ydj769h",
	}

	for i, addr := range addrs {
		_, err := k.SubmitToBounty(ctx, bountyID, "sample-"+string(rune('a'+i)), addr)
		require.NoError(t, err)
	}

	// Seed fitness: b > a > c.
	seedFitness(t, k, ctx, "sample-a", "0.60")
	seedFitness(t, k, ctx, "sample-b", "0.85")
	seedFitness(t, k, ctx, "sample-c", "0.40")

	resolution, err := k.ResolveBounty(ctx, bountyID)
	require.NoError(t, err)
	require.Len(t, resolution.Winners, 3)

	// 1st place: sample-b (0.85) → 50% = 5,000,000
	require.Equal(t, "sample-b", resolution.Winners[0].SampleID)
	require.Equal(t, uint64(1), resolution.Winners[0].Rank)
	require.Equal(t, "5000000", resolution.Winners[0].Reward)

	// 2nd place: sample-a (0.60) → 30% = 3,000,000
	require.Equal(t, "sample-a", resolution.Winners[1].SampleID)
	require.Equal(t, uint64(2), resolution.Winners[1].Rank)
	require.Equal(t, "3000000", resolution.Winners[1].Reward)

	// 3rd place: sample-c (0.40) → 20% = 2,000,000
	require.Equal(t, "sample-c", resolution.Winners[2].SampleID)
	require.Equal(t, uint64(3), resolution.Winners[2].Rank)
	require.Equal(t, "2000000", resolution.Winners[2].Reward)

	// Total distributed = 10M, remainder = 0.
	require.Equal(t, "0", resolution.Remainder)
}

// ─── Test: Resolve Bounty — Tie-Breaking (Earlier Wins) ─────────────────────

func TestResolveBounty_TieBreaking(t *testing.T) {
	k, ctx, _ := setupBountyTest(t)
	bountyID := getBountyID(t, k, ctx)

	addr2 := "zrn1qqqsyqcyq5rqwzqfpg9scrgwpugpzysnv2f5q0"

	// First submission at block 100.
	_, err := k.SubmitToBounty(ctx, bountyID, "sample-early", testAddr)
	require.NoError(t, err)

	// Second submission at block 200.
	ctx = sdk.UnwrapSDKContext(ctx).WithBlockHeight(200)
	_, err = k.SubmitToBounty(ctx, bountyID, "sample-late", addr2)
	require.NoError(t, err)

	// Same fitness score.
	seedFitness(t, k, ctx, "sample-early", "0.70")
	seedFitness(t, k, ctx, "sample-late", "0.70")

	resolution, err := k.ResolveBounty(ctx, bountyID)
	require.NoError(t, err)

	// Earlier submission wins the tie.
	require.Equal(t, "sample-early", resolution.Winners[0].SampleID)
	require.Equal(t, "sample-late", resolution.Winners[1].SampleID)
}

// ─── Test: Resolve Bounty — No Submissions ─────────────────────────────────

func TestResolveBounty_NoSubmissions(t *testing.T) {
	k, ctx, _ := setupBountyTest(t)
	bountyID := getBountyID(t, k, ctx)

	// Create competitive bounty without submissions.
	params := k.GetBountyBoardParams(ctx)
	_ = k.SetCompetitiveBounty(ctx, &keeper.CompetitiveBounty{
		BountyID:         bountyID,
		Status:           keeper.BountyStatusCompeting,
		SubmissionWindow: params.DefaultSubmissionWindow,
		TotalPool:        "10000000",
	})

	resolution, err := k.ResolveBounty(ctx, bountyID)
	require.NoError(t, err)
	require.Len(t, resolution.Winners, 0)
	require.Equal(t, "10000000", resolution.Remainder) // all funds returned

	// Bounty should be expired.
	comp, _ := k.GetCompetitiveBounty(ctx, bountyID)
	require.Equal(t, keeper.BountyStatusExpired, comp.Status)
}

// ─── Test: Resolve Bounty — Wrong State ─────────────────────────────────────

func TestResolveBounty_WrongState(t *testing.T) {
	k, ctx, _ := setupBountyTest(t)
	bountyID := getBountyID(t, k, ctx)

	// Create as already resolved.
	_ = k.SetCompetitiveBounty(ctx, &keeper.CompetitiveBounty{
		BountyID:  bountyID,
		Status:    keeper.BountyStatusResolved,
		TotalPool: "10000000",
	})

	_, err := k.ResolveBounty(ctx, bountyID)
	require.ErrorIs(t, err, types.ErrBountyNotResolvable)
}

// ─── Test: Add to Bounty Pool ───────────────────────────────────────────────

func TestAddToBountyPool(t *testing.T) {
	k, ctx, _ := setupBountyTest(t)
	bountyID := getBountyID(t, k, ctx)

	addr2 := "zrn1qqqsyqcyq5rqwzqfpg9scrgwpugpzysnv2f5q0"
	err := k.AddToBountyPool(ctx, bountyID, addr2, "5000000")
	require.NoError(t, err)

	comp, found := k.GetCompetitiveBounty(ctx, bountyID)
	require.True(t, found)
	require.Equal(t, "15000000", comp.TotalPool) // 10M + 5M
	require.Len(t, comp.Funders, 2)              // original auto + new
	require.Equal(t, addr2, comp.Funders[1].Address)
	require.Equal(t, "5000000", comp.Funders[1].Amount)

	// Base bounty should also be updated.
	baseBounty, _ := k.GetDataBounty(ctx, bountyID)
	require.Equal(t, "15000000", baseBounty.RewardAmount)
}

// ─── Test: Add to Pool — Resolved Bounty Rejected ───────────────────────────

func TestAddToBountyPool_ResolvedRejected(t *testing.T) {
	k, ctx, _ := setupBountyTest(t)
	bountyID := getBountyID(t, k, ctx)

	_ = k.SetCompetitiveBounty(ctx, &keeper.CompetitiveBounty{
		BountyID:  bountyID,
		Status:    keeper.BountyStatusResolved,
		TotalPool: "10000000",
	})

	err := k.AddToBountyPool(ctx, bountyID, testAddr, "5000000")
	require.ErrorIs(t, err, types.ErrBountyNotAccepting)
}

// ─── Test: Cancel Bounty — No Submissions ───────────────────────────────────

func TestCancelBounty_NoSubmissions(t *testing.T) {
	k, ctx, bk := setupBountyTest(t)
	bountyID := getBountyID(t, k, ctx)

	// Add a real funder so we can verify refund.
	_ = k.SetCompetitiveBounty(ctx, &keeper.CompetitiveBounty{
		BountyID: bountyID,
		Status:   keeper.BountyStatusOpen,
		TotalPool: "10000000",
		Funders: []keeper.BountyFunder{
			{Address: testAddr, Amount: "10000000", Block: 100},
		},
	})

	err := k.CancelBounty(ctx, bountyID, testAddr)
	require.NoError(t, err)

	comp, found := k.GetCompetitiveBounty(ctx, bountyID)
	require.True(t, found)
	require.Equal(t, keeper.BountyStatusCancelled, comp.Status)

	// Verify refund was attempted.
	require.NotEmpty(t, bk.moduleToAccountCalls)
}

// ─── Test: Cancel Bounty — With Submissions Rejected ────────────────────────

func TestCancelBounty_WithSubmissionsRejected(t *testing.T) {
	k, ctx, _ := setupBountyTest(t)
	bountyID := getBountyID(t, k, ctx)

	// Submit first.
	_, err := k.SubmitToBounty(ctx, bountyID, "sample-001", testAddr)
	require.NoError(t, err)

	// Try to cancel — should fail (status is COMPETING, not OPEN).
	err = k.CancelBounty(ctx, bountyID, testAddr)
	require.ErrorIs(t, err, types.ErrBountyCancelFailed)
}

// ─── Test: Bounty Leaderboard ───────────────────────────────────────────────

func TestGetBountyLeaderboard(t *testing.T) {
	k, ctx, _ := setupBountyTest(t)
	bountyID := getBountyID(t, k, ctx)

	addrs := []string{
		testAddr,
		"zrn1qqqsyqcyq5rqwzqfpg9scrgwpugpzysnv2f5q0",
		"zrn1qcxce9c4thzxnfmpr2dqnnlqea9ey35ydj769h",
	}

	for i, addr := range addrs {
		_, err := k.SubmitToBounty(ctx, bountyID, "sample-"+string(rune('x'+i)), addr)
		require.NoError(t, err)
	}

	seedFitness(t, k, ctx, "sample-x", "0.30")
	seedFitness(t, k, ctx, "sample-y", "0.90")
	seedFitness(t, k, ctx, "sample-z", "0.60")

	leaderboard := k.GetBountyLeaderboard(ctx, bountyID)
	require.Len(t, leaderboard, 3)

	// Ranked: y (0.9), z (0.6), x (0.3).
	require.Equal(t, "sample-y", leaderboard[0].SampleID)
	require.Equal(t, uint64(1), leaderboard[0].Rank)
	require.Equal(t, "sample-z", leaderboard[1].SampleID)
	require.Equal(t, uint64(2), leaderboard[1].Rank)
	require.Equal(t, "sample-x", leaderboard[2].SampleID)
	require.Equal(t, uint64(3), leaderboard[2].Rank)
}

// ─── Test: Bounty Board Stats ───────────────────────────────────────────────

func TestGetBountyBoardStats(t *testing.T) {
	k, ctx, _ := setupBountyTest(t)

	// Create bounties in different states.
	_ = k.SetCompetitiveBounty(ctx, &keeper.CompetitiveBounty{BountyID: "b1", Status: keeper.BountyStatusOpen, TotalPool: "1000"})
	_ = k.SetCompetitiveBounty(ctx, &keeper.CompetitiveBounty{BountyID: "b2", Status: keeper.BountyStatusOpen, TotalPool: "1000"})
	_ = k.SetCompetitiveBounty(ctx, &keeper.CompetitiveBounty{BountyID: "b3", Status: keeper.BountyStatusCompeting, TotalPool: "1000"})
	_ = k.SetCompetitiveBounty(ctx, &keeper.CompetitiveBounty{BountyID: "b4", Status: keeper.BountyStatusResolved, TotalPool: "1000"})
	_ = k.SetCompetitiveBounty(ctx, &keeper.CompetitiveBounty{BountyID: "b5", Status: keeper.BountyStatusExpired, TotalPool: "1000"})

	open, competing, judging, resolved, expired := k.GetBountyBoardStats(ctx)
	require.Equal(t, uint64(2), open)
	require.Equal(t, uint64(1), competing)
	require.Equal(t, uint64(0), judging)
	require.Equal(t, uint64(1), resolved)
	require.Equal(t, uint64(1), expired)
}

// ─── Test: Open Bounties Query ──────────────────────────────────────────────

func TestGetOpenBounties(t *testing.T) {
	k, ctx, _ := setupBountyTest(t)

	_ = k.SetCompetitiveBounty(ctx, &keeper.CompetitiveBounty{BountyID: "b1", Status: keeper.BountyStatusOpen, TotalPool: "1000"})
	_ = k.SetCompetitiveBounty(ctx, &keeper.CompetitiveBounty{BountyID: "b2", Status: keeper.BountyStatusCompeting, TotalPool: "2000"})
	_ = k.SetCompetitiveBounty(ctx, &keeper.CompetitiveBounty{BountyID: "b3", Status: keeper.BountyStatusResolved, TotalPool: "3000"})

	open := k.GetOpenBounties(ctx)
	require.Len(t, open, 2)
}

// ─── Test: Expire Bounties (BeginBlocker) ───────────────────────────────────

func TestExpireBounties_CompetingToJudging(t *testing.T) {
	k, ctx, _ := setupBountyTest(t)

	// Bounty with passed submission deadline.
	_ = k.SetCompetitiveBounty(ctx, &keeper.CompetitiveBounty{
		BountyID:           "b-expire",
		Status:             keeper.BountyStatusCompeting,
		SubmissionDeadline: 50, // block 50, current is 100
		TotalPool:          "5000000",
	})

	resolved, expired := k.ExpireBounties(ctx)
	require.Equal(t, uint64(0), resolved)
	require.Equal(t, uint64(0), expired)

	// Should have transitioned to JUDGING.
	comp, found := k.GetCompetitiveBounty(ctx, "b-expire")
	require.True(t, found)
	require.Equal(t, keeper.BountyStatusJudging, comp.Status)
	require.True(t, comp.JudgingDeadline > 0)
}

func TestExpireBounties_JudgingAutoResolves(t *testing.T) {
	k, ctx, _ := setupBountyTest(t)
	bountyID := getBountyID(t, k, ctx)

	// Submit something so resolution has work to do.
	_, err := k.SubmitToBounty(ctx, bountyID, "sample-auto", testAddr)
	require.NoError(t, err)

	seedFitness(t, k, ctx, "sample-auto", "0.65")

	// Set bounty to judging with passed deadline.
	comp, _ := k.GetCompetitiveBounty(ctx, bountyID)
	comp.Status = keeper.BountyStatusJudging
	comp.JudgingDeadline = 50 // past
	_ = k.SetCompetitiveBounty(ctx, comp)

	resolved, _ := k.ExpireBounties(ctx)
	require.Equal(t, uint64(1), resolved)

	// Should be resolved now.
	comp, _ = k.GetCompetitiveBounty(ctx, bountyID)
	require.Equal(t, keeper.BountyStatusResolved, comp.Status)
}

func TestExpireBounties_OpenExpired(t *testing.T) {
	k, ctx, _ := setupBountyTest(t)

	// Create a base bounty that's already expired.
	_ = k.SetDataBounty(ctx, &types.DataBounty{
		Id: "b-old", Domain: "science", RewardAmount: "1000000",
		ExpiresAtBlock: 50, // past
	})
	_ = k.SetCompetitiveBounty(ctx, &keeper.CompetitiveBounty{
		BountyID:  "b-old",
		Status:    keeper.BountyStatusOpen,
		TotalPool: "1000000",
	})

	_, expired := k.ExpireBounties(ctx)
	require.Equal(t, uint64(1), expired)

	comp, _ := k.GetCompetitiveBounty(ctx, "b-old")
	require.Equal(t, keeper.BountyStatusExpired, comp.Status)
}

// ─── Test: Bounty Board Params ──────────────────────────────────────────────

func TestBountyBoardParams_DefaultsAndPersist(t *testing.T) {
	k, ctx, _ := setupBountyTest(t)

	// Default params.
	params := k.GetBountyBoardParams(ctx)
	require.Equal(t, uint64(50_000), params.DefaultSubmissionWindow)
	require.Equal(t, []uint64{5000, 3000, 2000}, params.DefaultRewardTiers)
	require.Equal(t, uint64(1), params.DefaultMinSubmissions)

	// Custom params.
	custom := keeper.BountyBoardParams{
		DefaultSubmissionWindow: 100_000,
		DefaultRewardTiers:      []uint64{6000, 4000},
		DefaultMinSubmissions:   3,
		JudgingBuffer:           20_000,
		MinBountyAmount:         "5000000",
	}
	require.NoError(t, k.SetBountyBoardParams(ctx, custom))

	loaded := k.GetBountyBoardParams(ctx)
	require.Equal(t, uint64(100_000), loaded.DefaultSubmissionWindow)
	require.Equal(t, []uint64{6000, 4000}, loaded.DefaultRewardTiers)
	require.Equal(t, uint64(3), loaded.DefaultMinSubmissions)
}

// ─── Test: Resolve — Two Submissions, Three Tiers ───────────────────────────

func TestResolveBounty_FewerThanTiers(t *testing.T) {
	k, ctx, _ := setupBountyTest(t)
	bountyID := getBountyID(t, k, ctx)

	addr2 := "zrn1qqqsyqcyq5rqwzqfpg9scrgwpugpzysnv2f5q0"

	_, err := k.SubmitToBounty(ctx, bountyID, "sample-001", testAddr)
	require.NoError(t, err)
	_, err = k.SubmitToBounty(ctx, bountyID, "sample-002", addr2)
	require.NoError(t, err)

	seedFitness(t, k, ctx, "sample-001", "0.80")
	seedFitness(t, k, ctx, "sample-002", "0.60")

	resolution, err := k.ResolveBounty(ctx, bountyID)
	require.NoError(t, err)

	// Only 2 winners even though 3 tiers exist.
	require.Len(t, resolution.Winners, 2)
	require.Equal(t, "5000000", resolution.Winners[0].Reward) // 50%
	require.Equal(t, "3000000", resolution.Winners[1].Reward) // 30%
	require.Equal(t, "2000000", resolution.Remainder)          // 20% undistributed
}

// ─── Test: Resolve — Zero Fitness Still Ranks ───────────────────────────────

func TestResolveBounty_ZeroFitnessStillRanks(t *testing.T) {
	k, ctx, _ := setupBountyTest(t)
	bountyID := getBountyID(t, k, ctx)

	addr2 := "zrn1qqqsyqcyq5rqwzqfpg9scrgwpugpzysnv2f5q0"

	// Submit at different blocks so tie-breaking is deterministic.
	_, err := k.SubmitToBounty(ctx, bountyID, "sample-a", testAddr)
	require.NoError(t, err)

	ctx2 := sdk.UnwrapSDKContext(ctx).WithBlockHeight(200)
	_, err = k.SubmitToBounty(ctx2, bountyID, "sample-b", addr2)
	require.NoError(t, err)

	// No fitness records — both score zero. Earlier submission (block 100) wins.
	resolution, err := k.ResolveBounty(ctx, bountyID)
	require.NoError(t, err)
	require.Len(t, resolution.Winners, 2)
	require.Equal(t, "sample-a", resolution.Winners[0].SampleID) // submitted first at block 100
	require.Equal(t, "sample-b", resolution.Winners[1].SampleID) // submitted later at block 200
}

// ─── Test: Full Lifecycle ───────────────────────────────────────────────────

func TestBountyFullLifecycle(t *testing.T) {
	k, ctx, bk := setupBountyTest(t)
	bountyID := getBountyID(t, k, ctx)

	// 1. Two additional funders stack the pool.
	addr2 := "zrn1qqqsyqcyq5rqwzqfpg9scrgwpugpzysnv2f5q0"
	addr3 := "zrn1qcxce9c4thzxnfmpr2dqnnlqea9ey35ydj769h"

	err := k.AddToBountyPool(ctx, bountyID, addr2, "5000000")
	require.NoError(t, err)

	comp, _ := k.GetCompetitiveBounty(ctx, bountyID)
	require.Equal(t, "15000000", comp.TotalPool)

	// 2. Three agents compete.
	_, err = k.SubmitToBounty(ctx, bountyID, "s-alice", testAddr)
	require.NoError(t, err)
	_, err = k.SubmitToBounty(ctx, bountyID, "s-bob", addr2)
	require.NoError(t, err)
	_, err = k.SubmitToBounty(ctx, bountyID, "s-carol", addr3)
	require.NoError(t, err)

	comp, _ = k.GetCompetitiveBounty(ctx, bountyID)
	require.Equal(t, keeper.BountyStatusCompeting, comp.Status)
	require.Equal(t, uint64(3), comp.SubmissionCount)

	// 3. Fitness evolves.
	seedFitness(t, k, ctx, "s-alice", "0.55")
	seedFitness(t, k, ctx, "s-bob", "0.92")
	seedFitness(t, k, ctx, "s-carol", "0.70")

	// 4. Check live leaderboard.
	leaderboard := k.GetBountyLeaderboard(ctx, bountyID)
	require.Equal(t, "s-bob", leaderboard[0].SampleID)
	require.Equal(t, "s-carol", leaderboard[1].SampleID)
	require.Equal(t, "s-alice", leaderboard[2].SampleID)

	// 5. Resolve.
	resolution, err := k.ResolveBounty(ctx, bountyID)
	require.NoError(t, err)
	require.Len(t, resolution.Winners, 3)

	// 15M pool: 50% = 7.5M, 30% = 4.5M, 20% = 3M.
	require.Equal(t, "s-bob", resolution.Winners[0].SampleID)
	require.Equal(t, "7500000", resolution.Winners[0].Reward)
	require.Equal(t, "s-carol", resolution.Winners[1].SampleID)
	require.Equal(t, "4500000", resolution.Winners[1].Reward)
	require.Equal(t, "s-alice", resolution.Winners[2].SampleID)
	require.Equal(t, "3000000", resolution.Winners[2].Reward)
	require.Equal(t, "0", resolution.Remainder)

	// 6. Verify final state.
	comp, _ = k.GetCompetitiveBounty(ctx, bountyID)
	require.Equal(t, keeper.BountyStatusResolved, comp.Status)
	require.Len(t, comp.WinnerIDs, 3)

	baseBounty, _ := k.GetDataBounty(ctx, bountyID)
	require.True(t, baseBounty.Claimed)

	// Bank should have received transfer calls for rewards.
	require.True(t, len(bk.moduleToAccountCalls) >= 3)
}
