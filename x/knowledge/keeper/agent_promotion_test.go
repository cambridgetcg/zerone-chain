package keeper_test

import (
	"context"
	"fmt"
	"testing"

	sdkmath "cosmossdk.io/math"
	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/keeper"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// publishModelForPromotion creates a model that meets promotion criteria (benchmark ≥ 0.6, TDUs ≥ 50).
func publishModelForPromotion(t *testing.T, k keeper.Keeper, ctx context.Context, suffix string) string {
	t.Helper()
	attestHash := fmt.Sprintf("promo-attest-%s", suffix)
	seedTrainingRecord(t, k, ctx, attestHash)

	msg := &types.MsgPublishModel{
		Publisher:        testAddr,
		Name:             fmt.Sprintf("promo-model-%s", suffix),
		Domain:           "code/go",
		TrainingRecordID: attestHash,
		TDUIDs:           generateTDUIDs(55), // ≥ 50
		DatasetIDs:       []string{"ds-promo"},
		BenchmarkScore:   "0.750000000000000000", // ≥ 0.6
		FitnessWeighted:  "0.600000000000000000",
		TEEAttestation:   fmt.Sprintf("promo-tee-%s", suffix),
		ModelHash:        fmt.Sprintf("promo-hash-%s", suffix),
	}

	resp, err := k.PublishModel(ctx, msg)
	require.NoError(t, err)
	return resp.ModelID
}

// ─── Test: Happy Path Promotion ─────────────────────────────────────────────

func TestPromoteModelToAgent_HappyPath(t *testing.T) {
	k, ctx := setupKeeper(t)
	modelID := publishModelForPromotion(t, k, ctx, "happy")

	msg := &types.MsgPromoteModel{
		Sponsor: testAddr,
		ModelID: modelID,
		Stake:   "10000000", // 10 ZRN
	}

	resp, err := k.PromoteModelToAgent(ctx, msg)
	require.NoError(t, err)
	require.NotEmpty(t, resp.AgentID)
	require.NotEmpty(t, resp.Address)

	// Verify stored agent.
	agent, found := k.GetAgentIdentity(ctx, resp.AgentID)
	require.True(t, found)
	require.Equal(t, modelID, agent.ModelID)
	require.Equal(t, "code/go", agent.Domain)
	require.Equal(t, types.AgentStatusActive, agent.Status)
	require.True(t, agent.CanSubmit)
	require.True(t, agent.CanReview) // benchmark 0.75 ≥ 0.6
	require.Equal(t, uint64(0), agent.Generation)

	// Reputation = benchmark × 0.5 = 0.75 × 0.5 = 0.375
	rep := agent.GetReputation()
	expected := sdkmath.LegacyNewDecWithPrec(375, 3) // 0.375
	require.Equal(t, expected.String(), rep.String())
}

// ─── Test: Reject — Benchmark Too Low ───────────────────────────────────────

func TestPromoteModel_RejectLowBenchmark(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Publish with benchmark 0.5 (passes publish gate but fails promote gate of 0.6)
	attestHash := "promo-low-bench"
	seedTrainingRecord(t, k, ctx, attestHash)
	msg := &types.MsgPublishModel{
		Publisher:        testAddr,
		Name:             "low-bench-model",
		Domain:           "code/go",
		TrainingRecordID: attestHash,
		TDUIDs:           generateTDUIDs(55),
		DatasetIDs:       []string{"ds-1"},
		BenchmarkScore:   "0.500000000000000000", // passes publish (≥0.3) but fails promote (≥0.6)
		FitnessWeighted:  "0.600000000000000000",
		TEEAttestation:   "low-bench-tee",
		ModelHash:        "low-bench-hash",
	}
	resp, err := k.PublishModel(ctx, msg)
	require.NoError(t, err)

	promoteMsg := &types.MsgPromoteModel{
		Sponsor: testAddr,
		ModelID: resp.ModelID,
		Stake:   "10000000",
	}
	_, err = k.PromoteModelToAgent(ctx, promoteMsg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "below")
}

// ─── Test: Reject — Insufficient TDUs ───────────────────────────────────────

func TestPromoteModel_RejectInsufficientTDUs(t *testing.T) {
	k, ctx := setupKeeper(t)

	attestHash := "promo-few-tdus"
	seedTrainingRecord(t, k, ctx, attestHash)
	msg := &types.MsgPublishModel{
		Publisher:        testAddr,
		Name:             "few-tdu-model",
		Domain:           "code/go",
		TrainingRecordID: attestHash,
		TDUIDs:           generateTDUIDs(15), // passes publish (≥10) but fails promote (≥50)
		DatasetIDs:       []string{"ds-1"},
		BenchmarkScore:   "0.700000000000000000",
		FitnessWeighted:  "0.600000000000000000",
		TEEAttestation:   "few-tdu-tee",
		ModelHash:        "few-tdu-hash",
	}
	resp, err := k.PublishModel(ctx, msg)
	require.NoError(t, err)

	promoteMsg := &types.MsgPromoteModel{
		Sponsor: testAddr,
		ModelID: resp.ModelID,
		Stake:   "10000000",
	}
	_, err = k.PromoteModelToAgent(ctx, promoteMsg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "TDU count")
}

// ─── Test: Reject — Insufficient Stake ──────────────────────────────────────

func TestPromoteModel_RejectInsufficientStake(t *testing.T) {
	k, ctx := setupKeeper(t)
	modelID := publishModelForPromotion(t, k, ctx, "low-stake")

	msg := &types.MsgPromoteModel{
		Sponsor: testAddr,
		ModelID: modelID,
		Stake:   "5000000", // 5 ZRN < 10 ZRN minimum
	}

	_, err := k.PromoteModelToAgent(ctx, msg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "stake")
}

// ─── Test: Reject — Model Already Promoted ──────────────────────────────────

func TestPromoteModel_RejectAlreadyPromoted(t *testing.T) {
	k, ctx := setupKeeper(t)
	modelID := publishModelForPromotion(t, k, ctx, "dup")

	msg := &types.MsgPromoteModel{
		Sponsor: testAddr,
		ModelID: modelID,
		Stake:   "10000000",
	}

	_, err := k.PromoteModelToAgent(ctx, msg)
	require.NoError(t, err)

	// Try again.
	_, err = k.PromoteModelToAgent(ctx, msg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "already")
}

// ─── Test: Agent Suspension — Reputation Drop ───────────────────────────────

func TestAgentSuspension(t *testing.T) {
	k, ctx := setupKeeper(t)
	modelID := publishModelForPromotion(t, k, ctx, "suspend")

	resp, err := k.PromoteModelToAgent(ctx, &types.MsgPromoteModel{
		Sponsor: testAddr,
		ModelID: modelID,
		Stake:   "10000000",
	})
	require.NoError(t, err)

	// Manually suspend.
	err = k.SuspendAgent(ctx, resp.AgentID, "test suspension")
	require.NoError(t, err)

	agent, found := k.GetAgentIdentity(ctx, resp.AgentID)
	require.True(t, found)
	require.Equal(t, types.AgentStatusSuspended, agent.Status)

	// Should not be in active agents.
	active := k.GetActiveAgents(ctx)
	for _, a := range active {
		require.NotEqual(t, resp.AgentID, a.AgentID)
	}
}

// ─── Test: Auto-Suspend on Low Reputation ───────────────────────────────────

func TestAgentAutoSuspend(t *testing.T) {
	k, ctx := setupKeeper(t)
	modelID := publishModelForPromotion(t, k, ctx, "auto-suspend")

	resp, err := k.PromoteModelToAgent(ctx, &types.MsgPromoteModel{
		Sponsor: testAddr,
		ModelID: modelID,
		Stake:   "10000000",
	})
	require.NoError(t, err)

	// Drop reputation below 0.2 threshold.
	err = k.UpdateAgentReputation(ctx, resp.AgentID, sdkmath.LegacyNewDecWithPrec(15, 2)) // 0.15
	require.NoError(t, err)

	agent, found := k.GetAgentIdentity(ctx, resp.AgentID)
	require.True(t, found)
	require.Equal(t, types.AgentStatusSuspended, agent.Status)
}

// ─── Test: Agent Retirement — Model Deprecated ──────────────────────────────

func TestAgentRetirement(t *testing.T) {
	k, ctx := setupKeeper(t)
	modelID := publishModelForPromotion(t, k, ctx, "retire")

	resp, err := k.PromoteModelToAgent(ctx, &types.MsgPromoteModel{
		Sponsor: testAddr,
		ModelID: modelID,
		Stake:   "10000000",
	})
	require.NoError(t, err)

	err = k.RetireAgent(ctx, resp.AgentID)
	require.NoError(t, err)

	agent, found := k.GetAgentIdentity(ctx, resp.AgentID)
	require.True(t, found)
	require.Equal(t, types.AgentStatusRetired, agent.Status)
}

// ─── Test: Domain Query ─────────────────────────────────────────────────────

func TestGetAgentsByDomain(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Create 2 agents in code/go.
	for i := 0; i < 2; i++ {
		modelID := publishModelForPromotion(t, k, ctx, fmt.Sprintf("domain-%d", i))
		_, err := k.PromoteModelToAgent(ctx, &types.MsgPromoteModel{
			Sponsor: testAddr,
			ModelID: modelID,
			Stake:   "10000000",
		})
		require.NoError(t, err)
	}

	agents := k.GetAgentsByDomain(ctx, "code/go")
	require.Len(t, agents, 2)

	agents = k.GetAgentsByDomain(ctx, "code/rust")
	require.Len(t, agents, 0)
}

// ─── Test: Generation Stats ─────────────────────────────────────────────────

func TestGenerationStats(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Create 3 gen-0 agents.
	for i := 0; i < 3; i++ {
		modelID := publishModelForPromotion(t, k, ctx, fmt.Sprintf("gen-%d", i))
		_, err := k.PromoteModelToAgent(ctx, &types.MsgPromoteModel{
			Sponsor: testAddr,
			ModelID: modelID,
			Stake:   "10000000",
		})
		require.NoError(t, err)
	}

	stats := k.GetGenerationStats(ctx)
	require.Equal(t, uint64(3), stats[0])
}

// ─── Test: Earnings Tracking ────────────────────────────────────────────────

func TestAgentEarnings(t *testing.T) {
	k, ctx := setupKeeper(t)
	modelID := publishModelForPromotion(t, k, ctx, "earnings")

	resp, err := k.PromoteModelToAgent(ctx, &types.MsgPromoteModel{
		Sponsor: testAddr,
		ModelID: modelID,
		Stake:   "10000000",
	})
	require.NoError(t, err)

	// Add earnings.
	err = k.AddAgentEarnings(ctx, resp.AgentID, sdkmath.NewInt(500000))
	require.NoError(t, err)
	err = k.AddAgentEarnings(ctx, resp.AgentID, sdkmath.NewInt(300000))
	require.NoError(t, err)

	agent, found := k.GetAgentIdentity(ctx, resp.AgentID)
	require.True(t, found)
	require.Equal(t, sdkmath.NewInt(800000), agent.GetEarningsTotal())
}

// ─── Test: Task Counting ────────────────────────────────────────────────────

func TestAgentTaskCounting(t *testing.T) {
	k, ctx := setupKeeper(t)
	modelID := publishModelForPromotion(t, k, ctx, "tasks")

	resp, err := k.PromoteModelToAgent(ctx, &types.MsgPromoteModel{
		Sponsor: testAddr,
		ModelID: modelID,
		Stake:   "10000000",
	})
	require.NoError(t, err)

	for i := 0; i < 5; i++ {
		err = k.RecordAgentAction(ctx, resp.AgentID)
		require.NoError(t, err)
	}

	agent, found := k.GetAgentIdentity(ctx, resp.AgentID)
	require.True(t, found)
	require.Equal(t, uint64(5), agent.TasksComplete)
}

// ─── Test: GetAgentByModel ──────────────────────────────────────────────────

func TestGetAgentByModel(t *testing.T) {
	k, ctx := setupKeeper(t)
	modelID := publishModelForPromotion(t, k, ctx, "by-model")

	resp, err := k.PromoteModelToAgent(ctx, &types.MsgPromoteModel{
		Sponsor: testAddr,
		ModelID: modelID,
		Stake:   "10000000",
	})
	require.NoError(t, err)

	agent, found := k.GetAgentByModel(ctx, modelID)
	require.True(t, found)
	require.Equal(t, resp.AgentID, agent.AgentID)

	_, found = k.GetAgentByModel(ctx, "nonexistent-model")
	require.False(t, found)
}
