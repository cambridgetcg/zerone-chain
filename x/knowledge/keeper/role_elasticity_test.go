package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── CRUD Tests ──────────────────────────────────────────────────────────

func TestDomainRoleRecord_SetGet(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	record := &types.DomainRoleRecord{
		Domain:              "physics",
		AgentCorrectCalls:   8,
		AgentIncorrectCalls: 2,
		HumanCorrectCalls:   6,
		HumanIncorrectCalls: 4,
		LastUpdated:         100,
	}
	require.NoError(t, k.SetDomainRoleRecord(ctx, record))

	got, found := k.GetDomainRoleRecord(ctx, "physics")
	require.True(t, found)
	require.Equal(t, record.AgentCorrectCalls, got.AgentCorrectCalls)
	require.Equal(t, record.HumanIncorrectCalls, got.HumanIncorrectCalls)
}

func TestDomainRoleRecord_NotFound(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	_, found := k.GetDomainRoleRecord(ctx, "nonexistent")
	require.False(t, found)
}

// ─── Elasticity Calculation Tests ────────────────────────────────────────

func TestGetRoleElasticity_BelowMinCalls(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	record := &types.DomainRoleRecord{
		Domain:            "physics",
		AgentCorrectCalls: 4, AgentIncorrectCalls: 1,
		HumanCorrectCalls: 3, HumanIncorrectCalls: 2,
		LastUpdated: 100,
	}
	require.NoError(t, k.SetDomainRoleRecord(ctx, record))

	params, _ := k.GetParams(ctx)
	agentBonus, humanBonus := k.GetRoleElasticity(ctx, "physics")
	require.Equal(t, params.AgentVerificationBonusBps, agentBonus)
	require.Equal(t, params.HumanPatronageBonusBps, humanBonus)
}

func TestGetRoleElasticity_EqualAccuracy(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	record := &types.DomainRoleRecord{
		Domain:            "physics",
		AgentCorrectCalls: 16, AgentIncorrectCalls: 4,
		HumanCorrectCalls: 16, HumanIncorrectCalls: 4,
		LastUpdated: 100,
	}
	require.NoError(t, k.SetDomainRoleRecord(ctx, record))

	params, _ := k.GetParams(ctx)
	agentBonus, humanBonus := k.GetRoleElasticity(ctx, "physics")
	require.Equal(t, params.AgentVerificationBonusBps, agentBonus)
	require.Equal(t, params.HumanPatronageBonusBps, humanBonus)
}

func TestGetRoleElasticity_AgentDominant(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	record := &types.DomainRoleRecord{
		Domain:            "physics",
		AgentCorrectCalls: 18, AgentIncorrectCalls: 2,
		HumanCorrectCalls: 12, HumanIncorrectCalls: 8,
		LastUpdated: 100,
	}
	require.NoError(t, k.SetDomainRoleRecord(ctx, record))

	params, _ := k.GetParams(ctx)
	agentBonus, humanBonus := k.GetRoleElasticity(ctx, "physics")
	require.Greater(t, agentBonus, params.AgentVerificationBonusBps)
	require.Less(t, humanBonus, params.HumanPatronageBonusBps)
}

func TestGetRoleElasticity_BoundedMax(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Agent accuracy 100%, human accuracy 0% → multiplier hits max clamp.
	record := &types.DomainRoleRecord{
		Domain:            "physics",
		AgentCorrectCalls: 20, AgentIncorrectCalls: 0,
		HumanCorrectCalls: 0, HumanIncorrectCalls: 20,
		LastUpdated: 100,
	}
	require.NoError(t, k.SetDomainRoleRecord(ctx, record))

	params, _ := k.GetParams(ctx)
	agentBonus, humanBonus := k.GetRoleElasticity(ctx, "physics")
	require.Equal(t, params.AgentVerificationBonusBps*2, agentBonus)
	require.Equal(t, params.HumanPatronageBonusBps/2, humanBonus)
}

func TestGetRoleElasticity_NoDomain(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	params, _ := k.GetParams(ctx)
	agentBonus, humanBonus := k.GetRoleElasticity(ctx, "unknown_domain")
	require.Equal(t, params.AgentVerificationBonusBps, agentBonus)
	require.Equal(t, params.HumanPatronageBonusBps, humanBonus)
}

