package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/keeper"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Review Fee Distribution Tests (R19-6) ───────────────────────────────────

func TestSubmitClaim_FeeDistributed(t *testing.T) {
	// When a claim is submitted, the review fee should be collected from the
	// submitter and distributed: 55% verifier pool (stays in module),
	// 22% protocol treasury, 19.67% development, 3.33% research.
	k, ctx, bk := setupKnowledgeTestWithBank(t)
	ms := keeper.NewMsgServerImpl(k)

	submitter := makeValidBech32Addr("submitter-fee")

	_, err := ms.SubmitClaim(ctx, &types.MsgSubmitClaim{
		Submitter:   submitter,
		FactContent: "This is a long enough claim to pass validation checks for fee distribution",
		Domain:      "mathematics",
		Category:    "formal",
		Stake:       "1000000", // 1 ZRN
	})
	require.NoError(t, err)

	// Should have bank calls:
	// 1. SendCoinsFromAccountToModule (collect fee from submitter → knowledge)
	// 2. SendCoinsFromModuleToModule (knowledge → protocol_treasury) — 22%
	// 3. SendCoinsFromModuleToModule (knowledge → development_fund) — 19.67%
	// 4. SendCoinsFromModuleToModule (knowledge → research_fund) — 3.33%
	// (55% stays in knowledge module — no send needed)
	require.GreaterOrEqual(t, len(bk.sendCalls), 4,
		"fee distribution should generate at least 4 bank sends")

	// First call: collect fee from submitter
	require.Equal(t, submitter, bk.sendCalls[0].from)
	require.Equal(t, "knowledge", bk.sendCalls[0].to)

	// Verify distribution targets (order: protocol, development, research)
	var protocolSent, devSent, researchSent bool
	for _, call := range bk.sendCalls[1:] {
		switch call.to {
		case "protocol_treasury":
			protocolSent = true
			// 22% of 1,000,000 = 220,000
			require.Equal(t, int64(220_000), call.coins.AmountOf("uzrn").Int64())
		case "development_fund":
			devSent = true
			// 19.67% of 1,000,000 = 196,700
			require.Equal(t, int64(196_700), call.coins.AmountOf("uzrn").Int64())
		case "research_fund":
			researchSent = true
			// 3.33% = remainder: 1,000,000 - 550,000 - 220,000 - 196,700 = 33,300
			require.Equal(t, int64(33_300), call.coins.AmountOf("uzrn").Int64())
		}
	}
	require.True(t, protocolSent, "protocol treasury should receive 22%")
	require.True(t, devSent, "development fund should receive 19.67%")
	require.True(t, researchSent, "research fund should receive 3.33%")
}

func TestCompleteRound_Accept_NoRefund(t *testing.T) {
	// R19-6: accepted claims should NOT get a refund — fee is non-refundable
	k, ctx, bk := setupKnowledgeTestWithBank(t)

	claim := &types.Claim{
		Id:          "claim-accept-nrf",
		FactContent: "An accepted claim with non-refundable fee",
		Domain:      "mathematics",
		Category:    "formal",
		Submitter:   "zrn1sub",
		Stake:       "500000",
		Status:      types.ClaimStatus_CLAIM_STATUS_IN_VERIFICATION,
	}
	require.NoError(t, k.SetClaim(ctx, claim))

	round := makeRoundInPhase("r-accept-nrf", "claim-accept-nrf", types.VerificationPhase_VERIFICATION_PHASE_AGGREGATION, 80)
	require.NoError(t, k.SetVerificationRound(ctx, round))

	initialSendCount := len(bk.sendCalls)
	result := &keeper.VerificationResult{
		Verdict:    types.Verdict_VERDICT_ACCEPT,
		Confidence: 850_000,
	}

	require.NoError(t, k.CompleteRound(ctx, round, result))

	updatedClaim, _ := k.GetClaim(ctx, "claim-accept-nrf")
	require.Equal(t, types.ClaimStatus_CLAIM_STATUS_ACCEPTED, updatedClaim.Status)

	// No SendCoinsFromModuleToAccount calls — fee is non-refundable
	for _, call := range bk.sendCalls[initialSendCount:] {
		require.NotEqual(t, "zrn1sub", call.to,
			"accepted claim should NOT return fee to submitter")
	}
}

