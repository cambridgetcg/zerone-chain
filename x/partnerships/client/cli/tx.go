package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"

	"github.com/zerone-chain/zerone/x/partnerships/types"
)

// NewTxCmd returns the transaction commands for the partnerships module.
func NewTxCmd() *cobra.Command {
	txCmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "Partnerships module transaction commands",
		DisableFlagParsing:         false,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	txCmd.AddCommand(
		NewProposePartnershipCmd(),
		NewAcceptPartnershipCmd(),
		NewProposeConsensusOpCmd(),
		NewVoteConsensusOpCmd(),
		NewSafetyFreezeCmd(),
		NewRaiseCoercionCmd(),
		NewInitiateDissolutionCmd(),
		NewCreateSeedPartnershipCmd(),
		NewJoinFormationPoolCmd(),
		NewLeaveFormationPoolCmd(),
		NewProposeMentorshipCmd(),
		NewAcceptMentorshipCmd(),
		NewGraduateMenteeCmd(),
		NewEndMentorshipCmd(),
		NewAcceptMatchCmd(),
		NewDeclineMatchCmd(),
	)

	return txCmd
}

// NewProposePartnershipCmd creates a CLI command for MsgProposePartnership.
func NewProposePartnershipCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "propose [partner] [initial-deposit] [proposed-tier]",
		Short: "Propose a new partnership",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			proposedTier, err := strconv.ParseUint(args[2], 10, 32)
			if err != nil {
				return err
			}

			msg := &types.MsgProposePartnership{
				Proposer:       clientCtx.GetFromAddress().String(),
				Partner:        args[0],
				InitialDeposit: args[1],
				ProposedTier:   uint32(proposedTier),
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewAcceptPartnershipCmd creates a CLI command for MsgAcceptPartnership.
func NewAcceptPartnershipCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "accept [partnership-id] [deposit]",
		Short: "Accept a proposed partnership",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := &types.MsgAcceptPartnership{
				Accepter:      clientCtx.GetFromAddress().String(),
				PartnershipId: args[0],
				Deposit:       args[1],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewProposeConsensusOpCmd creates a CLI command for MsgProposeConsensusOp.
func NewProposeConsensusOpCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "propose-op [partnership-id] [op-type] [amount] [rationale]",
		Short: "Propose a consensus operation within a partnership",
		Args:  cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := &types.MsgProposeConsensusOp{
				Proposer:      clientCtx.GetFromAddress().String(),
				PartnershipId: args[0],
				OpType:        args[1],
				Amount:        args[2],
				Rationale:     args[3],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewVoteConsensusOpCmd creates a CLI command for MsgVoteConsensusOp.
func NewVoteConsensusOpCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vote-op [partnership-id] [operation-id] [approve:true/false] --rationale [r] --counter-amount [a]",
		Short: "Vote on a pending consensus operation",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			approve, err := strconv.ParseBool(args[2])
			if err != nil {
				return err
			}

			rationale, _ := cmd.Flags().GetString("rationale")
			counterAmount, _ := cmd.Flags().GetString("counter-amount")

			msg := &types.MsgVoteConsensusOp{
				Voter:         clientCtx.GetFromAddress().String(),
				PartnershipId: args[0],
				OperationId:   args[1],
				Approve:       approve,
				Rationale:     rationale,
				CounterAmount: counterAmount,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	cmd.Flags().String("rationale", "", "Rationale for the vote")
	cmd.Flags().String("counter-amount", "", "Counter-proposal amount")
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewSafetyFreezeCmd creates a CLI command for MsgSafetyFreeze.
func NewSafetyFreezeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "safety-freeze [partnership-id]",
		Short: "Trigger a safety freeze on a partnership",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := &types.MsgSafetyFreeze{
				Freezer:       clientCtx.GetFromAddress().String(),
				PartnershipId: args[0],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewRaiseCoercionCmd creates a CLI command for MsgRaiseCoercionSignal.
func NewRaiseCoercionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "raise-coercion [partnership-id]",
		Short: "Raise a coercion signal for a partnership",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := &types.MsgRaiseCoercionSignal{
				Raiser:        clientCtx.GetFromAddress().String(),
				PartnershipId: args[0],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewInitiateDissolutionCmd creates a CLI command for MsgInitiateDissolution.
func NewInitiateDissolutionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dissolve [partnership-id]",
		Short: "Initiate dissolution of a partnership",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := &types.MsgInitiateDissolution{
				Initiator:     clientCtx.GetFromAddress().String(),
				PartnershipId: args[0],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewCreateSeedPartnershipCmd creates a CLI command for MsgCreateSeedPartnership.
func NewCreateSeedPartnershipCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create-seed [agent] [human-contribution]",
		Short: "Create a seed partnership between a human and an agent",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := &types.MsgCreateSeedPartnership{
				Human:             clientCtx.GetFromAddress().String(),
				Agent:             args[0],
				HumanContribution: args[1],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewJoinFormationPoolCmd creates a CLI command for MsgJoinFormationPool.
func NewJoinFormationPoolCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "join-formation [deposit] --domains [d1,d2] --preferred-role [role]",
		Short: "Join the partnership formation pool",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			domainsStr, _ := cmd.Flags().GetString("domains")
			var domains []string
			if domainsStr != "" {
				domains = strings.Split(domainsStr, ",")
				for i := range domains {
					domains[i] = strings.TrimSpace(domains[i])
				}
			}

			preferredRole, _ := cmd.Flags().GetString("preferred-role")

			msg := &types.MsgJoinFormationPool{
				Joiner:        clientCtx.GetFromAddress().String(),
				Domains:       domains,
				PreferredRole: preferredRole,
				Deposit:       args[0],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	cmd.Flags().String("domains", "", "Comma-separated list of domains")
	cmd.Flags().String("preferred-role", "", "Preferred role in the partnership")
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewLeaveFormationPoolCmd creates a CLI command for MsgLeaveFormationPool.
func NewLeaveFormationPoolCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "leave-formation",
		Short: "Leave the partnership formation pool",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := &types.MsgLeaveFormationPool{
				Leaver: clientCtx.GetFromAddress().String(),
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewProposeMentorshipCmd creates a CLI command for MsgProposeMentorship.
func NewProposeMentorshipCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "propose-mentorship [mentee] [domain] [duration-blocks]",
		Short: "Propose a mentorship to a mentee",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			duration, err := strconv.ParseUint(args[2], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid duration: %w", err)
			}
			msg := &types.MsgProposeMentorship{
				Mentor:         clientCtx.GetFromAddress().String(),
				Mentee:         args[0],
				Domain:         args[1],
				DurationBlocks: duration,
			}
			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewAcceptMentorshipCmd creates a CLI command for MsgAcceptMentorship.
func NewAcceptMentorshipCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "accept-mentorship [mentorship-id]",
		Short: "Accept a mentorship proposal",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			msg := &types.MsgAcceptMentorship{
				Mentee:       clientCtx.GetFromAddress().String(),
				MentorshipId: args[0],
			}
			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewGraduateMenteeCmd creates a CLI command for MsgGraduateMentee.
func NewGraduateMenteeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "graduate-mentee [mentorship-id]",
		Short: "Graduate a mentee from an active mentorship",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			msg := &types.MsgGraduateMentee{
				Mentor:       clientCtx.GetFromAddress().String(),
				MentorshipId: args[0],
			}
			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewEndMentorshipCmd creates a CLI command for MsgEndMentorship.
func NewEndMentorshipCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "end-mentorship [mentorship-id]",
		Short: "End a mentorship early",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			msg := &types.MsgEndMentorship{
				Sender:       clientCtx.GetFromAddress().String(),
				MentorshipId: args[0],
			}
			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewAcceptMatchCmd creates a CLI command for MsgAcceptFormationMatch.
func NewAcceptMatchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "accept-match [match-id]",
		Short: "Accept a formation pool match",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			msg := &types.MsgAcceptFormationMatch{
				Accepter: clientCtx.GetFromAddress().String(),
				MatchId:  args[0],
			}
			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewDeclineMatchCmd creates a CLI command for MsgDeclineFormationMatch.
func NewDeclineMatchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "decline-match [match-id]",
		Short: "Decline a formation pool match",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			msg := &types.MsgDeclineFormationMatch{
				Decliner: clientCtx.GetFromAddress().String(),
				MatchId:  args[0],
			}
			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}
