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

// ─── richMockBankKeeper ────────────────────────────────────────────────────────
// Extends mockBankKeeper with configurable balances per address. Used by
// martyrance tests that need the submitter to have a positive uzrn balance.

type richMockBankKeeper struct {
	mockBankKeeper
	balances map[string]sdkmath.Int // addr string → amount
}

func newRichMockBankKeeper() *richMockBankKeeper {
	return &richMockBankKeeper{
		mockBankKeeper: mockBankKeeper{},
		balances:       make(map[string]sdkmath.Int),
	}
}

func (r *richMockBankKeeper) SetBalance(addr sdk.AccAddress, amount sdkmath.Int) {
	r.balances[addr.String()] = amount
}

func (r *richMockBankKeeper) GetBalance(_ context.Context, addr sdk.AccAddress, denom string) sdk.Coin {
	if amt, ok := r.balances[addr.String()]; ok && denom == "uzrn" {
		return sdk.NewCoin(denom, amt)
	}
	return sdk.NewInt64Coin(denom, 0)
}

// setupKeeperWithRichBank creates a keeper backed by a richMockBankKeeper.
func setupKeeperWithRichBank(t *testing.T) (keeper.Keeper, sdk.Context, *richMockBankKeeper) {
	t.Helper()
	ss := newMockStoreService()
	bk := newRichMockBankKeeper()
	k := keeper.NewKeeper(ss, nil, "authority", bk, nil)
	ctx := sdk.Context{}.
		WithBlockHeight(100).
		WithEventManager(sdk.NewEventManager()).
		WithMultiStore(&mockCacheMultiStore{})
	return k, ctx, bk
}

// ─── MartyranceParams Tests ───────────────────────────────────────────────────

func TestMartyranceParams_Defaults(t *testing.T) {
	p := types.DefaultMartyranceParams()
	require.Equal(t, uint64(9000), p.MinStakeRatioBps)
	require.Equal(t, uint32(3), p.VerifierMinTier)
	require.Equal(t, uint64(2), p.DeadlineMultiplier)
	require.Equal(t, uint64(5), p.ReputationMultiplier)
	require.Equal(t, uint64(10), p.MaxActiveMartyranceClaims)
	require.NoError(t, p.Validate())
}

func TestMartyranceParams_Validate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*types.MartyranceParams)
		wantErr string
	}{
		{
			name:    "zero_stake_ratio",
			mutate:  func(p *types.MartyranceParams) { p.MinStakeRatioBps = 0 },
			wantErr: "min_stake_ratio_bps",
		},
		{
			name:    "stake_ratio_exceeds_100pct",
			mutate:  func(p *types.MartyranceParams) { p.MinStakeRatioBps = 10_001 },
			wantErr: "min_stake_ratio_bps",
		},
		{
			name:    "stake_ratio_exactly_10000_ok",
			mutate:  func(p *types.MartyranceParams) { p.MinStakeRatioBps = 10_000 },
			wantErr: "",
		},
		{
			name:    "zero_verifier_tier",
			mutate:  func(p *types.MartyranceParams) { p.VerifierMinTier = 0 },
			wantErr: "verifier_min_tier",
		},
		{
			name:    "zero_deadline_multiplier",
			mutate:  func(p *types.MartyranceParams) { p.DeadlineMultiplier = 0 },
			wantErr: "deadline_multiplier",
		},
		{
			name:    "zero_reputation_multiplier",
			mutate:  func(p *types.MartyranceParams) { p.ReputationMultiplier = 0 },
			wantErr: "reputation_multiplier",
		},
		{
			name:    "zero_max_active_claims",
			mutate:  func(p *types.MartyranceParams) { p.MaxActiveMartyranceClaims = 0 },
			wantErr: "max_active_martyrance_claims",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := types.DefaultMartyranceParams()
			tc.mutate(&p)
			err := p.Validate()
			if tc.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.wantErr)
			}
		})
	}
}

func TestMartyranceParams_RoundTrip(t *testing.T) {
	k, ctx, _ := setupKeeperWithBank(t)
	p := types.DefaultMartyranceParams()
	p.ReputationMultiplier = 7
	require.NoError(t, k.SetMartyranceParams(ctx, p))

	got := k.GetMartyranceParams(ctx)
	require.Equal(t, p, got)
}

