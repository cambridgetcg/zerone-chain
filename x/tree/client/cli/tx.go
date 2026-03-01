package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"

	"github.com/zerone-chain/zerone/x/tree/types"
)

// NewTxCmd returns the transaction commands for the tree module.
func NewTxCmd() *cobra.Command {
	txCmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "Tree module transaction commands",
		DisableFlagParsing:         false,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	txCmd.AddCommand(
		// Project lifecycle
		NewCreateProjectCmd(),
		NewProposeProjectCmd(),
		NewStartDevelopmentCmd(),
		NewCompleteProjectCmd(),
		NewPauseProjectCmd(),
		NewResumeProjectCmd(),
		NewAbandonProjectCmd(),
		NewSpawnChildProjectCmd(),
		// Task workflow
		NewAddTaskCmd(),
		NewAssignTaskCmd(),
		NewStartWorkCmd(),
		NewSubmitDeliverableCmd(),
		NewApproveDeliverableCmd(),
		NewRejectDeliverableCmd(),
		NewReopenTaskCmd(),
		// Contributor management
		NewApplyToProjectCmd(),
		NewReviewApplicationCmd(),
		NewAddContributorCmd(),
		// Availability
		NewSetAvailabilityCmd(),
		// Service operations
		NewDeployServiceCmd(),
		NewCallServiceCmd(),
		NewSubscribeServiceCmd(),
		NewPauseServiceCmd(),
		NewResumeServiceCmd(),
		NewRetireServiceCmd(),
		// Seeding
		NewDetectOpportunityCmd(),
		NewBeginSeedingCmd(),
		NewClaimOpportunityCmd(),
		// Admin
		NewUpdateParamsCmd(),
	)

	return txCmd
}

// ============================================================
// Project Lifecycle
// ============================================================

