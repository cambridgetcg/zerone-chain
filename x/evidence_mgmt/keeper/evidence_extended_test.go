package keeper_test

import (
	"fmt"
	"testing"

	"github.com/zerone-chain/zerone/x/evidence_mgmt/keeper"
	"github.com/zerone-chain/zerone/x/evidence_mgmt/types"
)

// ========== Submit Evidence — Extended ==========

func TestSubmitEvidenceMultipleSequential(t *testing.T) {
	msgSrv, k, ctx, _, _ := setupMsgServer(t)

	submitter := testAddr("alice")
	ids := make([]string, 5)

	for i := 0; i < 5; i++ {
		resp, err := msgSrv.SubmitEvidence(ctx, &types.MsgSubmitEvidence{
			Submitter:    submitter,
			EvidenceType: types.EvidenceType_EVIDENCE_TYPE_DOCUMENT,
			ContentHash:  fmt.Sprintf("hash-%d", i),
			Metadata:     fmt.Sprintf("meta-%d", i),
		})
		if err != nil {
			t.Fatalf("SubmitEvidence %d failed: %v", i, err)
		}
		ids[i] = resp.EvidenceId
	}

	// All IDs should be unique
	seen := make(map[string]bool)
	for _, id := range ids {
		if seen[id] {
			t.Errorf("duplicate evidence ID: %s", id)
		}
		seen[id] = true
	}

	// All should be retrievable
	all := k.GetAllEvidences(ctx)
	if len(all) != 5 {
		t.Errorf("expected 5 evidences, got %d", len(all))
	}
}

func TestSubmitEvidenceDifferentTypes(t *testing.T) {
	msgSrv, k, ctx, _, _ := setupMsgServer(t)

	evidenceTypes := []types.EvidenceType{
		types.EvidenceType_EVIDENCE_TYPE_DOCUMENT,
		types.EvidenceType_EVIDENCE_TYPE_ATTESTATION,
		types.EvidenceType_EVIDENCE_TYPE_MEASUREMENT,
		types.EvidenceType_EVIDENCE_TYPE_COMPUTATION,
	}

	for i, et := range evidenceTypes {
		resp, err := msgSrv.SubmitEvidence(ctx, &types.MsgSubmitEvidence{
			Submitter:    testAddr(fmt.Sprintf("submitter-%d", i)),
			EvidenceType: et,
			ContentHash:  fmt.Sprintf("hash-type-%d", i),
			Metadata:     "typed evidence",
		})
		if err != nil {
			t.Fatalf("SubmitEvidence type %s failed: %v", et.String(), err)
		}

		evidence, found := k.GetEvidence(ctx, resp.EvidenceId)
		if !found {
			t.Fatalf("evidence not found for type %s", et.String())
		}
		if evidence.EvidenceType != et {
			t.Errorf("expected type %s, got %s", et.String(), evidence.EvidenceType.String())
		}
	}
}

func TestSubmitEvidenceSubmitterIndex(t *testing.T) {
	msgSrv, k, ctx, _, _ := setupMsgServer(t)

	alice := testAddr("alice")
	bob := testAddr("bob")

	// Alice submits 3 evidences
	for i := 0; i < 3; i++ {
		_, err := msgSrv.SubmitEvidence(ctx, &types.MsgSubmitEvidence{
			Submitter:    alice,
			EvidenceType: types.EvidenceType_EVIDENCE_TYPE_DOCUMENT,
			ContentHash:  fmt.Sprintf("alice-hash-%d", i),
		})
		if err != nil {
			t.Fatalf("SubmitEvidence (alice) failed: %v", err)
		}
	}

	// Bob submits 1 evidence
	_, err := msgSrv.SubmitEvidence(ctx, &types.MsgSubmitEvidence{
		Submitter:    bob,
		EvidenceType: types.EvidenceType_EVIDENCE_TYPE_DOCUMENT,
		ContentHash:  "bob-hash-1",
	})
	if err != nil {
		t.Fatalf("SubmitEvidence (bob) failed: %v", err)
	}

	aliceEvids := k.GetEvidenceBySubmitter(ctx, alice)
	if len(aliceEvids) != 3 {
		t.Errorf("expected 3 for alice, got %d", len(aliceEvids))
	}
	bobEvids := k.GetEvidenceBySubmitter(ctx, bob)
	if len(bobEvids) != 1 {
		t.Errorf("expected 1 for bob, got %d", len(bobEvids))
	}
}

