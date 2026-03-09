package keeper

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"

	"github.com/zerone-chain/zerone/x/knowledge/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// ─── REST Route Registration ────────────────────────────────────────────────
//
// Registers custom HTTP/JSON endpoints for the sovereignty stack query layer.
// These routes are consumed by agent daemons that need to query chain state
// without proto-generated gRPC clients.
//
// All endpoints are read-only (GET) and return JSON.
// Error handling: 400 for bad params, 404 for not-found, 200 for success.

// RegisterExtQueryRoutes registers all sovereignty stack query REST routes.
// The router should be the module's REST router (typically mounted at /zerone/knowledge/v1beta1/).
func RegisterExtQueryRoutes(router *mux.Router, keeper Keeper) {
	q := NewQueryExtServer(keeper)

	// ── Model Registry (R48) ──
	router.HandleFunc("/ext/models/{model_id}", withSDKContext(q, handleGetModel)).Methods("GET")
	router.HandleFunc("/ext/models/domain/{domain}", withSDKContext(q, handleGetModelsByDomain)).Methods("GET")
	router.HandleFunc("/ext/models/active", withSDKContext(q, handleGetActiveModels)).Methods("GET")
	router.HandleFunc("/ext/models/{model_id}/lineage", withSDKContext(q, handleGetModelLineage)).Methods("GET")
	router.HandleFunc("/ext/models/domain/{domain}/latest", withSDKContext(q, handleGetLatestModel)).Methods("GET")
	router.HandleFunc("/ext/models/{model_id}/endpoints", withSDKContext(q, handleGetModelEndpoints)).Methods("GET")

	// ── Agent (R49) ──
	router.HandleFunc("/ext/agents/{agent_id}", withSDKContext(q, handleGetAgentIdentity)).Methods("GET")
	router.HandleFunc("/ext/agents/active", withSDKContext(q, handleGetActiveAgents)).Methods("GET")
	router.HandleFunc("/ext/agents/domain/{domain}", withSDKContext(q, handleGetAgentsByDomain)).Methods("GET")
	router.HandleFunc("/ext/tasks/{task_id}", withSDKContext(q, handleGetAgentTask)).Methods("GET")
	router.HandleFunc("/ext/tasks/agent/{agent_id}", withSDKContext(q, handleGetTasksByAgent)).Methods("GET")
	router.HandleFunc("/ext/tasks/domain/{domain}", withSDKContext(q, handleGetTasksByDomain)).Methods("GET")
	router.HandleFunc("/ext/tasks/stats", withSDKContext(q, handleGetTaskStats)).Methods("GET")

	// ── Knowledge Graph ──
	router.HandleFunc("/ext/graph/edges/{edge_id}", withSDKContext(q, handleGetKnowledgeEdge)).Methods("GET")
	router.HandleFunc("/ext/graph/edges/source/{source_id}", withSDKContext(q, handleGetEdgesBySource)).Methods("GET")
	router.HandleFunc("/ext/graph/edges/target/{target_id}", withSDKContext(q, handleGetEdgesByTarget)).Methods("GET")
	router.HandleFunc("/ext/graph/stats", withSDKContext(q, handleGetGraphStats)).Methods("GET")

	// ── Bounty Board ──
	router.HandleFunc("/ext/bounties/{bounty_id}", withSDKContext(q, handleGetCompetitiveBounty)).Methods("GET")
	router.HandleFunc("/ext/bounties/open", withSDKContext(q, handleGetOpenBounties)).Methods("GET")
	router.HandleFunc("/ext/bounties/{bounty_id}/submissions", withSDKContext(q, handleGetBountySubmissions)).Methods("GET")
	router.HandleFunc("/ext/bounties/{bounty_id}/leaderboard", withSDKContext(q, handleGetBountyLeaderboard)).Methods("GET")
	router.HandleFunc("/ext/bounties/stats", withSDKContext(q, handleGetBountyBoardStats)).Methods("GET")

	// ── Memory System (R50/R51) ──
	router.HandleFunc("/ext/memory/{sample_id}/activation", withSDKContext(q, handleGetActivationRecord)).Methods("GET")
	router.HandleFunc("/ext/memory/params", withSDKContext(q, handleGetConsolidationParams)).Methods("GET")
	router.HandleFunc("/ext/memory/tier/{tier}", withSDKContext(q, handleGetTDUsByTier)).Methods("GET")
	router.HandleFunc("/ext/memory/{sample_id}/reconsolidation", withSDKContext(q, handleGetReconsolidationStatus)).Methods("GET")

	// ── Agent Consumer (R51) ──
	router.HandleFunc("/ext/consumer/{agent_id}/config", withSDKContext(q, handleGetAgentAPIConfig)).Methods("GET")
	router.HandleFunc("/ext/consumer/{agent_id}/profitability", withSDKContext(q, handleGetAgentProfitability)).Methods("GET")
	router.HandleFunc("/ext/consumer/{agent_id}/summary", withSDKContext(q, handleGetAgentEconomicSummary)).Methods("GET")

	// ── Swarms (R55) ──
	router.HandleFunc("/ext/swarms/{swarm_id}", withSDKContext(q, handleGetSwarm)).Methods("GET")
	router.HandleFunc("/ext/swarms/domain/{domain}", withSDKContext(q, handleGetSwarmsByDomain)).Methods("GET")
	router.HandleFunc("/ext/swarms/member/{agent_id}", withSDKContext(q, handleGetSwarmsByMember)).Methods("GET")
	router.HandleFunc("/ext/swarms/{swarm_id}/objectives", withSDKContext(q, handleGetSwarmObjectives)).Methods("GET")

	// ── Meta-Evolution (R57) ──
	router.HandleFunc("/ext/evolution/domain/{domain}/current", withSDKContext(q, handleGetCurrentEpoch)).Methods("GET")
	router.HandleFunc("/ext/evolution/domain/{domain}/history", withSDKContext(q, handleGetEpochHistory)).Methods("GET")
	router.HandleFunc("/ext/evolution/epoch/{epoch_id}", withSDKContext(q, handleGetEvolutionEpoch)).Methods("GET")
	router.HandleFunc("/ext/evolution/strategies", withSDKContext(q, handleGetActiveStrategies)).Methods("GET")
	router.HandleFunc("/ext/evolution/params/{param_id}", withSDKContext(q, handleGetMetaParameter)).Methods("GET")

	// ── Curation Strategy (R54) ──
	router.HandleFunc("/ext/curation/gaps/{domain}", withSDKContext(q, handleGetKnowledgeGaps)).Methods("GET")
	router.HandleFunc("/ext/curation/health/{domain}", withSDKContext(q, handleGetDomainHealth)).Methods("GET")
	router.HandleFunc("/ext/curation/strategies/agent/{agent_id}", withSDKContext(q, handleGetStrategiesByAgent)).Methods("GET")

	// ── Fitness ──
	router.HandleFunc("/ext/fitness/{sample_id}", withSDKContext(q, handleGetFitnessRecord)).Methods("GET")
	router.HandleFunc("/ext/fitness/domain/{domain}", withSDKContext(q, handleGetFitnessRecordsByDomain)).Methods("GET")
}

