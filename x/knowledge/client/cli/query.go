package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// parseSampleType maps a CLI string to a SampleType enum for query filtering.
func parseSampleType(s string) (types.SampleType, error) {
	if s == "" {
		return types.SampleType_SAMPLE_TYPE_UNSPECIFIED, nil
	}
	switch strings.ToLower(s) {
	case "discussion":
		return types.SampleType_SAMPLE_TYPE_DISCUSSION, nil
	case "debate":
		return types.SampleType_SAMPLE_TYPE_DEBATE, nil
	case "explanation":
		return types.SampleType_SAMPLE_TYPE_EXPLANATION, nil
	case "troubleshoot":
		return types.SampleType_SAMPLE_TYPE_TROUBLESHOOT, nil
	case "review":
		return types.SampleType_SAMPLE_TYPE_REVIEW, nil
	case "tutorial":
		return types.SampleType_SAMPLE_TYPE_TUTORIAL, nil
	case "opinion":
		return types.SampleType_SAMPLE_TYPE_OPINION, nil
	case "narrative":
		return types.SampleType_SAMPLE_TYPE_NARRATIVE, nil
	case "q_and_a", "qanda":
		return types.SampleType_SAMPLE_TYPE_Q_AND_A, nil
	case "creative":
		return types.SampleType_SAMPLE_TYPE_CREATIVE, nil
	case "annotation":
		return types.SampleType_SAMPLE_TYPE_ANNOTATION, nil
	case "correction":
		return types.SampleType_SAMPLE_TYPE_CORRECTION, nil
	default:
		return 0, fmt.Errorf("unknown sample type %q: must be discussion, debate, explanation, troubleshoot, review, tutorial, opinion, narrative, q_and_a, creative, annotation, or correction", s)
	}
}

// GetQueryCmd returns the root query command for the knowledge module.
func GetQueryCmd() *cobra.Command {
	queryCmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "Knowledge module query commands",
		DisableFlagParsing:         false,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	queryCmd.AddCommand(
		NewQueryParamsCmd(),
		NewQuerySampleCmd(),
		NewQuerySamplesCmd(),
		NewQuerySamplesByDomainCmd(),
		NewQuerySamplesByThreadCmd(),
		NewQuerySamplesBySubmitterCmd(),
		NewQuerySubmissionCmd(),
		NewQueryPendingSubmissionsCmd(),
		NewQueryQualityRoundCmd(),
		NewQueryDatasetCmd(),
		NewQueryDatasetsCmd(),
		NewQueryTrainingDemandCmd(),
		NewQueryDataBountiesCmd(),
		NewQueryDomainCmd(),
		NewQueryDomainsCmd(),
		NewQueryDomainStatsCmd(),
		NewQueryProtocolStatsCmd(),
	)

	return queryCmd
}

// ---------- Params ----------

func NewQueryParamsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "params",
		Short: "Query the knowledge module parameters",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			req := &types.QueryParamsRequest{}
			resp := &types.QueryParamsResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/Params", req, resp); err != nil {
				return fmt.Errorf("failed to query params: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// ---------- Sample ----------

func NewQuerySampleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sample [id]",
		Short: "Query a training data sample by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			req := &types.QuerySampleRequest{Id: args[0]}
			resp := &types.QuerySampleResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/Sample", req, resp); err != nil {
				return fmt.Errorf("failed to query sample: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// ---------- Samples ----------

func NewQuerySamplesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "samples",
		Short: "Query samples with optional filters",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			domain, _ := cmd.Flags().GetString("domain")
			status, _ := cmd.Flags().GetString("status")
			sampleTypeStr, _ := cmd.Flags().GetString("sample-type")
			sampleType, err := parseSampleType(sampleTypeStr)
			if err != nil {
				return err
			}
			req := &types.QuerySamplesRequest{
				Domain:     domain,
				Status:     status,
				SampleType: sampleType,
			}
			resp := &types.QuerySamplesResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/Samples", req, resp); err != nil {
				return fmt.Errorf("failed to query samples: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	cmd.Flags().String("domain", "", "Filter by domain")
	cmd.Flags().String("status", "", "Filter by status")
	cmd.Flags().String("sample-type", "", "Filter by sample type: discussion, debate, explanation, troubleshoot, review, tutorial, opinion, narrative, q_and_a, creative, annotation, correction")
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// ---------- SamplesByDomain ----------

func NewQuerySamplesByDomainCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "samples-by-domain [domain]",
		Short: "Query samples by domain",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			req := &types.QuerySamplesByDomainRequest{Domain: args[0]}
			resp := &types.QuerySamplesResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/SamplesByDomain", req, resp); err != nil {
				return fmt.Errorf("failed to query samples by domain: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// ---------- SamplesByThread ----------

func NewQuerySamplesByThreadCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "samples-by-thread [thread-id]",
		Short: "Query samples by conversation thread ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			req := &types.QuerySamplesByThreadRequest{ThreadId: args[0]}
			resp := &types.QuerySamplesResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/SamplesByThread", req, resp); err != nil {
				return fmt.Errorf("failed to query samples by thread: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// ---------- SamplesBySubmitter ----------

func NewQuerySamplesBySubmitterCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "samples-by-submitter [submitter]",
		Short: "Query samples by submitter address",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			req := &types.QuerySamplesBySubmitterRequest{Submitter: args[0]}
			resp := &types.QuerySamplesResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/SamplesBySubmitter", req, resp); err != nil {
				return fmt.Errorf("failed to query samples by submitter: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// ---------- Submission ----------

func NewQuerySubmissionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "submission [id]",
		Short: "Query a submission by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			req := &types.QuerySubmissionRequest{Id: args[0]}
			resp := &types.QuerySubmissionResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/Submission", req, resp); err != nil {
				return fmt.Errorf("failed to query submission: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// ---------- PendingSubmissions ----------

func NewQueryPendingSubmissionsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pending-submissions",
		Short: "Query all pending submissions",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			req := &types.QueryPendingSubmissionsRequest{}
			resp := &types.QuerySubmissionsResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/PendingSubmissions", req, resp); err != nil {
				return fmt.Errorf("failed to query pending submissions: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// ---------- QualityRound ----------

func NewQueryQualityRoundCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "quality-round [id]",
		Short: "Query a quality round by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			req := &types.QueryQualityRoundRequest{Id: args[0]}
			resp := &types.QueryQualityRoundResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/QualityRound", req, resp); err != nil {
				return fmt.Errorf("failed to query quality round: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// ---------- Dataset ----------

func NewQueryDatasetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dataset [id]",
		Short: "Query a dataset by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			req := &types.QueryDatasetRequest{Id: args[0]}
			resp := &types.QueryDatasetResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/Dataset", req, resp); err != nil {
				return fmt.Errorf("failed to query dataset: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// ---------- Datasets ----------

func NewQueryDatasetsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "datasets",
		Short: "Query datasets with optional domain filter",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			domain, _ := cmd.Flags().GetString("domain")
			req := &types.QueryDatasetsRequest{Domain: domain}
			resp := &types.QueryDatasetsResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/Datasets", req, resp); err != nil {
				return fmt.Errorf("failed to query datasets: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	cmd.Flags().String("domain", "", "Filter by domain")
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// ---------- TrainingDemand ----------

func NewQueryTrainingDemandCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "training-demand",
		Short: "Query training demand signals",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			domain, _ := cmd.Flags().GetString("domain")
			req := &types.QueryTrainingDemandRequest{Domain: domain}
			resp := &types.QueryTrainingDemandResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/TrainingDemand", req, resp); err != nil {
				return fmt.Errorf("failed to query training demand: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	cmd.Flags().String("domain", "", "Filter by domain")
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// ---------- DataBounties ----------

func NewQueryDataBountiesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "data-bounties",
		Short: "Query active data bounties",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			domain, _ := cmd.Flags().GetString("domain")
			req := &types.QueryDataBountiesRequest{Domain: domain}
			resp := &types.QueryDataBountiesResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/DataBounties", req, resp); err != nil {
				return fmt.Errorf("failed to query data bounties: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	cmd.Flags().String("domain", "", "Filter by domain")
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// ---------- Domain ----------

func NewQueryDomainCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "domain [name]",
		Short: "Query a knowledge domain by name",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			req := &types.QueryDomainRequest{Name: args[0]}
			resp := &types.QueryDomainResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/Domain", req, resp); err != nil {
				return fmt.Errorf("failed to query domain: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// ---------- Domains ----------

func NewQueryDomainsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "domains",
		Short: "Query all knowledge domains",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			req := &types.QueryDomainsRequest{}
			resp := &types.QueryDomainsResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/Domains", req, resp); err != nil {
				return fmt.Errorf("failed to query domains: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// ---------- DomainStats ----------

func NewQueryDomainStatsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "domain-stats [domain]",
		Short: "Query statistics for a domain",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			req := &types.QueryDomainStatsRequest{Domain: args[0]}
			resp := &types.QueryDomainStatsResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/DomainStats", req, resp); err != nil {
				return fmt.Errorf("failed to query domain stats: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// ---------- ProtocolStats ----------

func NewQueryProtocolStatsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "protocol-stats",
		Short: "Query aggregate protocol statistics",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			req := &types.QueryProtocolStatsRequest{}
			resp := &types.QueryProtocolStatsResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/ProtocolStats", req, resp); err != nil {
				return fmt.Errorf("failed to query protocol stats: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}
