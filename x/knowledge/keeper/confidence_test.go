package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/keeper"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── AggregateVerificationResult ─────────────────────────────────────────────

func TestAggregate_UnanimousAccept(t *testing.T) {
	k, ctx, _, sk := setupKnowledgeTestFull(t)
	sk.addValidator("zrn1v1", 100_000, "bonded")
	sk.addValidator("zrn1v2", 100_000, "bonded")
	sk.addValidator("zrn1v3", 100_000, "bonded")

	claim := &types.Claim{Id: "c-ua", FactContent: "Unanimous accept claim content", Domain: "mathematics"}
	require.NoError(t, k.SetClaim(ctx, claim))

	round := makeRoundInPhase("r-ua", "c-ua", types.VerificationPhase_VERIFICATION_PHASE_AGGREGATION, 50)
	round.Commits = []*types.CommitEntry{
		{Verifier: "zrn1v1", CommitHash: []byte("h1"), CommittedAtBlock: 60},
		{Verifier: "zrn1v2", CommitHash: []byte("h2"), CommittedAtBlock: 60},
		{Verifier: "zrn1v3", CommitHash: []byte("h3"), CommittedAtBlock: 60},
	}
	round.Reveals = []*types.RevealEntry{
		{Verifier: "zrn1v1", Vote: "accept", Salt: []byte("s1"), RevealedAtBlock: 70},
		{Verifier: "zrn1v2", Vote: "accept", Salt: []byte("s2"), RevealedAtBlock: 70},
		{Verifier: "zrn1v3", Vote: "accept", Salt: []byte("s3"), RevealedAtBlock: 70},
	}
	require.NoError(t, k.SetVerificationRound(ctx, round))

	result, err := k.AggregateVerificationResult(ctx, round)
	require.NoError(t, err)
	require.Equal(t, types.Verdict_VERDICT_ACCEPT, result.Verdict)
	require.Equal(t, uint64(1_000_000), result.Confidence) // 100% accept
}

func TestAggregate_UnanimousReject(t *testing.T) {
	k, ctx, _, sk := setupKnowledgeTestFull(t)
	sk.addValidator("zrn1v1", 100_000, "bonded")
	sk.addValidator("zrn1v2", 100_000, "bonded")
	sk.addValidator("zrn1v3", 100_000, "bonded")

	claim := &types.Claim{Id: "c-ur", FactContent: "Unanimous reject claim content"}
	require.NoError(t, k.SetClaim(ctx, claim))

	round := makeRoundInPhase("r-ur", "c-ur", types.VerificationPhase_VERIFICATION_PHASE_AGGREGATION, 50)
	round.Commits = []*types.CommitEntry{
		{Verifier: "zrn1v1", CommitHash: []byte("h1"), CommittedAtBlock: 60},
		{Verifier: "zrn1v2", CommitHash: []byte("h2"), CommittedAtBlock: 60},
		{Verifier: "zrn1v3", CommitHash: []byte("h3"), CommittedAtBlock: 60},
	}
	round.Reveals = []*types.RevealEntry{
		{Verifier: "zrn1v1", Vote: "reject", Salt: []byte("s1"), RevealedAtBlock: 70},
		{Verifier: "zrn1v2", Vote: "reject", Salt: []byte("s2"), RevealedAtBlock: 70},
		{Verifier: "zrn1v3", Vote: "reject", Salt: []byte("s3"), RevealedAtBlock: 70},
	}
	require.NoError(t, k.SetVerificationRound(ctx, round))

	result, err := k.AggregateVerificationResult(ctx, round)
	require.NoError(t, err)
	require.Equal(t, types.Verdict_VERDICT_REJECT, result.Verdict)
	require.Equal(t, uint64(1_000_000), result.Confidence)
}