// NewCreateProjectCmd creates a CLI command for MsgCreateProject.
func NewCreateProjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create-project [name] [description]",
		Short: "Create a new project in the tree",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := &types.MsgCreateProject{
				Creator:     clientCtx.GetFromAddress().String(),
				Title:       args[0],
				Description: args[1],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewProposeProjectCmd creates a CLI command for MsgProposeProject.
func NewProposeProjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "propose-project [project-id]",
		Short: "Propose a project for development",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := &types.MsgProposeProject{
				Proposer:  clientCtx.GetFromAddress().String(),
				ProjectId: args[0],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewStartDevelopmentCmd creates a CLI command for MsgStartDevelopment.
func NewStartDevelopmentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start-development [project-id]",
		Short: "Start development on a project (authority only)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := &types.MsgStartDevelopment{
				Authority: clientCtx.GetFromAddress().String(),
				ProjectId: args[0],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewCompleteProjectCmd creates a CLI command for MsgCompleteProject.
func NewCompleteProjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "complete-project [project-id]",
		Short: "Mark a project as completed (authority only)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := &types.MsgCompleteProject{
				Authority: clientCtx.GetFromAddress().String(),
				ProjectId: args[0],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewPauseProjectCmd creates a CLI command for MsgPauseProject.
func NewPauseProjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pause-project [project-id] [reason]",
		Short: "Pause a project (authority only)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := &types.MsgPauseProject{
				Authority: clientCtx.GetFromAddress().String(),
				ProjectId: args[0],
				Reason:    args[1],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewResumeProjectCmd creates a CLI command for MsgResumeProject.
func NewResumeProjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resume-project [project-id]",
		Short: "Resume a paused project (authority only)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := &types.MsgResumeProject{
				Authority: clientCtx.GetFromAddress().String(),
				ProjectId: args[0],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewAbandonProjectCmd creates a CLI command for MsgAbandonProject.
func NewAbandonProjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "abandon-project [project-id]",
		Short: "Abandon a project (authority only)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := &types.MsgAbandonProject{
				Authority: clientCtx.GetFromAddress().String(),
				ProjectId: args[0],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewSpawnChildProjectCmd creates a CLI command for MsgSpawnChildProject.
func NewSpawnChildProjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "spawn-child-project [parent-id] [title] [description] [budget]",
		Short: "Spawn a child project under an existing project",
		Args:  cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := &types.MsgSpawnChildProject{
				Creator:         clientCtx.GetFromAddress().String(),
				ParentProjectId: args[0],
				Title:           args[1],
				Description:     args[2],
				Budget:          args[3],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// ============================================================
// Task Workflow
// ============================================================

// NewAddTaskCmd creates a CLI command for MsgAddTask.
func NewAddTaskCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add-task [project-id] [title] [description] [bounty-amount]",
		Short: "Add a task to a project with a bounty",
		Args:  cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := &types.MsgAddTask{
				Creator:     clientCtx.GetFromAddress().String(),
				ProjectId:   args[0],
				Title:       args[1],
				Description: args[2],
				Bounty:      args[3],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewAssignTaskCmd creates a CLI command for MsgAssignTask.
func NewAssignTaskCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "assign-task [task-id] [assignee]",
		Short: "Assign a task to a contributor",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := &types.MsgAssignTask{
				Assigner: clientCtx.GetFromAddress().String(),
				TaskId:   args[0],
				Assignee: args[1],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewStartWorkCmd creates a CLI command for MsgStartWork.
func NewStartWorkCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start-work [task-id]",
		Short: "Start work on an assigned task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := &types.MsgStartWork{
				Worker: clientCtx.GetFromAddress().String(),
				TaskId: args[0],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewSubmitDeliverableCmd creates a CLI command for MsgSubmitDeliverable.
func NewSubmitDeliverableCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "submit-deliverable [task-id] [content-hash] [content-uri]",
		Short: "Submit a deliverable for a task",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := &types.MsgSubmitDeliverable{
				Worker:          clientCtx.GetFromAddress().String(),
				TaskId:          args[0],
				DeliverableHash: args[1],
				DeliverableUri:  args[2],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewApproveDeliverableCmd creates a CLI command for MsgApproveDeliverable.
func NewApproveDeliverableCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "approve-deliverable [task-id]",
		Short: "Approve a task deliverable",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := &types.MsgApproveDeliverable{
				Approver: clientCtx.GetFromAddress().String(),
				TaskId:   args[0],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewRejectDeliverableCmd creates a CLI command for MsgRejectDeliverable.
func NewRejectDeliverableCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reject-deliverable [task-id] [reason]",
		Short: "Reject a task deliverable",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := &types.MsgRejectDeliverable{
				Rejector: clientCtx.GetFromAddress().String(),
				TaskId:   args[0],
				Reason:   args[1],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewReopenTaskCmd creates a CLI command for MsgReopenTask.
func NewReopenTaskCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reopen-task [task-id]",
		Short: "Reopen a completed or rejected task (authority only)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := &types.MsgReopenTask{
				Authority: clientCtx.GetFromAddress().String(),
				TaskId:    args[0],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// ============================================================
// Contributor Management
// ============================================================

// NewApplyToProjectCmd creates a CLI command for MsgApplyToProject.
func NewApplyToProjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apply-to-project [project-id] [role] [pitch] [capabilities]",
		Short: "Apply to join a project (capabilities: comma-separated)",
		Args:  cobra.RangeArgs(3, 4),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			var capabilities []string
			if len(args) > 3 && args[3] != "" {
				capabilities = strings.Split(args[3], ",")
			}

			msg := &types.MsgApplyToProject{
				Applicant:    clientCtx.GetFromAddress().String(),
				ProjectId:    args[0],
				Role:         args[1],
				Pitch:        args[2],
				Capabilities: capabilities,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewReviewApplicationCmd creates a CLI command for MsgReviewApplication.
func NewReviewApplicationCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "review-application [application-id] [accepted] [reason]",
		Short: "Review a project application (accepted: true/false)",
		Args:  cobra.RangeArgs(2, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			accepted, err := strconv.ParseBool(args[1])
			if err != nil {
				return fmt.Errorf("invalid accepted value %q: must be true or false", args[1])
			}

			var reason string
			if len(args) > 2 {
				reason = args[2]
			}

			msg := &types.MsgReviewApplication{
				Reviewer:      clientCtx.GetFromAddress().String(),
				ApplicationId: args[0],
				Accepted:      accepted,
				Reason:        reason,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewAddContributorCmd creates a CLI command for MsgAddContributor.
func NewAddContributorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add-contributor [project-id] [address] [role]",
		Short: "Add a contributor to a project (authority only)",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := &types.MsgAddContributor{
				Authority:   clientCtx.GetFromAddress().String(),
				ProjectId:   args[0],
				Contributor: args[1],
				Role:        args[2],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// ============================================================
// Availability
// ============================================================

// NewSetAvailabilityCmd creates a CLI command for MsgSetAvailability.
func NewSetAvailabilityCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set-availability [available] [capabilities] [preferred-domains] [minimum-bounty]",
		Short: "Set agent availability (available: true/false, lists: comma-separated)",
		Args:  cobra.RangeArgs(1, 4),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			available, err := strconv.ParseBool(args[0])
			if err != nil {
				return fmt.Errorf("invalid available value %q: must be true or false", args[0])
			}

			var capabilities []string
			if len(args) > 1 && args[1] != "" {
				capabilities = strings.Split(args[1], ",")
			}

			var preferredDomains []string
			if len(args) > 2 && args[2] != "" {
				preferredDomains = strings.Split(args[2], ",")
			}

			var minimumBounty string
			if len(args) > 3 {
				minimumBounty = args[3]
			}

			msg := &types.MsgSetAvailability{
				Agent:            clientCtx.GetFromAddress().String(),
				Available:        available,
				Capabilities:     capabilities,
				PreferredDomains: preferredDomains,
				MinimumBounty:    minimumBounty,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// ============================================================
// Service Operations
// ============================================================

// NewDeployServiceCmd creates a CLI command for MsgDeployService.
func NewDeployServiceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy-service [name] [description] [endpoint] [price-per-call]",
		Short: "Deploy a service leaf",
		Args:  cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := &types.MsgDeployService{
				Deployer:     clientCtx.GetFromAddress().String(),
				Name:         args[0],
				Description:  args[1],
				Endpoint:     args[2],
				PricePerCall: args[3],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewCallServiceCmd creates a CLI command for MsgCallService.
func NewCallServiceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "call-service [service-id] [input-data] [max-fee]",
		Short: "Call a deployed service",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := &types.MsgCallService{
				Caller:    clientCtx.GetFromAddress().String(),
				ServiceId: args[0],
				Payload:   []byte(args[1]),
				MaxFee:    args[2],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewSubscribeServiceCmd creates a CLI command for MsgSubscribeService.
func NewSubscribeServiceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "subscribe-service [service-id] [duration-blocks]",
		Short: "Subscribe to a service",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			duration, err := strconv.ParseUint(args[1], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid duration-blocks %q: %w", args[1], err)
			}

			msg := &types.MsgSubscribeService{
				Subscriber:     clientCtx.GetFromAddress().String(),
				ServiceId:      args[0],
				DurationBlocks: duration,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewPauseServiceCmd creates a CLI command for MsgPauseService.
func NewPauseServiceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pause-service [service-id]",
		Short: "Pause a deployed service",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := &types.MsgPauseService{
				Owner:     clientCtx.GetFromAddress().String(),
				ServiceId: args[0],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewResumeServiceCmd creates a CLI command for MsgResumeService.
func NewResumeServiceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resume-service [service-id]",
		Short: "Resume a paused service",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := &types.MsgResumeService{
				Owner:     clientCtx.GetFromAddress().String(),
				ServiceId: args[0],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewRetireServiceCmd creates a CLI command for MsgRetireService.
func NewRetireServiceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "retire-service [service-id]",
		Short: "Permanently retire a service",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := &types.MsgRetireService{
				Owner:     clientCtx.GetFromAddress().String(),
				ServiceId: args[0],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// ============================================================
// Seeding
// ============================================================

// NewDetectOpportunityCmd creates a CLI command for MsgDetectOpportunity.
func NewDetectOpportunityCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "detect-opportunity [domain] [description] [related-facts]",
		Short: "Detect a new opportunity (related-facts: comma-separated)",
		Args:  cobra.RangeArgs(2, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			var relatedFacts []string
			if len(args) > 2 && args[2] != "" {
				relatedFacts = strings.Split(args[2], ",")
			}

			msg := &types.MsgDetectOpportunity{
				Detector:     clientCtx.GetFromAddress().String(),
				Domain:       args[0],
				Description:  args[1],
				RelatedFacts: relatedFacts,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewBeginSeedingCmd creates a CLI command for MsgBeginSeeding.
func NewBeginSeedingCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "begin-seeding [project-id] [domain]",
		Short: "Begin seeding a project in a domain",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := &types.MsgBeginSeeding{
				Seeder:    clientCtx.GetFromAddress().String(),
				ProjectId: args[0],
				Domain:    args[1],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewClaimOpportunityCmd creates a CLI command for MsgClaimOpportunity.
func NewClaimOpportunityCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "claim-opportunity [opportunity-id] [stake]",
		Short: "Claim a detected opportunity with stake",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := &types.MsgClaimOpportunity{
				Claimer:       clientCtx.GetFromAddress().String(),
				OpportunityId: args[0],
				Stake:         args[1],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// ============================================================
// Admin
// ============================================================

// NewUpdateParamsCmd creates a CLI command for MsgUpdateParams.
func NewUpdateParamsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update-params [params-json-file]",
		Short: "Update tree module params (authority only, reads params from JSON file)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			contents, err := os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("failed to read params file %q: %w", args[0], err)
			}

			var params types.Params
			if err := json.Unmarshal(contents, &params); err != nil {
				return fmt.Errorf("failed to parse params JSON: %w", err)
			}

			msg := &types.MsgUpdateParams{
				Authority: clientCtx.GetFromAddress().String(),
				Params:    &params,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}