// ─── Martyrance Queue Storage Tests ──────────────────────────────────────────

func TestMartyranceQueue_EnqueueDequeue(t *testing.T) {
	k, ctx, _ := setupKeeperWithBank(t)

	require.Equal(t, uint64(0), k.GetMartyranceQueueSize(ctx))
	require.False(t, k.IsMartyranceClaim(ctx, "s1"))

	require.NoError(t, k.EnqueueMartyranceClaim(ctx, "s1"))
	require.Equal(t, uint64(1), k.GetMartyranceQueueSize(ctx))
	require.True(t, k.IsMartyranceClaim(ctx, "s1"))

	require.NoError(t, k.EnqueueMartyranceClaim(ctx, "s2"))
	require.Equal(t, uint64(2), k.GetMartyranceQueueSize(ctx))

	require.NoError(t, k.DequeueMartyranceClaim(ctx, "s1"))
	require.Equal(t, uint64(1), k.GetMartyranceQueueSize(ctx))
	require.False(t, k.IsMartyranceClaim(ctx, "s1"))
	require.True(t, k.IsMartyranceClaim(ctx, "s2"))
}

func TestMartyranceQueue_Iterate(t *testing.T) {
	k, ctx, _ := setupKeeperWithBank(t)
	ids := []string{"s1", "s2", "s3"}
	for _, id := range ids {
		require.NoError(t, k.EnqueueMartyranceClaim(ctx, id))
	}

	var collected []string
	k.IterateMartyranceQueue(ctx, func(submissionID string) bool {
		collected = append(collected, submissionID)
		return false
	})
	require.Len(t, collected, 3)
	for _, id := range ids {
		require.Contains(t, collected, id)
	}
}

func TestMartyranceQueue_DequeueNonExistent(t *testing.T) {
	k, ctx, _ := setupKeeperWithBank(t)
	// Dequeue on an entry that doesn't exist should not panic, and the counter
	// clamps at 0 rather than wrapping.
	require.NoError(t, k.DequeueMartyranceClaim(ctx, "nonexistent"))
	require.Equal(t, uint64(0), k.GetMartyranceQueueSize(ctx))
}

// ─── Martyrance Metadata Tests ────────────────────────────────────────────────

func TestMartyranceSubmissionMeta_RoundTrip(t *testing.T) {
	k, ctx, _ := setupKeeperWithBank(t)
	meta := types.MartyranceSubmissionMeta{
		SubmissionID: "s42",
		Testimony:    "I witness the truth.",
	}
	require.NoError(t, k.SetMartyranceSubmissionMeta(ctx, meta))

	got, ok := k.GetMartyranceSubmissionMeta(ctx, "s42")
	require.True(t, ok)
	require.Equal(t, meta.SubmissionID, got.SubmissionID)
	require.Equal(t, meta.Testimony, got.Testimony)
}

func TestMartyranceRoundMeta_RoundTrip(t *testing.T) {
	k, ctx, _ := setupKeeperWithBank(t)
	meta := types.MartyranceRoundMeta{
		RoundID:               "r10",
		IsMartyrance:          true,
		IsSecondaryMartyrance: false,
		ExcludedVerifiers:     nil,
	}
	require.NoError(t, k.SetMartyranceRoundMeta(ctx, meta))

	got, ok := k.GetMartyranceRoundMeta(ctx, "r10")
	require.True(t, ok)
	require.True(t, got.IsMartyrance)
	require.False(t, got.IsSecondaryMartyrance)

	_, ok = k.GetMartyranceRoundMeta(ctx, "nonexistent")
	require.False(t, ok)
}

func TestMartyranceRoundMeta_Secondary(t *testing.T) {
	k, ctx, _ := setupKeeperWithBank(t)
	meta := types.MartyranceRoundMeta{
		RoundID:               "r20",
		IsMartyrance:          true,
		IsSecondaryMartyrance: true,
		ExcludedVerifiers:     []string{rv1, rv2},
	}
	require.NoError(t, k.SetMartyranceRoundMeta(ctx, meta))

	got, ok := k.GetMartyranceRoundMeta(ctx, "r20")
	require.True(t, ok)
	require.True(t, got.IsSecondaryMartyrance)
	require.Equal(t, []string{rv1, rv2}, got.ExcludedVerifiers)
}

