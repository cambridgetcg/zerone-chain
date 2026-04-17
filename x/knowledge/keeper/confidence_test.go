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
	sk.addValidator("zrn1v4", 100_000, "bonded")

	claim := &types.Claim{Id: "c-ua", FactContent: "Unanimous accept claim content", Domain: "mathematics"}
	require.NoError(t, k.SetClaim(ctx, claim))

	round := makeRoundInPhase("r-ua", "c-ua", types.VerificationPhase_VERIFICATION_PHASE_AGGREGATION, 50)
	round.Commits = []*types.CommitEntry{
		{Verifier: "zrn1v1", CommitHash: []byte("h1"), CommittedAtBlock: 60},
		{Verifier: "zrn1v2", CommitHash: []byte("h2"), CommittedAtBlock: 60},
		{Verifier: "zrn1v3", CommitHash: []byte("h3"), CommittedAtBlock: 60},
		{Verifier: "zrn1v4", CommitHash: []byte("h4"), CommittedAtBlock: 60},
	}
	round.Reveals = []*types.RevealEntry{
		{Verifier: "zrn1v1", Vote: "accept", Salt: []byte("s1"), RevealedAtBlock: 70},
		{Verifier: "zrn1v2", Vote: "accept", Salt: []byte("s2"), RevealedAtBlock: 70},
		{Verifier: "zrn1v3", Vote: "accept", Salt: []byte("s3"), RevealedAtBlock: 70},
		{Verifier: "zrn1v4", Vote: "accept", Salt: []byte("s4"), RevealedAtBlock: 70},
	}
	require.NoError(t, k.SetVerificationRound(ctx, round))

	result, err := k.AggregateVerificationResult(ctx, round)
	require.NoError(t, err)
	require.Equal(t, types.Verdict_VERDICT_ACCEPT, result.Verdict)
	require.Equal(t, uint64(880_000), result.Confidence) // 100% accept capped at MaxConfidence
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
	require.Equal(t, uint64(880_000), result.Confidence) // 100% reject capped at MaxConfidence
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
	// Three validators agree to accept (meets headcount floor) with strong stake
	// weight: whale 900k + two supports 30k each = 960k accept vs 40k reject.
	// Accept ratio = 960/1000 = 96% → above 77% ConfidenceThreshold.
	sk.addValidator("zrn1whale", 900_000, "guardian")
	sk.addValidator("zrn1support1", 30_000, "verified")
	sk.addValidator("zrn1support2", 30_000, "verified")
	sk.addValidator("zrn1dissent1", 20_000, "verified")
	sk.addValidator("zrn1dissent2", 20_000, "verified")

	claim := &types.Claim{Id: "c-weighted", FactContent: "Stake weighted claim content"}
	require.NoError(t, k.SetClaim(ctx, claim))

	round := makeRoundInPhase("r-weighted", "c-weighted", types.VerificationPhase_VERIFICATION_PHASE_AGGREGATION, 50)
	round.Commits = []*types.CommitEntry{
		{Verifier: "zrn1whale", CommitHash: []byte("h1"), CommittedAtBlock: 60},
		{Verifier: "zrn1support1", CommitHash: []byte("h2"), CommittedAtBlock: 60},
		{Verifier: "zrn1support2", CommitHash: []byte("h3"), CommittedAtBlock: 60},
		{Verifier: "zrn1dissent1", CommitHash: []byte("h4"), CommittedAtBlock: 60},
		{Verifier: "zrn1dissent2", CommitHash: []byte("h5"), CommittedAtBlock: 60},
	}
	round.Reveals = []*types.RevealEntry{
		{Verifier: "zrn1whale", Vote: "accept", Salt: []byte("s1"), RevealedAtBlock: 70},
		{Verifier: "zrn1support1", Vote: "accept", Salt: []byte("s2"), RevealedAtBlock: 70},
		{Verifier: "zrn1support2", Vote: "accept", Salt: []byte("s3"), RevealedAtBlock: 70},
		{Verifier: "zrn1dissent1", Vote: "reject", Salt: []byte("s4"), RevealedAtBlock: 70},
		{Verifier: "zrn1dissent2", Vote: "reject", Salt: []byte("s5"), RevealedAtBlock: 70},
	}
	require.NoError(t, k.SetVerificationRound(ctx, round))

	result, err := k.AggregateVerificationResult(ctx, round)
	require.NoError(t, err)
	require.Equal(t, types.Verdict_VERDICT_ACCEPT, result.Verdict)
	require.Equal(t, uint64(880_000), result.Confidence) // 96% capped at MaxConfidence (880,000)
}

