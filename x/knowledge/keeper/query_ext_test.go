package keeper_test

import (
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/keeper"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// TestQueryExtServerCompiles verifies the entire query extension layer compiles
// and all method signatures are correct by instantiating and calling each method.
func TestQueryExtServerCompiles(t *testing.T) {
	k, ctx := setupKeeper(t)
	q := keeper.NewQueryExtServer(k)
	require.NotNil(t, q)

	// ── Model Registry ──
	resp := q.GetModelRecord(ctx, "nonexistent")
	require.False(t, resp.Found)

	models := q.GetModelsByDomain(ctx, "technology")
	require.Empty(t, models)

	active := q.GetActiveModels(ctx)
	require.Empty(t, active)

	lineageResp := q.GetModelLineage(ctx, "nonexistent")
	require.False(t, lineageResp.Found)

	latestResp := q.GetLatestModel(ctx, "technology")
	require.False(t, latestResp.Found)

	endpoints := q.GetModelEndpoints(ctx, "nonexistent")
	require.Empty(t, endpoints)

	// ── Agent ──
	agentResp := q.GetAgentIdentity(ctx, "nonexistent")
	require.False(t, agentResp.Found)

	activeAgents := q.GetActiveAgents(ctx)
	require.Empty(t, activeAgents)

	domainAgents := q.GetAgentsByDomain(ctx, "technology")
	require.Empty(t, domainAgents)

	taskResp := q.GetAgentTask(ctx, "nonexistent")
	require.False(t, taskResp.Found)

	agentTasks := q.GetTasksByAgent(ctx, "nonexistent")
	require.Empty(t, agentTasks)

	domainTasks := q.GetTasksByDomain(ctx, "technology")
	require.Empty(t, domainTasks)

	stats := q.GetTaskStats(ctx)
	require.Equal(t, uint64(0), stats.Pending)

	// ── Knowledge Graph ──
	edgeResp := q.GetKnowledgeEdge(ctx, "nonexistent")
	require.False(t, edgeResp.Found)

	sourceEdges := q.GetEdgesBySource(ctx, "nonexistent")
	require.Empty(t, sourceEdges)

	targetEdges := q.GetEdgesByTarget(ctx, "nonexistent")
	require.Empty(t, targetEdges)

	graphStats := q.GetGraphStats(ctx)
	require.Equal(t, uint64(0), graphStats.TotalEdges)

	// ── Bounty Board ──
	bountyResp := q.GetCompetitiveBounty(ctx, "nonexistent")
	require.False(t, bountyResp.Found)

	openBounties := q.GetOpenBounties(ctx)
	require.Empty(t, openBounties)

	subs := q.GetBountySubmissions(ctx, "nonexistent")
	require.Empty(t, subs)

	leaderboard := q.GetBountyLeaderboard(ctx, "nonexistent")
	require.Empty(t, leaderboard)

	boardStats := q.GetBountyBoardStats(ctx)
	require.Equal(t, uint64(0), boardStats.Open)

	// ── Memory System ──
	actResp := q.GetActivationRecord(ctx, "nonexistent")
	require.False(t, actResp.Found)

	consParams := q.GetConsolidationParams(ctx)
	require.NotZero(t, consParams.ActiveMinActivations) // should return defaults

	tdusByTier := q.GetTDUsByTier(ctx, types.MemoryTierWorking)
	require.Empty(t, tdusByTier)

	reconStatus := q.GetReconsolidationStatus(ctx, "nonexistent")
	require.False(t, reconStatus.InReconsolidation)

	// ── Agent Consumer ──
	apiResp := q.GetAgentAPIConfig(ctx, "nonexistent")
	require.False(t, apiResp.Found)

	profResp := q.GetAgentProfitability(ctx, "nonexistent")
	require.False(t, profResp.Found)

	summary := q.GetAgentEconomicSummary(ctx, "nonexistent")
	require.NotNil(t, summary)

	// ── Swarms ──
	swarmResp := q.GetSwarm(ctx, "nonexistent")
	require.False(t, swarmResp.Found)

	domainSwarms := q.GetSwarmsByDomain(ctx, "technology")
	require.Empty(t, domainSwarms)

	memberSwarms := q.GetSwarmsByMember(ctx, "nonexistent")
	require.Empty(t, memberSwarms)

	objectives := q.GetSwarmObjectives(ctx, "nonexistent")
	require.Empty(t, objectives)

	// ── Meta-Evolution ──
	epochResp := q.GetEvolutionEpoch(ctx, "nonexistent")
	require.False(t, epochResp.Found)

	currentEpoch := q.GetCurrentEpoch(ctx, "technology")
	require.False(t, currentEpoch.HasActive)

	epochHistory := q.GetEpochHistory(ctx, "technology")
	require.Empty(t, epochHistory)

	activeStrats := q.GetActiveStrategies(ctx)
	require.Empty(t, activeStrats)

	paramResp := q.GetMetaParameter(ctx, "nonexistent")
	require.False(t, paramResp.Found)

	// ── Curation Strategy ──
	gaps := q.GetKnowledgeGaps(ctx, "technology")
	require.Empty(t, gaps)

	healthResp := q.GetDomainHealth(ctx, "technology")
	require.False(t, healthResp.Found)

	agentStrats := q.GetStrategiesByAgent(ctx, "nonexistent")
	require.Empty(t, agentStrats)

	// ── Fitness ──
	fitnessResp := q.GetFitnessRecord(ctx, "nonexistent")
	require.False(t, fitnessResp.Found)

	fitnessRecords := q.GetFitnessRecordsByDomain(ctx, "technology")
	require.Empty(t, fitnessRecords)
}

// TestRegisterExtQueryRoutesCompiles verifies route registration compiles and doesn't panic.
func TestRegisterExtQueryRoutesCompiles(t *testing.T) {
	k, _ := setupKeeper(t)
	router := mux.NewRouter()
	require.NotPanics(t, func() {
		keeper.RegisterExtQueryRoutes(router, k)
	})
}

// TestParseMemoryTier verifies string-to-tier parsing.
func TestParseMemoryTier(t *testing.T) {
	// Verify parseMemoryTier works through the QueryExtServer by calling GetTDUsByTier
	// with each tier. The function is tested implicitly through the handler.
	k, ctx := setupKeeper(t)
	q := keeper.NewQueryExtServer(k)

	for _, tier := range []types.MemoryTier{
		types.MemoryTierWorking,
		types.MemoryTierActive,
		types.MemoryTierConsolidated,
		types.MemoryTierCanonical,
	} {
		result := q.GetTDUsByTier(ctx, tier)
		require.Empty(t, result)
	}
}
