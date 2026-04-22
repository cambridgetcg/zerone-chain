package cross_stack_test

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	knowledgekeeper "github.com/zerone-chain/zerone/x/knowledge/keeper"
	knowledgetypes "github.com/zerone-chain/zerone/x/knowledge/types"
	ontologytypes "github.com/zerone-chain/zerone/x/ontology/types"
)

// TestFullLoop_HappyPath drives a single claim from SubmitClaim through
// commit → reveal → aggregation → fact creation using the real message
// server and ABCI phase transitions. No SetFact shortcut — this is the
// regression guard for cross-module integration that T13 required.
func TestFullLoop_HappyPath(t *testing.T) {
	h := NewTestHarness(t)

	// ─── Loosen gates so a single-validator harness can run a round ──────
	kParams, err := h.KnowledgeKeeper.GetParams(h.Ctx)
	require.NoError(t, err)
	kParams.MinVerifiers = 1
	kParams.CommitPhaseBlocks = 2
	kParams.RevealPhaseBlocks = 2
	kParams.AggregationPhaseBlocks = 1
	// Keep review fee modest so a funded account can pay.
	kParams.MinReviewFee = "1000000" // 1 ZRN
	require.NoError(t, h.KnowledgeKeeper.SetParams(h.Ctx, kParams))

	// ─── Register the domain in BOTH stores used by the loop ────────────
	domain := "fullloop_physics"
	// Knowledge module checks its own domain store (msg_server.go:46).
	require.NoError(t, h.KnowledgeKeeper.SetDomain(h.Ctx, &knowledgetypes.Domain{
		Name:   domain,
		Status: knowledgetypes.DomainStatus_DOMAIN_STATUS_ACTIVE,
	}))
	// Ontology keeper is consulted for stratum/depth during fact creation.
	h.App.ZeroneOntologyKeeper.SetDomain(h.Ctx, &ontologytypes.Domain{
		Name:    domain,
		Status:  "active",
		Stratum: uint32(ontologytypes.StratumEmpirical),
		Depth:   1,
	})

	// ─── Fund submitter and verifier (each needs ≥100 ZRN for commit gate) ─
	submitterAcc := sdk.AccAddress([]byte("fullloop-submitter00"))
	verifierAcc := sdk.AccAddress([]byte("fullloop-verifier001"))
	submitterAddr := submitterAcc.String()
	verifierAddr := verifierAcc.String()

	fund := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewInt(500_000_000))) // 500 ZRN
	require.NoError(t, h.FundAccount(submitterAcc, fund))
	require.NoError(t, h.FundAccount(verifierAcc, fund))

	// ─── Phase 1: Submit claim ──────────────────────────────────────────
	ms := knowledgekeeper.NewMsgServerImpl(h.KnowledgeKeeper)

	submitResp, err := ms.SubmitClaim(h.Ctx, &knowledgetypes.MsgSubmitClaim{
		Submitter:   submitterAddr,
		FactContent: "Photons travel at approximately 299,792,458 m/s in a vacuum.",
		Domain:      domain,
		Category:    "empirical",
		Stake:       "1000000", // matches MinReviewFee
	})
	require.NoError(t, err)
	claimID := submitResp.ClaimId
	require.NotEmpty(t, claimID)

	claim, ok := h.KnowledgeKeeper.GetClaim(h.Ctx, claimID)
	require.True(t, ok)
	require.Equal(t, knowledgetypes.ClaimStatus_CLAIM_STATUS_IN_VERIFICATION, claim.Status)
	roundID := claim.VerificationRoundId
	require.NotEmpty(t, roundID)

	round, ok := h.KnowledgeKeeper.GetVerificationRound(h.Ctx, roundID)
	require.True(t, ok)
	require.Equal(t, knowledgetypes.VerificationPhase_VERIFICATION_PHASE_COMMIT, round.Phase)

	// ─── Phase 2: Commit ────────────────────────────────────────────────
	vote := "accept"
	salt := []byte("fullloop-salt-001")
	commitHash := knowledgetypes.ComputeCommitmentHash(roundID, vote, 0, salt)

	_, err = ms.SubmitCommitment(h.Ctx, &knowledgetypes.MsgSubmitCommitment{
		Verifier:   verifierAddr,
		RoundId:    roundID,
		CommitHash: commitHash,
	})
	require.NoError(t, err)

	round, _ = h.KnowledgeKeeper.GetVerificationRound(h.Ctx, roundID)
	require.Len(t, round.Commits, 1, "commit must be recorded")
	require.Contains(t, round.SelectedVerifiers, verifierAddr,
		"unified path should populate SelectedVerifiers (T-i1)")

	// ─── Phase 3: Advance height past CommitDeadline and run phase transition ─
	h.Ctx = h.Ctx.WithBlockHeight(int64(round.CommitDeadline) + 1)
	require.NoError(t, h.KnowledgeKeeper.AdvanceRoundPhases(h.Ctx))

	round, _ = h.KnowledgeKeeper.GetVerificationRound(h.Ctx, roundID)
	require.Equal(t, knowledgetypes.VerificationPhase_VERIFICATION_PHASE_REVEAL, round.Phase,
		"AdvanceRoundPhases should transition COMMIT → REVEAL after CommitDeadline")

	// ─── Phase 4: Reveal ────────────────────────────────────────────────
	_, err = ms.SubmitReveal(h.Ctx, &knowledgetypes.MsgSubmitReveal{
		Verifier:   verifierAddr,
		RoundId:    roundID,
		Vote:       vote,
		Salt:       salt,
		Confidence: 0,
	})
	require.NoError(t, err, "canonical ComputeCommitmentHash must validate on tx path (T-i2)")

	round, _ = h.KnowledgeKeeper.GetVerificationRound(h.Ctx, roundID)
	require.Len(t, round.Reveals, 1)
	require.Equal(t, "accept", round.Reveals[0].Vote)

	// ─── Phase 5: Advance past RevealDeadline → aggregation + completion ─
	h.Ctx = h.Ctx.WithBlockHeight(int64(round.RevealDeadline) + 1)
	require.NoError(t, h.KnowledgeKeeper.AdvanceRoundPhases(h.Ctx))

	round, _ = h.KnowledgeKeeper.GetVerificationRound(h.Ctx, roundID)
	require.Equal(t, knowledgetypes.VerificationPhase_VERIFICATION_PHASE_COMPLETE, round.Phase,
		"phase transition should aggregate and complete the round")
	require.Equal(t, knowledgetypes.Verdict_VERDICT_ACCEPT, round.Verdict)

	// ─── Phase 6: Fact was created ──────────────────────────────────────
	claim, _ = h.KnowledgeKeeper.GetClaim(h.Ctx, claimID)
	require.Equal(t, knowledgetypes.ClaimStatus_CLAIM_STATUS_ACCEPTED, claim.Status)

	factCount := 0
	h.KnowledgeKeeper.IterateFactsByDomain(h.Ctx, domain, func(factID string) bool {
		factCount++
		fact, ok := h.KnowledgeKeeper.GetFact(h.Ctx, factID)
		require.True(t, ok)
		require.Greater(t, fact.Confidence, uint64(0), "fact must have non-zero confidence")
		require.Greater(t, fact.Energy, uint64(0), "fact must start with metabolism energy")
		return false
	})
	require.Equal(t, 1, factCount, "exactly one fact should be created from the accepted claim")
}

