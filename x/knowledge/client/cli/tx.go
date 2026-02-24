package cli

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// parseClaimType maps a CLI string to a ClaimType enum value.
func parseClaimType(s string) (types.ClaimType, error) {
	switch strings.ToLower(s) {
	case "", "assertion":
		return types.ClaimType_CLAIM_TYPE_ASSERTION, nil
	case "relation":
		return types.ClaimType_CLAIM_TYPE_RELATION, nil
	case "definition":
		return types.ClaimType_CLAIM_TYPE_DEFINITION, nil
	case "constraint":
		return types.ClaimType_CLAIM_TYPE_CONSTRAINT, nil
	case "negation":
		return types.ClaimType_CLAIM_TYPE_NEGATION, nil
	case "observation":
		return types.ClaimType_CLAIM_TYPE_OBSERVATION, nil
	default:
		return 0, fmt.Errorf("unknown claim type %q: must be assertion, relation, definition, constraint, negation, or observation", s)
	}
}

// parseRelationType maps a CLI string to a RelationType enum value.
func parseRelationType(s string) (types.RelationType, error) {
	switch strings.ToLower(s) {
	case "supports":
		return types.RelationType_RELATION_TYPE_SUPPORTS, nil
	case "contradicts":
		return types.RelationType_RELATION_TYPE_CONTRADICTS, nil
	case "requires":
		return types.RelationType_RELATION_TYPE_REQUIRES, nil
	case "refines":
		return types.RelationType_RELATION_TYPE_REFINES, nil
	case "generalizes":
		return types.RelationType_RELATION_TYPE_GENERALIZES, nil
	case "supersedes":
		return types.RelationType_RELATION_TYPE_SUPERSEDES, nil
	default:
		return 0, fmt.Errorf("unknown relation type %q: must be supports, contradicts, requires, refines, generalizes, or supersedes", s)
	}
}

// GetTxCmd returns the root transaction command for the knowledge module.
func GetTxCmd() *cobra.Command {
	txCmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "Knowledge module transaction commands",
		DisableFlagParsing:         false,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	txCmd.AddCommand(
		NewSubmitClaimCmd(),
		NewSubmitCommitmentCmd(),
		NewSubmitRevealCmd(),
		NewChallengeFactCmd(),
		NewAddFactCmd(),
		NewSubmitContradictionCmd(),
		NewPatronizeFactCmd(),
		NewProposeDomainCmd(),
		NewEndorseDomainProposalCmd(),
		NewChallengeDomainProposalCmd(),
		NewRegisterStratumCmd(),
		NewChallengeProvisionalFactCmd(),
		NewProposeResearchFundCmd(),
		NewVoteResearchProposalCmd(),
		NewExecuteResearchProposalCmd(),
	)

	return txCmd
}