func TestSubmitEvidenceCreatesInitialCustody(t *testing.T) {
	msgSrv, k, ctx, _, _ := setupMsgServer(t)

	submitter := testAddr("alice")
	resp, err := msgSrv.SubmitEvidence(ctx, &types.MsgSubmitEvidence{
		Submitter:    submitter,
		EvidenceType: types.EvidenceType_EVIDENCE_TYPE_DOCUMENT,
		ContentHash:  "hash1",
	})
	if err != nil {
		t.Fatalf("SubmitEvidence failed: %v", err)
	}

	evidence, _ := k.GetEvidence(ctx, resp.EvidenceId)
	if len(evidence.ChainOfCustody) != 1 {
		t.Fatalf("expected 1 custody entry, got %d", len(evidence.ChainOfCustody))
	}
	entry := evidence.ChainOfCustody[0]
	if entry.Custodian != submitter {
		t.Errorf("expected custodian %s, got %s", submitter, entry.Custodian)
	}
	if entry.Action != "submit" {
		t.Errorf("expected action 'submit', got %s", entry.Action)
	}
	if entry.Timestamp != 100 { // block height from setup
		t.Errorf("expected timestamp 100, got %d", entry.Timestamp)
	}
}

// ========== Transfer Custody — Extended ==========

func TestTransferCustodyMultipleTransfers(t *testing.T) {
	msgSrv, k, ctx, _, _ := setupMsgServer(t)

	alice := testAddr("alice")
	bob := testAddr("bob")
	charlie := testAddr("charlie")

	resp, _ := msgSrv.SubmitEvidence(ctx, &types.MsgSubmitEvidence{
		Submitter:    alice,
		EvidenceType: types.EvidenceType_EVIDENCE_TYPE_DOCUMENT,
		ContentHash:  "hash1",
	})

	// alice → bob
	_, err := msgSrv.TransferCustody(ctx, &types.MsgTransferCustody{
		EvidenceId:       resp.EvidenceId,
		CurrentCustodian: alice,
		NewCustodian:     bob,
		Notes:            "transfer to bob",
	})
	if err != nil {
		t.Fatalf("transfer alice→bob failed: %v", err)
	}

	// bob → charlie
	_, err = msgSrv.TransferCustody(ctx, &types.MsgTransferCustody{
		EvidenceId:       resp.EvidenceId,
		CurrentCustodian: bob,
		NewCustodian:     charlie,
		Notes:            "transfer to charlie",
	})
	if err != nil {
		t.Fatalf("transfer bob→charlie failed: %v", err)
	}

	evidence, _ := k.GetEvidence(ctx, resp.EvidenceId)
	if len(evidence.ChainOfCustody) != 3 {
		t.Fatalf("expected 3 custody entries, got %d", len(evidence.ChainOfCustody))
	}
	if evidence.ChainOfCustody[2].Custodian != charlie {
		t.Errorf("expected last custodian charlie, got %s", evidence.ChainOfCustody[2].Custodian)
	}
}

func TestTransferCustodyNonExistentEvidence(t *testing.T) {
	msgSrv, _, ctx, _, _ := setupMsgServer(t)

	_, err := msgSrv.TransferCustody(ctx, &types.MsgTransferCustody{
		EvidenceId:       "nonexistent",
		CurrentCustodian: testAddr("alice"),
		NewCustodian:     testAddr("bob"),
	})
	if err == nil {
		t.Fatal("expected error for nonexistent evidence")
	}
}

func TestTransferCustodyAliceCannotTransferAfterBob(t *testing.T) {
	msgSrv, _, ctx, _, _ := setupMsgServer(t)

	alice := testAddr("alice")
	bob := testAddr("bob")

	resp, _ := msgSrv.SubmitEvidence(ctx, &types.MsgSubmitEvidence{
		Submitter:    alice,
		EvidenceType: types.EvidenceType_EVIDENCE_TYPE_DOCUMENT,
		ContentHash:  "hash1",
	})

	// Transfer to bob
	_, _ = msgSrv.TransferCustody(ctx, &types.MsgTransferCustody{
		EvidenceId:       resp.EvidenceId,
		CurrentCustodian: alice,
		NewCustodian:     bob,
	})

	// Alice no longer custodian — should fail
	_, err := msgSrv.TransferCustody(ctx, &types.MsgTransferCustody{
		EvidenceId:       resp.EvidenceId,
		CurrentCustodian: alice,
		NewCustodian:     testAddr("charlie"),
	})
	if err == nil {
		t.Fatal("expected error: alice is no longer custodian")
	}
}

// ========== Verify Evidence — Extended ==========