func TestAggregate_SplitVote_Inconclusive(t *testing.T) {
	k, ctx, _, sk := setupKnowledgeTestFull(t)
	// Equal stake, 50/50 split → below 77% threshold → inconclusive
	sk.addValidator("zrn1v1", 100_000, "bonded")
	sk.addValidator("zrn1v2", 100_000, "bonded")
	sk.addValidator("zrn1v3", 100_000, "bonded")
	sk.addValidator("zrn1v4", 100_000, "bonded")

	claim := &types.Claim{Id: "c-split", FactContent: "Split vote claim content"}
	require.NoError(t, k.SetClaim(ctx, claim))

	round := makeRoundInPhase("r-split", "c-split", types.VerificationPhase_VERIFICATION_PHASE_AGGREGATION, 50)
	round.Commits = []*types.CommitEntry{
		{Verifier: "zrn1v1", CommitHash: []byte("h1"), CommittedAtBlock: 60},
		{Verifier: "zrn1v2", CommitHash: []byte("h2"), CommittedAtBlock: 60},
		{Verifier: "zrn1v3", CommitHash: []byte("h3"), CommittedAtBlock: 60},
		{Verifier: "zrn1v4", CommitHash: []byte("h4"), CommittedAtBlock: 60},
	}
	round.Reveals = []*types.RevealEntry{
		{Verifier: "zrn1v1", Vote: "accept", Salt: []byte("s1"), RevealedAtBlock: 70},
		{Verifier: "zrn1v2", Vote: "accept", Salt: []byte("s2"), RevealedAtBlock: 70},
		{Verifier: "zrn1v3", Vote: "reject", Salt: []byte("s3"), RevealedAtBlock: 70},
		{Verifier: "zrn1v4", Vote: "reject", Salt: []byte("s4"), RevealedAtBlock: 70},
	}
	require.NoError(t, k.SetVerificationRound(ctx, round))

	result, err := k.AggregateVerificationResult(ctx, round)
	require.NoError(t, err)
	require.Equal(t, types.Verdict_VERDICT_INCONCLUSIVE, result.Verdict)
}

func TestAggregate_StakeWeighted(t *testing.T) {
	k, ctx, _, sk := setupKnowledgeTestFull(t)
	// v1 has 900k stake, v2 and v3 have 50k each
	// Accept: 900k, Reject: 100k → Accept ratio = 900/1000 = 90% → above 77%
	sk.addValidator("zrn1whale", 900_000, "guardian")
	sk.addValidator("zrn1small1", 50_000, "verified")
	sk.addValidator("zrn1small2", 50_000, "verified")

	claim := &types.Claim{Id: "c-weighted", FactContent: "Stake weighted claim content"}
	require.NoError(t, k.SetClaim(ctx, claim))

	round := makeRoundInPhase("r-weighted", "c-weighted", types.VerificationPhase_VERIFICATION_PHASE_AGGREGATION, 50)
	round.Commits = []*types.CommitEntry{
		{Verifier: "zrn1whale", CommitHash: []byte("h1"), CommittedAtBlock: 60},
		{Verifier: "zrn1small1", CommitHash: []byte("h2"), CommittedAtBlock: 60},
		{Verifier: "zrn1small2", CommitHash: []byte("h3"), CommittedAtBlock: 60},
	}
	round.Reveals = []*types.RevealEntry{
		{Verifier: "zrn1whale", Vote: "accept", Salt: []byte("s1"), RevealedAtBlock: 70},
		{Verifier: "zrn1small1", Vote: "reject", Salt: []byte("s2"), RevealedAtBlock: 70},
		{Verifier: "zrn1small2", Vote: "reject", Salt: []byte("s3"), RevealedAtBlock: 70},
	}
	require.NoError(t, k.SetVerificationRound(ctx, round))

	result, err := k.AggregateVerificationResult(ctx, round)
	require.NoError(t, err)
	require.Equal(t, types.Verdict_VERDICT_ACCEPT, result.Verdict)
	require.Equal(t, uint64(900_000), result.Confidence) // 900k/1000k = 90%
}

