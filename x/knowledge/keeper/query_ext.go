package keeper

import (
	"context"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── QueryExtServer ─────────────────────────────────────────────────────────
//
// Provides read-only query access to the sovereignty stack modules (R48–R57)
// that lack gRPC proto RPCs. Returns Go structs directly — the REST route
// layer (query_ext_routes.go) handles JSON marshalling and HTTP.
//
// Agent daemons consume these endpoints to query chain state across all 10
// module domains: model registry, agents, knowledge graph, bounty board,
// memory system, agent consumer, swarms, meta-evolution, curation strategy,
// and fitness.

// QueryExtServer wraps the Keeper for read-only queries on sovereignty stack modules.
type QueryExtServer struct {
	k Keeper
}

// NewQueryExtServer creates a new query extension server.
func NewQueryExtServer(k Keeper) *QueryExtServer {
	return &QueryExtServer{k: k}
}

// ─── 1. Model Registry (R48) ────────────────────────────────────────────────

// ModelRecordResponse is the JSON response for a single model record.
type ModelRecordResponse struct {
	Found bool              `json:"found"`
	Model types.ModelRecord `json:"model,omitempty"`
}

// GetModelRecord returns a model by ID.
func (q *QueryExtServer) GetModelRecord(ctx context.Context, modelID string) ModelRecordResponse {
	record, found := q.k.GetModelRecord(ctx, modelID)
	return ModelRecordResponse{Found: found, Model: record}
}

// GetModelsByDomain returns all models registered under a domain.
func (q *QueryExtServer) GetModelsByDomain(ctx context.Context, domain string) []types.ModelRecord {
	return q.k.GetModelsByDomain(ctx, domain)
}

// GetActiveModels returns all models with active status.
func (q *QueryExtServer) GetActiveModels(ctx context.Context) []types.ModelRecord {
	return q.k.GetActiveModels(ctx)
}

// ModelLineageResponse wraps a lineage lookup.
type ModelLineageResponse struct {
	Found   bool               `json:"found"`
	Lineage types.ModelLineage `json:"lineage,omitempty"`
}

// GetModelLineage returns the lineage chain for a model.
func (q *QueryExtServer) GetModelLineage(ctx context.Context, modelID string) ModelLineageResponse {
	lineage, found := q.k.GetModelLineage(ctx, modelID)
	return ModelLineageResponse{Found: found, Lineage: lineage}
}

// GetLatestModel returns the latest active model for a domain.
func (q *QueryExtServer) GetLatestModel(ctx context.Context, domain string) ModelRecordResponse {
	record, found := q.k.GetLatestModel(ctx, domain)
	return ModelRecordResponse{Found: found, Model: record}
}

// GetModelEndpoints returns all registered inference endpoints for a model.
func (q *QueryExtServer) GetModelEndpoints(ctx context.Context, modelID string) []string {
	return q.k.GetModelEndpoints(ctx, modelID)
}

// ─── 2. Agent (R49) ─────────────────────────────────────────────────────────

// AgentIdentityResponse wraps an agent identity lookup.
type AgentIdentityResponse struct {
	Found bool                 `json:"found"`
	Agent types.AgentIdentity  `json:"agent,omitempty"`
}

// GetAgentIdentity returns an agent by ID.
func (q *QueryExtServer) GetAgentIdentity(ctx context.Context, agentID string) AgentIdentityResponse {
	agent, found := q.k.GetAgentIdentity(ctx, agentID)
	return AgentIdentityResponse{Found: found, Agent: agent}
}

// GetActiveAgents returns all active agents.
func (q *QueryExtServer) GetActiveAgents(ctx context.Context) []types.AgentIdentity {
	return q.k.GetActiveAgents(ctx)
}

// GetAgentsByDomain returns all agents in a domain.
func (q *QueryExtServer) GetAgentsByDomain(ctx context.Context, domain string) []types.AgentIdentity {
	return q.k.GetAgentsByDomain(ctx, domain)
}

// AgentTaskResponse wraps a task lookup.
type AgentTaskResponse struct {
	Found bool             `json:"found"`
	Task  *types.AgentTask `json:"task,omitempty"`
}

// GetAgentTask returns a task by ID.
func (q *QueryExtServer) GetAgentTask(ctx context.Context, taskID string) AgentTaskResponse {
	task, found := q.k.GetAgentTask(ctx, taskID)
	return AgentTaskResponse{Found: found, Task: task}
}

// GetTasksByAgent returns all tasks assigned to an agent.
func (q *QueryExtServer) GetTasksByAgent(ctx context.Context, agentID string) []*types.AgentTask {
	return q.k.GetTasksByAgent(ctx, agentID)
}

// GetTasksByDomain returns all tasks in a domain.
func (q *QueryExtServer) GetTasksByDomain(ctx context.Context, domain string) []*types.AgentTask {
	return q.k.GetTasksByDomain(ctx, domain)
}

// TaskStatsResponse contains aggregate task statistics.
type TaskStatsResponse struct {
	Pending   uint64 `json:"pending"`
	Assigned  uint64 `json:"assigned"`
	Completed uint64 `json:"completed"`
	Failed    uint64 `json:"failed"`
	Expired   uint64 `json:"expired"`
}

// GetTaskStats returns aggregate task statistics.
func (q *QueryExtServer) GetTaskStats(ctx context.Context) TaskStatsResponse {
	pending, assigned, completed, failed, expired := q.k.GetTaskStats(ctx)
	return TaskStatsResponse{
		Pending:   pending,
		Assigned:  assigned,
		Completed: completed,
		Failed:    failed,
		Expired:   expired,
	}
}

// ─── 3. Knowledge Graph (R48) ───────────────────────────────────────────────

// KnowledgeEdgeResponse wraps an edge lookup.
type KnowledgeEdgeResponse struct {
	Found bool                `json:"found"`
	Edge  types.KnowledgeEdge `json:"edge,omitempty"`
}

// GetKnowledgeEdge returns an edge by ID.
func (q *QueryExtServer) GetKnowledgeEdge(ctx context.Context, edgeID string) KnowledgeEdgeResponse {
	edge, found := q.k.GetKnowledgeEdge(ctx, edgeID)
	return KnowledgeEdgeResponse{Found: found, Edge: edge}
}

// GetEdgesBySource returns all edges originating from a TDU (outgoing).
func (q *QueryExtServer) GetEdgesBySource(ctx context.Context, sourceID string) []types.KnowledgeEdge {
	return q.k.GetOutgoingEdges(ctx, sourceID)
}

// GetEdgesByTarget returns all edges pointing to a TDU (incoming).
func (q *QueryExtServer) GetEdgesByTarget(ctx context.Context, targetID string) []types.KnowledgeEdge {
	return q.k.GetIncomingEdges(ctx, targetID)
}

// GraphStatsResponse contains knowledge graph statistics.
type GraphStatsResponse struct {
	TotalEdges    uint64 `json:"total_edges"`
	TotalClusters uint64 `json:"total_clusters"`
}

// GetGraphStats returns aggregate knowledge graph statistics.
func (q *QueryExtServer) GetGraphStats(ctx context.Context) GraphStatsResponse {
	edges, clusters := q.k.GetGraphStats(ctx)
	return GraphStatsResponse{TotalEdges: edges, TotalClusters: clusters}
}

// ─── 4. Bounty Board (R49) ──────────────────────────────────────────────────

// CompetitiveBountyResponse wraps a bounty lookup.
type CompetitiveBountyResponse struct {
	Found  bool               `json:"found"`
	Bounty *CompetitiveBounty `json:"bounty,omitempty"`
}

// GetCompetitiveBounty returns a competitive bounty by ID.
func (q *QueryExtServer) GetCompetitiveBounty(ctx context.Context, bountyID string) CompetitiveBountyResponse {
	bounty, found := q.k.GetCompetitiveBounty(ctx, bountyID)
	return CompetitiveBountyResponse{Found: found, Bounty: bounty}
}

// GetOpenBounties returns all bounties currently accepting submissions.
func (q *QueryExtServer) GetOpenBounties(ctx context.Context) []*CompetitiveBounty {
	return q.k.GetOpenBounties(ctx)
}

// GetBountySubmissions returns all submissions for a bounty.
func (q *QueryExtServer) GetBountySubmissions(ctx context.Context, bountyID string) []*BountySubmission {
	return q.k.GetBountySubmissions(ctx, bountyID)
}

// GetBountyLeaderboard returns submissions ranked by fitness (live ranking).
func (q *QueryExtServer) GetBountyLeaderboard(ctx context.Context, bountyID string) []BountySubmission {
	return q.k.GetBountyLeaderboard(ctx, bountyID)
}

// BountyBoardStatsResponse contains bounty board statistics.
type BountyBoardStatsResponse struct {
	Open      uint64 `json:"open"`
	Competing uint64 `json:"competing"`
	Judging   uint64 `json:"judging"`
	Resolved  uint64 `json:"resolved"`
	Expired   uint64 `json:"expired"`
}

// GetBountyBoardStats returns aggregate bounty board statistics.
func (q *QueryExtServer) GetBountyBoardStats(ctx context.Context) BountyBoardStatsResponse {
	open, competing, judging, resolved, expired := q.k.GetBountyBoardStats(ctx)
	return BountyBoardStatsResponse{
		Open:      open,
		Competing: competing,
		Judging:   judging,
		Resolved:  resolved,
		Expired:   expired,
	}
}

// ─── 5. Memory System (R50/R51) ─────────────────────────────────────────────

// ActivationRecordResponse wraps an activation record lookup.
type ActivationRecordResponse struct {
	Found  bool                     `json:"found"`
	Record *types.ActivationRecord  `json:"record,omitempty"`
}

// GetActivationRecord returns the activation record for a TDU.
func (q *QueryExtServer) GetActivationRecord(ctx context.Context, sampleID string) ActivationRecordResponse {
	record, found := q.k.GetActivationRecord(ctx, sampleID)
	return ActivationRecordResponse{Found: found, Record: record}
}

// GetConsolidationParams returns the memory consolidation parameters.
func (q *QueryExtServer) GetConsolidationParams(ctx context.Context) types.ConsolidationParams {
	return q.k.GetConsolidationParams(ctx)
}

// GetTDUsByTier returns all TDU sample IDs at a given memory tier.
func (q *QueryExtServer) GetTDUsByTier(ctx context.Context, tier types.MemoryTier) []string {
	return q.k.GetTDUsByTier(ctx, tier)
}

// ReconsolidationStatusResponse wraps a reconsolidation check.
type ReconsolidationStatusResponse struct {
	InReconsolidation bool                            `json:"in_reconsolidation"`
	Window            *types.ReconsolidationWindow    `json:"window,omitempty"`
	History           *types.ReconsolidationHistory   `json:"history,omitempty"`
}

// GetReconsolidationStatus returns whether a TDU is in reconsolidation and its history.
func (q *QueryExtServer) GetReconsolidationStatus(ctx context.Context, sampleID string) ReconsolidationStatusResponse {
	inRecon, window := q.k.IsInReconsolidation(ctx, sampleID)
	history := q.k.GetReconsolidationHistory(ctx, sampleID)
	return ReconsolidationStatusResponse{
		InReconsolidation: inRecon,
		Window:            window,
		History:           history,
	}
}

// ─── 6. Agent Consumer (R51) ────────────────────────────────────────────────

// AgentAPIConfigResponse wraps an agent API config lookup.
type AgentAPIConfigResponse struct {
	Found  bool                  `json:"found"`
	Config types.AgentAPIConfig  `json:"config,omitempty"`
}

// GetAgentAPIConfig returns an agent's API configuration.
func (q *QueryExtServer) GetAgentAPIConfig(ctx context.Context, agentID string) AgentAPIConfigResponse {
	config, found := q.k.GetAgentAPIConfig(ctx, agentID)
	return AgentAPIConfigResponse{Found: found, Config: config}
}

// AgentProfitabilityResponse wraps a profitability lookup.
type AgentProfitabilityResponse struct {
	Found         bool                       `json:"found"`
	Profitability types.AgentProfitability    `json:"profitability,omitempty"`
}

// GetAgentProfitability returns an agent's P&L record.
func (q *QueryExtServer) GetAgentProfitability(ctx context.Context, agentID string) AgentProfitabilityResponse {
	p, found := q.k.GetAgentProfitability(ctx, agentID)
	return AgentProfitabilityResponse{Found: found, Profitability: p}
}

// GetAgentEconomicSummary returns a complete economic snapshot.
func (q *QueryExtServer) GetAgentEconomicSummary(ctx context.Context, agentID string) map[string]string {
	return q.k.GetAgentEconomicSummary(ctx, agentID)
}

// ─── 7. Swarms (R55) ────────────────────────────────────────────────────────

// SwarmResponse wraps a swarm lookup.
type SwarmResponse struct {
	Found bool              `json:"found"`
	Swarm *types.AgentSwarm `json:"swarm,omitempty"`
}

// GetSwarm returns a swarm by ID.
func (q *QueryExtServer) GetSwarm(ctx context.Context, swarmID string) SwarmResponse {
	swarm, found := q.k.GetAgentSwarm(ctx, swarmID)
	return SwarmResponse{Found: found, Swarm: swarm}
}

// GetSwarmsByDomain returns all swarms in a domain.
func (q *QueryExtServer) GetSwarmsByDomain(ctx context.Context, domain string) []*types.AgentSwarm {
	return q.k.GetSwarmsByDomain(ctx, domain)
}

// GetSwarmsByMember returns all swarms an agent belongs to.
func (q *QueryExtServer) GetSwarmsByMember(ctx context.Context, agentID string) []*types.AgentSwarm {
	return q.k.GetSwarmsByAgent(ctx, agentID)
}

// GetSwarmObjectives returns all objectives for a swarm.
func (q *QueryExtServer) GetSwarmObjectives(ctx context.Context, swarmID string) []*types.SwarmObjective {
	return q.k.GetSwarmObjectives(ctx, swarmID)
}

// ─── 8. Meta-Evolution (R57) ────────────────────────────────────────────────

// EvolutionEpochResponse wraps an epoch lookup.
type EvolutionEpochResponse struct {
	Found bool                   `json:"found"`
	Epoch *types.EvolutionEpoch  `json:"epoch,omitempty"`
}

// GetEvolutionEpoch returns an epoch by ID.
func (q *QueryExtServer) GetEvolutionEpoch(ctx context.Context, epochID string) EvolutionEpochResponse {
	epoch, found := q.k.GetEvolutionEpoch(ctx, epochID)
	return EvolutionEpochResponse{Found: found, Epoch: epoch}
}

// CurrentEpochResponse wraps a current epoch lookup.
type CurrentEpochResponse struct {
	HasActive bool                  `json:"has_active"`
	EpochID   string                `json:"epoch_id,omitempty"`
	Epoch     *types.EvolutionEpoch `json:"epoch,omitempty"`
}

// GetCurrentEpoch returns the current active epoch for a domain.
func (q *QueryExtServer) GetCurrentEpoch(ctx context.Context, domain string) CurrentEpochResponse {
	epochID, hasActive := q.k.GetCurrentEpochID(ctx, domain)
	if !hasActive {
		return CurrentEpochResponse{HasActive: false}
	}
	epoch, found := q.k.GetEvolutionEpoch(ctx, epochID)
	if !found {
		return CurrentEpochResponse{HasActive: false}
	}
	return CurrentEpochResponse{HasActive: true, EpochID: epochID, Epoch: epoch}
}

// GetEpochHistory returns all epochs for a domain.
func (q *QueryExtServer) GetEpochHistory(ctx context.Context, domain string) []*types.EvolutionEpoch {
	return q.k.GetEpochsByDomain(ctx, domain)
}

// GetActiveStrategies returns all curation strategies across the system.
func (q *QueryExtServer) GetActiveStrategies(ctx context.Context) []*types.CurationStrategy {
	var strategies []*types.CurationStrategy
	q.k.IterateCurationStrategies(ctx, func(strategy *types.CurationStrategy) bool {
		strategies = append(strategies, strategy)
		return false
	})
	return strategies
}

// MetaParameterResponse wraps a meta-parameter lookup.
type MetaParameterResponse struct {
	Found bool                 `json:"found"`
	Param *types.MetaParameter `json:"param,omitempty"`
}

// GetMetaParameter returns a meta-parameter by ID.
func (q *QueryExtServer) GetMetaParameter(ctx context.Context, paramID string) MetaParameterResponse {
	param, found := q.k.GetMetaParameter(ctx, paramID)
	return MetaParameterResponse{Found: found, Param: param}
}

// ─── 9. Curation Strategy (R54) ─────────────────────────────────────────────

// GetKnowledgeGaps returns all open gaps for a domain.
func (q *QueryExtServer) GetKnowledgeGaps(ctx context.Context, domain string) []*types.KnowledgeGap {
	return q.k.GetOpenGapsByDomain(ctx, domain)
}

// DomainHealthResponse wraps a domain health lookup.
type DomainHealthResponse struct {
	Found  bool                `json:"found"`
	Health *types.DomainHealth `json:"health,omitempty"`
}

// GetDomainHealth returns the latest health snapshot for a domain.
func (q *QueryExtServer) GetDomainHealth(ctx context.Context, domain string) DomainHealthResponse {
	health, found := q.k.GetDomainHealth(ctx, domain)
	return DomainHealthResponse{Found: found, Health: health}
}

// GetStrategiesByAgent returns all curation strategies for an agent.
func (q *QueryExtServer) GetStrategiesByAgent(ctx context.Context, agentID string) []*types.CurationStrategy {
	return q.k.GetStrategiesByAgent(ctx, agentID)
}

// ─── 10. Fitness ─────────────────────────────────────────────────────────────

// FitnessRecordResponse wraps a fitness record lookup.
type FitnessRecordResponse struct {
	Found  bool                     `json:"found"`
	Record types.TDUFitnessRecord   `json:"record,omitempty"`
}

// GetFitnessRecord returns the fitness record for a TDU.
func (q *QueryExtServer) GetFitnessRecord(ctx context.Context, sampleID string) FitnessRecordResponse {
	record, found := q.k.GetFitnessRecord(ctx, sampleID)
	return FitnessRecordResponse{Found: found, Record: record}
}

// GetFitnessRecordsByDomain returns fitness records for all active TDUs in a domain.
// Joins samples-by-domain index with fitness records.
func (q *QueryExtServer) GetFitnessRecordsByDomain(ctx context.Context, domain string) []types.TDUFitnessRecord {
	sampleIDs := q.k.GetSamplesByDomain(ctx, domain)
	var records []types.TDUFitnessRecord
	for _, id := range sampleIDs {
		record, found := q.k.GetFitnessRecord(ctx, id)
		if found {
			records = append(records, record)
		}
	}
	return records
}