// ─── Handler type and context helper ────────────────────────────────────────

type extHandler func(q *QueryExtServer, ctx sdk.Context, w http.ResponseWriter, r *http.Request)

// withSDKContext wraps a handler to extract the SDK context from the request.
// In the Cosmos SDK REST server, the context is typically set via middleware.
// For standalone usage, the handler creates a background context.
func withSDKContext(q *QueryExtServer, handler extHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// The SDK context is injected by the REST server middleware.
		// If not available, we use a nil-safe approach.
		sdkCtx := r.Context().Value(sdk.SdkContextKey)
		if sdkCtx == nil {
			writeError(w, http.StatusInternalServerError, "SDK context not available")
			return
		}
		ctx, ok := sdkCtx.(sdk.Context)
		if !ok {
			writeError(w, http.StatusInternalServerError, "invalid SDK context type")
			return
		}
		handler(q, ctx, w, r)
	}
}

// ─── Model Registry Handlers ────────────────────────────────────────────────

func handleGetModel(q *QueryExtServer, ctx sdk.Context, w http.ResponseWriter, r *http.Request) {
	modelID := mux.Vars(r)["model_id"]
	writeJSON(w, q.GetModelRecord(ctx, modelID))
}

func handleGetModelsByDomain(q *QueryExtServer, ctx sdk.Context, w http.ResponseWriter, r *http.Request) {
	domain := mux.Vars(r)["domain"]
	writeJSON(w, q.GetModelsByDomain(ctx, domain))
}

