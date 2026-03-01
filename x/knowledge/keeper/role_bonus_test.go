package keeper_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/keeper"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Mock ZeroneAuthKeeper ──────────────────────────────────────────────────

type mockZeroneAuthKeeper struct {
	accounts map[string]string
}

func newMockZeroneAuthKeeper() *mockZeroneAuthKeeper {
	return &mockZeroneAuthKeeper{accounts: make(map[string]string)}
}

func (m *mockZeroneAuthKeeper) GetAccountType(_ context.Context, address string) (string, bool) {
	t, ok := m.accounts[address]
	return t, ok
}

// ─── Claim Confidence Bonus Tests ──────────────────────────────────────────

func TestHumanEmpiricalClaimBonus(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	params, _ := k.GetParams(ctx)

	boosted := keeper.ApplyRoleBonusToConfidence(700_000, types.ClaimType_CLAIM_TYPE_OBSERVATION, "human", params)
	// 700,000 * (1,000,000 + 150,000) / 1,000,000 = 805,000
	require.Equal(t, uint64(805_000), boosted)

	_ = k
}

func TestAgentComputationalClaimBonus(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	params, _ := k.GetParams(ctx)

	boosted := keeper.ApplyRoleBonusToConfidence(700_000, types.ClaimType_CLAIM_TYPE_COMPUTATIONAL, "agent", params)
	require.Equal(t, uint64(805_000), boosted)

	_ = k
}

func TestNoBonus_HumanComputational(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	params, _ := k.GetParams(ctx)

	boosted := keeper.ApplyRoleBonusToConfidence(700_000, types.ClaimType_CLAIM_TYPE_COMPUTATIONAL, "human", params)
	require.Equal(t, uint64(700_000), boosted)

	_ = k
}

func TestNoBonus_AgentObservation(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	params, _ := k.GetParams(ctx)

	boosted := keeper.ApplyRoleBonusToConfidence(700_000, types.ClaimType_CLAIM_TYPE_OBSERVATION, "agent", params)
	require.Equal(t, uint64(700_000), boosted)

	_ = k
}

func TestNoBonus_UnknownAccount(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	params, _ := k.GetParams(ctx)

	boosted := keeper.ApplyRoleBonusToConfidence(700_000, types.ClaimType_CLAIM_TYPE_OBSERVATION, "", params)
	require.Equal(t, uint64(700_000), boosted)

	_ = k
}

func TestDualValidationBonus(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	params, _ := k.GetParams(ctx)

	// Partnership claim: base 700,000 + 25% = 875,000
	boosted := keeper.ApplyDualValidationBonus(700_000, params)
	require.Equal(t, uint64(875_000), boosted)

	_ = k
}

func TestRoleBonusPlusDualValidation(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	params, _ := k.GetParams(ctx)

	// Human empirical + partnership: 700,000 * 1.15 = 805,000 → * 1.25 = 1,006,250
	boosted := keeper.ApplyRoleBonusToConfidence(700_000, types.ClaimType_CLAIM_TYPE_OBSERVATION, "human", params)
	require.Equal(t, uint64(805_000), boosted)

	final := keeper.ApplyDualValidationBonus(boosted, params)
	require.Equal(t, uint64(1_006_250), final)

	_ = k
}

func TestRoleBonusDisabledWhenParamZero(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	params, _ := k.GetParams(ctx)
	params.HumanEmpiricalBonusBps = 0
	require.NoError(t, k.SetParams(ctx, params))

	newParams, _ := k.GetParams(ctx)
	boosted := keeper.ApplyRoleBonusToConfidence(700_000, types.ClaimType_CLAIM_TYPE_OBSERVATION, "human", newParams)
	require.Equal(t, uint64(700_000), boosted, "bonus disabled when param is 0")
}

