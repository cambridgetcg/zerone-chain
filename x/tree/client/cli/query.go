package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"

	"github.com/zerone-chain/zerone/x/tree/types"
)

// NewQueryCmd returns the query commands for the tree module.
func NewQueryCmd() *cobra.Command {
	queryCmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "Tree module query commands",
		DisableFlagParsing:         false,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	queryCmd.AddCommand(
		NewQueryProjectCmd(),
		NewQueryProjectsByFounderCmd(),
		NewQueryTaskCmd(),
		NewQueryTasksByProjectCmd(),
		NewQueryServiceCmd(),
		NewQuerySeedCmd(),
		NewQueryParamsCmd(),
	)

	return queryCmd
}

// NewQueryProjectCmd returns the command to query a project by ID.
func NewQueryProjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project [id]",
		Short: "Query a project by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			req := &types.QueryProjectRequest{ProjectId: args[0]}
			resp := &types.QueryProjectResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.tree.v1.Query/Project", req, resp); err != nil {
				return fmt.Errorf("failed to query project: %w", err)
			}

			return clientCtx.PrintObjectLegacy(resp)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// NewQueryProjectsByFounderCmd returns the command to list projects by founder.
func NewQueryProjectsByFounderCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "projects-by-founder [founder-address]",
		Short: "List projects created by a founder",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			req := &types.QueryProjectsByFounderRequest{Founder: args[0]}
			resp := &types.QueryProjectsByFounderResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.tree.v1.Query/ProjectsByFounder", req, resp); err != nil {
				return fmt.Errorf("failed to query projects by founder: %w", err)
			}

			return clientCtx.PrintObjectLegacy(resp)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// NewQueryTaskCmd returns the command to query a task by ID.
func NewQueryTaskCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "task [id]",
		Short: "Query a task by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			req := &types.QueryTaskRequest{TaskId: args[0]}
			resp := &types.QueryTaskResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.tree.v1.Query/Task", req, resp); err != nil {
				return fmt.Errorf("failed to query task: %w", err)
			}

			return clientCtx.PrintObjectLegacy(resp)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// NewQueryTasksByProjectCmd returns the command to list tasks by project ID.
func NewQueryTasksByProjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tasks-by-project [project-id]",
		Short: "List tasks for a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			req := &types.QueryTasksByProjectRequest{ProjectId: args[0]}
			resp := &types.QueryTasksByProjectResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.tree.v1.Query/TasksByProject", req, resp); err != nil {
				return fmt.Errorf("failed to query tasks by project: %w", err)
			}

			return clientCtx.PrintObjectLegacy(resp)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// NewQueryServiceCmd returns the command to query a service by ID.
func NewQueryServiceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service [id]",
		Short: "Query a service by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			req := &types.QueryServiceRequest{ServiceId: args[0]}
			resp := &types.QueryServiceResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.tree.v1.Query/Service", req, resp); err != nil {
				return fmt.Errorf("failed to query service: %w", err)
			}

			return clientCtx.PrintObjectLegacy(resp)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// NewQuerySeedCmd returns the command to query a seed by ID.
func NewQuerySeedCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "seed [id]",
		Short: "Query a seed by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			req := &types.QuerySeedRequest{SeedId: args[0]}
			resp := &types.QuerySeedResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.tree.v1.Query/Seed", req, resp); err != nil {
				return fmt.Errorf("failed to query seed: %w", err)
			}

			return clientCtx.PrintObjectLegacy(resp)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// NewQueryParamsCmd returns the command to query tree module params.
func NewQueryParamsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "params",
		Short: "Query the tree module parameters",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			req := &types.QueryParamsRequest{}
			resp := &types.QueryParamsResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.tree.v1.Query/Params", req, resp); err != nil {
				return fmt.Errorf("failed to query params: %w", err)
			}

			return clientCtx.PrintObjectLegacy(resp)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}