func TestVerifyEvidenceQuorumRejection(t *testing.T) {
	msgSrv, k, ctx, sk, _ := setupMsgServer(t)

	submitter := testAddr("submitter")
	v1 := testAddr("v1")
	v2 := testAddr("v2")
	v3 := testAddr("v3")
	sk.tiers[v1] = 3
	sk.tiers[v2] = 3
	sk.tiers[v3] = 3

	resp, _ := msgSrv.SubmitEvidence(ctx, &types.MsgSubmitEvidence{
		Submitter:    submitter,
		EvidenceType: types.EvidenceType_EVIDENCE_TYPE_DOCUMENT,
		ContentHash:  "hash1",
	})

	// 2 negative, 1 positive → REJECTED
	msgSrv.VerifyEvidence(ctx, &types.MsgVerifyEvidence{EvidenceId: resp.EvidenceId, Verifier: v1, Outcome: false, Confidence: 800000, Method: "review"})
	msgSrv.VerifyEvidence(ctx, &types.MsgVerifyEvidence{EvidenceId: resp.EvidenceId, Verifier: v2, Outcome: false, Confidence: 700000, Method: "review"})
	msgSrv.VerifyEvidence(ctx, &types.MsgVerifyEvidence{EvidenceId: resp.EvidenceId, Verifier: v3, Outcome: true, Confidence: 900000, Method: "review"})

	evidence, _ := k.GetEvidence(ctx, resp.EvidenceId)
	if evidence.Status != types.EvidenceStatus_EVIDENCE_STATUS_REJECTED {
		t.Errorf("expected REJECTED, got %s", evidence.Status.String())
	}
}

func TestVerifyEvidenceSplitVote(t *testing.T) {
	msgSrv, k, ctx, sk, _ := setupMsgServer(t)

	// Set quorum to 4 for even split test
	k.SetParams(ctx, &types.Params{
		MinVerifierTier:       2,
		VerificationQuorum:    4,
		ChallengeBond:         "500000",
		ChallengeWindowBlocks: 50000,
	})

	submitter := testAddr("submitter")
	verifiers := make([]string, 4)
	for i := 0; i < 4; i++ {
		verifiers[i] = testAddr(fmt.Sprintf("v%d", i))
		sk.tiers[verifiers[i]] = 3
	}

	resp, _ := msgSrv.SubmitEvidence(ctx, &types.MsgSubmitEvidence{
		Submitter:    submitter,
		EvidenceType: types.EvidenceType_EVIDENCE_TYPE_DOCUMENT,
		ContentHash:  "hash1",
	})

	// 2 positive, 2 negative — tie → REJECTED (positive <= total/2)
	for i, v := range verifiers {
		msgSrv.VerifyEvidence(ctx, &types.MsgVerifyEvidence{
			EvidenceId: resp.EvidenceId,
			Verifier:   v,
			Outcome:    i < 2, // first 2 positive, last 2 negative
			Confidence: 500000,
			Method:     "review",
		})
	}

	evidence, _ := k.GetEvidence(ctx, resp.EvidenceId)
	if evidence.Status != types.EvidenceStatus_EVIDENCE_STATUS_REJECTED {
		t.Errorf("expected REJECTED for tie vote, got %s", evidence.Status.String())
	}
}

func TestVerifyEvidenceBelowQuorum(t *testing.T) {
	msgSrv, k, ctx, sk, _ := setupMsgServer(t)

	submitter := testAddr("submitter")
	v1 := testAddr("v1")
	sk.tiers[v1] = 3

	resp, _ := msgSrv.SubmitEvidence(ctx, &types.MsgSubmitEvidence{
		Submitter:    submitter,
		EvidenceType: types.EvidenceType_EVIDENCE_TYPE_DOCUMENT,
		ContentHash:  "hash1",
	})

	// Only 1 verification, quorum is 3 → status should NOT change
	msgSrv.VerifyEvidence(ctx, &types.MsgVerifyEvidence{
		EvidenceId: resp.EvidenceId,
		Verifier:   v1,
		Outcome:    true,
		Confidence: 900000,
		Method:     "review",
	})

	evidence, _ := k.GetEvidence(ctx, resp.EvidenceId)
	if evidence.Status != types.EvidenceStatus_EVIDENCE_STATUS_SUBMITTED {
		t.Errorf("expected SUBMITTED (below quorum), got %s", evidence.Status.String())
	}
}

func TestVerifyEvidenceNonExistent(t *testing.T) {
	msgSrv, _, ctx, sk, _ := setupMsgServer(t)

	v := testAddr("verifier")
	sk.tiers[v] = 3

	_, err := msgSrv.VerifyEvidence(ctx, &types.MsgVerifyEvidence{
		EvidenceId: "nonexistent",
		Verifier:   v,
		Outcome:    true,
		Confidence: 900000,
		Method:     "review",
	})
	if err == nil {
		t.Fatal("expected error for nonexistent evidence")
	}
}

func TestVerifyEvidenceMultipleMethods(t *testing.T) {
	msgSrv, k, ctx, sk, _ := setupMsgServer(t)

	submitter := testAddr("submitter")
	v1 := testAddr("v1")
	v2 := testAddr("v2")
	sk.tiers[v1] = 3
	sk.tiers[v2] = 3

	resp, _ := msgSrv.SubmitEvidence(ctx, &types.MsgSubmitEvidence{
		Submitter:    submitter,
		EvidenceType: types.EvidenceType_EVIDENCE_TYPE_DOCUMENT,
		ContentHash:  "hash1",
	})

	msgSrv.VerifyEvidence(ctx, &types.MsgVerifyEvidence{
		EvidenceId: resp.EvidenceId,
		Verifier:   v1,
		Outcome:    true,
		Confidence: 900000,
		Method:     "manual_review",
	})
	msgSrv.VerifyEvidence(ctx, &types.MsgVerifyEvidence{
		EvidenceId: resp.EvidenceId,
		Verifier:   v2,
		Outcome:    true,
		Confidence: 800000,
		Method:     "automated_check",
	})

	verifications := k.GetVerificationsByEvidence(ctx, resp.EvidenceId)
	methods := make(map[string]bool)
	for _, v := range verifications {
		methods[v.Method] = true
	}
	if !methods["manual_review"] || !methods["automated_check"] {
		t.Error("expected both verification methods to be recorded")
	}
}