// ─── SubmitMartyranceClaim Tests ──────────────────────────────────────────────

func TestSubmitMartyranceClaim_TestimonyRequired(t *testing.T) {
	k, ctx, _ := setupKeeperWithRichBank(t)
	setupDefaultDomains(t, k, ctx)

	_, err := k.SubmitMartyranceClaim(ctx, &types.MsgSubmitMartyranceClaim{
		Submitter: rv1,
		Content:   "unique content for martyrance test",
		Domain:    "technology",
		Category:  "test",
		Testimony: "",
	})
	require.ErrorIs(t, err, types.ErrMartyranceTestimonyEmpty)
}

func TestSubmitMartyranceClaim_QueueFull(t *testing.T) {
	k, ctx, bk := setupKeeperWithRichBank(t)
	setupDefaultDomains(t, k, ctx)

	// Set a tiny capacity limit.
	mp := types.DefaultMartyranceParams()
	mp.MaxActiveMartyranceClaims = 1
	require.NoError(t, k.SetMartyranceParams(ctx, mp))

	// Fill the queue by pre-enqueuing.
	require.NoError(t, k.EnqueueMartyranceClaim(ctx, "pre-existing"))

	// Give rv1 a large balance so stake is sufficient.
	addr1, _ := sdk.AccAddressFromBech32(rv1)
	bk.SetBalance(addr1, sdkmath.NewInt(10_000_000))

	_, err := k.SubmitMartyranceClaim(ctx, &types.MsgSubmitMartyranceClaim{
		Submitter: rv1,
		Content:   "fresh martyrance claim",
		Domain:    "technology",
		Category:  "test",
		Testimony: "I testify to the truth of this content.",
	})
	require.ErrorIs(t, err, types.ErrMartyranceQueueFull)
}

func TestSubmitMartyranceClaim_InsufficientBalance(t *testing.T) {
	k, ctx, bk := setupKeeperWithRichBank(t)
	setupDefaultDomains(t, k, ctx)

	// rv1 has only 100 uzrn: 90% = 90 uzrn < 1_000_000 min stake.
	addr1, _ := sdk.AccAddressFromBech32(rv1)
	bk.SetBalance(addr1, sdkmath.NewInt(100))

	_, err := k.SubmitMartyranceClaim(ctx, &types.MsgSubmitMartyranceClaim{
		Submitter: rv1,
		Content:   "content with low balance",
		Domain:    "technology",
		Category:  "test",
		Testimony: "I am witnessing this.",
	})
	require.ErrorIs(t, err, types.ErrMartyranceStakeInsufficient)
}