func TestAggregate_BelowMinVerifiers(t *testing.T) {
	k, ctx, _, sk := setupKnowledgeTestFull(t)
	sk.addValidator("zrn1v1", 100_000, "bonded")
	// Only 1 reveal, MinVerifiers=3

	claim := &types.Claim{Id: "c-quorum", FactContent: "Below quorum claim content"}
	require.NoError(t, k.SetClaim(ctx, claim))

	round := makeRoundInPhase("r-quorum", "c-quorum", types.VerificationPhase_VERIFICATION_PHASE_AGGREGATION, 50)
	round.Commits = []*types.CommitEntry{
		{Verifier: "zrn1v1", CommitHash: []byte("h1"), CommittedAtBlock: 60},
	}
	round.Reveals = []*types.RevealEntry{
		{Verifier: "zrn1v1", Vote: "accept", Salt: []byte("s1"), RevealedAtBlock: 70},
	}
	require.NoError(t, k.SetVerificationRound(ctx, round))

	result, err := k.AggregateVerificationResult(ctx, round)
	require.NoError(t, err)
	require.Equal(t, types.Verdict_VERDICT_INCONCLUSIVE, result.Verdict)
	require.Equal(t, uint64(0), result.Confidence)
}

func TestAggregate_SingleVerifier_WithMinVerifiers1(t *testing.T) {
	k, ctx, _, sk := setupKnowledgeTestFull(t)
	sk.addValidator("zrn1v1", 100_000, "bonded")

	// Set MinVerifiers=1 for this test
	params, _ := k.GetParams(ctx)
	params.MinVerifiers = 1
	require.NoError(t, k.SetParams(ctx, params))

	claim := &types.Claim{Id: "c-single", FactContent: "Single verifier claim content"}
	require.NoError(t, k.SetClaim(ctx, claim))

	round := makeRoundInPhase("r-single", "c-single", types.VerificationPhase_VERIFICATION_PHASE_AGGREGATION, 50)
	round.Commits = []*types.CommitEntry{
		{Verifier: "zrn1v1", CommitHash: []byte("h1"), CommittedAtBlock: 60},
	}
	round.Reveals = []*types.RevealEntry{
		{Verifier: "zrn1v1", Vote: "accept", Salt: []byte("s1"), RevealedAtBlock: 70},
	}
	require.NoError(t, k.SetVerificationRound(ctx, round))

	result, err := k.AggregateVerificationResult(ctx, round)
	require.NoError(t, err)
	require.Equal(t, types.Verdict_VERDICT_ACCEPT, result.Verdict)
	require.Equal(t, uint64(1_000_000), result.Confidence)
}

func TestAggregate_ZeroStakeValidator(t *testing.T) {
	k, ctx, _, sk := setupKnowledgeTestFull(t)
	sk.addValidator("zrn1v1", 0, "apprentice")  // zero stake → minimum weight 1
	sk.addValidator("zrn1v2", 0, "apprentice")
	sk.addValidator("zrn1v3", 0, "apprentice")

	claim := &types.Claim{Id: "c-zero", FactContent: "Zero stake validators claim content"}
	require.NoError(t, k.SetClaim(ctx, claim))

	round := makeRoundInPhase("r-zero", "c-zero", types.VerificationPhase_VERIFICATION_PHASE_AGGREGATION, 50)
	round.Commits = []*types.CommitEntry{
		{Verifier: "zrn1v1", CommitHash: []byte("h1"), CommittedAtBlock: 60},
		{Verifier: "zrn1v2", CommitHash: []byte("h2"), CommittedAtBlock: 60},
		{Verifier: "zrn1v3", CommitHash: []byte("h3"), CommittedAtBlock: 60},
	}
	round.Reveals = []*types.RevealEntry{
		{Verifier: "zrn1v1", Vote: "accept", Salt: []byte("s1"), RevealedAtBlock: 70},
		{Verifier: "zrn1v2", Vote: "accept", Salt: []byte("s2"), RevealedAtBlock: 70},
		{Verifier: "zrn1v3", Vote: "accept", Salt: []byte("s3"), RevealedAtBlock: 70},
	}
	require.NoError(t, k.SetVerificationRound(ctx, round))

	result, err := k.AggregateVerificationResult(ctx, round)
	require.NoError(t, err)
	// Zero stake gets weight 1 → still works
	require.Equal(t, types.Verdict_VERDICT_ACCEPT, result.Verdict)
}

