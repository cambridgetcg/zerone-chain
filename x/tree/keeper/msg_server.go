package keeper

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/tree/types"
)

type msgServer struct {
	Keeper
	types.UnimplementedMsgServer
}

func NewMsgServerImpl(keeper Keeper) types.MsgServer {
	return &msgServer{Keeper: keeper}
}

var _ types.MsgServer = msgServer{}

// ======================== Project Messages ========================

func (m msgServer) CreateProject(goCtx context.Context, msg *types.MsgCreateProject) (*types.MsgCreateProjectResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	currentBlock := uint64(ctx.BlockHeight())

	counter := m.Keeper.GetNextProjectId(ctx)
	projectId := fmt.Sprintf("proj-%d-%d", currentBlock, counter)

	project := &types.ProductProject{
		Id:                  projectId,
		Name:                msg.Title,
		Description:         msg.Description,
		Phase:               string(types.PhaseSeed),
		CreatedAtBlock:      currentBlock,
		PhaseChangedAtBlock: currentBlock,
		KnowledgeDomain:     msg.Domain,
		Founder:             msg.Creator,
		Contributors: []*types.ContributorRecord{
			{
				Did:           msg.Creator,
				Role:          string(types.RoleFounder),
				JoinedAtBlock: currentBlock,
			},
		},
		Budget:     msg.Budget,
		Spent:      "0",
		TaskIds:    []string{},
		ServiceIds: []string{},
	}

	m.Keeper.SetProject(ctx, project)

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.tree.project_created",
		sdk.NewAttribute("project_id", projectId),
		sdk.NewAttribute("founder", msg.Creator),
		sdk.NewAttribute("domain", msg.Domain),
	))

	return &types.MsgCreateProjectResponse{ProjectId: projectId}, nil
}

func (m msgServer) ProposeProject(goCtx context.Context, msg *types.MsgProposeProject) (*types.MsgProposeProjectResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	project, found := m.Keeper.GetProject(ctx, msg.ProjectId)
	if !found {
		return nil, types.ErrProjectNotFound
	}
	if project.Founder != msg.Proposer {
		return nil, types.ErrNotFounder
	}
	if !IsValidPhaseTransition(types.ProjectPhase(project.Phase), types.PhaseSprout) {
		return nil, types.ErrInvalidPhaseTransition
	}

	project.Phase = string(types.PhaseSprout)
	project.PreviousPhase = string(types.PhaseSeed)
	project.PhaseChangedAtBlock = uint64(ctx.BlockHeight())
	m.Keeper.SetProject(ctx, project)

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.tree.project_proposed",
		sdk.NewAttribute("project_id", msg.ProjectId),
	))

	return &types.MsgProposeProjectResponse{}, nil
}

func (m msgServer) StartDevelopment(goCtx context.Context, msg *types.MsgStartDevelopment) (*types.MsgStartDevelopmentResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	project, found := m.Keeper.GetProject(ctx, msg.ProjectId)
	if !found {
		return nil, types.ErrProjectNotFound
	}
	if project.Founder != msg.Authority {
		return nil, types.ErrNotFounder
	}
	if !IsValidPhaseTransition(types.ProjectPhase(project.Phase), types.PhaseGrowing) {
		return nil, types.ErrInvalidPhaseTransition
	}

	params := m.Keeper.GetParams(ctx)
	if uint32(len(project.Contributors)) < params.MinContributorsToStart {
		return nil, fmt.Errorf("need at least %d contributors to start development", params.MinContributorsToStart)
	}

	project.Phase = string(types.PhaseGrowing)
	project.PreviousPhase = string(types.PhaseSprout)
	project.PhaseChangedAtBlock = uint64(ctx.BlockHeight())
	m.Keeper.SetProject(ctx, project)

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.tree.development_started",
		sdk.NewAttribute("project_id", msg.ProjectId),
	))

	return &types.MsgStartDevelopmentResponse{}, nil
}

func (m msgServer) CompleteProject(goCtx context.Context, msg *types.MsgCompleteProject) (*types.MsgCompleteProjectResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	project, found := m.Keeper.GetProject(ctx, msg.ProjectId)
	if !found {
		return nil, types.ErrProjectNotFound
	}
	if project.Founder != msg.Authority {
		return nil, types.ErrNotFounder
	}
	if !IsValidPhaseTransition(types.ProjectPhase(project.Phase), types.PhaseMature) {
		return nil, types.ErrInvalidPhaseTransition
	}

	project.PreviousPhase = project.Phase
	project.Phase = string(types.PhaseMature)
	project.PhaseChangedAtBlock = uint64(ctx.BlockHeight())
	m.Keeper.SetProject(ctx, project)

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.tree.project_completed",
		sdk.NewAttribute("project_id", msg.ProjectId),
	))

	return &types.MsgCompleteProjectResponse{}, nil
}

func (m msgServer) PauseProject(goCtx context.Context, msg *types.MsgPauseProject) (*types.MsgPauseProjectResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	project, found := m.Keeper.GetProject(ctx, msg.ProjectId)
	if !found {
		return nil, types.ErrProjectNotFound
	}
	if project.Founder != msg.Authority {
		return nil, types.ErrNotFounder
	}
	if !IsValidPhaseTransition(types.ProjectPhase(project.Phase), types.PhaseDormant) {
		return nil, types.ErrInvalidPhaseTransition
	}

	project.PreviousPhase = project.Phase
	project.Phase = string(types.PhaseDormant)
	project.PauseReason = msg.Reason
	project.PhaseChangedAtBlock = uint64(ctx.BlockHeight())
	m.Keeper.SetProject(ctx, project)

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.tree.project_paused",
		sdk.NewAttribute("project_id", msg.ProjectId),
		sdk.NewAttribute("reason", msg.Reason),
	))

	return &types.MsgPauseProjectResponse{}, nil
}

