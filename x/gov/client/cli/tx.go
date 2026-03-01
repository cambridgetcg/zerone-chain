package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"

	"github.com/zerone-chain/zerone/x/gov/types"
)

// NewTxCmd returns the transaction commands for the zerone_gov module.
func NewTxCmd() *cobra.Command {
	txCmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "Zerone governance module transaction commands",
		DisableFlagParsing:         false,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	txCmd.AddCommand(
		NewSubmitLIPCmd(),
		NewStakeLIPCmd(),
		NewAdvanceLIPStageCmd(),
		NewCastVoteCmd(),
		NewWithdrawLIPCmd(),
		NewSubmitResearchSpendCmd(),
		NewVoteResearchSpendCmd(),
		NewSetResearchVotersCmd(),
		NewNominateSeatElectionCmd(),
		NewAcceptSeatNominationCmd(),
		NewVoteSeatElectionCmd(),
		NewProposePhaseTransitionCmd(),
		NewProposePhaseRollbackCmd(),
	)

	return txCmd
}

// NewSubmitLIPCmd creates a CLI command for MsgSubmitLIP.
func NewSubmitLIPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "submit-lip [title] [description] [category] [initial-stake]",
		Short: "Submit a new LIP proposal",
		Long:  "Submit a Zerone Improvement Proposal (LIP) with a title, description, category, and initial stake (uzrn).",
		Args:  cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := &types.MsgSubmitLIP{
				Proposer:     clientCtx.GetFromAddress().String(),
				Title:        args[0],
				Description:  args[1],
				Category:     args[2],
				InitialStake: args[3],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewStakeLIPCmd creates a CLI command for MsgStakeLIP.
func NewStakeLIPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stake-lip [lip-id] [amount]",
		Short: "Stake tokens on an existing LIP",
		Long:  "Add stake (uzrn) to an existing LIP to signal support.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := &types.MsgStakeLIP{
				Staker: clientCtx.GetFromAddress().String(),
				LipId:  args[0],
				Amount: args[1],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewAdvanceLIPStageCmd creates a CLI command for MsgAdvanceLIPStage.
func NewAdvanceLIPStageCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "advance-lip-stage [lip-id]",
		Short: "Advance a LIP to its next governance stage",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := &types.MsgAdvanceLIPStage{
				Authority: clientCtx.GetFromAddress().String(),
				LipId:     args[0],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewCastVoteCmd creates a CLI command for MsgCastVote.
func NewCastVoteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cast-vote [lip-id] [option]",
		Short: "Cast a stake-weighted vote on a LIP (yes, no, abstain)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := &types.MsgCastVote{
				Voter:  clientCtx.GetFromAddress().String(),
				LipId:  args[0],
				Option: args[1],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewWithdrawLIPCmd creates a CLI command for MsgWithdrawLIP.
func NewWithdrawLIPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "withdraw-lip [lip-id]",
		Short: "Withdraw a LIP (proposer only, before voting begins)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := &types.MsgWithdrawLIP{
				Proposer: clientCtx.GetFromAddress().String(),
				LipId:    args[0],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewSubmitResearchSpendCmd creates a CLI command for MsgSubmitResearchSpend.
func NewSubmitResearchSpendCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "submit-research-spend [title] [description] [recipient] [amount] [justification]",
		Short: "Submit a research fund spend proposal (designated voter only)",
		Long:  "Submit a 2-of-2 research fund spend proposal. Only designated voters can submit. Amount is in uzrn.",
		Args:  cobra.ExactArgs(5),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := &types.MsgSubmitResearchSpend{
				Proposer:      clientCtx.GetFromAddress().String(),
				Title:         args[0],
				Description:   args[1],
				Recipient:     args[2],
				Amount:        args[3],
				Justification: args[4],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewVoteResearchSpendCmd creates a CLI command for MsgVoteResearchSpend.
func NewVoteResearchSpendCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vote-research-spend [proposal-id] [vote] [reasoning]",
		Short: "Vote on a research spend proposal (yes or no)",
		Long:  "Cast a vote on a research fund spend proposal. Vote must be 'yes' or 'no'. Only designated voters can vote.",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			proposalID, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid proposal-id: %w", err)
			}

			msg := &types.MsgVoteResearchSpend{
				Voter:      clientCtx.GetFromAddress().String(),
				ProposalId: proposalID,
				Vote:       args[1],
				Reasoning:  args[2],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewSetResearchVotersCmd creates a CLI command for MsgSetResearchVoters.
func NewSetResearchVotersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set-research-voters [voter1] [voter2]",
		Short: "Set the 2-of-2 research fund voter addresses (governance authority only)",
		Long:  "Configure the two designated voters for research fund spend proposals. Requires governance authority.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := &types.MsgSetResearchVoters{
				Authority: clientCtx.GetFromAddress().String(),
				Voters: &types.ResearchFundVoters{
					Voter1: args[0],
					Voter2: args[1],
				},
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewNominateSeatElectionCmd creates a CLI command for MsgNominateSeatElection.
func NewNominateSeatElectionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "nominate-research-seat",
		Short: "Nominate a candidate for a research fund community seat",
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			candidate, _ := cmd.Flags().GetString("candidate")
			seatIndex, _ := cmd.Flags().GetUint32("seat-index")
			statement, _ := cmd.Flags().GetString("statement")

			msg := &types.MsgNominateSeatElection{
				Proposer:  clientCtx.GetFromAddress().String(),
				Candidate: candidate,
				SeatIndex: seatIndex,
				Statement: statement,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	cmd.Flags().String("candidate", "", "Candidate bech32 address")
	cmd.Flags().Uint32("seat-index", 0, "Seat index (0 for Phase 1; 0-2 for Phase 2)")
	cmd.Flags().String("statement", "", "Candidate's governance statement (max 2000 chars)")
	_ = cmd.MarkFlagRequired("candidate")
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewAcceptSeatNominationCmd creates a CLI command for MsgAcceptSeatNomination.
func NewAcceptSeatNominationCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "accept-research-nomination",
		Short: "Accept a pending seat election nomination",
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			proposalID, _ := cmd.Flags().GetUint64("proposal-id")

			msg := &types.MsgAcceptSeatNomination{
				Candidate:  clientCtx.GetFromAddress().String(),
				ProposalId: proposalID,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	cmd.Flags().Uint64("proposal-id", 0, "Seat election proposal ID")
	_ = cmd.MarkFlagRequired("proposal-id")
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewVoteSeatElectionCmd creates a CLI command for MsgVoteSeatElection.
func NewVoteSeatElectionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vote-seat-election",
		Short: "Cast a vote on a seat election",
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			proposalID, _ := cmd.Flags().GetUint64("proposal-id")
			option, _ := cmd.Flags().GetString("option")

			msg := &types.MsgVoteSeatElection{
				Voter:      clientCtx.GetFromAddress().String(),
				ProposalId: proposalID,
				Option:     option,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	cmd.Flags().Uint64("proposal-id", 0, "Seat election proposal ID")
	cmd.Flags().String("option", "", "Vote option: yes, no, abstain")
	_ = cmd.MarkFlagRequired("proposal-id")
	_ = cmd.MarkFlagRequired("option")
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewProposePhaseTransitionCmd creates a CLI command to propose a phase transition.
// Internally submits a LIP with category "research_phase_transition".
func NewProposePhaseTransitionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "propose-phase-transition",
		Short: "Propose a research fund governance phase transition",
		Long:  "Propose advancing the research fund to the next governance phase. Requires exit conditions to be met. Submits a LIP with supermajority (66.7%) voting threshold and 7-day activation delay.",
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			targetPhase, _ := cmd.Flags().GetUint32("target-phase")
			justification, _ := cmd.Flags().GetString("justification")
			amount, _ := cmd.Flags().GetString("amount")

			// Encode target phase as JSON in the description field.
			description := fmt.Sprintf(`{"target_phase":%d}`, targetPhase)

			msg := &types.MsgSubmitLIP{
				Proposer:     clientCtx.GetFromAddress().String(),
				Title:        justification,
				Description:  description,
				Category:     types.CategoryPhaseTransition,
				InitialStake: amount,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	cmd.Flags().Uint32("target-phase", 0, "Target phase number (must be current + 1)")
	cmd.Flags().String("justification", "", "Justification for why the community is ready")
	cmd.Flags().String("amount", "", "Stake amount in uzrn")
	_ = cmd.MarkFlagRequired("target-phase")
	_ = cmd.MarkFlagRequired("justification")
	_ = cmd.MarkFlagRequired("amount")
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewProposePhaseRollbackCmd creates a CLI command to propose a phase rollback.
// Internally submits a LIP with category "research_phase_rollback".
func NewProposePhaseRollbackCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "propose-phase-rollback",
		Short: "Propose a research fund governance phase rollback",
		Long:  "Propose rolling back the research fund to the previous governance phase. Requires gridlock (3+ expired proposals) or emergency halt evidence.",
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			justification, _ := cmd.Flags().GetString("justification")
			amount, _ := cmd.Flags().GetString("amount")

			msg := &types.MsgSubmitLIP{
				Proposer:     clientCtx.GetFromAddress().String(),
				Title:        justification,
				Description:  "Phase rollback proposal",
				Category:     types.CategoryPhaseRollback,
				InitialStake: amount,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	cmd.Flags().String("justification", "", "Justification for rollback (gridlock or emergency halt)")
	cmd.Flags().String("amount", "", "Stake amount in uzrn")
	_ = cmd.MarkFlagRequired("justification")
	_ = cmd.MarkFlagRequired("amount")
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}
