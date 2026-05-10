package cli

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"

	"github.com/zerone-chain/zerone/x/claiming_pot/types"
)

func NewTxCmd() *cobra.Command {
	txCmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "Claiming pot module transaction commands",
		DisableFlagParsing:         false,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}
	txCmd.AddCommand(
		NewClaimCmd(),
		NewAddBootstrapEntryCmd(),
	)
	return txCmd
}

func NewClaimCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "claim [pot-id]",
		Short: "Claim tokens from a claiming pot",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := &types.MsgClaim{
				Claimant: clientCtx.GetFromAddress().String(),
				PotId:    args[0],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewAddBootstrapEntryCmd builds a tx that admits one or more late
// bootstrap entries. Authority-gated — the --from address must equal
// the module's GetAuthority() (the governance account on mainnet).
//
// Direct CLI use is intended for testnet operators. On mainnet, the
// message is wrapped in a governance LIP so admission is deliberate
// and audited (commitment 20: continuously, governance-gated).
func NewAddBootstrapEntryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add-bootstrap-entry [addr1,addr2,...]",
		Short: "(governance) Admit one or more late bootstrap entries",
		Long: `Admit late participants by creating one bootstrap pot per address. Each
address gets a single-claimant pot (0.222 ZRN, instant vest). Duplicates
are silently skipped. Authority-gated — the --from address must equal
the claiming_pot module authority (governance on mainnet).

Direct CLI use is for testnet operators. Mainnet must wrap this message
in a governance LIP so admission is deliberate and audited.

Example:
  zeroned tx claiming_pot add-bootstrap-entry zrn1abc...,zrn1def... --from authority`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			raw := strings.Split(args[0], ",")
			addresses := make([]string, 0, len(raw))
			for _, r := range raw {
				if s := strings.TrimSpace(r); s != "" {
					addresses = append(addresses, s)
				}
			}
			msg := &types.MsgAddBootstrapEntry{
				Authority: clientCtx.GetFromAddress().String(),
				Addresses: addresses,
			}
			if err := msg.ValidateBasic(); err != nil {
				return err
			}
			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}