func (m msgServer) ResumeProject(goCtx context.Context, msg *types.MsgResumeProject) (*types.MsgResumeProjectResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	project, found := m.Keeper.GetProject(ctx, msg.ProjectId)
	if !found {
		return nil, types.ErrProjectNotFound
	}
	if project.Founder != msg.Authority {
		return nil, types.ErrNotFounder
	}
	if project.Phase != string(types.PhaseDormant) {
		return nil, types.ErrInvalidPhaseTransition
	}

	resumeTo := project.PreviousPhase
	if resumeTo == "" || resumeTo == string(types.PhaseDormant) {
		resumeTo = string(types.PhaseGrowing)
	}
	if !IsValidPhaseTransition(types.ProjectPhase(project.Phase), types.ProjectPhase(resumeTo)) {
		return nil, types.ErrInvalidPhaseTransition
	}

	project.PreviousPhase = string(types.PhaseDormant)
	project.Phase = resumeTo
	project.PauseReason = ""
	project.PhaseChangedAtBlock = uint64(ctx.BlockHeight())
	m.Keeper.SetProject(ctx, project)

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.tree.project_resumed",
		sdk.NewAttribute("project_id", msg.ProjectId),
		sdk.NewAttribute("resumed_to", string(resumeTo)),
	))

	return &types.MsgResumeProjectResponse{}, nil
}

func (m msgServer) AbandonProject(goCtx context.Context, msg *types.MsgAbandonProject) (*types.MsgAbandonProjectResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	project, found := m.Keeper.GetProject(ctx, msg.ProjectId)
	if !found {
		return nil, types.ErrProjectNotFound
	}
	if !IsFounderOrContributor(project, msg.Authority) {
		return nil, types.ErrNotContributor
	}
	if !IsValidPhaseTransition(types.ProjectPhase(project.Phase), types.PhaseWithered) {
		return nil, types.ErrInvalidPhaseTransition
	}

	currentBlock := uint64(ctx.BlockHeight())

	pa, hasProposal := m.Keeper.GetPendingAbandon(ctx, msg.ProjectId)

	if !hasProposal {
		pa = types.PendingAbandon{
			ProjectId:       msg.ProjectId,
			ProposedBy:      msg.Authority,
			ProposedAtBlock: currentBlock,
			Consented:       []string{msg.Authority},
		}
		m.Keeper.SetPendingAbandon(ctx, pa)

		ctx.EventManager().EmitEvent(sdk.NewEvent(
			"zerone.tree.abandon_proposed",
			sdk.NewAttribute("project_id", msg.ProjectId),
			sdk.NewAttribute("proposed_by", msg.Authority),
			sdk.NewAttribute("expires_at_block", fmt.Sprintf("%d", currentBlock+types.AbandonTimelockBlocks)),
		))

		return &types.MsgAbandonProjectResponse{}, nil
	}

	if msg.Authority != pa.ProposedBy {
		alreadyConsented := false
		for _, addr := range pa.Consented {
			if addr == msg.Authority {
				alreadyConsented = true
				break
			}
		}
		if !alreadyConsented {
			pa.Consented = append(pa.Consented, msg.Authority)
			m.Keeper.SetPendingAbandon(ctx, pa)
		}
	}

	timelockExpired := currentBlock >= pa.ProposedAtBlock+types.AbandonTimelockBlocks
	hasMajority := ContributorMajorityConsented(project, pa)

	if !timelockExpired && !hasMajority {
		return nil, types.ErrAbandonTimelockActive.Wrapf(
			"timelock expires at block %d, or get contributor majority consent",
			pa.ProposedAtBlock+types.AbandonTimelockBlocks)
	}

	project.PreviousPhase = project.Phase
	project.Phase = string(types.PhaseWithered)
	project.PhaseChangedAtBlock = currentBlock
	m.Keeper.SetProject(ctx, project)
	m.Keeper.DeletePendingAbandon(ctx, msg.ProjectId)

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.tree.project_abandoned",
		sdk.NewAttribute("project_id", msg.ProjectId),
		sdk.NewAttribute("executed_by", msg.Authority),
		sdk.NewAttribute("consent_count", fmt.Sprintf("%d", len(pa.Consented))),
	))

	return &types.MsgAbandonProjectResponse{}, nil
}

func (m msgServer) SpawnChildProject(goCtx context.Context, msg *types.MsgSpawnChildProject) (*types.MsgSpawnChildProjectResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	parent, found := m.Keeper.GetProject(ctx, msg.ParentProjectId)
	if !found {
		return nil, types.ErrProjectNotFound
	}
	if !IsFounderOrContributor(parent, msg.Creator) {
		return nil, types.ErrNotContributor
	}
	if parent.Phase != string(types.PhaseFruiting) && parent.Phase != string(types.PhaseSeeding) {
		return nil, types.ErrInvalidPhaseTransition
	}

	currentBlock := uint64(ctx.BlockHeight())
	counter := m.Keeper.GetNextProjectId(ctx)
	childId := fmt.Sprintf("proj-%d-%d", currentBlock, counter)

	child := &types.ProductProject{
		Id:                  childId,
		Name:                msg.Title,
		Description:         msg.Description,
		Phase:               string(types.PhaseSeed),
		CreatedAtBlock:      currentBlock,
		PhaseChangedAtBlock: currentBlock,
		KnowledgeDomain:     parent.KnowledgeDomain,
		ParentProjectId:     msg.ParentProjectId,
		Founder:             msg.Creator,
		Contributors: []*types.ContributorRecord{
			{
				Did:           msg.Creator,
				Role:          string(types.RoleFounder),
				JoinedAtBlock: currentBlock,
			},
		},
		Budget:     msg.Budget,
		Spent:      "0",
		TaskIds:    []string{},
		ServiceIds: []string{},
	}

	m.Keeper.SetProject(ctx, child)

	if parent.Phase == string(types.PhaseFruiting) {
		parent.PreviousPhase = string(types.PhaseFruiting)
		parent.Phase = string(types.PhaseSeeding)
		parent.PhaseChangedAtBlock = currentBlock
		m.Keeper.SetProject(ctx, parent)
	}

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.tree.child_project_spawned",
		sdk.NewAttribute("child_id", childId),
		sdk.NewAttribute("parent_id", msg.ParentProjectId),
	))

	return &types.MsgSpawnChildProjectResponse{ChildProjectId: childId}, nil
}

