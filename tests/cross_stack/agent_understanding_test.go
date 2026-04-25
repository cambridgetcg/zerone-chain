package cross_stack_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	knowledgekeeper "github.com/zerone-chain/zerone/x/knowledge/keeper"
	knowledgetypes "github.com/zerone-chain/zerone/x/knowledge/types"
)

// TestAgentUnderstanding_ProfileReflectsAuthoredFacts is the
// minimum viable integration test: an agent authors a fact in a
// domain; the agent_understanding profile must show that fact.
//
// This binds the synthesizer's read path against real chain state
// — if the knowledge → agent_understanding adapter breaks, this
// fails.
func TestAgentUnderstanding_ProfileReflectsAuthoredFacts(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	agent := testAddr("au_agent_alpha").String()

	// Fund and submit a claim, run it to verdict.
	claim := &knowledgetypes.Claim{
		Id:          "claim-au-test",
		Submitter:   agent,
		FactContent: "test fact for understanding profile",
		Domain:      "philosophy",
		Category:    "empirical",
		MethodId:    knowledgetypes.MethodologyEmpirical,
		Status:      knowledgetypes.ClaimStatus_CLAIM_STATUS_IN_VERIFICATION,
		Stake:       "1000000",
	}
	require.NoError(t, h.KnowledgeKeeper.SetClaim(h.Ctx, claim))
	round := &knowledgetypes.VerificationRound{
		Id: "round-au-test", ClaimId: claim.Id,
		Phase: knowledgetypes.VerificationPhase_VERIFICATION_PHASE_COMPLETE, StartedAtBlock: 1,
	}
	require.NoError(t, h.KnowledgeKeeper.CompleteRound(h.Ctx, round, &knowledgekeeper.VerificationResult{
		Verdict: knowledgetypes.Verdict_VERDICT_ACCEPT, Confidence: 900_000, AcceptCount: 3,
	}))

	// Compose the profile via the synthesizer.
	profile := h.AgentUnderstandingKeeper.ComposeProfile(h.Ctx, agent)
	require.NotNil(t, profile)
	require.Equal(t, agent, profile.Agent)
	require.GreaterOrEqual(t, profile.TotalFactsAuthored, uint64(1),
		"agent_understanding profile must reflect a fact the agent just authored — without this, the synthesizer is reading the wrong source")

	// And the per-domain profile must have the philosophy entry.
	var philProfile *struct{ count uint64 }
	for _, d := range profile.Domains {
		if d.Domain == "philosophy" {
			philProfile = &struct{ count uint64 }{count: d.FactsAuthored}
			break
		}
	}
	require.NotNil(t, philProfile, "philosophy domain must appear in profile")
	require.GreaterOrEqual(t, philProfile.count, uint64(1),
		"philosophy.facts_authored must reflect the new fact")
}

// TestAgentUnderstanding_ZeroActivityReturnsEmpty asserts an agent
// with no activity gets a clean empty profile (no spurious counts).
func TestAgentUnderstanding_ZeroActivityReturnsEmpty(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	agent := testAddr("au_unknown_agent").String()
	profile := h.AgentUnderstandingKeeper.ComposeProfile(h.Ctx, agent)
	require.Equal(t, uint64(0), profile.TotalFactsAuthored)
	require.Equal(t, uint64(0), profile.TotalCounterexamplesValidated)
	require.Equal(t, uint64(0), profile.TotalInquiriesWon)
	require.Equal(t, uint64(0), profile.FrontierReachBps)
	require.Equal(t, uint64(0), profile.CompositeScoreBps)
}