func handleGetActiveModels(q *QueryExtServer, ctx sdk.Context, w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, q.GetActiveModels(ctx))
}

func handleGetModelLineage(q *QueryExtServer, ctx sdk.Context, w http.ResponseWriter, r *http.Request) {
	modelID := mux.Vars(r)["model_id"]
	writeJSON(w, q.GetModelLineage(ctx, modelID))
}

func handleGetLatestModel(q *QueryExtServer, ctx sdk.Context, w http.ResponseWriter, r *http.Request) {
	domain := mux.Vars(r)["domain"]
	writeJSON(w, q.GetLatestModel(ctx, domain))
}

func handleGetModelEndpoints(q *QueryExtServer, ctx sdk.Context, w http.ResponseWriter, r *http.Request) {
	modelID := mux.Vars(r)["model_id"]
	writeJSON(w, q.GetModelEndpoints(ctx, modelID))
}

// ─── Agent Handlers ─────────────────────────────────────────────────────────

func handleGetAgentIdentity(q *QueryExtServer, ctx sdk.Context, w http.ResponseWriter, r *http.Request) {
	agentID := mux.Vars(r)["agent_id"]
	writeJSON(w, q.GetAgentIdentity(ctx, agentID))
}

func handleGetActiveAgents(q *QueryExtServer, ctx sdk.Context, w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, q.GetActiveAgents(ctx))
}

func handleGetAgentsByDomain(q *QueryExtServer, ctx sdk.Context, w http.ResponseWriter, r *http.Request) {
	domain := mux.Vars(r)["domain"]
	writeJSON(w, q.GetAgentsByDomain(ctx, domain))
}

func handleGetAgentTask(q *QueryExtServer, ctx sdk.Context, w http.ResponseWriter, r *http.Request) {
	taskID := mux.Vars(r)["task_id"]
	writeJSON(w, q.GetAgentTask(ctx, taskID))
}

func handleGetTasksByAgent(q *QueryExtServer, ctx sdk.Context, w http.ResponseWriter, r *http.Request) {
	agentID := mux.Vars(r)["agent_id"]
	writeJSON(w, q.GetTasksByAgent(ctx, agentID))
}

func handleGetTasksByDomain(q *QueryExtServer, ctx sdk.Context, w http.ResponseWriter, r *http.Request) {
	domain := mux.Vars(r)["domain"]
	writeJSON(w, q.GetTasksByDomain(ctx, domain))
}

func handleGetTaskStats(q *QueryExtServer, ctx sdk.Context, w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, q.GetTaskStats(ctx))
}

// ─── Knowledge Graph Handlers ───────────────────────────────────────────────

func handleGetKnowledgeEdge(q *QueryExtServer, ctx sdk.Context, w http.ResponseWriter, r *http.Request) {
	edgeID := mux.Vars(r)["edge_id"]
	writeJSON(w, q.GetKnowledgeEdge(ctx, edgeID))
}

func handleGetEdgesBySource(q *QueryExtServer, ctx sdk.Context, w http.ResponseWriter, r *http.Request) {
	sourceID := mux.Vars(r)["source_id"]
	writeJSON(w, q.GetEdgesBySource(ctx, sourceID))
}

func handleGetEdgesByTarget(q *QueryExtServer, ctx sdk.Context, w http.ResponseWriter, r *http.Request) {
	targetID := mux.Vars(r)["target_id"]
	writeJSON(w, q.GetEdgesByTarget(ctx, targetID))
}

func handleGetGraphStats(q *QueryExtServer, ctx sdk.Context, w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, q.GetGraphStats(ctx))
}

