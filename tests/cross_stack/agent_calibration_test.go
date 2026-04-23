package cross_stack_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	knowledgekeeper "github.com/zerone-chain/zerone/x/knowledge/keeper"
	knowledgetypes "github.com/zerone-chain/zerone/x/knowledge/types"
)

// TestAgentCalibration_ScoreFormula pins the score semantics for unit-sized
// cases — no round-simulation, just direct calibration struct evaluation.
func TestAgentCalibration_ScoreFormula(t *testing.T) {
	cases := []struct {
		name         string
		c            *knowledgetypes.AgentCalibration
		wantAtLeast  uint64
		wantAtMost   uint64
		description  string
	}{
		{
			name:        "zero-submissions → zero",
			c:           &knowledgetypes.AgentCalibration{TotalSubmissions: 0},
			wantAtLeast: 0,
			wantAtMost:  0,
			description: "no track record → score 0",
		},
		{
			name: "all-accepted-no-corr → acceptance rate only",
			c: &knowledgetypes.AgentCalibration{
				TotalSubmissions: 10, Accepted: 10,
			},
			wantAtLeast: 1_000_000,
			wantAtMost:  1_000_000,
			description: "10/10 accepted, no bonuses or penalties → BPS",
		},
		{
			name: "half-accepted → half score",
			c: &knowledgetypes.AgentCalibration{
				TotalSubmissions: 10, Accepted: 5,
			},
			wantAtLeast: 500_000,
			wantAtMost:  500_000,
			description: "5/10 accepted → 500_000 BPS",
		},
		{
			name: "accepted-with-corroborations → boosted",
			c: &knowledgetypes.AgentCalibration{
				TotalSubmissions: 10, Accepted: 10, CorroborationsEarned: 20,
			},
			wantAtLeast: 1_000_000,
			wantAtMost:  1_000_000,
			description: "10/10 + 20 corroborations — already at cap, stays there",
		},
		{
			name: "accepted-then-disproven → penalized",
			c: &knowledgetypes.AgentCalibration{
				TotalSubmissions: 10, Accepted: 10, DisprovenCount: 5,
			},
			wantAtLeast: 500_000,
			wantAtMost:  500_000,
			description: "10/10 accepted but 5 later disproven → 500_000 BPS",
		},
		{
			name: "mostly-rejected → low score",
			c: &knowledgetypes.AgentCalibration{
				TotalSubmissions: 10, Accepted: 2,
			},
			wantAtLeast: 200_000,
			wantAtMost:  200_000,
			description: "2/10 accepted → 200_000 BPS",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := knowledgekeeper.ComputeAgentCalibrationScore(c.c)
			require.GreaterOrEqual(t, got, c.wantAtLeast, c.description)
			require.LessOrEqual(t, got, c.wantAtMost, c.description)
		})
	}
}