// ======================== Task Messages ========================

func (m msgServer) AddTask(goCtx context.Context, msg *types.MsgAddTask) (*types.MsgAddTaskResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	project, found := m.Keeper.GetProject(ctx, msg.ProjectId)
	if !found {
		return nil, types.ErrProjectNotFound
	}
	if !IsFounderOrContributor(project, msg.Creator) {
		return nil, types.ErrNotContributor
	}

	params := m.Keeper.GetParams(ctx)
	if project.TotalTasks >= params.MaxTasksPerProject {
		return nil, types.ErrMaxTasksReached
	}

	currentBlock := uint64(ctx.BlockHeight())

	bountyStr := msg.Bounty
	if bountyStr != "" && bountyStr != "0" {
		bountyAmt, ok := math.NewIntFromString(bountyStr)
		if ok && bountyAmt.IsPositive() {
			budget, _ := strconv.ParseInt(project.Budget, 10, 64)
			spent, _ := strconv.ParseInt(project.Spent, 10, 64)
			if budget > 0 && bountyAmt.Int64()+spent > budget {
				return nil, types.ErrBountyExceedsBudget.Wrapf(
					"bounty %s + spent %d exceeds budget %d", bountyStr, spent, budget)
			}

			creatorAddr, err := sdk.AccAddressFromBech32(msg.Creator)
			if err != nil {
				return nil, fmt.Errorf("invalid creator address: %w", err)
			}
			coins := sdk.NewCoins(sdk.NewCoin("uzrn", bountyAmt))
			if err := m.Keeper.bankKeeper.SendCoinsFromAccountToModule(ctx, creatorAddr, types.ModuleName, coins); err != nil {
				return nil, types.ErrInsufficientPayment.Wrap("failed to escrow bounty: " + err.Error())
			}

			project.Spent = strconv.FormatInt(spent+bountyAmt.Int64(), 10)
		}
	}

	counter := m.Keeper.GetNextTaskId(ctx)
	taskId := fmt.Sprintf("task-%d-%d", currentBlock, counter)

	task := &types.ProjectTask{
		Id:                   taskId,
		ProjectId:            msg.ProjectId,
		Title:                msg.Title,
		Description:          msg.Description,
		RequiredCapabilities: msg.RequiredCapabilities,
		Status:               string(types.TaskOpen),
		CreatedAtBlock:       currentBlock,
		BountyAmount:         msg.Bounty,
		Reviewers:            []string{},
	}

	m.Keeper.SetTask(ctx, task)

	project.TaskIds = append(project.TaskIds, taskId)
	project.TotalTasks++
	m.Keeper.SetProject(ctx, project)

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.tree.task_added",
		sdk.NewAttribute("task_id", taskId),
		sdk.NewAttribute("project_id", msg.ProjectId),
		sdk.NewAttribute("bounty_escrowed", bountyStr),
	))

	return &types.MsgAddTaskResponse{TaskId: taskId}, nil
}

func (m msgServer) AssignTask(goCtx context.Context, msg *types.MsgAssignTask) (*types.MsgAssignTaskResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	task, found := m.Keeper.GetTask(ctx, msg.TaskId)
	if !found {
		return nil, types.ErrTaskNotFound
	}

	project, found := m.Keeper.GetProject(ctx, task.ProjectId)
	if !found {
		return nil, types.ErrProjectNotFound
	}
	if !IsFounderOrContributor(project, msg.Assigner) {
		return nil, types.ErrNotContributor
	}

	if task.Status != string(types.TaskOpen) {
		return nil, types.ErrInvalidTaskState
	}

	task.Assignee = msg.Assignee
	task.Status = string(types.TaskAssigned)
	m.Keeper.SetTask(ctx, task)

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.tree.task_assigned",
		sdk.NewAttribute("task_id", msg.TaskId),
		sdk.NewAttribute("assignee", msg.Assignee),
	))

	return &types.MsgAssignTaskResponse{}, nil
}

func (m msgServer) StartWork(goCtx context.Context, msg *types.MsgStartWork) (*types.MsgStartWorkResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	task, found := m.Keeper.GetTask(ctx, msg.TaskId)
	if !found {
		return nil, types.ErrTaskNotFound
	}
	if task.Assignee != msg.Worker {
		return nil, types.ErrNotAssignee
	}
	if task.Status != string(types.TaskAssigned) {
		return nil, types.ErrInvalidTaskState
	}

	task.Status = string(types.TaskInProgress)
	m.Keeper.SetTask(ctx, task)

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.tree.work_started",
		sdk.NewAttribute("task_id", msg.TaskId),
	))

	return &types.MsgStartWorkResponse{}, nil
}

