package cli

import (
	"encoding/json"
	"os"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/spf13/cobra"

	"github.com/zerone-chain/zerone/x/substrate_bridge/types"
)

func GetTxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   types.ModuleName,
		Short: "substrate_bridge transactions",
	}
	cmd.AddCommand(cmdSubmitExternalAttestation())
	return cmd
}

func cmdSubmitExternalAttestation() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "submit-attestation [adapter-id] [work-class-id] [link-json-file] [bond-uzrn]",
		Short: "Submit an external attestation. link-json-file is a JSON-encoded SubstrateLink.",
		Args:  cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			cctx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			// Read SubstrateLink from JSON file.
			var link types.SubstrateLink
			if err := readJSONFile(args[2], &link); err != nil {
				return err
			}
			msg := &types.MsgSubmitExternalAttestation{
				Submitter:   cctx.GetFromAddress().String(),
				AdapterId:   args[0],
				WorkClassId: args[1],
				Link:        &link,
				BondUzrn:    args[3],
			}
			return tx.GenerateOrBroadcastTxCLI(cctx, cmd.Flags(), msg)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// readJSONFile reads a JSON file and unmarshals it into v.
func readJSONFile(path string, v interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}
