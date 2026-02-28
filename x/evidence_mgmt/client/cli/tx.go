package cli

import (
	"fmt"
	"math/big"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"

	"github.com/zerone-chain/zerone/x/evidence_mgmt/types"
)

func NewTxCmd() *cobra.Command {
	txCmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "Evidence management module transaction commands",
		DisableFlagParsing:         false,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}
	txCmd.AddCommand(
		NewSubmitEvidenceCmd(),
		NewTransferCustodyCmd(),
		NewVerifyEvidenceCmd(),
		NewChallengeEvidenceCmd(),
	)
	return txCmd
}

func NewSubmitEvidenceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "submit [evidence-type] [content-hash] [metadata]",
		Short: "Submit new evidence (type: 1=document, 2=attestation, 3=measurement, 4=computation)",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			evidenceType := types.EvidenceType_EVIDENCE_TYPE_DOCUMENT
			switch args[0] {
			case "1", "document":
				evidenceType = types.EvidenceType_EVIDENCE_TYPE_DOCUMENT
			case "2", "attestation":
				evidenceType = types.EvidenceType_EVIDENCE_TYPE_ATTESTATION
			case "3", "measurement":
				evidenceType = types.EvidenceType_EVIDENCE_TYPE_MEASUREMENT
			case "4", "computation":
				evidenceType = types.EvidenceType_EVIDENCE_TYPE_COMPUTATION
			}

			msg := &types.MsgSubmitEvidence{
				Submitter:    clientCtx.GetFromAddress().String(),
				EvidenceType: evidenceType,
				ContentHash:  args[1],
				Metadata:     args[2],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

func NewTransferCustodyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "transfer-custody [evidence-id] [new-custodian]",
		Short: "Transfer custody of evidence to a new custodian",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			notes, _ := cmd.Flags().GetString("notes")

			msg := &types.MsgTransferCustody{
				CurrentCustodian: clientCtx.GetFromAddress().String(),
				EvidenceId:       args[0],
				NewCustodian:     args[1],
				Notes:            notes,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}
	cmd.Flags().String("notes", "", "Optional transfer notes")
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

func NewVerifyEvidenceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "verify [evidence-id] [outcome] [confidence] [method]",
		Short: "Verify evidence (outcome: true/false, confidence: 0-1000000)",
		Args:  cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			outcome, err := strconv.ParseBool(args[1])
			if err != nil {
				return fmt.Errorf("invalid outcome %q: must be true or false", args[1])
			}

			confidence, err := strconv.ParseUint(args[2], 10, 32)
			if err != nil {
				return fmt.Errorf("invalid confidence %q: must be uint32", args[2])
			}
			if confidence > 1000000 {
				return fmt.Errorf("confidence must be <= 1000000")
			}

			msg := &types.MsgVerifyEvidence{
				Verifier:   clientCtx.GetFromAddress().String(),
				EvidenceId: args[0],
				Outcome:    outcome,
				Confidence: uint32(confidence),
				Method:     args[3],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

func NewChallengeEvidenceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "challenge [evidence-id] [reason] [bond]",
		Short: "Challenge evidence with a bond (bond in uzrn)",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			bond := new(big.Int)
			if _, ok := bond.SetString(args[2], 10); !ok || bond.Sign() <= 0 {
				return fmt.Errorf("bond must be a positive integer")
			}

			msg := &types.MsgChallengeEvidence{
				Challenger: clientCtx.GetFromAddress().String(),
				EvidenceId: args[0],
				Reason:     args[1],
				Bond:       args[2],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}