func (m msgServer) SubmitDeliverable(goCtx context.Context, msg *types.MsgSubmitDeliverable) (*types.MsgSubmitDeliverableResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	task, found := m.Keeper.GetTask(ctx, msg.TaskId)
	if !found {
		return nil, types.ErrTaskNotFound
	}
	if task.Assignee != msg.Worker {
		return nil, types.ErrNotAssignee
	}
	if task.Status != string(types.TaskInProgress) {
		return nil, types.ErrInvalidTaskState
	}

	deliverable := types.TaskDeliverable{
		Hash:             msg.DeliverableHash,
		SubmittedAtBlock: uint64(ctx.BlockHeight()),
	}
	task.Deliverable = &deliverable
	task.Status = string(types.TaskReview)
	m.Keeper.SetTask(ctx, task)

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.tree.deliverable_submitted",
		sdk.NewAttribute("task_id", msg.TaskId),
		sdk.NewAttribute("hash", msg.DeliverableHash),
	))

	return &types.MsgSubmitDeliverableResponse{}, nil
}

func (m msgServer) ApproveDeliverable(goCtx context.Context, msg *types.MsgApproveDeliverable) (*types.MsgApproveDeliverableResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	task, found := m.Keeper.GetTask(ctx, msg.TaskId)
	if !found {
		return nil, types.ErrTaskNotFound
	}
	if task.Status != string(types.TaskReview) {
		return nil, types.ErrInvalidTaskState
	}
	if msg.Approver == task.Assignee {
		return nil, types.ErrSelfReview
	}

	project, found := m.Keeper.GetProject(ctx, task.ProjectId)
	if !found {
		return nil, types.ErrProjectNotFound
	}
	if !IsFounderOrContributor(project, msg.Approver) {
		return nil, types.ErrNotContributor
	}

	task.Status = string(types.TaskCompleted)
	task.CompletedAtBlock = uint64(ctx.BlockHeight())
	if task.Deliverable != nil {
		task.Deliverable.VerifiedBy = append(task.Deliverable.VerifiedBy, msg.Approver)
	}
	m.Keeper.SetTask(ctx, task)

	project.CompletedTasks++

	if task.BountyAmount != "" && task.BountyAmount != "0" {
		bounty, ok := math.NewIntFromString(task.BountyAmount)
		if ok && bounty.IsPositive() {
			assigneeAddr, addrErr := sdk.AccAddressFromBech32(task.Assignee)
			if addrErr == nil {
				coins := sdk.NewCoins(sdk.NewCoin("uzrn", bounty))
				err := m.Keeper.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, assigneeAddr, coins)
				if err != nil {
					return nil, fmt.Errorf("bounty payout failed: %w", err)
				}

				for i, c := range project.Contributors {
					if c.Did == task.Assignee {
						project.Contributors[i].TasksCompleted++
						existing, _ := strconv.ParseInt(project.Contributors[i].TotalEarned, 10, 64)
						project.Contributors[i].TotalEarned = strconv.FormatInt(existing+bounty.Int64(), 10)
						break
					}
				}
			}
		}
	}

	m.Keeper.SetProject(ctx, project)

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.tree.deliverable_approved",
		sdk.NewAttribute("task_id", msg.TaskId),
		sdk.NewAttribute("approver", msg.Approver),
	))

	return &types.MsgApproveDeliverableResponse{}, nil
}

func (m msgServer) RejectDeliverable(goCtx context.Context, msg *types.MsgRejectDeliverable) (*types.MsgRejectDeliverableResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	task, found := m.Keeper.GetTask(ctx, msg.TaskId)
	if !found {
		return nil, types.ErrTaskNotFound
	}
	if task.Status != string(types.TaskReview) {
		return nil, types.ErrInvalidTaskState
	}

	project, found := m.Keeper.GetProject(ctx, task.ProjectId)
	if !found {
		return nil, types.ErrProjectNotFound
	}
	if !IsFounderOrContributor(project, msg.Rejector) {
		return nil, types.ErrNotContributor
	}

	params := m.Keeper.GetParams(ctx)
	task.RejectionCount++

	if task.RejectionCount >= params.MaxRejections {
		task.Status = string(types.TaskDisputed)

		if task.BountyAmount != "" && task.BountyAmount != "0" {
			bounty, ok := math.NewIntFromString(task.BountyAmount)
			if ok && bounty.IsPositive() {
				slashAmount := bounty.Int64() * types.DisputeSlashBps / types.BpsDenominator
				refundAmount := bounty.Int64() - slashAmount

				if refundAmount > 0 {
					founderAddr, addrErr := sdk.AccAddressFromBech32(project.Founder)
					if addrErr == nil {
						refundCoins := sdk.NewCoins(sdk.NewCoin("uzrn", math.NewInt(refundAmount)))
						if err := m.Keeper.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, founderAddr, refundCoins); err != nil {
							return nil, types.ErrDisputeSlashFailed.Wrap("refund failed: " + err.Error())
						}
					}
				}

				spent, _ := strconv.ParseInt(project.Spent, 10, 64)
				spent -= refundAmount
				if spent < 0 {
					spent = 0
				}
				project.Spent = strconv.FormatInt(spent, 10)
				m.Keeper.SetProject(ctx, project)

				ctx.EventManager().EmitEvent(sdk.NewEvent(
					"zerone.tree.dispute_slash",
					sdk.NewAttribute("task_id", msg.TaskId),
					sdk.NewAttribute("slash_amount", strconv.FormatInt(slashAmount, 10)),
					sdk.NewAttribute("refund_amount", strconv.FormatInt(refundAmount, 10)),
				))
			}
		}
	} else {
		task.Status = string(types.TaskInProgress)
	}
	m.Keeper.SetTask(ctx, task)

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.tree.deliverable_rejected",
		sdk.NewAttribute("task_id", msg.TaskId),
		sdk.NewAttribute("reason", msg.Reason),
		sdk.NewAttribute("rejection_count", fmt.Sprintf("%d", task.RejectionCount)),
		sdk.NewAttribute("disputed", fmt.Sprintf("%v", task.RejectionCount >= params.MaxRejections)),
	))

	return &types.MsgRejectDeliverableResponse{}, nil
}

