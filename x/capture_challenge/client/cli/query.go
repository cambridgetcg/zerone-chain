package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"

	"github.com/zerone-chain/zerone/x/capture_challenge/types"
)

func NewQueryCmd() *cobra.Command {
	queryCmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "Capture challenge module query commands",
		DisableFlagParsing:         false,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	queryCmd.AddCommand(
		NewQueryParamsCmd(),
		NewQueryChallengeCmd(),
		NewQueryBountyPoolCmd(),
		NewQueryChallengesByDomainCmd(),
		NewQueryActiveChallengesCmd(),
	)

	return queryCmd
}

func NewQueryParamsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "params",
		Short: "Query the capture challenge module parameters",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			req := &types.QueryParamsRequest{}
			resp := &types.QueryParamsResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.capture_challenge.v1.Query/Params", req, resp); err != nil {
				return fmt.Errorf("failed to query params: %w", err)
			}

			return clientCtx.PrintObjectLegacy(resp)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func NewQueryChallengeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "challenge [id]",
		Short: "Query a capture challenge by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			req := &types.QueryChallengeRequest{Id: args[0]}
			resp := &types.QueryChallengeResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.capture_challenge.v1.Query/Challenge", req, resp); err != nil {
				return fmt.Errorf("failed to query challenge: %w", err)
			}

			return clientCtx.PrintObjectLegacy(resp)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func NewQueryBountyPoolCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bounty-pool [domain]",
		Short: "Query a domain's bounty pool balance",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			req := &types.QueryBountyPoolRequest{Domain: args[0]}
			resp := &types.QueryBountyPoolResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.capture_challenge.v1.Query/BountyPool", req, resp); err != nil {
				return fmt.Errorf("failed to query bounty pool: %w", err)
			}

			return clientCtx.PrintObjectLegacy(resp)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func NewQueryActiveChallengesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "active",
		Short: "Query all active capture challenges",
		Long:  "Query all capture challenges with an active (non-terminal) status: OPEN, EVIDENCE, or UNDER_REVIEW.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			req := &types.QueryActiveChallengesRequest{}
			resp := &types.QueryActiveChallengesResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.capture_challenge.v1.Query/ActiveChallenges", req, resp); err != nil {
				return fmt.Errorf("failed to query active challenges: %w", err)
			}

			return clientCtx.PrintObjectLegacy(resp)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func NewQueryChallengesByDomainCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "challenges-by-domain [domain]",
		Short: "Query all challenges for a domain",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			req := &types.QueryChallengesByDomainRequest{Domain: args[0]}
			resp := &types.QueryChallengesByDomainResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.capture_challenge.v1.Query/ChallengesByDomain", req, resp); err != nil {
				return fmt.Errorf("failed to query challenges: %w", err)
			}

			return clientCtx.PrintObjectLegacy(resp)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}