func TestCompleteRound_Reject_NoRefund(t *testing.T) {
	// R19-6: rejected claims get no refund and no additional slashing
	k, ctx, bk := setupKnowledgeTestWithBank(t)

	claim := &types.Claim{
		Id:          "claim-reject-nrf",
		FactContent: "A rejected claim with non-refundable fee",
		Domain:      "physics",
		Submitter:   "zrn1sub",
		Stake:       "1000000",
		Status:      types.ClaimStatus_CLAIM_STATUS_IN_VERIFICATION,
	}
	require.NoError(t, k.SetClaim(ctx, claim))

	round := makeRoundInPhase("r-reject-nrf", "claim-reject-nrf", types.VerificationPhase_VERIFICATION_PHASE_AGGREGATION, 80)
	require.NoError(t, k.SetVerificationRound(ctx, round))

	initialSendCount := len(bk.sendCalls)
	result := &keeper.VerificationResult{
		Verdict:    types.Verdict_VERDICT_REJECT,
		Confidence: 850_000,
	}

	require.NoError(t, k.CompleteRound(ctx, round, result))

	updatedClaim, _ := k.GetClaim(ctx, "claim-reject-nrf")
	require.Equal(t, types.ClaimStatus_CLAIM_STATUS_REJECTED, updatedClaim.Status)

	// No bank sends on rejection — fee already distributed at submission
	require.Equal(t, initialSendCount, len(bk.sendCalls),
		"rejected claim should NOT trigger any bank sends")
}

func TestCompleteRound_Inconclusive_NoRefund(t *testing.T) {
	// R19-6: inconclusive claims get no refund — verifiers still did work
	k, ctx, bk := setupKnowledgeTestWithBank(t)

	claim := &types.Claim{
		Id:          "claim-inc-nrf",
		FactContent: "An inconclusive claim with non-refundable fee",
		Domain:      "general",
		Submitter:   "zrn1sub",
		Stake:       "1000000",
		Status:      types.ClaimStatus_CLAIM_STATUS_IN_VERIFICATION,
	}
	require.NoError(t, k.SetClaim(ctx, claim))

	round := makeRoundInPhase("r-inc-nrf", "claim-inc-nrf", types.VerificationPhase_VERIFICATION_PHASE_AGGREGATION, 80)
	require.NoError(t, k.SetVerificationRound(ctx, round))

	initialSendCount := len(bk.sendCalls)
	result := &keeper.VerificationResult{
		Verdict:    types.Verdict_VERDICT_INCONCLUSIVE,
		Confidence: 0,
	}

	require.NoError(t, k.CompleteRound(ctx, round, result))

	updatedClaim, _ := k.GetClaim(ctx, "claim-inc-nrf")
	require.Equal(t, types.ClaimStatus_CLAIM_STATUS_INSUFFICIENT, updatedClaim.Status)

	// No bank sends — fee already distributed at submission
	require.Equal(t, initialSendCount, len(bk.sendCalls),
		"inconclusive claim should NOT trigger any bank sends")
}

func TestVerifierRewards_FundedFromPool(t *testing.T) {
	// Verifier rewards should come from the 55% fee pool, not from a fixed amount
	k, ctx, bk, sk := setupKnowledgeTestFull(t)

	verifier1 := makeValidBech32Addr("verifier1")
	sk.addValidator(verifier1, 100_000, "bonded")

	claim := &types.Claim{
		Id:          "claim-pool",
		FactContent: "Claim with pool-funded verifier rewards",
		Domain:      "physics",
		Category:    "empirical",
		Submitter:   "zrn1sub",
		Stake:       "1000000", // 1 ZRN fee → 550,000 uzrn (55%) verifier pool
		Status:      types.ClaimStatus_CLAIM_STATUS_IN_VERIFICATION,
	}
	require.NoError(t, k.SetClaim(ctx, claim))

	round := makeRoundInPhase("r-pool", "claim-pool", types.VerificationPhase_VERIFICATION_PHASE_AGGREGATION, 80)
	require.NoError(t, k.SetVerificationRound(ctx, round))

	initialSendCount := len(bk.sendCalls)
	result := &keeper.VerificationResult{
		Verdict:    types.Verdict_VERDICT_ACCEPT,
		Confidence: 900_000,
		Rewards: []keeper.VerifierReward{
			{Verifier: verifier1, Amount: 3_000_000}, // Amount field ignored — pool-based now
		},
	}

	require.NoError(t, k.CompleteRound(ctx, round, result))

	// Should have a bank send from knowledge module to the verifier
	var verifierPaid bool
	for _, call := range bk.sendCalls[initialSendCount:] {
		if call.from == "knowledge" {
			// 55% of 1,000,000 = 550,000 — should go to verifier
			if call.coins.AmountOf("uzrn").Int64() == 550_000 {
				verifierPaid = true
			}
		}
	}
	require.True(t, verifierPaid,
		"verifier should receive 55%% of review fee (550,000 uzrn)")
	_ = sk
}

