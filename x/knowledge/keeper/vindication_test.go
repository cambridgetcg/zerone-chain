package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/keeper"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

func TestVindicationPending_SetGetDelete(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	factId := "fact_abc123"

	// Initially empty
	got := k.GetVindicationPending(ctx, factId)
	require.Nil(t, got)

	// Set entries
	entries := []types.VindicationEntry{
		{
			Verifier:    "zrn1validator1",
			Vote:        "REJECT",
			SlashAmount: "5000",
			SlashBps:    50000,
			RoundId:     "round_001",
			FactId:      factId,
			Height:      100,
		},
		{
			Verifier:    "zrn1validator2",
			Vote:        "REJECT",
			SlashAmount: "3000",
			SlashBps:    30000,
			RoundId:     "round_001",
			FactId:      factId,
			Height:      100,
		},
	}
	require.NoError(t, k.SetVindicationPending(ctx, factId, entries))

	// Get them back
	got = k.GetVindicationPending(ctx, factId)
	require.Len(t, got, 2)
	require.Equal(t, "zrn1validator1", got[0].Verifier)
	require.Equal(t, "REJECT", got[0].Vote)
	require.Equal(t, "5000", got[0].SlashAmount)
	require.Equal(t, uint64(50000), got[0].SlashBps)
	require.Equal(t, "zrn1validator2", got[1].Verifier)
	require.Equal(t, "3000", got[1].SlashAmount)

	// Delete
	k.DeleteVindicationPending(ctx, factId)

	// Verify empty after delete
	got = k.GetVindicationPending(ctx, factId)
	require.Nil(t, got)
}

func TestVindicationRecord_SetGet(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	factId := "fact_xyz789"
	verifier := "zrn1validator1"

	// Non-existent record returns false
	_, found := k.GetVindicationRecord(ctx, factId, verifier)
	require.False(t, found)

	// Store a record
	record := types.VindicationRecord{
		Verifier:     verifier,
		FactId:       factId,
		RefundAmount: "5000",
		BonusAmount:  "1000",
		VindicatedAt: 500,
		DisprovenBy:  "challenge_round_002",
		RoundId:      "round_001",
	}
	require.NoError(t, k.SetVindicationRecord(ctx, factId, record))

	// Retrieve it
	got, found := k.GetVindicationRecord(ctx, factId, verifier)
	require.True(t, found)
	require.Equal(t, verifier, got.Verifier)
	require.Equal(t, factId, got.FactId)
	require.Equal(t, "5000", got.RefundAmount)
	require.Equal(t, "1000", got.BonusAmount)
	require.Equal(t, uint64(500), got.VindicatedAt)
	require.Equal(t, "challenge_round_002", got.DisprovenBy)
	require.Equal(t, "round_001", got.RoundId)

	// Different verifier returns false
	_, found = k.GetVindicationRecord(ctx, factId, "zrn1validator99")
	require.False(t, found)
}

func TestVindicationPending_GetAllPending(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	factId1 := "fact_aaa"
	factId2 := "fact_bbb"

	entries1 := []types.VindicationEntry{
		{
			Verifier:    "zrn1validator1",
			Vote:        "REJECT",
			SlashAmount: "5000",
			SlashBps:    50000,
			RoundId:     "round_001",
			FactId:      factId1,
			Height:      100,
		},
	}
	entries2 := []types.VindicationEntry{
		{
			Verifier:    "zrn1validator3",
			Vote:        "REJECT",
			SlashAmount: "8000",
			SlashBps:    80000,
			RoundId:     "round_002",
			FactId:      factId2,
			Height:      200,
		},
		{
			Verifier:    "zrn1validator4",
			Vote:        "REJECT",
			SlashAmount: "2000",
			SlashBps:    20000,
			RoundId:     "round_002",
			FactId:      factId2,
			Height:      200,
		},
	}

	require.NoError(t, k.SetVindicationPending(ctx, factId1, entries1))
	require.NoError(t, k.SetVindicationPending(ctx, factId2, entries2))

	// GetAll returns both
	all := k.GetAllVindicationPending(ctx)
	require.Len(t, all, 2)

	require.Contains(t, all, factId1)
	require.Contains(t, all, factId2)
	require.Len(t, all[factId1], 1)
	require.Len(t, all[factId2], 2)
	require.Equal(t, "zrn1validator1", all[factId1][0].Verifier)
	require.Equal(t, "zrn1validator3", all[factId2][0].Verifier)
	require.Equal(t, "zrn1validator4", all[factId2][1].Verifier)
}

