package keeper

import (
	"context"
	"fmt"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Sovereignty Genesis State ──────────────────────────────────────────────
//
// Captures the full sovereignty stack (R45-R57) for genesis export/import.
// All types are JSON-serialized in the KVStore — no proto dependency.

// SovereigntyGenesisState holds all R45-R57 sovereignty module state.
type SovereigntyGenesisState struct {
	// R45-1: Model Registry
	ModelRecords  []types.ModelRecord  `json:"model_records"`
	ModelLineages []types.ModelLineage `json:"model_lineages"`

	// R45-2: Agent Promotion
	AgentIdentities []types.AgentIdentity `json:"agent_identities"`

	// R46: Knowledge Graph
	KnowledgeEdges    []types.KnowledgeEdge    `json:"knowledge_edges"`
	KnowledgeClusters []types.KnowledgeCluster `json:"knowledge_clusters"`

	// R47: Bounty Board
	CompetitiveBounties []CompetitiveBounty `json:"competitive_bounties"`
	BountySubmissions   []BountySubmission  `json:"bounty_submissions"`
	BountyBoardParams   *BountyBoardParams  `json:"bounty_board_params,omitempty"`

	// R48: Agent Execution
	AgentTasks       []types.AgentTask       `json:"agent_tasks"`
	AgentTaskResults []types.AgentTaskResult  `json:"agent_task_results"`
	SchedulerParams  *SchedulerParams        `json:"scheduler_params,omitempty"`

	// R49: Curriculum Training
	Curricula             []types.Curriculum           `json:"curricula"`
	CurriculumEnrollments []types.CurriculumEnrollment `json:"curriculum_enrollments"`

	// R50: Memory Consolidation
	ActivationRecords  []types.ActivationRecord   `json:"activation_records"`
	ConsolidationParams *types.ConsolidationParams `json:"consolidation_params,omitempty"`

	// R51: Reconsolidation
	ReconsolidationWindows   []types.ReconsolidationWindow  `json:"reconsolidation_windows"`
	ReconsolidationHistories []types.ReconsolidationHistory `json:"reconsolidation_histories"`
	ReconsolidationParams    *types.ReconsolidationParams   `json:"reconsolidation_params,omitempty"`

	// R51: Agent Consumer
	AgentAPIConfigs      []types.AgentAPIConfig      `json:"agent_api_configs"`
	AgentProfitabilities []types.AgentProfitability   `json:"agent_profitabilities"`
	AgentConsumerParams  *types.AgentConsumerParams   `json:"agent_consumer_params,omitempty"`

	// R54: Strategic Curation
	KnowledgeGaps        []types.KnowledgeGap          `json:"knowledge_gaps"`
	CurationStrategies   []types.CurationStrategy       `json:"curation_strategies"`
	DomainHealths        []types.DomainHealth           `json:"domain_healths"`
	CurationStrategyParams *types.CurationStrategyParams `json:"curation_strategy_params,omitempty"`

	// R55: Agent Swarms
	AgentSwarms     []types.AgentSwarm     `json:"agent_swarms"`
	SwarmObjectives []types.SwarmObjective  `json:"swarm_objectives"`
	SwarmParams     *types.SwarmParams      `json:"swarm_params,omitempty"`

	// R56: Model Composition
	ModelEnsembles   []types.ModelEnsemble   `json:"model_ensembles"`
	DistillationJobs []types.DistillationJob `json:"distillation_jobs"`
	CompositionParams *types.CompositionParams `json:"composition_params,omitempty"`

	// R57: Meta-Evolution
	EvolutionEpochs     []types.EvolutionEpoch      `json:"evolution_epochs"`
	MetaParameters      []types.MetaParameter       `json:"meta_parameters"`
	MetaEvolutionParams *types.MetaEvolutionParams   `json:"meta_evolution_params,omitempty"`

	// Fitness
	FitnessRecords    []types.TDUFitnessRecord  `json:"fitness_records"`
	FitnessDecayParams *types.FitnessDecayParams `json:"fitness_decay_params,omitempty"`
}

// DefaultSovereigntyGenesis returns an empty sovereignty genesis with default params.
func DefaultSovereigntyGenesis() SovereigntyGenesisState {
	bbp := DefaultBountyBoardParams()
	sp := DefaultSchedulerParams()
	cp := types.DefaultConsolidationParams()
	rp := types.DefaultReconsolidationParams()
	acp := types.DefaultAgentConsumerParams()
	csp := types.DefaultCurationStrategyParams()
	swp := types.DefaultSwarmParams()
	comp := types.DefaultCompositionParams()
	mep := types.DefaultMetaEvolutionParams()
	fdp := types.DefaultFitnessDecayParams()

	return SovereigntyGenesisState{
		BountyBoardParams:      &bbp,
		SchedulerParams:        &sp,
		ConsolidationParams:    &cp,
		ReconsolidationParams:  &rp,
		AgentConsumerParams:    &acp,
		CurationStrategyParams: &csp,
		SwarmParams:            &swp,
		CompositionParams:      &comp,
		MetaEvolutionParams:    &mep,
		FitnessDecayParams:     &fdp,
	}
}

// ─── Validation ─────────────────────────────────────────────────────────────

// ValidateSovereigntyGenesis performs cross-reference checks on sovereignty genesis.
func ValidateSovereigntyGenesis(gs SovereigntyGenesisState) error {
	// Build model ID set.
	modelIDs := make(map[string]bool, len(gs.ModelRecords))
	for _, m := range gs.ModelRecords {
		if m.ModelID == "" {
			return fmt.Errorf("model record has empty ID")
		}
		if modelIDs[m.ModelID] {
			return fmt.Errorf("duplicate model ID: %s", m.ModelID)
		}
		modelIDs[m.ModelID] = true
	}

	// Agent model IDs must reference existing models.
	agentIDs := make(map[string]bool, len(gs.AgentIdentities))
	for _, a := range gs.AgentIdentities {
		if a.AgentID == "" {
			return fmt.Errorf("agent identity has empty ID")
		}
		if agentIDs[a.AgentID] {
			return fmt.Errorf("duplicate agent ID: %s", a.AgentID)
		}
		agentIDs[a.AgentID] = true
		if a.ModelID != "" && !modelIDs[a.ModelID] {
			return fmt.Errorf("agent %s references unknown model %s", a.AgentID, a.ModelID)
		}
	}

	// Model lineage references must exist.
	for _, lin := range gs.ModelLineages {
		if lin.ModelID == "" {
			return fmt.Errorf("model lineage has empty model ID")
		}
		if !modelIDs[lin.ModelID] {
			return fmt.Errorf("lineage references unknown model %s", lin.ModelID)
		}
	}

	// Edge source/target must not be empty.
	edgeIDs := make(map[string]bool, len(gs.KnowledgeEdges))
	for _, e := range gs.KnowledgeEdges {
		if e.EdgeID == "" {
			return fmt.Errorf("knowledge edge has empty ID")
		}
		if edgeIDs[e.EdgeID] {
			return fmt.Errorf("duplicate edge ID: %s", e.EdgeID)
		}
		edgeIDs[e.EdgeID] = true
		if e.SourceID == "" || e.TargetID == "" {
			return fmt.Errorf("edge %s has empty source or target", e.EdgeID)
		}
		if e.SourceID == e.TargetID {
			return fmt.Errorf("edge %s is a self-reference", e.EdgeID)
		}
	}

	// Cluster IDs unique.
	clusterIDs := make(map[string]bool, len(gs.KnowledgeClusters))
	for _, c := range gs.KnowledgeClusters {
		if c.ClusterID == "" {
			return fmt.Errorf("knowledge cluster has empty ID")
		}
		if clusterIDs[c.ClusterID] {
			return fmt.Errorf("duplicate cluster ID: %s", c.ClusterID)
		}
		clusterIDs[c.ClusterID] = true
	}

	// Bounty IDs unique.
	bountyIDs := make(map[string]bool, len(gs.CompetitiveBounties))
	for _, b := range gs.CompetitiveBounties {
		if b.BountyID == "" {
			return fmt.Errorf("competitive bounty has empty ID")
		}
		if bountyIDs[b.BountyID] {
			return fmt.Errorf("duplicate bounty ID: %s", b.BountyID)
		}
		bountyIDs[b.BountyID] = true
	}

	// Bounty submissions reference valid bounties.
	for _, s := range gs.BountySubmissions {
		if s.SubmissionID == "" {
			return fmt.Errorf("bounty submission has empty ID")
		}
		if s.BountyID != "" && !bountyIDs[s.BountyID] {
			return fmt.Errorf("submission %s references unknown bounty %s", s.SubmissionID, s.BountyID)
		}
	}

	// Task IDs unique.
	taskIDs := make(map[string]bool, len(gs.AgentTasks))
	for _, t := range gs.AgentTasks {
		if t.TaskID == "" {
			return fmt.Errorf("agent task has empty ID")
		}
		if taskIDs[t.TaskID] {
			return fmt.Errorf("duplicate task ID: %s", t.TaskID)
		}
		taskIDs[t.TaskID] = true
	}

	// Task results reference valid tasks.
	for _, r := range gs.AgentTaskResults {
		if r.TaskID == "" {
			return fmt.Errorf("agent task result has empty task ID")
		}
		if !taskIDs[r.TaskID] {
			return fmt.Errorf("task result references unknown task %s", r.TaskID)
		}
	}

	// Curriculum IDs unique.
	curriculumIDs := make(map[string]bool, len(gs.Curricula))
	for _, c := range gs.Curricula {
		if c.CurriculumID == "" {
			return fmt.Errorf("curriculum has empty ID")
		}
		if curriculumIDs[c.CurriculumID] {
			return fmt.Errorf("duplicate curriculum ID: %s", c.CurriculumID)
		}
		curriculumIDs[c.CurriculumID] = true
	}

	// Enrollment references valid curriculum.
	for _, e := range gs.CurriculumEnrollments {
		if e.EnrollmentID == "" {
			return fmt.Errorf("enrollment has empty ID")
		}
		if e.CurriculumID != "" && !curriculumIDs[e.CurriculumID] {
			return fmt.Errorf("enrollment %s references unknown curriculum %s", e.EnrollmentID, e.CurriculumID)
		}
	}

	// Swarm IDs unique.
	swarmIDs := make(map[string]bool, len(gs.AgentSwarms))
	for _, s := range gs.AgentSwarms {
		if s.SwarmID == "" {
			return fmt.Errorf("agent swarm has empty ID")
		}
		if swarmIDs[s.SwarmID] {
			return fmt.Errorf("duplicate swarm ID: %s", s.SwarmID)
		}
		swarmIDs[s.SwarmID] = true
	}

	// Swarm objectives reference valid swarms.
	for _, o := range gs.SwarmObjectives {
		if o.ObjectiveID == "" {
			return fmt.Errorf("swarm objective has empty ID")
		}
		if o.SwarmID != "" && !swarmIDs[o.SwarmID] {
			return fmt.Errorf("objective %s references unknown swarm %s", o.ObjectiveID, o.SwarmID)
		}
	}

	// Ensemble IDs unique.
	ensembleIDs := make(map[string]bool, len(gs.ModelEnsembles))
	for _, e := range gs.ModelEnsembles {
		if e.EnsembleID == "" {
			return fmt.Errorf("model ensemble has empty ID")
		}
		if ensembleIDs[e.EnsembleID] {
			return fmt.Errorf("duplicate ensemble ID: %s", e.EnsembleID)
		}
		ensembleIDs[e.EnsembleID] = true
	}

	// Distillation jobs reference valid ensembles.
	for _, d := range gs.DistillationJobs {
		if d.JobID == "" {
			return fmt.Errorf("distillation job has empty ID")
		}
		if d.EnsembleID != "" && !ensembleIDs[d.EnsembleID] {
			return fmt.Errorf("distillation job %s references unknown ensemble %s", d.JobID, d.EnsembleID)
		}
	}

	// Validate params if present.
	if gs.CurationStrategyParams != nil {
		if err := gs.CurationStrategyParams.Validate(); err != nil {
			return fmt.Errorf("invalid curation strategy params: %w", err)
		}
	}
	if gs.SwarmParams != nil {
		if err := gs.SwarmParams.Validate(); err != nil {
			return fmt.Errorf("invalid swarm params: %w", err)
		}
	}
	if gs.CompositionParams != nil {
		if err := gs.CompositionParams.Validate(); err != nil {
			return fmt.Errorf("invalid composition params: %w", err)
		}
	}
	if gs.MetaEvolutionParams != nil {
		if err := gs.MetaEvolutionParams.Validate(); err != nil {
			return fmt.Errorf("invalid meta-evolution params: %w", err)
		}
	}
	if gs.FitnessDecayParams != nil {
		if err := gs.FitnessDecayParams.Validate(); err != nil {
			return fmt.Errorf("invalid fitness decay params: %w", err)
		}
	}
	if gs.AgentConsumerParams != nil {
		if err := gs.AgentConsumerParams.Validate(); err != nil {
			return fmt.Errorf("invalid agent consumer params: %w", err)
		}
	}

	return nil
}

// ─── Export ─────────────────────────────────────────────────────────────────

// ExportSovereigntyGenesis iterates all sovereignty stores and collects into a struct.
func (k Keeper) ExportSovereigntyGenesis(ctx context.Context) SovereigntyGenesisState {
	gs := SovereigntyGenesisState{}

	// R45-1
	k.IterateModelRecords(ctx, func(r types.ModelRecord) bool {
		gs.ModelRecords = append(gs.ModelRecords, r)
		return false
	})
	k.IterateModelLineages(ctx, func(l types.ModelLineage) bool {
		gs.ModelLineages = append(gs.ModelLineages, l)
		return false
	})

	// R45-2
	k.IterateAgentIdentities(ctx, func(a types.AgentIdentity) bool {
		gs.AgentIdentities = append(gs.AgentIdentities, a)
		return false
	})

	// R46
	k.IterateKnowledgeEdges(ctx, func(e types.KnowledgeEdge) bool {
		gs.KnowledgeEdges = append(gs.KnowledgeEdges, e)
		return false
	})
	k.IterateKnowledgeClusters(ctx, func(c types.KnowledgeCluster) bool {
		gs.KnowledgeClusters = append(gs.KnowledgeClusters, c)
		return false
	})

	// R47
	k.IterateCompetitiveBounties(ctx, func(b *CompetitiveBounty) bool {
		gs.CompetitiveBounties = append(gs.CompetitiveBounties, *b)
		return false
	})
	k.IterateBountySubmissions(ctx, func(s BountySubmission) bool {
		gs.BountySubmissions = append(gs.BountySubmissions, s)
		return false
	})
	bbp := k.GetBountyBoardParams(ctx)
	gs.BountyBoardParams = &bbp

	// R48
	k.IterateAgentTasks(ctx, func(t types.AgentTask) bool {
		gs.AgentTasks = append(gs.AgentTasks, t)
		return false
	})
	k.IterateAgentTaskResults(ctx, func(r types.AgentTaskResult) bool {
		gs.AgentTaskResults = append(gs.AgentTaskResults, r)
		return false
	})
	sp := k.GetSchedulerParams(ctx)
	gs.SchedulerParams = &sp

	// R49
	k.IterateCurricula(ctx, func(c types.Curriculum) bool {
		gs.Curricula = append(gs.Curricula, c)
		return false
	})
	k.IterateCurriculumEnrollments(ctx, func(e types.CurriculumEnrollment) bool {
		gs.CurriculumEnrollments = append(gs.CurriculumEnrollments, e)
		return false
	})

	// R50
	k.IterateActivationRecords(ctx, func(r types.ActivationRecord) bool {
		gs.ActivationRecords = append(gs.ActivationRecords, r)
		return false
	})
	cp := k.GetConsolidationParams(ctx)
	gs.ConsolidationParams = &cp

	// R51 reconsolidation
	k.IterateReconsolidationWindows(ctx, func(w types.ReconsolidationWindow) bool {
		gs.ReconsolidationWindows = append(gs.ReconsolidationWindows, w)
		return false
	})
	k.IterateReconsolidationHistories(ctx, func(h types.ReconsolidationHistory) bool {
		gs.ReconsolidationHistories = append(gs.ReconsolidationHistories, h)
		return false
	})
	rp := k.GetReconsolidationParams(ctx)
	gs.ReconsolidationParams = &rp

	// R51 consumer
	k.IterateAgentAPIConfigs(ctx, func(c *types.AgentAPIConfig) bool {
		gs.AgentAPIConfigs = append(gs.AgentAPIConfigs, *c)
		return false
	})
	k.IterateAgentProfitabilities(ctx, func(p types.AgentProfitability) bool {
		gs.AgentProfitabilities = append(gs.AgentProfitabilities, p)
		return false
	})
	acp := k.GetAgentConsumerParams(ctx)
	gs.AgentConsumerParams = &acp

	// R54
	k.IterateKnowledgeGaps(ctx, func(g types.KnowledgeGap) bool {
		gs.KnowledgeGaps = append(gs.KnowledgeGaps, g)
		return false
	})
	k.IterateCurationStrategies(ctx, func(s *types.CurationStrategy) bool {
		gs.CurationStrategies = append(gs.CurationStrategies, *s)
		return false
	})
	k.IterateDomainHealths(ctx, func(h types.DomainHealth) bool {
		gs.DomainHealths = append(gs.DomainHealths, h)
		return false
	})
	csp := k.GetCurationStrategyParams(ctx)
	gs.CurationStrategyParams = &csp

	// R55
	k.IterateAgentSwarms(ctx, func(s types.AgentSwarm) bool {
		gs.AgentSwarms = append(gs.AgentSwarms, s)
		return false
	})
	k.IterateSwarmObjectives(ctx, func(o types.SwarmObjective) bool {
		gs.SwarmObjectives = append(gs.SwarmObjectives, o)
		return false
	})
	swp := k.GetSwarmParams(ctx)
	gs.SwarmParams = &swp

	// R56
	k.IterateModelEnsembles(ctx, func(e types.ModelEnsemble) bool {
		gs.ModelEnsembles = append(gs.ModelEnsembles, e)
		return false
	})
	k.IterateDistillationJobs(ctx, func(j types.DistillationJob) bool {
		gs.DistillationJobs = append(gs.DistillationJobs, j)
		return false
	})
	comp := k.GetCompositionParams(ctx)
	gs.CompositionParams = &comp

	// R57
	k.IterateEvolutionEpochs(ctx, func(e *types.EvolutionEpoch) bool {
		gs.EvolutionEpochs = append(gs.EvolutionEpochs, *e)
		return false
	})
	k.IterateMetaParameters(ctx, func(p types.MetaParameter) bool {
		gs.MetaParameters = append(gs.MetaParameters, p)
		return false
	})
	mep := k.GetMetaEvolutionParams(ctx)
	gs.MetaEvolutionParams = &mep

	// Fitness
	k.IterateFitnessRecords(ctx, func(r types.TDUFitnessRecord) bool {
		gs.FitnessRecords = append(gs.FitnessRecords, r)
		return false
	})
	fdp := k.GetFitnessDecayParams(ctx)
	gs.FitnessDecayParams = &fdp

	return gs
}

// ─── Import ─────────────────────────────────────────────────────────────────

// ImportSovereigntyGenesis restores sovereignty state from a genesis struct.
func (k Keeper) ImportSovereigntyGenesis(ctx context.Context, gs SovereigntyGenesisState) error {
	// R45-1
	for _, r := range gs.ModelRecords {
		if err := k.setModelRecordRaw(ctx, r); err != nil {
			return fmt.Errorf("failed to set model record %s: %w", r.ModelID, err)
		}
	}
	for _, l := range gs.ModelLineages {
		if err := k.setModelLineageRaw(ctx, l); err != nil {
			return fmt.Errorf("failed to set model lineage %s: %w", l.ModelID, err)
		}
	}

	// R45-2
	for i := range gs.AgentIdentities {
		if err := k.SetAgentIdentity(ctx, &gs.AgentIdentities[i]); err != nil {
			return fmt.Errorf("failed to set agent identity %s: %w", gs.AgentIdentities[i].AgentID, err)
		}
	}

	// R46
	for _, e := range gs.KnowledgeEdges {
		if err := k.setKnowledgeEdgeRaw(ctx, e); err != nil {
			return fmt.Errorf("failed to set knowledge edge %s: %w", e.EdgeID, err)
		}
	}
	for _, c := range gs.KnowledgeClusters {
		if err := k.setKnowledgeClusterRaw(ctx, c); err != nil {
			return fmt.Errorf("failed to set knowledge cluster %s: %w", c.ClusterID, err)
		}
	}

	// R47
	for i := range gs.CompetitiveBounties {
		if err := k.SetCompetitiveBounty(ctx, &gs.CompetitiveBounties[i]); err != nil {
			return fmt.Errorf("failed to set competitive bounty %s: %w", gs.CompetitiveBounties[i].BountyID, err)
		}
	}
	for _, s := range gs.BountySubmissions {
		if err := k.setBountySubmissionRaw(ctx, s); err != nil {
			return fmt.Errorf("failed to set bounty submission %s: %w", s.SubmissionID, err)
		}
	}
	if gs.BountyBoardParams != nil {
		if err := k.SetBountyBoardParams(ctx, *gs.BountyBoardParams); err != nil {
			return fmt.Errorf("failed to set bounty board params: %w", err)
		}
	}

	// R48
	for _, t := range gs.AgentTasks {
		if err := k.setAgentTaskRaw(ctx, t); err != nil {
			return fmt.Errorf("failed to set agent task %s: %w", t.TaskID, err)
		}
	}
	for _, r := range gs.AgentTaskResults {
		if err := k.setAgentTaskResultRaw(ctx, r); err != nil {
			return fmt.Errorf("failed to set agent task result %s: %w", r.TaskID, err)
		}
	}
	if gs.SchedulerParams != nil {
		if err := k.SetSchedulerParams(ctx, *gs.SchedulerParams); err != nil {
			return fmt.Errorf("failed to set scheduler params: %w", err)
		}
	}

	// R49
	for _, c := range gs.Curricula {
		if err := k.setCurriculumRaw(ctx, c); err != nil {
			return fmt.Errorf("failed to set curriculum %s: %w", c.CurriculumID, err)
		}
	}
	for _, e := range gs.CurriculumEnrollments {
		if err := k.setCurriculumEnrollmentRaw(ctx, e); err != nil {
			return fmt.Errorf("failed to set enrollment %s: %w", e.EnrollmentID, err)
		}
	}

	// R50
	for i := range gs.ActivationRecords {
		if err := k.SetActivationRecord(ctx, &gs.ActivationRecords[i]); err != nil {
			return fmt.Errorf("failed to set activation record %s: %w", gs.ActivationRecords[i].SampleID, err)
		}
	}
	if gs.ConsolidationParams != nil {
		if err := k.SetConsolidationParams(ctx, *gs.ConsolidationParams); err != nil {
			return fmt.Errorf("failed to set consolidation params: %w", err)
		}
	}

	// R51 reconsolidation
	for _, w := range gs.ReconsolidationWindows {
		if err := k.setReconsolidationWindowRaw(ctx, w); err != nil {
			return fmt.Errorf("failed to set reconsolidation window %s: %w", w.WindowID, err)
		}
	}
	for i := range gs.ReconsolidationHistories {
		if err := k.SetReconsolidationHistory(ctx, &gs.ReconsolidationHistories[i]); err != nil {
			return fmt.Errorf("failed to set reconsolidation history %s: %w", gs.ReconsolidationHistories[i].SampleID, err)
		}
	}
	if gs.ReconsolidationParams != nil {
		if err := k.SetReconsolidationParams(ctx, *gs.ReconsolidationParams); err != nil {
			return fmt.Errorf("failed to set reconsolidation params: %w", err)
		}
	}

	// R51 consumer
	for i := range gs.AgentAPIConfigs {
		if err := k.SetAgentAPIConfig(ctx, &gs.AgentAPIConfigs[i]); err != nil {
			return fmt.Errorf("failed to set agent API config %s: %w", gs.AgentAPIConfigs[i].AgentID, err)
		}
	}
	for i := range gs.AgentProfitabilities {
		if err := k.SetAgentProfitability(ctx, &gs.AgentProfitabilities[i]); err != nil {
			return fmt.Errorf("failed to set agent profitability %s: %w", gs.AgentProfitabilities[i].AgentID, err)
		}
	}
	if gs.AgentConsumerParams != nil {
		if err := k.SetAgentConsumerParams(ctx, *gs.AgentConsumerParams); err != nil {
			return fmt.Errorf("failed to set agent consumer params: %w", err)
		}
	}

	// R54
	for _, g := range gs.KnowledgeGaps {
		if err := k.setKnowledgeGapRaw(ctx, g); err != nil {
			return fmt.Errorf("failed to set knowledge gap %s: %w", g.GapID, err)
		}
	}
	for _, s := range gs.CurationStrategies {
		if err := k.setCurationStrategyRaw(ctx, s); err != nil {
			return fmt.Errorf("failed to set curation strategy %s: %w", s.StrategyID, err)
		}
	}
	for _, h := range gs.DomainHealths {
		if err := k.setDomainHealthRaw(ctx, h); err != nil {
			return fmt.Errorf("failed to set domain health %s: %w", h.Domain, err)
		}
	}
	if gs.CurationStrategyParams != nil {
		if err := k.SetCurationStrategyParams(ctx, *gs.CurationStrategyParams); err != nil {
			return fmt.Errorf("failed to set curation strategy params: %w", err)
		}
	}

	// R55
	for _, s := range gs.AgentSwarms {
		if err := k.setAgentSwarmRaw(ctx, s); err != nil {
			return fmt.Errorf("failed to set agent swarm %s: %w", s.SwarmID, err)
		}
	}
	for _, o := range gs.SwarmObjectives {
		if err := k.setSwarmObjectiveRaw(ctx, o); err != nil {
			return fmt.Errorf("failed to set swarm objective %s: %w", o.ObjectiveID, err)
		}
	}
	if gs.SwarmParams != nil {
		if err := k.SetSwarmParams(ctx, *gs.SwarmParams); err != nil {
			return fmt.Errorf("failed to set swarm params: %w", err)
		}
	}

	// R56
	for _, e := range gs.ModelEnsembles {
		if err := k.setModelEnsembleRaw(ctx, e); err != nil {
			return fmt.Errorf("failed to set model ensemble %s: %w", e.EnsembleID, err)
		}
	}
	for _, d := range gs.DistillationJobs {
		if err := k.setDistillationJobRaw(ctx, d); err != nil {
			return fmt.Errorf("failed to set distillation job %s: %w", d.JobID, err)
		}
	}
	if gs.CompositionParams != nil {
		if err := k.SetCompositionParams(ctx, *gs.CompositionParams); err != nil {
			return fmt.Errorf("failed to set composition params: %w", err)
		}
	}

	// R57
	for _, e := range gs.EvolutionEpochs {
		if err := k.setEvolutionEpochRaw(ctx, e); err != nil {
			return fmt.Errorf("failed to set evolution epoch %s: %w", e.EpochID, err)
		}
	}
	for _, p := range gs.MetaParameters {
		if err := k.SetMetaParameter(ctx, &p); err != nil {
			return fmt.Errorf("failed to set meta parameter %s: %w", p.ParamID, err)
		}
	}
	if gs.MetaEvolutionParams != nil {
		if err := k.SetMetaEvolutionParams(ctx, *gs.MetaEvolutionParams); err != nil {
			return fmt.Errorf("failed to set meta-evolution params: %w", err)
		}
	}

	// Fitness
	for _, r := range gs.FitnessRecords {
		if err := k.SetFitnessRecord(ctx, r); err != nil {
			return fmt.Errorf("failed to set fitness record %s: %w", r.SampleID, err)
		}
	}
	if gs.FitnessDecayParams != nil {
		if err := k.SetFitnessDecayParams(ctx, *gs.FitnessDecayParams); err != nil {
			return fmt.Errorf("failed to set fitness decay params: %w", err)
		}
	}

	return nil
}
