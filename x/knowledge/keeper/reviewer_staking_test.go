package keeper_test

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/keeper"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// Valid bech32 reviewer addresses for staking tests. The verifier1/2/3
// constants from keeper_test.go contain 'i' which is not in the bech32
// alphabet, so AccAddressFromBech32 fails on them. These are needed for
// any test that involves actual bank transfers.
const (
	rv1 = "zrn1pg9q5zs2pg9q5zs2pg9q5zs2pg9q5zs2rr2y5e"
	rv2 = "zrn1zs2pg9q5zs2pg9q5zs2pg9q5zs2pg9q5790d9j"
	rv3 = "zrn1rc0pu8s7rc0pu8s7rc0pu8s7rc0pu8s7peac8q"
	rv4 = "zrn19q5zs2pg9q5zs2pg9q5zs2pg9q5zs2pg7qclfu"
	rv5 = "zrn1xgeryv3jxgeryv3jxgeryv3jxgeryv3jjjuk5a"
)

// ─── Helpers ────────────────────────────────────────────────────────────────

// setupReviewerStakingRound creates a submission with stake and a round with
// reviewer staking params using valid bech32 addresses as verifiers.
func setupReviewerStakingRound(t *testing.T, stake string) (keeper.Keeper, sdk.Context, *mockBankKeeper, string) {
	t.Helper()
	k, ctx, bk := setupKeeperWithBank(t)
	setupDefaultDomains(t, k, ctx)

	rp := types.DefaultReviewerStakingParams()
	require.NoError(t, k.SetReviewerStakingParams(ctx, rp))

	sub := &types.Submission{
		Id:          "s1",
		Domain:      "technology",
		Submitter:   testAddr,
		Content:     "reviewer staking test content",
		Stake:       stake,
		Status:      types.SubmissionStatus_SUBMISSION_STATUS_PENDING,
		Consent:     &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
		ContentHash: "abc123deadbeef",
	}
	require.NoError(t, k.SetSubmission(ctx, sub))

	verifiers := []string{rv1, rv2, rv3}
	roundID, err := k.InitiateQualityRound(ctx, "s1", "", verifiers)
	require.NoError(t, err)
	return k, sdk.UnwrapSDKContext(ctx), bk, roundID
}

// runFullRoundWithStaking commits and reveals for a round using rv1/rv2/rv3.
func runFullRoundWithStaking(t *testing.T, k keeper.Keeper, ctx sdk.Context, roundID string, votes []*types.QualityVote) {
	t.Helper()
	salts := [][]byte{[]byte("s1"), []byte("s2"), []byte("s3")}
	verifiers := []string{rv1, rv2, rv3}

	for i, v := range verifiers {
		hash := types.ComputeQualityCommitHash(roundID, votes[i], salts[i])
		require.NoError(t, k.SubmitCommitment(ctx, &types.MsgSubmitCommitment{
			Verifier: v, RoundId: roundID, CommitHash: hash,
		}))
	}

	round, _ := k.GetQualityRound(ctx, roundID)
	round.Phase = types.VerificationPhase_VERIFICATION_PHASE_REVEAL
	require.NoError(t, k.SetQualityRound(ctx, round))

	for i, v := range verifiers {
		require.NoError(t, k.SubmitReveal(ctx, &types.MsgSubmitReveal{
			Verifier: v, RoundId: roundID, Scores: votes[i], Salt: salts[i],
		}))
	}
}

func findTransfer(calls []bankTransfer, fn func(bankTransfer) bool) *bankTransfer {
	for i := range calls {
		if fn(calls[i]) {
			return &calls[i]
		}
	}
	return nil
}

func totalPaid(calls []bankTransfer) sdkmath.Int {
	total := sdkmath.ZeroInt()
	for _, c := range calls {
		for _, coin := range c.amount {
			if coin.Denom == "uzrn" {
				total = total.Add(coin.Amount)
			}
		}
	}
	return total
}

// ─── Table-Driven Tests ─────────────────────────────────────────────────────