// ─── Integration Tests ──────────────────────────────────────────────────────

// completeRoundWithMinority is a helper that creates a claim, round, commits,
// reveals (with a minority voter who votes "reject" while majority votes "accept"),
// aggregates the result, and calls CompleteRound. Returns the claim, round, result,
// and the factId created by the ACCEPT verdict.
func completeRoundWithMinority(
	t *testing.T,
	k keeper.Keeper,
	ctx sdk.Context,
	sk *trackingStakingKeeper,
	claimID, domain string,
	majorityAddrs []string,
	minorityAddrs []string,
) (claim *types.Claim, round *types.VerificationRound, result *keeper.VerificationResult, factId string) {
	t.Helper()

	// Register validators
	for _, addr := range majorityAddrs {
		sk.addValidator(addr, 100_000, "bonded")
	}
	for _, addr := range minorityAddrs {
		sk.addValidator(addr, 100_000, "bonded")
	}

	claim = &types.Claim{
		Id:          claimID,
		FactContent: "Test claim content for vindication: " + claimID,
		Domain:      domain,
		Category:    "formal",
		Submitter:   "zrn1submitter1",
		Stake:       "1000000",
		Status:      types.ClaimStatus_CLAIM_STATUS_IN_VERIFICATION,
	}
	require.NoError(t, k.SetClaim(ctx, claim))

	height := uint64(ctx.BlockHeight())
	roundID := keeper.GenerateRoundID(claimID, height)

	round = &types.VerificationRound{
		Id:                  roundID,
		ClaimId:             claimID,
		StartedAtBlock:      height,
		Phase:               types.VerificationPhase_VERIFICATION_PHASE_AGGREGATION,
		CommitDeadline:      height + 200,
		RevealDeadline:      height + 400,
		AggregationDeadline: height + 450,
	}

	// Add commits and reveals for majority (vote "accept")
	for _, addr := range majorityAddrs {
		round.Commits = append(round.Commits, &types.CommitEntry{
			Verifier:         addr,
			CommitHash:       []byte("hash_" + addr),
			CommittedAtBlock: height,
		})
		round.Reveals = append(round.Reveals, &types.RevealEntry{
			Verifier:        addr,
			Vote:            "accept",
			Salt:            []byte("salt_" + addr),
			RevealedAtBlock: height,
		})
	}

	// Add commits and reveals for minority (vote "reject")
	for _, addr := range minorityAddrs {
		round.Commits = append(round.Commits, &types.CommitEntry{
			Verifier:         addr,
			CommitHash:       []byte("hash_" + addr),
			CommittedAtBlock: height,
		})
		round.Reveals = append(round.Reveals, &types.RevealEntry{
			Verifier:        addr,
			Vote:            "reject",
			Salt:            []byte("salt_" + addr),
			RevealedAtBlock: height,
		})
	}

	require.NoError(t, k.SetVerificationRound(ctx, round))

	// Aggregate
	result, err := k.AggregateVerificationResult(ctx, round)
	require.NoError(t, err)
	require.Equal(t, types.Verdict_VERDICT_ACCEPT, result.Verdict, "majority should win with ACCEPT")

	// Complete the round (creates fact, processes slashes, etc.)
	require.NoError(t, k.CompleteRound(ctx, round, result))

	// Find the factId created by CompleteRound
	expectedFactId := keeper.GenerateFactID(claimID, height)
	_, found := k.GetFact(ctx, expectedFactId)
	require.True(t, found, "fact should exist after ACCEPT verdict")
	factId = expectedFactId

	return claim, round, result, factId
}