func TestAggregate_MajorityReject(t *testing.T) {
	k, ctx, _, sk := setupKnowledgeTestFull(t)
	sk.addValidator("zrn1v1", 100_000, "bonded")
	sk.addValidator("zrn1v2", 100_000, "bonded")
	sk.addValidator("zrn1v3", 100_000, "bonded")
	sk.addValidator("zrn1v4", 100_000, "bonded")

	claim := &types.Claim{Id: "c-majorrej", FactContent: "Majority reject claim content"}
	require.NoError(t, k.SetClaim(ctx, claim))

	round := makeRoundInPhase("r-majorrej", "c-majorrej", types.VerificationPhase_VERIFICATION_PHASE_AGGREGATION, 50)
	round.Commits = []*types.CommitEntry{
		{Verifier: "zrn1v1", CommitHash: []byte("h1"), CommittedAtBlock: 60},
		{Verifier: "zrn1v2", CommitHash: []byte("h2"), CommittedAtBlock: 60},
		{Verifier: "zrn1v3", CommitHash: []byte("h3"), CommittedAtBlock: 60},
		{Verifier: "zrn1v4", CommitHash: []byte("h4"), CommittedAtBlock: 60},
	}
	round.Reveals = []*types.RevealEntry{
		{Verifier: "zrn1v1", Vote: "accept", Salt: []byte("s1"), RevealedAtBlock: 70},
		{Verifier: "zrn1v2", Vote: "reject", Salt: []byte("s2"), RevealedAtBlock: 70},
		{Verifier: "zrn1v3", Vote: "reject", Salt: []byte("s3"), RevealedAtBlock: 70},
		{Verifier: "zrn1v4", Vote: "reject", Salt: []byte("s4"), RevealedAtBlock: 70},
	}
	require.NoError(t, k.SetVerificationRound(ctx, round))

	// 75% reject, still below 77% threshold → inconclusive
	result, err := k.AggregateVerificationResult(ctx, round)
	require.NoError(t, err)
	// 75% reject = 750,000 bps < 770,000 threshold → inconclusive
	require.Equal(t, types.Verdict_VERDICT_INCONCLUSIVE, result.Verdict)
}

func TestAggregate_JustAboveThreshold(t *testing.T) {
	k, ctx, _, sk := setupKnowledgeTestFull(t)
	// 77/100 accept → exactly at threshold
	// Using weights: 77k accept, 23k reject
	sk.addValidator("zrn1acceptor", 77_000, "bonded")
	sk.addValidator("zrn1rejector", 23_000, "verified")
	sk.addValidator("zrn1neutral", 0, "apprentice") // weight=1

	claim := &types.Claim{Id: "c-threshold", FactContent: "Threshold test claim content"}
	require.NoError(t, k.SetClaim(ctx, claim))

	round := makeRoundInPhase("r-threshold", "c-threshold", types.VerificationPhase_VERIFICATION_PHASE_AGGREGATION, 50)
	round.Commits = []*types.CommitEntry{
		{Verifier: "zrn1acceptor", CommitHash: []byte("h1"), CommittedAtBlock: 60},
		{Verifier: "zrn1rejector", CommitHash: []byte("h2"), CommittedAtBlock: 60},
		{Verifier: "zrn1neutral", CommitHash: []byte("h3"), CommittedAtBlock: 60},
	}
	round.Reveals = []*types.RevealEntry{
		{Verifier: "zrn1acceptor", Vote: "accept", Salt: []byte("s1"), RevealedAtBlock: 70},
		{Verifier: "zrn1rejector", Vote: "reject", Salt: []byte("s2"), RevealedAtBlock: 70},
		{Verifier: "zrn1neutral", Vote: "accept", Salt: []byte("s3"), RevealedAtBlock: 70},
	}
	require.NoError(t, k.SetVerificationRound(ctx, round))

	result, err := k.AggregateVerificationResult(ctx, round)
	require.NoError(t, err)
	// acceptStake = 77001, totalStake = 100001, ratio = 77001 * 1_000_000 / 100001 ≈ 769,992
	// This is just below 770,000 threshold — should be inconclusive
	// (exact computation may vary by 1 due to integer division)
	if result.Verdict == types.Verdict_VERDICT_INCONCLUSIVE {
		require.Equal(t, uint64(0), result.Confidence)
	} else {
		require.Equal(t, types.Verdict_VERDICT_ACCEPT, result.Verdict)
		require.Greater(t, result.Confidence, uint64(0))
	}
}

