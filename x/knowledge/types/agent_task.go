package types

import (
	sdkmath "cosmossdk.io/math"
)

// ─── Agent Task Types ───────────────────────────────────────────────────────

// TaskType defines the kind of autonomous action an agent can perform.
type TaskType string

const (
	TaskTypeSubmit       TaskType = "submit"        // submit TDU to a domain
	TaskTypeReview       TaskType = "review"         // review a pending submission
	TaskTypeBountyEntry  TaskType = "bounty_entry"   // submit to a competitive bounty
	TaskTypeEdge         TaskType = "create_edge"    // create knowledge graph edge
)

// ValidTaskTypes defines which task types exist.
var ValidTaskTypes = map[TaskType]bool{
	TaskTypeSubmit:      true,
	TaskTypeReview:      true,
	TaskTypeBountyEntry: true,
	TaskTypeEdge:        true,
}

// TaskStatus tracks the lifecycle of an agent task.
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"   // queued, not yet executed
	TaskStatusAssigned  TaskStatus = "assigned"  // claimed by an agent
	TaskStatusCompleted TaskStatus = "completed" // successfully executed
	TaskStatusFailed    TaskStatus = "failed"    // execution failed
	TaskStatusExpired   TaskStatus = "expired"   // deadline passed without execution
)

// TaskPriority defines urgency levels.
type TaskPriority uint64

const (
	TaskPriorityLow    TaskPriority = 1
	TaskPriorityNormal TaskPriority = 5
	TaskPriorityHigh   TaskPriority = 10
	TaskPriorityCritical TaskPriority = 20
)

// ─── AgentTask ──────────────────────────────────────────────────────────────

// AgentTask represents a unit of work that an autonomous agent can perform.
// Tasks are created by the protocol (e.g., bounties needing entries, domains
// needing reviews) or by other agents (delegation).
type AgentTask struct {
	TaskID      string       `json:"task_id"`
	TaskType    TaskType     `json:"task_type"`
	Domain      string       `json:"domain"`
	Status      TaskStatus   `json:"status"`
	Priority    TaskPriority `json:"priority"`

	// What needs to be done.
	Description string            `json:"description,omitempty"`
	Params      map[string]string `json:"params,omitempty"` // task-specific parameters

	// Bounty connection (if task is bounty-driven).
	BountyID string `json:"bounty_id,omitempty"`

	// Assignment.
	AssignedTo    string `json:"assigned_to,omitempty"`     // agentID
	AssignedAt    int64  `json:"assigned_at,omitempty"`     // block height
	CompletedAt   int64  `json:"completed_at,omitempty"`
	ResultID      string `json:"result_id,omitempty"`       // sampleID, edgeID, etc.
	FailureReason string `json:"failure_reason,omitempty"`

	// Lifecycle.
	CreatedAt  int64  `json:"created_at"`  // block height
	ExpiresAt  int64  `json:"expires_at"`  // block height deadline
	CreatedBy  string `json:"created_by"`  // protocol, agentID, or governance
	RewardPool string `json:"reward_pool"` // available uzrn for completing this task

	// Capability requirements.
	MinReputation string `json:"min_reputation,omitempty"` // minimum agent reputation
	MaxGeneration uint64 `json:"max_generation,omitempty"` // 0 = any generation
}

// GetRewardPool parses the reward amount.
func (t *AgentTask) GetRewardPool() sdkmath.Int {
	if t.RewardPool == "" {
		return sdkmath.ZeroInt()
	}
	i, ok := sdkmath.NewIntFromString(t.RewardPool)
	if !ok {
		return sdkmath.ZeroInt()
	}
	return i
}

// GetMinReputation parses the minimum reputation requirement.
func (t *AgentTask) GetMinReputation() sdkmath.LegacyDec {
	if t.MinReputation == "" {
		return sdkmath.LegacyZeroDec()
	}
	d, err := sdkmath.LegacyNewDecFromStr(t.MinReputation)
	if err != nil {
		return sdkmath.LegacyZeroDec()
	}
	return d
}

// ─── AgentTaskResult ────────────────────────────────────────────────────────

// AgentTaskResult captures the outcome of an executed task.
type AgentTaskResult struct {
	TaskID      string     `json:"task_id"`
	AgentID     string     `json:"agent_id"`
	Status      TaskStatus `json:"status"`
	ResultID    string     `json:"result_id,omitempty"` // artifact created
	RewardPaid  string     `json:"reward_paid,omitempty"`
	RepChange   string     `json:"rep_change,omitempty"` // reputation delta
	CompletedAt int64      `json:"completed_at"`
}

// ─── Events ─────────────────────────────────────────────────────────────────

const (
	EventTaskCreated   = "agent_task_created"
	EventTaskAssigned  = "agent_task_assigned"
	EventTaskCompleted = "agent_task_completed"
	EventTaskFailed    = "agent_task_failed"
	EventTaskExpired   = "agent_task_expired"

	AttributeTaskID   = "task_id"
	AttributeTaskType = "task_type"
)
