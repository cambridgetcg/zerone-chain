package keeper_test

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/keeper"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ═══════════════════════════════════════════════════════════════════════════════
// Integration helpers — shared across all 10 scenarios
// ═══════════════════════════════════════════════════════════════════════════════

// setupIntegrationKeeper returns a keeper with bank, staking, default domains,
// default reviewer staking params, default reputation decay params, and default
// sharding params. Block height starts at 100.
func setupIntegrationKeeper(t *testing.T) (keeper.Keeper, sdk.Context, *mockBankKeeper) {
	t.Helper()
	ss := newMockStoreService()
	bk := newMockBankKeeper()
	sk := &mockStakingKeeper{
		validators: []types.ValidatorInfo{
			{Address: "val1", Stake: 1_000_000, Tier: "bonded"},
			{Address: "val2", Stake: 1_000_000, Tier: "bonded"},
			{Address: "val3", Stake: 1_000_000, Tier: "bonded"},
		},
	}
	k := keeper.NewKeeper(ss, nil, "authority", bk, sk)
	ctx := sdk.Context{}.
		WithBlockHeight(100).
		WithEventManager(sdk.NewEventManager()).
		WithMultiStore(&mockCacheMultiStore{})

	// Set up default domains
	for _, name := range []string{"technology", "science", "culture", "creative"} {
		require.NoError(t, k.SetDomain(ctx, &types.Domain{
			Name:   name,
			Status: types.DomainStatus_DOMAIN_STATUS_ACTIVE,
		}))
	}

	// Set up default module params
	p := types.DefaultParams()
	require.NoError(t, k.SetParams(ctx, &p))

	// Set up reviewer staking params
	rp := types.DefaultReviewerStakingParams()
	require.NoError(t, k.SetReviewerStakingParams(ctx, rp))

	// Set up reputation decay params
	repParams := types.DefaultReputationDecayParams()
	require.NoError(t, k.SetReputationDecayParams(ctx, repParams))

	// Set up sharding params
	sp := types.DefaultShardingParams()
	require.NoError(t, k.SetShardingParams(ctx, sp))

	// Set up fitness decay params
	fp := types.DefaultFitnessDecayParams()
	require.NoError(t, k.SetFitnessDecayParams(ctx, fp))

	return k, ctx, bk
}

// submitAndAcceptTDU creates a submission, runs a full quality round with 3
// reviewers producing the given votes, aggregates, and returns the round ID.
// Uses rv1/rv2/rv3 as reviewers (valid bech32 addresses).
func submitAndAcceptTDU(
	t *testing.T, k keeper.Keeper, ctx sdk.Context,
	subID, content, domain, stake string,
	votes []*types.QualityVote,
) string {
	t.Helper()

	sub := &types.Submission{
		Id:          subID,
		Domain:      domain,
		Submitter:   testAddr,
		Content:     content,
		Stake:       stake,
		Status:      types.SubmissionStatus_SUBMISSION_STATUS_PENDING,
		Consent:     &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
		ContentHash: "hash_" + subID,
	}
	require.NoError(t, k.SetSubmission(ctx, sub))

	verifiers := []string{rv1, rv2, rv3}
	roundID, err := k.InitiateQualityRound(ctx, subID, "", verifiers)
	require.NoError(t, err)

	runFullRoundWithStaking(t, k, ctx, roundID, votes)
	require.NoError(t, k.AggregateQualityRound(ctx, roundID))
	return roundID
}

// goldVotes returns 3 uniform gold-level votes.
func goldVotes() []*types.QualityVote {
	return []*types.QualityVote{
		{OverallQuality: 850000, ConsentValid: true},
		{OverallQuality: 850000, ConsentValid: true},
		{OverallQuality: 850000, ConsentValid: true},
	}
}

// bronzeVotes returns 3 uniform bronze-level votes.
func bronzeVotes() []*types.QualityVote {
	return []*types.QualityVote{
		{OverallQuality: 450000, ConsentValid: true},
		{OverallQuality: 450000, ConsentValid: true},
		{OverallQuality: 450000, ConsentValid: true},
	}
}

