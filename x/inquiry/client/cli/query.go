package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"

	"github.com/zerone-chain/zerone/x/inquiry/types"
)

const (
	flagLimit       = "limit"
	flagStartAfter  = "start-after"
	flagStatus      = "status"
)

func NewQueryCmd() *cobra.Command {
	queryCmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "Inquiry module query commands",
		DisableFlagParsing:         false,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}
	queryCmd.AddCommand(
		NewQueryParamsCmd(),
		NewQueryInquiryCmd(),
		NewQueryInquiriesCmd(),
		NewQueryByDomainCmd(),
		NewQueryByAskerCmd(),
		NewQueryAnswersCmd(),
	)
	return queryCmd
}

func NewQueryParamsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "params",
		Short: "Query inquiry module parameters",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			req := &types.QueryParamsRequest{}
			resp := &types.QueryParamsResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.inquiry.v1.Query/Params", req, resp); err != nil {
				return fmt.Errorf("query params: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func NewQueryInquiryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inquiry [id]",
		Short: "Query a single inquiry by id",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			req := &types.QueryInquiryRequest{Id: args[0]}
			resp := &types.QueryInquiryResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.inquiry.v1.Query/Inquiry", req, resp); err != nil {
				return fmt.Errorf("query inquiry: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func NewQueryInquiriesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inquiries",
		Short: "List inquiries (paginated)",
		Long: `List inquiries on the chain. Filter by --status (open, answered,
resolved, expired, cancelled).`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			limit, _ := cmd.Flags().GetUint32(flagLimit)
			startAfter, _ := cmd.Flags().GetString(flagStartAfter)
			statusStr, _ := cmd.Flags().GetString(flagStatus)

			status := types.InquiryStatus_INQUIRY_STATUS_UNSPECIFIED
			switch strings.ToLower(statusStr) {
			case "":
			case "open":
				status = types.InquiryStatus_INQUIRY_STATUS_OPEN
			case "answered":
				status = types.InquiryStatus_INQUIRY_STATUS_ANSWERED
			case "resolved":
				status = types.InquiryStatus_INQUIRY_STATUS_RESOLVED
			case "expired":
				status = types.InquiryStatus_INQUIRY_STATUS_EXPIRED
			case "cancelled", "canceled":
				status = types.InquiryStatus_INQUIRY_STATUS_CANCELLED
			default:
				return fmt.Errorf("invalid --status %q", statusStr)
			}

			req := &types.QueryInquiriesRequest{
				Status:       status,
				Limit:        limit,
				StartAfterId: startAfter,
			}
			resp := &types.QueryInquiriesResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.inquiry.v1.Query/Inquiries", req, resp); err != nil {
				return fmt.Errorf("query inquiries: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	cmd.Flags().Uint32(flagLimit, 50, "Maximum number of inquiries to return")
	cmd.Flags().String(flagStartAfter, "", "Inquiry id to start after (pagination cursor)")
	cmd.Flags().String(flagStatus, "", "Filter by status: open | answered | resolved | expired | cancelled")
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func NewQueryByDomainCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "by-domain [domain]",
		Short: "List inquiries in a domain",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			req := &types.QueryByDomainRequest{Domain: args[0]}
			resp := &types.QueryByDomainResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.inquiry.v1.Query/InquiriesByDomain", req, resp); err != nil {
				return fmt.Errorf("query by domain: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func NewQueryByAskerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "by-asker [asker]",
		Short: "List inquiries submitted by a given address",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			req := &types.QueryByAskerRequest{Asker: args[0]}
			resp := &types.QueryByAskerResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.inquiry.v1.Query/InquiriesByAsker", req, resp); err != nil {
				return fmt.Errorf("query by asker: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func NewQueryAnswersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "answers [inquiry-id]",
		Short: "List answers linked to an inquiry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			req := &types.QueryAnswersByInquiryRequest{InquiryId: args[0]}
			resp := &types.QueryAnswersByInquiryResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.inquiry.v1.Query/AnswersByInquiry", req, resp); err != nil {
				return fmt.Errorf("query answers: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}
