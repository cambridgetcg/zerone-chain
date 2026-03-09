package keeper

import (
	"context"
	"encoding/json"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── R45-1: Model Registry Iteration ────────────────────────────────────────

func (k Keeper) IterateModelRecords(ctx context.Context, cb func(types.ModelRecord) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.ModelRecordPrefix, prefixEndBytes(types.ModelRecordPrefix))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var rec types.ModelRecord
		if err := json.Unmarshal(iter.Value(), &rec); err != nil {
			continue
		}
		if cb(rec) {
			break
		}
	}
}

func (k Keeper) setModelRecordRaw(ctx context.Context, rec types.ModelRecord) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	return store.Set(types.ModelRecordKey(rec.ModelID), bz)
}

func (k Keeper) IterateModelLineages(ctx context.Context, cb func(types.ModelLineage) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.ModelLineagePrefix, prefixEndBytes(types.ModelLineagePrefix))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var lin types.ModelLineage
		if err := json.Unmarshal(iter.Value(), &lin); err != nil {
			continue
		}
		if cb(lin) {
			break
		}
	}
}

func (k Keeper) setModelLineageRaw(ctx context.Context, lin types.ModelLineage) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(lin)
	if err != nil {
		return err
	}
	return store.Set(types.ModelLineageKey(lin.ModelID), bz)
}

// ─── R45-2: Agent Promotion Iteration ───────────────────────────────────────

func (k Keeper) IterateAgentIdentities(ctx context.Context, cb func(types.AgentIdentity) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.AgentIdentityPrefix, prefixEndBytes(types.AgentIdentityPrefix))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var agent types.AgentIdentity
		if err := json.Unmarshal(iter.Value(), &agent); err != nil {
			continue
		}
		if cb(agent) {
			break
		}
	}
}

// ─── R46: Knowledge Graph Iteration ─────────────────────────────────────────

func (k Keeper) IterateKnowledgeEdges(ctx context.Context, cb func(types.KnowledgeEdge) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.KnowledgeEdgePrefix, prefixEndBytes(types.KnowledgeEdgePrefix))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var edge types.KnowledgeEdge
		if err := json.Unmarshal(iter.Value(), &edge); err != nil {
			continue
		}
		if cb(edge) {
			break
		}
	}
}

func (k Keeper) setKnowledgeEdgeRaw(ctx context.Context, edge types.KnowledgeEdge) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(edge)
	if err != nil {
		return err
	}
	return store.Set(types.KnowledgeEdgeKey(edge.EdgeID), bz)
}

func (k Keeper) IterateKnowledgeClusters(ctx context.Context, cb func(types.KnowledgeCluster) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.KnowledgeClusterPrefix, prefixEndBytes(types.KnowledgeClusterPrefix))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var cluster types.KnowledgeCluster
		if err := json.Unmarshal(iter.Value(), &cluster); err != nil {
			continue
		}
		if cb(cluster) {
			break
		}
	}
}

func (k Keeper) setKnowledgeClusterRaw(ctx context.Context, cluster types.KnowledgeCluster) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(cluster)
	if err != nil {
		return err
	}
	return store.Set(types.KnowledgeClusterKey(cluster.ClusterID), bz)
}

// ─── R47: Bounty Board Iteration ────────────────────────────────────────────
// IterateCompetitiveBounties is defined in bounty_board.go.

func (k Keeper) IterateBountySubmissions(ctx context.Context, cb func(BountySubmission) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.BountySubmissionPrefix, prefixEndBytes(types.BountySubmissionPrefix))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var sub BountySubmission
		if err := json.Unmarshal(iter.Value(), &sub); err != nil {
			continue
		}
		if cb(sub) {
			break
		}
	}
}

func (k Keeper) setBountySubmissionRaw(ctx context.Context, sub BountySubmission) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(sub)
	if err != nil {
		return err
	}
	return store.Set(types.BountySubmissionKey(sub.SubmissionID), bz)
}

// ─── R48: Agent Execution Iteration ─────────────────────────────────────────

func (k Keeper) IterateAgentTasks(ctx context.Context, cb func(types.AgentTask) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.AgentTaskPrefix, prefixEndBytes(types.AgentTaskPrefix))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var task types.AgentTask
		if err := json.Unmarshal(iter.Value(), &task); err != nil {
			continue
		}
		if cb(task) {
			break
		}
	}
}

