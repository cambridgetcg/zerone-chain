package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/keeper"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// sampleSovereigntyGenesis returns a populated SovereigntyGenesisState for roundtrip testing.
func sampleSovereigntyGenesis() keeper.SovereigntyGenesisState {
	gs := keeper.DefaultSovereigntyGenesis()

	// R45-1
	gs.ModelRecords = []types.ModelRecord{
		{ModelID: "model-001", Name: "TestModel", Domain: "science", Version: 1, Status: types.ModelStatusActive},
		{ModelID: "model-002", Name: "TestModel2", Domain: "technology", Version: 1, ParentModelID: "", Status: types.ModelStatusActive},
	}
	gs.ModelLineages = []types.ModelLineage{
		{ModelID: "model-001", Ancestors: []string{}, Generation: 0},
		{ModelID: "model-002", Ancestors: []string{}, Generation: 0},
	}

	// R45-2
	gs.AgentIdentities = []types.AgentIdentity{
		{AgentID: "agent-001", ModelID: "model-001", Domain: "science", Generation: 0, CanSubmit: true, Reputation: "0.5"},
		{AgentID: "agent-002", ModelID: "model-002", Domain: "technology", Generation: 0, CanSubmit: true, CanReview: true, Reputation: "0.7"},
	}

	// R46
	gs.KnowledgeEdges = []types.KnowledgeEdge{
		{EdgeID: "edge-001", SourceID: "tdu-001", TargetID: "tdu-002", EdgeType: "supports", Weight: "0.8"},
		{EdgeID: "edge-002", SourceID: "tdu-003", TargetID: "tdu-001", EdgeType: "extends", Weight: "0.6"},
	}
	gs.KnowledgeClusters = []types.KnowledgeCluster{
		{ClusterID: "cluster-001", Domain: "science", Label: "physics", MemberIDs: []string{"tdu-001", "tdu-002"}, EdgeCount: 1},
	}

	// R47
	gs.CompetitiveBounties = []keeper.CompetitiveBounty{
		{BountyID: "bounty-001", Status: "open", TotalPool: "5000000", SubmissionCount: 1},
	}
	gs.BountySubmissions = []keeper.BountySubmission{
		{SubmissionID: "bsub-001", BountyID: "bounty-001", SampleID: "tdu-001", Submitter: "zrn1test"},
	}

	// R48
	gs.AgentTasks = []types.AgentTask{
		{TaskID: "task-001", Domain: "science", Status: "pending", CreatedAt: 100},
		{TaskID: "task-002", Domain: "technology", Status: "completed", CreatedAt: 110, CompletedAt: 120},
	}
	gs.AgentTaskResults = []types.AgentTaskResult{
		{TaskID: "task-002", AgentID: "agent-002", Status: "completed", CompletedAt: 120},
	}

	// R49
	gs.Curricula = []types.Curriculum{
		{CurriculumID: "curr-001", Name: "Intro Physics", Domain: "science", Version: 1, CreatedAt: 100},
	}
	gs.CurriculumEnrollments = []types.CurriculumEnrollment{
		{EnrollmentID: "enroll-001", CurriculumID: "curr-001", AgentID: "agent-001", EnrolledAt: 150, Status: "active"},
	}

	// R50
	gs.ActivationRecords = []types.ActivationRecord{
		{SampleID: "tdu-001", TotalActivations: 5, UniqueCycles: 3, MemoryTier: int(types.MemoryTierActive)},
		{SampleID: "tdu-002", TotalActivations: 12, UniqueCycles: 8, MemoryTier: int(types.MemoryTierConsolidated)},
	}

	// R51 reconsolidation
	gs.ReconsolidationWindows = []types.ReconsolidationWindow{
		{WindowID: "win-001", SampleID: "tdu-003", TriggeredAt: 200, ExpiresAt: 311, Status: types.ReconsolidationOpen},
	}
	gs.ReconsolidationHistories = []types.ReconsolidationHistory{
		{SampleID: "tdu-003", TotalWindows: 1, UncorrectedCount: 0, CorrectedCount: 0, ActiveWindowID: "win-001"},
	}

	// R51 consumer
	gs.AgentAPIConfigs = []types.AgentAPIConfig{
		{AgentID: "agent-001", AutoSelect: true, TotalCalls: 10, TotalSpent: "100000", CreatedAt: 100},
	}
	gs.AgentProfitabilities = []types.AgentProfitability{
		{AgentID: "agent-001", TotalEarned: "500000", TotalSpent: "100000", NetProfitLoss: "400000", ProfitRatio: "5.0"},
	}

	// R54
	gs.KnowledgeGaps = []types.KnowledgeGap{
		{GapID: "gap-001", Domain: "science", GapType: types.GapTypeCoverage, Severity: "0.7", Status: "open"},
	}
	gs.CurationStrategies = []types.CurationStrategy{
		{StrategyID: "strat-001", AgentID: "agent-001", FocusDomains: []string{"science"}, GapsIdentified: 3},
	}
	gs.DomainHealths = []types.DomainHealth{
		{Domain: "science", TotalTDUs: 100, ActiveTDUs: 80, HealthScore: "0.75"},
	}

	// R55
	gs.AgentSwarms = []types.AgentSwarm{
		{SwarmID: "swarm-001", Name: "SciSwarm", Domain: "science", Status: types.SwarmStatusActive, MinMembers: 2, MaxMembers: 10},
	}
	gs.SwarmObjectives = []types.SwarmObjective{
		{ObjectiveID: "obj-001", SwarmID: "swarm-001", Description: "Fill physics gap", TargetTDUs: 50, Status: "active"},
	}

	// R56
	gs.ModelEnsembles = []types.ModelEnsemble{
		{EnsembleID: "ens-001", Name: "SciTech Ensemble", RoutingType: types.RoutingDomain, Status: "active"},
	}
	gs.DistillationJobs = []types.DistillationJob{
		{JobID: "distill-001", EnsembleID: "ens-001", Status: "pending", CaptureCount: 500},
	}

	// R57
	gs.EvolutionEpochs = []types.EvolutionEpoch{
		{EpochID: "epoch-001", Domain: "science", StartBlock: 1000, EndBlock: 11000, Status: "completed"},
	}
	gs.MetaParameters = []types.MetaParameter{
		{ParamID: "mp-001", Name: "fitness_weight", CurrentValue: "0.5", MinValue: "0.1", MaxValue: "0.9"},
	}

	// Fitness
	gs.FitnessRecords = []types.TDUFitnessRecord{
		{SampleID: "tdu-001", FitnessScore: "0.75", OriginalStake: "1000000", CycleCount: 10},
		{SampleID: "tdu-002", FitnessScore: "0.85", OriginalStake: "2000000", CycleCount: 20},
	}

	return gs
}

