package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"

	"github.com/zerone-chain/zerone/x/gov/types"
)

// NewQueryCmd returns the query commands for the zerone_gov module.
func NewQueryCmd() *cobra.Command {
	queryCmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "Zerone governance module query commands",
		DisableFlagParsing:         false,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	queryCmd.AddCommand(
		NewQueryLIPCmd(),
		NewQueryLIPsCmd(),
		NewQueryVoteCmd(),
		NewQueryVotesCmd(),
		NewQueryTallyResultCmd(),
		NewQueryParamsCmd(),
		NewQueryResearchSpendCmd(),
		NewQueryResearchSpendsCmd(),
		NewQueryResearchVotersCmd(),
		NewQueryResearchFundGovernanceCmd(),
		NewQueryResearchFundSeatsCmd(),
		NewQuerySeatElectionCmd(),
		NewQuerySeatElectionsCmd(),
		NewQueryPendingPhaseTransitionCmd(),
	)

	return queryCmd
}

// NewQueryLIPCmd returns the command to query a LIP by ID.
func NewQueryLIPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lip [lip-id]",
		Short: "Query a LIP by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			req := &types.QueryLIPRequest{LipId: args[0]}
			resp := &types.QueryLIPResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.gov.v1.Query/LIP", req, resp); err != nil {
				return fmt.Errorf("failed to query LIP: %w", err)
			}

			return clientCtx.PrintObjectLegacy(resp)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// NewQueryLIPsCmd returns the command to list LIPs with optional filters.
func NewQueryLIPsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lips",
		Short: "List LIPs with optional status and category filters",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			status, _ := cmd.Flags().GetString("status")
			category, _ := cmd.Flags().GetString("category")
			limit, _ := cmd.Flags().GetUint64("limit")
			offset, _ := cmd.Flags().GetUint64("offset")

			req := &types.QueryLIPsRequest{
				Status:   status,
				Category: category,
				Limit:    limit,
				Offset:   offset,
			}
			resp := &types.QueryLIPsResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.gov.v1.Query/LIPs", req, resp); err != nil {
				return fmt.Errorf("failed to query LIPs: %w", err)
			}

			return clientCtx.PrintObjectLegacy(resp)
		},
	}

	cmd.Flags().String("status", "", "Filter by stage (optional)")
	cmd.Flags().String("category", "", "Filter by category (optional)")
	cmd.Flags().Uint64("limit", 100, "Maximum results")
	cmd.Flags().Uint64("offset", 0, "Pagination offset")
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// NewQueryVoteCmd returns the command to query a specific vote.
func NewQueryVoteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vote [lip-id] [voter]",
		Short: "Query a vote by LIP ID and voter address",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			req := &types.QueryVoteRequest{LipId: args[0], Voter: args[1]}
			resp := &types.QueryVoteResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.gov.v1.Query/Vote", req, resp); err != nil {
				return fmt.Errorf("failed to query vote: %w", err)
			}

			return clientCtx.PrintObjectLegacy(resp)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// NewQueryVotesCmd returns the command to query all votes for a LIP.
func NewQueryVotesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "votes [lip-id]",
		Short: "Query all votes for a LIP",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			req := &types.QueryVotesRequest{LipId: args[0]}
			resp := &types.QueryVotesResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.gov.v1.Query/Votes", req, resp); err != nil {
				return fmt.Errorf("failed to query votes: %w", err)
			}

			return clientCtx.PrintObjectLegacy(resp)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// NewQueryTallyResultCmd returns the command to query the tally result for a LIP.
func NewQueryTallyResultCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tally-result [lip-id]",
		Short: "Query the tally result for a LIP",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			req := &types.QueryTallyResultRequest{LipId: args[0]}
			resp := &types.QueryTallyResultResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.gov.v1.Query/TallyResult", req, resp); err != nil {
				return fmt.Errorf("failed to query tally result: %w", err)
			}

			return clientCtx.PrintObjectLegacy(resp)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// NewQueryParamsCmd returns the command to query zerone_gov module params.
func NewQueryParamsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "params",
		Short: "Query the zerone governance module parameters",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			req := &types.QueryParamsRequest{}
			resp := &types.QueryParamsResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.gov.v1.Query/Params", req, resp); err != nil {
				return fmt.Errorf("failed to query params: %w", err)
			}

			return clientCtx.PrintObjectLegacy(resp)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// NewQueryResearchSpendCmd returns the command to query a research spend proposal.
func NewQueryResearchSpendCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "research-spend [proposal-id]",
		Short: "Query a research spend proposal by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			proposalID, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid proposal-id: %w", err)
			}

			req := &types.QueryResearchSpendRequest{ProposalId: proposalID}
			resp := &types.QueryResearchSpendResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.gov.v1.Query/ResearchSpend", req, resp); err != nil {
				return fmt.Errorf("failed to query research spend: %w", err)
			}

			return clientCtx.PrintObjectLegacy(resp)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// NewQueryResearchSpendsCmd returns the command to list research spend proposals.
func NewQueryResearchSpendsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "research-spends",
		Short: "List research spend proposals with optional stage filter",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			stage, _ := cmd.Flags().GetString("stage")
			limit, _ := cmd.Flags().GetUint64("limit")
			offset, _ := cmd.Flags().GetUint64("offset")

			req := &types.QueryResearchSpendsRequest{
				Stage:  stage,
				Limit:  limit,
				Offset: offset,
			}
			resp := &types.QueryResearchSpendsResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.gov.v1.Query/ResearchSpends", req, resp); err != nil {
				return fmt.Errorf("failed to query research spends: %w", err)
			}

			return clientCtx.PrintObjectLegacy(resp)
		},
	}

	cmd.Flags().String("stage", "", "Filter by stage (discussion, voting, executed, rejected, expired)")
	cmd.Flags().Uint64("limit", 100, "Maximum results")
	cmd.Flags().Uint64("offset", 0, "Pagination offset")
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// NewQueryResearchVotersCmd returns the command to query the research fund voter config.
func NewQueryResearchVotersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "research-voters",
		Short: "Query the 2-of-2 research fund voter configuration",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			req := &types.QueryResearchVotersRequest{}
			resp := &types.QueryResearchVotersResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.gov.v1.Query/ResearchVoters", req, resp); err != nil {
				return fmt.Errorf("failed to query research voters: %w", err)
			}

			return clientCtx.PrintObjectLegacy(resp)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// NewQueryResearchFundGovernanceCmd returns the command to query research fund governance state.
func NewQueryResearchFundGovernanceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "research-fund-governance",
		Short: "Query the research fund governance phase, state, and exit conditions",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			req := &types.QueryResearchFundGovernanceRequest{}
			resp := &types.QueryResearchFundGovernanceResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.gov.v1.Query/ResearchFundGovernance", req, resp); err != nil {
				return fmt.Errorf("failed to query research fund governance: %w", err)
			}

			return clientCtx.PrintObjectLegacy(resp)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// NewQueryResearchFundSeatsCmd returns the command to query current community seat holders.
func NewQueryResearchFundSeatsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "research-fund-seats",
		Short: "Query current community seat holders",
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			req := &types.QueryResearchFundSeatsRequest{}
			resp := &types.QueryResearchFundSeatsResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.gov.v1.Query/ResearchFundSeats", req, resp); err != nil {
				return fmt.Errorf("failed to query research fund seats: %w", err)
			}

			return clientCtx.PrintObjectLegacy(resp)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// NewQuerySeatElectionCmd returns the command to query a seat election proposal by ID.
func NewQuerySeatElectionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "seat-election [proposal-id]",
		Short: "Query a seat election proposal by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			proposalID, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid proposal-id: %w", err)
			}

			req := &types.QuerySeatElectionRequest{ProposalId: proposalID}
			resp := &types.QuerySeatElectionResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.gov.v1.Query/SeatElection", req, resp); err != nil {
				return fmt.Errorf("failed to query seat election: %w", err)
			}

			return clientCtx.PrintObjectLegacy(resp)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// NewQuerySeatElectionsCmd returns the command to list seat election proposals.
func NewQuerySeatElectionsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "seat-elections",
		Short: "List seat election proposals",
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			stage, _ := cmd.Flags().GetString("stage")
			limit, _ := cmd.Flags().GetUint64("limit")
			offset, _ := cmd.Flags().GetUint64("offset")

			req := &types.QuerySeatElectionsRequest{
				Stage:  stage,
				Limit:  limit,
				Offset: offset,
			}
			resp := &types.QuerySeatElectionsResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.gov.v1.Query/SeatElections", req, resp); err != nil {
				return fmt.Errorf("failed to query seat elections: %w", err)
			}

			return clientCtx.PrintObjectLegacy(resp)
		},
	}

	cmd.Flags().String("stage", "", "Filter by stage")
	cmd.Flags().Uint64("limit", 100, "Max results")
	cmd.Flags().Uint64("offset", 0, "Result offset")
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// NewQueryPendingPhaseTransitionCmd returns the command to query pending phase transitions.
// Uses the LIP query filtered by phase transition categories.
func NewQueryPendingPhaseTransitionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pending-phase-transition",
		Short: "Query pending phase transition proposals",
		Long:  "Query LIPs with research_phase_transition or research_phase_rollback categories that are not in terminal state.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			// Query phase transition LIPs.
			req := &types.QueryLIPsRequest{
				Category: types.CategoryPhaseTransition,
				Limit:    100,
			}
			resp := &types.QueryLIPsResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.gov.v1.Query/LIPs", req, resp); err != nil {
				return fmt.Errorf("failed to query phase transition LIPs: %w", err)
			}

			// Also query rollback LIPs.
			reqRollback := &types.QueryLIPsRequest{
				Category: types.CategoryPhaseRollback,
				Limit:    100,
			}
			respRollback := &types.QueryLIPsResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.gov.v1.Query/LIPs", reqRollback, respRollback); err != nil {
				return fmt.Errorf("failed to query phase rollback LIPs: %w", err)
			}

			// Merge results.
			resp.Lips = append(resp.Lips, respRollback.Lips...)
			resp.Total += respRollback.Total

			return clientCtx.PrintObjectLegacy(resp)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}