// ========== Challenge Evidence — Extended ==========

func TestChallengeEvidenceBondBelowMinimum(t *testing.T) {
	msgSrv, _, ctx, _, _ := setupMsgServer(t)

	submitter := testAddr("submitter")
	challenger := testAddr("challenger")

	resp, _ := msgSrv.SubmitEvidence(ctx, &types.MsgSubmitEvidence{
		Submitter:    submitter,
		EvidenceType: types.EvidenceType_EVIDENCE_TYPE_DOCUMENT,
		ContentHash:  "hash1",
	})

	// Default min bond is 500000; send 100
	_, err := msgSrv.ChallengeEvidence(ctx, &types.MsgChallengeEvidence{
		EvidenceId: resp.EvidenceId,
		Challenger: challenger,
		Reason:     "suspicious",
		Bond:       "100",
	})
	if err == nil {
		t.Fatal("expected error for bond below minimum")
	}
}

func TestChallengeEvidenceInvalidBond(t *testing.T) {
	msgSrv, _, ctx, _, _ := setupMsgServer(t)

	submitter := testAddr("submitter")
	challenger := testAddr("challenger")

	resp, _ := msgSrv.SubmitEvidence(ctx, &types.MsgSubmitEvidence{
		Submitter:    submitter,
		EvidenceType: types.EvidenceType_EVIDENCE_TYPE_DOCUMENT,
		ContentHash:  "hash1",
	})

	_, err := msgSrv.ChallengeEvidence(ctx, &types.MsgChallengeEvidence{
		EvidenceId: resp.EvidenceId,
		Challenger: challenger,
		Reason:     "suspicious",
		Bond:       "not_a_number",
	})
	if err == nil {
		t.Fatal("expected error for invalid bond format")
	}
}

func TestChallengeEvidenceNonExistent(t *testing.T) {
	msgSrv, _, ctx, _, _ := setupMsgServer(t)

	_, err := msgSrv.ChallengeEvidence(ctx, &types.MsgChallengeEvidence{
		EvidenceId: "nonexistent",
		Challenger: testAddr("challenger"),
		Reason:     "suspicious",
		Bond:       "500000",
	})
	if err == nil {
		t.Fatal("expected error for nonexistent evidence")
	}
}

func TestChallengeEvidenceAddsToChainOfCustody(t *testing.T) {
	msgSrv, k, ctx, _, _ := setupMsgServer(t)

	submitter := testAddr("submitter")
	challenger := testAddr("challenger")

	resp, _ := msgSrv.SubmitEvidence(ctx, &types.MsgSubmitEvidence{
		Submitter:    submitter,
		EvidenceType: types.EvidenceType_EVIDENCE_TYPE_DOCUMENT,
		ContentHash:  "hash1",
	})

	_, err := msgSrv.ChallengeEvidence(ctx, &types.MsgChallengeEvidence{
		EvidenceId: resp.EvidenceId,
		Challenger: challenger,
		Reason:     "fabricated data",
		Bond:       "500000",
	})
	if err != nil {
		t.Fatalf("ChallengeEvidence failed: %v", err)
	}

	evidence, _ := k.GetEvidence(ctx, resp.EvidenceId)
	// Should have 2 entries: submit + challenge
	if len(evidence.ChainOfCustody) != 2 {
		t.Fatalf("expected 2 custody entries, got %d", len(evidence.ChainOfCustody))
	}
	last := evidence.ChainOfCustody[1]
	if last.Custodian != challenger {
		t.Errorf("expected challenger as custodian, got %s", last.Custodian)
	}
	if last.Action != "challenge" {
		t.Errorf("expected action 'challenge', got %s", last.Action)
	}
	if last.Notes != "fabricated data" {
		t.Errorf("expected notes 'fabricated data', got %s", last.Notes)
	}
}

func TestChallengeEvidenceZeroBond(t *testing.T) {
	msgSrv, _, ctx, _, _ := setupMsgServer(t)

	submitter := testAddr("submitter")
	challenger := testAddr("challenger")

	resp, _ := msgSrv.SubmitEvidence(ctx, &types.MsgSubmitEvidence{
		Submitter:    submitter,
		EvidenceType: types.EvidenceType_EVIDENCE_TYPE_DOCUMENT,
		ContentHash:  "hash1",
	})

	_, err := msgSrv.ChallengeEvidence(ctx, &types.MsgChallengeEvidence{
		EvidenceId: resp.EvidenceId,
		Challenger: challenger,
		Reason:     "suspicious",
		Bond:       "0",
	})
	if err == nil {
		t.Fatal("expected error for zero bond")
	}
}