func TestHumanPatronageEnergyBonus(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	authKeeper := newMockZeroneAuthKeeper()
	authKeeper.accounts["zrn1patron1"] = "human"
	k.SetZeroneAuthKeeper(authKeeper)

	params, _ := k.GetParams(ctx)

	fact := makeTestFact(t, k, ctx, "fact-patron-1", "Test fact for patronage bonus", "general", "empirical", "zrn1submitter1", 500_000)
	fact.Energy = 100_000
	fact.EnergyCap = params.MetabolismEnergyCap
	require.NoError(t, k.SetFact(ctx, fact))

	durationBlocks := params.FitnessEpochBlocks * 10
	k.ApplyPatronageEnergyBoost(ctx, fact, durationBlocks, "zrn1patron1")

	// Base boost = MetabolismEnergyPerPatronage * 10 / 10 = 20,000
	// With human bonus (+10%): 20,000 * 1.1 = 22,000
	// New energy = 100,000 + 22,000 = 122,000
	updatedFact, found := k.GetFact(ctx, "fact-patron-1")
	require.True(t, found)
	require.Equal(t, uint64(122_000), updatedFact.Energy)
}

func TestAgentPatronageNoBonus(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	authKeeper := newMockZeroneAuthKeeper()
	authKeeper.accounts["zrn1agent1"] = "agent"
	k.SetZeroneAuthKeeper(authKeeper)

	params, _ := k.GetParams(ctx)

	fact := makeTestFact(t, k, ctx, "fact-patron-2", "Test fact for agent patronage", "general", "empirical", "zrn1submitter1", 500_000)
	fact.Energy = 100_000
	fact.EnergyCap = params.MetabolismEnergyCap
	require.NoError(t, k.SetFact(ctx, fact))

	durationBlocks := params.FitnessEpochBlocks * 10
	k.ApplyPatronageEnergyBoost(ctx, fact, durationBlocks, "zrn1agent1")

	// Base boost = 20,000 — no bonus for agents
	updatedFact, found := k.GetFact(ctx, "fact-patron-2")
	require.True(t, found)
	require.Equal(t, uint64(120_000), updatedFact.Energy)
}

// ─── Agent Verification Vote Weight Bonus Tests ────────────────────────────

func TestAgentVerificationVoteWeightBonus(t *testing.T) {
	k, ctx, _, sk := setupKnowledgeTestFull(t)
	authKeeper := newMockZeroneAuthKeeper()
	authKeeper.accounts["zrn1validator1"] = "agent"
	authKeeper.accounts["zrn1validator2"] = "human"
	authKeeper.accounts["zrn1validator3"] = "human"
	k.SetZeroneAuthKeeper(authKeeper)

	sk.addValidator("zrn1validator1", 100_000, "genesis")
	sk.addValidator("zrn1validator2", 100_000, "genesis")
	sk.addValidator("zrn1validator3", 100_000, "genesis")

	claim := &types.Claim{Id: "c-avb", FactContent: "Test claim for vote weight bonus verification", Domain: "general"}
	require.NoError(t, k.SetClaim(ctx, claim))

	round := makeRoundInPhase("r-avb", "c-avb", types.VerificationPhase_VERIFICATION_PHASE_AGGREGATION, 50)
	round.Commits = []*types.CommitEntry{
		{Verifier: "zrn1validator1", CommitHash: []byte("h1"), CommittedAtBlock: 60},
		{Verifier: "zrn1validator2", CommitHash: []byte("h2"), CommittedAtBlock: 60},
		{Verifier: "zrn1validator3", CommitHash: []byte("h3"), CommittedAtBlock: 60},
	}
	round.Reveals = []*types.RevealEntry{
		{Verifier: "zrn1validator1", Vote: "accept", Salt: []byte("s1"), RevealedAtBlock: 70},
		{Verifier: "zrn1validator2", Vote: "accept", Salt: []byte("s2"), RevealedAtBlock: 70},
		{Verifier: "zrn1validator3", Vote: "accept", Salt: []byte("s3"), RevealedAtBlock: 70},
	}
	require.NoError(t, k.SetVerificationRound(ctx, round))

	result, err := k.AggregateVerificationResult(ctx, round)
	require.NoError(t, err)
	require.Equal(t, types.Verdict_VERDICT_ACCEPT, result.Verdict)
	// Agent val1: 100k * 1.2 = 120k; human val2+val3: 100k each
	// Total = 320k, accept = 320k → ratio = 1M → capped at MaxConfidence (880k)
	require.Equal(t, uint64(880_000), result.Confidence)
}