func TestReviewerStaking_AcceptUnanimous(t *testing.T) {
	k, ctx, bk, roundID := setupReviewerStakingRound(t, "1000000")

	votes := []*types.QualityVote{
		{OverallQuality: 800000, ConsentValid: true},
		{OverallQuality: 850000, ConsentValid: true},
		{OverallQuality: 900000, ConsentValid: true},
	}
	runFullRoundWithStaking(t, k, ctx, roundID, votes)

	// Clear escrow calls to only track distribution.
	bk.moduleToAccountCalls = nil
	require.NoError(t, k.AggregateQualityRound(ctx, roundID))

	// ShowUpPool = 100,000. MinorityPot = 0. AcceptReward = min(300,000, 0) = 0.
	// Submitter = 1,000,000 - 100,000 + 0 = 900,000.
	submitterPay := findTransfer(bk.moduleToAccountCalls, func(bt bankTransfer) bool {
		return bt.to == testAddr
	})
	require.NotNil(t, submitterPay, "submitter should be paid")
	require.Equal(t, sdk.NewInt64Coin("uzrn", 900_000), submitterPay.amount[0])

	// Each reviewer = 300,000 + 33,333 + 0 = 333,333.
	for _, addr := range []string{rv1, rv2, rv3} {
		pay := findTransfer(bk.moduleToAccountCalls, func(bt bankTransfer) bool {
			return bt.to == addr
		})
		require.NotNil(t, pay, "reviewer %s should be paid", addr)
		require.Equal(t, sdk.NewInt64Coin("uzrn", 333_333), pay.amount[0])
	}

	sub, found := k.GetSubmission(ctx, "s1")
	require.True(t, found)
	require.Equal(t, types.SubmissionStatus_SUBMISSION_STATUS_ACCEPTED, sub.Status)

	// Verify staking event.
	events := ctx.EventManager().Events()
	stakingEvent := false
	for _, e := range events {
		if e.Type == "reviewer_staking" {
			for _, attr := range e.Attributes {
				if attr.Key == "outcome" && attr.Value == "accept" {
					stakingEvent = true
				}
			}
		}
	}
	require.True(t, stakingEvent)
}

func TestReviewerStaking_AcceptWithMinority(t *testing.T) {
	k, ctx, bk, roundID := setupReviewerStakingRound(t, "1000000")

	votes := []*types.QualityVote{
		{OverallQuality: 800000, ConsentValid: true}, // accept
		{OverallQuality: 700000, ConsentValid: true}, // accept
		{OverallQuality: 200000, ConsentValid: true}, // reject (< BronzeThreshold)
	}
	runFullRoundWithStaking(t, k, ctx, roundID, votes)

	bk.moduleToAccountCalls = nil
	require.NoError(t, k.AggregateQualityRound(ctx, roundID))

	// MinorityPot = 300,000 (rv3). AcceptReward = min(300,000, 300,000) = 300,000.
	// Submitter = 1,000,000 - 100,000 + 300,000 = 1,200,000.
	submitterPay := findTransfer(bk.moduleToAccountCalls, func(bt bankTransfer) bool {
		return bt.to == testAddr
	})
	require.NotNil(t, submitterPay)
	require.Equal(t, sdk.NewInt64Coin("uzrn", 1_200_000), submitterPay.amount[0])

	// Each majority = 300,000 + 50,000 + 0 = 350,000.
	for _, addr := range []string{rv1, rv2} {
		pay := findTransfer(bk.moduleToAccountCalls, func(bt bankTransfer) bool {
			return bt.to == addr
		})
		require.NotNil(t, pay)
		require.Equal(t, sdk.NewInt64Coin("uzrn", 350_000), pay.amount[0])
	}

	// Minority (rv3): no payout.
	v3Pay := findTransfer(bk.moduleToAccountCalls, func(bt bankTransfer) bool {
		return bt.to == rv3
	})
	require.Nil(t, v3Pay, "minority reviewer should not be paid")
}

func TestReviewerStaking_RejectUnanimous(t *testing.T) {
	k, ctx, bk, roundID := setupReviewerStakingRound(t, "1000000")

	votes := []*types.QualityVote{
		{OverallQuality: 100000, ConsentValid: true},
		{OverallQuality: 200000, ConsentValid: true},
		{OverallQuality: 300000, ConsentValid: true},
	}
	runFullRoundWithStaking(t, k, ctx, roundID, votes)

	bk.moduleToAccountCalls = nil
	require.NoError(t, k.AggregateQualityRound(ctx, roundID))

	// ShowUp=100,000. ChallengeBonus=500,000. MinorityPot=0.
	// Each majority = 300,000 + (100,000+500,000+0)/3 = 300,000 + 200,000 = 500,000.
	for _, addr := range []string{rv1, rv2, rv3} {
		pay := findTransfer(bk.moduleToAccountCalls, func(bt bankTransfer) bool {
			return bt.to == addr
		})
		require.NotNil(t, pay, "reviewer %s should be paid", addr)
		require.Equal(t, sdk.NewInt64Coin("uzrn", 500_000), pay.amount[0])
	}

	// Submitter: no payout.
	submitterPay := findTransfer(bk.moduleToAccountCalls, func(bt bankTransfer) bool {
		return bt.to == testAddr
	})
	require.Nil(t, submitterPay, "submitter loses everything on reject")

	sub, found := k.GetSubmission(ctx, "s1")
	require.True(t, found)
	require.Equal(t, types.SubmissionStatus_SUBMISSION_STATUS_REJECTED, sub.Status)
}