// ─── CountVotesByAccountType Tests ───────────────────────────────────────

func TestCountVotesByAccountType(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	mockAuth := newMockZeroneAuthKeeper()
	mockAuth.accounts["agent1"] = "agent"
	mockAuth.accounts["agent2"] = "agent"
	mockAuth.accounts["human1"] = "human"
	k.SetZeroneAuthKeeper(mockAuth)

	round := &types.VerificationRound{
		Id:      "round1",
		ClaimId: "claim1",
		Reveals: []*types.RevealEntry{
			{Verifier: "agent1", Vote: "accept"},
			{Verifier: "agent2", Vote: "accept"},
			{Verifier: "human1", Vote: "reject"},
		},
	}

	agentVotes, humanVotes := k.CountVotesByAccountType(ctx, round)
	require.Equal(t, uint64(2), agentVotes)
	require.Equal(t, uint64(1), humanVotes)
}

func TestCountVotesByAccountType_NoAuthKeeper(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	round := &types.VerificationRound{
		Id: "round1",
		Reveals: []*types.RevealEntry{
			{Verifier: "someone", Vote: "accept"},
		},
	}

	agentVotes, humanVotes := k.CountVotesByAccountType(ctx, round)
	require.Equal(t, uint64(0), agentVotes)
	require.Equal(t, uint64(0), humanVotes)
}

// ─── Decay Tests ─────────────────────────────────────────────────────────

func TestDecayRoleRecords(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	record := &types.DomainRoleRecord{
		Domain:              "physics",
		AgentCorrectCalls:   1000,
		AgentIncorrectCalls: 200,
		HumanCorrectCalls:   800,
		HumanIncorrectCalls: 100,
		LastUpdated:         100,
	}
	require.NoError(t, k.SetDomainRoleRecord(ctx, record))

	k.DecayRoleRecords(ctx)

	got, found := k.GetDomainRoleRecord(ctx, "physics")
	require.True(t, found)
	require.Equal(t, uint64(950), got.AgentCorrectCalls)
	require.Equal(t, uint64(190), got.AgentIncorrectCalls)
	require.Equal(t, uint64(760), got.HumanCorrectCalls)
	require.Equal(t, uint64(95), got.HumanIncorrectCalls)
}

func TestDecayRoleRecords_SmallValues(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	record := &types.DomainRoleRecord{
		Domain:            "physics",
		AgentCorrectCalls: 1,
		LastUpdated:       100,
	}
	require.NoError(t, k.SetDomainRoleRecord(ctx, record))

	k.DecayRoleRecords(ctx)

	got, found := k.GetDomainRoleRecord(ctx, "physics")
	require.True(t, found)
	require.Equal(t, uint64(0), got.AgentCorrectCalls)
}

// ─── GetRoleAccuracies Tests ─────────────────────────────────────────────

func TestGetRoleAccuracies(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	record := &types.DomainRoleRecord{
		Domain:              "physics",
		AgentCorrectCalls:   18,
		AgentIncorrectCalls: 2,
		HumanCorrectCalls:   12,
		HumanIncorrectCalls: 8,
		LastUpdated:         100,
	}
	require.NoError(t, k.SetDomainRoleRecord(ctx, record))

	agentAcc, humanAcc := k.GetRoleAccuracies(ctx, "physics")
	require.Equal(t, uint64(900_000), agentAcc)
	require.Equal(t, uint64(600_000), humanAcc)
}

func TestGetRoleAccuracies_NoCalls(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	agentAcc, humanAcc := k.GetRoleAccuracies(ctx, "unknown")
	require.Equal(t, uint64(0), agentAcc)
	require.Equal(t, uint64(0), humanAcc)
}

// ─── Vindication Role Impact Tests ──────────────────────────────────────