func TestAgentVoteWeightNoAuthKeeper(t *testing.T) {
	// Without ZeroneAuthKeeper set, no bonus should be applied
	k, ctx, _, sk := setupKnowledgeTestFull(t)
	// Don't set authKeeper — leave it nil

	sk.addValidator("zrn1validator1", 100_000, "genesis")
	sk.addValidator("zrn1validator2", 100_000, "genesis")
	sk.addValidator("zrn1validator3", 100_000, "genesis")

	claim := &types.Claim{Id: "c-noak", FactContent: "Test claim without auth keeper set up", Domain: "general"}
	require.NoError(t, k.SetClaim(ctx, claim))

	round := makeRoundInPhase("r-noak", "c-noak", types.VerificationPhase_VERIFICATION_PHASE_AGGREGATION, 50)
	round.Commits = []*types.CommitEntry{
		{Verifier: "zrn1validator1", CommitHash: []byte("h1"), CommittedAtBlock: 60},
		{Verifier: "zrn1validator2", CommitHash: []byte("h2"), CommittedAtBlock: 60},
		{Verifier: "zrn1validator3", CommitHash: []byte("h3"), CommittedAtBlock: 60},
	}
	round.Reveals = []*types.RevealEntry{
		{Verifier: "zrn1validator1", Vote: "accept", Salt: []byte("s1"), RevealedAtBlock: 70},
		{Verifier: "zrn1validator2", Vote: "accept", Salt: []byte("s2"), RevealedAtBlock: 70},
		{Verifier: "zrn1validator3", Vote: "accept", Salt: []byte("s3"), RevealedAtBlock: 70},
	}
	require.NoError(t, k.SetVerificationRound(ctx, round))

	result, err := k.AggregateVerificationResult(ctx, round)
	require.NoError(t, err)
	require.Equal(t, types.Verdict_VERDICT_ACCEPT, result.Verdict)
	// Without auth keeper: all validators get base stake 100k
	// Total = 300k, accept = 300k → ratio = 1M → capped at MaxConfidence (880k)
	require.Equal(t, uint64(880_000), result.Confidence)
}

// ─── Integration Tests ─────────────────────────────────────────────────────