// NewSubmitClaimCmd creates a CLI command for MsgSubmitClaim.
func NewSubmitClaimCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "submit-claim [fact-content] [domain] [category] [stake]",
		Short: "Submit a knowledge claim for verification",
		Args:  cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			refs, _ := cmd.Flags().GetString("references")
			var references []string
			if refs != "" {
				references = strings.Split(refs, ",")
			}

			partnershipId, _ := cmd.Flags().GetString("partnership-id")

			claimTypeStr, _ := cmd.Flags().GetString("claim-type")
			claimType, err := parseClaimType(claimTypeStr)
			if err != nil {
				return err
			}

			// Parse typed relations
			relationsStr, _ := cmd.Flags().GetString("relations")
			var relations []*types.ClaimRelation
			if relationsStr != "" {
				for _, pair := range strings.Split(relationsStr, ",") {
					parts := strings.SplitN(pair, ":", 2)
					if len(parts) != 2 {
						return fmt.Errorf("invalid relation format %q: expected type:fact_id", pair)
					}
					relType, err := parseRelationType(parts[0])
					if err != nil {
						return err
					}
					relations = append(relations, &types.ClaimRelation{
						TargetFactId: parts[1],
						Relation:     relType,
					})
				}
			}

			msg := &types.MsgSubmitClaim{
				Submitter:     clientCtx.GetFromAddress().String(),
				FactContent:   args[0],
				Domain:        args[1],
				Category:      args[2],
				Stake:         args[3],
				References:    references,
				PartnershipId: partnershipId,
				ClaimType:     claimType,
				Relations:     relations,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	cmd.Flags().String("references", "", "Comma-separated fact IDs to reference")
	cmd.Flags().String("partnership-id", "", "Partnership ID for collaborative claims")
	cmd.Flags().String("claim-type", "assertion", "Claim type: assertion (default), relation, definition, constraint, negation, observation")
	cmd.Flags().String("relations", "", "Typed relations: supports:FACT_ID,contradicts:FACT_ID,requires:FACT_ID")
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewSubmitCommitmentCmd creates a CLI command for MsgSubmitCommitment.
func NewSubmitCommitmentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "submit-commitment [round-id] [commit-hash-hex]",
		Short: "Submit a verification commitment (commit-reveal phase 1)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			commitHash, err := hex.DecodeString(args[1])
			if err != nil {
				return err
			}

			msg := &types.MsgSubmitCommitment{
				Verifier:   clientCtx.GetFromAddress().String(),
				RoundId:    args[0],
				CommitHash: commitHash,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewSubmitRevealCmd creates a CLI command for MsgSubmitReveal.
func NewSubmitRevealCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "submit-reveal [round-id] [vote] [salt-hex]",
		Short: "Submit a verification reveal (commit-reveal phase 2)",
		Long:  `Vote must be "accept", "reject", or "malformed". Salt is the hex-encoded salt used in commitment.`,
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			salt, err := hex.DecodeString(args[2])
			if err != nil {
				return err
			}

			msg := &types.MsgSubmitReveal{
				Verifier: clientCtx.GetFromAddress().String(),
				RoundId:  args[0],
				Vote:     args[1],
				Salt:     salt,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewChallengeFactCmd creates a CLI command for MsgChallengeFact.
func NewChallengeFactCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "challenge-fact [fact-id] [stake] [reason]",
		Short: "Challenge an existing fact",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			evidenceStr, _ := cmd.Flags().GetString("evidence-ids")
			var evidenceIds []string
			if evidenceStr != "" {
				evidenceIds = strings.Split(evidenceStr, ",")
			}

			msg := &types.MsgChallengeFact{
				Challenger:  clientCtx.GetFromAddress().String(),
				FactId:      args[0],
				Stake:       args[1],
				Reason:      args[2],
				EvidenceIds: evidenceIds,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	cmd.Flags().String("evidence-ids", "", "Comma-separated evidence IDs")
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewAddFactCmd creates a CLI command for MsgAddFact (authority only).
func NewAddFactCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add-fact [content] [domain] [category] [confidence]",
		Short: "Add a fact directly (authority only)",
		Args:  cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			confidence, err := strconv.ParseUint(args[3], 10, 64)
			if err != nil {
				return err
			}

			refsStr, _ := cmd.Flags().GetString("references")
			var references []string
			if refsStr != "" {
				references = strings.Split(refsStr, ",")
			}

			msg := &types.MsgAddFact{
				Authority:  clientCtx.GetFromAddress().String(),
				Content:    args[0],
				Domain:     args[1],
				Category:   args[2],
				Confidence: confidence,
				References: references,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	cmd.Flags().String("references", "", "Comma-separated fact IDs to reference")
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewSubmitContradictionCmd creates a CLI command for MsgSubmitContradiction.
func NewSubmitContradictionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "submit-contradiction [fact-id] [counter-claim] [stake] [reason]",
		Short: "Submit a contradiction to an existing fact",
		Args:  cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			domain, _ := cmd.Flags().GetString("domain")
			category, _ := cmd.Flags().GetString("category")
			evidenceStr, _ := cmd.Flags().GetString("evidence-ids")
			var evidenceIds []string
			if evidenceStr != "" {
				evidenceIds = strings.Split(evidenceStr, ",")
			}

			msg := &types.MsgSubmitContradiction{
				Submitter:   clientCtx.GetFromAddress().String(),
				FactId:      args[0],
				CounterClaim: args[1],
				Stake:       args[2],
				Reason:      args[3],
				Domain:      domain,
				Category:    category,
				EvidenceIds: evidenceIds,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	cmd.Flags().String("domain", "", "Knowledge domain")
	cmd.Flags().String("category", "", "Epistemic category")
	cmd.Flags().String("evidence-ids", "", "Comma-separated evidence IDs")
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewPatronizeFactCmd creates a CLI command for MsgPatronizeFact.
func NewPatronizeFactCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "patronize-fact [fact-id] [amount] [duration-blocks]",
		Short: "Patronize a fact with staking support",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			durationBlocks, err := strconv.ParseUint(args[2], 10, 64)
			if err != nil {
				return err
			}

			msg := &types.MsgPatronizeFact{
				Patron:         clientCtx.GetFromAddress().String(),
				FactId:         args[0],
				Amount:         args[1],
				DurationBlocks: durationBlocks,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewProposeDomainCmd creates a CLI command for MsgProposeDomain.
func NewProposeDomainCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "propose-domain [name] [description] [stratum] [stake]",
		Short: "Propose a new knowledge domain",
		Args:  cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := &types.MsgProposeDomain{
				Proposer:    clientCtx.GetFromAddress().String(),
				Name:        args[0],
				Description: args[1],
				Stratum:     args[2],
				Stake:       args[3],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewEndorseDomainProposalCmd creates a CLI command for MsgEndorseDomainProposal.
func NewEndorseDomainProposalCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "endorse-domain [proposal-id]",
		Short: "Endorse a domain proposal",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := &types.MsgEndorseDomainProposal{
				Endorser:   clientCtx.GetFromAddress().String(),
				ProposalId: args[0],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewChallengeDomainProposalCmd creates a CLI command for MsgChallengeDomainProposal.
func NewChallengeDomainProposalCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "challenge-domain [proposal-id] [reason]",
		Short: "Challenge a domain proposal",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := &types.MsgChallengeDomainProposal{
				Challenger: clientCtx.GetFromAddress().String(),
				ProposalId: args[0],
				Reason:     args[1],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewRegisterStratumCmd creates a CLI command for MsgRegisterStratum.
func NewRegisterStratumCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "register-stratum [name] [description] [confidence-ceiling]",
		Short: "Register a new epistemic stratum (authority only)",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			ceiling, err := strconv.ParseUint(args[2], 10, 64)
			if err != nil {
				return err
			}

			parentsStr, _ := cmd.Flags().GetString("parent-strata")
			var parents []string
			if parentsStr != "" {
				parents = strings.Split(parentsStr, ",")
			}

			msg := &types.MsgRegisterStratum{
				Authority:         clientCtx.GetFromAddress().String(),
				Name:              args[0],
				Description:       args[1],
				ConfidenceCeiling: ceiling,
				ParentStrata:      parents,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	cmd.Flags().String("parent-strata", "", "Comma-separated parent stratum names")
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewChallengeProvisionalFactCmd creates a CLI command for MsgChallengeProvisionalFact.
func NewChallengeProvisionalFactCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "challenge-provisional [claim-id] [fact-id] [stake] [reason]",
		Short: "Challenge a provisional fact during its grace period",
		Args:  cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			counterClaim, _ := cmd.Flags().GetString("counter-claim")
			evidenceStr, _ := cmd.Flags().GetString("evidence-ids")
			var evidenceIds []string
			if evidenceStr != "" {
				evidenceIds = strings.Split(evidenceStr, ",")
			}

			msg := &types.MsgChallengeProvisionalFact{
				Challenger:   clientCtx.GetFromAddress().String(),
				ClaimId:      args[0],
				FactId:       args[1],
				Stake:        args[2],
				Reason:       args[3],
				EvidenceIds:  evidenceIds,
				CounterClaim: counterClaim,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	cmd.Flags().String("counter-claim", "", "Counter-claim text")
	cmd.Flags().String("evidence-ids", "", "Comma-separated evidence IDs")
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewProposeResearchFundCmd creates a CLI command for MsgProposeResearchFund.
func NewProposeResearchFundCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "propose-research-fund [title] [description] [amount] [recipient] [voting-period-blocks]",
		Short: "Propose a research fund allocation",
		Args:  cobra.ExactArgs(5),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			votingPeriod, err := strconv.ParseUint(args[4], 10, 64)
			if err != nil {
				return err
			}

			msg := &types.MsgProposeResearchFund{
				Proposer:           clientCtx.GetFromAddress().String(),
				Title:              args[0],
				Description:        args[1],
				Amount:             args[2],
				Recipient:          args[3],
				VotingPeriodBlocks: votingPeriod,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewVoteResearchProposalCmd creates a CLI command for MsgVoteResearchProposal.
func NewVoteResearchProposalCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vote-research-proposal [proposal-id] [vote:true/false]",
		Short: "Vote on a research fund proposal",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			vote, err := strconv.ParseBool(args[1])
			if err != nil {
				return err
			}

			msg := &types.MsgVoteResearchProposal{
				Voter:      clientCtx.GetFromAddress().String(),
				ProposalId: args[0],
				Vote:       vote,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewExecuteResearchProposalCmd creates a CLI command for MsgExecuteResearchProposal.
func NewExecuteResearchProposalCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "execute-research-proposal [proposal-id]",
		Short: "Execute an approved research fund proposal (authority only)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := &types.MsgExecuteResearchProposal{
				Authority:  clientCtx.GetFromAddress().String(),
				ProposalId: args[0],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}
