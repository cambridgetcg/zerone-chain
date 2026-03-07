package keeper

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Agent Execution Framework ──────────────────────────────────────────────
//
// This is what makes promoted agents alive. Without execution, agents are
// just records on-chain. With it, they become autonomous participants in
// the knowledge economy.
//
// The framework provides:
//   1. Task creation — protocol, governance, or other agents create work items
//   2. Task assignment — agents claim tasks matching their capabilities
//   3. Task execution — agents perform the work (submit, review, etc.)
//   4. Reward distribution — successful execution earns ZRN + reputation
//   5. Performance tracking — failures cost reputation, may trigger suspension
//
// The recursive loop becomes:
//   data → training → model → agent → [EXECUTE TASKS] → better data → repeat

// ─── Scheduler Params ───────────────────────────────────────────────────────

// SchedulerParams configures the agent task scheduler.
type SchedulerParams struct {
	DefaultTaskExpiry   int64  `json:"default_task_expiry"`   // blocks before unassigned tasks expire
	MaxTasksPerAgent    uint64 `json:"max_tasks_per_agent"`   // concurrent task limit
	ReputationGainRate  string `json:"reputation_gain_rate"`  // rep gained per successful task
	ReputationLossRate  string `json:"reputation_loss_rate"`  // rep lost per failed task
	TaskCooldownBlocks  int64  `json:"task_cooldown_blocks"`  // blocks between agent claims
	AutoCreateFromBounty bool  `json:"auto_create_from_bounty"` // auto-generate tasks from bounties
}

// DefaultSchedulerParams returns sensible defaults.
func DefaultSchedulerParams() SchedulerParams {
	return SchedulerParams{
		DefaultTaskExpiry:    50_000,                   // ~3.5 days
		MaxTasksPerAgent:     5,                        // concurrent limit
		ReputationGainRate:   "0.010000000000000000",   // +0.01 per success
		ReputationLossRate:   "0.020000000000000000",   // -0.02 per failure (asymmetric)
		TaskCooldownBlocks:   100,                      // ~10 min between claims
		AutoCreateFromBounty: true,
	}
}

// ─── CreateTask ─────────────────────────────────────────────────────────────

// CreateTask creates a new task for agents to claim and execute.
// Tasks can be created by the protocol (auto), governance, or other agents.
func (k Keeper) CreateTask(ctx context.Context, task *types.AgentTask) (string, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	kvStore := k.storeService.OpenKVStore(ctx)

	// Validate task type.
	if !types.ValidTaskTypes[task.TaskType] {
		return "", fmt.Errorf("invalid task type: %s", task.TaskType)
	}

	// Validate domain exists.
	if task.Domain != "" {
		if _, found := k.GetDomain(ctx, task.Domain); !found {
			return "", types.ErrDomainNotFound.Wrapf("domain %s", task.Domain)
		}
	}

	// Generate task ID.
	taskID := k.nextTaskID(ctx)
	task.TaskID = taskID
	task.Status = types.TaskStatusPending
	task.CreatedAt = sdkCtx.BlockHeight()

	// Set default expiry.
	if task.ExpiresAt == 0 {
		params := k.GetSchedulerParams(ctx)
		task.ExpiresAt = sdkCtx.BlockHeight() + params.DefaultTaskExpiry
	}

	// Store task.
	if err := k.setAgentTask(ctx, task); err != nil {
		return "", err
	}

	// Indexes.
	if task.Domain != "" {
		_ = kvStore.Set(types.AgentTaskDomainIndexKey(task.Domain, taskID), []byte{0x01})
	}
	_ = kvStore.Set(types.AgentTaskStatusIndexKey(string(task.Status), taskID), []byte{0x01})
	_ = kvStore.Set(types.AgentTaskTypeIndexKey(string(task.TaskType), taskID), []byte{0x01})
	if task.BountyID != "" {
		_ = kvStore.Set(types.AgentTaskBountyIndexKey(task.BountyID, taskID), []byte{0x01})
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventTaskCreated,
		sdk.NewAttribute(types.AttributeTaskID, taskID),
		sdk.NewAttribute(types.AttributeTaskType, string(task.TaskType)),
		sdk.NewAttribute("domain", task.Domain),
		sdk.NewAttribute("priority", fmt.Sprintf("%d", task.Priority)),
	))

	return taskID, nil
}