// ─── Bounty Board Handlers ──────────────────────────────────────────────────

func handleGetCompetitiveBounty(q *QueryExtServer, ctx sdk.Context, w http.ResponseWriter, r *http.Request) {
	bountyID := mux.Vars(r)["bounty_id"]
	writeJSON(w, q.GetCompetitiveBounty(ctx, bountyID))
}

func handleGetOpenBounties(q *QueryExtServer, ctx sdk.Context, w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, q.GetOpenBounties(ctx))
}

func handleGetBountySubmissions(q *QueryExtServer, ctx sdk.Context, w http.ResponseWriter, r *http.Request) {
	bountyID := mux.Vars(r)["bounty_id"]
	writeJSON(w, q.GetBountySubmissions(ctx, bountyID))
}

func handleGetBountyLeaderboard(q *QueryExtServer, ctx sdk.Context, w http.ResponseWriter, r *http.Request) {
	bountyID := mux.Vars(r)["bounty_id"]
	writeJSON(w, q.GetBountyLeaderboard(ctx, bountyID))
}

func handleGetBountyBoardStats(q *QueryExtServer, ctx sdk.Context, w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, q.GetBountyBoardStats(ctx))
}

// ─── Memory System Handlers ─────────────────────────────────────────────────

func handleGetActivationRecord(q *QueryExtServer, ctx sdk.Context, w http.ResponseWriter, r *http.Request) {
	sampleID := mux.Vars(r)["sample_id"]
	writeJSON(w, q.GetActivationRecord(ctx, sampleID))
}

func handleGetConsolidationParams(q *QueryExtServer, ctx sdk.Context, w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, q.GetConsolidationParams(ctx))
}

func handleGetTDUsByTier(q *QueryExtServer, ctx sdk.Context, w http.ResponseWriter, r *http.Request) {
	tierStr := mux.Vars(r)["tier"]
	tier, err := parseMemoryTier(tierStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid tier: use working, active, consolidated, or canonical")
		return
	}
	writeJSON(w, q.GetTDUsByTier(ctx, tier))
}

func handleGetReconsolidationStatus(q *QueryExtServer, ctx sdk.Context, w http.ResponseWriter, r *http.Request) {
	sampleID := mux.Vars(r)["sample_id"]
	writeJSON(w, q.GetReconsolidationStatus(ctx, sampleID))
}

// ─── Agent Consumer Handlers ────────────────────────────────────────────────

func handleGetAgentAPIConfig(q *QueryExtServer, ctx sdk.Context, w http.ResponseWriter, r *http.Request) {
	agentID := mux.Vars(r)["agent_id"]
	writeJSON(w, q.GetAgentAPIConfig(ctx, agentID))
}

func handleGetAgentProfitability(q *QueryExtServer, ctx sdk.Context, w http.ResponseWriter, r *http.Request) {
	agentID := mux.Vars(r)["agent_id"]
	writeJSON(w, q.GetAgentProfitability(ctx, agentID))
}

func handleGetAgentEconomicSummary(q *QueryExtServer, ctx sdk.Context, w http.ResponseWriter, r *http.Request) {
	agentID := mux.Vars(r)["agent_id"]
	writeJSON(w, q.GetAgentEconomicSummary(ctx, agentID))
}

// ─── Swarm Handlers ─────────────────────────────────────────────────────────

func handleGetSwarm(q *QueryExtServer, ctx sdk.Context, w http.ResponseWriter, r *http.Request) {
	swarmID := mux.Vars(r)["swarm_id"]
	writeJSON(w, q.GetSwarm(ctx, swarmID))
}

func handleGetSwarmsByDomain(q *QueryExtServer, ctx sdk.Context, w http.ResponseWriter, r *http.Request) {
	domain := mux.Vars(r)["domain"]
	writeJSON(w, q.GetSwarmsByDomain(ctx, domain))
}

func handleGetSwarmsByMember(q *QueryExtServer, ctx sdk.Context, w http.ResponseWriter, r *http.Request) {
	agentID := mux.Vars(r)["agent_id"]
	writeJSON(w, q.GetSwarmsByMember(ctx, agentID))
}