// TestVindication_FullLifecycle tests the complete vindication flow:
// original round with minority voter -> challenge succeeds -> vindication fires.
func TestVindication_FullLifecycle(t *testing.T) {
	k, ctx, _, sk := setupKnowledgeTestFull(t)

	// Need 4 majority + 1 minority = 80% accept ratio > 77% threshold
	majorityAddrs := []string{
		makeValidatorAddr(1), makeValidatorAddr(2),
		makeValidatorAddr(3), makeValidatorAddr(4),
	}
	minorityAddrs := []string{makeValidatorAddr(5)}

	// Step 1-2: Create a claim and complete the round with ACCEPT verdict.
	// The minority voter (validator5) voted "reject" and gets slashed.
	_, _, _, factId := completeRoundWithMinority(
		t, k, ctx, sk,
		"claim-vindication-lifecycle", "mathematics",
		majorityAddrs, minorityAddrs,
	)

	// Step 3: Verify VindicationPending entry exists for the created fact
	pending := k.GetVindicationPending(ctx, factId)
	require.NotNil(t, pending, "vindication pending entries should exist for the fact")
	require.Len(t, pending, 1, "exactly one minority voter should have a pending entry")
	require.Equal(t, makeValidatorAddr(5), pending[0].Verifier)
	require.Equal(t, "reject", pending[0].Vote)

	// Record initial slash count (slashes from original round)
	initialSlashCount := len(sk.slashes)

	// Step 4: Create a challenge claim with ProvisionalFactId pointing to the created fact
	challengeClaimID := "claim-challenge-lifecycle"
	challengeClaim := &types.Claim{
		Id:                challengeClaimID,
		FactContent:       "Challenge: the original fact is wrong: " + challengeClaimID,
		Domain:            "mathematics", // must match original fact's domain
		Category:          "formal",
		Submitter:         "zrn1challenger1",
		Stake:             "1000000",
		Status:            types.ClaimStatus_CLAIM_STATUS_IN_VERIFICATION,
		ProvisionalFactId: factId,
	}
	require.NoError(t, k.SetClaim(ctx, challengeClaim))

	// Step 5: Mark the original fact as CHALLENGED
	originalFact, found := k.GetFact(ctx, factId)
	require.True(t, found)
	originalFact.Status = types.FactStatus_FACT_STATUS_CHALLENGED
	require.NoError(t, k.SetFact(ctx, originalFact))

	// Step 6: Complete the challenge round with ACCEPT verdict
	// (meaning the challenge is valid -- the original fact is wrong)
	challengeHeight := uint64(ctx.BlockHeight())
	challengeRoundID := keeper.GenerateRoundID(challengeClaimID, challengeHeight)
	challengeRound := &types.VerificationRound{
		Id:                  challengeRoundID,
		ClaimId:             challengeClaimID,
		StartedAtBlock:      challengeHeight,
		Phase:               types.VerificationPhase_VERIFICATION_PHASE_AGGREGATION,
		CommitDeadline:      challengeHeight + 200,
		RevealDeadline:      challengeHeight + 400,
		AggregationDeadline: challengeHeight + 450,
	}

	// All 3 challenge verifiers accept the challenge claim
	challengeVerifiers := []string{makeValidatorAddr(6), makeValidatorAddr(7), makeValidatorAddr(8)}
	for _, addr := range challengeVerifiers {
		sk.addValidator(addr, 100_000, "bonded")
		challengeRound.Commits = append(challengeRound.Commits, &types.CommitEntry{
			Verifier:         addr,
			CommitHash:       []byte("chash_" + addr),
			CommittedAtBlock: challengeHeight,
		})
		challengeRound.Reveals = append(challengeRound.Reveals, &types.RevealEntry{
			Verifier:        addr,
			Vote:            "accept",
			Salt:            []byte("csalt_" + addr),
			RevealedAtBlock: challengeHeight,
		})
	}
	require.NoError(t, k.SetVerificationRound(ctx, challengeRound))

	challengeResult := &keeper.VerificationResult{
		Verdict:    types.Verdict_VERDICT_ACCEPT,
		Confidence: 1_000_000,
		Rewards: []keeper.VerifierReward{
			{Verifier: makeValidatorAddr(6), Amount: 3_000_000},
			{Verifier: makeValidatorAddr(7), Amount: 3_000_000},
			{Verifier: makeValidatorAddr(8), Amount: 3_000_000},
		},
	}

	require.NoError(t, k.CompleteRound(ctx, challengeRound, challengeResult))

	// Step 7: Verify original fact status is DISPROVEN
	disprovenFact, found := k.GetFact(ctx, factId)
	require.True(t, found)
	require.Equal(t, types.FactStatus_FACT_STATUS_DISPROVEN, disprovenFact.Status,
		"original fact should be DISPROVEN after challenge succeeds")

	// Step 8: Verify VindicationPending is cleared
	pendingAfter := k.GetVindicationPending(ctx, factId)
	require.Nil(t, pendingAfter, "vindication pending should be cleared after vindication executes")

	// Step 9: Verify VindicationRecord exists for the minority voter
	record, found := k.GetVindicationRecord(ctx, factId, makeValidatorAddr(5))
	require.True(t, found, "vindication record should exist for the minority voter")
	require.Equal(t, makeValidatorAddr(5), record.Verifier)
	require.Equal(t, factId, record.FactId)
	require.NotEmpty(t, record.RefundAmount)
	require.NotEmpty(t, record.RoundId)

	// Step 10: Verify the tracking staking keeper recorded the majority vindication slash
	// The original round created 1 minority slash (to escrow).
	// The vindication should have slashed the 3 majority voters.
	majoritySlashCount := 0
	for i := initialSlashCount; i < len(sk.slashes); i++ {
		slash := sk.slashes[i]
		for _, addr := range majorityAddrs {
			if slash.Validator == addr {
				majoritySlashCount++
			}
		}
	}
	require.Equal(t, 4, majoritySlashCount,
		"all 4 majority voters should be slashed during vindication")
}

