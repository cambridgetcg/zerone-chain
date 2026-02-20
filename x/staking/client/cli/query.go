package cli

import (
	"fmt"
	"strconv"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/spf13/cobra"

	"github.com/zerone-chain/zerone/x/staking/types"
)

// GetQueryCmd returns the parent query command for the staking module.
func GetQueryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "Querying commands for the zerone staking module",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		CmdQueryParams(),
		CmdQueryValidator(),
		CmdQueryValidators(),
		CmdQueryDelegation(),
		CmdQueryDelegationsForValidator(),
		CmdQueryUnbondings(),
		CmdQueryTierConfig(),
	)
	return cmd
}

// CmdQueryParams queries module parameters.
func CmdQueryParams() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "params",
		Short: "Query staking module parameters",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)
			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.Params(cmd.Context(), &types.QueryParamsRequest{})
			if err != nil {
				return err
			}
			return clientCtx.PrintObjectLegacy(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// CmdQueryValidator queries a single validator.
func CmdQueryValidator() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validator [address]",
		Short: "Query a validator by operator address",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)
			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.Validator(cmd.Context(), &types.QueryValidatorRequest{Address: args[0]})
			if err != nil {
				return err
			}
			return clientCtx.PrintObjectLegacy(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// CmdQueryValidators queries all validators with optional filters.
func CmdQueryValidators() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validators",
		Short: "Query all validators",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)
			queryClient := types.NewQueryClient(clientCtx)

			activeOnly, _ := cmd.Flags().GetBool("active-only")
			tier, _ := cmd.Flags().GetInt32("tier")
			limit, _ := cmd.Flags().GetUint64("limit")
			offset, _ := cmd.Flags().GetUint64("offset")

			res, err := queryClient.Validators(cmd.Context(), &types.QueryValidatorsRequest{
				ActiveOnly: activeOnly,
				Tier:       tier,
				Limit:      limit,
				Offset:     offset,
			})
			if err != nil {
				return err
			}
			return clientCtx.PrintObjectLegacy(res)
		},
	}

	cmd.Flags().Bool("active-only", false, "Only show active validators")
	cmd.Flags().Int32("tier", -1, "Filter by tier (-1 = all)")
	cmd.Flags().Uint64("limit", 100, "Maximum results")
	cmd.Flags().Uint64("offset", 0, "Pagination offset")
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// CmdQueryDelegation queries a single delegation.
func CmdQueryDelegation() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delegation [delegator] [validator]",
		Short: "Query a delegation",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)
			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.Delegation(cmd.Context(), &types.QueryDelegationRequest{
				Delegator: args[0],
				Validator: args[1],
			})
			if err != nil {
				return err
			}
			return clientCtx.PrintObjectLegacy(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// CmdQueryDelegationsForValidator queries all delegations for a validator.
func CmdQueryDelegationsForValidator() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delegations-for-validator [validator]",
		Short: "Query all delegations for a validator",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)
			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.ValidatorDelegations(cmd.Context(), &types.QueryValidatorDelegationsRequest{
				Validator: args[0],
			})
			if err != nil {
				return err
			}
			return clientCtx.PrintObjectLegacy(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// CmdQueryUnbondings queries unbonding entries for a delegator.
func CmdQueryUnbondings() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unbondings [delegator]",
		Short: "Query unbonding entries for a delegator",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)
			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.DelegatorDelegations(cmd.Context(), &types.QueryDelegatorDelegationsRequest{
				Delegator: args[0],
			})
			if err != nil {
				return err
			}
			return clientCtx.PrintObjectLegacy(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// CmdQueryTierConfig queries a specific tier configuration.
func CmdQueryTierConfig() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tier [tier-number]",
		Short: "Query tier configuration (1=Apprentice, 2=Verified, 3=Scholar, 4=Guardian)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)
			queryClient := types.NewQueryClient(clientCtx)

			tier, err := strconv.ParseUint(args[0], 10, 32)
			if err != nil {
				return fmt.Errorf("invalid tier number: %s", args[0])
			}

			res, err := queryClient.TierConfig(cmd.Context(), &types.QueryTierConfigRequest{
				Tier: uint32(tier),
			})
			if err != nil {
				return err
			}
			return clientCtx.PrintObjectLegacy(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}