func (m msgServer) ReopenTask(goCtx context.Context, msg *types.MsgReopenTask) (*types.MsgReopenTaskResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	task, found := m.Keeper.GetTask(ctx, msg.TaskId)
	if !found {
		return nil, types.ErrTaskNotFound
	}

	project, found := m.Keeper.GetProject(ctx, task.ProjectId)
	if !found {
		return nil, types.ErrProjectNotFound
	}
	if project.Founder != msg.Authority {
		return nil, types.ErrNotFounder
	}

	if task.Status != string(types.TaskRejected) && task.Status != string(types.TaskDisputed) {
		return nil, types.ErrInvalidTaskState
	}

	task.Status = string(types.TaskOpen)
	task.Assignee = ""
	task.Deliverable = nil
	task.RejectionCount = 0
	m.Keeper.SetTask(ctx, task)

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.tree.task_reopened",
		sdk.NewAttribute("task_id", msg.TaskId),
	))

	return &types.MsgReopenTaskResponse{}, nil
}

// ======================== Application Messages ========================

func (m msgServer) ApplyToProject(goCtx context.Context, msg *types.MsgApplyToProject) (*types.MsgApplyToProjectResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	project, found := m.Keeper.GetProject(ctx, msg.ProjectId)
	if !found {
		return nil, types.ErrProjectNotFound
	}

	params := m.Keeper.GetParams(ctx)
	if uint32(len(project.Applications)) >= params.MaxApplications {
		return nil, types.ErrMaxApplicationsReached
	}

	for _, app := range project.Applications {
		if app.Did == msg.Applicant && app.Status == string(types.AppPending) {
			return nil, types.ErrAlreadyApplied
		}
	}

	project.Applications = append(project.Applications, &types.ProjectApplication{
		Did:            msg.Applicant,
		ProposedRole:   msg.Role,
		Message:        msg.Pitch,
		Capabilities:   msg.Capabilities,
		AppliedAtBlock: uint64(ctx.BlockHeight()),
		Status:         string(types.AppPending),
	})
	m.Keeper.SetProject(ctx, project)

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.tree.project_application",
		sdk.NewAttribute("project_id", msg.ProjectId),
		sdk.NewAttribute("applicant", msg.Applicant),
	))

	return &types.MsgApplyToProjectResponse{}, nil
}

func (m msgServer) ReviewApplication(goCtx context.Context, msg *types.MsgReviewApplication) (*types.MsgReviewApplicationResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	var targetProject *types.ProductProject
	var appIndex int = -1

	m.Keeper.IterateProjects(ctx, func(p *types.ProductProject) bool {
		for i, app := range p.Applications {
			if app.Did == msg.ApplicationId && app.Status == string(types.AppPending) {
				targetProject = p
				appIndex = i
				return true
			}
		}
		return false
	})

	if targetProject == nil || appIndex < 0 {
		return nil, types.ErrApplicationNotFound
	}
	if targetProject.Founder != msg.Reviewer {
		return nil, types.ErrNotFounder
	}

	if msg.Accepted {
		targetProject.Applications[appIndex].Status = string(types.AppApproved)

		params := m.Keeper.GetParams(ctx)
		if uint32(len(targetProject.Contributors)) >= params.MaxContributors {
			return nil, types.ErrMaxContributorsReached
		}

		app := targetProject.Applications[appIndex]
		targetProject.Contributors = append(targetProject.Contributors, &types.ContributorRecord{
			Did:           app.Did,
			Role:          app.ProposedRole,
			JoinedAtBlock: uint64(ctx.BlockHeight()),
		})
	} else {
		targetProject.Applications[appIndex].Status = string(types.AppRejected)
	}

	m.Keeper.SetProject(ctx, targetProject)

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.tree.application_reviewed",
		sdk.NewAttribute("applicant", msg.ApplicationId),
		sdk.NewAttribute("accepted", fmt.Sprintf("%v", msg.Accepted)),
	))

	return &types.MsgReviewApplicationResponse{}, nil
}

func (m msgServer) SetAvailability(goCtx context.Context, msg *types.MsgSetAvailability) (*types.MsgSetAvailabilityResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	avail := &types.AgentAvailability{
		Agent:            msg.Agent,
		Available:        msg.Available,
		Capabilities:     msg.Capabilities,
		PreferredDomains: msg.PreferredDomains,
		MinimumBounty:    msg.MinimumBounty,
		UpdatedAtBlock:   ctx.BlockHeight(),
	}

	m.SetAgentAvailability(ctx, avail)

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.tree.availability_set",
		sdk.NewAttribute("agent", msg.Agent),
		sdk.NewAttribute("available", fmt.Sprintf("%v", msg.Available)),
	))

	return &types.MsgSetAvailabilityResponse{}, nil
}

// ======================== Service Messages ========================

func (m msgServer) DeployService(goCtx context.Context, msg *types.MsgDeployService) (*types.MsgDeployServiceResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	currentBlock := uint64(ctx.BlockHeight())
	counter := m.Keeper.GetNextServiceId(ctx)
	serviceId := fmt.Sprintf("svc-%d-%d", currentBlock, counter)

	pricePerCall := msg.PricePerCall
	if pricePerCall == "" {
		pricePerCall = "0"
	}

	service := &types.ServiceLeaf{
		Id:              serviceId,
		Name:            msg.Name,
		Description:     msg.Description,
		ContractAddress: msg.Endpoint,
		TotalCalls:      "0",
		TotalRevenue:    "0",
		UptimeBlocks:    "0",
		Status:          string(types.ServiceDeploying),
		DeployedAtBlock: currentBlock,
		PricePerCall:    pricePerCall,
	}

	m.Keeper.SetService(ctx, service)

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.tree.service_deployed",
		sdk.NewAttribute("service_id", serviceId),
		sdk.NewAttribute("deployer", msg.Deployer),
	))

	return &types.MsgDeployServiceResponse{ServiceId: serviceId}, nil
}

