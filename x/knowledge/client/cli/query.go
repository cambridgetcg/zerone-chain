package cli

import (
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/spf13/cobra"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// GetQueryCmd returns the root query command for the knowledge module.
func GetQueryCmd() *cobra.Command {
	queryCmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "Zerone knowledge query subcommands",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}
	// Sub-commands are added in R2-3 (CLI implementation).
	return queryCmd
}
