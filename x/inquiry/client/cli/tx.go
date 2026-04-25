package cli

import (
	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"

	"github.com/zerone-chain/zerone/x/inquiry/types"
)

const (
	flagContext      = "context"
	flagExpiryBlocks = "expiry-blocks"
)

func NewTxCmd() *cobra.Command {
	txCmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "Inquiry module transaction commands",
		DisableFlagParsing:         false,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}
	txCmd.AddCommand(
		NewSubmitInquiryCmd(),
		NewSubmitAnswerCmd(),
		NewResolveInquiryCmd(),
		NewCancelInquiryCmd(),
	)
	return txCmd
}

func NewSubmitInquiryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "submit-inquiry [question] [domain] [bounty]",
		Short: "Submit an open question with an escrowed bounty",
		Long: `Publish a question to the chain. The bounty (in uzrn) is
escrowed in the inquiry-bounty-pool module account; the first
answerer whose linked claim resolves to an accepted fact wins it.

Optional flags:
  --context        Additional context, hints, or constraints
  --expiry-blocks  Custom expiry window. Default: params.default_expiry_blocks`,
		Args: cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			ctxStr, _ := cmd.Flags().GetString(flagContext)
			expiry, _ := cmd.Flags().GetUint64(flagExpiryBlocks)

			msg := &types.MsgSubmitInquiry{
				Asker:        clientCtx.GetFromAddress().String(),
				Question:     args[0],
				Domain:       args[1],
				Bounty:       args[2],
				Context:      ctxStr,
				ExpiryBlocks: expiry,
			}
			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}
	cmd.Flags().String(flagContext, "", "Additional context for the question")
	cmd.Flags().Uint64(flagExpiryBlocks, 0, "Custom expiry window in blocks (0 = default)")
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

func NewSubmitAnswerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "submit-answer [inquiry-id] [claim-id]",
		Short: "Link a knowledge claim as an answer to an inquiry",
		Long: `Link a claim you have already submitted via x/knowledge as an
answer to an open inquiry. The claim must be owned by you. When
the claim's verification round produces an accepted Fact, the
inquiry resolves and pays you the bounty.

Multiple answers may be linked to the same inquiry; the first
whose claim accepts wins.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			msg := &types.MsgSubmitAnswer{
				Answerer:  clientCtx.GetFromAddress().String(),
				InquiryId: args[0],
				ClaimId:   args[1],
			}
			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

func NewResolveInquiryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resolve-inquiry [inquiry-id]",
		Short: "Manually resolve an inquiry (auto-runs in BeginBlocker too)",
		Long: `Force the resolver to check the inquiry's linked answers. If any
have produced an accepted fact, the inquiry resolves to RESOLVED
and pays the bounty. If past expiry with no accepted answer, it
resolves to EXPIRED and refunds the asker.

The BeginBlocker auto-resolves inquiries each block; this command
exists so anyone can poke a stale inquiry without waiting.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			msg := &types.MsgResolveInquiry{
				Caller:    clientCtx.GetFromAddress().String(),
				InquiryId: args[0],
			}
			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

func NewCancelInquiryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cancel-inquiry [inquiry-id]",
		Short: "Cancel an inquiry you submitted (only before any answer is linked)",
		Long: `Refund your bounty if no answer has been linked yet. Once any
agent has submitted an answer, you cannot cancel — their
verification work has already started.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			msg := &types.MsgCancelInquiry{
				Asker:     clientCtx.GetFromAddress().String(),
				InquiryId: args[0],
			}
			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}