func (m msgServer) PauseService(goCtx context.Context, msg *types.MsgPauseService) (*types.MsgPauseServiceResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	service, found := m.Keeper.GetService(ctx, msg.ServiceId)
	if !found {
		return nil, types.ErrServiceNotFound
	}

	if service.Status != string(types.ServiceActive) && service.Status != string(types.ServiceDeploying) && service.Status != string(types.ServiceDegraded) {
		return nil, types.ErrInvalidServiceState
	}

	service.Status = string(types.ServicePaused)
	m.Keeper.SetService(ctx, service)

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.tree.service_paused",
		sdk.NewAttribute("service_id", msg.ServiceId),
	))

	return &types.MsgPauseServiceResponse{}, nil
}

func (m msgServer) ResumeService(goCtx context.Context, msg *types.MsgResumeService) (*types.MsgResumeServiceResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	service, found := m.Keeper.GetService(ctx, msg.ServiceId)
	if !found {
		return nil, types.ErrServiceNotFound
	}

	if service.Status != string(types.ServicePaused) {
		return nil, types.ErrInvalidServiceState
	}

	service.Status = string(types.ServiceActive)
	m.Keeper.SetService(ctx, service)

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.tree.service_resumed",
		sdk.NewAttribute("service_id", msg.ServiceId),
	))

	return &types.MsgResumeServiceResponse{}, nil
}

func (m msgServer) RetireService(goCtx context.Context, msg *types.MsgRetireService) (*types.MsgRetireServiceResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	service, found := m.Keeper.GetService(ctx, msg.ServiceId)
	if !found {
		return nil, types.ErrServiceNotFound
	}

	if service.Status == string(types.ServiceRetired) {
		return nil, types.ErrInvalidServiceState
	}

	service.Status = string(types.ServiceRetired)
	m.Keeper.SetService(ctx, service)

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.tree.service_retired",
		sdk.NewAttribute("service_id", msg.ServiceId),
	))

	return &types.MsgRetireServiceResponse{}, nil
}

// ======================== CallService ========================

func (m msgServer) CallService(goCtx context.Context, msg *types.MsgCallService) (*types.MsgCallServiceResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	service, found := m.Keeper.GetService(ctx, msg.ServiceId)
	if !found {
		return nil, types.ErrServiceNotFound
	}
	if service.Status != string(types.ServiceActive) {
		return nil, types.ErrServiceNotActive
	}

	hasSubscription := m.Keeper.HasActiveSubscription(ctx, msg.ServiceId, msg.Caller)

	price, ok := math.NewIntFromString(service.PricePerCall)
	if !ok {
		price = math.ZeroInt()
	}

	channelPayment := false
	if price.IsPositive() && !hasSubscription {
		if strings.HasPrefix(msg.MaxFee, "channel:") && m.Keeper.channelsKeeper != nil {
			channelId := strings.TrimPrefix(msg.MaxFee, "channel:")

			payer, _, _, status, chFound := m.Keeper.channelsKeeper.GetChannelInfo(ctx, channelId)
			if !chFound {
				return nil, fmt.Errorf("payment channel %s not found", channelId)
			}
			if status != "open" {
				return nil, fmt.Errorf("payment channel %s is not open (status: %s)", channelId, status)
			}
			if payer != msg.Caller {
				return nil, fmt.Errorf("caller %s is not the payer of channel %s", msg.Caller, channelId)
			}

			if err := m.Keeper.channelsKeeper.SpendFromChannel(ctx, channelId, price.String(), types.ModuleName); err != nil {
				return nil, types.ErrInsufficientPayment.Wrap(err.Error())
			}
			channelPayment = true
		}

		if !channelPayment {
			callerAddr, err := sdk.AccAddressFromBech32(msg.Caller)
			if err != nil {
				return nil, fmt.Errorf("invalid caller address: %w", err)
			}

			paymentCoins := sdk.NewCoins(sdk.NewCoin("uzrn", price))
			err = m.Keeper.bankKeeper.SendCoinsFromAccountToModule(ctx, callerAddr, types.ModuleName, paymentCoins)
			if err != nil {
				return nil, types.ErrInsufficientPayment.Wrap(err.Error())
			}
		}

		project, projFound := m.Keeper.GetProject(ctx, service.ProjectId)
		if projFound {
			params := m.Keeper.GetParams(ctx)

			dist := CalculateRevenue(
				price.Int64(),
				params.ContributorsBp,
				params.ProtocolTreasuryBp,
				params.ResearchFundBp,
				params.BurnBp, // proto field 13: development fund share (formerly burn)
				project.Contributors,
			)

			for _, share := range dist.ContributorShares {
				if share.Amount > 0 {
					addr, addrErr := sdk.AccAddressFromBech32(share.Address)
					if addrErr != nil {
						return nil, fmt.Errorf("invalid contributor address %s: %w", share.Address, addrErr)
					}
					coins := sdk.NewCoins(sdk.NewCoin("uzrn", math.NewInt(share.Amount)))
					if err := m.Keeper.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, addr, coins); err != nil {
						return nil, fmt.Errorf("contributor payment failed: %w", err)
					}
				}
			}

			if dist.ResearchFund > 0 {
				coins := sdk.NewCoins(sdk.NewCoin("uzrn", math.NewInt(dist.ResearchFund)))
				if err := m.Keeper.researchFundDepositor.DepositToResearchFund(ctx, types.ModuleName, coins); err != nil {
					return nil, fmt.Errorf("research fund deposit failed: %w", err)
				}
			}

			if dist.ProtocolTreasury > 0 {
				coins := sdk.NewCoins(sdk.NewCoin("uzrn", math.NewInt(dist.ProtocolTreasury)))
				if err := m.Keeper.bankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, "treasury_protocol", coins); err != nil {
					return nil, fmt.Errorf("treasury transfer failed: %w", err)
				}
			}

			if dist.VerificationPool > 0 {
				coins := sdk.NewCoins(sdk.NewCoin("uzrn", math.NewInt(dist.VerificationPool)))
				if err := m.Keeper.bankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, KnowledgeModuleName, coins); err != nil {
					return nil, fmt.Errorf("verification pool transfer failed: %w", err)
				}
			}

			if dist.DevelopmentFund > 0 {
				coins := sdk.NewCoins(sdk.NewCoin("uzrn", math.NewInt(dist.DevelopmentFund)))
				if err := m.Keeper.bankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, "development_fund", coins); err != nil {
					return nil, fmt.Errorf("development fund deposit failed: %w", err)
				}
			}

			existingRevenue, _ := strconv.ParseInt(project.RevenueGenerated, 10, 64)
			project.RevenueGenerated = strconv.FormatInt(existingRevenue+price.Int64(), 10)
			m.Keeper.SetProject(ctx, project)
		}
	}

	existingCalls, _ := strconv.ParseInt(service.TotalCalls, 10, 64)
	service.TotalCalls = strconv.FormatInt(existingCalls+1, 10)

	if !hasSubscription {
		existingRevenue, _ := strconv.ParseInt(service.TotalRevenue, 10, 64)
		service.TotalRevenue = strconv.FormatInt(existingRevenue+price.Int64(), 10)
	}

	service.LastCalledBlock = uint64(ctx.BlockHeight())
	m.Keeper.SetService(ctx, service)

	paymentMethod := "direct"
	if hasSubscription {
		paymentMethod = "subscription"
	} else if channelPayment {
		paymentMethod = "channel"
	}

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.tree.service_called",
		sdk.NewAttribute("service_id", msg.ServiceId),
		sdk.NewAttribute("caller", msg.Caller),
		sdk.NewAttribute("amount", price.String()),
		sdk.NewAttribute("payment_method", paymentMethod),
	))

	return &types.MsgCallServiceResponse{
		Result: []byte(price.String()),
	}, nil
}