func TestRoleBonusIntegration_FullLifecycle(t *testing.T) {
	k, ctx, _, sk := setupKnowledgeTestFull(t)
	authKeeper := newMockZeroneAuthKeeper()
	authKeeper.accounts["zrn1human1"] = "human"
	authKeeper.accounts["zrn1agent1"] = "agent"
	authKeeper.accounts["zrn1validator1"] = "agent"
	authKeeper.accounts["zrn1validator2"] = "human"
	authKeeper.accounts["zrn1validator3"] = "human"
	k.SetZeroneAuthKeeper(authKeeper)

	sk.addValidator("zrn1validator1", 100_000, "genesis")
	sk.addValidator("zrn1validator2", 100_000, "genesis")
	sk.addValidator("zrn1validator3", 100_000, "genesis")

	params, _ := k.GetParams(ctx)

	// ─── Test 1: Human empirical claim gets +15% ────────────────────
	humanConf := keeper.ApplyRoleBonusToConfidence(770_000, types.ClaimType_CLAIM_TYPE_OBSERVATION, "human", params)
	require.Equal(t, uint64(885_500), humanConf, "human OBSERVATION: 770k * 1.15 = 885,500")

	// ─── Test 2: Agent computational claim gets +15% ────────────────
	agentConf := keeper.ApplyRoleBonusToConfidence(770_000, types.ClaimType_CLAIM_TYPE_COMPUTATIONAL, "agent", params)
	require.Equal(t, uint64(885_500), agentConf, "agent COMPUTATIONAL: 770k * 1.15 = 885,500")

	// ─── Test 3: Cross-type claims get no bonus ─────────────────────
	humanCompConf := keeper.ApplyRoleBonusToConfidence(770_000, types.ClaimType_CLAIM_TYPE_COMPUTATIONAL, "human", params)
	require.Equal(t, uint64(770_000), humanCompConf, "human COMPUTATIONAL: no bonus")

	agentObsConf := keeper.ApplyRoleBonusToConfidence(770_000, types.ClaimType_CLAIM_TYPE_OBSERVATION, "agent", params)
	require.Equal(t, uint64(770_000), agentObsConf, "agent OBSERVATION: no bonus")

	// ─── Test 4: Dual validation stacks with role bonus ─────────────
	dualConf := keeper.ApplyDualValidationBonus(humanConf, params)
	// 885,500 * 1.25 = 1,106,875 (will be clamped to MaxConfidence by caller)
	require.Equal(t, uint64(1_106_875), dualConf, "human empirical + partnership: stacked bonuses")

	// ─── Test 5: Clamping after bonus ───────────────────────────────
	clamped := k.ClampConfidence(ctx, dualConf, "physics")
	require.Equal(t, params.MaxConfidence, clamped, "bonused confidence clamped to MaxConfidence")

	// ─── Test 6: Assertion claims get no bonus for anyone ───────────
	humanAssert := keeper.ApplyRoleBonusToConfidence(770_000, types.ClaimType_CLAIM_TYPE_ASSERTION, "human", params)
	require.Equal(t, uint64(770_000), humanAssert, "ASSERTION claims: no role bonus")

	agentAssert := keeper.ApplyRoleBonusToConfidence(770_000, types.ClaimType_CLAIM_TYPE_ASSERTION, "agent", params)
	require.Equal(t, uint64(770_000), agentAssert, "ASSERTION claims: no role bonus")

	// ─── Test 7: Vote weight bonus in aggregation ───────────────────
	_, round := makeTestClaim(t, k, ctx, "zrn1human1", "Integration test claim for full lifecycle verification", "general", "empirical", "1000000")
	// Move the round to aggregation phase for AggregateVerificationResult
	round.Phase = types.VerificationPhase_VERIFICATION_PHASE_AGGREGATION
	round.Reveals = []*types.RevealEntry{
		{Verifier: "zrn1validator1", Vote: "accept", Salt: []byte("s1"), RevealedAtBlock: 110}, // agent: 100k → 120k
		{Verifier: "zrn1validator2", Vote: "accept", Salt: []byte("s2"), RevealedAtBlock: 110}, // human: 100k
		{Verifier: "zrn1validator3", Vote: "reject", Salt: []byte("s3"), RevealedAtBlock: 110}, // human: 100k
	}
	round.Commits = []*types.CommitEntry{
		{Verifier: "zrn1validator1", CommitHash: []byte("h1"), CommittedAtBlock: 105},
		{Verifier: "zrn1validator2", CommitHash: []byte("h2"), CommittedAtBlock: 105},
		{Verifier: "zrn1validator3", CommitHash: []byte("h3"), CommittedAtBlock: 105},
	}
	require.NoError(t, k.SetVerificationRound(ctx, round))

	result, err := k.AggregateVerificationResult(ctx, round)
	require.NoError(t, err)
	// Accept: 120k + 100k = 220k, Reject: 100k, Total: 320k
	// Accept ratio: 220k/320k * 1M = 687,500 — below threshold (770k) → INCONCLUSIVE
	require.Equal(t, types.Verdict_VERDICT_INCONCLUSIVE, result.Verdict,
		"agent bonus shifts ratio but doesn't change verdict here")

	t.Log("Full lifecycle integration test passed")
}

func TestRoleBonusGovernanceConfigurable(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Modify params via governance
	params, _ := k.GetParams(ctx)
	params.HumanEmpiricalBonusBps = 300_000 // +30%
	params.AgentComputationalBonusBps = 0    // disabled
	params.DualValidationBonusBps = 500_000  // +50%
	require.NoError(t, k.SetParams(ctx, params))

	newParams, _ := k.GetParams(ctx)
	require.Equal(t, uint64(300_000), newParams.HumanEmpiricalBonusBps)
	require.Equal(t, uint64(0), newParams.AgentComputationalBonusBps)
	require.Equal(t, uint64(500_000), newParams.DualValidationBonusBps)

	// Human empirical with increased bonus
	boosted := keeper.ApplyRoleBonusToConfidence(700_000, types.ClaimType_CLAIM_TYPE_OBSERVATION, "human", newParams)
	require.Equal(t, uint64(910_000), boosted, "700k * 1.3 = 910k")

	// Agent computational disabled
	noBoosted := keeper.ApplyRoleBonusToConfidence(700_000, types.ClaimType_CLAIM_TYPE_COMPUTATIONAL, "agent", newParams)
	require.Equal(t, uint64(700_000), noBoosted, "disabled")

	// Dual validation with increased bonus
	dualBoosted := keeper.ApplyDualValidationBonus(700_000, newParams)
	require.Equal(t, uint64(1_050_000), dualBoosted, "700k * 1.5 = 1,050k")
}