// TestVindication_ChallengeFails_NoVindication tests that when a challenge is
// rejected, the original fact is restored to ACTIVE and vindication is NOT triggered.
func TestVindication_ChallengeFails_NoVindication(t *testing.T) {
	k, ctx, _, sk := setupKnowledgeTestFull(t)

	// Step 1: Create a fact with pending vindication entries set directly
	factId := "fact_challenge_fails"
	fact := makeTestFact(t, k, ctx, factId, "A fact that will survive challenge",
		"physics", "empirical", "zrn1submitter1", 900_000)

	// Manually set fact status to CHALLENGED
	fact.Status = types.FactStatus_FACT_STATUS_CHALLENGED
	require.NoError(t, k.SetFact(ctx, fact))

	// Set up pending vindication entries (as if a minority was slashed in the original round)
	originalRoundID := "round_original_challenge_fails"
	pendingEntries := []types.VindicationEntry{
		{
			Verifier:    makeValidatorAddr(1),
			Vote:        "reject",
			SlashAmount: "5000",
			SlashBps:    50_000,
			RoundId:     originalRoundID,
			FactId:      factId,
			Height:      100,
		},
	}
	require.NoError(t, k.SetVindicationPending(ctx, factId, pendingEntries))

	// Also store the original round so ExecuteVindication can find it
	originalRound := makeRoundInPhase(originalRoundID, "claim_original",
		types.VerificationPhase_VERIFICATION_PHASE_COMPLETE, 80)
	originalRound.Reveals = []*types.RevealEntry{
		{Verifier: makeValidatorAddr(1), Vote: "reject"},
		{Verifier: makeValidatorAddr(2), Vote: "accept"},
		{Verifier: makeValidatorAddr(3), Vote: "accept"},
		{Verifier: makeValidatorAddr(4), Vote: "accept"},
	}
	require.NoError(t, k.SetVerificationRound(ctx, originalRound))

	// Step 2: Create a challenge claim with ProvisionalFactId
	challengeClaimID := "claim-challenge-rejected"
	challengeClaim := &types.Claim{
		Id:                challengeClaimID,
		FactContent:       "Challenge that will fail: " + challengeClaimID,
		Domain:            "physics",
		Category:          "empirical",
		Submitter:         "zrn1challenger1",
		Stake:             "1000000",
		Status:            types.ClaimStatus_CLAIM_STATUS_IN_VERIFICATION,
		ProvisionalFactId: factId,
	}
	require.NoError(t, k.SetClaim(ctx, challengeClaim))

	// Step 3: Complete the challenge round with REJECT verdict
	challengeHeight := uint64(ctx.BlockHeight())
	challengeRoundID := keeper.GenerateRoundID(challengeClaimID, challengeHeight)
	challengeRound := &types.VerificationRound{
		Id:                  challengeRoundID,
		ClaimId:             challengeClaimID,
		StartedAtBlock:      challengeHeight,
		Phase:               types.VerificationPhase_VERIFICATION_PHASE_AGGREGATION,
		CommitDeadline:      challengeHeight + 200,
		RevealDeadline:      challengeHeight + 400,
		AggregationDeadline: challengeHeight + 450,
	}

	// All verifiers reject the challenge (meaning the challenge is invalid)
	challengeVerifiers := []string{makeValidatorAddr(5), makeValidatorAddr(6), makeValidatorAddr(7)}
	for _, addr := range challengeVerifiers {
		sk.addValidator(addr, 100_000, "bonded")
		challengeRound.Commits = append(challengeRound.Commits, &types.CommitEntry{
			Verifier:         addr,
			CommitHash:       []byte("chash_" + addr),
			CommittedAtBlock: challengeHeight,
		})
		challengeRound.Reveals = append(challengeRound.Reveals, &types.RevealEntry{
			Verifier:        addr,
			Vote:            "reject",
			Salt:            []byte("csalt_" + addr),
			RevealedAtBlock: challengeHeight,
		})
	}
	require.NoError(t, k.SetVerificationRound(ctx, challengeRound))

	challengeResult := &keeper.VerificationResult{
		Verdict:    types.Verdict_VERDICT_REJECT,
		Confidence: 1_000_000,
	}

	require.NoError(t, k.CompleteRound(ctx, challengeRound, challengeResult))

	// Step 4: Verify original fact is restored to ACTIVE
	restoredFact, found := k.GetFact(ctx, factId)
	require.True(t, found)
	require.Equal(t, types.FactStatus_FACT_STATUS_ACTIVE, restoredFact.Status,
		"original fact should be restored to ACTIVE when challenge is rejected")

	// Step 5: Verify VindicationPending entries still exist (not triggered)
	stillPending := k.GetVindicationPending(ctx, factId)
	require.NotNil(t, stillPending, "vindication pending entries should still exist")
	require.Len(t, stillPending, 1)
	require.Equal(t, makeValidatorAddr(1), stillPending[0].Verifier)

	// Step 6: Verify no VindicationRecords created
	records := k.GetVindicationRecordsForFact(ctx, factId)
	require.Empty(t, records, "no vindication records should be created when challenge fails")
}