func TestSubmitMartyranceClaim_Success(t *testing.T) {
	k, ctx, bk := setupKeeperWithRichBank(t)
	setupDefaultDomains(t, k, ctx)

	addr1, _ := sdk.AccAddressFromBech32(rv1)
	// 10 ZRN balance; 90% = 9 ZRN ≥ 1 ZRN min stake.
	bk.SetBalance(addr1, sdkmath.NewInt(10_000_000))

	testimony := "I, the author, solemnly testify to the truthfulness of this claim."
	resp, err := k.SubmitMartyranceClaim(ctx, &types.MsgSubmitMartyranceClaim{
		Submitter: rv1,
		Content:   "the verifiable martyrance claim content",
		Domain:    "technology",
		Category:  "epistemic",
		Testimony: testimony,
	})
	require.NoError(t, err)
	require.NotEmpty(t, resp.SubmissionId)
	require.NotEmpty(t, resp.RoundId)
	require.NotEmpty(t, resp.StakeAmount)

	// Stake should be 90% of 10_000_000 = 9_000_000.
	require.Equal(t, "9000000", resp.StakeAmount)

	// Submission stored with correct stake.
	sub, found := k.GetSubmission(ctx, resp.SubmissionId)
	require.True(t, found)
	require.Equal(t, "9000000", sub.Stake)
	require.Equal(t, rv1, sub.Submitter)
	require.Equal(t, types.SubmissionStatus_SUBMISSION_STATUS_PENDING_REVIEW, sub.Status)

	// Testimony stored in martyrance metadata.
	meta, ok := k.GetMartyranceSubmissionMeta(ctx, resp.SubmissionId)
	require.True(t, ok)
	require.Equal(t, testimony, meta.Testimony)
	require.Equal(t, resp.SubmissionId, meta.SubmissionID)

	// Queue size incremented.
	require.Equal(t, uint64(1), k.GetMartyranceQueueSize(ctx))
	require.True(t, k.IsMartyranceClaim(ctx, resp.SubmissionId))

	// Quality round created and is a martyrance round.
	round, found := k.GetQualityRound(ctx, resp.RoundId)
	require.True(t, found)
	require.Equal(t, resp.SubmissionId, round.SubmissionId)

	roundMeta, ok := k.GetMartyranceRoundMeta(ctx, resp.RoundId)
	require.True(t, ok)
	require.True(t, roundMeta.IsMartyrance)
	require.False(t, roundMeta.IsSecondaryMartyrance)

	// Bank escrow call recorded.
	require.Len(t, bk.accountToModuleCalls, 1)
	call := bk.accountToModuleCalls[0]
	require.Equal(t, rv1, call.from)
	require.Equal(t, types.ModuleName, call.to)
	require.Equal(t, sdkmath.NewInt(9_000_000), call.amount.AmountOf("uzrn"))
}

// ─── InitiateMartyranceQualityRound Tests ─────────────────────────────────────

func TestInitiateMartyranceQualityRound_ExtendedDeadlines(t *testing.T) {
	k, ctx, _ := setupKeeperWithBank(t)
	setupDefaultDomains(t, k, ctx)

	// Set a deadline multiplier of 3 to distinguish from the default.
	mp := types.DefaultMartyranceParams()
	mp.DeadlineMultiplier = 3
	require.NoError(t, k.SetMartyranceParams(ctx, mp))

	// Also read standard params for comparison.
	params, err := k.GetParams(ctx)
	require.NoError(t, err)
	expectedCommitPeriod := params.CommitPeriodBlocks * 3
	expectedRevealPeriod := params.RevealPeriodBlocks * 3

	// Create a submission to attach the round to.
	sub := &types.Submission{
		Id:          "sm1",
		Submitter:   testAddr,
		Content:     "martyrance round deadline test",
		Domain:      "technology",
		Stake:       "1000000",
		Status:      types.SubmissionStatus_SUBMISSION_STATUS_PENDING,
		ContentHash: "deadbeef01",
		Consent:     &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
	}
	require.NoError(t, k.SetSubmission(ctx, sub))

	roundID, err := k.InitiateMartyranceQualityRound(ctx, "sm1")
	require.NoError(t, err)

	round, found := k.GetQualityRound(ctx, roundID)
	require.True(t, found)

	// Block height is 100 (from setupKeeperWithBank).
	require.Equal(t, uint64(100)+expectedCommitPeriod, round.CommitDeadline)
	require.Equal(t, uint64(100)+expectedCommitPeriod+expectedRevealPeriod, round.RevealDeadline)

	// Round meta marks it as martyrance.
	meta, ok := k.GetMartyranceRoundMeta(ctx, roundID)
	require.True(t, ok)
	require.True(t, meta.IsMartyrance)
	require.False(t, meta.IsSecondaryMartyrance)
	require.Empty(t, meta.ExcludedVerifiers)
}

// ─── InitiateSecondaryMartyranceRound Tests ────────────────────────────────────

