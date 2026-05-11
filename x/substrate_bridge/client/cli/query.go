package cli

import (
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/spf13/cobra"

	"github.com/zerone-chain/zerone/x/substrate_bridge/types"
)

func GetQueryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   types.ModuleName,
		Short: "Query substrate_bridge state",
	}
	cmd.AddCommand(cmdQueryParams())
	cmd.AddCommand(cmdQueryAdapter())
	cmd.AddCommand(cmdQueryAdapters())
	cmd.AddCommand(cmdQueryAttestation())
	cmd.AddCommand(cmdQueryLineageForward())
	cmd.AddCommand(cmdQueryLineageBackward())
	cmd.AddCommand(cmdQueryLineageAccumulator())
	return cmd
}

func cmdQueryParams() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "params",
		Short: "Show module params",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cctx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			res, err := types.NewQueryClient(cctx).Params(cmd.Context(), &types.QueryParamsRequest{})
			if err != nil {
				return err
			}
			return cctx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func cmdQueryAdapter() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "adapter [adapter-id]",
		Short: "Show a registered adapter",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cctx, _ := client.GetClientQueryContext(cmd)
			res, err := types.NewQueryClient(cctx).Adapter(cmd.Context(), &types.QueryAdapterRequest{AdapterId: args[0]})
			if err != nil {
				return err
			}
			return cctx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func cmdQueryAdapters() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "adapters",
		Short: "List adapters (optionally filtered by status)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cctx, _ := client.GetClientQueryContext(cmd)
			res, err := types.NewQueryClient(cctx).Adapters(cmd.Context(), &types.QueryAdaptersRequest{})
			if err != nil {
				return err
			}
			return cctx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func cmdQueryAttestation() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "attestation [attestation-id]",
		Short: "Show an external attestation",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cctx, _ := client.GetClientQueryContext(cmd)
			res, err := types.NewQueryClient(cctx).Attestation(cmd.Context(), &types.QueryAttestationRequest{AttestationId: args[0]})
			if err != nil {
				return err
			}
			return cctx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func cmdQueryLineageForward() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lineage-forward [attestation-id]",
		Short: "Walk forward lineage (downstream uses)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cctx, _ := client.GetClientQueryContext(cmd)
			res, err := types.NewQueryClient(cctx).LineageForwardWalk(cmd.Context(), &types.QueryLineageForwardWalkRequest{AttestationId: args[0]})
			if err != nil {
				return err
			}
			return cctx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func cmdQueryLineageBackward() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lineage-backward [attestation-id]",
		Short: "Walk backward lineage (upstream cites)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cctx, _ := client.GetClientQueryContext(cmd)
			res, err := types.NewQueryClient(cctx).LineageBackwardWalk(cmd.Context(), &types.QueryLineageBackwardWalkRequest{AttestationId: args[0]})
			if err != nil {
				return err
			}
			return cctx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func cmdQueryLineageAccumulator() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lineage-accumulator [attestation-id]",
		Short: "Cumulative lineage royalty income for an attestation",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cctx, _ := client.GetClientQueryContext(cmd)
			res, err := types.NewQueryClient(cctx).LineageAccumulator(cmd.Context(), &types.QueryLineageAccumulatorRequest{AttestationId: args[0]})
			if err != nil {
				return err
			}
			return cctx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}