func handleGetSwarmObjectives(q *QueryExtServer, ctx sdk.Context, w http.ResponseWriter, r *http.Request) {
	swarmID := mux.Vars(r)["swarm_id"]
	writeJSON(w, q.GetSwarmObjectives(ctx, swarmID))
}

// ─── Meta-Evolution Handlers ────────────────────────────────────────────────

func handleGetCurrentEpoch(q *QueryExtServer, ctx sdk.Context, w http.ResponseWriter, r *http.Request) {
	domain := mux.Vars(r)["domain"]
	writeJSON(w, q.GetCurrentEpoch(ctx, domain))
}

func handleGetEpochHistory(q *QueryExtServer, ctx sdk.Context, w http.ResponseWriter, r *http.Request) {
	domain := mux.Vars(r)["domain"]
	writeJSON(w, q.GetEpochHistory(ctx, domain))
}

func handleGetEvolutionEpoch(q *QueryExtServer, ctx sdk.Context, w http.ResponseWriter, r *http.Request) {
	epochID := mux.Vars(r)["epoch_id"]
	writeJSON(w, q.GetEvolutionEpoch(ctx, epochID))
}

func handleGetActiveStrategies(q *QueryExtServer, ctx sdk.Context, w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, q.GetActiveStrategies(ctx))
}

func handleGetMetaParameter(q *QueryExtServer, ctx sdk.Context, w http.ResponseWriter, r *http.Request) {
	paramID := mux.Vars(r)["param_id"]
	writeJSON(w, q.GetMetaParameter(ctx, paramID))
}

// ─── Curation Strategy Handlers ─────────────────────────────────────────────

func handleGetKnowledgeGaps(q *QueryExtServer, ctx sdk.Context, w http.ResponseWriter, r *http.Request) {
	domain := mux.Vars(r)["domain"]
	writeJSON(w, q.GetKnowledgeGaps(ctx, domain))
}

func handleGetDomainHealth(q *QueryExtServer, ctx sdk.Context, w http.ResponseWriter, r *http.Request) {
	domain := mux.Vars(r)["domain"]
	writeJSON(w, q.GetDomainHealth(ctx, domain))
}

func handleGetStrategiesByAgent(q *QueryExtServer, ctx sdk.Context, w http.ResponseWriter, r *http.Request) {
	agentID := mux.Vars(r)["agent_id"]
	writeJSON(w, q.GetStrategiesByAgent(ctx, agentID))
}

// ─── Fitness Handlers ───────────────────────────────────────────────────────

func handleGetFitnessRecord(q *QueryExtServer, ctx sdk.Context, w http.ResponseWriter, r *http.Request) {
	sampleID := mux.Vars(r)["sample_id"]
	writeJSON(w, q.GetFitnessRecord(ctx, sampleID))
}

func handleGetFitnessRecordsByDomain(q *QueryExtServer, ctx sdk.Context, w http.ResponseWriter, r *http.Request) {
	domain := mux.Vars(r)["domain"]
	writeJSON(w, q.GetFitnessRecordsByDomain(ctx, domain))
}

// ─── Helpers ────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if v == nil {
		w.Write([]byte("null")) //nolint:errcheck
		return
	}
	bz, err := json.Marshal(v)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Write(bz) //nolint:errcheck
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	errResp := map[string]string{"error": msg}
	bz, _ := json.Marshal(errResp)
	w.Write(bz) //nolint:errcheck
}

// parseMemoryTier converts a string to types.MemoryTier.
func parseMemoryTier(s string) (types.MemoryTier, error) {
	switch s {
	case "working", "0":
		return types.MemoryTierWorking, nil
	case "active", "1":
		return types.MemoryTierActive, nil
	case "consolidated", "2":
		return types.MemoryTierConsolidated, nil
	case "canonical", "3":
		return types.MemoryTierCanonical, nil
	default:
		// Try numeric.
		n, err := strconv.Atoi(s)
		if err != nil {
			return 0, err
		}
		return types.MemoryTier(n), nil
	}
}