// ─── ClaimTask ──────────────────────────────────────────────────────────────

// ClaimTask allows an agent to claim a pending task for execution.
// Validates agent capability, reputation, and concurrent task limits.
func (k Keeper) ClaimTask(ctx context.Context, taskID, agentID string) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	kvStore := k.storeService.OpenKVStore(ctx)

	// Get task.
	task, found := k.GetAgentTask(ctx, taskID)
	if !found {
		return types.ErrTaskNotFound.Wrapf("task %s", taskID)
	}
	if task.Status != types.TaskStatusPending {
		return types.ErrTaskNotAssignable.Wrapf("task %s status: %s", taskID, task.Status)
	}
	if sdkCtx.BlockHeight() > task.ExpiresAt {
		return types.ErrTaskExpired.Wrapf("expired at block %d", task.ExpiresAt)
	}

	// Get agent.
	agent, found := k.GetAgentIdentity(ctx, agentID)
	if !found {
		return types.ErrAgentNotFound.Wrapf("agent %s", agentID)
	}
	if agent.Status != types.AgentStatusActive {
		return types.ErrAgentNotActive.Wrapf("agent %s status: %s", agentID, agent.Status)
	}

	// Check capability.
	if err := k.validateAgentCapability(ctx, &agent, task); err != nil {
		return err
	}

	// Check reputation meets task minimum.
	if task.MinReputation != "" {
		minRep := task.GetMinReputation()
		if agent.GetReputation().LT(minRep) {
			return types.ErrAgentReputationTooLow.Wrapf(
				"agent rep %s < task minimum %s", agent.Reputation, task.MinReputation,
			)
		}
	}

	// Check concurrent task limit.
	params := k.GetSchedulerParams(ctx)
	activeTasks := k.countAgentActiveTasks(ctx, agentID)
	if activeTasks >= params.MaxTasksPerAgent {
		return types.ErrAgentCannotPerformTask.Wrapf(
			"agent has %d active tasks (max %d)", activeTasks, params.MaxTasksPerAgent,
		)
	}

	// Update status indexes.
	_ = kvStore.Delete(types.AgentTaskStatusIndexKey(string(types.TaskStatusPending), taskID))
	_ = kvStore.Set(types.AgentTaskStatusIndexKey(string(types.TaskStatusAssigned), taskID), []byte{0x01})

	// Agent index.
	_ = kvStore.Set(types.AgentTaskAgentIndexKey(agentID, taskID), []byte{0x01})

	// Update task.
	task.Status = types.TaskStatusAssigned
	task.AssignedTo = agentID
	task.AssignedAt = sdkCtx.BlockHeight()
	if err := k.setAgentTask(ctx, task); err != nil {
		return err
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventTaskAssigned,
		sdk.NewAttribute(types.AttributeTaskID, taskID),
		sdk.NewAttribute(types.AttributeAgentID, agentID),
	))

	return nil
}

// ─── CompleteTask ───────────────────────────────────────────────────────────