func TestInitiateSecondaryMartyranceRound_ExcludesFirstRoundVerifiers(t *testing.T) {
	k, ctx, _ := setupKeeperWithBank(t)
	setupDefaultDomains(t, k, ctx)

	sub := &types.Submission{
		Id:          "sm2",
		Submitter:   testAddr,
		Content:     "secondary round test",
		Domain:      "science",
		Stake:       "1000000",
		Status:      types.SubmissionStatus_SUBMISSION_STATUS_PENDING,
		ContentHash: "deadbeef02",
		Consent:     &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
	}
	require.NoError(t, k.SetSubmission(ctx, sub))

	firstRoundVerifiers := []string{rv1, rv2, rv3}
	roundID, err := k.InitiateSecondaryMartyranceRound(ctx, "sm2", firstRoundVerifiers)
	require.NoError(t, err)

	round, found := k.GetQualityRound(ctx, roundID)
	require.True(t, found)
	require.Equal(t, "sm2", round.SubmissionId)

	meta, ok := k.GetMartyranceRoundMeta(ctx, roundID)
	require.True(t, ok)
	require.True(t, meta.IsMartyrance)
	require.True(t, meta.IsSecondaryMartyrance)
	require.Equal(t, firstRoundVerifiers, meta.ExcludedVerifiers)
}

// ─── BeginBlocker Elevated Minimum Validators Test ───────────────────────────

func TestMartyranceRound_BeginBlocker_ElevatedMinValidators(t *testing.T) {
	k, ctx, _ := setupKeeperWithBank(t)
	setupDefaultDomains(t, k, ctx)

	// Create a martyrance submission and round.
	sub := &types.Submission{
		Id:          "sm3",
		Submitter:   testAddr,
		Content:     "martyrance elevated min validators test",
		Domain:      "technology",
		Stake:       "1000000",
		Status:      types.SubmissionStatus_SUBMISSION_STATUS_PENDING,
		ContentHash: "deadbeef03",
		Consent:     &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
	}
	require.NoError(t, k.SetSubmission(ctx, sub))

	roundID, err := k.InitiateMartyranceQualityRound(ctx, "sm3")
	require.NoError(t, err)

	// Advance past commit deadline with fewer than 5 commits — should expire.
	round, _ := k.GetQualityRound(ctx, roundID)
	// Add only 3 commits (below martyrance threshold of 5).
	round.Commits = []*types.CommitEntry{
		{Verifier: rv1, CommitHash: []byte("h1")},
		{Verifier: rv2, CommitHash: []byte("h2")},
		{Verifier: rv3, CommitHash: []byte("h3")},
	}
	require.NoError(t, k.SetQualityRound(ctx, round))

	// Advance block height beyond commit deadline.
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	expiredCtx := sdkCtx.WithBlockHeight(int64(round.CommitDeadline + 1))
	require.NoError(t, k.BeginBlocker(expiredCtx))

	// Round should be expired (3 commits < martyrance min of 5).
	updatedRound, found := k.GetQualityRound(ctx, roundID)
	require.True(t, found)
	require.Equal(t, types.VerificationPhase_VERIFICATION_PHASE_EXPIRED, updatedRound.Phase)

	// Submission stake returned (module → account).
}

// ─── DistributeMartyranceOutcome Tests ───────────────────────────────────────

func setupMartyranceRound(t *testing.T, stake string) (keeper.Keeper, sdk.Context, *mockBankKeeper, string) {
	t.Helper()
	k, ctx, bk := setupKeeperWithBank(t)
	setupDefaultDomains(t, k, ctx)

	require.NoError(t, k.SetReviewerStakingParams(ctx, types.DefaultReviewerStakingParams()))
	require.NoError(t, k.SetMartyranceParams(ctx, types.DefaultMartyranceParams()))

	sub := &types.Submission{
		Id:          "ms1",
		Domain:      "technology",
		Submitter:   testAddr,
		Content:     "martyrance outcome test content",
		Stake:       stake,
		Status:      types.SubmissionStatus_SUBMISSION_STATUS_PENDING,
		Consent:     &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
		ContentHash: "martyrance_hash_01",
	}
	require.NoError(t, k.SetSubmission(ctx, sub))

	verifiers := []string{rv1, rv2, rv3}
	roundID, err := k.InitiateMartyranceQualityRound(ctx, "ms1")
	require.NoError(t, err)
	require.NoError(t, k.EnqueueMartyranceClaim(ctx, "ms1"))

	// Set it as if verifiers have committed and revealed to reach aggregation.
	round, _ := k.GetQualityRound(ctx, roundID)
	round.SelectedVerifiers = verifiers
	require.NoError(t, k.SetQualityRound(ctx, round))

	return k, sdk.UnwrapSDKContext(ctx), bk, roundID
}