func TestReviewerStaking_RejectWithMinority(t *testing.T) {
	k, ctx, bk, roundID := setupReviewerStakingRound(t, "1000000")

	votes := []*types.QualityVote{
		{OverallQuality: 100000, ConsentValid: true}, // reject
		{OverallQuality: 200000, ConsentValid: true}, // reject
		{OverallQuality: 800000, ConsentValid: true}, // accept (minority)
	}
	runFullRoundWithStaking(t, k, ctx, roundID, votes)

	bk.moduleToAccountCalls = nil
	require.NoError(t, k.AggregateQualityRound(ctx, roundID))

	// MinorityPot = 300,000 (rv3).
	// Each majority = 300,000 + (100,000 + 500,000 + 300,000)/2 = 300,000 + 450,000 = 750,000.
	for _, addr := range []string{rv1, rv2} {
		pay := findTransfer(bk.moduleToAccountCalls, func(bt bankTransfer) bool {
			return bt.to == addr
		})
		require.NotNil(t, pay)
		require.Equal(t, sdk.NewInt64Coin("uzrn", 750_000), pay.amount[0])
	}

	// rv3 (minority): no payout.
	v3Pay := findTransfer(bk.moduleToAccountCalls, func(bt bankTransfer) bool {
		return bt.to == rv3
	})
	require.Nil(t, v3Pay, "minority reviewer should not be paid on reject")
}

func TestReviewerStaking_DeepContested(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)
	setupDefaultDomains(t, k, ctx)
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	rp := types.DefaultReviewerStakingParams()
	require.NoError(t, k.SetReviewerStakingParams(sdkCtx, rp))

	sub := &types.Submission{
		Id: "s1", Domain: "technology", Submitter: testAddr,
		Content: "contested content", Stake: "1000000",
		Status:      types.SubmissionStatus_SUBMISSION_STATUS_PENDING,
		Consent:     &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
		ContentHash: "contested_hash_123",
	}
	require.NoError(t, k.SetSubmission(sdkCtx, sub))

	// 5 verifiers: 3 accept, 2 reject → 3/5 = 60% < 66.7% → deep contested.
	verifiers := []string{rv1, rv2, rv3, rv4, rv5}
	roundID, err := k.InitiateQualityRound(sdkCtx, "s1", "", verifiers)
	require.NoError(t, err)

	votes := []*types.QualityVote{
		{OverallQuality: 800000, ConsentValid: true},
		{OverallQuality: 700000, ConsentValid: true},
		{OverallQuality: 500000, ConsentValid: true},
		{OverallQuality: 200000, ConsentValid: true},
		{OverallQuality: 100000, ConsentValid: true},
	}
	salts := [][]byte{[]byte("s1"), []byte("s2"), []byte("s3"), []byte("s4"), []byte("s5")}

	for i, v := range verifiers {
		hash := types.ComputeQualityCommitHash(roundID, votes[i], salts[i])
		require.NoError(t, k.SubmitCommitment(sdkCtx, &types.MsgSubmitCommitment{
			Verifier: v, RoundId: roundID, CommitHash: hash,
		}))
	}

	round, _ := k.GetQualityRound(sdkCtx, roundID)
	round.Phase = types.VerificationPhase_VERIFICATION_PHASE_REVEAL
	require.NoError(t, k.SetQualityRound(sdkCtx, round))

	for i, v := range verifiers {
		require.NoError(t, k.SubmitReveal(sdkCtx, &types.MsgSubmitReveal{
			Verifier: v, RoundId: roundID, Scores: votes[i], Salt: salts[i],
		}))
	}

	bk.moduleToAccountCalls = nil
	require.NoError(t, k.AggregateQualityRound(sdkCtx, roundID))

	// ALL stakes returned.
	submitterPay := findTransfer(bk.moduleToAccountCalls, func(bt bankTransfer) bool {
		return bt.to == testAddr
	})
	require.NotNil(t, submitterPay, "submitter gets stake back on deep contested")
	require.Equal(t, sdk.NewInt64Coin("uzrn", 1_000_000), submitterPay.amount[0])

	for _, v := range verifiers {
		pay := findTransfer(bk.moduleToAccountCalls, func(bt bankTransfer) bool {
			return bt.to == v
		})
		require.NotNil(t, pay, "reviewer %s should get stake back", v)
		require.Equal(t, sdk.NewInt64Coin("uzrn", 300_000), pay.amount[0])
	}

	count := k.GetContestedDeepCount(sdkCtx, "contested_hash_123")
	require.Equal(t, uint64(1), count)

	events := sdkCtx.EventManager().Events()
	deepEvent := false
	for _, e := range events {
		if e.Type == "deep_contested" {
			deepEvent = true
		}
	}
	require.True(t, deepEvent)
}

