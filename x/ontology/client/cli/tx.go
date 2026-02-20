package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"

	"github.com/zerone-chain/zerone/x/ontology/types"
)

// NewTxCmd returns the transaction commands for the ontology module.
func NewTxCmd() *cobra.Command {
	txCmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "Ontology module transaction commands",
		DisableFlagParsing:         false,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	txCmd.AddCommand(
		NewProposeDomainCmd(),
		NewVoteDomainProposalCmd(),
		NewUpdateDomainCmd(),
		NewRegisterLogicZoneCmd(),
		NewAcknowledgeIncompletenessCmd(),
	)

	return txCmd
}

// NewProposeDomainCmd creates a CLI command for MsgProposeDomain.
func NewProposeDomainCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "propose-domain [name] [display-name] [stratum] [stake]",
		Short: "Propose a new knowledge domain",
		Long: `Propose a new knowledge domain within a stratum.

Strata:
  0 - Axiomatic (mathematical axioms, tautologies)
  1 - Formal (formal proofs)
  2 - Protocol (blockchain-verifiable)
  3 - Computational (computation results)
  4 - Empirical (scientific observations)
  5 - Historical (historical records)
  6 - Testimonial (human attestations)

Stake must meet the minimum proposal stake parameter.`,
		Args: cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			stratum, err := strconv.ParseUint(args[2], 10, 32)
			if err != nil {
				return fmt.Errorf("invalid stratum: %w", err)
			}

			description, _ := cmd.Flags().GetString("description")

			msg := &types.MsgProposeDomain{
				Name:        args[0],
				DisplayName: args[1],
				Description: description,
				Stratum:     uint32(stratum),
				Proposer:    clientCtx.GetFromAddress().String(),
				Stake:       args[3],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	cmd.Flags().String("description", "", "Domain description")
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewVoteDomainProposalCmd creates a CLI command for MsgVoteDomainProposal.
func NewVoteDomainProposalCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vote-proposal [proposal-id] [approve]",
		Short: "Vote on a domain proposal",
		Long: `Vote on an active domain proposal.

approve: true or false`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			approve, err := strconv.ParseBool(args[1])
			if err != nil {
				return fmt.Errorf("invalid approve value: must be 'true' or 'false'")
			}

			msg := &types.MsgVoteDomainProposal{
				ProposalId: args[0],
				Voter:      clientCtx.GetFromAddress().String(),
				Approve:    approve,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewUpdateDomainCmd creates a CLI command for MsgUpdateDomain.
func NewUpdateDomainCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update-domain [domain-name]",
		Short: "Update domain metadata (authority only)",
		Long: `Update domain metadata. Only the governance authority can update domains.

Use flags to specify which fields to update.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			displayName, _ := cmd.Flags().GetString("display-name")
			description, _ := cmd.Flags().GetString("description")
			status, _ := cmd.Flags().GetString("status")

			msg := &types.MsgUpdateDomain{
				Authority:   clientCtx.GetFromAddress().String(),
				DomainName:  args[0],
				DisplayName: displayName,
				Description: description,
				Status:      status,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	cmd.Flags().String("display-name", "", "New display name")
	cmd.Flags().String("description", "", "New description")
	cmd.Flags().String("status", "", "New status (active, deprecated)")
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewRegisterLogicZoneCmd creates a CLI command for MsgRegisterLogicZone.
func NewRegisterLogicZoneCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "register-logic-zone [zone-name] [max-confidence-bps]",
		Short: "Register a new logic zone (authority only)",
		Long: `Register a new logic zone with its formal properties.

Built-in zones: propositional, presburger, peano, set_theory, empirical.
Max confidence in basis points (1000000 = 100%).`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			maxConf, err := strconv.ParseUint(args[1], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid max confidence: %w", err)
			}

			complete, _ := cmd.Flags().GetBool("complete")
			decidable, _ := cmd.Flags().GetBool("decidable")
			goedelApplies, _ := cmd.Flags().GetBool("goedel-applies")
			description, _ := cmd.Flags().GetString("description")

			msg := &types.MsgRegisterLogicZone{
				Authority: clientCtx.GetFromAddress().String(),
				ZoneProperties: &types.LogicZoneProperties{
					Zone:             args[0],
					Complete:         complete,
					Decidable:        decidable,
					GoedelApplies:    goedelApplies,
					MaxConfidenceBps: maxConf,
					Description:      description,
				},
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	cmd.Flags().Bool("complete", false, "Is the system complete?")
	cmd.Flags().Bool("decidable", false, "Is there a decision procedure?")
	cmd.Flags().Bool("goedel-applies", false, "Does Godel's incompleteness apply?")
	cmd.Flags().String("description", "", "Zone description")
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewAcknowledgeIncompletenessCmd creates a CLI command for MsgAcknowledgeIncompleteness.
func NewAcknowledgeIncompletenessCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "acknowledge-incompleteness [fact-id] [zone] [reason]",
		Short: "Record a Godelian incompleteness acknowledgment",
		Long: `Acknowledge that a fact in an incomplete logic zone has epistemic
limitations. The zone must have GoedelApplies=true.`,
		Args: cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := &types.MsgAcknowledgeIncompleteness{
				Submitter: clientCtx.GetFromAddress().String(),
				FactId:    args[0],
				Zone:      args[1],
				Reason:    args[2],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}
