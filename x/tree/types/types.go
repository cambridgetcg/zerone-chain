package types

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// --- Enums ---

type ProjectPhase string

const (
	PhaseSeed     ProjectPhase = "seed"
	PhaseSprout   ProjectPhase = "sprout"
	PhaseGrowing  ProjectPhase = "growing"
	PhaseMature   ProjectPhase = "mature"
	PhaseFruiting ProjectPhase = "fruiting"
	PhaseSeeding  ProjectPhase = "seeding"
	PhaseDormant  ProjectPhase = "dormant"
	PhaseWithered ProjectPhase = "withered"
)

type TaskStatus string

const (
	TaskOpen       TaskStatus = "open"
	TaskAssigned   TaskStatus = "assigned"
	TaskInProgress TaskStatus = "in_progress"
	TaskReview     TaskStatus = "review"
	TaskCompleted  TaskStatus = "completed"
	TaskRejected   TaskStatus = "rejected"
	TaskDisputed   TaskStatus = "disputed"
)

type TaskType string

const (
	TaskDesign     TaskType = "design"
	TaskImplement  TaskType = "implement"
	TaskTest       TaskType = "test"
	TaskReviewType TaskType = "review"
	TaskDeploy     TaskType = "deploy"
	TaskDocument   TaskType = "document"
	TaskVerify     TaskType = "verify"
	TaskResearch   TaskType = "research"
	TaskGovernance TaskType = "governance"
	TaskMaintain   TaskType = "maintain"
)

type ServiceStatus string

const (
	ServiceDeploying ServiceStatus = "deploying"
	ServiceActive    ServiceStatus = "active"
	ServiceDegraded  ServiceStatus = "degraded"
	ServicePaused    ServiceStatus = "paused"
	ServiceRetired   ServiceStatus = "retired"
)

type ServiceType string

const (
	ServiceAPI          ServiceType = "api"
	ServiceDataFeed     ServiceType = "data_feed"
	ServiceComputation  ServiceType = "computation"
	ServiceVerification ServiceType = "verification"
	ServiceAggregation  ServiceType = "aggregation"
	ServiceAnalytics    ServiceType = "analytics"
)

type SeedStatus string

const (
	SeedDetected SeedStatus = "detected"
	SeedProposed SeedStatus = "proposed"
	SeedClaimed  SeedStatus = "claimed"
	SeedExpired  SeedStatus = "expired"
)

type ContributorRole string

const (
	RoleFounder    ContributorRole = "founder"
	RoleArchitect  ContributorRole = "architect"
	RoleDeveloper  ContributorRole = "developer"
	RoleReviewer   ContributorRole = "reviewer"
	RoleTester     ContributorRole = "tester"
	RoleMaintainer ContributorRole = "maintainer"
)

type ApplicationStatus string

const (
	AppPending  ApplicationStatus = "pending"
	AppApproved ApplicationStatus = "approved"
	AppRejected ApplicationStatus = "rejected"
)

type FundingSourceType string

const (
	FundingResearch  FundingSourceType = "research_fund"
	FundingBounty    FundingSourceType = "bounty_pool"
	FundingAgent     FundingSourceType = "agent_funded"
	FundingCommunity FundingSourceType = "community"
)

// --- Constants ---

const (
	// AbandonTimelockBlocks is the minimum waiting period before an abandon proposal
	// can be executed. ~1 day at 2521ms block time.
	AbandonTimelockBlocks = uint64(34272)

	// DisputeSlashBps is the fraction of bounty burned when a task enters disputed status.
	DisputeSlashBps = int64(300000)

	// BpsDenominator is the basis-point denominator (1,000,000 = 100%).
	BpsDenominator = int64(1000000)
)

// TaskResolved is the status for a task whose dispute has been resolved.
const TaskResolved TaskStatus = "resolved"

// ProjectTypeDataCollection identifies tree projects that are data collection campaigns.
const ProjectTypeDataCollection = "data_collection"

// --- JSON-serialized types (no proto) ---

// PendingAbandon tracks a proposed project abandonment with timelock.
type PendingAbandon struct {
	ProjectId       string   `json:"project_id"`
	ProposedBy      string   `json:"proposed_by"`
	ProposedAtBlock uint64   `json:"proposed_at_block"`
	Consented       []string `json:"consented"`
}

// ServiceSubscription represents an active subscription to a service.
type ServiceSubscription struct {
	Id             string `json:"id"`
	ServiceId      string `json:"service_id"`
	Subscriber     string `json:"subscriber"`
	StartBlock     uint64 `json:"start_block"`
	DurationBlocks uint64 `json:"duration_blocks"`
	ExpiresAtBlock uint64 `json:"expires_at_block"`
}