func (k Keeper) setAgentTaskRaw(ctx context.Context, task types.AgentTask) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(task)
	if err != nil {
		return err
	}
	return store.Set(types.AgentTaskKey(task.TaskID), bz)
}

func (k Keeper) IterateAgentTaskResults(ctx context.Context, cb func(types.AgentTaskResult) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.AgentTaskResultPrefix, prefixEndBytes(types.AgentTaskResultPrefix))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var result types.AgentTaskResult
		if err := json.Unmarshal(iter.Value(), &result); err != nil {
			continue
		}
		if cb(result) {
			break
		}
	}
}

func (k Keeper) setAgentTaskResultRaw(ctx context.Context, result types.AgentTaskResult) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return store.Set(types.AgentTaskResultKey(result.TaskID), bz)
}

// ─── R49: Curriculum Training Iteration ─────────────────────────────────────

func (k Keeper) IterateCurricula(ctx context.Context, cb func(types.Curriculum) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.CurriculumPrefix, prefixEndBytes(types.CurriculumPrefix))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var cur types.Curriculum
		if err := json.Unmarshal(iter.Value(), &cur); err != nil {
			continue
		}
		if cb(cur) {
			break
		}
	}
}

func (k Keeper) setCurriculumRaw(ctx context.Context, cur types.Curriculum) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(cur)
	if err != nil {
		return err
	}
	return store.Set(types.CurriculumKey(cur.CurriculumID), bz)
}

func (k Keeper) IterateCurriculumEnrollments(ctx context.Context, cb func(types.CurriculumEnrollment) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.CurriculumEnrollmentPrefix, prefixEndBytes(types.CurriculumEnrollmentPrefix))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var enroll types.CurriculumEnrollment
		if err := json.Unmarshal(iter.Value(), &enroll); err != nil {
			continue
		}
		if cb(enroll) {
			break
		}
	}
}

func (k Keeper) setCurriculumEnrollmentRaw(ctx context.Context, enroll types.CurriculumEnrollment) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(enroll)
	if err != nil {
		return err
	}
	return store.Set(types.CurriculumEnrollmentKey(enroll.EnrollmentID), bz)
}

// ─── R50: Memory Consolidation Iteration ────────────────────────────────────
// IterateActivationRecords is defined in memory_consolidation.go.

// ─── R51: Reconsolidation Iteration ─────────────────────────────────────────

func (k Keeper) IterateReconsolidationWindows(ctx context.Context, cb func(types.ReconsolidationWindow) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.ReconsolidationWindowPrefix, prefixEndBytes(types.ReconsolidationWindowPrefix))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var win types.ReconsolidationWindow
		if err := json.Unmarshal(iter.Value(), &win); err != nil {
			continue
		}
		if cb(win) {
			break
		}
	}
}

func (k Keeper) setReconsolidationWindowRaw(ctx context.Context, win types.ReconsolidationWindow) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(win)
	if err != nil {
		return err
	}
	return store.Set(types.ReconsolidationWindowKey(win.WindowID), bz)
}

func (k Keeper) IterateReconsolidationHistories(ctx context.Context, cb func(types.ReconsolidationHistory) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.ReconsolidationHistoryPrefix, prefixEndBytes(types.ReconsolidationHistoryPrefix))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var hist types.ReconsolidationHistory
		if err := json.Unmarshal(iter.Value(), &hist); err != nil {
			continue
		}
		if cb(hist) {
			break
		}
	}
}

// ─── R51: Agent Consumer Iteration ──────────────────────────────────────────
// IterateAgentAPIConfigs is defined in agent_consumer.go.

func (k Keeper) IterateAgentProfitabilities(ctx context.Context, cb func(types.AgentProfitability) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.AgentProfitabilityPrefix, prefixEndBytes(types.AgentProfitabilityPrefix))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var prof types.AgentProfitability
		if err := json.Unmarshal(iter.Value(), &prof); err != nil {
			continue
		}
		if cb(prof) {
			break
		}
	}
}

// ─── R54: Strategic Curation Iteration ──────────────────────────────────────

func (k Keeper) IterateKnowledgeGaps(ctx context.Context, cb func(types.KnowledgeGap) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.KnowledgeGapPrefix, prefixEndBytes(types.KnowledgeGapPrefix))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var gap types.KnowledgeGap
		if err := json.Unmarshal(iter.Value(), &gap); err != nil {
			continue
		}
		if cb(gap) {
			break
		}
	}
}