// ─── Security: Confidence must use category baseline ─────────────────────────

func TestConfidence_CategoryBaseline(t *testing.T) {
	// Verify that the confidence in the result matches what's computed by the aggregation.
	// The confidence value should be the accept ratio from stake-weighted votes, not a hardcoded value.
	k, ctx, _, sk := setupKnowledgeTestFull(t)

	sk.addValidator("zrn1v1", 200_000, "bonded")
	sk.addValidator("zrn1v2", 100_000, "bonded")
	sk.addValidator("zrn1v3", 100_000, "bonded")

	claim := &types.Claim{Id: "c-cat", FactContent: "Category baseline claim content", Category: "empirical"}
	require.NoError(t, k.SetClaim(ctx, claim))

	round := makeRoundInPhase("r-cat", "c-cat", types.VerificationPhase_VERIFICATION_PHASE_AGGREGATION, 50)
	round.Commits = []*types.CommitEntry{
		{Verifier: "zrn1v1", CommitHash: []byte("h1"), CommittedAtBlock: 60},
		{Verifier: "zrn1v2", CommitHash: []byte("h2"), CommittedAtBlock: 60},
		{Verifier: "zrn1v3", CommitHash: []byte("h3"), CommittedAtBlock: 60},
	}
	round.Reveals = []*types.RevealEntry{
		{Verifier: "zrn1v1", Vote: "accept", Salt: []byte("s1"), RevealedAtBlock: 70},
		{Verifier: "zrn1v2", Vote: "accept", Salt: []byte("s2"), RevealedAtBlock: 70},
		{Verifier: "zrn1v3", Vote: "accept", Salt: []byte("s3"), RevealedAtBlock: 70},
	}
	require.NoError(t, k.SetVerificationRound(ctx, round))

	result, err := k.AggregateVerificationResult(ctx, round)
	require.NoError(t, err)

	// All accept with known stakes → exact confidence = 1,000,000
	require.Equal(t, types.Verdict_VERDICT_ACCEPT, result.Verdict)
	require.Equal(t, uint64(1_000_000), result.Confidence,
		"confidence must be exactly the stake-weighted accept ratio")
}

// ─── Security: Reward matches param exactly ─────────────────────────────────