// ======================== Contributor Messages ========================

func (m msgServer) AddContributor(goCtx context.Context, msg *types.MsgAddContributor) (*types.MsgAddContributorResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	project, found := m.Keeper.GetProject(ctx, msg.ProjectId)
	if !found {
		return nil, types.ErrProjectNotFound
	}
	if project.Founder != msg.Authority {
		return nil, types.ErrNotFounder
	}

	params := m.Keeper.GetParams(ctx)
	if uint32(len(project.Contributors)) >= params.MaxContributors {
		return nil, types.ErrMaxContributorsReached
	}

	for _, c := range project.Contributors {
		if c.Did == msg.Contributor {
			return nil, types.ErrAlreadyApplied.Wrap("address is already a contributor")
		}
	}

	project.Contributors = append(project.Contributors, &types.ContributorRecord{
		Did:           msg.Contributor,
		Role:          msg.Role,
		JoinedAtBlock: uint64(ctx.BlockHeight()),
	})
	m.Keeper.SetProject(ctx, project)

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.tree.contributor_added",
		sdk.NewAttribute("project_id", msg.ProjectId),
		sdk.NewAttribute("contributor", msg.Contributor),
		sdk.NewAttribute("role", msg.Role),
	))

	return &types.MsgAddContributorResponse{}, nil
}

// ======================== Subscription Messages ========================

func (m msgServer) SubscribeService(goCtx context.Context, msg *types.MsgSubscribeService) (*types.MsgSubscribeServiceResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	service, found := m.Keeper.GetService(ctx, msg.ServiceId)
	if !found {
		return nil, types.ErrServiceNotFound
	}
	if service.Status != string(types.ServiceActive) {
		return nil, types.ErrServiceNotActive
	}

	currentBlock := uint64(ctx.BlockHeight())
	counter := m.Keeper.GetNextSubscriptionId(ctx)
	subId := fmt.Sprintf("sub-%d-%d", currentBlock, counter)

	sub := types.ServiceSubscription{
		Id:             subId,
		ServiceId:      msg.ServiceId,
		Subscriber:     msg.Subscriber,
		StartBlock:     currentBlock,
		DurationBlocks: msg.DurationBlocks,
		ExpiresAtBlock: currentBlock + msg.DurationBlocks,
	}

	m.Keeper.SetSubscription(ctx, sub)

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.tree.service_subscribed",
		sdk.NewAttribute("subscription_id", subId),
		sdk.NewAttribute("service_id", msg.ServiceId),
		sdk.NewAttribute("subscriber", msg.Subscriber),
		sdk.NewAttribute("duration_blocks", strconv.FormatUint(msg.DurationBlocks, 10)),
	))

	return &types.MsgSubscribeServiceResponse{SubscriptionId: subId}, nil
}

// ======================== Seeding Messages ========================