// AgentAvailability stores an agent's availability for project work.
type AgentAvailability struct {
	Agent            string   `json:"agent"`
	Available        bool     `json:"available"`
	Capabilities     []string `json:"capabilities,omitempty"`
	PreferredDomains []string `json:"preferred_domains,omitempty"`
	MinimumBounty    string   `json:"minimum_bounty,omitempty"`
	UpdatedAtBlock   int64    `json:"updated_at_block"`
}

// --- Params Validation ---

func (p *Params) Validate() error {
	if p.MaxTasksPerProject == 0 {
		return fmt.Errorf("max tasks per project must be positive")
	}
	if p.MaxRejections == 0 {
		return fmt.Errorf("max rejections must be positive")
	}
	bpSum := p.ContributorsBp + p.ProtocolTreasuryBp + p.ResearchFundBp + p.BurnBp
	if bpSum != 0 && bpSum != 1000000 {
		return fmt.Errorf("revenue basis points must sum to 1000000, got %d", bpSum)
	}
	if p.EvidenceTaxBp > 1000000 {
		return fmt.Errorf("evidence_tax_bp must be <= 1000000, got %d", p.EvidenceTaxBp)
	}
	return nil
}

// --- ValidateBasic methods for proto-generated Msg types ---

func (m *MsgCreateProject) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Creator); err != nil {
		return fmt.Errorf("invalid creator address: %w", err)
	}
	if m.Title == "" {
		return fmt.Errorf("title cannot be empty")
	}
	return nil
}

func (m *MsgProposeProject) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Proposer); err != nil {
		return fmt.Errorf("invalid proposer address: %w", err)
	}
	if m.ProjectId == "" {
		return fmt.Errorf("project id cannot be empty")
	}
	return nil
}

func (m *MsgStartDevelopment) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Authority); err != nil {
		return fmt.Errorf("invalid authority: %w", err)
	}
	if m.ProjectId == "" {
		return fmt.Errorf("project id cannot be empty")
	}
	return nil
}

func (m *MsgCompleteProject) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Authority); err != nil {
		return fmt.Errorf("invalid authority: %w", err)
	}
	if m.ProjectId == "" {
		return fmt.Errorf("project id cannot be empty")
	}
	return nil
}

func (m *MsgPauseProject) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Authority); err != nil {
		return fmt.Errorf("invalid authority: %w", err)
	}
	if m.ProjectId == "" {
		return fmt.Errorf("project id cannot be empty")
	}
	return nil
}

func (m *MsgResumeProject) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Authority); err != nil {
		return fmt.Errorf("invalid authority: %w", err)
	}
	if m.ProjectId == "" {
		return fmt.Errorf("project id cannot be empty")
	}
	return nil
}

func (m *MsgAbandonProject) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Authority); err != nil {
		return fmt.Errorf("invalid authority: %w", err)
	}
	if m.ProjectId == "" {
		return fmt.Errorf("project id cannot be empty")
	}
	return nil
}

func (m *MsgSpawnChildProject) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Creator); err != nil {
		return fmt.Errorf("invalid creator: %w", err)
	}
	if m.ParentProjectId == "" {
		return fmt.Errorf("parent project id cannot be empty")
	}
	if m.Title == "" {
		return fmt.Errorf("title cannot be empty")
	}
	return nil
}

func (m *MsgAddTask) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Creator); err != nil {
		return fmt.Errorf("invalid creator: %w", err)
	}
	if m.ProjectId == "" {
		return fmt.Errorf("project id cannot be empty")
	}
	if m.Title == "" {
		return fmt.Errorf("title cannot be empty")
	}
	return nil
}

func (m *MsgAssignTask) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Assigner); err != nil {
		return fmt.Errorf("invalid assigner: %w", err)
	}
	if m.TaskId == "" {
		return fmt.Errorf("task id cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(m.Assignee); err != nil {
		return fmt.Errorf("invalid assignee: %w", err)
	}
	return nil
}

func (m *MsgStartWork) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Worker); err != nil {
		return fmt.Errorf("invalid worker: %w", err)
	}
	if m.TaskId == "" {
		return fmt.Errorf("task id cannot be empty")
	}
	return nil
}

func (m *MsgSubmitDeliverable) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Worker); err != nil {
		return fmt.Errorf("invalid worker: %w", err)
	}
	if m.TaskId == "" {
		return fmt.Errorf("task id cannot be empty")
	}
	return nil
}