// TestVindication_DisabledParam tests that when VindicationRefundEnabled is false,
// minority slashes go through regular SlashValidator (not to escrow) and no
// VindicationPending entries are created.
func TestVindication_DisabledParam(t *testing.T) {
	k, ctx, _, sk := setupKnowledgeTestFull(t)

	// Step 1: Set VindicationRefundEnabled = false
	params, err := k.GetParams(ctx)
	require.NoError(t, err)
	params.VindicationRefundEnabled = false
	require.NoError(t, k.SetParams(ctx, params))

	// Need 4 majority + 1 minority = 80% accept ratio > 77% threshold
	majorityAddrs := []string{
		makeValidatorAddr(1), makeValidatorAddr(2),
		makeValidatorAddr(3), makeValidatorAddr(4),
	}
	minorityAddrs := []string{makeValidatorAddr(5)}

	// Step 2-3: Create a claim and round with minority voter, complete with ACCEPT
	_, _, _, factId := completeRoundWithMinority(
		t, k, ctx, sk,
		"claim-vindication-disabled", "mathematics",
		majorityAddrs, minorityAddrs,
	)

	// Step 4: Verify no VindicationPending entries exist
	pending := k.GetVindicationPending(ctx, factId)
	require.Nil(t, pending,
		"no vindication pending entries should exist when VindicationRefundEnabled is false")

	// The minority voter should still have been slashed (via regular SlashValidator)
	var minoritySlashed bool
	for _, slash := range sk.slashes {
		if slash.Validator == makeValidatorAddr(5) {
			minoritySlashed = true
			break
		}
	}
	require.True(t, minoritySlashed,
		"minority voter should still be slashed via regular SlashValidator")
}

