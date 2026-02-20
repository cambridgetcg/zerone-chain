package cli

import (
	"fmt"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/spf13/cobra"

	"github.com/zerone-chain/zerone/x/auth/types"
)

// GetQueryCmd returns the query commands for this module.
func GetQueryCmd() *cobra.Command {
	queryCmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "Querying commands for the zerone auth module",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	queryCmd.AddCommand(
		CmdQueryParams(),
		CmdQueryAccount(),
		CmdQueryAccountByDID(),
		CmdQuerySessionKeys(),
		CmdQueryFrozenAccounts(),
	)

	return queryCmd
}

// CmdQueryParams queries module parameters.
func CmdQueryParams() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "params",
		Short: "Query the current auth module parameters",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.Params(cmd.Context(), &types.QueryParamsRequest{})
			if err != nil {
				return fmt.Errorf("failed to query params: %w", err)
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// CmdQueryAccount queries an account by address.
func CmdQueryAccount() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "account [address]",
		Short: "Query a Zerone account by bech32 address",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.Account(cmd.Context(), &types.QueryAccountRequest{Address: args[0]})
			if err != nil {
				return fmt.Errorf("failed to query account: %w", err)
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// CmdQueryAccountByDID queries an account by DID.
func CmdQueryAccountByDID() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "account-by-did [did]",
		Short: "Query a Zerone account by DID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.AccountByDID(cmd.Context(), &types.QueryAccountByDIDRequest{Did: args[0]})
			if err != nil {
				return fmt.Errorf("failed to query account by DID: %w", err)
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// CmdQuerySessionKeys queries session keys for an owner.
func CmdQuerySessionKeys() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "session-keys [owner-address]",
		Short: "Query all session keys for an owner",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.SessionKeys(cmd.Context(), &types.QuerySessionKeysRequest{Owner: args[0]})
			if err != nil {
				return fmt.Errorf("failed to query session keys: %w", err)
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// CmdQueryFrozenAccounts queries all frozen accounts.
func CmdQueryFrozenAccounts() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "frozen-accounts",
		Short: "Query all frozen accounts",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.FrozenAccounts(cmd.Context(), &types.QueryFrozenAccountsRequest{})
			if err != nil {
				return fmt.Errorf("failed to query frozen accounts: %w", err)
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}