func TestSovereigntyGenesisRoundtrip(t *testing.T) {
	k, ctx := setupKeeper(t)

	original := sampleSovereigntyGenesis()

	// Validate before import.
	require.NoError(t, keeper.ValidateSovereigntyGenesis(original))

	// Import.
	require.NoError(t, k.ImportSovereigntyGenesis(ctx, original))

	// Export.
	exported := k.ExportSovereigntyGenesis(ctx)

	// ── R45-1: Model records ──
	require.Len(t, exported.ModelRecords, len(original.ModelRecords))
	for i, m := range exported.ModelRecords {
		require.Equal(t, original.ModelRecords[i].ModelID, m.ModelID)
		require.Equal(t, original.ModelRecords[i].Name, m.Name)
		require.Equal(t, original.ModelRecords[i].Domain, m.Domain)
	}

	require.Len(t, exported.ModelLineages, len(original.ModelLineages))
	for i, l := range exported.ModelLineages {
		require.Equal(t, original.ModelLineages[i].ModelID, l.ModelID)
	}

	// ── R45-2: Agent identities ──
	require.Len(t, exported.AgentIdentities, len(original.AgentIdentities))
	for i, a := range exported.AgentIdentities {
		require.Equal(t, original.AgentIdentities[i].AgentID, a.AgentID)
		require.Equal(t, original.AgentIdentities[i].ModelID, a.ModelID)
	}

	// ── R46: Knowledge graph ──
	require.Len(t, exported.KnowledgeEdges, len(original.KnowledgeEdges))
	for i, e := range exported.KnowledgeEdges {
		require.Equal(t, original.KnowledgeEdges[i].EdgeID, e.EdgeID)
		require.Equal(t, original.KnowledgeEdges[i].SourceID, e.SourceID)
		require.Equal(t, original.KnowledgeEdges[i].TargetID, e.TargetID)
	}
	require.Len(t, exported.KnowledgeClusters, len(original.KnowledgeClusters))

	// ── R47: Bounty board ──
	require.Len(t, exported.CompetitiveBounties, len(original.CompetitiveBounties))
	require.Equal(t, original.CompetitiveBounties[0].BountyID, exported.CompetitiveBounties[0].BountyID)
	require.Len(t, exported.BountySubmissions, len(original.BountySubmissions))
	require.NotNil(t, exported.BountyBoardParams)

	// ── R48: Agent tasks ──
	require.Len(t, exported.AgentTasks, len(original.AgentTasks))
	for i, task := range exported.AgentTasks {
		require.Equal(t, original.AgentTasks[i].TaskID, task.TaskID)
	}
	require.Len(t, exported.AgentTaskResults, len(original.AgentTaskResults))
	require.NotNil(t, exported.SchedulerParams)

	// ── R49: Curricula ──
	require.Len(t, exported.Curricula, len(original.Curricula))
	require.Len(t, exported.CurriculumEnrollments, len(original.CurriculumEnrollments))

	// ── R50: Activation records ──
	require.Len(t, exported.ActivationRecords, len(original.ActivationRecords))
	for i, r := range exported.ActivationRecords {
		require.Equal(t, original.ActivationRecords[i].SampleID, r.SampleID)
		require.Equal(t, original.ActivationRecords[i].TotalActivations, r.TotalActivations)
	}
	require.NotNil(t, exported.ConsolidationParams)

	// ── R51: Reconsolidation ──
	require.Len(t, exported.ReconsolidationWindows, len(original.ReconsolidationWindows))
	require.Len(t, exported.ReconsolidationHistories, len(original.ReconsolidationHistories))
	require.NotNil(t, exported.ReconsolidationParams)

	// ── R51: Agent consumer ──
	require.Len(t, exported.AgentAPIConfigs, len(original.AgentAPIConfigs))
	require.Len(t, exported.AgentProfitabilities, len(original.AgentProfitabilities))
	require.NotNil(t, exported.AgentConsumerParams)

	// ── R54: Curation ──
	require.Len(t, exported.KnowledgeGaps, len(original.KnowledgeGaps))
	require.Equal(t, original.KnowledgeGaps[0].GapID, exported.KnowledgeGaps[0].GapID)
	require.Len(t, exported.CurationStrategies, len(original.CurationStrategies))
	require.Len(t, exported.DomainHealths, len(original.DomainHealths))
	require.NotNil(t, exported.CurationStrategyParams)

	// ── R55: Swarms ──
	require.Len(t, exported.AgentSwarms, len(original.AgentSwarms))
	require.Equal(t, original.AgentSwarms[0].SwarmID, exported.AgentSwarms[0].SwarmID)
	require.Len(t, exported.SwarmObjectives, len(original.SwarmObjectives))
	require.NotNil(t, exported.SwarmParams)

	// ── R56: Composition ──
	require.Len(t, exported.ModelEnsembles, len(original.ModelEnsembles))
	require.Equal(t, original.ModelEnsembles[0].EnsembleID, exported.ModelEnsembles[0].EnsembleID)
	require.Len(t, exported.DistillationJobs, len(original.DistillationJobs))
	require.NotNil(t, exported.CompositionParams)

	// ── R57: Meta-evolution ──
	require.Len(t, exported.EvolutionEpochs, len(original.EvolutionEpochs))
	require.Equal(t, original.EvolutionEpochs[0].EpochID, exported.EvolutionEpochs[0].EpochID)
	require.Len(t, exported.MetaParameters, len(original.MetaParameters))
	require.NotNil(t, exported.MetaEvolutionParams)

	// ── Fitness ──
	require.Len(t, exported.FitnessRecords, len(original.FitnessRecords))
	for i, r := range exported.FitnessRecords {
		require.Equal(t, original.FitnessRecords[i].SampleID, r.SampleID)
		require.Equal(t, original.FitnessRecords[i].FitnessScore, r.FitnessScore)
	}
	require.NotNil(t, exported.FitnessDecayParams)
}