func (m *MsgApproveDeliverable) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Approver); err != nil {
		return fmt.Errorf("invalid approver: %w", err)
	}
	if m.TaskId == "" {
		return fmt.Errorf("task id cannot be empty")
	}
	return nil
}

func (m *MsgRejectDeliverable) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Rejector); err != nil {
		return fmt.Errorf("invalid rejector: %w", err)
	}
	if m.TaskId == "" {
		return fmt.Errorf("task id cannot be empty")
	}
	return nil
}

func (m *MsgReopenTask) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Authority); err != nil {
		return fmt.Errorf("invalid authority: %w", err)
	}
	if m.TaskId == "" {
		return fmt.Errorf("task id cannot be empty")
	}
	return nil
}

func (m *MsgApplyToProject) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Applicant); err != nil {
		return fmt.Errorf("invalid applicant: %w", err)
	}
	if m.ProjectId == "" {
		return fmt.Errorf("project id cannot be empty")
	}
	return nil
}

func (m *MsgReviewApplication) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Reviewer); err != nil {
		return fmt.Errorf("invalid reviewer: %w", err)
	}
	if m.ApplicationId == "" {
		return fmt.Errorf("application id cannot be empty")
	}
	return nil
}

func (m *MsgSetAvailability) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Agent); err != nil {
		return fmt.Errorf("invalid agent: %w", err)
	}
	return nil
}

func (m *MsgDeployService) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Deployer); err != nil {
		return fmt.Errorf("invalid deployer: %w", err)
	}
	if m.Name == "" {
		return fmt.Errorf("name cannot be empty")
	}
	return nil
}

func (m *MsgPauseService) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Owner); err != nil {
		return fmt.Errorf("invalid owner: %w", err)
	}
	if m.ServiceId == "" {
		return fmt.Errorf("service id cannot be empty")
	}
	return nil
}

func (m *MsgResumeService) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Owner); err != nil {
		return fmt.Errorf("invalid owner: %w", err)
	}
	if m.ServiceId == "" {
		return fmt.Errorf("service id cannot be empty")
	}
	return nil
}

func (m *MsgRetireService) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Owner); err != nil {
		return fmt.Errorf("invalid owner: %w", err)
	}
	if m.ServiceId == "" {
		return fmt.Errorf("service id cannot be empty")
	}
	return nil
}

func (m *MsgCallService) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Caller); err != nil {
		return fmt.Errorf("invalid caller address: %w", err)
	}
	if m.ServiceId == "" {
		return fmt.Errorf("service id cannot be empty")
	}
	return nil
}

func (m *MsgSubscribeService) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Subscriber); err != nil {
		return fmt.Errorf("invalid subscriber address: %w", err)
	}
	if m.ServiceId == "" {
		return fmt.Errorf("service id cannot be empty")
	}
	if m.DurationBlocks == 0 {
		return fmt.Errorf("duration must be positive")
	}
	return nil
}

func (m *MsgBeginSeeding) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Seeder); err != nil {
		return fmt.Errorf("invalid seeder address: %w", err)
	}
	if m.ProjectId == "" {
		return fmt.Errorf("project id cannot be empty")
	}
	return nil
}

func (m *MsgDetectOpportunity) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Detector); err != nil {
		return fmt.Errorf("invalid detector address: %w", err)
	}
	if m.Domain == "" {
		return fmt.Errorf("domain cannot be empty")
	}
	if m.Description == "" {
		return fmt.Errorf("description cannot be empty")
	}
	return nil
}

func (m *MsgClaimOpportunity) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Claimer); err != nil {
		return fmt.Errorf("invalid claimer address: %w", err)
	}
	if m.OpportunityId == "" {
		return fmt.Errorf("opportunity id cannot be empty")
	}
	return nil
}

func (m *MsgAddContributor) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Authority); err != nil {
		return fmt.Errorf("invalid authority address: %w", err)
	}
	if m.ProjectId == "" {
		return fmt.Errorf("project id cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(m.Contributor); err != nil {
		return fmt.Errorf("invalid contributor address: %w", err)
	}
	if m.Role == "" {
		return fmt.Errorf("role cannot be empty")
	}
	return nil
}

func (m *MsgUpdateParams) ValidateBasic() error {
	if m.Authority == "" {
		return fmt.Errorf("authority cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(m.Authority); err != nil {
		return fmt.Errorf("invalid authority address: %w", err)
	}
	if m.Params == nil {
		return fmt.Errorf("params cannot be nil")
	}
	return m.Params.Validate()
}