// CompleteTask records successful task execution by an agent.
// Awards reputation and ZRN rewards.
func (k Keeper) CompleteTask(ctx context.Context, taskID, agentID, resultID string) (*types.AgentTaskResult, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	kvStore := k.storeService.OpenKVStore(ctx)

	task, found := k.GetAgentTask(ctx, taskID)
	if !found {
		return nil, types.ErrTaskNotFound.Wrapf("task %s", taskID)
	}
	if task.Status != types.TaskStatusAssigned {
		return nil, types.ErrTaskNotCompletable.Wrapf("task %s status: %s", taskID, task.Status)
	}
	if task.AssignedTo != agentID {
		return nil, types.ErrAgentCannotPerformTask.Wrapf("task assigned to %s, not %s", task.AssignedTo, agentID)
	}

	// Check if result already exists.
	existingResult, err := kvStore.Get(types.AgentTaskResultKey(taskID))
	if err == nil && existingResult != nil {
		return nil, types.ErrTaskResultExists.Wrapf("task %s", taskID)
	}

	params := k.GetSchedulerParams(ctx)

	// Update task status.
	_ = kvStore.Delete(types.AgentTaskStatusIndexKey(string(types.TaskStatusAssigned), taskID))
	_ = kvStore.Set(types.AgentTaskStatusIndexKey(string(types.TaskStatusCompleted), taskID), []byte{0x01})

	task.Status = types.TaskStatusCompleted
	task.CompletedAt = sdkCtx.BlockHeight()
	task.ResultID = resultID
	_ = k.setAgentTask(ctx, task)

	// Reward: reputation gain.
	repGain, _ := sdkmath.LegacyNewDecFromStr(params.ReputationGainRate)
	agent, _ := k.GetAgentIdentity(ctx, agentID)
	newRep := agent.GetReputation().Add(repGain)
	_ = k.UpdateAgentReputation(ctx, agentID, newRep)

	// Reward: ZRN payment.
	rewardPaid := sdkmath.ZeroInt()
	taskReward := task.GetRewardPool()
	if taskReward.IsPositive() {
		agentAddr, err := sdk.AccAddressFromBech32(agent.Address)
		if err == nil {
			coins := sdk.NewCoins(sdk.NewCoin("uzrn", taskReward))
			if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, agentAddr, coins); err == nil {
				rewardPaid = taskReward
				_ = k.AddAgentEarnings(ctx, agentID, rewardPaid)
			}
		}
	}

	// Record action.
	_ = k.RecordAgentAction(ctx, agentID)

	// Store result.
	result := &types.AgentTaskResult{
		TaskID:      taskID,
		AgentID:     agentID,
		Status:      types.TaskStatusCompleted,
		ResultID:    resultID,
		RewardPaid:  rewardPaid.String(),
		RepChange:   repGain.String(),
		CompletedAt: sdkCtx.BlockHeight(),
	}
	resultBz, _ := json.Marshal(result)
	_ = kvStore.Set(types.AgentTaskResultKey(taskID), resultBz)

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventTaskCompleted,
		sdk.NewAttribute(types.AttributeTaskID, taskID),
		sdk.NewAttribute(types.AttributeAgentID, agentID),
		sdk.NewAttribute("result_id", resultID),
		sdk.NewAttribute("reward", rewardPaid.String()),
	))

	return result, nil
}

// ─── FailTask ───────────────────────────────────────────────────────────────

// FailTask records a failed task execution. Penalizes agent reputation.
func (k Keeper) FailTask(ctx context.Context, taskID, agentID, reason string) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	kvStore := k.storeService.OpenKVStore(ctx)

	task, found := k.GetAgentTask(ctx, taskID)
	if !found {
		return types.ErrTaskNotFound.Wrapf("task %s", taskID)
	}
	if task.Status != types.TaskStatusAssigned {
		return types.ErrTaskNotCompletable.Wrapf("task %s status: %s", taskID, task.Status)
	}
	if task.AssignedTo != agentID {
		return types.ErrAgentCannotPerformTask.Wrapf("task assigned to %s, not %s", task.AssignedTo, agentID)
	}

	params := k.GetSchedulerParams(ctx)

	// Update status.
	_ = kvStore.Delete(types.AgentTaskStatusIndexKey(string(types.TaskStatusAssigned), taskID))
	_ = kvStore.Set(types.AgentTaskStatusIndexKey(string(types.TaskStatusFailed), taskID), []byte{0x01})

	task.Status = types.TaskStatusFailed
	task.FailureReason = reason
	task.CompletedAt = sdkCtx.BlockHeight()
	_ = k.setAgentTask(ctx, task)

	// Penalty: reputation loss (asymmetric — losing is worse than winning).
	repLoss, _ := sdkmath.LegacyNewDecFromStr(params.ReputationLossRate)
	agent, _ := k.GetAgentIdentity(ctx, agentID)
	newRep := agent.GetReputation().Sub(repLoss)
	_ = k.UpdateAgentReputation(ctx, agentID, newRep)

	// Store result.
	result := types.AgentTaskResult{
		TaskID:      taskID,
		AgentID:     agentID,
		Status:      types.TaskStatusFailed,
		RepChange:   repLoss.Neg().String(),
		CompletedAt: sdkCtx.BlockHeight(),
	}
	resultBz, _ := json.Marshal(result)
	_ = kvStore.Set(types.AgentTaskResultKey(taskID), resultBz)

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventTaskFailed,
		sdk.NewAttribute(types.AttributeTaskID, taskID),
		sdk.NewAttribute(types.AttributeAgentID, agentID),
		sdk.NewAttribute("reason", reason),
	))

	return nil
}