func TestSovereigntyGenesisValidation_DuplicateModelID(t *testing.T) {
	gs := keeper.DefaultSovereigntyGenesis()
	gs.ModelRecords = []types.ModelRecord{
		{ModelID: "dup", Name: "A"},
		{ModelID: "dup", Name: "B"},
	}
	err := keeper.ValidateSovereigntyGenesis(gs)
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate model ID")
}

func TestSovereigntyGenesisValidation_AgentRefsMissingModel(t *testing.T) {
	gs := keeper.DefaultSovereigntyGenesis()
	gs.AgentIdentities = []types.AgentIdentity{
		{AgentID: "a1", ModelID: "nonexistent"},
	}
	err := keeper.ValidateSovereigntyGenesis(gs)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown model")
}

func TestSovereigntyGenesisValidation_EdgeSelfRef(t *testing.T) {
	gs := keeper.DefaultSovereigntyGenesis()
	gs.KnowledgeEdges = []types.KnowledgeEdge{
		{EdgeID: "e1", SourceID: "same", TargetID: "same"},
	}
	err := keeper.ValidateSovereigntyGenesis(gs)
	require.Error(t, err)
	require.Contains(t, err.Error(), "self-reference")
}

func TestSovereigntyGenesisValidation_SubmissionRefsMissingBounty(t *testing.T) {
	gs := keeper.DefaultSovereigntyGenesis()
	gs.BountySubmissions = []keeper.BountySubmission{
		{SubmissionID: "s1", BountyID: "nonexistent"},
	}
	err := keeper.ValidateSovereigntyGenesis(gs)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown bounty")
}

