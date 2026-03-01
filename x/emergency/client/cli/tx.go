package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"

	"github.com/zerone-chain/zerone/x/emergency/types"
)

// NewTxCmd returns the transaction commands for the emergency module.
func NewTxCmd() *cobra.Command {
	txCmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "Emergency module transaction commands",
		DisableFlagParsing:         false,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	txCmd.AddCommand(
		NewProposeHaltCmd(),
		NewVoteHaltCmd(),
		NewProposeRevertCmd(),
		NewVoteRevertCmd(),
		NewProposeResumeCmd(),
		NewVoteResumeCmd(),
	)

	return txCmd
}

// NewProposeHaltCmd creates a CLI command for MsgProposeHalt.
func NewProposeHaltCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "propose-halt [reason]",
		Short: "Propose an emergency chain halt (Guardian-only)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := &types.MsgProposeHalt{
				Proposer: clientCtx.GetFromAddress().String(),
				Reason:   args[0],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewVoteHaltCmd creates a CLI command for MsgVoteHalt.
func NewVoteHaltCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vote-halt [proposal-id] [approve: true/false]",
		Short: "Vote on a halt ceremony (Guardian-only)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			approve, err := strconv.ParseBool(args[1])
			if err != nil {
				return fmt.Errorf("invalid approve value: %w", err)
			}

			msg := &types.MsgVoteHalt{
				Voter:      clientCtx.GetFromAddress().String(),
				ProposalId: args[0],
				Approve:    approve,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewProposeRevertCmd creates a CLI command for MsgProposeRevert.
func NewProposeRevertCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "propose-revert [target-height] [justification]",
		Short: "Propose a state revert (Guardian-only, chain must be halted)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			height, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid target height: %w", err)
			}

			msg := &types.MsgProposeRevert{
				Proposer:      clientCtx.GetFromAddress().String(),
				RevertToHeight: height,
				Justification: args[1],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewVoteRevertCmd creates a CLI command for MsgVoteRevert.
func NewVoteRevertCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vote-revert [proposal-id] [approve: true/false]",
		Short: "Vote on a revert ceremony (Guardian-only)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			approve, err := strconv.ParseBool(args[1])
			if err != nil {
				return fmt.Errorf("invalid approve value: %w", err)
			}

			msg := &types.MsgVoteRevert{
				Voter:      clientCtx.GetFromAddress().String(),
				ProposalId: args[0],
				Approve:    approve,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewProposeResumeCmd creates a CLI command for MsgProposeResume.
func NewProposeResumeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "propose-resume",
		Short: "Propose chain resume (Guardian-only, chain must be halted)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := &types.MsgProposeResume{
				Proposer: clientCtx.GetFromAddress().String(),
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewVoteResumeCmd creates a CLI command for MsgVoteResume.
func NewVoteResumeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vote-resume [proposal-id] [approve: true/false]",
		Short: "Vote on a resume ceremony (Guardian-only)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			approve, err := strconv.ParseBool(args[1])
			if err != nil {
				return fmt.Errorf("invalid approve value: %w", err)
			}

			msg := &types.MsgVoteResume{
				Voter:      clientCtx.GetFromAddress().String(),
				ProposalId: args[0],
				Approve:    approve,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}