// ─── ExpireTasks (BeginBlocker) ─────────────────────────────────────────────

// ExpireTasks marks overdue tasks as expired and returns them to the pool.
// Called from BeginBlocker.
func (k Keeper) ExpireTasks(ctx context.Context) (expired uint64) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	blockHeight := sdkCtx.BlockHeight()

	// Check pending tasks.
	k.iterateTasksByStatus(ctx, string(types.TaskStatusPending), func(task *types.AgentTask) bool {
		if blockHeight > task.ExpiresAt {
			k.expireTask(ctx, task)
			expired++
		}
		return false
	})

	// Check assigned but not completed tasks (stale).
	k.iterateTasksByStatus(ctx, string(types.TaskStatusAssigned), func(task *types.AgentTask) bool {
		if blockHeight > task.ExpiresAt {
			// Fail the task — agent took too long.
			_ = k.FailTask(ctx, task.TaskID, task.AssignedTo, "deadline exceeded")
			expired++
		}
		return false
	})

	return expired
}

func (k Keeper) expireTask(ctx context.Context, task *types.AgentTask) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	kvStore := k.storeService.OpenKVStore(ctx)

	_ = kvStore.Delete(types.AgentTaskStatusIndexKey(string(task.Status), task.TaskID))
	_ = kvStore.Set(types.AgentTaskStatusIndexKey(string(types.TaskStatusExpired), task.TaskID), []byte{0x01})

	task.Status = types.TaskStatusExpired
	_ = k.setAgentTask(ctx, task)

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventTaskExpired,
		sdk.NewAttribute(types.AttributeTaskID, task.TaskID),
	))
}

// ─── Auto-Generate Tasks from Bounties ──────────────────────────────────────

// GenerateTasksFromBounties creates agent tasks for open competitive bounties.
// This bridges the bounty board to the execution framework — bounties become
// work items that agents can autonomously claim and fulfill.
func (k Keeper) GenerateTasksFromBounties(ctx context.Context) (created uint64) {
	params := k.GetSchedulerParams(ctx)
	if !params.AutoCreateFromBounty {
		return 0
	}

	openBounties := k.GetOpenBounties(ctx)
	for _, comp := range openBounties {
		// Check if a task already exists for this bounty.
		existingTasks := k.getTasksByBounty(ctx, comp.BountyID)
		hasPendingTask := false
		for _, t := range existingTasks {
			if t.Status == types.TaskStatusPending || t.Status == types.TaskStatusAssigned {
				hasPendingTask = true
				break
			}
		}
		if hasPendingTask {
			continue // already has an active task
		}

		// Get the base bounty for domain info.
		baseBounty, found := k.GetDataBounty(ctx, comp.BountyID)
		if !found {
			continue
		}

		task := &types.AgentTask{
			TaskType:    types.TaskTypeBountyEntry,
			Domain:      baseBounty.Domain,
			Priority:    types.TaskPriorityNormal,
			Description: fmt.Sprintf("Submit data for bounty: %s (domain: %s, subject: %s)", comp.BountyID, baseBounty.Domain, baseBounty.Subject),
			BountyID:    comp.BountyID,
			CreatedBy:   "protocol",
			RewardPool:  comp.TotalPool,
			Params: map[string]string{
				"bounty_id": comp.BountyID,
				"domain":    baseBounty.Domain,
				"subject":   baseBounty.Subject,
			},
		}

		if _, err := k.CreateTask(ctx, task); err == nil {
			created++
		}
	}

	return created
}

