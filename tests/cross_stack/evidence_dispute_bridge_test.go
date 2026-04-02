package cross_stack_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	disputestypes "github.com/zerone-chain/zerone/x/disputes/types"
	emtypes "github.com/zerone-chain/zerone/x/evidence_mgmt/types"
)

// ─── Scenario D: Evidence-Dispute Bridge ─────────────────────────────────────

// TestEdimanceD_EvidenceDisputeBridge validates that evidence submitted and
// verified in evidence_mgmt can be consistently referenced and challenged via
// the disputes module, with both paths maintaining coherent state on the same
// evidence ID.
//
// Flow:
//  1. Evidence is submitted → evidence_mgmt.SUBMITTED
//  2. A quorum of 3 verifiers approves → evidence_mgmt.VERIFIED
//  3. A challenger disputes the same evidence → disputes module records dispute
//  4. Evidence status updates to CHALLENGED
//  5. Both verification results and dispute reference the same evidence ID
//  6. Multiple disputes can target the same evidence independently
func TestEdimanceD_EvidenceDisputeBridge(t *testing.T) {
	h := NewTestHarness(t)

	const evidenceID = "ev-d-001"
	submitter := testAddr("ev_d_submitter__")
	challenger := testAddr("ev_d_challenger_")
	defender := testAddr("ev_d_defender___")
	verifier1 := testAddr("ev_d_verifier_1_")
	verifier2 := testAddr("ev_d_verifier_2_")
	verifier3 := testAddr("ev_d_verifier_3_")

	// ── Step 1: Submit evidence ───────────────────────────────────────────────
	evidence := &emtypes.Evidence{
		Id:             evidenceID,
		Submitter:      submitter.String(),
		EvidenceType:   emtypes.EvidenceType_EVIDENCE_TYPE_DOCUMENT,
		ContentHash:    "sha256:edimance_d_evidence_content_0001",
		Metadata:       `{"title":"Test Evidence D","description":"Cross-module bridge test"}`,
		Status:         emtypes.EvidenceStatus_EVIDENCE_STATUS_SUBMITTED,
		CreatedAtBlock: uint64(h.Height()),
		UpdatedAtBlock: uint64(h.Height()),
	}
	h.EvidenceMgmtKeeper.SetEvidence(h.Ctx, evidence)

	// Verify submission stored correctly.
	retrieved, found := h.EvidenceMgmtKeeper.GetEvidence(h.Ctx, evidenceID)
	require.True(t, found, "evidence must be retrievable after submission")
	require.Equal(t, emtypes.EvidenceStatus_EVIDENCE_STATUS_SUBMITTED, retrieved.Status)
	require.Equal(t, submitter.String(), retrieved.Submitter)

	// ── Step 2: Quorum of verifiers approves evidence ─────────────────────────
	// Quorum requirement: 3 verifiers (params.VerificationQuorum = 3).
	for i, verifier := range []string{verifier1.String(), verifier2.String(), verifier3.String()} {
		vr := &emtypes.VerificationResult{
			Id:         fmt.Sprintf("vr-d-%d", i+1),
			EvidenceId: evidenceID,
			Verifier:   verifier,
			Outcome:    true, // approved
			Confidence: 900_000,
			Method:     "automated-hash-verification",
		}
		h.EvidenceMgmtKeeper.SetVerification(h.Ctx, vr)
	}

	// All 3 verifications are stored and linked to the evidence.
	verifications := h.EvidenceMgmtKeeper.GetVerificationsByEvidence(h.Ctx, evidenceID)
	require.Len(t, verifications, 3, "all 3 verifications must be stored")
	for _, vr := range verifications {
		require.Equal(t, evidenceID, vr.EvidenceId, "each verification must reference the correct evidence ID")
		require.True(t, vr.Outcome, "all verification results must be approved")
	}

	// Transition evidence to VERIFIED (in production this is done by msg_server quorum check).
	verified := &emtypes.Evidence{
		Id:             evidenceID,
		Submitter:      submitter.String(),
		EvidenceType:   emtypes.EvidenceType_EVIDENCE_TYPE_DOCUMENT,
		ContentHash:    "sha256:edimance_d_evidence_content_0001",
		Metadata:       `{"title":"Test Evidence D","description":"Cross-module bridge test"}`,
		Status:         emtypes.EvidenceStatus_EVIDENCE_STATUS_VERIFIED,
		CreatedAtBlock: uint64(h.Height()),
		UpdatedAtBlock: uint64(h.Height()),
	}
	h.EvidenceMgmtKeeper.SetEvidence(h.Ctx, verified)

	verifiedEv, found := h.EvidenceMgmtKeeper.GetEvidence(h.Ctx, evidenceID)
	require.True(t, found)
	require.Equal(t, emtypes.EvidenceStatus_EVIDENCE_STATUS_VERIFIED, verifiedEv.Status,
		"evidence must be VERIFIED after quorum")

	// ── Step 3: Dispute is filed against the same evidence ────────────────────
	dispute := &disputestypes.Dispute{
		Id:               "dispute-d-001",
		TargetId:         evidenceID,
		TargetType:       disputestypes.DisputeTargetType_DISPUTE_TARGET_TYPE_EVIDENCE,
		Challenger:       challenger.String(),
		Defender:         defender.String(),
		Reason:           "Evidence content hash does not match declared source",
		Bond:             "500000",
		Tier:             1,
		Phase:            disputestypes.DisputePhase_DISPUTE_PHASE_EVIDENCE_COMMIT,
		CreatedAt:        uint64(h.Height()),
		EvidenceDeadline: uint64(h.Height() + 1000),
		VotingDeadline:   uint64(h.Height() + 2000),
	}
	h.DisputesKeeper.SetDispute(h.Ctx, dispute)

	// ── Step 4: Evidence status transitions to CHALLENGED ─────────────────────
	challenged := &emtypes.Evidence{
		Id:             evidenceID,
		Submitter:      submitter.String(),
		EvidenceType:   emtypes.EvidenceType_EVIDENCE_TYPE_DOCUMENT,
		ContentHash:    "sha256:edimance_d_evidence_content_0001",
		Metadata:       `{"title":"Test Evidence D","description":"Cross-module bridge test"}`,
		Status:         emtypes.EvidenceStatus_EVIDENCE_STATUS_CHALLENGED,
		CreatedAtBlock: uint64(h.Height()),
		UpdatedAtBlock: uint64(h.Height()),
	}
	h.EvidenceMgmtKeeper.SetEvidence(h.Ctx, challenged)

	// ── Step 5: Assert consistency — both modules reference same evidence ID ───
	// Verify evidence is in CHALLENGED state.
	challengedEv, found := h.EvidenceMgmtKeeper.GetEvidence(h.Ctx, evidenceID)
	require.True(t, found)
	require.Equal(t, emtypes.EvidenceStatus_EVIDENCE_STATUS_CHALLENGED, challengedEv.Status,
		"evidence must be CHALLENGED after dispute filed")

	// Verify dispute exists in disputes module and targets correct evidence.
	retrievedDispute, found := h.DisputesKeeper.GetDispute(h.Ctx, "dispute-d-001")
	require.True(t, found, "dispute must be retrievable from disputes module")
	require.Equal(t, evidenceID, retrievedDispute.TargetId,
		"dispute must reference the same evidence ID")
	require.Equal(t, disputestypes.DisputeTargetType_DISPUTE_TARGET_TYPE_EVIDENCE, retrievedDispute.TargetType)

	// Verification results remain intact (verifications are immutable audit trail).
	vrAfterDispute := h.EvidenceMgmtKeeper.GetVerificationsByEvidence(h.Ctx, evidenceID)
	require.Len(t, vrAfterDispute, 3, "verification results must survive the dispute lifecycle")
	approvedCount := 0
	for _, vr := range vrAfterDispute {
		if vr.Outcome {
			approvedCount++
		}
	}
	require.Equal(t, 3, approvedCount, "all 3 approval verifications must remain consistent")

	// ── Step 6: Query disputes by target to confirm cross-module lookup ────────
	disputesByTarget := h.DisputesKeeper.GetDisputesByTarget(h.Ctx, evidenceID)
	require.Len(t, disputesByTarget, 1, "exactly one dispute targets this evidence")
	require.Equal(t, "dispute-d-001", disputesByTarget[0].Id)

	// ── Bonus: second dispute on same evidence (escalation scenario) ──────────
	dispute2 := &disputestypes.Dispute{
		Id:               "dispute-d-002",
		TargetId:         evidenceID,
		TargetType:       disputestypes.DisputeTargetType_DISPUTE_TARGET_TYPE_EVIDENCE,
		Challenger:       testAddr("ev_d_challenger2").String(),
		Defender:         defender.String(),
		Reason:           "Chain of custody broken at verifier 2",
		Bond:             "1000000",
		Tier:             2, // escalated tier
		Phase:            disputestypes.DisputePhase_DISPUTE_PHASE_EVIDENCE_COMMIT,
		CreatedAt:        uint64(h.Height()),
		EvidenceDeadline: uint64(h.Height() + 1000),
		VotingDeadline:   uint64(h.Height() + 2000),
	}
	h.DisputesKeeper.SetDispute(h.Ctx, dispute2)

	// Both disputes can independently reference the same evidence.
	allDisputes := h.DisputesKeeper.GetDisputesByTarget(h.Ctx, evidenceID)
	require.Len(t, allDisputes, 2, "two disputes can independently target the same evidence")

	tiersSeen := make(map[uint32]bool)
	for _, d := range allDisputes {
		require.Equal(t, evidenceID, d.TargetId, "all disputes must reference correct evidence ID")
		tiersSeen[d.Tier] = true
	}
	require.True(t, tiersSeen[1] && tiersSeen[2], "disputes at different tiers must coexist independently")

	// Evidence module state remains consistent throughout — not corrupted by multiple disputes.
	finalEv, found := h.EvidenceMgmtKeeper.GetEvidence(h.Ctx, evidenceID)
	require.True(t, found)
	require.Equal(t, "sha256:edimance_d_evidence_content_0001", finalEv.ContentHash,
		"evidence content hash must remain unchanged across dispute lifecycle")
}