// TestAgentCalibration_FeedbackLoop drives the complete loop: two submitters
// issue claims under the same harness; round outcomes update their records;
// the leaderboard reflects the resulting calibration scores.
func TestAgentCalibration_FeedbackLoop(t *testing.T) {
	h := NewTestHarness(t)
	require.NoError(t, h.KnowledgeKeeper.SeedDefaultMethodologies(h.Ctx))

	domain := "calibration_loop_domain"
	require.NoError(t, h.KnowledgeKeeper.SetDomain(h.Ctx, &knowledgetypes.Domain{
		Name:   domain,
		Status: knowledgetypes.DomainStatus_DOMAIN_STATUS_ACTIVE,
	}))

	agentA := "zerone1calibagent000000000000000000aaa"
	agentB := "zerone1calibagent000000000000000000bbb"

	// Helper to run one claim → round → verdict cycle.
	submit := func(id, submitter string, verdict knowledgetypes.Verdict) {
		claim := &knowledgetypes.Claim{
			Id:          id,
			Submitter:   submitter,
			FactContent: "claim " + id,
			Domain:      domain,
			Category:    "empirical",
			Status:      knowledgetypes.ClaimStatus_CLAIM_STATUS_IN_VERIFICATION,
			Stake:       "1000000",
			MethodId:    knowledgetypes.MethodologyEmpirical,
		}
		require.NoError(t, h.KnowledgeKeeper.SetClaim(h.Ctx, claim))
		round := &knowledgetypes.VerificationRound{
			Id:             "round-" + id,
			ClaimId:        claim.Id,
			Phase:          knowledgetypes.VerificationPhase_VERIFICATION_PHASE_COMPLETE,
			StartedAtBlock: 1,
		}
		var result *knowledgekeeper.VerificationResult
		switch verdict {
		case knowledgetypes.Verdict_VERDICT_ACCEPT:
			result = &knowledgekeeper.VerificationResult{
				Verdict: verdict, Confidence: 900_000, AcceptCount: 3,
			}
		default:
			result = &knowledgekeeper.VerificationResult{
				Verdict: verdict, Confidence: 700_000, RejectCount: 3,
			}
		}
		require.NoError(t, h.KnowledgeKeeper.CompleteRound(h.Ctx, round, result))
	}

	// Agent A: 3 submissions, all accepted — should have a 1M score.
	submit("a1", agentA, knowledgetypes.Verdict_VERDICT_ACCEPT)
	submit("a2", agentA, knowledgetypes.Verdict_VERDICT_ACCEPT)
	submit("a3", agentA, knowledgetypes.Verdict_VERDICT_ACCEPT)

	// Agent B: 3 submissions, 1 accepted, 2 rejected — should have a low score.
	submit("b1", agentB, knowledgetypes.Verdict_VERDICT_ACCEPT)
	submit("b2", agentB, knowledgetypes.Verdict_VERDICT_REJECT)
	submit("b3", agentB, knowledgetypes.Verdict_VERDICT_REJECT)

	qs := knowledgekeeper.NewQueryServerImpl(h.KnowledgeKeeper)

	// Agent A profile
	calA, err := qs.AgentCalibration(h.Ctx, &knowledgetypes.QueryAgentCalibrationRequest{Address: agentA})
	require.NoError(t, err)
	require.True(t, calA.Found)
	require.Equal(t, uint64(3), calA.Calibration.TotalSubmissions)
	require.Equal(t, uint64(3), calA.Calibration.Accepted)
	require.Equal(t, uint64(1_000_000), calA.Calibration.CalibrationScoreBps)

	// Agent B profile
	calB, err := qs.AgentCalibration(h.Ctx, &knowledgetypes.QueryAgentCalibrationRequest{Address: agentB})
	require.NoError(t, err)
	require.True(t, calB.Found)
	require.Equal(t, uint64(3), calB.Calibration.TotalSubmissions)
	require.Equal(t, uint64(1), calB.Calibration.Accepted)
	require.Equal(t, uint64(2), calB.Calibration.Rejected)
	// 1/3 acceptance = 333_333 BPS
	require.Equal(t, uint64(333_333), calB.Calibration.CalibrationScoreBps)

	// Per-method slot populated.
	perMethodA, ok := calA.Calibration.PerMethod[knowledgetypes.MethodologyEmpirical]
	require.True(t, ok)
	require.Equal(t, uint64(3), perMethodA.Accepted)

	// Leaderboard — A must outrank B.
	lb, err := qs.AgentLeaderboard(h.Ctx, &knowledgetypes.QueryAgentLeaderboardRequest{
		MinSubmissions: 3,
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(lb.Entries), 2)
	require.Equal(t, agentA, lb.Entries[0].Address,
		"agent A (3/3) must rank above agent B (1/3)")
	require.Greater(t, lb.Entries[0].CalibrationScoreBps, lb.Entries[1].CalibrationScoreBps)
	require.Greater(t, lb.SnapshotBlockHeight, uint64(0),
		"leaderboard must pin a block height for reproducibility")

	// Per-method leaderboard for empirical — same ranking, filtered.
	lbEmpirical, err := qs.AgentLeaderboard(h.Ctx, &knowledgetypes.QueryAgentLeaderboardRequest{
		MethodId:       knowledgetypes.MethodologyEmpirical,
		MinSubmissions: 3,
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(lbEmpirical.Entries), 2)
	require.Equal(t, agentA, lbEmpirical.Entries[0].Address)
}

// TestAgentCalibration_DisprovalPenalty validates that a fact going DISPROVEN
// post-acceptance penalizes the submitter's score. The key Popperian signal
// for the feedback loop: surviving scrutiny is different from initial acceptance.
func TestAgentCalibration_DisprovalPenalty(t *testing.T) {
	h := NewTestHarness(t)
	require.NoError(t, h.KnowledgeKeeper.SeedDefaultMethodologies(h.Ctx))

	domain := "disproval_penalty_domain"
	require.NoError(t, h.KnowledgeKeeper.SetDomain(h.Ctx, &knowledgetypes.Domain{
		Name:   domain,
		Status: knowledgetypes.DomainStatus_DOMAIN_STATUS_ACTIVE,
	}))

	submitter := "zerone1disprovedsubmitter00000000000aa"

	// One accepted submission — baseline score should be BPS.
	claim := &knowledgetypes.Claim{
		Id:          "disproval-target-claim",
		Submitter:   submitter,
		FactContent: "To be disproven.",
		Domain:      domain,
		Category:    "empirical",
		Status:      knowledgetypes.ClaimStatus_CLAIM_STATUS_IN_VERIFICATION,
		Stake:       "1000000",
		MethodId:    knowledgetypes.MethodologyEmpirical,
	}
	require.NoError(t, h.KnowledgeKeeper.SetClaim(h.Ctx, claim))
	round := &knowledgetypes.VerificationRound{
		Id: "round-disproval-baseline", ClaimId: claim.Id,
		Phase: knowledgetypes.VerificationPhase_VERIFICATION_PHASE_COMPLETE, StartedAtBlock: 1,
	}
	require.NoError(t, h.KnowledgeKeeper.CompleteRound(h.Ctx, round, &knowledgekeeper.VerificationResult{
		Verdict: knowledgetypes.Verdict_VERDICT_ACCEPT, Confidence: 900_000, AcceptCount: 3,
	}))

	qs := knowledgekeeper.NewQueryServerImpl(h.KnowledgeKeeper)
	before, err := qs.AgentCalibration(h.Ctx, &knowledgetypes.QueryAgentCalibrationRequest{Address: submitter})
	require.NoError(t, err)
	require.Equal(t, uint64(1_000_000), before.Calibration.CalibrationScoreBps,
		"1/1 accepted → BPS baseline")

	// Find the created fact and challenge it successfully.
	var targetFact *knowledgetypes.Fact
	h.KnowledgeKeeper.IterateFactsByDomain(h.Ctx, domain, func(factID string) bool {
		f, _ := h.KnowledgeKeeper.GetFact(h.Ctx, factID)
		if f != nil && f.ClaimId == claim.Id {
			targetFact = f
			return true
		}
		return false
	})
	require.NotNil(t, targetFact)

	// Challenge the fact with an ACCEPT verdict (challenge succeeds → DISPROVEN).
	challenger := "zerone1challenger00000000000000000000aa"
	challengeClaim := &knowledgetypes.Claim{
		Id:                "disproval-challenge",
		Submitter:         challenger,
		FactContent:       "contradicts " + targetFact.Id,
		Domain:            domain,
		Category:          "empirical",
		Status:            knowledgetypes.ClaimStatus_CLAIM_STATUS_IN_VERIFICATION,
		Stake:             "11000000",
		ProvisionalFactId: targetFact.Id,
		MethodId:          knowledgetypes.MethodologyDialectical,
		Relations: []*knowledgetypes.ClaimRelation{{
			TargetFactId: targetFact.Id,
			Relation:     knowledgetypes.RelationType_RELATION_TYPE_CONTRADICTS,
		}},
	}
	require.NoError(t, h.KnowledgeKeeper.SetClaim(h.Ctx, challengeClaim))
	cRound := &knowledgetypes.VerificationRound{
		Id: "round-disproval-challenge", ClaimId: challengeClaim.Id,
		Phase: knowledgetypes.VerificationPhase_VERIFICATION_PHASE_COMPLETE, StartedAtBlock: 2,
	}
	require.NoError(t, h.KnowledgeKeeper.CompleteRound(h.Ctx, cRound, &knowledgekeeper.VerificationResult{
		Verdict: knowledgetypes.Verdict_VERDICT_ACCEPT, Confidence: 900_000, AcceptCount: 3,
	}))

	// The submitter's score should now be penalized.
	after, err := qs.AgentCalibration(h.Ctx, &knowledgetypes.QueryAgentCalibrationRequest{Address: submitter})
	require.NoError(t, err)
	require.Equal(t, uint64(1), after.Calibration.DisprovenCount,
		"disproved fact credited to submitter's disproven_count")
	require.Less(t, after.Calibration.CalibrationScoreBps, before.Calibration.CalibrationScoreBps,
		"disproval must reduce calibration score below the pre-disproval baseline")

	// The challenger should accrue a successful challenge.
	chal, err := qs.AgentCalibration(h.Ctx, &knowledgetypes.QueryAgentCalibrationRequest{Address: challenger})
	require.NoError(t, err)
	require.Equal(t, uint64(1), chal.Calibration.ChallengesIssued)
	require.Equal(t, uint64(1), chal.Calibration.ChallengesSucceeded)

	_ = fmt.Sprintf // silence unused import
}