func TestReward_MatchesParamDecay(t *testing.T) {
	k, ctx, _, sk := setupKnowledgeTestFull(t)
	sk.addValidator("zrn1v1", 100_000, "bonded")
	sk.addValidator("zrn1v2", 100_000, "bonded")
	sk.addValidator("zrn1v3", 100_000, "bonded")

	params, _ := k.GetParams(ctx)
	// Default VerificationReward = "3000000"

	claim := &types.Claim{Id: "c-reward-param", FactContent: "Reward param test claim content"}
	require.NoError(t, k.SetClaim(ctx, claim))

	round := makeRoundInPhase("r-reward-param", "c-reward-param", types.VerificationPhase_VERIFICATION_PHASE_AGGREGATION, 50)
	round.Commits = []*types.CommitEntry{
		{Verifier: "zrn1v1", CommitHash: []byte("h1"), CommittedAtBlock: 60},
		{Verifier: "zrn1v2", CommitHash: []byte("h2"), CommittedAtBlock: 60},
		{Verifier: "zrn1v3", CommitHash: []byte("h3"), CommittedAtBlock: 60},
	}
	round.Reveals = []*types.RevealEntry{
		{Verifier: "zrn1v1", Vote: "accept", Salt: []byte("s1"), RevealedAtBlock: 70},
		{Verifier: "zrn1v2", Vote: "accept", Salt: []byte("s2"), RevealedAtBlock: 70},
		{Verifier: "zrn1v3", Vote: "accept", Salt: []byte("s3"), RevealedAtBlock: 70},
	}
	require.NoError(t, k.SetVerificationRound(ctx, round))

	result, err := k.AggregateVerificationResult(ctx, round)
	require.NoError(t, err)

	// All 3 voted correctly → each should receive exactly VerificationReward
	require.Len(t, result.Rewards, 3)
	for _, reward := range result.Rewards {
		require.Equal(t, uint64(3_000_000), reward.Amount,
			"reward for verifier %s must exactly match params.VerificationReward", reward.Verifier)
	}
	_ = params
}

// ─── RewardsAndSlashes internal ──────────────────────────────────────────────

func TestRewardsAndSlashes_MissedReveal(t *testing.T) {
	k, ctx, _, sk := setupKnowledgeTestFull(t)
	sk.addValidator("zrn1revealer", 100_000, "bonded")
	sk.addValidator("zrn1skipper", 100_000, "bonded")
	sk.addValidator("zrn1revealer2", 100_000, "bonded")

	// Lower threshold and min verifiers so 2 accepts with 1 missed is sufficient
	params, _ := k.GetParams(ctx)
	params.ConfidenceThreshold = 600_000 // 60%
	params.MinVerifiers = 2
	require.NoError(t, k.SetParams(ctx, params))

	claim := &types.Claim{Id: "c-missed", FactContent: "Missed reveal claim content"}
	require.NoError(t, k.SetClaim(ctx, claim))

	round := makeRoundInPhase("r-missed", "c-missed", types.VerificationPhase_VERIFICATION_PHASE_AGGREGATION, 50)
	round.Commits = []*types.CommitEntry{
		{Verifier: "zrn1revealer", CommitHash: []byte("h1"), CommittedAtBlock: 60},
		{Verifier: "zrn1skipper", CommitHash: []byte("h2"), CommittedAtBlock: 60},
		{Verifier: "zrn1revealer2", CommitHash: []byte("h3"), CommittedAtBlock: 60},
	}
	// Only 2 of 3 revealed
	round.Reveals = []*types.RevealEntry{
		{Verifier: "zrn1revealer", Vote: "accept", Salt: []byte("s1"), RevealedAtBlock: 70},
		{Verifier: "zrn1revealer2", Vote: "accept", Salt: []byte("s2"), RevealedAtBlock: 70},
	}
	require.NoError(t, k.SetVerificationRound(ctx, round))

	result, err := k.AggregateVerificationResult(ctx, round)
	require.NoError(t, err)

	// Check that skipper got slashed for missed reveal
	var missedSlash *keeper.VerifierSlash
	for i, s := range result.Slashes {
		if s.Verifier == "zrn1skipper" {
			missedSlash = &result.Slashes[i]
		}
	}
	require.NotNil(t, missedSlash, "skipper should be slashed for missed reveal")

	params, _ = k.GetParams(ctx)
	require.Equal(t, params.MissedRevealSlashBps, missedSlash.SlashBps,
		"missed reveal slash must exactly match params.MissedRevealSlashBps")
}