// ─── Task Matching ──────────────────────────────────────────────────────────

// FindTasksForAgent returns pending tasks that an agent is qualified to perform.
// Filters by capability, domain, reputation, and generation constraints.
func (k Keeper) FindTasksForAgent(ctx context.Context, agentID string) []*types.AgentTask {
	agent, found := k.GetAgentIdentity(ctx, agentID)
	if !found || agent.Status != types.AgentStatusActive {
		return nil
	}

	var matching []*types.AgentTask

	k.iterateTasksByStatus(ctx, string(types.TaskStatusPending), func(task *types.AgentTask) bool {
		// Check capability.
		if err := k.validateAgentCapability(ctx, &agent, task); err != nil {
			return false
		}

		// Check reputation.
		if task.MinReputation != "" {
			minRep := task.GetMinReputation()
			if agent.GetReputation().LT(minRep) {
				return false
			}
		}

		// Check generation limit.
		if task.MaxGeneration > 0 && agent.Generation > task.MaxGeneration {
			return false
		}

		// Domain match (prefer same domain, but don't require).
		matching = append(matching, task)
		return false
	})

	return matching
}

// ─── Queries ────────────────────────────────────────────────────────────────

// GetAgentTask retrieves a task by ID.
func (k Keeper) GetAgentTask(ctx context.Context, taskID string) (*types.AgentTask, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.AgentTaskKey(taskID))
	if err != nil || bz == nil {
		return nil, false
	}
	var task types.AgentTask
	if err := json.Unmarshal(bz, &task); err != nil {
		return nil, false
	}
	return &task, true
}

// GetAgentTaskResult retrieves the result for a completed task.
func (k Keeper) GetAgentTaskResult(ctx context.Context, taskID string) (*types.AgentTaskResult, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.AgentTaskResultKey(taskID))
	if err != nil || bz == nil {
		return nil, false
	}
	var result types.AgentTaskResult
	if err := json.Unmarshal(bz, &result); err != nil {
		return nil, false
	}
	return &result, true
}

// GetTasksByAgent returns all tasks assigned to an agent.
func (k Keeper) GetTasksByAgent(ctx context.Context, agentID string) []*types.AgentTask {
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := types.AgentTaskAgentPrefix(agentID)

	var tasks []*types.AgentTask
	iter, err := kvStore.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		taskID := string(iter.Key()[len(prefix):])
		task, found := k.GetAgentTask(ctx, taskID)
		if found {
			tasks = append(tasks, task)
		}
	}
	return tasks
}

// GetTasksByDomain returns all tasks in a domain.
func (k Keeper) GetTasksByDomain(ctx context.Context, domain string) []*types.AgentTask {
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := types.AgentTaskDomainPrefix(domain)

	var tasks []*types.AgentTask
	iter, err := kvStore.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		taskID := string(iter.Key()[len(prefix):])
		task, found := k.GetAgentTask(ctx, taskID)
		if found {
			tasks = append(tasks, task)
		}
	}
	return tasks
}

// GetTaskStats returns aggregate task statistics.
func (k Keeper) GetTaskStats(ctx context.Context) (pending, assigned, completed, failed, expired uint64) {
	for _, status := range []types.TaskStatus{types.TaskStatusPending, types.TaskStatusAssigned, types.TaskStatusCompleted, types.TaskStatusFailed, types.TaskStatusExpired} {
		k.iterateTasksByStatus(ctx, string(status), func(task *types.AgentTask) bool {
			switch status {
			case types.TaskStatusPending:
				pending++
			case types.TaskStatusAssigned:
				assigned++
			case types.TaskStatusCompleted:
				completed++
			case types.TaskStatusFailed:
				failed++
			case types.TaskStatusExpired:
				expired++
			}
			return false
		})
	}
	return
}

