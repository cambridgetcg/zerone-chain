package keeper_test

import (
	"testing"

	"github.com/zerone-chain/zerone/x/evidence_mgmt/types"
)

// ========== Params ==========

func TestSetGetParams(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	custom := &types.Params{
		MinVerifierTier:       5,
		VerificationQuorum:    7,
		ChallengeBond:         "2000000",
		ChallengeWindowBlocks: 99999,
	}
	k.SetParams(ctx, custom)

	got := k.GetParams(ctx)
	if got.MinVerifierTier != 5 {
		t.Errorf("expected min_verifier_tier 5, got %d", got.MinVerifierTier)
	}
	if got.VerificationQuorum != 7 {
		t.Errorf("expected verification_quorum 7, got %d", got.VerificationQuorum)
	}
	if got.ChallengeBond != "2000000" {
		t.Errorf("expected challenge_bond 2000000, got %s", got.ChallengeBond)
	}
	if got.ChallengeWindowBlocks != 99999 {
		t.Errorf("expected challenge_window_blocks 99999, got %d", got.ChallengeWindowBlocks)
	}
}

func TestParamsValidation(t *testing.T) {
	tests := []struct {
		name    string
		params  *types.Params
		wantErr bool
	}{
		{
			name:    "valid defaults",
			params:  types.DefaultParams(),
			wantErr: false,
		},
		{
			name: "zero min_verifier_tier",
			params: &types.Params{
				MinVerifierTier:       0,
				VerificationQuorum:    3,
				ChallengeBond:         "500000",
				ChallengeWindowBlocks: 50000,
			},
			wantErr: true,
		},
		{
			name: "zero verification_quorum",
			params: &types.Params{
				MinVerifierTier:       2,
				VerificationQuorum:    0,
				ChallengeBond:         "500000",
				ChallengeWindowBlocks: 50000,
			},
			wantErr: true,
		},
		{
			name: "invalid bond (negative)",
			params: &types.Params{
				MinVerifierTier:       2,
				VerificationQuorum:    3,
				ChallengeBond:         "-1",
				ChallengeWindowBlocks: 50000,
			},
			wantErr: true,
		},
		{
			name: "invalid bond (non-numeric)",
			params: &types.Params{
				MinVerifierTier:       2,
				VerificationQuorum:    3,
				ChallengeBond:         "notanumber",
				ChallengeWindowBlocks: 50000,
			},
			wantErr: true,
		},
		{
			name: "zero challenge_window_blocks",
			params: &types.Params{
				MinVerifierTier:       2,
				VerificationQuorum:    3,
				ChallengeBond:         "500000",
				ChallengeWindowBlocks: 0,
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.params.Validate()
			if tc.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestGetParamsReturnsDefaultWhenUnset(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	// Params should return defaults (setupKeeper doesn't explicitly set params)
	got := k.GetParams(ctx)
	def := types.DefaultParams()
	if got.MinVerifierTier != def.MinVerifierTier {
		t.Errorf("expected default min_verifier_tier %d, got %d", def.MinVerifierTier, got.MinVerifierTier)
	}
}

// ========== Evidence CRUD ==========

func TestSetGetEvidence(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	evidence := &types.Evidence{
		Id:          "evid-test-1",
		Submitter:   testAddr("alice"),
		ContentHash: "hash-abc",
		Status:      types.EvidenceStatus_EVIDENCE_STATUS_SUBMITTED,
	}
	k.SetEvidence(ctx, evidence)

	got, found := k.GetEvidence(ctx, "evid-test-1")
	if !found {
		t.Fatal("evidence not found")
	}
	if got.Submitter != evidence.Submitter {
		t.Errorf("expected submitter %s, got %s", evidence.Submitter, got.Submitter)
	}
	if got.ContentHash != "hash-abc" {
		t.Errorf("expected content hash hash-abc, got %s", got.ContentHash)
	}
}

func TestGetEvidenceNotFound(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	_, found := k.GetEvidence(ctx, "nonexistent")
	if found {
		t.Error("expected not found for nonexistent evidence")
	}
}

func TestGetAllEvidences(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	for i := 0; i < 5; i++ {
		k.SetEvidence(ctx, &types.Evidence{
			Id:          testAddr(string(rune('a' + i))),
			Submitter:   testAddr("alice"),
			ContentHash: "hash",
			Status:      types.EvidenceStatus_EVIDENCE_STATUS_SUBMITTED,
		})
	}

	all := k.GetAllEvidences(ctx)
	if len(all) != 5 {
		t.Errorf("expected 5 evidences, got %d", len(all))
	}
}

func TestGetAllEvidencesEmpty(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	all := k.GetAllEvidences(ctx)
	if len(all) != 0 {
		t.Errorf("expected 0 evidences, got %d", len(all))
	}
}

func TestIterateEvidences(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	for i := 0; i < 3; i++ {
		k.SetEvidence(ctx, &types.Evidence{
			Id:          testAddr(string(rune('a' + i))),
			Submitter:   testAddr("alice"),
			ContentHash: "hash",
			Status:      types.EvidenceStatus_EVIDENCE_STATUS_SUBMITTED,
		})
	}

	count := 0
	k.IterateEvidences(ctx, func(e *types.Evidence) bool {
		count++
		return false
	})
	if count != 3 {
		t.Errorf("expected 3 iterations, got %d", count)
	}
}

func TestIterateEvidencesEarlyStop(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	for i := 0; i < 5; i++ {
		k.SetEvidence(ctx, &types.Evidence{
			Id:          testAddr(string(rune('a' + i))),
			Submitter:   testAddr("alice"),
			ContentHash: "hash",
			Status:      types.EvidenceStatus_EVIDENCE_STATUS_SUBMITTED,
		})
	}

	count := 0
	k.IterateEvidences(ctx, func(e *types.Evidence) bool {
		count++
		return count >= 2 // stop after 2
	})
	if count != 2 {
		t.Errorf("expected iteration to stop at 2, got %d", count)
	}
}

func TestGetEvidenceBySubmitter(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	alice := testAddr("alice")
	bob := testAddr("bob")

	k.SetEvidence(ctx, &types.Evidence{Id: "evid-a1", Submitter: alice, ContentHash: "h1", Status: types.EvidenceStatus_EVIDENCE_STATUS_SUBMITTED})
	k.SetEvidence(ctx, &types.Evidence{Id: "evid-a2", Submitter: alice, ContentHash: "h2", Status: types.EvidenceStatus_EVIDENCE_STATUS_SUBMITTED})
	k.SetEvidence(ctx, &types.Evidence{Id: "evid-b1", Submitter: bob, ContentHash: "h3", Status: types.EvidenceStatus_EVIDENCE_STATUS_SUBMITTED})

	aliceEvids := k.GetEvidenceBySubmitter(ctx, alice)
	if len(aliceEvids) != 2 {
		t.Errorf("expected 2 evidences for alice, got %d", len(aliceEvids))
	}

	bobEvids := k.GetEvidenceBySubmitter(ctx, bob)
	if len(bobEvids) != 1 {
		t.Errorf("expected 1 evidence for bob, got %d", len(bobEvids))
	}
}

func TestGetEvidenceBySubmitterEmpty(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	evids := k.GetEvidenceBySubmitter(ctx, testAddr("nobody"))
	if len(evids) != 0 {
		t.Errorf("expected 0 evidences for unknown submitter, got %d", len(evids))
	}
}

func TestSetEvidenceOverwrite(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	k.SetEvidence(ctx, &types.Evidence{
		Id:          "evid-1",
		Submitter:   testAddr("alice"),
		ContentHash: "original",
		Status:      types.EvidenceStatus_EVIDENCE_STATUS_SUBMITTED,
	})

	// Overwrite with new status
	k.SetEvidence(ctx, &types.Evidence{
		Id:          "evid-1",
		Submitter:   testAddr("alice"),
		ContentHash: "original",
		Status:      types.EvidenceStatus_EVIDENCE_STATUS_VERIFIED,
	})

	got, found := k.GetEvidence(ctx, "evid-1")
	if !found {
		t.Fatal("evidence not found")
	}
	if got.Status != types.EvidenceStatus_EVIDENCE_STATUS_VERIFIED {
		t.Errorf("expected VERIFIED, got %s", got.Status.String())
	}
}

// ========== Verification CRUD ==========

func TestSetGetVerification(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	v := &types.VerificationResult{
		Id:         "ver-1",
		EvidenceId: "evid-1",
		Verifier:   testAddr("verifier1"),
		Outcome:    true,
		Confidence: 900000,
		Method:     "manual",
	}
	k.SetVerification(ctx, v)

	results := k.GetVerificationsByEvidence(ctx, "evid-1")
	if len(results) != 1 {
		t.Fatalf("expected 1 verification, got %d", len(results))
	}
	if results[0].Verifier != v.Verifier {
		t.Errorf("expected verifier %s, got %s", v.Verifier, results[0].Verifier)
	}
	if results[0].Confidence != 900000 {
		t.Errorf("expected confidence 900000, got %d", results[0].Confidence)
	}
}

func TestGetVerificationsByEvidenceEmpty(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	results := k.GetVerificationsByEvidence(ctx, "nonexistent")
	if len(results) != 0 {
		t.Errorf("expected 0 verifications, got %d", len(results))
	}
}

func TestGetVerificationsByEvidenceMultiple(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	for i := 0; i < 4; i++ {
		k.SetVerification(ctx, &types.VerificationResult{
			Id:         testAddr(string(rune('a' + i))),
			EvidenceId: "evid-1",
			Verifier:   testAddr(string(rune('a' + i))),
			Outcome:    i%2 == 0,
			Confidence: uint32(500000 + i*100000),
			Method:     "review",
		})
	}

	results := k.GetVerificationsByEvidence(ctx, "evid-1")
	if len(results) != 4 {
		t.Errorf("expected 4 verifications, got %d", len(results))
	}
}

func TestGetAllVerifications(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	// Verifications for different evidences
	k.SetVerification(ctx, &types.VerificationResult{Id: "v1", EvidenceId: "evid-1", Verifier: testAddr("v1")})
	k.SetVerification(ctx, &types.VerificationResult{Id: "v2", EvidenceId: "evid-2", Verifier: testAddr("v2")})
	k.SetVerification(ctx, &types.VerificationResult{Id: "v3", EvidenceId: "evid-1", Verifier: testAddr("v3")})

	all := k.GetAllVerifications(ctx)
	if len(all) != 3 {
		t.Errorf("expected 3 total verifications, got %d", len(all))
	}
}

func TestGetAllVerificationsEmpty(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	all := k.GetAllVerifications(ctx)
	if len(all) != 0 {
		t.Errorf("expected 0 verifications, got %d", len(all))
	}
}

func TestVerificationIsolationBetweenEvidences(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	k.SetVerification(ctx, &types.VerificationResult{Id: "v1", EvidenceId: "evid-A", Verifier: testAddr("v1")})
	k.SetVerification(ctx, &types.VerificationResult{Id: "v2", EvidenceId: "evid-A", Verifier: testAddr("v2")})
	k.SetVerification(ctx, &types.VerificationResult{Id: "v3", EvidenceId: "evid-B", Verifier: testAddr("v3")})

	aResults := k.GetVerificationsByEvidence(ctx, "evid-A")
	if len(aResults) != 2 {
		t.Errorf("expected 2 verifications for evid-A, got %d", len(aResults))
	}

	bResults := k.GetVerificationsByEvidence(ctx, "evid-B")
	if len(bResults) != 1 {
		t.Errorf("expected 1 verification for evid-B, got %d", len(bResults))
	}
}

// ========== Counters ==========

func TestGetNextEvidenceIDSequential(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	id1 := k.GetNextEvidenceID(ctx)
	id2 := k.GetNextEvidenceID(ctx)
	id3 := k.GetNextEvidenceID(ctx)

	if id1 != 1 {
		t.Errorf("expected first ID 1, got %d", id1)
	}
	if id2 != 2 {
		t.Errorf("expected second ID 2, got %d", id2)
	}
	if id3 != 3 {
		t.Errorf("expected third ID 3, got %d", id3)
	}
}

func TestGetNextVerificationIDSequential(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	id1 := k.GetNextVerificationID(ctx)
	id2 := k.GetNextVerificationID(ctx)

	if id1 != 1 {
		t.Errorf("expected first ID 1, got %d", id1)
	}
	if id2 != 2 {
		t.Errorf("expected second ID 2, got %d", id2)
	}
}

func TestCounterIndependence(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	// Advance evidence counter
	_ = k.GetNextEvidenceID(ctx) // 1
	_ = k.GetNextEvidenceID(ctx) // 2
	_ = k.GetNextEvidenceID(ctx) // 3

	// Verification counter should start independently
	vID := k.GetNextVerificationID(ctx)
	if vID != 1 {
		t.Errorf("expected verification ID 1 (independent of evidence counter), got %d", vID)
	}

	eID := k.GetNextEvidenceID(ctx)
	if eID != 4 {
		t.Errorf("expected evidence ID 4, got %d", eID)
	}
}

// ========== Genesis Extended ==========

func TestGenesisRoundTripMultiple(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	genState := &types.GenesisState{
		Params: &types.Params{
			MinVerifierTier:       4,
			VerificationQuorum:    5,
			ChallengeBond:         "1000000",
			ChallengeWindowBlocks: 80000,
		},
		Evidences: []*types.Evidence{
			{Id: "evid-1", Submitter: testAddr("alice"), ContentHash: "h1", Status: types.EvidenceStatus_EVIDENCE_STATUS_SUBMITTED},
			{Id: "evid-2", Submitter: testAddr("bob"), ContentHash: "h2", Status: types.EvidenceStatus_EVIDENCE_STATUS_VERIFIED},
			{Id: "evid-3", Submitter: testAddr("alice"), ContentHash: "h3", Status: types.EvidenceStatus_EVIDENCE_STATUS_CHALLENGED},
		},
		Verifications: []*types.VerificationResult{
			{Id: "v1", EvidenceId: "evid-1", Verifier: testAddr("v1"), Outcome: true, Confidence: 800000},
			{Id: "v2", EvidenceId: "evid-2", Verifier: testAddr("v2"), Outcome: true, Confidence: 900000},
			{Id: "v3", EvidenceId: "evid-2", Verifier: testAddr("v3"), Outcome: true, Confidence: 750000},
		},
		NextEvidenceId:     10,
		NextVerificationId: 7,
	}

	k.InitGenesis(ctx, genState)
	exported := k.ExportGenesis(ctx)

	if len(exported.Evidences) != 3 {
		t.Errorf("expected 3 evidences, got %d", len(exported.Evidences))
	}
	if len(exported.Verifications) != 3 {
		t.Errorf("expected 3 verifications, got %d", len(exported.Verifications))
	}
	if exported.NextEvidenceId != 10 {
		t.Errorf("expected next_evidence_id 10, got %d", exported.NextEvidenceId)
	}
	if exported.NextVerificationId != 7 {
		t.Errorf("expected next_verification_id 7, got %d", exported.NextVerificationId)
	}
}

func TestGenesisRoundTripEmpty(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	genState := &types.GenesisState{
		Params:        types.DefaultParams(),
		Evidences:     []*types.Evidence{},
		Verifications: []*types.VerificationResult{},
	}

	k.InitGenesis(ctx, genState)
	exported := k.ExportGenesis(ctx)

	if len(exported.Evidences) != 0 {
		t.Errorf("expected 0 evidences, got %d", len(exported.Evidences))
	}
	if len(exported.Verifications) != 0 {
		t.Errorf("expected 0 verifications, got %d", len(exported.Verifications))
	}
}

func TestGenesisValidationDuplicateIDs(t *testing.T) {
	gs := &types.GenesisState{
		Params: types.DefaultParams(),
		Evidences: []*types.Evidence{
			{Id: "evid-dup", Submitter: testAddr("alice"), ContentHash: "h1"},
			{Id: "evid-dup", Submitter: testAddr("bob"), ContentHash: "h2"},
		},
	}

	err := gs.Validate()
	if err == nil {
		t.Error("expected error for duplicate evidence IDs")
	}
}

func TestGenesisValidationNilParams(t *testing.T) {
	gs := &types.GenesisState{
		Params: nil,
	}
	// nil params should pass validation (will use defaults)
	err := gs.Validate()
	if err != nil {
		t.Errorf("unexpected error with nil params: %v", err)
	}
}
