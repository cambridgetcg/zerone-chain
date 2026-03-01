package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"

	"github.com/zerone-chain/zerone/x/partnerships/types"
)

// NewQueryCmd returns the query commands for the partnerships module.
func NewQueryCmd() *cobra.Command {
	queryCmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "Partnerships module query commands",
		DisableFlagParsing:         false,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	queryCmd.AddCommand(
		NewQueryPartnershipCmd(),
		NewQueryByAddressCmd(),
		NewQueryPendingOpsCmd(),
		NewQueryFormationPoolCmd(),
		NewQueryParamsCmd(),
		NewQueryMentorshipCmd(),
		NewQueryMentorshipsByAddressCmd(),
		NewQueryFormationMatchesCmd(),
	)

	return queryCmd
}

// NewQueryPartnershipCmd returns the command to query a partnership by ID.
func NewQueryPartnershipCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "partnership [id]",
		Short: "Query a partnership by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			req := &types.QueryPartnershipRequest{Id: args[0]}
			resp := &types.QueryPartnershipResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.partnerships.v1.Query/Partnership", req, resp); err != nil {
				return fmt.Errorf("failed to query partnership: %w", err)
			}

			return clientCtx.PrintObjectLegacy(resp)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// NewQueryByAddressCmd returns the command to query partnerships by address.
func NewQueryByAddressCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "by-address [address]",
		Short: "Query partnerships by participant address",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			req := &types.QueryByAddressRequest{Address: args[0]}
			resp := &types.QueryByAddressResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.partnerships.v1.Query/PartnershipsByAddress", req, resp); err != nil {
				return fmt.Errorf("failed to query partnerships by address: %w", err)
			}

			return clientCtx.PrintObjectLegacy(resp)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// NewQueryPendingOpsCmd returns the command to query pending operations for a partnership.
func NewQueryPendingOpsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pending-ops [partnership-id]",
		Short: "Query pending consensus operations for a partnership",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			req := &types.QueryPendingOpsRequest{PartnershipId: args[0]}
			resp := &types.QueryPendingOpsResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.partnerships.v1.Query/PendingOps", req, resp); err != nil {
				return fmt.Errorf("failed to query pending operations: %w", err)
			}

			return clientCtx.PrintObjectLegacy(resp)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// NewQueryFormationPoolCmd returns the command to query the formation pool.
func NewQueryFormationPoolCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "formation-pool",
		Short: "Query the partnership formation pool",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			req := &types.QueryFormationPoolRequest{}
			resp := &types.QueryFormationPoolResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.partnerships.v1.Query/FormationPool", req, resp); err != nil {
				return fmt.Errorf("failed to query formation pool: %w", err)
			}

			return clientCtx.PrintObjectLegacy(resp)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// NewQueryParamsCmd returns the command to query partnerships module params.
func NewQueryParamsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "params",
		Short: "Query the partnerships module parameters",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			req := &types.QueryParamsRequest{}
			resp := &types.QueryParamsResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.partnerships.v1.Query/Params", req, resp); err != nil {
				return fmt.Errorf("failed to query params: %w", err)
			}

			return clientCtx.PrintObjectLegacy(resp)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// NewQueryMentorshipCmd returns the command to query a mentorship by ID.
func NewQueryMentorshipCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mentorship [id]",
		Short: "Query a mentorship by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			req := &types.QueryMentorshipRequest{Id: args[0]}
			resp := &types.QueryMentorshipResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.partnerships.v1.Query/Mentorship", req, resp); err != nil {
				return fmt.Errorf("failed to query mentorship: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// NewQueryMentorshipsByAddressCmd returns the command to query mentorships by address.
func NewQueryMentorshipsByAddressCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mentorships-by-address [address]",
		Short: "Query mentorships by participant address",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			req := &types.QueryMentorshipsByAddressRequest{Address: args[0]}
			resp := &types.QueryMentorshipsByAddressResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.partnerships.v1.Query/MentorshipsByAddress", req, resp); err != nil {
				return fmt.Errorf("failed to query mentorships: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// NewQueryFormationMatchesCmd returns the command to query all formation matches.
func NewQueryFormationMatchesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "formation-matches",
		Short: "Query all formation matches",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			req := &types.QueryFormationMatchesRequest{}
			resp := &types.QueryFormationMatchesResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.partnerships.v1.Query/FormationMatches", req, resp); err != nil {
				return fmt.Errorf("failed to query formation matches: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}