func TestVerifierRewards_SplitEvenly(t *testing.T) {
	// Pool should be divided equally among multiple correct verifiers
	k, ctx, bk, sk := setupKnowledgeTestFull(t)

	verifier1 := makeValidBech32Addr("verifier1-split")
	verifier2 := makeValidBech32Addr("verifier2-split")
	sk.addValidator(verifier1, 100_000, "bonded")
	sk.addValidator(verifier2, 100_000, "bonded")

	claim := &types.Claim{
		Id:          "claim-split",
		FactContent: "Claim with multiple verifiers splitting pool",
		Domain:      "physics",
		Category:    "empirical",
		Submitter:   "zrn1sub",
		Stake:       "1000000", // 55% = 550,000 pool ÷ 2 = 275,000 each
		Status:      types.ClaimStatus_CLAIM_STATUS_IN_VERIFICATION,
	}
	require.NoError(t, k.SetClaim(ctx, claim))

	round := makeRoundInPhase("r-split", "claim-split", types.VerificationPhase_VERIFICATION_PHASE_AGGREGATION, 80)
	require.NoError(t, k.SetVerificationRound(ctx, round))

	initialSendCount := len(bk.sendCalls)
	result := &keeper.VerificationResult{
		Verdict:    types.Verdict_VERDICT_ACCEPT,
		Confidence: 900_000,
		Rewards: []keeper.VerifierReward{
			{Verifier: verifier1, Amount: 0},
			{Verifier: verifier2, Amount: 0},
		},
	}

	require.NoError(t, k.CompleteRound(ctx, round, result))

	// Count verifier reward sends — should be 2 sends of 275,000 each
	var verifierSends int
	for _, call := range bk.sendCalls[initialSendCount:] {
		if call.from == "knowledge" {
			verifierSends++
		}
	}
	require.Equal(t, 2, verifierSends, "should send rewards to 2 verifiers")
	_ = sk
}

func TestReviewFee_BelowMinimum(t *testing.T) {
	// Submitting with a fee below MinReviewFee should fail
	k, ctx := setupKnowledgeTest(t)
	ms := keeper.NewMsgServerImpl(k)

	_, err := ms.SubmitClaim(ctx, &types.MsgSubmitClaim{
		Submitter:   makeValidBech32Addr("submitter-low"),
		FactContent: "This claim has a fee below the minimum review fee amount",
		Domain:      "physics",
		Stake:       "100", // below MinReviewFee of 100000
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "below minimum")
}

func TestReviewFee_AboveMinimum(t *testing.T) {
	// Higher fee = larger verifier pool — submitting above minimum should work
	// and produce proportionally larger distribution
	k, ctx, bk := setupKnowledgeTestWithBank(t)
	ms := keeper.NewMsgServerImpl(k)

	submitter := makeValidBech32Addr("submitter-high")

	_, err := ms.SubmitClaim(ctx, &types.MsgSubmitClaim{
		Submitter:   submitter,
		FactContent: "This claim has a higher review fee for better review quality",
		Domain:      "mathematics",
		Category:    "formal",
		Stake:       "10000000", // 10 ZRN — 10× the minimum
	})
	require.NoError(t, err)

	// Verify proportional distribution with 10 ZRN fee:
	// Protocol: 22% of 10,000,000 = 2,200,000
	// Development: 19.67% of 10,000,000 = 1,967,000
	// Research: remainder = 10,000,000 - 5,500,000 - 2,200,000 - 1,967,000 = 333,000
	var protocolAmt, devAmt, researchAmt int64
	for _, call := range bk.sendCalls {
		switch call.to {
		case "protocol_treasury":
			protocolAmt = call.coins.AmountOf("uzrn").Int64()
		case "development_fund":
			devAmt = call.coins.AmountOf("uzrn").Int64()
		case "research_fund":
			researchAmt = call.coins.AmountOf("uzrn").Int64()
		}
	}
	require.Equal(t, int64(2_200_000), protocolAmt, "protocol should get 22%% of 10 ZRN")
	require.Equal(t, int64(1_967_000), devAmt, "development should get 19.67%% of 10 ZRN")
	require.Equal(t, int64(333_000), researchAmt, "research should get remainder of 10 ZRN")
}