// runMartyranceRoundWithVotes runs commits and reveals directly on a martyrance
// round, bypassing the open-enrollment enrollment check by setting SelectedVerifiers.
func runMartyranceRoundWithVotes(t *testing.T, k keeper.Keeper, ctx context.Context, roundID string, votes []*types.QualityVote) {
	t.Helper()
	salts := [][]byte{[]byte("ms1"), []byte("ms2"), []byte("ms3")}
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

func TestMartyranceFirstPassAccept_StartsSecondaryRound(t *testing.T) {
	k, ctx, _, roundID := setupMartyranceRound(t, "1000000")

	// Three positive votes → first-pass accept.
	votes := []*types.QualityVote{
		{OverallQuality: 800000, ConsentValid: true},
		{OverallQuality: 850000, ConsentValid: true},
		{OverallQuality: 900000, ConsentValid: true},
	}
	runMartyranceRoundWithVotes(t, k, ctx, roundID, votes)

	require.NoError(t, k.AggregateQualityRound(ctx, roundID))

	// First round should be COMPLETE.
	round, found := k.GetQualityRound(ctx, roundID)
	require.True(t, found)
	require.Equal(t, types.VerificationPhase_VERIFICATION_PHASE_COMPLETE, round.Phase)

	// A secondary martyrance round must have been created for the same submission.
	sub, found := k.GetSubmission(ctx, "ms1")
	require.True(t, found)
	// The submission's QualityRoundId should point to the new secondary round.
	require.NotEqual(t, roundID, sub.QualityRoundId,
		"submission round_id should be updated to the secondary round")

	secondaryMeta, ok := k.GetMartyranceRoundMeta(ctx, sub.QualityRoundId)
	require.True(t, ok)
	require.True(t, secondaryMeta.IsMartyrance)
	require.True(t, secondaryMeta.IsSecondaryMartyrance)

	// Submission should still be pending review (not complete yet).
	require.Equal(t, types.SubmissionStatus_SUBMISSION_STATUS_PENDING_REVIEW, sub.Status)

	// Queue size unchanged (still active).
	require.Equal(t, uint64(1), k.GetMartyranceQueueSize(ctx))
}

func TestMartyranceRejection_BurnsStake(t *testing.T) {
	k, ctx, bk, roundID := setupMartyranceRound(t, "1000000")

	// Three reject votes.
	votes := []*types.QualityVote{
		{OverallQuality: 100000, ConsentValid: true},
		{OverallQuality: 150000, ConsentValid: true},
		{OverallQuality: 200000, ConsentValid: true},
	}
	runMartyranceRoundWithVotes(t, k, ctx, roundID, votes)

	require.NoError(t, k.AggregateQualityRound(ctx, roundID))

	// Round should be complete/rejected.
	round, found := k.GetQualityRound(ctx, roundID)
	require.True(t, found)
	require.Equal(t, types.VerificationPhase_VERIFICATION_PHASE_COMPLETE, round.Phase)

	// Queue should be cleared.
	require.Equal(t, uint64(0), k.GetMartyranceQueueSize(ctx))

	// Submitter should NOT receive their stake back (it was burned).
	refundToSubmitter := findTransfer(bk.moduleToAccountCalls, func(bt bankTransfer) bool {
		return bt.to == testAddr && bt.from == types.ModuleName
	})
	require.Nil(t, refundToSubmitter, "submitter stake must not be returned on martyrance rejection")

	// Majority verifiers received payouts.
	majorityPaid := 0
	for _, addr := range []string{rv1, rv2, rv3} {
		t := findTransfer(bk.moduleToAccountCalls, func(bt bankTransfer) bool {
			return bt.to == addr
		})
		if t != nil {
			majorityPaid++
		}
	}
	require.Greater(t, majorityPaid, 0, "majority verifiers should receive payouts")
}