// ─── Params ─────────────────────────────────────────────────────────────────

// GetSchedulerParams retrieves scheduler parameters.
func (k Keeper) GetSchedulerParams(ctx context.Context) SchedulerParams {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.AgentTaskSchedulerParamsKey)
	if err != nil || bz == nil {
		return DefaultSchedulerParams()
	}
	var params SchedulerParams
	if err := json.Unmarshal(bz, &params); err != nil {
		return DefaultSchedulerParams()
	}
	return params
}

// SetSchedulerParams stores scheduler parameters.
func (k Keeper) SetSchedulerParams(ctx context.Context, params SchedulerParams) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("failed to marshal scheduler params: %w", err)
	}
	return kvStore.Set(types.AgentTaskSchedulerParamsKey, bz)
}

// ─── Internal ───────────────────────────────────────────────────────────────

func (k Keeper) setAgentTask(ctx context.Context, task *types.AgentTask) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("failed to marshal agent task: %w", err)
	}
	return kvStore.Set(types.AgentTaskKey(task.TaskID), bz)
}

func (k Keeper) nextTaskID(ctx context.Context) string {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.AgentTaskSeqKey)
	var seq uint64 = 1
	if err == nil && len(bz) == 8 {
		seq = binary.BigEndian.Uint64(bz)
	}

	// Deterministic ID from sequence.
	input := fmt.Sprintf("task:%d", seq)
	hash := sha256.Sum256([]byte(input))
	taskID := hex.EncodeToString(hash[:16]) // 32-char hex

	// Increment.
	next := make([]byte, 8)
	binary.BigEndian.PutUint64(next, seq+1)
	_ = kvStore.Set(types.AgentTaskSeqKey, next)

	return taskID
}

func (k Keeper) validateAgentCapability(_ context.Context, agent *types.AgentIdentity, task *types.AgentTask) error {
	switch task.TaskType {
	case types.TaskTypeSubmit, types.TaskTypeBountyEntry:
		if !agent.CanSubmit {
			return types.ErrAgentCannotPerformTask.Wrapf("agent %s cannot submit", agent.AgentID)
		}
	case types.TaskTypeReview:
		if !agent.CanReview {
			return types.ErrAgentCannotPerformTask.Wrapf("agent %s cannot review (benchmark too low)", agent.AgentID)
		}
	case types.TaskTypeEdge:
		if !agent.CanSubmit { // creating edges requires submission capability
			return types.ErrAgentCannotPerformTask.Wrapf("agent %s cannot create edges", agent.AgentID)
		}
	default:
		return types.ErrAgentCannotPerformTask.Wrapf("unknown task type: %s", task.TaskType)
	}
	return nil
}

func (k Keeper) countAgentActiveTasks(ctx context.Context, agentID string) uint64 {
	var count uint64
	tasks := k.GetTasksByAgent(ctx, agentID)
	for _, t := range tasks {
		if t.Status == types.TaskStatusAssigned {
			count++
		}
	}
	return count
}

func (k Keeper) iterateTasksByStatus(ctx context.Context, status string, cb func(task *types.AgentTask) bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := types.AgentTaskStatusPrefix(status)

	iter, err := kvStore.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		taskID := string(iter.Key()[len(prefix):])
		task, found := k.GetAgentTask(ctx, taskID)
		if found {
			if cb(task) {
				break
			}
		}
	}
}

func (k Keeper) getTasksByBounty(ctx context.Context, bountyID string) []*types.AgentTask {
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := append(append([]byte{}, types.AgentTaskByBountyPrefix...), []byte(bountyID)...)
	prefix = append(prefix, '/')

	var tasks []*types.AgentTask
	iter, err := kvStore.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		taskID := string(iter.Key()[len(prefix):])
		task, found := k.GetAgentTask(ctx, taskID)
		if found {
			tasks = append(tasks, task)
		}
	}
	return tasks
}