// ========== Query Server — Extended ==========

func TestQueryEvidenceBySubmitter(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	qs := keeper.NewQueryServerImpl(k)

	alice := testAddr("alice")
	k.SetEvidence(ctx, &types.Evidence{Id: "evid-a1", Submitter: alice, ContentHash: "h1", Status: types.EvidenceStatus_EVIDENCE_STATUS_SUBMITTED})
	k.SetEvidence(ctx, &types.Evidence{Id: "evid-a2", Submitter: alice, ContentHash: "h2", Status: types.EvidenceStatus_EVIDENCE_STATUS_SUBMITTED})

	resp, err := qs.QueryEvidenceBySubmitter(ctx, &types.QueryEvidenceBySubmitterRequest{Submitter: alice})
	if err != nil {
		t.Fatalf("QueryEvidenceBySubmitter failed: %v", err)
	}
	if len(resp.Evidences) != 2 {
		t.Errorf("expected 2 evidences, got %d", len(resp.Evidences))
	}
}

func TestQueryEvidenceBySubmitterEmpty(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	qs := keeper.NewQueryServerImpl(k)

	resp, err := qs.QueryEvidenceBySubmitter(ctx, &types.QueryEvidenceBySubmitterRequest{Submitter: testAddr("nobody")})
	if err != nil {
		t.Fatalf("QueryEvidenceBySubmitter failed: %v", err)
	}
	if len(resp.Evidences) != 0 {
		t.Errorf("expected 0 evidences, got %d", len(resp.Evidences))
	}
}

func TestQueryCustodyChain(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	qs := keeper.NewQueryServerImpl(k)

	k.SetEvidence(ctx, &types.Evidence{
		Id:          "evid-1",
		Submitter:   testAddr("alice"),
		ContentHash: "h1",
		ChainOfCustody: []*types.ChainOfCustodyEntry{
			{Custodian: testAddr("alice"), Action: "submit", Timestamp: 100},
			{Custodian: testAddr("bob"), Action: "transfer", Timestamp: 200},
		},
	})

	resp, err := qs.QueryCustodyChain(ctx, &types.QueryCustodyChainRequest{EvidenceId: "evid-1"})
	if err != nil {
		t.Fatalf("QueryCustodyChain failed: %v", err)
	}
	if len(resp.Entries) != 2 {
		t.Errorf("expected 2 custody entries, got %d", len(resp.Entries))
	}
}

func TestQueryCustodyChainNotFound(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	qs := keeper.NewQueryServerImpl(k)

	_, err := qs.QueryCustodyChain(ctx, &types.QueryCustodyChainRequest{EvidenceId: "nonexistent"})
	if err == nil {
		t.Fatal("expected error for nonexistent evidence")
	}
}

func TestQueryVerifications(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	qs := keeper.NewQueryServerImpl(k)

	k.SetVerification(ctx, &types.VerificationResult{Id: "v1", EvidenceId: "evid-1", Verifier: testAddr("v1"), Outcome: true, Confidence: 900000})
	k.SetVerification(ctx, &types.VerificationResult{Id: "v2", EvidenceId: "evid-1", Verifier: testAddr("v2"), Outcome: false, Confidence: 300000})

	resp, err := qs.QueryVerifications(ctx, &types.QueryVerificationsRequest{EvidenceId: "evid-1"})
	if err != nil {
		t.Fatalf("QueryVerifications failed: %v", err)
	}
	if len(resp.Verifications) != 2 {
		t.Errorf("expected 2 verifications, got %d", len(resp.Verifications))
	}
}

func TestQueryVerificationsEmpty(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	qs := keeper.NewQueryServerImpl(k)

	resp, err := qs.QueryVerifications(ctx, &types.QueryVerificationsRequest{EvidenceId: "evid-none"})
	if err != nil {
		t.Fatalf("QueryVerifications failed: %v", err)
	}
	if len(resp.Verifications) != 0 {
		t.Errorf("expected 0 verifications, got %d", len(resp.Verifications))
	}
}