func (m msgServer) BeginSeeding(goCtx context.Context, msg *types.MsgBeginSeeding) (*types.MsgBeginSeedingResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	project, found := m.Keeper.GetProject(ctx, msg.ProjectId)
	if !found {
		return nil, types.ErrProjectNotFound
	}
	if !IsFounderOrContributor(project, msg.Seeder) {
		return nil, types.ErrNotContributor
	}
	if !IsValidPhaseTransition(types.ProjectPhase(project.Phase), types.PhaseSeeding) {
		return nil, types.ErrInvalidPhaseTransition
	}

	currentBlock := uint64(ctx.BlockHeight())

	project.PreviousPhase = project.Phase
	project.Phase = string(types.PhaseSeeding)
	project.PhaseChangedAtBlock = currentBlock
	m.Keeper.SetProject(ctx, project)

	params := m.Keeper.GetParams(ctx)
	seedCounter := m.Keeper.GetNextSeedId(ctx)
	seedId := fmt.Sprintf("seed-%d-%d", currentBlock, seedCounter)

	domain := msg.Domain
	if domain == "" {
		domain = project.KnowledgeDomain
	}

	seed := &types.OpportunitySeed{
		Id:              seedId,
		DetectedAtBlock: currentBlock,
		KnowledgeDomain: domain,
		Status:          string(types.SeedDetected),
		ProjectId:       msg.ProjectId,
		ExpiresAtBlock:  currentBlock + params.SeedExpiryBlocks,
	}
	m.Keeper.SetSeed(ctx, seed)

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.tree.seeding_begun",
		sdk.NewAttribute("project_id", msg.ProjectId),
		sdk.NewAttribute("seed_id", seedId),
		sdk.NewAttribute("domain", domain),
	))

	return &types.MsgBeginSeedingResponse{}, nil
}

// ======================== Opportunity Messages ========================

func (m msgServer) DetectOpportunity(goCtx context.Context, msg *types.MsgDetectOpportunity) (*types.MsgDetectOpportunityResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	currentBlock := uint64(ctx.BlockHeight())
	params := m.Keeper.GetParams(ctx)
	counter := m.Keeper.GetNextSeedId(ctx)
	seedId := fmt.Sprintf("seed-%d-%d", currentBlock, counter)

	seed := &types.OpportunitySeed{
		Id:              seedId,
		DetectedAtBlock: currentBlock,
		KnowledgeDomain: msg.Domain,
		Confidence:      "0.5",
		Status:          string(types.SeedDetected),
		ExpiresAtBlock:  currentBlock + params.SeedExpiryBlocks,
	}

	if msg.Description != "" || len(msg.RelatedFacts) > 0 {
		evidence := make(map[string]string)
		evidence["description"] = msg.Description
		for i, fact := range msg.RelatedFacts {
			evidence[fmt.Sprintf("related_fact_%d", i)] = fact
		}
		seed.Signal = &types.DemandSignal{
			Type:     "agent_detection",
			Evidence: evidence,
			Strength: "0.5",
		}
	}

	m.Keeper.SetSeed(ctx, seed)

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.tree.opportunity_detected",
		sdk.NewAttribute("opportunity_id", seedId),
		sdk.NewAttribute("detector", msg.Detector),
		sdk.NewAttribute("domain", msg.Domain),
	))

	return &types.MsgDetectOpportunityResponse{OpportunityId: seedId}, nil
}

func (m msgServer) ClaimOpportunity(goCtx context.Context, msg *types.MsgClaimOpportunity) (*types.MsgClaimOpportunityResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	seed, found := m.Keeper.GetSeed(ctx, msg.OpportunityId)
	if !found {
		return nil, types.ErrSeedNotFound
	}
	if seed.Status == string(types.SeedClaimed) {
		return nil, types.ErrSeedAlreadyClaimed
	}
	if seed.Status == string(types.SeedExpired) {
		return nil, types.ErrSeedExpired
	}

	currentBlock := uint64(ctx.BlockHeight())
	if seed.ExpiresAtBlock > 0 && currentBlock >= seed.ExpiresAtBlock {
		seed.Status = string(types.SeedExpired)
		m.Keeper.SetSeed(ctx, seed)
		return nil, types.ErrSeedExpired
	}

	if msg.Stake != "" && msg.Stake != "0" {
		stakeAmount, ok := math.NewIntFromString(msg.Stake)
		if ok && stakeAmount.IsPositive() {
			claimerAddr, err := sdk.AccAddressFromBech32(msg.Claimer)
			if err != nil {
				return nil, fmt.Errorf("invalid claimer address: %w", err)
			}
			coins := sdk.NewCoins(sdk.NewCoin("uzrn", stakeAmount))
			err = m.Keeper.bankKeeper.SendCoinsFromAccountToModule(ctx, claimerAddr, types.ModuleName, coins)
			if err != nil {
				return nil, types.ErrInsufficientPayment.Wrap(err.Error())
			}
		}
	}

	seed.Status = string(types.SeedClaimed)
	seed.ClaimedBy = msg.Claimer
	m.Keeper.SetSeed(ctx, seed)

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.tree.opportunity_claimed",
		sdk.NewAttribute("opportunity_id", msg.OpportunityId),
		sdk.NewAttribute("claimer", msg.Claimer),
		sdk.NewAttribute("stake", msg.Stake),
	))

	return &types.MsgClaimOpportunityResponse{}, nil
}

// ======================== Params ========================

func (m msgServer) UpdateParams(goCtx context.Context, msg *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	if m.GetAuthority() != msg.Authority {
		return nil, fmt.Errorf("unauthorized: expected %s, got %s", m.GetAuthority(), msg.Authority)
	}
	ctx := sdk.UnwrapSDKContext(goCtx)
	m.SetParams(ctx, msg.Params)
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.tree.params_updated",
			sdk.NewAttribute("authority", msg.Authority),
		),
	)
	return &types.MsgUpdateParamsResponse{}, nil
}