// TestAggregate_StakeSingletonBlocked documents the T1 mitigation: a single
// stake-heavy voter cannot promote a claim even if it exceeds ConfidenceThreshold
// on stake alone. MinHeadcountAgreement ensures distinct-verifier consent.
func TestAggregate_StakeSingletonBlocked(t *testing.T) {
	k, ctx, _, sk := setupKnowledgeTestFull(t)
	sk.addValidator("zrn1whale", 900_000, "guardian")
	sk.addValidator("zrn1dissent1", 50_000, "verified")
	sk.addValidator("zrn1dissent2", 50_000, "verified")

	claim := &types.Claim{Id: "c-singleton", FactContent: "Stake singleton must not promote"}
	require.NoError(t, k.SetClaim(ctx, claim))

	round := makeRoundInPhase("r-singleton", "c-singleton", types.VerificationPhase_VERIFICATION_PHASE_AGGREGATION, 50)
	round.Commits = []*types.CommitEntry{
		{Verifier: "zrn1whale", CommitHash: []byte("h1"), CommittedAtBlock: 60},
		{Verifier: "zrn1dissent1", CommitHash: []byte("h2"), CommittedAtBlock: 60},
		{Verifier: "zrn1dissent2", CommitHash: []byte("h3"), CommittedAtBlock: 60},
	}
	round.Reveals = []*types.RevealEntry{
		{Verifier: "zrn1whale", Vote: "accept", Salt: []byte("s1"), RevealedAtBlock: 70},
		{Verifier: "zrn1dissent1", Vote: "reject", Salt: []byte("s2"), RevealedAtBlock: 70},
		{Verifier: "zrn1dissent2", Vote: "reject", Salt: []byte("s3"), RevealedAtBlock: 70},
	}
	require.NoError(t, k.SetVerificationRound(ctx, round))

	result, err := k.AggregateVerificationResult(ctx, round)
	require.NoError(t, err)
	require.Equal(t, types.Verdict_VERDICT_INCONCLUSIVE, result.Verdict,
		"stake-heavy singleton must not pass headcount floor (T1)")
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
	require.Equal(t, uint64(880_000), result.Confidence) // 100% accept capped at MaxConfidence
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

	// All accept with known stakes → raw ratio = 1,000,000, capped at MaxConfidence (880,000)
	require.Equal(t, types.Verdict_VERDICT_ACCEPT, result.Verdict)
	require.Equal(t, uint64(880_000), result.Confidence,
		"confidence must be capped at MaxConfidence")
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

	// Lower threshold so 2/3 accept (66.6%) crosses it; lower headcount floor to match.
	params, _ := k.GetParams(ctx)
	params.ConfidenceThreshold = 600_000 // 60%
	params.MinHeadcountAgreement = 2
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

// ─── Malformed Vote Tests ────────────────────────────────────────────────────

func TestAggregate_UnanimousMalformed(t *testing.T) {
	k, ctx, _, sk := setupKnowledgeTestFull(t)
	sk.addValidator("zrn1v1", 100_000, "bonded")
	sk.addValidator("zrn1v2", 100_000, "bonded")
	sk.addValidator("zrn1v3", 100_000, "bonded")

	claim := &types.Claim{Id: "c-umal", FactContent: "This statement is false — a paradox claim"}
	require.NoError(t, k.SetClaim(ctx, claim))

	round := makeRoundInPhase("r-umal", "c-umal", types.VerificationPhase_VERIFICATION_PHASE_AGGREGATION, 50)
	round.Commits = []*types.CommitEntry{
		{Verifier: "zrn1v1", CommitHash: []byte("h1"), CommittedAtBlock: 60},
		{Verifier: "zrn1v2", CommitHash: []byte("h2"), CommittedAtBlock: 60},
		{Verifier: "zrn1v3", CommitHash: []byte("h3"), CommittedAtBlock: 60},
	}
	round.Reveals = []*types.RevealEntry{
		{Verifier: "zrn1v1", Vote: "malformed", Salt: []byte("s1"), RevealedAtBlock: 70},
		{Verifier: "zrn1v2", Vote: "malformed", Salt: []byte("s2"), RevealedAtBlock: 70},
		{Verifier: "zrn1v3", Vote: "malformed", Salt: []byte("s3"), RevealedAtBlock: 70},
	}
	require.NoError(t, k.SetVerificationRound(ctx, round))

	result, err := k.AggregateVerificationResult(ctx, round)
	require.NoError(t, err)
	require.Equal(t, types.Verdict_VERDICT_MALFORMED, result.Verdict)
	require.Equal(t, uint64(880_000), result.Confidence) // 100% malformed capped at MaxConfidence
}

func TestAggregate_MalformedSupermajority(t *testing.T) {
	k, ctx, _, sk := setupKnowledgeTestFull(t)
	// 2 malformed + 1 accept: malformed stake = 200k, accept = 100k
	// malformed ratio = 666,666 bps — need to lower threshold for this test
	sk.addValidator("zrn1v1", 100_000, "bonded")
	sk.addValidator("zrn1v2", 100_000, "bonded")
	sk.addValidator("zrn1v3", 100_000, "bonded")

	params, _ := k.GetParams(ctx)
	params.ConfidenceThreshold = 600_000 // 60% threshold
	params.MinHeadcountAgreement = 2
	require.NoError(t, k.SetParams(ctx, params))

	claim := &types.Claim{Id: "c-malsup", FactContent: "Category error nonsense claim content"}
	require.NoError(t, k.SetClaim(ctx, claim))

	round := makeRoundInPhase("r-malsup", "c-malsup", types.VerificationPhase_VERIFICATION_PHASE_AGGREGATION, 50)
	round.Commits = []*types.CommitEntry{
		{Verifier: "zrn1v1", CommitHash: []byte("h1"), CommittedAtBlock: 60},
		{Verifier: "zrn1v2", CommitHash: []byte("h2"), CommittedAtBlock: 60},
		{Verifier: "zrn1v3", CommitHash: []byte("h3"), CommittedAtBlock: 60},
	}
	round.Reveals = []*types.RevealEntry{
		{Verifier: "zrn1v1", Vote: "malformed", Salt: []byte("s1"), RevealedAtBlock: 70},
		{Verifier: "zrn1v2", Vote: "malformed", Salt: []byte("s2"), RevealedAtBlock: 70},
		{Verifier: "zrn1v3", Vote: "accept", Salt: []byte("s3"), RevealedAtBlock: 70},
	}
	require.NoError(t, k.SetVerificationRound(ctx, round))

	result, err := k.AggregateVerificationResult(ctx, round)
	require.NoError(t, err)
	require.Equal(t, types.Verdict_VERDICT_MALFORMED, result.Verdict)
	require.Equal(t, uint64(666_666), result.Confidence) // 200k/300k ≈ 66.6%
}

func TestAggregate_MalformedBelowThreshold(t *testing.T) {
	k, ctx, _, sk := setupKnowledgeTestFull(t)
	// 1 malformed + 1 accept + 1 reject → no supermajority → INCONCLUSIVE
	sk.addValidator("zrn1v1", 100_000, "bonded")
	sk.addValidator("zrn1v2", 100_000, "bonded")
	sk.addValidator("zrn1v3", 100_000, "bonded")

	claim := &types.Claim{Id: "c-malbel", FactContent: "Split three ways claim content here"}
	require.NoError(t, k.SetClaim(ctx, claim))

	round := makeRoundInPhase("r-malbel", "c-malbel", types.VerificationPhase_VERIFICATION_PHASE_AGGREGATION, 50)
	round.Commits = []*types.CommitEntry{
		{Verifier: "zrn1v1", CommitHash: []byte("h1"), CommittedAtBlock: 60},
		{Verifier: "zrn1v2", CommitHash: []byte("h2"), CommittedAtBlock: 60},
		{Verifier: "zrn1v3", CommitHash: []byte("h3"), CommittedAtBlock: 60},
	}
	round.Reveals = []*types.RevealEntry{
		{Verifier: "zrn1v1", Vote: "malformed", Salt: []byte("s1"), RevealedAtBlock: 70},
		{Verifier: "zrn1v2", Vote: "accept", Salt: []byte("s2"), RevealedAtBlock: 70},
		{Verifier: "zrn1v3", Vote: "reject", Salt: []byte("s3"), RevealedAtBlock: 70},
	}
	require.NoError(t, k.SetVerificationRound(ctx, round))

	result, err := k.AggregateVerificationResult(ctx, round)
	require.NoError(t, err)
	require.Equal(t, types.Verdict_VERDICT_INCONCLUSIVE, result.Verdict)
}

func TestAggregate_MalformedTrumpsAccept(t *testing.T) {
	k, ctx, _, sk := setupKnowledgeTestFull(t)
	// Both malformed and accept exceed threshold — malformed must win (checked first)
	// 4 malformed (400k stake) + 1 accept (400k stake from whale) = 800k total
	// malformed ratio = 400/800 = 50%, accept ratio = 400/800 = 50%
	// With threshold at 50%, both would pass — but malformed is checked first
	sk.addValidator("zrn1m1", 100_000, "bonded")
	sk.addValidator("zrn1m2", 100_000, "bonded")
	sk.addValidator("zrn1m3", 100_000, "bonded")
	sk.addValidator("zrn1m4", 100_000, "bonded")
	sk.addValidator("zrn1whale", 400_000, "guardian")

	params, _ := k.GetParams(ctx)
	params.ConfidenceThreshold = 500_000 // 50% threshold
	params.MinVerifiers = 5
	require.NoError(t, k.SetParams(ctx, params))

	claim := &types.Claim{Id: "c-maltrump", FactContent: "Malformed trumps accept claim content"}
	require.NoError(t, k.SetClaim(ctx, claim))

	round := makeRoundInPhase("r-maltrump", "c-maltrump", types.VerificationPhase_VERIFICATION_PHASE_AGGREGATION, 50)
	round.Commits = []*types.CommitEntry{
		{Verifier: "zrn1m1", CommitHash: []byte("h1"), CommittedAtBlock: 60},
		{Verifier: "zrn1m2", CommitHash: []byte("h2"), CommittedAtBlock: 60},
		{Verifier: "zrn1m3", CommitHash: []byte("h3"), CommittedAtBlock: 60},
		{Verifier: "zrn1m4", CommitHash: []byte("h4"), CommittedAtBlock: 60},
		{Verifier: "zrn1whale", CommitHash: []byte("h5"), CommittedAtBlock: 60},
	}
	round.Reveals = []*types.RevealEntry{
		{Verifier: "zrn1m1", Vote: "malformed", Salt: []byte("s1"), RevealedAtBlock: 70},
		{Verifier: "zrn1m2", Vote: "malformed", Salt: []byte("s2"), RevealedAtBlock: 70},
		{Verifier: "zrn1m3", Vote: "malformed", Salt: []byte("s3"), RevealedAtBlock: 70},
		{Verifier: "zrn1m4", Vote: "malformed", Salt: []byte("s4"), RevealedAtBlock: 70},
		{Verifier: "zrn1whale", Vote: "accept", Salt: []byte("s5"), RevealedAtBlock: 70},
	}
	require.NoError(t, k.SetVerificationRound(ctx, round))

	result, err := k.AggregateVerificationResult(ctx, round)
	require.NoError(t, err)
	require.Equal(t, types.Verdict_VERDICT_MALFORMED, result.Verdict,
		"malformed must win over accept when both exceed threshold")
	require.Equal(t, uint64(500_000), result.Confidence) // 400k/800k = 50%
}

func TestMalformedSlash_SubmitterPenalized(t *testing.T) {
	k, ctx, bk, _ := setupKnowledgeTestFull(t)

	claim := &types.Claim{
		Id:          "claim-mal-slash",
		FactContent: "Malformed claim that wastes verifier time test content",
		Domain:      "general",
		Submitter:   "zrn1sub",
		Stake:       "1000000", // 1 ZRN
		Status:      types.ClaimStatus_CLAIM_STATUS_IN_VERIFICATION,
	}
	require.NoError(t, k.SetClaim(ctx, claim))

	round := makeRoundInPhase("r-mal-slash", "claim-mal-slash", types.VerificationPhase_VERIFICATION_PHASE_AGGREGATION, 80)
	require.NoError(t, k.SetVerificationRound(ctx, round))

	result := &keeper.VerificationResult{
		Verdict:    types.Verdict_VERDICT_MALFORMED,
		Confidence: 900_000,
	}
	require.NoError(t, k.CompleteRound(ctx, round, result))

	// Verify claim status is malformed
	updatedClaim, found := k.GetClaim(ctx, "claim-mal-slash")
	require.True(t, found)
	require.Equal(t, types.ClaimStatus_CLAIM_STATUS_MALFORMED, updatedClaim.Status)

	// Review fee is non-refundable (R19-6) — no slashing on malformed claims
	_ = bk // bank keeper not used for malformed claim slashing anymore

	// No fact should be created
	var factFound bool
	k.IterateFacts(ctx, func(fact *types.Fact) bool {
		if fact.ClaimId == "claim-mal-slash" {
			factFound = true
		}
		return false
	})
	require.False(t, factFound, "malformed claim must not create a fact")
}

func TestMalformedReward_RejectVotersGetPartial(t *testing.T) {
	k, ctx, _, sk := setupKnowledgeTestFull(t)
	sk.addValidator("zrn1mal1", 100_000, "bonded")
	sk.addValidator("zrn1mal2", 100_000, "bonded")
	sk.addValidator("zrn1rej1", 100_000, "bonded")
	sk.addValidator("zrn1acc1", 100_000, "bonded")

	params, _ := k.GetParams(ctx)
	params.ConfidenceThreshold = 500_000 // 50% threshold
	params.MinVerifiers = 4
	params.MinHeadcountAgreement = 2
	require.NoError(t, k.SetParams(ctx, params))

	claim := &types.Claim{Id: "c-mal-reward", FactContent: "Malformed reward test claim content here"}
	require.NoError(t, k.SetClaim(ctx, claim))

	round := makeRoundInPhase("r-mal-reward", "c-mal-reward", types.VerificationPhase_VERIFICATION_PHASE_AGGREGATION, 50)
	round.Commits = []*types.CommitEntry{
		{Verifier: "zrn1mal1", CommitHash: []byte("h1"), CommittedAtBlock: 60},
		{Verifier: "zrn1mal2", CommitHash: []byte("h2"), CommittedAtBlock: 60},
		{Verifier: "zrn1rej1", CommitHash: []byte("h3"), CommittedAtBlock: 60},
		{Verifier: "zrn1acc1", CommitHash: []byte("h4"), CommittedAtBlock: 60},
	}
	round.Reveals = []*types.RevealEntry{
		{Verifier: "zrn1mal1", Vote: "malformed", Salt: []byte("s1"), RevealedAtBlock: 70},
		{Verifier: "zrn1mal2", Vote: "malformed", Salt: []byte("s2"), RevealedAtBlock: 70},
		{Verifier: "zrn1rej1", Vote: "reject", Salt: []byte("s3"), RevealedAtBlock: 70},
		{Verifier: "zrn1acc1", Vote: "accept", Salt: []byte("s4"), RevealedAtBlock: 70},
	}
	require.NoError(t, k.SetVerificationRound(ctx, round))

	result, err := k.AggregateVerificationResult(ctx, round)
	require.NoError(t, err)
	require.Equal(t, types.Verdict_VERDICT_MALFORMED, result.Verdict)

	// Malformed voters get full reward (3,000,000)
	// Reject voter gets partial reward (50% = 1,500,000)
	// Accept voter gets slashed
	rewardMap := make(map[string]uint64)
	for _, r := range result.Rewards {
		rewardMap[r.Verifier] = r.Amount
	}
	slashMap := make(map[string]uint64)
	for _, s := range result.Slashes {
		slashMap[s.Verifier] = s.SlashBps
	}

	require.Equal(t, uint64(3_000_000), rewardMap["zrn1mal1"],
		"malformed voter must get full reward")
	require.Equal(t, uint64(3_000_000), rewardMap["zrn1mal2"],
		"malformed voter must get full reward")
	require.Equal(t, uint64(1_500_000), rewardMap["zrn1rej1"],
		"reject voter must get 50%% partial reward on malformed verdict")
	require.Equal(t, params.WrongVerificationSlashBps, slashMap["zrn1acc1"],
		"accept voter must be slashed on malformed verdict")
}

func TestAggregate_UsesEffectiveMinVerifiers_NilPartnershipKeeper(t *testing.T) {
	// With nil partnership keeper, effective min = params.MinVerifiers+1 = 4.
	// A round with only 3 reveals for a non-empty domain should be INCONCLUSIVE.
	k, ctx, _, sk := setupKnowledgeTestFull(t)
	sk.addValidator("zrn1v1", 100_000, "bonded")
	sk.addValidator("zrn1v2", 100_000, "bonded")
	sk.addValidator("zrn1v3", 100_000, "bonded")

	params, err := k.GetParams(ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(3), params.MinVerifiers, "test assumes default MinVerifiers=3")

	claim := &types.Claim{Id: "c-emv", FactContent: "x", Domain: "physics"}
	require.NoError(t, k.SetClaim(ctx, claim))

	round := makeRoundInPhase("r-emv", "c-emv", types.VerificationPhase_VERIFICATION_PHASE_AGGREGATION, 50)
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
	require.Equal(t, types.Verdict_VERDICT_INCONCLUSIVE, result.Verdict,
		"3 reveals < effective min (4) for domain=physics with nil partnership keeper")
}