func TestQueryEvidenceNilRequest(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	qs := keeper.NewQueryServerImpl(k)

	_, err := qs.QueryEvidence(ctx, nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
}

func TestQueryEvidenceEmptyID(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	qs := keeper.NewQueryServerImpl(k)

	_, err := qs.QueryEvidence(ctx, &types.QueryEvidenceRequest{Id: ""})
	if err == nil {
		t.Fatal("expected error for empty evidence ID")
	}
}

func TestQueryEvidenceBySubmitterNilRequest(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	qs := keeper.NewQueryServerImpl(k)

	_, err := qs.QueryEvidenceBySubmitter(ctx, nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
}

func TestQueryCustodyChainNilRequest(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	qs := keeper.NewQueryServerImpl(k)

	_, err := qs.QueryCustodyChain(ctx, nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
}

func TestQueryVerificationsNilRequest(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	qs := keeper.NewQueryServerImpl(k)

	_, err := qs.QueryVerifications(ctx, nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
}

func TestQueryParamsNilRequest(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	qs := keeper.NewQueryServerImpl(k)

	_, err := qs.QueryParams(ctx, nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
}

// ========== ValidateBasic ==========

func TestMsgSubmitEvidenceValidateBasic(t *testing.T) {
	validSubmitter := testAddr("alice")

	tests := []struct {
		name    string
		makeMsg func() *types.MsgSubmitEvidence
		wantErr bool
	}{
		{"valid", func() *types.MsgSubmitEvidence {
			return &types.MsgSubmitEvidence{Submitter: validSubmitter, EvidenceType: types.EvidenceType_EVIDENCE_TYPE_DOCUMENT, ContentHash: "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"}
		}, false},
		{"invalid content hash (not hex)", func() *types.MsgSubmitEvidence {
			return &types.MsgSubmitEvidence{Submitter: validSubmitter, EvidenceType: types.EvidenceType_EVIDENCE_TYPE_DOCUMENT, ContentHash: "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"}
		}, true},
		{"invalid content hash (wrong length)", func() *types.MsgSubmitEvidence {
			return &types.MsgSubmitEvidence{Submitter: validSubmitter, EvidenceType: types.EvidenceType_EVIDENCE_TYPE_DOCUMENT, ContentHash: "abcd1234"}
		}, true},
		{"empty submitter", func() *types.MsgSubmitEvidence {
			return &types.MsgSubmitEvidence{Submitter: "", EvidenceType: types.EvidenceType_EVIDENCE_TYPE_DOCUMENT, ContentHash: "hash"}
		}, true},
		{"invalid submitter address", func() *types.MsgSubmitEvidence {
			return &types.MsgSubmitEvidence{Submitter: "invalid", EvidenceType: types.EvidenceType_EVIDENCE_TYPE_DOCUMENT, ContentHash: "hash"}
		}, true},
		{"unspecified evidence type", func() *types.MsgSubmitEvidence {
			return &types.MsgSubmitEvidence{Submitter: validSubmitter, EvidenceType: types.EvidenceType_EVIDENCE_TYPE_UNSPECIFIED, ContentHash: "hash"}
		}, true},
		{"empty content hash", func() *types.MsgSubmitEvidence {
			return &types.MsgSubmitEvidence{Submitter: validSubmitter, EvidenceType: types.EvidenceType_EVIDENCE_TYPE_DOCUMENT, ContentHash: ""}
		}, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.makeMsg().ValidateBasic()
			if tc.wantErr && err == nil {
				t.Error("expected error")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestMsgTransferCustodyValidateBasic(t *testing.T) {
	alice := testAddr("alice")
	bob := testAddr("bob")

	tests := []struct {
		name    string
		makeMsg func() *types.MsgTransferCustody
		wantErr bool
	}{
		{"valid", func() *types.MsgTransferCustody {
			return &types.MsgTransferCustody{CurrentCustodian: alice, EvidenceId: "evid-1", NewCustodian: bob}
		}, false},
		{"empty current_custodian", func() *types.MsgTransferCustody {
			return &types.MsgTransferCustody{CurrentCustodian: "", EvidenceId: "evid-1", NewCustodian: bob}
		}, true},
		{"invalid current_custodian", func() *types.MsgTransferCustody {
			return &types.MsgTransferCustody{CurrentCustodian: "invalid", EvidenceId: "evid-1", NewCustodian: bob}
		}, true},
		{"empty evidence_id", func() *types.MsgTransferCustody {
			return &types.MsgTransferCustody{CurrentCustodian: alice, EvidenceId: "", NewCustodian: bob}
		}, true},
		{"empty new_custodian", func() *types.MsgTransferCustody {
			return &types.MsgTransferCustody{CurrentCustodian: alice, EvidenceId: "evid-1", NewCustodian: ""}
		}, true},
		{"invalid new_custodian", func() *types.MsgTransferCustody {
			return &types.MsgTransferCustody{CurrentCustodian: alice, EvidenceId: "evid-1", NewCustodian: "invalid"}
		}, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.makeMsg().ValidateBasic()
			if tc.wantErr && err == nil {
				t.Error("expected error")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestMsgVerifyEvidenceValidateBasic(t *testing.T) {
	valid := testAddr("verifier")

	tests := []struct {
		name    string
		makeMsg func() *types.MsgVerifyEvidence
		wantErr bool
	}{
		{"valid", func() *types.MsgVerifyEvidence {
			return &types.MsgVerifyEvidence{Verifier: valid, EvidenceId: "evid-1", Outcome: true, Confidence: 500000}
		}, false},
		{"empty verifier", func() *types.MsgVerifyEvidence {
			return &types.MsgVerifyEvidence{Verifier: "", EvidenceId: "evid-1"}
		}, true},
		{"invalid verifier address", func() *types.MsgVerifyEvidence {
			return &types.MsgVerifyEvidence{Verifier: "invalid", EvidenceId: "evid-1"}
		}, true},
		{"empty evidence_id", func() *types.MsgVerifyEvidence {
			return &types.MsgVerifyEvidence{Verifier: valid, EvidenceId: ""}
		}, true},
		{"confidence above max", func() *types.MsgVerifyEvidence {
			return &types.MsgVerifyEvidence{Verifier: valid, EvidenceId: "evid-1", Confidence: 1000001}
		}, true},
		{"confidence at max", func() *types.MsgVerifyEvidence {
			return &types.MsgVerifyEvidence{Verifier: valid, EvidenceId: "evid-1", Confidence: 1000000}
		}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.makeMsg().ValidateBasic()
			if tc.wantErr && err == nil {
				t.Error("expected error")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestMsgChallengeEvidenceValidateBasic(t *testing.T) {
	valid := testAddr("challenger")

	tests := []struct {
		name    string
		makeMsg func() *types.MsgChallengeEvidence
		wantErr bool
	}{
		{"valid", func() *types.MsgChallengeEvidence {
			return &types.MsgChallengeEvidence{Challenger: valid, EvidenceId: "evid-1", Reason: "suspicious", Bond: "500000"}
		}, false},
		{"empty challenger", func() *types.MsgChallengeEvidence {
			return &types.MsgChallengeEvidence{Challenger: "", EvidenceId: "evid-1", Reason: "suspicious", Bond: "500000"}
		}, true},
		{"invalid challenger", func() *types.MsgChallengeEvidence {
			return &types.MsgChallengeEvidence{Challenger: "invalid", EvidenceId: "evid-1", Reason: "suspicious", Bond: "500000"}
		}, true},
		{"empty evidence_id", func() *types.MsgChallengeEvidence {
			return &types.MsgChallengeEvidence{Challenger: valid, EvidenceId: "", Reason: "suspicious", Bond: "500000"}
		}, true},
		{"empty reason", func() *types.MsgChallengeEvidence {
			return &types.MsgChallengeEvidence{Challenger: valid, EvidenceId: "evid-1", Reason: "", Bond: "500000"}
		}, true},
		{"invalid bond", func() *types.MsgChallengeEvidence {
			return &types.MsgChallengeEvidence{Challenger: valid, EvidenceId: "evid-1", Reason: "suspicious", Bond: "abc"}
		}, true},
		{"negative bond", func() *types.MsgChallengeEvidence {
			return &types.MsgChallengeEvidence{Challenger: valid, EvidenceId: "evid-1", Reason: "suspicious", Bond: "-1"}
		}, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.makeMsg().ValidateBasic()
			if tc.wantErr && err == nil {
				t.Error("expected error")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestMsgUpdateParamsValidateBasic(t *testing.T) {
	valid := testAddr("authority")

	tests := []struct {
		name    string
		makeMsg func() *types.MsgUpdateParams
		wantErr bool
	}{
		{"valid", func() *types.MsgUpdateParams {
			return &types.MsgUpdateParams{Authority: valid, Params: types.DefaultParams()}
		}, false},
		{"empty authority", func() *types.MsgUpdateParams {
			return &types.MsgUpdateParams{Authority: "", Params: types.DefaultParams()}
		}, true},
		{"nil params", func() *types.MsgUpdateParams {
			return &types.MsgUpdateParams{Authority: valid, Params: nil}
		}, true},
		{"invalid params", func() *types.MsgUpdateParams {
			return &types.MsgUpdateParams{Authority: valid, Params: &types.Params{MinVerifierTier: 0, VerificationQuorum: 3, ChallengeBond: "500000", ChallengeWindowBlocks: 50000}}
		}, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.makeMsg().ValidateBasic()
			if tc.wantErr && err == nil {
				t.Error("expected error")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// ========== Full Lifecycle / Security ==========

func TestFullEvidenceLifecycle(t *testing.T) {
	msgSrv, k, ctx, sk, dk := setupMsgServer(t)

	submitter := testAddr("submitter")
	custodian := testAddr("custodian")
	v1 := testAddr("v1")
	v2 := testAddr("v2")
	v3 := testAddr("v3")
	challenger := testAddr("challenger")

	sk.tiers[v1] = 3
	sk.tiers[v2] = 3
	sk.tiers[v3] = 3

	// 1. Submit
	resp, err := msgSrv.SubmitEvidence(ctx, &types.MsgSubmitEvidence{
		Submitter:    submitter,
		EvidenceType: types.EvidenceType_EVIDENCE_TYPE_DOCUMENT,
		ContentHash:  "lifecycle-hash",
		Metadata:     "lifecycle test",
	})
	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}

	// 2. Transfer custody
	_, err = msgSrv.TransferCustody(ctx, &types.MsgTransferCustody{
		EvidenceId:       resp.EvidenceId,
		CurrentCustodian: submitter,
		NewCustodian:     custodian,
		Notes:            "for analysis",
	})
	if err != nil {
		t.Fatalf("transfer custody failed: %v", err)
	}

	// 3. Verify (3 verifiers → quorum met → VERIFIED)
	for _, v := range []string{v1, v2, v3} {
		_, err = msgSrv.VerifyEvidence(ctx, &types.MsgVerifyEvidence{
			EvidenceId: resp.EvidenceId,
			Verifier:   v,
			Outcome:    true,
			Confidence: 850000,
			Method:     "review",
		})
		if err != nil {
			t.Fatalf("verify failed: %v", err)
		}
	}

	evidence, _ := k.GetEvidence(ctx, resp.EvidenceId)
	if evidence.Status != types.EvidenceStatus_EVIDENCE_STATUS_VERIFIED {
		t.Errorf("expected VERIFIED after quorum, got %s", evidence.Status.String())
	}

	// 4. Challenge (even verified evidence can be challenged within window)
	_, err = msgSrv.ChallengeEvidence(ctx, &types.MsgChallengeEvidence{
		EvidenceId: resp.EvidenceId,
		Challenger: challenger,
		Reason:     "new contradicting evidence found",
		Bond:       "500000",
	})
	if err != nil {
		t.Fatalf("challenge failed: %v", err)
	}

	evidence, _ = k.GetEvidence(ctx, resp.EvidenceId)
	if evidence.Status != types.EvidenceStatus_EVIDENCE_STATUS_CHALLENGED {
		t.Errorf("expected CHALLENGED, got %s", evidence.Status.String())
	}

	// Verify full chain of custody
	if len(evidence.ChainOfCustody) != 3 { // submit + transfer + challenge
		t.Errorf("expected 3 custody entries, got %d", len(evidence.ChainOfCustody))
	}

	// Verify dispute was bridged
	if _, found := dk.disputes[resp.EvidenceId]; !found {
		t.Error("expected dispute to be created")
	}

	// Verify verifications are preserved
	verifications := k.GetVerificationsByEvidence(ctx, resp.EvidenceId)
	if len(verifications) != 3 {
		t.Errorf("expected 3 verifications preserved, got %d", len(verifications))
	}
}

func TestMultipleEvidencesSameSubmitter(t *testing.T) {
	msgSrv, k, ctx, _, _ := setupMsgServer(t)

	submitter := testAddr("prolific")

	for i := 0; i < 10; i++ {
		_, err := msgSrv.SubmitEvidence(ctx, &types.MsgSubmitEvidence{
			Submitter:    submitter,
			EvidenceType: types.EvidenceType_EVIDENCE_TYPE_DOCUMENT,
			ContentHash:  fmt.Sprintf("hash-%d", i),
		})
		if err != nil {
			t.Fatalf("SubmitEvidence %d failed: %v", i, err)
		}
	}

	bySubmitter := k.GetEvidenceBySubmitter(ctx, submitter)
	if len(bySubmitter) != 10 {
		t.Errorf("expected 10 evidences by submitter, got %d", len(bySubmitter))
	}

	all := k.GetAllEvidences(ctx)
	if len(all) != 10 {
		t.Errorf("expected 10 total evidences, got %d", len(all))
	}
}

func TestUpdateParamsAffectsVerification(t *testing.T) {
	msgSrv, _, ctx, sk, _ := setupMsgServer(t)

	submitter := testAddr("submitter")
	v := testAddr("verifier")
	sk.tiers[v] = 2 // meets default min_tier=2

	resp, _ := msgSrv.SubmitEvidence(ctx, &types.MsgSubmitEvidence{
		Submitter:    submitter,
		EvidenceType: types.EvidenceType_EVIDENCE_TYPE_DOCUMENT,
		ContentHash:  "hash1",
	})

	// Raise min tier to 5
	_, err := msgSrv.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: "zrn1authority",
		Params: &types.Params{
			MinVerifierTier:       5,
			VerificationQuorum:    3,
			ChallengeBond:         "500000",
			ChallengeWindowBlocks: 50000,
		},
	})
	if err != nil {
		t.Fatalf("UpdateParams failed: %v", err)
	}

	// Now tier-2 verifier should be rejected
	_, err = msgSrv.VerifyEvidence(ctx, &types.MsgVerifyEvidence{
		EvidenceId: resp.EvidenceId,
		Verifier:   v,
		Outcome:    true,
		Confidence: 900000,
		Method:     "review",
	})
	if err == nil {
		t.Fatal("expected error: verifier tier 2 < new min 5")
	}
}