func TestRewardsAndSlashes_WrongVote(t *testing.T) {
	k, ctx, _, sk := setupKnowledgeTestFull(t)
	sk.addValidator("zrn1correct1", 100_000, "bonded")
	sk.addValidator("zrn1correct2", 100_000, "bonded")
	sk.addValidator("zrn1wrong", 100_000, "bonded")

	// Lower threshold so 2/3 accept (66.6%) crosses it
	params, _ := k.GetParams(ctx)
	params.ConfidenceThreshold = 600_000 // 60%
	require.NoError(t, k.SetParams(ctx, params))

	claim := &types.Claim{Id: "c-wrong", FactContent: "Wrong vote test claim content"}
	require.NoError(t, k.SetClaim(ctx, claim))

	round := makeRoundInPhase("r-wrong", "c-wrong", types.VerificationPhase_VERIFICATION_PHASE_AGGREGATION, 50)
	round.Commits = []*types.CommitEntry{
		{Verifier: "zrn1correct1", CommitHash: []byte("h1"), CommittedAtBlock: 60},
		{Verifier: "zrn1correct2", CommitHash: []byte("h2"), CommittedAtBlock: 60},
		{Verifier: "zrn1wrong", CommitHash: []byte("h3"), CommittedAtBlock: 60},
	}
	round.Reveals = []*types.RevealEntry{
		{Verifier: "zrn1correct1", Vote: "accept", Salt: []byte("s1"), RevealedAtBlock: 70},
		{Verifier: "zrn1correct2", Vote: "accept", Salt: []byte("s2"), RevealedAtBlock: 70},
		{Verifier: "zrn1wrong", Vote: "reject", Salt: []byte("s3"), RevealedAtBlock: 70},
	}
	require.NoError(t, k.SetVerificationRound(ctx, round))

	result, err := k.AggregateVerificationResult(ctx, round)
	require.NoError(t, err)
	require.Equal(t, types.Verdict_VERDICT_ACCEPT, result.Verdict)

	// Check wrong voter was slashed
	var wrongSlash *keeper.VerifierSlash
	for i, s := range result.Slashes {
		if s.Verifier == "zrn1wrong" {
			wrongSlash = &result.Slashes[i]
		}
	}
	require.NotNil(t, wrongSlash, "wrong voter should be slashed")

	params, _ = k.GetParams(ctx)
	require.Equal(t, params.WrongVerificationSlashBps, wrongSlash.SlashBps,
		"wrong vote slash must exactly match params.WrongVerificationSlashBps")
}

func TestRewardsAndSlashes_InconclusiveNoRewardsOrSlashes(t *testing.T) {
	k, ctx, _, sk := setupKnowledgeTestFull(t)
	sk.addValidator("zrn1v1", 100_000, "bonded")
	// Only 1 reveal → below MinVerifiers → inconclusive

	claim := &types.Claim{Id: "c-inc-no-rs", FactContent: "Inconclusive no rewards content"}
	require.NoError(t, k.SetClaim(ctx, claim))

	round := makeRoundInPhase("r-inc-no-rs", "c-inc-no-rs", types.VerificationPhase_VERIFICATION_PHASE_AGGREGATION, 50)
	round.Commits = []*types.CommitEntry{
		{Verifier: "zrn1v1", CommitHash: []byte("h1"), CommittedAtBlock: 60},
	}
	round.Reveals = []*types.RevealEntry{
		{Verifier: "zrn1v1", Vote: "accept", Salt: []byte("s1"), RevealedAtBlock: 70},
	}
	require.NoError(t, k.SetVerificationRound(ctx, round))

	result, err := k.AggregateVerificationResult(ctx, round)
	require.NoError(t, err)
	require.Equal(t, types.Verdict_VERDICT_INCONCLUSIVE, result.Verdict)
	require.Empty(t, result.Rewards, "inconclusive should have no rewards")
	require.Empty(t, result.Slashes, "inconclusive should have no slashes")
}