func (k Keeper) setKnowledgeGapRaw(ctx context.Context, gap types.KnowledgeGap) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(gap)
	if err != nil {
		return err
	}
	return store.Set(types.KnowledgeGapKey(gap.GapID), bz)
}

// IterateCurationStrategies is defined in meta_evolution.go.

func (k Keeper) setCurationStrategyRaw(ctx context.Context, strat types.CurationStrategy) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(strat)
	if err != nil {
		return err
	}
	return store.Set(types.CurationStrategyKey(strat.StrategyID), bz)
}

func (k Keeper) IterateDomainHealths(ctx context.Context, cb func(types.DomainHealth) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.DomainHealthPrefix, prefixEndBytes(types.DomainHealthPrefix))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var health types.DomainHealth
		if err := json.Unmarshal(iter.Value(), &health); err != nil {
			continue
		}
		if cb(health) {
			break
		}
	}
}

func (k Keeper) setDomainHealthRaw(ctx context.Context, health types.DomainHealth) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(health)
	if err != nil {
		return err
	}
	return store.Set(types.DomainHealthKey(health.Domain), bz)
}

// ─── R55: Agent Swarm Iteration ─────────────────────────────────────────────

func (k Keeper) IterateAgentSwarms(ctx context.Context, cb func(types.AgentSwarm) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.AgentSwarmPrefix, prefixEndBytes(types.AgentSwarmPrefix))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var swarm types.AgentSwarm
		if err := json.Unmarshal(iter.Value(), &swarm); err != nil {
			continue
		}
		if cb(swarm) {
			break
		}
	}
}

func (k Keeper) setAgentSwarmRaw(ctx context.Context, swarm types.AgentSwarm) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(swarm)
	if err != nil {
		return err
	}
	return store.Set(types.AgentSwarmKey(swarm.SwarmID), bz)
}

func (k Keeper) IterateSwarmObjectives(ctx context.Context, cb func(types.SwarmObjective) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.SwarmObjectivePrefix, prefixEndBytes(types.SwarmObjectivePrefix))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var obj types.SwarmObjective
		if err := json.Unmarshal(iter.Value(), &obj); err != nil {
			continue
		}
		if cb(obj) {
			break
		}
	}
}

func (k Keeper) setSwarmObjectiveRaw(ctx context.Context, obj types.SwarmObjective) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	return store.Set(types.SwarmObjectiveKey(obj.ObjectiveID), bz)
}

// ─── R56: Model Composition Iteration ───────────────────────────────────────

func (k Keeper) IterateModelEnsembles(ctx context.Context, cb func(types.ModelEnsemble) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.ModelEnsemblePrefix, prefixEndBytes(types.ModelEnsemblePrefix))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var ens types.ModelEnsemble
		if err := json.Unmarshal(iter.Value(), &ens); err != nil {
			continue
		}
		if cb(ens) {
			break
		}
	}
}

func (k Keeper) setModelEnsembleRaw(ctx context.Context, ens types.ModelEnsemble) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(ens)
	if err != nil {
		return err
	}
	return store.Set(types.ModelEnsembleKey(ens.EnsembleID), bz)
}

func (k Keeper) IterateDistillationJobs(ctx context.Context, cb func(types.DistillationJob) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.DistillationJobPrefix, prefixEndBytes(types.DistillationJobPrefix))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var job types.DistillationJob
		if err := json.Unmarshal(iter.Value(), &job); err != nil {
			continue
		}
		if cb(job) {
			break
		}
	}
}

func (k Keeper) setDistillationJobRaw(ctx context.Context, job types.DistillationJob) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(job)
	if err != nil {
		return err
	}
	return store.Set(types.DistillationJobKey(job.JobID), bz)
}

// ─── R57: Meta-Evolution Iteration ──────────────────────────────────────────
// IterateEvolutionEpochs is defined in meta_evolution.go.

func (k Keeper) setEvolutionEpochRaw(ctx context.Context, epoch types.EvolutionEpoch) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(epoch)
	if err != nil {
		return err
	}
	return store.Set(types.EvolutionEpochKey(epoch.EpochID), bz)
}

func (k Keeper) IterateMetaParameters(ctx context.Context, cb func(types.MetaParameter) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.MetaParameterPrefix, prefixEndBytes(types.MetaParameterPrefix))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var param types.MetaParameter
		if err := json.Unmarshal(iter.Value(), &param); err != nil {
			continue
		}
		if cb(param) {
			break
		}
	}
}

// IterateFitnessRecords — defined in fitness.go
