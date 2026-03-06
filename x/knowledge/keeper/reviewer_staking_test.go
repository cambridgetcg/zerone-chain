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

// ─── Table-Driven Outcome Tests ─────────────────────────────────────────────

func TestReviewerStaking_Outcomes(t *testing.T) {
	tests := []struct {
		name string
		// votes for rv1, rv2, rv3 respectively
		votes []*types.QualityVote
		// expected payouts (-1 = no payout)
		wantSubmitter int64
		wantRV1       int64
		wantRV2       int64
		wantRV3       int64
		wantOutcome   string
	}{
		{
			name: "accept_unanimous_no_showup",
			votes: []*types.QualityVote{
				{OverallQuality: 800000, ConsentValid: true},
				{OverallQuality: 850000, ConsentValid: true},
				{OverallQuality: 900000, ConsentValid: true},
			},
			// minorityPot=0, showUp=0, acceptReward=0
			// submitter=1M, each reviewer=300K (own stake only)
			wantSubmitter: 1_000_000,
			wantRV1:       300_000,
			wantRV2:       300_000,
			wantRV3:       300_000,
			wantOutcome:   "accept",
		},
		{
			name: "accept_with_minority_showup_from_minority_pot",
			votes: []*types.QualityVote{
				{OverallQuality: 800000, ConsentValid: true}, // accept
				{OverallQuality: 700000, ConsentValid: true}, // accept
				{OverallQuality: 200000, ConsentValid: true}, // reject (minority)
			},
			// minorityPot=300K, showUp=30K, afterShowUp=270K
			// acceptReward=min(300K,270K)=270K, remaining=0
			// submitter=1M+270K=1,270K
			// each majority=300K+(30K+0)/2=315K
			// rv3 loses stake (no payout)
			wantSubmitter: 1_270_000,
			wantRV1:       315_000,
			wantRV2:       315_000,
			wantRV3:       -1,
			wantOutcome:   "accept",
		},
		{
			name: "reject_unanimous_no_showup",
			votes: []*types.QualityVote{
				{OverallQuality: 100000, ConsentValid: true},
				{OverallQuality: 200000, ConsentValid: true},
				{OverallQuality: 300000, ConsentValid: true},
			},
			// minorityPot=0, challengeBonus=500K
			// rewardPool=500K, perMaj=166,666
			// each reviewer=300K+166,666=466,666
			// submitter loses everything
			wantSubmitter: -1,
			wantRV1:       466_666,
			wantRV2:       466_666,
			wantRV3:       466_666,
			wantOutcome:   "reject",
		},
		{
			name: "reject_with_minority",
			votes: []*types.QualityVote{
				{OverallQuality: 100000, ConsentValid: true}, // reject
				{OverallQuality: 200000, ConsentValid: true}, // reject
				{OverallQuality: 800000, ConsentValid: true}, // accept (minority)
			},
			// minorityPot=300K, challengeBonus=500K
			// rewardPool=800K, perMaj=400K
			// rv1,rv2=300K+400K=700K. rv3 loses stake.
			wantSubmitter: -1,
			wantRV1:       700_000,
			wantRV2:       700_000,
			wantRV3:       -1,
			wantOutcome:   "reject",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			k, ctx, bk, roundID := setupReviewerStakingRound(t, "1000000")
			runFullRoundWithStaking(t, k, ctx, roundID, tc.votes)

			bk.moduleToAccountCalls = nil
			require.NoError(t, k.AggregateQualityRound(ctx, roundID))

			// Check submitter payout.
			submitterPay := findTransfer(bk.moduleToAccountCalls, func(bt bankTransfer) bool {
				return bt.to == testAddr
			})
			if tc.wantSubmitter < 0 {
				require.Nil(t, submitterPay, "submitter should not be paid")
			} else {
				require.NotNil(t, submitterPay, "submitter should be paid")
				require.Equal(t, sdk.NewInt64Coin("uzrn", tc.wantSubmitter), submitterPay.amount[0])
			}

			// Check reviewer payouts.
			for _, check := range []struct {
				addr string
				want int64
			}{
				{rv1, tc.wantRV1},
				{rv2, tc.wantRV2},
				{rv3, tc.wantRV3},
			} {
				pay := findTransfer(bk.moduleToAccountCalls, func(bt bankTransfer) bool {
					return bt.to == check.addr
				})
				if check.want < 0 {
					require.Nil(t, pay, "reviewer %s should not be paid", check.addr)
				} else {
					require.NotNil(t, pay, "reviewer %s should be paid", check.addr)
					require.Equal(t, sdk.NewInt64Coin("uzrn", check.want), pay.amount[0],
						"reviewer %s payout mismatch", check.addr)
				}
			}

			// Check staking event outcome.
			events := ctx.EventManager().Events()
			found := false
			for _, e := range events {
				if e.Type == "reviewer_staking" {
					for _, attr := range e.Attributes {
						if attr.Key == "outcome" && attr.Value == tc.wantOutcome {
							found = true
						}
					}
				}
			}
			require.True(t, found, "expected staking event with outcome=%s", tc.wantOutcome)
		})
	}
}

// ─── Deep Contested & Strikes ───────────────────────────────────────────────

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

	count, permanent, err := k.RecordContestedStrike(ctx, "repeat_hash", rp)
	require.NoError(t, err)
	require.Equal(t, uint64(3), count)
	require.True(t, permanent, "should trigger permanent reject at 3 strikes")
}

func TestReviewerStaking_RecordContestedStrike_EmptyHash(t *testing.T) {
	k, ctx := setupKeeper(t)
	rp := types.DefaultReviewerStakingParams()

	count, permanent, err := k.RecordContestedStrike(ctx, "", rp)
	require.NoError(t, err)
	require.Equal(t, uint64(0), count)
	require.False(t, permanent)
}