func TestVindicationRoleImpact_AgentMajority(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	mockAuth := newMockZeroneAuthKeeper()
	mockAuth.accounts["agent1"] = "agent"
	mockAuth.accounts["agent2"] = "agent"
	mockAuth.accounts["human1"] = "human"
	k.SetZeroneAuthKeeper(mockAuth)

	round := &types.VerificationRound{
		Id:      "round1",
		ClaimId: "claim1",
		Reveals: []*types.RevealEntry{
			{Verifier: "agent1", Vote: "accept"},
			{Verifier: "agent2", Vote: "accept"},
			{Verifier: "human1", Vote: "reject"},
		},
	}

	k.RecordVindicationRoleImpact(ctx, round, "physics")

	record, found := k.GetDomainRoleRecord(ctx, "physics")
	require.True(t, found)
	require.Equal(t, uint64(1), record.AgentIncorrectCalls)
	require.Equal(t, uint64(0), record.HumanIncorrectCalls)
}

func TestVindicationRoleImpact_HumanMajority(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	mockAuth := newMockZeroneAuthKeeper()
	mockAuth.accounts["agent1"] = "agent"
	mockAuth.accounts["human1"] = "human"
	mockAuth.accounts["human2"] = "human"
	k.SetZeroneAuthKeeper(mockAuth)

	round := &types.VerificationRound{
		Id:      "round1",
		ClaimId: "claim1",
		Reveals: []*types.RevealEntry{
			{Verifier: "agent1", Vote: "reject"},
			{Verifier: "human1", Vote: "accept"},
			{Verifier: "human2", Vote: "accept"},
		},
	}

	k.RecordVindicationRoleImpact(ctx, round, "ecology")

	record, found := k.GetDomainRoleRecord(ctx, "ecology")
	require.True(t, found)
	require.Equal(t, uint64(0), record.AgentIncorrectCalls)
	require.Equal(t, uint64(1), record.HumanIncorrectCalls)
}

// ─── Full Lifecycle Integration Test ─────────────────────────────────────

func TestFullLifecycle_VindicationUpdatesElasticity(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	mockAuth := newMockZeroneAuthKeeper()
	mockAuth.accounts["agent1"] = "agent"
	mockAuth.accounts["agent2"] = "agent"
	mockAuth.accounts["human1"] = "human"
	k.SetZeroneAuthKeeper(mockAuth)

	params, _ := k.GetParams(ctx)

	// Initially: no track record, base bonuses
	agentBonus, humanBonus := k.GetRoleElasticity(ctx, "physics")
	require.Equal(t, params.AgentVerificationBonusBps, agentBonus)
	require.Equal(t, params.HumanPatronageBonusBps, humanBonus)

	// Simulate 15 vindications where agents were the majority (and wrong)
	round := &types.VerificationRound{
		Id:      "round1",
		ClaimId: "claim1",
		Reveals: []*types.RevealEntry{
			{Verifier: "agent1", Vote: "accept"},
			{Verifier: "agent2", Vote: "accept"},
			{Verifier: "human1", Vote: "reject"},
		},
	}

	for i := 0; i < 15; i++ {
		k.RecordVindicationRoleImpact(ctx, round, "physics")
	}

	record, found := k.GetDomainRoleRecord(ctx, "physics")
	require.True(t, found)
	require.Equal(t, uint64(15), record.AgentIncorrectCalls)
	require.Equal(t, uint64(0), record.AgentCorrectCalls)

	// Now add enough human data: 10 correct
	record.HumanCorrectCalls = 10
	require.NoError(t, k.SetDomainRoleRecord(ctx, record))

	// Now elasticity activates: agent accuracy = 0%, human accuracy = 100%
	// Agent should get min (50%), human should get max (200%)
	agentBonus, humanBonus = k.GetRoleElasticity(ctx, "physics")
	require.Equal(t, params.AgentVerificationBonusBps/2, agentBonus)  // 50% of base
	require.Equal(t, params.HumanPatronageBonusBps*2, humanBonus)     // 200% of base

	// Decay should reduce counts
	k.DecayRoleRecords(ctx)
	record, _ = k.GetDomainRoleRecord(ctx, "physics")
	require.Less(t, record.AgentIncorrectCalls, uint64(15))
	require.Less(t, record.HumanCorrectCalls, uint64(10))
}
