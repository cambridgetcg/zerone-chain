package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"

	"github.com/zerone-chain/zerone/x/alignment/types"
)

// NewQueryCmd returns the query commands for the alignment module.
func NewQueryCmd() *cobra.Command {
	queryCmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "Alignment module query commands",
		DisableFlagParsing:         false,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	queryCmd.AddCommand(
		NewQueryParamsCmd(),
		NewQueryStateCmd(),
		NewQueryObservationCmd(),
		NewQueryScoresCmd(),
		NewQueryHealthIndexCmd(),
		NewQueryCorrectionHistoryCmd(),
		NewQueryHealthHistoryCmd(),
		NewQueryCorrectionConfidenceCmd(),
		NewQueryGlobalPacingCmd(),
	)

	return queryCmd
}

func NewQueryParamsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "params",
		Short: "Query the alignment module parameters",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			req := &types.QueryParamsRequest{}
			resp := &types.QueryParamsResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.alignment.v1.Query/Params", req, resp); err != nil {
				return fmt.Errorf("failed to query params: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func NewQueryStateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "state",
		Short: "Query the current alignment state",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			req := &types.QueryStateRequest{}
			resp := &types.QueryStateResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.alignment.v1.Query/State", req, resp); err != nil {
				return fmt.Errorf("failed to query state: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func NewQueryObservationCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "observation [height]",
		Short: "Query alignment observation at a given height",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			height, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return err
			}
			req := &types.QueryObservationRequest{Height: height}
			resp := &types.QueryObservationResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.alignment.v1.Query/Observation", req, resp); err != nil {
				return fmt.Errorf("failed to query observation: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func NewQueryScoresCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scores [height]",
		Short: "Query alignment scores at a given height",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			height, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return err
			}
			req := &types.QueryScoresRequest{Height: height}
			resp := &types.QueryScoresResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.alignment.v1.Query/Scores", req, resp); err != nil {
				return fmt.Errorf("failed to query scores: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func NewQueryHealthIndexCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "health-index [height]",
		Short: "Query the alignment health index at a given height",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			height, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return err
			}
			req := &types.QueryHealthIndexRequest{Height: height}
			resp := &types.QueryHealthIndexResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.alignment.v1.Query/HealthIndex", req, resp); err != nil {
				return fmt.Errorf("failed to query health index: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func NewQueryCorrectionHistoryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "correction-history",
		Short: "Query alignment correction history",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			limit, _ := cmd.Flags().GetUint32("limit")
			offset, _ := cmd.Flags().GetUint32("offset")
			req := &types.QueryCorrectionHistoryRequest{
				Limit:  limit,
				Offset: offset,
			}
			resp := &types.QueryCorrectionHistoryResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.alignment.v1.Query/CorrectionHistory", req, resp); err != nil {
				return fmt.Errorf("failed to query correction history: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	cmd.Flags().Uint32("limit", 20, "Maximum number of entries to return")
	cmd.Flags().Uint32("offset", 0, "Number of entries to skip")
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func NewQueryHealthHistoryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "history",
		Short: "Query alignment health history (most recent observations)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			limit, _ := cmd.Flags().GetUint32("limit")
			req := &types.QueryHealthHistoryRequest{Limit: limit}
			resp := &types.QueryHealthHistoryResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.alignment.v1.Query/HealthHistory", req, resp); err != nil {
				return fmt.Errorf("failed to query health history: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	cmd.Flags().Uint32("limit", 20, "Maximum number of entries to return (max 100)")
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func NewQueryGlobalPacingCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "global-pacing",
		Short: "Query the global adaptive pacing state",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			req := &types.QueryGlobalPacingRequest{}
			resp := &types.QueryGlobalPacingResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.alignment.v1.Query/GlobalPacing", req, resp); err != nil {
				return fmt.Errorf("failed to query global pacing: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func NewQueryCorrectionConfidenceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "correction-confidence",
		Short: "Query correction confidence score and effective bounds",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			req := &types.QueryCorrectionConfidenceRequest{}
			resp := &types.QueryCorrectionConfidenceResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.alignment.v1.Query/CorrectionConfidence", req, resp); err != nil {
				return fmt.Errorf("failed to query correction confidence: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}