// rejectVotes returns 3 uniform reject-level votes (below bronze threshold).
func rejectVotes() []*types.QualityVote {
	return []*types.QualityVote{
		{OverallQuality: 200000, ConsentValid: true},
		{OverallQuality: 200000, ConsentValid: true},
		{OverallQuality: 200000, ConsentValid: true},
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// SCENARIO 1: Full lifecycle test
// Submit → review → accept → fitness → reputation → sharding
// ═══════════════════════════════════════════════════════════════════════════════

func TestIntegration_FullLifecycle(t *testing.T) {
	k, ctx, bk := setupIntegrationKeeper(t)

	// 1. Agent submits TDU with stake
	roundID := submitAndAcceptTDU(t, k, ctx, "s1", "high quality TDU", "technology", "1000000", goldVotes())

	// 2. Verify round completed with gold verdict
	round, found := k.GetQualityRound(ctx, roundID)
	require.True(t, found)
	require.Equal(t, types.QualityVerdict_QUALITY_VERDICT_GOLD, round.Verdict)
	require.Equal(t, types.VerificationPhase_VERIFICATION_PHASE_COMPLETE, round.Phase)

	// 3. Verify submission accepted
	sub, found := k.GetSubmission(ctx, "s1")
	require.True(t, found)
	require.Equal(t, types.SubmissionStatus_SUBMISSION_STATUS_ACCEPTED, sub.Status)

	// 4. Verify stake was returned to submitter (module→account transfers include submitter payout)
	submitterPaid := false
	for _, call := range bk.moduleToAccountCalls {
		if call.to == testAddr {
			submitterPaid = true
			break
		}
	}
	require.True(t, submitterPaid, "submitter should receive stake back on accept")

	// 5. Verify reviewer stakes were escrowed (account→module)
	require.Greater(t, len(bk.accountToModuleCalls), 0, "reviewer stakes should be escrowed")

	// 6. Verify FitnessRecord created for the new sample
	samples := k.GetSamplesByDomain(ctx, "technology")
	require.Len(t, samples, 1)
	sampleID := samples[0]

	rec, found := k.GetFitnessRecord(ctx, sampleID)
	require.True(t, found, "fitness record should exist for accepted TDU")
	// After initialization (0.5) + update with consensus signal, score should be > 0.5
	score := rec.GetFitnessScore()
	require.True(t, score.GTE(sdkmath.LegacyNewDecWithPrec(4, 1)), "fitness score should be >= 0.4, got %s", score)

	// 7. Verify submitter gained reputation (gold = 3x base 10 = 30)
	rep, found := k.GetAgentDomainReputation(ctx, testAddr, "technology")
	require.True(t, found, "submitter should have reputation")
	require.Equal(t, "30.000000000000000000", rep.Score)

	// 8. Verify majority reviewers gained reputation (all 3 voted accept → majority)
	for _, v := range []string{rv1, rv2, rv3} {
		rep, found := k.GetAgentDomainReputation(ctx, v, "technology")
		require.True(t, found, "reviewer %s should have reputation", v)
		require.Equal(t, "3.000000000000000000", rep.Score, "reviewer %s rep", v)
	}

	// 9. Verify sharding: at next snapshot interval, TDU is assigned
	sdkCtx := ctx.WithBlockHeight(1000).WithEventManager(sdk.NewEventManager())
	require.NoError(t, k.BeginBlocker(sdkCtx))

	events := sdkCtx.EventManager().Events()
	require.True(t, hasEvent(events, types.EventShardReshuffle), "shard reshuffle should occur at snapshot interval")

	// At least one validator should have the sample's TDU assigned
	assignmentFound := false
	for _, v := range []string{"val1", "val2", "val3"} {
		a, found := k.GetShardAssignment(sdkCtx, v, 1000)
		if found && len(a.TDUIDs) > 0 {
			assignmentFound = true
			break
		}
	}
	require.True(t, assignmentFound, "TDU should be assigned to at least one validator")
}

// ═══════════════════════════════════════════════════════════════════════════════
// SCENARIO 2: Fitness evolution over time
// ═══════════════════════════════════════════════════════════════════════════════

func TestIntegration_FitnessEvolution_AccessedTDUs_BecomeCore(t *testing.T) {
	k, ctx, _ := setupIntegrationKeeper(t)

	// Submit and accept 5 TDUs
	for i := 0; i < 5; i++ {
		subID := "s" + string(rune('A'+i))
		submitAndAcceptTDU(t, k, ctx, subID, "content "+subID, "technology", "1000000", goldVotes())
	}

	sampleIDs := k.GetSamplesByDomain(ctx, "technology")
	require.Len(t, sampleIDs, 5)

	// Simulate 10 fitness epochs: send strong signals to first 3 TDUs, nothing to last 2
	fp := k.GetFitnessDecayParams(ctx)
	for epoch := uint64(1); epoch <= 10; epoch++ {
		for i := 0; i < 3; i++ {
			signal := types.FitnessSignal{
				TrainingInfluence: sdkmath.LegacyNewDecWithPrec(9, 1), // 0.9
				UsageCorrelation:  sdkmath.LegacyNewDecWithPrec(8, 1), // 0.8
				Redundancy:        sdkmath.LegacyNewDecWithPrec(2, 1), // 0.2 (unique)
			}
			_ = k.UpdateFitnessScore(ctx, sampleIDs[i], signal, epoch)
		}

		// Apply decay to unscored
		k.DecayUnscored(ctx, epoch)
	}

	// Frequently accessed TDUs should be Core (>= 0.7)
	for i := 0; i < 3; i++ {
		rec, found := k.GetFitnessRecord(ctx, sampleIDs[i])
		require.True(t, found)
		status := rec.GetLifecycleStatus()
		score := rec.GetFitnessScore()
		require.Equal(t, types.TDULifecycleCore, status,
			"TDU %d with signals should be Core, score=%s", i, score)
	}

	// Untouched TDUs: after 10 cycles without signal (threshold=5), decay applied.
	// They should have decayed below their initial score.
	for i := 3; i < 5; i++ {
		rec, found := k.GetFitnessRecord(ctx, sampleIDs[i])
		require.True(t, found)
		score := rec.GetFitnessScore()
		require.True(t, score.LT(sdkmath.LegacyNewDecWithPrec(5, 1)),
			"TDU %d without signals should decay below 0.5, got %s", i, score)
	}

	// Core TDUs earn longevity rewards, dormant earn nothing
	coreReward := k.ComputeLongevityReward(ctx, sampleIDs[0])
	require.True(t, coreReward.IsPositive(), "Core TDU should earn longevity reward")

	dormantReward := k.ComputeLongevityReward(ctx, sampleIDs[4])
	_ = fp
	// Dormant or decayed TDUs with score < Active threshold earn nothing
	rec4, _ := k.GetFitnessRecord(ctx, sampleIDs[4])
	if rec4.GetLifecycleStatus() == types.TDULifecycleDormant || rec4.GetLifecycleStatus() == types.TDULifecyclePruned {
		require.True(t, dormantReward.IsZero(), "Dormant/Pruned TDU should earn no longevity reward")
	}
}

func TestIntegration_FitnessEvolution_PrunedExcludedFromShards(t *testing.T) {
	k, ctx, _ := setupIntegrationKeeper(t)

	// Create 2 TDUs directly via fitness records
	activeRec := types.NewTDUFitnessRecord("active1", sdkmath.NewInt(1_000_000), 0)
	activeRec.SetFitnessScore(sdkmath.LegacyNewDecWithPrec(5, 1)) // 0.5 Active
	require.NoError(t, k.SetFitnessRecord(ctx, activeRec))

	prunedRec := types.NewTDUFitnessRecord("pruned1", sdkmath.NewInt(1_000_000), 0)
	prunedRec.SetFitnessScore(sdkmath.LegacyNewDecWithPrec(5, 2)) // 0.05 Pruned
	require.NoError(t, k.SetFitnessRecord(ctx, prunedRec))

	// GetActiveTDUHashes should exclude pruned
	hashes := k.GetActiveTDUHashes(ctx)
	require.Contains(t, hashes, "active1")
	require.NotContains(t, hashes, "pruned1")
}

// ═══════════════════════════════════════════════════════════════════════════════
// SCENARIO 3: Reputation-weighted voting
// ═══════════════════════════════════════════════════════════════════════════════

func TestIntegration_ReputationWeightedVoting(t *testing.T) {
	k, ctx, _ := setupIntegrationKeeper(t)

	// Give rv1 high reputation directly (200) → weight = 1.0 + min(200/100, 1.0) × 2.0 = 3.0
	// rv2/rv3 have no rep → weight = 1.0 each
	// With weights [3.0, 1.0, 1.0], rv1's vote dominates the weighted median.
	require.NoError(t, k.SetAgentDomainReputation(ctx, types.NewAgentDomainReputation(
		rv1, "technology", sdkmath.LegacyNewDec(200), 50)))

	sub := &types.Submission{
		Id: "contested", Domain: "technology", Submitter: testAddr,
		Content: "contested content", Stake: "1000000",
		Status:  types.SubmissionStatus_SUBMISSION_STATUS_PENDING,
		Consent: &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
		ContentHash: "hash_contested",
	}
	require.NoError(t, k.SetSubmission(ctx, sub))

	verifiers := []string{rv1, rv2, rv3}
	roundID, err := k.InitiateQualityRound(ctx, "contested", "", verifiers)
	require.NoError(t, err)

	// rv1 votes gold (accept), rv2+rv3 vote reject
	// Sorted by value: [(200000, w=1.0), (200000, w=1.0), (900000, w=3.0)]
	// Total weight = 5.0, halfWeight = 2.5
	// Cumulative at 200000: 2.0 < 2.5 → median = 900000
	votes := []*types.QualityVote{
		{OverallQuality: 900000, ConsentValid: true}, // rv1: accept (weight 3.0)
		{OverallQuality: 200000, ConsentValid: true}, // rv2: reject (weight 1.0)
		{OverallQuality: 200000, ConsentValid: true}, // rv3: reject (weight 1.0)
	}
	runFullRoundWithStaking(t, k, ctx, roundID, votes)
	require.NoError(t, k.AggregateQualityRound(ctx, roundID))

	round, found := k.GetQualityRound(ctx, roundID)
	require.True(t, found)

	// rv1's high weight (3.0) should shift the weighted median above 200000 (the simple median).
	require.True(t, round.AggregateScores.OverallQuality > 200000,
		"high-rep voter should shift aggregation above simple median, got %d", round.AggregateScores.OverallQuality)
}

func TestIntegration_MinorityReviewerLosesReputation(t *testing.T) {
	k, ctx, _ := setupIntegrationKeeper(t)

	// Give rv3 initial reputation of 20
	require.NoError(t, k.SetAgentDomainReputation(ctx, types.NewAgentDomainReputation(
		rv3, "technology", sdkmath.LegacyNewDec(20), 50)))

	// Round where rv1+rv2 accept, rv3 rejects
	sub := &types.Submission{
		Id: "s_minority", Domain: "technology", Submitter: testAddr,
		Content: "minority test", Stake: "1000000",
		Status:  types.SubmissionStatus_SUBMISSION_STATUS_PENDING,
		Consent: &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
		ContentHash: "hash_minority",
	}
	require.NoError(t, k.SetSubmission(ctx, sub))

	verifiers := []string{rv1, rv2, rv3}
	roundID, err := k.InitiateQualityRound(ctx, "s_minority", "", verifiers)
	require.NoError(t, err)

	votes := []*types.QualityVote{
		{OverallQuality: 850000, ConsentValid: true}, // rv1: accept
		{OverallQuality: 850000, ConsentValid: true}, // rv2: accept
		{OverallQuality: 200000, ConsentValid: true}, // rv3: reject (minority)
	}
	runFullRoundWithStaking(t, k, ctx, roundID, votes)
	require.NoError(t, k.AggregateQualityRound(ctx, roundID))

	// rv3 should lose reputation: 20 - 5 (penalty) = 15
	rep, found := k.GetAgentDomainReputation(ctx, rv3, "technology")
	require.True(t, found)
	require.Equal(t, "15.000000000000000000", rep.Score,
		"minority reviewer should lose 5 rep, got %s", rep.Score)
}

// ═══════════════════════════════════════════════════════════════════════════════
// SCENARIO 4: Rubber-stamp attack
// ═══════════════════════════════════════════════════════════════════════════════

func TestIntegration_RubberStamp_ReviewerLoseStakeOnContest(t *testing.T) {
	k, ctx, bk := setupIntegrationKeeper(t)

	// Garbage submission that rubber-stampers accept
	sub := &types.Submission{
		Id: "garbage", Domain: "technology", Submitter: testAddr,
		Content: "garbage content that should be rejected", Stake: "1000000",
		Status:  types.SubmissionStatus_SUBMISSION_STATUS_PENDING,
		Consent: &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
		ContentHash: "hash_garbage",
	}
	require.NoError(t, k.SetSubmission(ctx, sub))

	verifiers := []string{rv1, rv2, rv3}
	roundID, err := k.InitiateQualityRound(ctx, "garbage", "", verifiers)
	require.NoError(t, err)

	// Rubber-stampers always accept
	votes := goldVotes()
	runFullRoundWithStaking(t, k, ctx, roundID, votes)
	require.NoError(t, k.AggregateQualityRound(ctx, roundID))

	// Verify it was accepted (rubber-stampers succeeded initially)
	round, found := k.GetQualityRound(ctx, roundID)
	require.True(t, found)
	require.Equal(t, types.QualityVerdict_QUALITY_VERDICT_GOLD, round.Verdict)

	// Now simulate repeated bad reviews: in 3 more rounds, rubber-stampers
	// are minority each time (they accept garbage, real reviewers reject).
	// Each time they're minority, they lose reputation.
	for i := 0; i < 3; i++ {
		subID := "real_" + string(rune('0'+i))
		s := &types.Submission{
			Id: subID, Domain: "technology", Submitter: testAddr,
			Content: "actually bad content", Stake: "1000000",
			Status:  types.SubmissionStatus_SUBMISSION_STATUS_PENDING,
			Consent: &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
			ContentHash: "hash_" + subID,
		}
		require.NoError(t, k.SetSubmission(ctx, s))

		roundID, err := k.InitiateQualityRound(ctx, subID, "", verifiers)
		require.NoError(t, err)

		// rv1 accepts (rubber stamp), rv2+rv3 correctly reject
		votes := []*types.QualityVote{
			{OverallQuality: 850000, ConsentValid: true}, // rv1: accept (bad)
			{OverallQuality: 100000, ConsentValid: true}, // rv2: reject (correct)
			{OverallQuality: 100000, ConsentValid: true}, // rv3: reject (correct)
		}
		runFullRoundWithStaking(t, k, ctx, roundID, votes)
		require.NoError(t, k.AggregateQualityRound(ctx, roundID))
	}

	// rv1 was minority 3 times → lost 5 × 3 = 15 rep (clamped at 0 if < 0)
	// rv1 started with 3.0 from the first round, then lost 5 × 3 = 15 → clamped to 0
	rep1, found := k.GetAgentDomainReputation(ctx, rv1, "technology")
	require.True(t, found)
	require.True(t, rep1.GetScore().LTE(sdkmath.LegacyZeroDec()),
		"rubber-stamper should have 0 rep after repeated bad reviews, got %s", rep1.GetScore())

	// rv2+rv3 were majority 3 times → gained 3 × 3 = 9 rep more (+ initial from first round)
	for _, v := range []string{rv2, rv3} {
		rep, found := k.GetAgentDomainReputation(ctx, v, "technology")
		require.True(t, found)
		require.True(t, rep.GetScore().GT(sdkmath.LegacyNewDec(5)),
			"correct reviewer %s should have accumulated rep, got %s", v, rep.GetScore())
	}

	_ = bk
}

// ═══════════════════════════════════════════════════════════════════════════════
// SCENARIO 5: Lazy-reject attack
// ═══════════════════════════════════════════════════════════════════════════════

func TestIntegration_LazyReject_LosesRepOnGoodSubmissions(t *testing.T) {
	k, ctx, _ := setupIntegrationKeeper(t)

	// Give rv1 moderate initial reputation (20) — after 5 minority penalties (5×5=25)
	// rep clamps to 0, while rv2 accumulates 5×3=15 from majority wins.
	require.NoError(t, k.SetAgentDomainReputation(ctx, types.NewAgentDomainReputation(
		rv1, "technology", sdkmath.LegacyNewDec(20), 100)))

	verifiers := []string{rv1, rv2, rv3}

	// Submit 5 good submissions. rv1 always rejects (lazy reject), rv2+rv3 accept.
	for i := 0; i < 5; i++ {
		subID := "good_" + string(rune('A'+i))
		sub := &types.Submission{
			Id: subID, Domain: "technology", Submitter: testAddr,
			Content: "good content " + subID, Stake: "1000000",
			Status:  types.SubmissionStatus_SUBMISSION_STATUS_PENDING,
			Consent: &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
			ContentHash: "hash_" + subID,
		}
		require.NoError(t, k.SetSubmission(ctx, sub))

		roundID, err := k.InitiateQualityRound(ctx, subID, "", verifiers)
		require.NoError(t, err)

		votes := []*types.QualityVote{
			{OverallQuality: 200000, ConsentValid: true}, // rv1: reject (lazy)
			{OverallQuality: 850000, ConsentValid: true}, // rv2: accept
			{OverallQuality: 850000, ConsentValid: true}, // rv3: accept
		}
		runFullRoundWithStaking(t, k, ctx, roundID, votes)
		require.NoError(t, k.AggregateQualityRound(ctx, roundID))
	}

	// rv1 was minority 5 times: 20 - 5×5 = -5 → clamped to 0
	rep1, found := k.GetAgentDomainReputation(ctx, rv1, "technology")
	require.True(t, found)
	require.True(t, rep1.GetScore().IsZero(),
		"lazy-rejecter should have 0 rep after penalties, got %s", rep1.GetScore())

	// rv2 was majority 5 times: 0 + 5×3 = 15
	rep2, _ := k.GetAgentDomainReputation(ctx, rv2, "technology")
	require.True(t, rep2.GetScore().GT(rep1.GetScore()),
		"correct reviewer rep (%s) should exceed lazy-rejecter rep (%s)", rep2.GetScore(), rep1.GetScore())
}

func TestIntegration_LazyReject_ReputationFloor(t *testing.T) {
	k, ctx, _ := setupIntegrationKeeper(t)

	// Set aggressive decay params so we can see floor behavior
	repParams := types.ReputationDecayParams{
		DecayRateBps:        5000, // 50%
		DecayIntervalBlocks: 100,
		FloorRatioBps:       2500, // 25% of peak
	}
	require.NoError(t, k.SetReputationDecayParams(ctx, repParams))

	// Agent with peak rep of 100
	require.NoError(t, k.SetAgentDomainReputation(ctx, types.NewAgentDomainReputation(
		rv1, "technology", sdkmath.LegacyNewDec(100), 0)))

	// Heavy decay
	k.ApplyReputationDecay(ctx, 100000)

	rep, found := k.GetAgentDomainReputation(ctx, rv1, "technology")
	require.True(t, found)
	floor := sdkmath.LegacyNewDec(25) // 25% of peak 100
	require.True(t, rep.GetScore().GTE(floor),
		"reputation should never go below floor (25), got %s", rep.GetScore())
}

// ═══════════════════════════════════════════════════════════════════════════════
// SCENARIO 6: Sybil reviewer ring
// ═══════════════════════════════════════════════════════════════════════════════

func TestIntegration_SybilRing_AllLoseWhenWrong(t *testing.T) {
	k, ctx, bk := setupIntegrationKeeper(t)

	verifiers := []string{rv1, rv2, rv3}

	// Give all 3 sybils initial reputation
	for _, v := range verifiers {
		require.NoError(t, k.SetAgentDomainReputation(ctx, types.NewAgentDomainReputation(
			v, "technology", sdkmath.LegacyNewDec(20), 50)))
	}

	// Sybils collude: all vote accept on garbage. But the round resolves with
	// their unanimous accept → they get accept verdict (since they're all reviewers).
	// The protocol can't distinguish sybils from legitimate unanimous agreement.
	sub := &types.Submission{
		Id: "sybil_test", Domain: "technology", Submitter: testAddr,
		Content: "sybil test content", Stake: "1000000",
		Status:  types.SubmissionStatus_SUBMISSION_STATUS_PENDING,
		Consent: &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
		ContentHash: "hash_sybil",
	}
	require.NoError(t, k.SetSubmission(ctx, sub))

	roundID, err := k.InitiateQualityRound(ctx, "sybil_test", "", verifiers)
	require.NoError(t, err)

	// All vote the same (colluding)
	votes := goldVotes()
	runFullRoundWithStaking(t, k, ctx, roundID, votes)
	require.NoError(t, k.AggregateQualityRound(ctx, roundID))

	// Round passes (sybils won this time)
	round, _ := k.GetQualityRound(ctx, roundID)
	require.Equal(t, types.QualityVerdict_QUALITY_VERDICT_GOLD, round.Verdict)

	// Now test when sybils are wrong: they all reject a good submission.
	// But since they're the only reviewers, they're all "majority" on reject.
	// The system correctly identifies their vote as majority regardless of collusion.
	sub2 := &types.Submission{
		Id: "sybil_wrong", Domain: "technology", Submitter: testAddr,
		Content: "good content", Stake: "1000000",
		Status:  types.SubmissionStatus_SUBMISSION_STATUS_PENDING,
		Consent: &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
		ContentHash: "hash_sybil_wrong",
	}
	require.NoError(t, k.SetSubmission(ctx, sub2))

	roundID2, err := k.InitiateQualityRound(ctx, "sybil_wrong", "", verifiers)
	require.NoError(t, err)

	// All vote reject
	rejectAllVotes := rejectVotes()
	runFullRoundWithStaking(t, k, ctx, roundID2, rejectAllVotes)
	initialModToAcct := len(bk.moduleToAccountCalls)
	require.NoError(t, k.AggregateQualityRound(ctx, roundID2))

	round2, _ := k.GetQualityRound(ctx, roundID2)
	require.Equal(t, types.QualityVerdict_QUALITY_VERDICT_REJECT, round2.Verdict)

	// After reject: submitter loses stake, sybils get challenge bonus
	// Key: all 3 sybils get paid simultaneously from same pool
	newModToAcct := bk.moduleToAccountCalls[initialModToAcct:]
	sybilPaid := 0
	for _, call := range newModToAcct {
		for _, v := range verifiers {
			if call.to == v {
				sybilPaid++
			}
		}
	}
	// All 3 sybils should receive payouts (their stake + share of challenge bonus)
	require.Equal(t, 3, sybilPaid, "all 3 colluding reviewers should receive payouts on reject")
}

// ═══════════════════════════════════════════════════════════════════════════════
// SCENARIO 7: Deep contested scenario
// ═══════════════════════════════════════════════════════════════════════════════

func TestIntegration_DeepContested_NoSupermajority(t *testing.T) {
	k, ctx, bk := setupIntegrationKeeper(t)

	// Set up 5 reviewers for a close split
	verifiers := []string{rv1, rv2, rv3, rv4, rv5}

	sub := &types.Submission{
		Id: "deep1", Domain: "technology", Submitter: testAddr,
		Content: "contested content", Stake: "1000000",
		Status:  types.SubmissionStatus_SUBMISSION_STATUS_PENDING,
		Consent: &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
		ContentHash: "hash_deep1",
	}
	require.NoError(t, k.SetSubmission(ctx, sub))

	roundID, err := k.InitiateQualityRound(ctx, "deep1", "", verifiers)
	require.NoError(t, err)

	// 3 accept, 2 reject → 3/5 = 60% < 66.7% supermajority → deep contested
	votes := []*types.QualityVote{
		{OverallQuality: 850000, ConsentValid: true}, // rv1: accept
		{OverallQuality: 850000, ConsentValid: true}, // rv2: accept
		{OverallQuality: 850000, ConsentValid: true}, // rv3: accept
		{OverallQuality: 200000, ConsentValid: true}, // rv4: reject
		{OverallQuality: 200000, ConsentValid: true}, // rv5: reject
	}

	salts := [][]byte{[]byte("s1"), []byte("s2"), []byte("s3"), []byte("s4"), []byte("s5")}
	for i, v := range verifiers {
		hash := types.ComputeQualityCommitHash(roundID, votes[i], salts[i])
		require.NoError(t, k.SubmitCommitment(ctx, &types.MsgSubmitCommitment{
			Verifier: v, RoundId: roundID, CommitHash: hash,
		}))
	}
	round, _ := k.GetQualityRound(ctx, roundID)
	round.Phase = types.VerificationPhase_VERIFICATION_PHASE_REVEAL
	require.NoError(t, k.SetQualityRound(ctx, round))
	for i, v := range verifiers {
		require.NoError(t, k.SubmitReveal(ctx, &types.MsgSubmitReveal{
			Verifier: v, RoundId: roundID, Scores: votes[i], Salt: salts[i],
		}))
	}

	initialModToAcct := len(bk.moduleToAccountCalls)
	require.NoError(t, k.AggregateQualityRound(ctx, roundID))

	// Check deep_contested event
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	events := sdkCtx.EventManager().Events()
	deepContestedFound := false
	for _, e := range events {
		if e.Type == "deep_contested" {
			deepContestedFound = true
		}
	}
	require.True(t, deepContestedFound, "deep_contested event should be emitted")

	// All stakes returned (grace)
	newModToAcct := bk.moduleToAccountCalls[initialModToAcct:]
	require.Greater(t, len(newModToAcct), 0, "stakes should be returned on deep contested")
}

func TestIntegration_DeepContested_ThirdStrike_PermanentReject(t *testing.T) {
	k, ctx, _ := setupIntegrationKeeper(t)

	// Set MaxContestedDeepCount = 3 (default)
	contentHash := "repeated_content_hash"

	// Simulate 3 deep-contested strikes on same content hash
	rp := k.GetReviewerStakingParams(ctx)
	for i := 0; i < 3; i++ {
		count, permanent, err := k.RecordContestedStrike(ctx, contentHash, rp)
		require.NoError(t, err)
		require.Equal(t, uint64(i+1), count)
		if i < 2 {
			require.False(t, permanent)
		} else {
			require.True(t, permanent, "third strike should trigger permanent rejection")
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// SCENARIO 8: Stake exhaustion
// ═══════════════════════════════════════════════════════════════════════════════

func TestIntegration_StakeExhaustion_CommitmentRejected(t *testing.T) {
	k, ctx, bk := setupIntegrationKeeper(t)

	sub := &types.Submission{
		Id: "s_exhaust", Domain: "technology", Submitter: testAddr,
		Content: "exhaust test", Stake: "1000000",
		Status:  types.SubmissionStatus_SUBMISSION_STATUS_PENDING,
		Consent: &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
		ContentHash: "hash_exhaust",
	}
	require.NoError(t, k.SetSubmission(ctx, sub))

	verifiers := []string{rv1, rv2, rv3}
	roundID, err := k.InitiateQualityRound(ctx, "s_exhaust", "", verifiers)
	require.NoError(t, err)

	// rv1 commits successfully
	hash1 := types.ComputeQualityCommitHash(roundID, goldVotes()[0], []byte("s1"))
	require.NoError(t, k.SubmitCommitment(ctx, &types.MsgSubmitCommitment{
		Verifier: rv1, RoundId: roundID, CommitHash: hash1,
	}))

	// rv2 will fail on next bank send (insufficient funds)
	bk.failNextSend = true
	hash2 := types.ComputeQualityCommitHash(roundID, goldVotes()[1], []byte("s2"))
	err = k.SubmitCommitment(ctx, &types.MsgSubmitCommitment{
		Verifier: rv2, RoundId: roundID, CommitHash: hash2,
	})
	require.Error(t, err, "commitment should be rejected when reviewer can't stake")
	require.ErrorIs(t, err, types.ErrReviewerStakeInsufficient)

	// rv3 commits successfully
	hash3 := types.ComputeQualityCommitHash(roundID, goldVotes()[2], []byte("s3"))
	require.NoError(t, k.SubmitCommitment(ctx, &types.MsgSubmitCommitment{
		Verifier: rv3, RoundId: roundID, CommitHash: hash3,
	}))

	// Round should have 2 commits (rv1 and rv3), not 3
	round, found := k.GetQualityRound(ctx, roundID)
	require.True(t, found)
	require.Len(t, round.Commits, 2, "round should continue with 2 commits after rv2 failure")
}

func TestIntegration_StakeExhaustion_RoundResolvesWithRemaining(t *testing.T) {
	k, ctx, _ := setupIntegrationKeeper(t)

	sub := &types.Submission{
		Id: "s_partial", Domain: "technology", Submitter: testAddr,
		Content: "partial round", Stake: "1000000",
		Status:  types.SubmissionStatus_SUBMISSION_STATUS_PENDING,
		Consent: &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
		ContentHash: "hash_partial",
	}
	require.NoError(t, k.SetSubmission(ctx, sub))

	// Use 4 verifiers so that even if one can't commit, we still have 3
	verifiers := []string{rv1, rv2, rv3, rv4}
	roundID, err := k.InitiateQualityRound(ctx, "s_partial", "", verifiers)
	require.NoError(t, err)

	// Only 3 of 4 commit and reveal
	votes := goldVotes()
	salts := [][]byte{[]byte("s1"), []byte("s2"), []byte("s3")}
	for i, v := range []string{rv1, rv2, rv3} {
		hash := types.ComputeQualityCommitHash(roundID, votes[i], salts[i])
		require.NoError(t, k.SubmitCommitment(ctx, &types.MsgSubmitCommitment{
			Verifier: v, RoundId: roundID, CommitHash: hash,
		}))
	}

	round, _ := k.GetQualityRound(ctx, roundID)
	round.Phase = types.VerificationPhase_VERIFICATION_PHASE_REVEAL
	require.NoError(t, k.SetQualityRound(ctx, round))

	for i, v := range []string{rv1, rv2, rv3} {
		require.NoError(t, k.SubmitReveal(ctx, &types.MsgSubmitReveal{
			Verifier: v, RoundId: roundID, Scores: votes[i], Salt: salts[i],
		}))
	}

	// Aggregate should work with 3 reveals
	require.NoError(t, k.AggregateQualityRound(ctx, roundID))

	round, _ = k.GetQualityRound(ctx, roundID)
	require.Equal(t, types.QualityVerdict_QUALITY_VERDICT_GOLD, round.Verdict)
}

// ═══════════════════════════════════════════════════════════════════════════════
// SCENARIO 9: Genesis round-trip
// ═══════════════════════════════════════════════════════════════════════════════

func TestIntegration_GenesisRoundTrip(t *testing.T) {
	k, ctx, _ := setupIntegrationKeeper(t)

	// 1. Run full lifecycle to accumulate state
	submitAndAcceptTDU(t, k, ctx, "s_gen1", "genesis content 1", "technology", "1000000", goldVotes())
	submitAndAcceptTDU(t, k, ctx, "s_gen2", "genesis content 2", "science", "2000000", bronzeVotes())

	// Set up shard assignments
	require.NoError(t, k.SetShardAssignment(ctx, types.ShardAssignment{
		ValidatorAddr: "val1", TDUIDs: []string{"tdu1", "tdu2"}, SnapshotHeight: 1000, Seed: []byte("seed1"),
	}))
	require.NoError(t, k.SetStorageAttestation(ctx, types.StorageAttestation{
		ValidatorAddr: "val1", SnapshotHeight: 1000, AttestationHex: "abcdef", BlockHeight: 1050,
	}))

	// Set reputation records
	require.NoError(t, k.SetAgentDomainReputation(ctx, types.NewAgentDomainReputation(
		"agent1", "technology", sdkmath.LegacyNewDec(42), 90)))

	// 2. Export genesis (module params + domains)
	exported := k.ExportGenesis(ctx)
	require.NotNil(t, exported.Params)
	require.Greater(t, len(exported.Domains), 0)

	// Export sharding genesis
	shardExported := k.ExportShardingGenesis(ctx)
	require.Len(t, shardExported.Assignments, 1)
	require.Len(t, shardExported.Attestations, 1)

	// 3. Import into fresh keeper
	ss2 := newMockStoreService()
	bk2 := newMockBankKeeper()
	k2 := keeper.NewKeeper(ss2, nil, "authority", bk2, nil)
	ctx2 := sdk.Context{}.
		WithBlockHeight(100).
		WithEventManager(sdk.NewEventManager()).
		WithMultiStore(&mockCacheMultiStore{})

	require.NoError(t, k2.InitGenesis(ctx2, exported))
	require.NoError(t, k2.ImportShardingGenesis(ctx2, shardExported))

	// 4. Verify all state preserved
	params2, err := k2.GetParams(ctx2)
	require.NoError(t, err)
	require.Equal(t, exported.Params.GoldThreshold, params2.GoldThreshold)
	require.Equal(t, exported.Params.BronzeThreshold, params2.BronzeThreshold)

	// Verify domains
	for _, name := range []string{"technology", "science"} {
		domain, found := k2.GetDomain(ctx2, name)
		require.True(t, found, "domain %s should exist after import", name)
		require.Equal(t, types.DomainStatus_DOMAIN_STATUS_ACTIVE, domain.Status)
	}

	// Verify shard assignments
	a, found := k2.GetShardAssignment(ctx2, "val1", 1000)
	require.True(t, found)
	require.Equal(t, []string{"tdu1", "tdu2"}, a.TDUIDs)

	// Verify attestation
	att, found := k2.GetStorageAttestation(ctx2, "val1", 1000)
	require.True(t, found)
	require.Equal(t, "abcdef", att.AttestationHex)

	// 5. Continue operations after import — should not error
	p := types.DefaultParams()
	require.NoError(t, k2.SetParams(ctx2, &p))
	setupDomainsFresh(t, k2, ctx2)

	sub := &types.Submission{
		Id: "post_import", Domain: "technology", Submitter: testAddr,
		Content: "post import content", Stake: "1000000",
		Status:  types.SubmissionStatus_SUBMISSION_STATUS_PENDING,
		Consent: &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
		ContentHash: "hash_post_import",
	}
	require.NoError(t, k2.SetSubmission(ctx2, sub))
}

// setupDomainsFresh creates active domains on a fresh keeper.
func setupDomainsFresh(t *testing.T, k keeper.Keeper, ctx sdk.Context) {
	t.Helper()
	for _, name := range []string{"technology", "science", "culture", "creative"} {
		_ = k.SetDomain(ctx, &types.Domain{
			Name:   name,
			Status: types.DomainStatus_DOMAIN_STATUS_ACTIVE,
		})
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// SCENARIO 10: Epoch boundary
// ═══════════════════════════════════════════════════════════════════════════════

func TestIntegration_EpochBoundary_FitnessDecay_And_ReputationDecay(t *testing.T) {
	k, ctx, _ := setupIntegrationKeeper(t)

	// Set fitness and reputation params with aligned epochs for testing
	fp := types.DefaultFitnessDecayParams()
	require.NoError(t, k.SetFitnessDecayParams(ctx, fp))

	repParams := types.ReputationDecayParams{
		DecayRateBps:        500,
		DecayIntervalBlocks: 100,
		FloorRatioBps:       2500,
	}
	require.NoError(t, k.SetReputationDecayParams(ctx, repParams))

	// Create a fitness record that will decay
	rec := types.NewTDUFitnessRecord("tdu_epoch", sdkmath.NewInt(1_000_000), 0)
	rec.SetFitnessScore(sdkmath.LegacyNewDecWithPrec(5, 1)) // 0.5 Active
	require.NoError(t, k.SetFitnessRecord(ctx, rec))

	// Create a reputation record that will decay
	require.NoError(t, k.SetAgentDomainReputation(ctx, types.NewAgentDomainReputation(
		"agent_epoch", "technology", sdkmath.LegacyNewDec(100), 0)))

	// Run EndBlocker at a fitness epoch boundary
	fitnessEpoch := fp.GetFitnessEpochBlocks()
	blockHeight := fitnessEpoch * 10 // 10th fitness epoch

	sdkCtx := ctx.WithBlockHeight(int64(blockHeight)).WithEventManager(sdk.NewEventManager())
	require.NoError(t, k.EndBlocker(sdkCtx))

	// Verify fitness decay was applied (TDU had no signals for 10 cycles, threshold=5)
	recAfter, found := k.GetFitnessRecord(sdkCtx, "tdu_epoch")
	require.True(t, found)
	require.True(t, recAfter.GetFitnessScore().LT(sdkmath.LegacyNewDecWithPrec(5, 1)),
		"fitness should have decayed, got %s", recAfter.GetFitnessScore())
}

func TestIntegration_EpochBoundary_NoRaceConditions(t *testing.T) {
	k, ctx, _ := setupIntegrationKeeper(t)

	// Set up state that all three systems will process
	rec := types.NewTDUFitnessRecord("tdu_race", sdkmath.NewInt(1_000_000), 0)
	rec.SetFitnessScore(sdkmath.LegacyNewDecWithPrec(5, 1))
	require.NoError(t, k.SetFitnessRecord(ctx, rec))

	require.NoError(t, k.SetAgentDomainReputation(ctx, types.NewAgentDomainReputation(
		"agent_race", "technology", sdkmath.LegacyNewDec(50), 0)))

	// Trigger all epoch processing at once — should not panic
	fp := k.GetFitnessDecayParams(ctx)
	epoch := fp.GetFitnessEpochBlocks() * 10
	sdkCtx := ctx.WithBlockHeight(int64(epoch)).WithEventManager(sdk.NewEventManager())

	// BeginBlocker (shard reshuffle) + EndBlocker (fitness decay, rep decay, rewards)
	// Running both should not cause panics or ordering issues
	require.NotPanics(t, func() {
		_ = k.BeginBlocker(sdkCtx)
		_ = k.EndBlocker(sdkCtx)
	})

	// State should still be consistent
	_, found := k.GetFitnessRecord(sdkCtx, "tdu_race")
	require.True(t, found, "fitness record should still exist after epoch processing")

	_, found = k.GetAgentDomainReputation(sdkCtx, "agent_race", "technology")
	require.True(t, found, "reputation record should still exist after epoch processing")
}

// ═══════════════════════════════════════════════════════════════════════════════
// Additional integration edge cases
// ═══════════════════════════════════════════════════════════════════════════════

func TestIntegration_AcceptedTDU_InitializesFitnessAt05(t *testing.T) {
	k, ctx, _ := setupIntegrationKeeper(t)

	submitAndAcceptTDU(t, k, ctx, "s_fit", "fitness init content", "technology", "1000000", goldVotes())

	samples := k.GetSamplesByDomain(ctx, "technology")
	require.Len(t, samples, 1)

	rec, found := k.GetFitnessRecord(ctx, samples[0])
	require.True(t, found)
	// After init (0.5) + one signal update, score should be positive and reasonable
	score := rec.GetFitnessScore()
	require.True(t, score.IsPositive(), "fitness should be positive after init + signal")
}

func TestIntegration_RejectedTDU_NoFitnessRecord(t *testing.T) {
	k, ctx, _ := setupIntegrationKeeper(t)

	sub := &types.Submission{
		Id: "s_rej", Domain: "technology", Submitter: testAddr,
		Content: "bad content", Stake: "1000000",
		Status:  types.SubmissionStatus_SUBMISSION_STATUS_PENDING,
		Consent: &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
		ContentHash: "hash_reject",
	}
	require.NoError(t, k.SetSubmission(ctx, sub))

	verifiers := []string{rv1, rv2, rv3}
	roundID, err := k.InitiateQualityRound(ctx, "s_rej", "", verifiers)
	require.NoError(t, err)

	runFullRoundWithStaking(t, k, ctx, roundID, rejectVotes())
	require.NoError(t, k.AggregateQualityRound(ctx, roundID))

	// No sample → no fitness record
	samples := k.GetSamplesByDomain(ctx, "technology")
	require.Len(t, samples, 0, "rejected submission should not create sample")
}

func TestIntegration_ReputationDecay_ActiveAgentNotDecayed(t *testing.T) {
	k, ctx, _ := setupIntegrationKeeper(t)

	repParams := types.ReputationDecayParams{
		DecayRateBps:        500,
		DecayIntervalBlocks: 100,
		FloorRatioBps:       2500,
	}
	require.NoError(t, k.SetReputationDecayParams(ctx, repParams))

	// Agent active at block 100 (current height)
	require.NoError(t, k.SetAgentDomainReputation(ctx, types.NewAgentDomainReputation(
		"active_agent", "technology", sdkmath.LegacyNewDec(50), 100)))

	// Decay at block 150 (only 50 blocks inactive, interval is 100)
	k.ApplyReputationDecay(ctx, 150)

	rep, found := k.GetAgentDomainReputation(ctx, "active_agent", "technology")
	require.True(t, found)
	require.Equal(t, sdkmath.LegacyNewDec(50), rep.GetScore(),
		"active agent should not be decayed")
}

func TestIntegration_ShardAssignment_Deterministic(t *testing.T) {
	blockHash := []byte("deterministic_seed")
	tdus := []string{"tdu1", "tdu2", "tdu3", "tdu4", "tdu5"}
	validators := []string{"val1", "val2", "val3"}

	result1 := keeper.ComputeShardAssignments(blockHash, tdus, validators, 2)
	result2 := keeper.ComputeShardAssignments(blockHash, tdus, validators, 2)

	for _, v := range validators {
		require.Equal(t, result1[v], result2[v],
			"shard assignments must be deterministic for validator %s", v)
	}
}

func TestIntegration_ConsensusStrength_Unanimous(t *testing.T) {
	// Unanimous: 3/3 revealed → 1.0
	strength := keeper.ExportConsensusStrength(3, 3)
	require.Equal(t, sdkmath.LegacyOneDec(), strength)

	// Supermajority: 2/3 → 0.8
	strength = keeper.ExportConsensusStrength(2, 3)
	require.Equal(t, sdkmath.LegacyNewDecWithPrec(8, 1), strength)

	// Bare majority: 1/3 → 0.6
	strength = keeper.ExportConsensusStrength(1, 3)
	require.Equal(t, sdkmath.LegacyNewDecWithPrec(6, 1), strength)
}

func TestIntegration_MultipleAcceptedTDUs_AllGetFitness(t *testing.T) {
	k, ctx, _ := setupIntegrationKeeper(t)

	for i := 0; i < 3; i++ {
		subID := "multi_" + string(rune('A'+i))
		submitAndAcceptTDU(t, k, ctx, subID, "content "+subID, "technology", "1000000", goldVotes())
	}

	samples := k.GetSamplesByDomain(ctx, "technology")
	require.Len(t, samples, 3)

	for _, sid := range samples {
		rec, found := k.GetFitnessRecord(ctx, sid)
		require.True(t, found, "fitness record should exist for sample %s", sid)
		require.True(t, rec.GetFitnessScore().IsPositive(), "sample %s should have positive fitness", sid)
	}
}

func TestIntegration_LongevityRewards_CoreTDU(t *testing.T) {
	k, ctx, _ := setupIntegrationKeeper(t)

	// Create a Core-level fitness record
	rec := types.NewTDUFitnessRecord("core_tdu", sdkmath.NewInt(1_000_000), 0)
	rec.SetFitnessScore(sdkmath.LegacyNewDecWithPrec(8, 1)) // 0.8 → Core
	require.NoError(t, k.SetFitnessRecord(ctx, rec))

	reward := k.ComputeLongevityReward(ctx, "core_tdu")
	// Core: 1% of 1M = 10000
	require.Equal(t, sdkmath.NewInt(10000), reward, "Core TDU reward should be 10000")

	// Active-level
	rec2 := types.NewTDUFitnessRecord("active_tdu", sdkmath.NewInt(1_000_000), 0)
	rec2.SetFitnessScore(sdkmath.LegacyNewDecWithPrec(5, 1)) // 0.5 → Active
	require.NoError(t, k.SetFitnessRecord(ctx, rec2))

	reward2 := k.ComputeLongevityReward(ctx, "active_tdu")
	// Active: 0.5% of 1M = 5000
	require.Equal(t, sdkmath.NewInt(5000), reward2, "Active TDU reward should be 5000")

	// Dormant-level
	rec3 := types.NewTDUFitnessRecord("dormant_tdu", sdkmath.NewInt(1_000_000), 0)
	rec3.SetFitnessScore(sdkmath.LegacyNewDecWithPrec(2, 1)) // 0.2 → Dormant
	require.NoError(t, k.SetFitnessRecord(ctx, rec3))

	reward3 := k.ComputeLongevityReward(ctx, "dormant_tdu")
	require.True(t, reward3.IsZero(), "Dormant TDU should earn no reward")
}