// TestPruneExpiredVindications tests that expired vindication entries are pruned
// based on the block height window.
func TestPruneExpiredVindications(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Step 1: Create VindicationPending entries at different heights
	factIdOld := "fact_old_prune"
	factIdRecent := "fact_recent_prune"

	entriesOld := []types.VindicationEntry{
		{
			Verifier:    makeValidatorAddr(1),
			Vote:        "reject",
			SlashAmount: "5000",
			SlashBps:    50_000,
			RoundId:     "round_old",
			FactId:      factIdOld,
			Height:      100, // old height
		},
	}
	entriesRecent := []types.VindicationEntry{
		{
			Verifier:    makeValidatorAddr(2),
			Vote:        "reject",
			SlashAmount: "3000",
			SlashBps:    30_000,
			RoundId:     "round_recent",
			FactId:      factIdRecent,
			Height:      90_000, // recent height
		},
	}

	require.NoError(t, k.SetVindicationPending(ctx, factIdOld, entriesOld))
	require.NoError(t, k.SetVindicationPending(ctx, factIdRecent, entriesRecent))

	// Verify both exist before pruning
	allBefore := k.GetAllVindicationPending(ctx)
	require.Len(t, allBefore, 2)

	// Step 2: Call PruneExpiredVindications at height 100101 with window 100000
	// Entry at height 100: 100101 - 100 = 100001 > 100000 → expired
	// Entry at height 90000: 100101 - 90000 = 10101 ≤ 100000 → NOT expired
	pruneCtx := ctx.WithBlockHeight(100_101)
	k.PruneExpiredVindications(pruneCtx, 100_101, 100_000)

	// Step 3: Verify entry at height 100 is pruned
	prunedEntries := k.GetVindicationPending(ctx, factIdOld)
	require.Nil(t, prunedEntries, "old entry (height 100) should be pruned")

	// Step 4: Verify entry at height 90000 is NOT pruned
	keptEntries := k.GetVindicationPending(ctx, factIdRecent)
	require.NotNil(t, keptEntries, "recent entry (height 90000) should NOT be pruned")
	require.Len(t, keptEntries, 1)
	require.Equal(t, makeValidatorAddr(2), keptEntries[0].Verifier)
}