// ─── Escrow Tests ───────────────────────────────────────────────────────────

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

// ─── Rounding Dust ──────────────────────────────────────────────────────────

func TestReviewerStaking_RoundingDust(t *testing.T) {
	k, ctx, bk, roundID := setupReviewerStakingRound(t, "1000000")

	// Reject unanimous: 500K challengeBonus / 3 = 166,666 per reviewer → dust.
	votes := []*types.QualityVote{
		{OverallQuality: 100000, ConsentValid: true},
		{OverallQuality: 200000, ConsentValid: true},
		{OverallQuality: 300000, ConsentValid: true},
	}
	runFullRoundWithStaking(t, k, ctx, roundID, votes)

	bk.moduleToAccountCalls = nil
	require.NoError(t, k.AggregateQualityRound(ctx, roundID))

	// 466,666 * 3 = 1,399,998. 2 uzrn dust stays in module.
	total := totalPaid(bk.moduleToAccountCalls)
	require.Equal(t, sdkmath.NewInt(1_399_998), total)
}

// ─── Params ─────────────────────────────────────────────────────────────────

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

// ─── State CRUD ─────────────────────────────────────────────────────────────

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

// ─── EscrowReviewerStake unit test ──────────────────────────────────────────

func TestEscrowReviewerStake_Direct(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	require.NoError(t, k.SetReviewerStakingParams(sdkCtx, types.DefaultReviewerStakingParams()))

	submitterStake := sdkmath.NewInt(1_000_000)
	require.NoError(t, k.EscrowReviewerStake(sdkCtx, "r1", rv1, submitterStake))

	require.Len(t, bk.accountToModuleCalls, 1)
	require.Equal(t, sdk.NewInt64Coin("uzrn", 300_000), bk.accountToModuleCalls[0].amount[0])

	stakeStr, found := k.GetReviewerStake(sdkCtx, "r1", rv1)
	require.True(t, found)
	require.Equal(t, "300000", stakeStr)
}

func TestEscrowReviewerStake_ZeroSubmitterStake(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	require.NoError(t, k.SetReviewerStakingParams(sdkCtx, types.DefaultReviewerStakingParams()))

	require.NoError(t, k.EscrowReviewerStake(sdkCtx, "r1", rv1, sdkmath.ZeroInt()))
	require.Empty(t, bk.accountToModuleCalls, "no escrow for zero stake")
}

// ─── Show-up reward source verification ─────────────────────────────────────

func TestReviewerStaking_ShowUpFromMinorityPotOnly(t *testing.T) {
	// Accept unanimous: no minority pot → submitter gets full stake,
	// reviewers get only their own stake back (no show-up deduction).
	k, ctx, bk, roundID := setupReviewerStakingRound(t, "1000000")

	votes := []*types.QualityVote{
		{OverallQuality: 800000, ConsentValid: true},
		{OverallQuality: 850000, ConsentValid: true},
		{OverallQuality: 900000, ConsentValid: true},
	}
	runFullRoundWithStaking(t, k, ctx, roundID, votes)

	bk.moduleToAccountCalls = nil
	require.NoError(t, k.AggregateQualityRound(ctx, roundID))

	// Submitter: gets full 1M back (no show-up deduction).
	submitterPay := findTransfer(bk.moduleToAccountCalls, func(bt bankTransfer) bool {
		return bt.to == testAddr
	})
	require.NotNil(t, submitterPay)
	require.Equal(t, sdk.NewInt64Coin("uzrn", 1_000_000), submitterPay.amount[0],
		"unanimous accept: submitter should get full stake back without show-up deduction")

	// Each reviewer: gets exactly their own stake back (300K).
	for _, addr := range []string{rv1, rv2, rv3} {
		pay := findTransfer(bk.moduleToAccountCalls, func(bt bankTransfer) bool {
			return bt.to == addr
		})
		require.NotNil(t, pay)
		require.Equal(t, sdk.NewInt64Coin("uzrn", 300_000), pay.amount[0],
			"unanimous accept: reviewer should get only own stake back")
	}
}

// ─── Submission status verification ─────────────────────────────────────────

func TestReviewerStaking_AcceptSetsSubmissionStatus(t *testing.T) {
	k, ctx, _, roundID := setupReviewerStakingRound(t, "1000000")

	votes := []*types.QualityVote{
		{OverallQuality: 800000, ConsentValid: true},
		{OverallQuality: 850000, ConsentValid: true},
		{OverallQuality: 900000, ConsentValid: true},
	}
	runFullRoundWithStaking(t, k, ctx, roundID, votes)
	require.NoError(t, k.AggregateQualityRound(ctx, roundID))

	sub, found := k.GetSubmission(ctx, "s1")
	require.True(t, found)
	require.Equal(t, types.SubmissionStatus_SUBMISSION_STATUS_ACCEPTED, sub.Status)
}

func TestReviewerStaking_RejectSetsSubmissionStatus(t *testing.T) {
	k, ctx, _, roundID := setupReviewerStakingRound(t, "1000000")

	votes := []*types.QualityVote{
		{OverallQuality: 100000, ConsentValid: true},
		{OverallQuality: 200000, ConsentValid: true},
		{OverallQuality: 300000, ConsentValid: true},
	}
	runFullRoundWithStaking(t, k, ctx, roundID, votes)
	require.NoError(t, k.AggregateQualityRound(ctx, roundID))

	sub, found := k.GetSubmission(ctx, "s1")
	require.True(t, found)
	require.Equal(t, types.SubmissionStatus_SUBMISSION_STATUS_REJECTED, sub.Status)
}