func TestSovereigntyGenesisValidation_TaskResultRefsMissingTask(t *testing.T) {
	gs := keeper.DefaultSovereigntyGenesis()
	gs.AgentTaskResults = []types.AgentTaskResult{
		{TaskID: "nonexistent"},
	}
	err := keeper.ValidateSovereigntyGenesis(gs)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown task")
}

func TestSovereigntyGenesisValidation_EmptyIsValid(t *testing.T) {
	gs := keeper.DefaultSovereigntyGenesis()
	require.NoError(t, keeper.ValidateSovereigntyGenesis(gs))
}

func TestSovereigntyGenesisDefaultParams(t *testing.T) {
	gs := keeper.DefaultSovereigntyGenesis()
	require.NotNil(t, gs.BountyBoardParams)
	require.NotNil(t, gs.SchedulerParams)
	require.NotNil(t, gs.ConsolidationParams)
	require.NotNil(t, gs.ReconsolidationParams)
	require.NotNil(t, gs.AgentConsumerParams)
	require.NotNil(t, gs.CurationStrategyParams)
	require.NotNil(t, gs.SwarmParams)
	require.NotNil(t, gs.CompositionParams)
	require.NotNil(t, gs.MetaEvolutionParams)
	require.NotNil(t, gs.FitnessDecayParams)
}