func TestReviewerStaking_PermanentRejectAfterMaxStrikes(t *testing.T) {
	k, ctx := setupKeeper(t)

	require.NoError(t, k.SetContestedDeepCount(ctx, "repeat_hash", 2))

	rp := types.DefaultReviewerStakingParams()

	newCount, err := k.IncrementContestedDeepCount(ctx, "repeat_hash")
	require.NoError(t, err)
	require.Equal(t, uint64(3), newCount)
	require.True(t, newCount >= rp.MaxContestedDeepCount, "should trigger permanent reject")
}

func TestReviewerStaking_CommitEscrowsStake(t *testing.T) {
	k, ctx, bk, roundID := setupReviewerStakingRound(t, "1000000")

	vote := &types.QualityVote{OverallQuality: 800000, ConsentValid: true}
	hash := types.ComputeQualityCommitHash(roundID, vote, []byte("s1"))

	require.NoError(t, k.SubmitCommitment(ctx, &types.MsgSubmitCommitment{
		Verifier: rv1, RoundId: roundID, CommitHash: hash,
	}))

	require.Len(t, bk.accountToModuleCalls, 1)
	require.Equal(t, rv1, bk.accountToModuleCalls[0].from)
	require.Equal(t, "knowledge", bk.accountToModuleCalls[0].to)
	require.Equal(t, sdk.NewInt64Coin("uzrn", 300_000), bk.accountToModuleCalls[0].amount[0])

	stakeStr, found := k.GetReviewerStake(ctx, roundID, rv1)
	require.True(t, found)
	require.Equal(t, "300000", stakeStr)
}

func TestReviewerStaking_CommitFailsInsufficientFunds(t *testing.T) {
	k, ctx, bk, roundID := setupReviewerStakingRound(t, "1000000")

	bk.failNextSend = true
	vote := &types.QualityVote{OverallQuality: 800000}
	hash := types.ComputeQualityCommitHash(roundID, vote, []byte("s1"))

	err := k.SubmitCommitment(ctx, &types.MsgSubmitCommitment{
		Verifier: rv1, RoundId: roundID, CommitHash: hash,
	})
	require.ErrorIs(t, err, types.ErrReviewerStakeInsufficient)
}

func TestReviewerStaking_NoStakeSubmission(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)
	setupDefaultDomains(t, k, ctx)
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	require.NoError(t, k.SetReviewerStakingParams(sdkCtx, types.DefaultReviewerStakingParams()))

	sub := &types.Submission{
		Id: "s1", Domain: "technology", Submitter: testAddr,
		Content: "no stake", Stake: "",
		Status:  types.SubmissionStatus_SUBMISSION_STATUS_PENDING,
		Consent: &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
	}
	require.NoError(t, k.SetSubmission(sdkCtx, sub))

	roundID, err := k.InitiateQualityRound(sdkCtx, "s1", "", []string{rv1, rv2, rv3})
	require.NoError(t, err)

	vote := &types.QualityVote{OverallQuality: 800000, ConsentValid: true}
	hash := types.ComputeQualityCommitHash(roundID, vote, []byte("s1"))

	require.NoError(t, k.SubmitCommitment(sdkCtx, &types.MsgSubmitCommitment{
		Verifier: rv1, RoundId: roundID, CommitHash: hash,
	}))
	require.Empty(t, bk.accountToModuleCalls, "no escrow when submission has no stake")
}

