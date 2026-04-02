package cross_stack_test

import (
	"fmt"
	"testing"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	zeroneauthtypes "github.com/zerone-chain/zerone/x/auth/types"
	knowledgetypes "github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Test Helpers ────────────────────────────────────────────────────────────

// setupDistributedVerifierSet creates n independent verifier accounts, each
// funded with 500_000 uzrn (enough for reviewer stake), and returns their addresses.
func setupDistributedVerifierSet(t *testing.T, h *TestHarness, prefix string, n int) []sdk.AccAddress {
	t.Helper()
	addrs := make([]sdk.AccAddress, n)
	for i := 0; i < n; i++ {
		seed := fmt.Sprintf("%s_v%02d____", prefix, i)
		addrs[i] = testAddr(seed)
		require.NoError(t, h.FundAccount(addrs[i], sdk.NewCoins(sdk.NewInt64Coin("uzrn", 500_000))))
	}
	return addrs
}

// fundKnowledgeModule pre-loads the knowledge module account with uzrn so that
// distributeAccept/distributeReject bank sends can succeed.
func fundKnowledgeModule(t *testing.T, h *TestHarness, amount int64) {
	t.Helper()
	coins := sdk.NewCoins(sdk.NewInt64Coin("uzrn", amount))
	require.NoError(t, h.BankKeeper.MintCoins(h.Ctx, zeroneauthtypes.ModuleName, coins))
	require.NoError(t, h.BankKeeper.SendCoinsFromModuleToModule(
		h.Ctx, zeroneauthtypes.ModuleName, knowledgetypes.ModuleName, coins,
	))
}

// runCrossStackVerificationRound runs commit-reveal for the given round,
// submitting the provided votes in order. Phase is forced to REVEAL after commits.
func runCrossStackVerificationRound(
	t *testing.T,
	h *TestHarness,
	roundID string,
	verifiers []sdk.AccAddress,
	votes []*knowledgetypes.QualityVote,
) {
	t.Helper()
	require.Equal(t, len(verifiers), len(votes), "verifier and vote counts must match")

	// ── Commit phase ──────────────────────────────────────────────────────────
	for i, addr := range verifiers {
		salt := []byte(fmt.Sprintf("edimance_salt_%02d", i))
		hash := knowledgetypes.ComputeQualityCommitHash(roundID, votes[i], salt)
		require.NoError(t, h.KnowledgeKeeper.SubmitCommitment(h.Ctx, &knowledgetypes.MsgSubmitCommitment{
			Verifier:   addr.String(),
			RoundId:    roundID,
			CommitHash: hash,
		}), "verifier %d commit failed", i)
	}

	// ── Advance to reveal phase (direct state transition) ────────────────────
	round, found := h.KnowledgeKeeper.GetQualityRound(h.Ctx, roundID)
	require.True(t, found, "round not found after commits")
	round.Phase = knowledgetypes.VerificationPhase_VERIFICATION_PHASE_REVEAL
	require.NoError(t, h.KnowledgeKeeper.SetQualityRound(h.Ctx, round))

	// ── Reveal phase ──────────────────────────────────────────────────────────
	for i, addr := range verifiers {
		salt := []byte(fmt.Sprintf("edimance_salt_%02d", i))
		require.NoError(t, h.KnowledgeKeeper.SubmitReveal(h.Ctx, &knowledgetypes.MsgSubmitReveal{
			Verifier: addr.String(),
			RoundId:  roundID,
			Scores:   votes[i],
			Salt:     salt,
		}), "verifier %d reveal failed", i)
	}
}

// hasStakingEvent returns true if the context event manager has a reviewer_staking
// event matching the given round, outcome, majority, and minority counts.
func hasStakingEvent(events sdk.Events, roundID, outcome string, majority, minority int) bool {
	for _, e := range events {
		if e.Type != "reviewer_staking" {
			continue
		}
		attrs := make(map[string]string, len(e.Attributes))
		for _, a := range e.Attributes {
			attrs[a.Key] = a.Value
		}
		if attrs["round_id"] == roundID &&
			attrs["outcome"] == outcome &&
			attrs["majority_count"] == fmt.Sprintf("%d", majority) &&
			attrs["minority_count"] == fmt.Sprintf("%d", minority) {
			return true
		}
	}
	return false
}

// ─── Scenario A: Multi-Verifier Consensus ────────────────────────────────────

// TestEdimanceA_MultiVerifierConsensus validates that 5 verifiers with 4 accepting
// and 1 rejecting produces an ACCEPT staking outcome, pays majority, and applies
// DOKIMANT partial retention to the minority.
func TestEdimanceA_MultiVerifierConsensus(t *testing.T) {
	h := NewTestHarness(t)

	// ── Setup: fund module (submitter stake simulation) + 5 verifiers ─────────
	const submitterStake = 1_000_000
	fundKnowledgeModule(t, h, 2_000_000) // extra buffer for distribution
	verifiers := setupDistributedVerifierSet(t, h, "scen_a", 5)

	submitter := testAddr("submitter_a_____")
	require.NoError(t, h.FundAccount(submitter, sdk.NewCoins(sdk.NewInt64Coin("uzrn", 100))))

	// ── Create submission with 1 ZRN stake ────────────────────────────────────
	sub := &knowledgetypes.Submission{
		Id:          "edimance-a-sub",
		Domain:      "technology",
		Submitter:   submitter.String(),
		Content:     "EDIMANCE Scenario A: multi-verifier consensus",
		Stake:       fmt.Sprintf("%d", submitterStake),
		Status:      knowledgetypes.SubmissionStatus_SUBMISSION_STATUS_PENDING,
		Consent:     &knowledgetypes.ConsentProof{Type: knowledgetypes.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
		ContentHash: "edimance_a_hash_001",
	}
	require.NoError(t, h.KnowledgeKeeper.SetSubmission(h.Ctx, sub))

	// ── Build verifier string list and initiate round ──────────────────────────
	verifierStrs := make([]string, len(verifiers))
	for i, v := range verifiers {
		verifierStrs[i] = v.String()
	}
	roundID, err := h.KnowledgeKeeper.InitiateQualityRound(h.Ctx, "edimance-a-sub", "", verifierStrs)
	require.NoError(t, err)

	// ── Votes: v0-v3 accept (high quality), v4 rejects (minority) ─────────────
	// All accept votes are above BronzeThreshold=400_000.
	// The reject vote is below it → reject side.
	votes := []*knowledgetypes.QualityVote{
		{OverallQuality: 810_000, ReasoningDepth: 720_000, Novelty: 650_000, Toxicity: 80_000, FactualAccuracy: 760_000, ConsentValid: true},
		{OverallQuality: 790_000, ReasoningDepth: 700_000, Novelty: 630_000, Toxicity: 90_000, FactualAccuracy: 740_000, ConsentValid: true},
		{OverallQuality: 830_000, ReasoningDepth: 710_000, Novelty: 670_000, Toxicity: 75_000, FactualAccuracy: 770_000, ConsentValid: true},
		{OverallQuality: 800_000, ReasoningDepth: 730_000, Novelty: 660_000, Toxicity: 85_000, FactualAccuracy: 750_000, ConsentValid: true},
		{OverallQuality: 180_000, ReasoningDepth: 150_000, Novelty: 100_000, Toxicity: 700_000, FactualAccuracy: 150_000, ConsentValid: true}, // minority
	}

	// ── Run commit-reveal ─────────────────────────────────────────────────────
	runCrossStackVerificationRound(t, h, roundID, verifiers, votes)

	// ── Aggregate ─────────────────────────────────────────────────────────────
	require.NoError(t, h.KnowledgeKeeper.AggregateQualityRound(h.Ctx, roundID))

	// ── Assertions: verdict ───────────────────────────────────────────────────
	round, found := h.KnowledgeKeeper.GetQualityRound(h.Ctx, roundID)
	require.True(t, found)
	require.Equal(t, knowledgetypes.VerificationPhase_VERIFICATION_PHASE_COMPLETE, round.Phase,
		"round must be COMPLETE after aggregation")

	// Aggregate overall quality = median of [800K,810K,790K,830K,180K] = 800K → SILVER/GOLD
	accepted := round.Verdict == knowledgetypes.QualityVerdict_QUALITY_VERDICT_GOLD ||
		round.Verdict == knowledgetypes.QualityVerdict_QUALITY_VERDICT_SILVER ||
		round.Verdict == knowledgetypes.QualityVerdict_QUALITY_VERDICT_BRONZE
	require.True(t, accepted, "verdict must be ACCEPTED (GOLD/SILVER/BRONZE), got: %v", round.Verdict)

	// 5 verifiers participated.
	require.Len(t, round.Reveals, 5, "all 5 verifiers must have revealed")

	// ── Assertions: staking event (ACCEPT, 4 majority, 1 minority) ────────────
	events := h.Ctx.EventManager().Events()
	require.True(t, hasStakingEvent(events, roundID, "accept", 4, 1),
		"expected reviewer_staking event: outcome=accept majority=4 minority=1")

	// ── Assertions: DOKIMANT partial retention ────────────────────────────────
	// Minority verifier (v4) must have received 20% of their escrowed stake.
	// Reviewer stake per verifier = 1_000_000 × 30% = 300_000.
	// MinorityRetention = 300_000 × 20% = 60_000.
	// Before commit: 500_000. After escrow: 200_000. After partial return: 260_000.
	minorityBalance := h.GetBalance(verifiers[4], "uzrn")
	require.Equal(t, sdkmath.NewInt(260_000), minorityBalance.Amount,
		"minority verifier must have 260K after partial retention (500K funded - 300K escrowed + 60K returned)")

	// Majority verifiers must each have more than their initial balance after escrow.
	// Each majority: 500K funded - 300K escrowed + 300K own stake back + 6K show-up share = 506K.
	for i := 0; i < 4; i++ {
		bal := h.GetBalance(verifiers[i], "uzrn")
		require.True(t, bal.Amount.GT(sdkmath.NewInt(200_000)),
			"majority verifier %d must receive rewards (balance > 200K post-escrow)", i)
	}

	// Submission must be ACCEPTED.
	updatedSub, found := h.KnowledgeKeeper.GetSubmission(h.Ctx, "edimance-a-sub")
	require.True(t, found)
	require.Equal(t, knowledgetypes.SubmissionStatus_SUBMISSION_STATUS_ACCEPTED, updatedSub.Status)
}

// ─── Scenario B: Split Verification (No Supermajority) ───────────────────────

// TestEdimanceB_SplitVerificationNoSupermajority validates that 3/5 accepting
// (60% < 66.7% threshold) triggers DEEP_CONTESTED staking outcome and records a
// contested strike on the content hash.
func TestEdimanceB_SplitVerificationNoSupermajority(t *testing.T) {
	h := NewTestHarness(t)

	fundKnowledgeModule(t, h, 2_000_000)
	verifiers := setupDistributedVerifierSet(t, h, "scen_b", 5)
	submitter := testAddr("submitter_b_____")

	sub := &knowledgetypes.Submission{
		Id:          "edimance-b-sub",
		Domain:      "technology",
		Submitter:   submitter.String(),
		Content:     "EDIMANCE Scenario B: split verification",
		Stake:       "1000000",
		Status:      knowledgetypes.SubmissionStatus_SUBMISSION_STATUS_PENDING,
		Consent:     &knowledgetypes.ConsentProof{Type: knowledgetypes.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
		ContentHash: "edimance_b_hash_001",
	}
	require.NoError(t, h.KnowledgeKeeper.SetSubmission(h.Ctx, sub))

	verifierStrs := make([]string, len(verifiers))
	for i, v := range verifiers {
		verifierStrs[i] = v.String()
	}
	roundID, err := h.KnowledgeKeeper.InitiateQualityRound(h.Ctx, "edimance-b-sub", "", verifierStrs)
	require.NoError(t, err)

	// 3 accept (≥400K), 2 reject (<400K). 3/5 = 60% < 66.7% → no supermajority either way.
	votes := []*knowledgetypes.QualityVote{
		{OverallQuality: 800_000, ConsentValid: true}, // accept
		{OverallQuality: 750_000, ConsentValid: true}, // accept
		{OverallQuality: 820_000, ConsentValid: true}, // accept
		{OverallQuality: 200_000, ConsentValid: true}, // reject
		{OverallQuality: 150_000, ConsentValid: true}, // reject
	}
	runCrossStackVerificationRound(t, h, roundID, verifiers, votes)
	require.NoError(t, h.KnowledgeKeeper.AggregateQualityRound(h.Ctx, roundID))

	// ── Staking outcome must be DEEP_CONTESTED ────────────────────────────────
	// acceptCount=3, rejectCount=2, total=5:
	//   3×3=9 < 5×2=10 → not ACCEPT; 2×3=6 < 5×2=10 → not REJECT → DEEP_CONTESTED.
	events := h.Ctx.EventManager().Events()
	require.True(t, hasStakingEvent(events, roundID, "deep_contested", 0, 0),
		"expected DEEP_CONTESTED staking event")

	// ── Contested strike recorded ─────────────────────────────────────────────
	strikeCount := h.KnowledgeKeeper.GetContestedDeepCount(h.Ctx, "edimance_b_hash_001")
	require.Equal(t, uint64(1), strikeCount, "one contested strike must be recorded on content hash")

	// ── All reviewer stakes returned ──────────────────────────────────────────
	// Each verifier: 500K funded - 300K escrowed + 300K returned = 500K.
	for i, v := range verifiers {
		bal := h.GetBalance(v, "uzrn")
		require.Equal(t, sdkmath.NewInt(500_000), bal.Amount,
			"verifier %d should have full stake returned on DEEP_CONTESTED", i)
	}

	// Round is COMPLETE; submission status may be accepted or rejected based on aggregate.
	round, found := h.KnowledgeKeeper.GetQualityRound(h.Ctx, roundID)
	require.True(t, found)
	require.Equal(t, knowledgetypes.VerificationPhase_VERIFICATION_PHASE_COMPLETE, round.Phase)
}

// ─── Scenario C: Concurrent Rounds Independence ──────────────────────────────

// TestEdimanceC_ConcurrentRoundsIndependence verifies that 3 quality rounds
// running simultaneously with distinct verifier sets do not interfere with each
// other — verdicts and state are fully isolated per round.
func TestEdimanceC_ConcurrentRoundsIndependence(t *testing.T) {
	h := NewTestHarness(t)

	// Use zero stake: we are testing isolation, not reward distribution.
	const roundCount = 3
	roundIDs := make([]string, roundCount)

	for r := 0; r < roundCount; r++ {
		prefix := fmt.Sprintf("scen_c_%d", r)
		verifiers := setupDistributedVerifierSet(t, h, prefix, 3)
		verifierStrs := make([]string, 3)
		for i, v := range verifiers {
			verifierStrs[i] = v.String()
		}

		subID := fmt.Sprintf("edimance-c-sub-%d", r)
		sub := &knowledgetypes.Submission{
			Id:          subID,
			Domain:      "technology",
			Submitter:   testAddr(fmt.Sprintf("submitter_c_%d___", r)).String(),
			Content:     fmt.Sprintf("EDIMANCE Scenario C: concurrent round %d", r),
			Stake:       "0",
			Status:      knowledgetypes.SubmissionStatus_SUBMISSION_STATUS_PENDING,
			Consent:     &knowledgetypes.ConsentProof{Type: knowledgetypes.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
			ContentHash: fmt.Sprintf("edimance_c_hash_%03d", r),
		}
		require.NoError(t, h.KnowledgeKeeper.SetSubmission(h.Ctx, sub))

		rID, err := h.KnowledgeKeeper.InitiateQualityRound(h.Ctx, subID, "", verifierStrs)
		require.NoError(t, err)
		roundIDs[r] = rID

		// Votes: unanimous accept for rounds 0,1; unanimous reject for round 2.
		var votes []*knowledgetypes.QualityVote
		if r < 2 {
			votes = []*knowledgetypes.QualityVote{
				{OverallQuality: 800_000, ConsentValid: true},
				{OverallQuality: 820_000, ConsentValid: true},
				{OverallQuality: 790_000, ConsentValid: true},
			}
		} else {
			votes = []*knowledgetypes.QualityVote{
				{OverallQuality: 100_000, ConsentValid: true},
				{OverallQuality: 150_000, ConsentValid: true},
				{OverallQuality: 120_000, ConsentValid: true},
			}
		}
		runCrossStackVerificationRound(t, h, rID, verifiers, votes)
	}

	// ── Aggregate all rounds independently ────────────────────────────────────
	for _, rID := range roundIDs {
		require.NoError(t, h.KnowledgeKeeper.AggregateQualityRound(h.Ctx, rID))
	}

	// ── Assert: each round is COMPLETE with independent verdicts ──────────────
	for r, rID := range roundIDs {
		round, found := h.KnowledgeKeeper.GetQualityRound(h.Ctx, rID)
		require.True(t, found, "round %d must exist", r)
		require.Equal(t, knowledgetypes.VerificationPhase_VERIFICATION_PHASE_COMPLETE, round.Phase,
			"round %d must be COMPLETE", r)
		require.Len(t, round.Reveals, 3, "round %d must have exactly 3 reveals", r)

		// Rounds 0 and 1: accepted (aggregate quality is high).
		if r < 2 {
			accepted := round.Verdict == knowledgetypes.QualityVerdict_QUALITY_VERDICT_GOLD ||
				round.Verdict == knowledgetypes.QualityVerdict_QUALITY_VERDICT_SILVER ||
				round.Verdict == knowledgetypes.QualityVerdict_QUALITY_VERDICT_BRONZE
			require.True(t, accepted, "round %d with high-quality votes must be ACCEPTED", r)
		}
		// Round 2: rejected (aggregate quality is low).
		if r == 2 {
			require.Equal(t, knowledgetypes.QualityVerdict_QUALITY_VERDICT_REJECT, round.Verdict,
				"round 2 with low-quality votes must be REJECTED")
		}
	}

	// ── Assert: round IDs are distinct ───────────────────────────────────────
	seen := make(map[string]bool)
	for _, rID := range roundIDs {
		require.False(t, seen[rID], "duplicate round ID detected: %s", rID)
		seen[rID] = true
	}

	// ── Assert: submission states are independent ─────────────────────────────
	for r := 0; r < roundCount; r++ {
		subID := fmt.Sprintf("edimance-c-sub-%d", r)
		sub, found := h.KnowledgeKeeper.GetSubmission(h.Ctx, subID)
		require.True(t, found)
		if r < 2 {
			require.Equal(t, knowledgetypes.SubmissionStatus_SUBMISSION_STATUS_ACCEPTED, sub.Status,
				"submission %d must be ACCEPTED", r)
		} else {
			require.Equal(t, knowledgetypes.SubmissionStatus_SUBMISSION_STATUS_REJECTED, sub.Status,
				"submission 2 must be REJECTED")
		}
	}

	// ── Assert: no cross-contamination in round state ─────────────────────────
	// Each round's AggregateScores should only reflect its own verifiers' votes.
	for r, rID := range roundIDs {
		round, _ := h.KnowledgeKeeper.GetQualityRound(h.Ctx, rID)
		require.NotNil(t, round.AggregateScores, "round %d must have aggregate scores", r)
		if r < 2 {
			require.GreaterOrEqual(t, round.AggregateScores.OverallQuality, uint64(400_000),
				"round %d aggregate quality must be high (accept territory)", r)
		} else {
			require.Less(t, round.AggregateScores.OverallQuality, uint64(400_000),
				"round 2 aggregate quality must be low (reject territory)")
		}
	}
}