// TestFullLoop_ContradictionRejected exercises T-i4: when a contradicting
// claim is rejected, the target fact's CONTESTED flag should be reversed.
func TestFullLoop_ContradictionRejected(t *testing.T) {
	h := NewTestHarness(t)

	// Seed a VERIFIED target fact directly — the loop we're testing is the
	// REJECTED contradiction's side-effect reversal, not the fact creation.
	domain := "contradiction_test_domain"
	require.NoError(t, h.KnowledgeKeeper.SetDomain(h.Ctx, &knowledgetypes.Domain{
		Name:   domain,
		Status: knowledgetypes.DomainStatus_DOMAIN_STATUS_ACTIVE,
	}))
	h.App.ZeroneOntologyKeeper.SetDomain(h.Ctx, &ontologytypes.Domain{
		Name:    domain,
		Status:  "active",
		Stratum: uint32(ontologytypes.StratumEmpirical),
		Depth:   1,
	})

	targetFactID := "target-fact-contested"
	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, &knowledgetypes.Fact{
		Id:         targetFactID,
		Content:    "Water boils at 100 degrees Celsius at sea level.",
		Domain:     domain,
		Category:   "empirical",
		Confidence: 800_000,
		Status:     knowledgetypes.FactStatus_FACT_STATUS_VERIFIED,
	}))

	// Simulate the side-effect that SubmitClaim would have applied on a
	// CONTRADICTS relation: flip target fact to CONTESTED.
	target, ok := h.KnowledgeKeeper.GetFact(h.Ctx, targetFactID)
	require.True(t, ok)
	target.Status = knowledgetypes.FactStatus_FACT_STATUS_CONTESTED
	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, target))

	// Build a claim with a CONTRADICTS relation and a completed REJECT round.
	claim := &knowledgetypes.Claim{
		Id:          "claim-contradicting-rejected",
		Submitter:   sdk.AccAddress([]byte("contra-submitter0001")).String(),
		FactContent: "Water boils at 80 degrees Celsius at sea level (wrong).",
		Domain:      domain,
		Category:    "empirical",
		Status:      knowledgetypes.ClaimStatus_CLAIM_STATUS_IN_VERIFICATION,
		Relations: []*knowledgetypes.ClaimRelation{
			{
				Relation:     knowledgetypes.RelationType_RELATION_TYPE_CONTRADICTS,
				TargetFactId: targetFactID,
			},
		},
	}
	require.NoError(t, h.KnowledgeKeeper.SetClaim(h.Ctx, claim))

	round := &knowledgetypes.VerificationRound{
		Id:             "round-contradicting",
		ClaimId:        claim.Id,
		Phase:          knowledgetypes.VerificationPhase_VERIFICATION_PHASE_COMPLETE,
		StartedAtBlock: 1,
	}

	// CompleteRound with REJECT verdict should trigger reverseContradictionsFromClaim.
	result := &knowledgekeeper.VerificationResult{
		Verdict:     knowledgetypes.Verdict_VERDICT_REJECT,
		Confidence:  800_000,
		AcceptCount: 0,
		RejectCount: 1,
	}
	require.NoError(t, h.KnowledgeKeeper.CompleteRound(h.Ctx, round, result))

	// Target fact must be restored to VERIFIED (T-i4 fix).
	restored, ok := h.KnowledgeKeeper.GetFact(h.Ctx, targetFactID)
	require.True(t, ok)
	require.Equal(t, knowledgetypes.FactStatus_FACT_STATUS_VERIFIED, restored.Status,
		"target fact must be restored from CONTESTED → VERIFIED when the contradicting claim fails")
}
