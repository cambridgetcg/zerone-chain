package keeper_test

import (
	"context"
	"testing"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/keeper"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Helpers ────────────────────────────────────────────────────────────────

func setupExecutionTest(t *testing.T) (keeper.Keeper, context.Context, *mockBankKeeper) {
	t.Helper()
	k, ctx, bk := setupKeeperWithBank(t)
	setupDefaultDomains(t, k, ctx)
	return k, ctx, bk
}

// setupAgent creates an agent identity for testing.
func setupAgent(t *testing.T, k keeper.Keeper, ctx context.Context, agentID, domain string, canSubmit, canReview bool) {
	t.Helper()
	agent := &types.AgentIdentity{
		AgentID:       agentID,
		ModelID:       "model-" + agentID,
		Address:       testAddr,
		Domain:        domain,
		Generation:    0,
		CanSubmit:     canSubmit,
		CanReview:     canReview,
		CanTrain:      false,
		Status:        types.AgentStatusActive,
		PromotedAt:    50,
		SponsorAddr:   testAddr,
		InitialStake:  "10000000",
		Reputation:    "0.500000000000000000",
		EarningsTotal: "0",
	}
	require.NoError(t, k.SetAgentIdentity(ctx, agent))
}

func newSubmitTask(domain string) *types.AgentTask {
	return &types.AgentTask{
		TaskType:    types.TaskTypeSubmit,
		Domain:      domain,
		Priority:    types.TaskPriorityNormal,
		Description: "Submit training data for " + domain,
		CreatedBy:   "protocol",
		RewardPool:  "1000000",
	}
}

func newReviewTask(domain string) *types.AgentTask {
	return &types.AgentTask{
		TaskType:    types.TaskTypeReview,
		Domain:      domain,
		Priority:    types.TaskPriorityHigh,
		Description: "Review pending submission in " + domain,
		CreatedBy:   "protocol",
		RewardPool:  "500000",
	}
}

func newBountyTask(domain, bountyID string) *types.AgentTask {
	return &types.AgentTask{
		TaskType:    types.TaskTypeBountyEntry,
		Domain:      domain,
		BountyID:    bountyID,
		Priority:    types.TaskPriorityNormal,
		Description: "Submit to bounty " + bountyID,
		CreatedBy:   "protocol",
		RewardPool:  "5000000",
		Params: map[string]string{
			"bounty_id": bountyID,
			"domain":    domain,
		},
	}
}

// ─── Test: Create Task — Happy Path ─────────────────────────────────────────

func TestCreateTask_HappyPath(t *testing.T) {
	k, ctx, _ := setupExecutionTest(t)

	task := newSubmitTask("technology")
	taskID, err := k.CreateTask(ctx, task)
	require.NoError(t, err)
	require.NotEmpty(t, taskID)

	// Verify stored.
	stored, found := k.GetAgentTask(ctx, taskID)
	require.True(t, found)
	require.Equal(t, taskID, stored.TaskID)
	require.Equal(t, types.TaskTypeSubmit, stored.TaskType)
	require.Equal(t, "technology", stored.Domain)
	require.Equal(t, types.TaskStatusPending, stored.Status)
	require.Equal(t, int64(100), stored.CreatedAt) // block height from setup
	require.True(t, stored.ExpiresAt > stored.CreatedAt)
}

// ─── Test: Create Task — Invalid Type ───────────────────────────────────────

func TestCreateTask_InvalidType(t *testing.T) {
	k, ctx, _ := setupExecutionTest(t)

	task := &types.AgentTask{
		TaskType: "nonexistent",
		Domain:   "technology",
	}
	_, err := k.CreateTask(ctx, task)
	require.Error(t, err)
}

// ─── Test: Create Task — Domain Not Found ───────────────────────────────────

func TestCreateTask_DomainNotFound(t *testing.T) {
	k, ctx, _ := setupExecutionTest(t)

	task := &types.AgentTask{
		TaskType: types.TaskTypeSubmit,
		Domain:   "nonexistent",
	}
	_, err := k.CreateTask(ctx, task)
	require.ErrorIs(t, err, types.ErrDomainNotFound)
}

// ─── Test: Claim Task — Happy Path ──────────────────────────────────────────

func TestClaimTask_HappyPath(t *testing.T) {
	k, ctx, _ := setupExecutionTest(t)
	setupAgent(t, k, ctx, "agent-alpha", "technology", true, true)

	task := newSubmitTask("technology")
	taskID, err := k.CreateTask(ctx, task)
	require.NoError(t, err)

	err = k.ClaimTask(ctx, taskID, "agent-alpha")
	require.NoError(t, err)

	// Verify assignment.
	stored, found := k.GetAgentTask(ctx, taskID)
	require.True(t, found)
	require.Equal(t, types.TaskStatusAssigned, stored.Status)
	require.Equal(t, "agent-alpha", stored.AssignedTo)
	require.Equal(t, int64(100), stored.AssignedAt)
}

// ─── Test: Claim Task — Agent Not Found ─────────────────────────────────────

func TestClaimTask_AgentNotFound(t *testing.T) {
	k, ctx, _ := setupExecutionTest(t)

	task := newSubmitTask("technology")
	taskID, _ := k.CreateTask(ctx, task)

	err := k.ClaimTask(ctx, taskID, "nonexistent")
	require.ErrorIs(t, err, types.ErrAgentNotFound)
}

// ─── Test: Claim Task — Capability Check (Submit) ───────────────────────────

func TestClaimTask_CannotSubmit(t *testing.T) {
	k, ctx, _ := setupExecutionTest(t)
	setupAgent(t, k, ctx, "agent-nosubmit", "technology", false, true)

	task := newSubmitTask("technology")
	taskID, _ := k.CreateTask(ctx, task)

	err := k.ClaimTask(ctx, taskID, "agent-nosubmit")
	require.ErrorIs(t, err, types.ErrAgentCannotPerformTask)
}

// ─── Test: Claim Task — Capability Check (Review) ───────────────────────────

func TestClaimTask_CannotReview(t *testing.T) {
	k, ctx, _ := setupExecutionTest(t)
	setupAgent(t, k, ctx, "agent-noreview", "technology", true, false)

	task := newReviewTask("technology")
	taskID, _ := k.CreateTask(ctx, task)

	err := k.ClaimTask(ctx, taskID, "agent-noreview")
	require.ErrorIs(t, err, types.ErrAgentCannotPerformTask)
}

// ─── Test: Claim Task — Reputation Too Low ──────────────────────────────────

func TestClaimTask_ReputationTooLow(t *testing.T) {
	k, ctx, _ := setupExecutionTest(t)
	setupAgent(t, k, ctx, "agent-lowrep", "technology", true, true)

	// Set low reputation.
	agent, _ := k.GetAgentIdentity(ctx, "agent-lowrep")
	agent.Reputation = "0.100000000000000000"
	_ = k.SetAgentIdentity(ctx, &agent)

	task := newSubmitTask("technology")
	task.MinReputation = "0.500000000000000000"
	taskID, _ := k.CreateTask(ctx, task)

	err := k.ClaimTask(ctx, taskID, "agent-lowrep")
	require.ErrorIs(t, err, types.ErrAgentReputationTooLow)
}

// ─── Test: Claim Task — Already Assigned ────────────────────────────────────

func TestClaimTask_AlreadyAssigned(t *testing.T) {
	k, ctx, _ := setupExecutionTest(t)
	setupAgent(t, k, ctx, "agent-1", "technology", true, true)
	setupAgent(t, k, ctx, "agent-2", "technology", true, true)

	task := newSubmitTask("technology")
	taskID, _ := k.CreateTask(ctx, task)

	err := k.ClaimTask(ctx, taskID, "agent-1")
	require.NoError(t, err)

	err = k.ClaimTask(ctx, taskID, "agent-2")
	require.ErrorIs(t, err, types.ErrTaskNotAssignable)
}

// ─── Test: Claim Task — Expired ─────────────────────────────────────────────

func TestClaimTask_Expired(t *testing.T) {
	k, ctx, _ := setupExecutionTest(t)
	setupAgent(t, k, ctx, "agent-late", "technology", true, true)

	task := newSubmitTask("technology")
	task.ExpiresAt = 90 // before current block (100)
	taskID, _ := k.CreateTask(ctx, task)

	err := k.ClaimTask(ctx, taskID, "agent-late")
	require.ErrorIs(t, err, types.ErrTaskExpired)
}

// ─── Test: Claim Task — Max Concurrent Limit ────────────────────────────────

func TestClaimTask_MaxConcurrent(t *testing.T) {
	k, ctx, _ := setupExecutionTest(t)
	setupAgent(t, k, ctx, "agent-busy", "technology", true, true)

	// Set max to 2.
	params := keeper.DefaultSchedulerParams()
	params.MaxTasksPerAgent = 2
	require.NoError(t, k.SetSchedulerParams(ctx, params))

	// Create and claim 2 tasks.
	for i := 0; i < 2; i++ {
		task := newSubmitTask("technology")
		taskID, _ := k.CreateTask(ctx, task)
		require.NoError(t, k.ClaimTask(ctx, taskID, "agent-busy"))
	}

	// Third should fail.
	task := newSubmitTask("technology")
	taskID, _ := k.CreateTask(ctx, task)
	err := k.ClaimTask(ctx, taskID, "agent-busy")
	require.ErrorIs(t, err, types.ErrAgentCannotPerformTask)
}

// ─── Test: Complete Task — Happy Path ───────────────────────────────────────

func TestCompleteTask_HappyPath(t *testing.T) {
	k, ctx, bk := setupExecutionTest(t)
	setupAgent(t, k, ctx, "agent-worker", "technology", true, true)

	task := newSubmitTask("technology")
	taskID, _ := k.CreateTask(ctx, task)
	_ = k.ClaimTask(ctx, taskID, "agent-worker")

	result, err := k.CompleteTask(ctx, taskID, "agent-worker", "sample-xyz")
	require.NoError(t, err)
	require.Equal(t, taskID, result.TaskID)
	require.Equal(t, "agent-worker", result.AgentID)
	require.Equal(t, types.TaskStatusCompleted, result.Status)
	require.Equal(t, "sample-xyz", result.ResultID)

	// Verify task status.
	stored, _ := k.GetAgentTask(ctx, taskID)
	require.Equal(t, types.TaskStatusCompleted, stored.Status)
	require.Equal(t, "sample-xyz", stored.ResultID)

	// Verify reputation gained.
	agent, _ := k.GetAgentIdentity(ctx, "agent-worker")
	require.True(t, agent.GetReputation().GT(sdkmath.LegacyNewDecWithPrec(5, 1))) // > 0.5

	// Verify result stored.
	storedResult, found := k.GetAgentTaskResult(ctx, taskID)
	require.True(t, found)
	require.Equal(t, types.TaskStatusCompleted, storedResult.Status)

	// Verify bank transfer attempted (for reward).
	require.NotEmpty(t, bk.moduleToAccountCalls)
}

// ─── Test: Complete Task — Wrong Agent ──────────────────────────────────────

func TestCompleteTask_WrongAgent(t *testing.T) {
	k, ctx, _ := setupExecutionTest(t)
	setupAgent(t, k, ctx, "agent-1", "technology", true, true)
	setupAgent(t, k, ctx, "agent-2", "technology", true, true)

	task := newSubmitTask("technology")
	taskID, _ := k.CreateTask(ctx, task)
	_ = k.ClaimTask(ctx, taskID, "agent-1")

	_, err := k.CompleteTask(ctx, taskID, "agent-2", "result-xyz")
	require.ErrorIs(t, err, types.ErrAgentCannotPerformTask)
}

// ─── Test: Complete Task — Not Assigned ─────────────────────────────────────

func TestCompleteTask_NotAssigned(t *testing.T) {
	k, ctx, _ := setupExecutionTest(t)

	task := newSubmitTask("technology")
	taskID, _ := k.CreateTask(ctx, task)

	_, err := k.CompleteTask(ctx, taskID, "anyone", "result")
	require.ErrorIs(t, err, types.ErrTaskNotCompletable)
}

// ─── Test: Fail Task — Happy Path ───────────────────────────────────────────

func TestFailTask_HappyPath(t *testing.T) {
	k, ctx, _ := setupExecutionTest(t)
	setupAgent(t, k, ctx, "agent-fail", "technology", true, true)

	task := newSubmitTask("technology")
	taskID, _ := k.CreateTask(ctx, task)
	_ = k.ClaimTask(ctx, taskID, "agent-fail")

	err := k.FailTask(ctx, taskID, "agent-fail", "quality too low")
	require.NoError(t, err)

	// Verify status.
	stored, _ := k.GetAgentTask(ctx, taskID)
	require.Equal(t, types.TaskStatusFailed, stored.Status)
	require.Equal(t, "quality too low", stored.FailureReason)

	// Verify reputation lost (asymmetric: loss > gain).
	agent, _ := k.GetAgentIdentity(ctx, "agent-fail")
	require.True(t, agent.GetReputation().LT(sdkmath.LegacyNewDecWithPrec(5, 1))) // < 0.5

	// Verify result stored.
	result, found := k.GetAgentTaskResult(ctx, taskID)
	require.True(t, found)
	require.Equal(t, types.TaskStatusFailed, result.Status)
}

// ─── Test: Fail Task — Wrong Agent ──────────────────────────────────────────

func TestFailTask_WrongAgent(t *testing.T) {
	k, ctx, _ := setupExecutionTest(t)
	setupAgent(t, k, ctx, "agent-1", "technology", true, true)

	task := newSubmitTask("technology")
	taskID, _ := k.CreateTask(ctx, task)
	_ = k.ClaimTask(ctx, taskID, "agent-1")

	err := k.FailTask(ctx, taskID, "agent-intruder", "sabotage")
	require.ErrorIs(t, err, types.ErrAgentCannotPerformTask)
}

// ─── Test: Expire Tasks ─────────────────────────────────────────────────────

func TestExpireTasks_PendingExpired(t *testing.T) {
	k, ctx, _ := setupExecutionTest(t)

	task := newSubmitTask("technology")
	task.ExpiresAt = 50 // already past
	taskID, _ := k.CreateTask(ctx, task)

	expired := k.ExpireTasks(ctx)
	require.Equal(t, uint64(1), expired)

	stored, _ := k.GetAgentTask(ctx, taskID)
	require.Equal(t, types.TaskStatusExpired, stored.Status)
}

func TestExpireTasks_AssignedExpired(t *testing.T) {
	k, ctx, _ := setupExecutionTest(t)
	setupAgent(t, k, ctx, "agent-slow", "technology", true, true)

	// Create task that expires at block 150 (after current block 100).
	task := newSubmitTask("technology")
	task.ExpiresAt = 150
	taskID, _ := k.CreateTask(ctx, task)
	require.NoError(t, k.ClaimTask(ctx, taskID, "agent-slow"))

	// Advance block past expiry and run expire.
	ctx = sdk.UnwrapSDKContext(ctx).WithBlockHeight(200)
	expired := k.ExpireTasks(ctx)
	require.Equal(t, uint64(1), expired)

	stored, _ := k.GetAgentTask(ctx, taskID)
	require.Equal(t, types.TaskStatusFailed, stored.Status)
	require.Equal(t, "deadline exceeded", stored.FailureReason)
}

// ─── Test: Find Tasks for Agent ─────────────────────────────────────────────

func TestFindTasksForAgent(t *testing.T) {
	k, ctx, _ := setupExecutionTest(t)
	setupAgent(t, k, ctx, "agent-smart", "technology", true, true)

	// Create various tasks.
	k.CreateTask(ctx, newSubmitTask("technology"))
	k.CreateTask(ctx, newReviewTask("technology"))
	k.CreateTask(ctx, newSubmitTask("science"))

	matching := k.FindTasksForAgent(ctx, "agent-smart")
	require.Len(t, matching, 3) // agent can do all of them
}

func TestFindTasksForAgent_FilteredByCapability(t *testing.T) {
	k, ctx, _ := setupExecutionTest(t)
	setupAgent(t, k, ctx, "agent-submitonly", "technology", true, false) // can submit, can't review

	k.CreateTask(ctx, newSubmitTask("technology"))
	k.CreateTask(ctx, newReviewTask("technology"))

	matching := k.FindTasksForAgent(ctx, "agent-submitonly")
	require.Len(t, matching, 1)                              // only the submit task
	require.Equal(t, types.TaskTypeSubmit, matching[0].TaskType)
}

// ─── Test: Tasks by Agent ───────────────────────────────────────────────────

func TestGetTasksByAgent(t *testing.T) {
	k, ctx, _ := setupExecutionTest(t)
	setupAgent(t, k, ctx, "agent-multi", "technology", true, true)

	taskID1, _ := k.CreateTask(ctx, newSubmitTask("technology"))
	taskID2, _ := k.CreateTask(ctx, newReviewTask("technology"))
	_ = k.ClaimTask(ctx, taskID1, "agent-multi")
	_ = k.ClaimTask(ctx, taskID2, "agent-multi")

	tasks := k.GetTasksByAgent(ctx, "agent-multi")
	require.Len(t, tasks, 2)
}

// ─── Test: Task Stats ───────────────────────────────────────────────────────

func TestGetTaskStats(t *testing.T) {
	k, ctx, _ := setupExecutionTest(t)
	setupAgent(t, k, ctx, "agent-stats", "technology", true, true)

	// Create various states.
	t1, _ := k.CreateTask(ctx, newSubmitTask("technology")) // pending
	t2, _ := k.CreateTask(ctx, newSubmitTask("technology")) // will assign
	t3, _ := k.CreateTask(ctx, newSubmitTask("technology")) // will complete
	t4, _ := k.CreateTask(ctx, newSubmitTask("technology")) // will fail

	_ = k.ClaimTask(ctx, t2, "agent-stats")
	_ = k.ClaimTask(ctx, t3, "agent-stats")
	_ = k.ClaimTask(ctx, t4, "agent-stats")
	_, _ = k.CompleteTask(ctx, t3, "agent-stats", "result-3")
	_ = k.FailTask(ctx, t4, "agent-stats", "bad quality")

	pending, assigned, completed, failed, expired := k.GetTaskStats(ctx)
	_ = t1
	require.Equal(t, uint64(1), pending)
	require.Equal(t, uint64(1), assigned)
	require.Equal(t, uint64(1), completed)
	require.Equal(t, uint64(1), failed)
	require.Equal(t, uint64(0), expired)
}

// ─── Test: Scheduler Params ─────────────────────────────────────────────────

func TestSchedulerParams_DefaultsAndPersist(t *testing.T) {
	k, ctx, _ := setupExecutionTest(t)

	// Defaults.
	params := k.GetSchedulerParams(ctx)
	require.Equal(t, int64(50_000), params.DefaultTaskExpiry)
	require.Equal(t, uint64(5), params.MaxTasksPerAgent)

	// Custom.
	custom := keeper.SchedulerParams{
		DefaultTaskExpiry:    100_000,
		MaxTasksPerAgent:     10,
		ReputationGainRate:   "0.020000000000000000",
		ReputationLossRate:   "0.040000000000000000",
		TaskCooldownBlocks:   200,
		AutoCreateFromBounty: false,
	}
	require.NoError(t, k.SetSchedulerParams(ctx, custom))

	loaded := k.GetSchedulerParams(ctx)
	require.Equal(t, int64(100_000), loaded.DefaultTaskExpiry)
	require.Equal(t, uint64(10), loaded.MaxTasksPerAgent)
	require.False(t, loaded.AutoCreateFromBounty)
}

// ─── Test: Bounty Task Auto-Generation ──────────────────────────────────────

func TestGenerateTasksFromBounties(t *testing.T) {
	k, ctx, _ := setupExecutionTest(t)

	// Create a bounty first.
	resp, err := k.FundBounty(ctx, &types.MsgFundBounty{
		Funder:        testAddr,
		Domain:        "technology",
		Topic:         "golang",
		Amount:        "5000000",
		ExpiresBlocks: 100_000,
	})
	require.NoError(t, err)

	// Initialize competitive bounty (needed for GetOpenBounties).
	_ = k.SetCompetitiveBounty(ctx, &keeper.CompetitiveBounty{
		BountyID:  resp.BountyId,
		Status:    keeper.BountyStatusOpen,
		TotalPool: "5000000",
	})

	created := k.GenerateTasksFromBounties(ctx)
	require.Equal(t, uint64(1), created)

	// Verify task was created.
	tasks := k.GetTasksByDomain(ctx, "technology")
	var bountyTask *types.AgentTask
	for _, t := range tasks {
		if t.BountyID == resp.BountyId {
			bountyTask = t
			break
		}
	}
	require.NotNil(t, bountyTask)
	require.Equal(t, types.TaskTypeBountyEntry, bountyTask.TaskType)
	require.Equal(t, "5000000", bountyTask.RewardPool)
}

func TestGenerateTasksFromBounties_NoDuplicate(t *testing.T) {
	k, ctx, _ := setupExecutionTest(t)

	resp, _ := k.FundBounty(ctx, &types.MsgFundBounty{
		Funder: testAddr, Domain: "technology", Topic: "go", Amount: "5000000", ExpiresBlocks: 100_000,
	})
	_ = k.SetCompetitiveBounty(ctx, &keeper.CompetitiveBounty{
		BountyID: resp.BountyId, Status: keeper.BountyStatusOpen, TotalPool: "5000000",
	})

	// Generate once.
	created1 := k.GenerateTasksFromBounties(ctx)
	require.Equal(t, uint64(1), created1)

	// Generate again — should not create duplicate.
	created2 := k.GenerateTasksFromBounties(ctx)
	require.Equal(t, uint64(0), created2)
}

// ─── Test: Full Agent Execution Lifecycle ───────────────────────────────────

func TestAgentExecutionLifecycle(t *testing.T) {
	k, ctx, bk := setupExecutionTest(t)
	setupAgent(t, k, ctx, "agent-auto", "technology", true, true)

	// 1. Create a bounty → auto-generate task.
	resp, _ := k.FundBounty(ctx, &types.MsgFundBounty{
		Funder: testAddr, Domain: "technology", Topic: "concurrency",
		Amount: "10000000", ExpiresBlocks: 100_000,
	})
	_ = k.SetCompetitiveBounty(ctx, &keeper.CompetitiveBounty{
		BountyID: resp.BountyId, Status: keeper.BountyStatusOpen, TotalPool: "10000000",
	})

	created := k.GenerateTasksFromBounties(ctx)
	require.Equal(t, uint64(1), created)

	// 2. Agent finds matching tasks.
	matching := k.FindTasksForAgent(ctx, "agent-auto")
	require.NotEmpty(t, matching)
	taskID := matching[0].TaskID

	// 3. Agent claims the task.
	err := k.ClaimTask(ctx, taskID, "agent-auto")
	require.NoError(t, err)

	task, _ := k.GetAgentTask(ctx, taskID)
	require.Equal(t, types.TaskStatusAssigned, task.Status)

	// 4. Agent completes the task (submits data to bounty).
	result, err := k.CompleteTask(ctx, taskID, "agent-auto", "sample-concurrency-101")
	require.NoError(t, err)
	require.Equal(t, "sample-concurrency-101", result.ResultID)

	// 5. Verify: task completed, reputation up, earnings recorded.
	task, _ = k.GetAgentTask(ctx, taskID)
	require.Equal(t, types.TaskStatusCompleted, task.Status)

	agent, _ := k.GetAgentIdentity(ctx, "agent-auto")
	require.True(t, agent.GetReputation().GT(sdkmath.LegacyNewDecWithPrec(5, 1))) // > 0.5
	require.Equal(t, uint64(1), agent.TasksComplete)

	// 6. Verify stats.
	_, _, completed, _, _ := k.GetTaskStats(ctx)
	require.Equal(t, uint64(1), completed)

	// Bank was called for reward transfer.
	require.NotEmpty(t, bk.moduleToAccountCalls)
}