func TestReviewerStaking_RoundingDust(t *testing.T) {
	k, ctx, bk, roundID := setupReviewerStakingRound(t, "1000000")

	votes := []*types.QualityVote{
		{OverallQuality: 800000, ConsentValid: true},
		{OverallQuality: 850000, ConsentValid: true},
		{OverallQuality: 900000, ConsentValid: true},
	}
	runFullRoundWithStaking(t, k, ctx, roundID, votes)

	bk.moduleToAccountCalls = nil
	require.NoError(t, k.AggregateQualityRound(ctx, roundID))

	// Submitter: 900,000. Each reviewer: 333,333. Total = 900,000 + 999,999 = 1,899,999.
	total := totalPaid(bk.moduleToAccountCalls)
	require.Equal(t, sdkmath.NewInt(1_899_999), total)
}

func TestReviewerStaking_ParamsStoredAndRetrieved(t *testing.T) {
	k, ctx := setupKeeper(t)

	rp := k.GetReviewerStakingParams(ctx)
	require.Equal(t, uint64(3000), rp.ReviewerStakeRatioBps)
	require.Equal(t, uint64(1000), rp.ShowUpRewardRatioBps)
	require.Equal(t, uint64(3000), rp.AcceptRewardRatioBps)
	require.Equal(t, uint64(5000), rp.RejectBonusRatioBps)
	require.Equal(t, uint64(3), rp.MaxContestedDeepCount)

	custom := types.ReviewerStakingParams{
		ReviewerStakeRatioBps: 5000,
		ShowUpRewardRatioBps:  500,
		AcceptRewardRatioBps:  2000,
		RejectBonusRatioBps:   4000,
		MaxContestedDeepCount: 5,
	}
	require.NoError(t, k.SetReviewerStakingParams(ctx, custom))
	require.Equal(t, custom, k.GetReviewerStakingParams(ctx))
}

func TestReviewerStakingParams_Validate(t *testing.T) {
	tests := []struct {
		name    string
		params  types.ReviewerStakingParams
		wantErr bool
	}{
		{"valid defaults", types.DefaultReviewerStakingParams(), false},
		{"zero stake ratio", types.ReviewerStakingParams{
			ReviewerStakeRatioBps: 0, ShowUpRewardRatioBps: 1000,
			AcceptRewardRatioBps: 3000, RejectBonusRatioBps: 5000, MaxContestedDeepCount: 3,
		}, true},
		{"stake ratio too high", types.ReviewerStakingParams{
			ReviewerStakeRatioBps: 10001, ShowUpRewardRatioBps: 1000,
			AcceptRewardRatioBps: 3000, RejectBonusRatioBps: 5000, MaxContestedDeepCount: 3,
		}, true},
		{"show_up + reject > 100%", types.ReviewerStakingParams{
			ReviewerStakeRatioBps: 3000, ShowUpRewardRatioBps: 6000,
			AcceptRewardRatioBps: 3000, RejectBonusRatioBps: 5000, MaxContestedDeepCount: 3,
		}, true},
		{"max contested = 0", types.ReviewerStakingParams{
			ReviewerStakeRatioBps: 3000, ShowUpRewardRatioBps: 1000,
			AcceptRewardRatioBps: 3000, RejectBonusRatioBps: 5000, MaxContestedDeepCount: 0,
		}, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.params.Validate()
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestReviewerStaking_ContestedDeepCountCRUD(t *testing.T) {
	k, ctx := setupKeeper(t)

	require.Equal(t, uint64(0), k.GetContestedDeepCount(ctx, "hash1"))

	count, err := k.IncrementContestedDeepCount(ctx, "hash1")
	require.NoError(t, err)
	require.Equal(t, uint64(1), count)

	count, err = k.IncrementContestedDeepCount(ctx, "hash1")
	require.NoError(t, err)
	require.Equal(t, uint64(2), count)

	require.NoError(t, k.SetContestedDeepCount(ctx, "hash1", 10))
	require.Equal(t, uint64(10), k.GetContestedDeepCount(ctx, "hash1"))

	require.Equal(t, uint64(0), k.GetContestedDeepCount(ctx, "hash2"))
}

func TestReviewerStaking_GetAllStakes(t *testing.T) {
	k, ctx := setupKeeper(t)

	require.NoError(t, k.SetReviewerStake(ctx, "round1", rv1, "300000"))
	require.NoError(t, k.SetReviewerStake(ctx, "round1", rv2, "300000"))
	require.NoError(t, k.SetReviewerStake(ctx, "round1", rv3, "300000"))

	stakes := k.GetAllReviewerStakes(ctx, "round1")
	require.Len(t, stakes, 3)
	require.Equal(t, "300000", stakes[rv1])
	require.Equal(t, "300000", stakes[rv2])
	require.Equal(t, "300000", stakes[rv3])

	require.Empty(t, k.GetAllReviewerStakes(ctx, "round2"))
}
