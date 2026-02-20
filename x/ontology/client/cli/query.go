package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"

	"github.com/zerone-chain/zerone/x/ontology/types"
)

// NewQueryCmd returns the query commands for the ontology module.
func NewQueryCmd() *cobra.Command {
	queryCmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "Ontology module query commands",
		DisableFlagParsing:         false,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	queryCmd.AddCommand(
		NewQueryParamsCmd(),
		NewQueryStratumCmd(),
		NewQueryAllStrataCmd(),
		NewQueryDomainCmd(),
		NewQueryDomainsByStratumCmd(),
		NewQueryAllDomainsCmd(),
		NewQueryProposalCmd(),
		NewQueryConfidenceCeilingCmd(),
		NewQueryLogicZoneCmd(),
		NewQueryAllLogicZonesCmd(),
	)

	return queryCmd
}

// NewQueryParamsCmd returns the command to query ontology module params.
func NewQueryParamsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "params",
		Short: "Query the ontology module parameters",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			req := &types.QueryParamsRequest{}
			resp := &types.QueryParamsResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.ontology.v1.Query/Params", req, resp); err != nil {
				return fmt.Errorf("failed to query params: %w", err)
			}

			return clientCtx.PrintObjectLegacy(resp)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// NewQueryStratumCmd returns the command to query a stratum.
func NewQueryStratumCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stratum [stratum-level]",
		Short: "Query a stratum by level (0-6)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			level, err := strconv.ParseUint(args[0], 10, 32)
			if err != nil {
				return fmt.Errorf("invalid stratum level: %w", err)
			}

			req := &types.QueryStratumRequest{Stratum: uint32(level)}
			resp := &types.QueryStratumResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.ontology.v1.Query/Stratum", req, resp); err != nil {
				return fmt.Errorf("failed to query stratum: %w", err)
			}

			return clientCtx.PrintObjectLegacy(resp)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// NewQueryAllStrataCmd returns the command to list all strata.
func NewQueryAllStrataCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "all-strata",
		Short: "List all registered strata",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			req := &types.QueryAllStrataRequest{}
			resp := &types.QueryAllStrataResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.ontology.v1.Query/AllStrata", req, resp); err != nil {
				return fmt.Errorf("failed to query strata: %w", err)
			}

			return clientCtx.PrintObjectLegacy(resp)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// NewQueryDomainCmd returns the command to query a domain.
func NewQueryDomainCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "domain [domain-name]",
		Short: "Query a domain by name",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			req := &types.QueryDomainRequest{Name: args[0]}
			resp := &types.QueryDomainResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.ontology.v1.Query/Domain", req, resp); err != nil {
				return fmt.Errorf("failed to query domain: %w", err)
			}

			return clientCtx.PrintObjectLegacy(resp)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// NewQueryDomainsByStratumCmd returns the command to query domains by stratum.
func NewQueryDomainsByStratumCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "domains-by-stratum [stratum-level]",
		Short: "Query all domains in a stratum",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			level, err := strconv.ParseUint(args[0], 10, 32)
			if err != nil {
				return fmt.Errorf("invalid stratum level: %w", err)
			}

			req := &types.QueryDomainsByStratumRequest{Stratum: uint32(level)}
			resp := &types.QueryDomainsByStratumResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.ontology.v1.Query/DomainsByStratum", req, resp); err != nil {
				return fmt.Errorf("failed to query domains: %w", err)
			}

			return clientCtx.PrintObjectLegacy(resp)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// NewQueryAllDomainsCmd returns the command to list all domains.
func NewQueryAllDomainsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "all-domains",
		Short: "List all registered domains",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			req := &types.QueryAllDomainsRequest{}
			resp := &types.QueryAllDomainsResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.ontology.v1.Query/AllDomains", req, resp); err != nil {
				return fmt.Errorf("failed to query domains: %w", err)
			}

			return clientCtx.PrintObjectLegacy(resp)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// NewQueryProposalCmd returns the command to query a domain proposal.
func NewQueryProposalCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "proposal [proposal-id]",
		Short: "Query a domain proposal by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			req := &types.QueryProposalRequest{ProposalId: args[0]}
			resp := &types.QueryProposalResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.ontology.v1.Query/Proposal", req, resp); err != nil {
				return fmt.Errorf("failed to query proposal: %w", err)
			}

			return clientCtx.PrintObjectLegacy(resp)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// NewQueryConfidenceCeilingCmd returns the command to query confidence ceiling.
func NewQueryConfidenceCeilingCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "confidence-ceiling [domain-name]",
		Short: "Query the confidence ceiling for a domain",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			req := &types.QueryConfidenceCeilingRequest{DomainName: args[0]}
			resp := &types.QueryConfidenceCeilingResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.ontology.v1.Query/ConfidenceCeiling", req, resp); err != nil {
				return fmt.Errorf("failed to query confidence ceiling: %w", err)
			}

			return clientCtx.PrintObjectLegacy(resp)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// NewQueryLogicZoneCmd returns the command to query a logic zone.
func NewQueryLogicZoneCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logic-zone [zone-name]",
		Short: "Query a logic zone by name",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			req := &types.QueryLogicZoneRequest{Zone: args[0]}
			resp := &types.QueryLogicZoneResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.ontology.v1.Query/LogicZone", req, resp); err != nil {
				return fmt.Errorf("failed to query logic zone: %w", err)
			}

			return clientCtx.PrintObjectLegacy(resp)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// NewQueryAllLogicZonesCmd returns the command to list all logic zones.
func NewQueryAllLogicZonesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "all-logic-zones",
		Short: "List all registered logic zones",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			req := &types.QueryAllLogicZonesRequest{}
			resp := &types.QueryAllLogicZonesResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.ontology.v1.Query/AllLogicZones", req, resp); err != nil {
				return fmt.Errorf("failed to query logic zones: %w", err)
			}

			return clientCtx.PrintObjectLegacy(resp)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}
