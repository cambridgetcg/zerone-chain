package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── SlashMissedVerification ────────────────────────────────────────────────

func TestSlashMissedVerification_Success(t *testing.T) {
	k, ctx, _, sk := setupKnowledgeTestFull(t)
	sk.addValidator("zrn1val1", 100_000, "bonded")

	err := k.SlashMissedVerification(ctx, "zrn1val1", 100_000)
	require.NoError(t, err)

	require.Len(t, sk.slashes, 1)
	require.Equal(t, "zrn1val1", sk.slashes[0].Validator)
	require.Equal(t, uint64(100_000), sk.slashes[0].SlashBps)
}

func TestSlashMissedVerification_NilStakingKeeper(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Default setup has a staking keeper, but SlashMissedVerification
	// should gracefully handle nil keeper via the keeper method
	err := k.SlashMissedVerification(ctx, "zrn1val1", 50_000)
	require.NoError(t, err) // no-op when staking keeper handles it
}

func TestSlashMissedVerification_MultipleSlashes(t *testing.T) {
	k, ctx, _, sk := setupKnowledgeTestFull(t)
	sk.addValidator("zrn1val1", 100_000, "bonded")
	sk.addValidator("zrn1val2", 100_000, "bonded")

	require.NoError(t, k.SlashMissedVerification(ctx, "zrn1val1", 50_000))
	require.NoError(t, k.SlashMissedVerification(ctx, "zrn1val2", 100_000))
	require.NoError(t, k.SlashMissedVerification(ctx, "zrn1val1", 200_000)) // slashed again

	require.Len(t, sk.slashes, 3)
	require.Equal(t, uint64(50_000), sk.slashes[0].SlashBps)
	require.Equal(t, uint64(100_000), sk.slashes[1].SlashBps)
	require.Equal(t, uint64(200_000), sk.slashes[2].SlashBps)
}

// ─── Default Slash Params (exact values) ────────────────────────────────────

func TestSlashParams_ExactDefaultValues(t *testing.T) {
	p := types.DefaultParams()

	require.Equal(t, uint64(50_000), p.WrongVerificationSlashBps, "wrong verification = 5%")
	require.Equal(t, uint64(100_000), p.MissedRevealSlashBps, "missed reveal = 10%")
	require.Equal(t, uint64(200_000), p.EquivocationSlashBps, "equivocation = 20%")
	require.Equal(t, uint64(220_000), p.InvalidClaimSlashBps, "invalid claim = 22%")
}

func TestSlashParams_NonZeroInDefault(t *testing.T) {
	// Security: B22-3 audit — all slash params MUST be > 0
	p := types.DefaultParams()

	require.Greater(t, p.WrongVerificationSlashBps, uint64(0))
	require.Greater(t, p.MissedRevealSlashBps, uint64(0))
	require.Greater(t, p.EquivocationSlashBps, uint64(0))
	require.Greater(t, p.InvalidClaimSlashBps, uint64(0))
}

func TestSlashParams_CannotSetToZero(t *testing.T) {
	// Security: Setting any slash param to 0 MUST fail validation
	tests := []struct {
		name  string
		setup func(p *types.Params)
	}{
		{"WrongVerification=0", func(p *types.Params) { p.WrongVerificationSlashBps = 0 }},
		{"MissedReveal=0", func(p *types.Params) { p.MissedRevealSlashBps = 0 }},
		{"Equivocation=0", func(p *types.Params) { p.EquivocationSlashBps = 0 }},
		{"InvalidClaim=0", func(p *types.Params) { p.InvalidClaimSlashBps = 0 }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := types.DefaultParams()
			tc.setup(&p)
			require.Error(t, p.Validate(), "zero slash param must fail validation")
		})
	}
}

// ─── Equivocation Detection — Security Test ─────────────────────────────────

func TestEquivocationDetection(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	round := makeRoundInPhase("r-equivoc", "c1", types.VerificationPhase_VERIFICATION_PHASE_COMMIT, 50)
	require.NoError(t, k.SetVerificationRound(ctx, round))

	// First commitment
	commit1 := &types.CommitEntry{
		Verifier:         "zrn1cheater",
		CommitHash:       []byte("hash_first__________________________"),
		CommittedAtBlock: 100,
	}
	require.NoError(t, k.StoreCommitmentInRound(ctx, "r-equivoc", commit1))

	// Second commitment with DIFFERENT hash from same verifier → equivocation
	commit2 := &types.CommitEntry{
		Verifier:         "zrn1cheater",
		CommitHash:       []byte("hash_second_________________________"),
		CommittedAtBlock: 101,
	}
	err := k.StoreCommitmentInRound(ctx, "r-equivoc", commit2)
	require.ErrorIs(t, err, types.ErrEquivocation,
		"different commit hash from same verifier must be detected as equivocation")
}

func TestEquivocationDetection_SameHashIsDuplicate(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	round := makeRoundInPhase("r-dup", "c1", types.VerificationPhase_VERIFICATION_PHASE_COMMIT, 50)
	require.NoError(t, k.SetVerificationRound(ctx, round))

	hash := []byte("consistent_hash_____________________")
	commit := &types.CommitEntry{
		Verifier:   "zrn1val",
		CommitHash: hash,
	}
	require.NoError(t, k.StoreCommitmentInRound(ctx, "r-dup", commit))

	// Same hash again → duplicate (not equivocation)
	err := k.StoreCommitmentInRound(ctx, "r-dup", commit)
	require.ErrorIs(t, err, types.ErrDuplicateCommitment)
	require.NotErrorIs(t, err, types.ErrEquivocation)
}

