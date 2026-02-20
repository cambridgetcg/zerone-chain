package cli

import (
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/spf13/cobra"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// GetTxCmd returns the root transaction command for the knowledge module.
func GetTxCmd() *cobra.Command {
	txCmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "Zerone knowledge transaction subcommands",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}
	// Sub-commands are added in R2-3 (CLI implementation).
	return txCmd
}