// TestVindication_MultiplePendingEntries tests that when multiple minority voters
// are slashed, all get VindicationPending entries and all are vindicated when
// the challenge succeeds.
func TestVindication_MultiplePendingEntries(t *testing.T) {
	k, ctx, _, sk := setupKnowledgeTestFull(t)

	// Need accept ratio > 77%. Use 8 majority + 2 minority = 80% accept ratio.
	majorityAddrs := []string{
		makeValidatorAddr(1), makeValidatorAddr(2),
		makeValidatorAddr(3), makeValidatorAddr(4),
		makeValidatorAddr(5), makeValidatorAddr(6),
		makeValidatorAddr(7), makeValidatorAddr(8),
	}
	minorityAddrs := []string{
		makeValidatorAddr(9), makeValidatorAddr(10),
	}

	// Step 1-2: Create and complete round. Both minority voters get slashed.
	_, _, _, factId := completeRoundWithMinority(
		t, k, ctx, sk,
		"claim-multi-minority", "mathematics",
		majorityAddrs, minorityAddrs,
	)

	// Step 3: Verify both minority voters have VindicationPending entries
	pending := k.GetVindicationPending(ctx, factId)
	require.NotNil(t, pending)
	require.Len(t, pending, 2, "both minority voters should have pending entries")

	verifiers := make(map[string]bool)
	for _, entry := range pending {
		verifiers[entry.Verifier] = true
		require.Equal(t, "reject", entry.Vote)
	}
	require.True(t, verifiers[makeValidatorAddr(9)], "validator9 should have pending entry")
	require.True(t, verifiers[makeValidatorAddr(10)], "validator10 should have pending entry")

	// Step 4: Challenge succeeds — disprove the original fact
	initialSlashCount := len(sk.slashes)

	// Mark fact as CHALLENGED
	fact, found := k.GetFact(ctx, factId)
	require.True(t, found)
	fact.Status = types.FactStatus_FACT_STATUS_CHALLENGED
	require.NoError(t, k.SetFact(ctx, fact))

	// Create and complete challenge claim
	challengeClaimID := "claim-challenge-multi"
	challengeClaim := &types.Claim{
		Id:                challengeClaimID,
		FactContent:       "Challenge to multi-minority fact: " + challengeClaimID,
		Domain:            "mathematics",
		Category:          "formal",
		Submitter:         "zrn1challenger2",
		Stake:             "1000000",
		Status:            types.ClaimStatus_CLAIM_STATUS_IN_VERIFICATION,
		ProvisionalFactId: factId,
	}
	require.NoError(t, k.SetClaim(ctx, challengeClaim))

	challengeHeight := uint64(ctx.BlockHeight())
	challengeRoundID := keeper.GenerateRoundID(challengeClaimID, challengeHeight)
	challengeRound := &types.VerificationRound{
		Id:                  challengeRoundID,
		ClaimId:             challengeClaimID,
		StartedAtBlock:      challengeHeight,
		Phase:               types.VerificationPhase_VERIFICATION_PHASE_AGGREGATION,
		CommitDeadline:      challengeHeight + 200,
		RevealDeadline:      challengeHeight + 400,
		AggregationDeadline: challengeHeight + 450,
	}

	challengeVerifiers := []string{makeValidatorAddr(11), makeValidatorAddr(12), makeValidatorAddr(13)}
	for _, addr := range challengeVerifiers {
		sk.addValidator(addr, 100_000, "bonded")
		challengeRound.Commits = append(challengeRound.Commits, &types.CommitEntry{
			Verifier:         addr,
			CommitHash:       []byte("chash_" + addr),
			CommittedAtBlock: challengeHeight,
		})
		challengeRound.Reveals = append(challengeRound.Reveals, &types.RevealEntry{
			Verifier:        addr,
			Vote:            "accept",
			Salt:            []byte("csalt_" + addr),
			RevealedAtBlock: challengeHeight,
		})
	}
	require.NoError(t, k.SetVerificationRound(ctx, challengeRound))

	challengeResult := &keeper.VerificationResult{
		Verdict:    types.Verdict_VERDICT_ACCEPT,
		Confidence: 1_000_000,
		Rewards: []keeper.VerifierReward{
			{Verifier: makeValidatorAddr(11), Amount: 3_000_000},
			{Verifier: makeValidatorAddr(12), Amount: 3_000_000},
			{Verifier: makeValidatorAddr(13), Amount: 3_000_000},
		},
	}

	require.NoError(t, k.CompleteRound(ctx, challengeRound, challengeResult))

	// Step 5: Verify both minority voters get VindicationRecords
	records := k.GetVindicationRecordsForFact(ctx, factId)
	require.Len(t, records, 2, "both minority voters should get vindication records")

	recordVerifiers := make(map[string]bool)
	for _, rec := range records {
		recordVerifiers[rec.Verifier] = true
		require.Equal(t, factId, rec.FactId)
		require.NotEmpty(t, rec.RefundAmount)
		require.NotEmpty(t, rec.RoundId)
	}
	require.True(t, recordVerifiers[makeValidatorAddr(9)], "validator9 should have vindication record")
	require.True(t, recordVerifiers[makeValidatorAddr(10)], "validator10 should have vindication record")

	// Verify VindicationPending is cleared
	pendingAfter := k.GetVindicationPending(ctx, factId)
	require.Nil(t, pendingAfter, "vindication pending should be cleared after vindication")

	// Verify original fact is DISPROVEN
	disprovenFact, found := k.GetFact(ctx, factId)
	require.True(t, found)
	require.Equal(t, types.FactStatus_FACT_STATUS_DISPROVEN, disprovenFact.Status)

	// Verify majority voters were slashed during vindication
	majoritySlashCount := 0
	for i := initialSlashCount; i < len(sk.slashes); i++ {
		for _, addr := range majorityAddrs {
			if sk.slashes[i].Validator == addr {
				majoritySlashCount++
			}
		}
	}
	require.Equal(t, 8, majoritySlashCount,
		"all 8 majority voters should be slashed during vindication")
}
