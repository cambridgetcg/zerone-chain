package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"

	"github.com/zerone-chain/zerone/x/evidence_mgmt/types"
)

func NewQueryCmd() *cobra.Command {
	queryCmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "Evidence management module query commands",
		DisableFlagParsing:         false,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}
	queryCmd.AddCommand(
		NewQueryEvidenceCmd(),
		NewQueryParamsCmd(),
		NewQueryEvidenceBySubmitterCmd(),
		NewQueryCustodyChainCmd(),
		NewQueryVerificationsCmd(),
	)
	return queryCmd
}

func NewQueryEvidenceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "evidence [id]",
		Short: "Query evidence by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			req := &types.QueryEvidenceRequest{Id: args[0]}
			resp := &types.QueryEvidenceResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.evidence_mgmt.v1.Query/QueryEvidence", req, resp); err != nil {
				return fmt.Errorf("failed to query evidence: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func NewQueryEvidenceBySubmitterCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "by-submitter [address]",
		Short: "Query evidence by submitter address",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			req := &types.QueryEvidenceBySubmitterRequest{Submitter: args[0]}
			resp := &types.QueryEvidenceBySubmitterResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.evidence_mgmt.v1.Query/QueryEvidenceBySubmitter", req, resp); err != nil {
				return fmt.Errorf("failed to query evidence by submitter: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func NewQueryCustodyChainCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "custody-chain [evidence-id]",
		Short: "Query the chain of custody for evidence",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			req := &types.QueryCustodyChainRequest{EvidenceId: args[0]}
			resp := &types.QueryCustodyChainResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.evidence_mgmt.v1.Query/QueryCustodyChain", req, resp); err != nil {
				return fmt.Errorf("failed to query custody chain: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func NewQueryVerificationsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "verifications [evidence-id]",
		Short: "Query verifications for evidence",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			req := &types.QueryVerificationsRequest{EvidenceId: args[0]}
			resp := &types.QueryVerificationsResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.evidence_mgmt.v1.Query/QueryVerifications", req, resp); err != nil {
				return fmt.Errorf("failed to query verifications: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func NewQueryParamsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "params",
		Short: "Query evidence management module parameters",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			req := &types.QueryParamsRequest{}
			resp := &types.QueryParamsResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.evidence_mgmt.v1.Query/QueryParams", req, resp); err != nil {
				return fmt.Errorf("failed to query params: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}