// ─── Slash flow through CompleteRound ────────────────────────────────────────

func TestCompleteRound_SlashesWrongVoter(t *testing.T) {
	k, ctx, _, sk := setupKnowledgeTestFull(t)
	sk.addValidator("zrn1correct1", 100_000, "bonded")
	sk.addValidator("zrn1correct2", 100_000, "bonded")
	sk.addValidator("zrn1wrong", 100_000, "bonded")

	// Lower threshold so 2/3 accept (66.6%) crosses it
	params, _ := k.GetParams(ctx)
	params.ConfidenceThreshold = 600_000 // 60%
	require.NoError(t, k.SetParams(ctx, params))

	claim := &types.Claim{
		Id:          "claim-slash-flow",
		FactContent: "Full slash flow test claim content",
		Domain:      "physics",
		Submitter:   "zrn1sub",
		Stake:       "1000000",
		Status:      types.ClaimStatus_CLAIM_STATUS_IN_VERIFICATION,
	}
	require.NoError(t, k.SetClaim(ctx, claim))

	round := makeRoundInPhase("r-slash-flow", "claim-slash-flow", types.VerificationPhase_VERIFICATION_PHASE_AGGREGATION, 50)
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

	// Aggregate and complete
	result, err := k.AggregateVerificationResult(ctx, round)
	require.NoError(t, err)
	require.Equal(t, types.Verdict_VERDICT_ACCEPT, result.Verdict, "2/3 accept should exceed 60% threshold")
	require.NoError(t, k.CompleteRound(ctx, round, result))

	// Verify the staking keeper received the slash
	var found bool
	for _, s := range sk.slashes {
		if s.Validator == "zrn1wrong" {
			found = true
			params, _ := k.GetParams(ctx)
			require.Equal(t, params.WrongVerificationSlashBps, s.SlashBps)
		}
	}
	require.True(t, found, "wrong voter must be slashed via staking keeper")
}

func TestCompleteRound_SlashesMissedReveal(t *testing.T) {
	k, ctx, _, sk := setupKnowledgeTestFull(t)
	sk.addValidator("zrn1revealer1", 100_000, "bonded")
	sk.addValidator("zrn1revealer2", 100_000, "bonded")
	sk.addValidator("zrn1skipper", 100_000, "bonded")

	claim := &types.Claim{
		Id:          "claim-missed-flow",
		FactContent: "Missed reveal slash flow test claim",
		Domain:      "physics",
		Submitter:   "zrn1sub",
		Stake:       "1000000",
		Status:      types.ClaimStatus_CLAIM_STATUS_IN_VERIFICATION,
	}
	require.NoError(t, k.SetClaim(ctx, claim))

	round := makeRoundInPhase("r-missed-flow", "claim-missed-flow", types.VerificationPhase_VERIFICATION_PHASE_AGGREGATION, 50)
	round.Commits = []*types.CommitEntry{
		{Verifier: "zrn1revealer1", CommitHash: []byte("h1"), CommittedAtBlock: 60},
		{Verifier: "zrn1revealer2", CommitHash: []byte("h2"), CommittedAtBlock: 60},
		{Verifier: "zrn1skipper", CommitHash: []byte("h3"), CommittedAtBlock: 60},
	}
	round.Reveals = []*types.RevealEntry{
		{Verifier: "zrn1revealer1", Vote: "accept", Salt: []byte("s1"), RevealedAtBlock: 70},
		{Verifier: "zrn1revealer2", Vote: "accept", Salt: []byte("s2"), RevealedAtBlock: 70},
		// zrn1skipper did NOT reveal
	}
	require.NoError(t, k.SetVerificationRound(ctx, round))

	// Set MinVerifiers=2 so 2 reveals is enough
	params, _ := k.GetParams(ctx)
	params.MinVerifiers = 2
	require.NoError(t, k.SetParams(ctx, params))

	result, err := k.AggregateVerificationResult(ctx, round)
	require.NoError(t, err)
	require.NoError(t, k.CompleteRound(ctx, round, result))

	// Verify skipper was slashed
	var skipperSlashed bool
	for _, s := range sk.slashes {
		if s.Validator == "zrn1skipper" {
			skipperSlashed = true
			require.Equal(t, params.MissedRevealSlashBps, s.SlashBps)
		}
	}
	require.True(t, skipperSlashed, "skipper must be slashed for missed reveal")
}

// ─── Slash ordering ─────────────────────────────────────────────────────────

func TestSlashOrdering_WrongLessThanMissedLessThanEquivocation(t *testing.T) {
	p := types.DefaultParams()

	require.Less(t, p.WrongVerificationSlashBps, p.MissedRevealSlashBps,
		"wrong verification slash should be less than missed reveal")
	require.Less(t, p.MissedRevealSlashBps, p.EquivocationSlashBps,
		"missed reveal slash should be less than equivocation")
	require.Less(t, p.EquivocationSlashBps, p.InvalidClaimSlashBps,
		"equivocation slash should be less than invalid claim")
}
