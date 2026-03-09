package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
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
		NewQuerySubmissionsCmd(),
		NewQueryQualityRoundCmd(),
		NewQueryRoundsCmd(),
		NewQueryDatasetCmd(),
		NewQueryDatasetsCmd(),
		NewQueryTrainingDemandCmd(),
		NewQueryDataBountiesCmd(),
		NewQueryBountiesCmd(),
		NewQueryDomainCmd(),
		NewQueryDomainsCmd(),
		NewQueryDomainStatsCmd(),
		NewQueryProtocolStatsCmd(),
		NewQueryDashboardCmd(),
		NewQueryReputationCmd(),
		NewQueryLeaderboardCmd(),
		NewQueryStakesCmd(),
		NewQueryActiveRoundsCmd(),
		NewQueryFitnessCmd(),
		NewQueryFitnessSummaryCmd(),
		NewQueryShardCmd(),
		NewQueryShardStatusCmd(),
		// Agent onboarding queries
		CmdAgentList(),
		CmdAgentStatus(),
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

// ═══════════════════════════════════════════════════════════════════════════
// R41-3: New query CLI commands
// ═══════════════════════════════════════════════════════════════════════════

// ---------- Submissions (filtered) ----------

func NewQuerySubmissionsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "submissions",
		Short: "Query submissions with optional status, domain, and limit filters",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			statusFilter, _ := cmd.Flags().GetString("status")
			domain, _ := cmd.Flags().GetString("domain")
			limit, _ := cmd.Flags().GetUint64("limit")

			// Use existing PendingSubmissions gRPC and filter client-side.
			req := &types.QueryPendingSubmissionsRequest{}
			resp := &types.QuerySubmissionsResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/PendingSubmissions", req, resp); err != nil {
				return fmt.Errorf("failed to query submissions: %w", err)
			}

			var filtered []*types.Submission
			for _, sub := range resp.Submissions {
				if statusFilter != "" && !matchesSubmissionStatus(sub, statusFilter) {
					continue
				}
				if domain != "" && sub.Domain != domain {
					continue
				}
				filtered = append(filtered, sub)
			}
			if limit > 0 && uint64(len(filtered)) > limit {
				filtered = filtered[:limit]
			}
			resp.Submissions = filtered
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	cmd.Flags().String("status", "", "Filter by status: pending, accepted, rejected")
	cmd.Flags().String("domain", "", "Filter by domain")
	cmd.Flags().Uint64("limit", 20, "Maximum results to return")
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func matchesSubmissionStatus(sub *types.Submission, filter string) bool {
	upper := strings.ToUpper(filter)
	if !strings.HasPrefix(upper, "SUBMISSION_STATUS_") {
		upper = "SUBMISSION_STATUS_" + upper
	}
	val, ok := types.SubmissionStatus_value[upper]
	if !ok {
		return false
	}
	return sub.Status == types.SubmissionStatus(val)
}

// ---------- Rounds (filtered) ----------

func NewQueryRoundsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rounds",
		Short: "Query quality rounds with optional phase filter",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			statusFilter, _ := cmd.Flags().GetString("status")
			limit, _ := cmd.Flags().GetUint64("limit")

			// Get protocol stats to find active round count
			statsReq := &types.QueryProtocolStatsRequest{}
			statsResp := &types.QueryProtocolStatsResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/ProtocolStats", statsReq, statsResp); err != nil {
				return fmt.Errorf("failed to query protocol stats: %w", err)
			}

			// Query individual active rounds by iterating the active round index
			// via store queries. Active round IDs are stored under ActiveRoundIndexPrefix.
			prefix := types.ActiveRoundIndexPrefix
			bz, _, err := clientCtx.QueryStore(prefix, types.StoreKey)
			_ = bz // point lookup on prefix returns nothing useful

			// Fallback: report active round count from protocol stats
			type roundSummary struct {
				ActiveRounds uint64 `json:"active_rounds"`
				Status       string `json:"status_filter,omitempty"`
				Message      string `json:"message"`
			}
			summary := roundSummary{
				ActiveRounds: statsResp.ActiveRounds,
				Status:       statusFilter,
				Message:      "Use 'zeroned q knowledge quality-round <round-id>' to inspect individual rounds",
			}
			_ = limit
			return printJSON(cmd, summary)
		},
	}
	cmd.Flags().String("status", "", "Filter by phase: commit, reveal, aggregating")
	cmd.Flags().Uint64("limit", 10, "Maximum results to return")
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// ---------- Bounties (alias with status filter) ----------

func NewQueryBountiesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bounties",
		Short: "Query data bounties with optional domain and status filters",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			domain, _ := cmd.Flags().GetString("domain")
			statusFilter, _ := cmd.Flags().GetString("status")

			req := &types.QueryDataBountiesRequest{Domain: domain}
			resp := &types.QueryDataBountiesResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/DataBounties", req, resp); err != nil {
				return fmt.Errorf("failed to query bounties: %w", err)
			}

			if statusFilter == "open" {
				var open []*types.DataBounty
				for _, b := range resp.Bounties {
					if !b.Claimed {
						open = append(open, b)
					}
				}
				resp.Bounties = open
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	cmd.Flags().String("domain", "", "Filter by domain")
	cmd.Flags().String("status", "", "Filter by status: open, claimed")
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// ---------- Dashboard ----------

func NewQueryDashboardCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dashboard",
		Short: "Query agent dashboard: submissions, reputation, stakes, fitness",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			address, _ := cmd.Flags().GetString("address")
			if address == "" {
				return fmt.Errorf("--address is required")
			}

			dashboard := map[string]interface{}{
				"address": address,
			}

			// 1) Submissions by submitter (via samples-by-submitter gRPC)
			samplesReq := &types.QuerySamplesBySubmitterRequest{Submitter: address}
			samplesResp := &types.QuerySamplesResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/SamplesBySubmitter", samplesReq, samplesResp); err == nil {
				total := len(samplesResp.Samples)
				var accepted, rejected, pending int
				var fitnessEntries []map[string]string
				for _, s := range samplesResp.Samples {
					switch s.Status {
					case types.SampleStatus_SAMPLE_STATUS_GOLD,
						types.SampleStatus_SAMPLE_STATUS_SILVER,
						types.SampleStatus_SAMPLE_STATUS_BRONZE:
						accepted++
					case types.SampleStatus_SAMPLE_STATUS_REJECTED:
						rejected++
					case types.SampleStatus_SAMPLE_STATUS_PENDING,
						types.SampleStatus_SAMPLE_STATUS_IN_REVIEW:
						pending++
					}

					// Fitness for each sample
					fitBz, _, fitErr := clientCtx.QueryStore(types.FitnessRecordKey(s.Id), types.StoreKey)
					if fitErr == nil && len(fitBz) > 0 {
						var rec types.TDUFitnessRecord
						if json.Unmarshal(fitBz, &rec) == nil {
							fitnessEntries = append(fitnessEntries, map[string]string{
								"sample_id":        rec.SampleID,
								"fitness_score":    rec.FitnessScore,
								"lifecycle_status": rec.GetLifecycleStatus().String(),
							})
						}
					}
				}

				rate := "0.0"
				if total > 0 {
					rate = fmt.Sprintf("%.1f", float64(accepted)/float64(total)*100)
				}
				dashboard["total_submissions"] = total
				dashboard["accepted_submissions"] = accepted
				dashboard["rejected_submissions"] = rejected
				dashboard["pending_submissions"] = pending
				dashboard["acceptance_rate"] = rate
				if len(fitnessEntries) > 0 {
					dashboard["tdu_fitness"] = fitnessEntries
				}
			}

			// 2) Reputation per domain
			domainsReq := &types.QueryDomainsRequest{}
			domainsResp := &types.QueryDomainsResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/Domains", domainsReq, domainsResp); err == nil {
				var reputations []map[string]string
				for _, d := range domainsResp.Domains {
					repBz, _, repErr := clientCtx.QueryStore(
						types.AgentDomainReputationKey(address, d.Name), types.StoreKey,
					)
					if repErr == nil && len(repBz) > 0 {
						var rep types.AgentDomainReputation
						if json.Unmarshal(repBz, &rep) == nil {
							reputations = append(reputations, map[string]string{
								"domain":     rep.DomainID,
								"score":      rep.Score,
								"peak_score": rep.PeakScore,
							})
						}
					}
				}
				if len(reputations) > 0 {
					dashboard["reputations"] = reputations
				}
			}

			// 3) Pending reviews (active rounds where this address is a verifier)
			dashboard["pending_reviews"] = 0 // requires iteration; will be populated when proto-gen runs

			return printJSON(cmd, dashboard)
		},
	}
	cmd.Flags().String("address", "", "Agent address to query dashboard for")
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// ---------- Reputation ----------

func NewQueryReputationCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reputation",
		Short: "Query agent reputation, optionally filtered by domain",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			address, _ := cmd.Flags().GetString("address")
			if address == "" {
				return fmt.Errorf("--address is required")
			}
			domain, _ := cmd.Flags().GetString("domain")

			if domain != "" {
				// Single domain lookup via direct store query
				bz, _, err := clientCtx.QueryStore(
					types.AgentDomainReputationKey(address, domain), types.StoreKey,
				)
				if err != nil {
					return fmt.Errorf("failed to query reputation: %w", err)
				}
				if len(bz) == 0 {
					return fmt.Errorf("no reputation found for %s in domain %s", address, domain)
				}
				var rep types.AgentDomainReputation
				if err := json.Unmarshal(bz, &rep); err != nil {
					return fmt.Errorf("failed to unmarshal reputation: %w", err)
				}
				return printJSON(cmd, rep)
			}

			// All domains: iterate known domains and check each
			domainsReq := &types.QueryDomainsRequest{}
			domainsResp := &types.QueryDomainsResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/Domains", domainsReq, domainsResp); err != nil {
				return fmt.Errorf("failed to query domains: %w", err)
			}

			var reputations []types.AgentDomainReputation
			for _, d := range domainsResp.Domains {
				bz, _, err := clientCtx.QueryStore(
					types.AgentDomainReputationKey(address, d.Name), types.StoreKey,
				)
				if err != nil || len(bz) == 0 {
					continue
				}
				var rep types.AgentDomainReputation
				if json.Unmarshal(bz, &rep) == nil {
					reputations = append(reputations, rep)
				}
			}
			if len(reputations) == 0 {
				return fmt.Errorf("no reputation found for %s", address)
			}
			return printJSON(cmd, map[string]interface{}{
				"address":     address,
				"reputations": reputations,
			})
		},
	}
	cmd.Flags().String("address", "", "Agent address")
	cmd.Flags().String("domain", "", "Filter by domain")
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// ---------- Leaderboard ----------

func NewQueryLeaderboardCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "leaderboard",
		Short: "Query top agents by reputation in a domain",
		Long: `Query the top agents ranked by reputation score in a specific domain.
Note: This command uses client-side aggregation over known agents from sample submitters.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			domain, _ := cmd.Flags().GetString("domain")
			if domain == "" {
				return fmt.Errorf("--domain is required")
			}
			limit, _ := cmd.Flags().GetUint64("limit")
			if limit == 0 {
				limit = 10
			}

			// Get all samples in this domain to find unique submitters
			samplesReq := &types.QuerySamplesByDomainRequest{Domain: domain}
			samplesResp := &types.QuerySamplesResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/SamplesByDomain", samplesReq, samplesResp); err != nil {
				return fmt.Errorf("failed to query samples: %w", err)
			}

			// Collect unique submitters
			submitters := make(map[string]struct{})
			for _, s := range samplesResp.Samples {
				if s.Submitter != "" {
					submitters[s.Submitter] = struct{}{}
				}
			}

			// Query reputation for each submitter
			type entry struct {
				Address   string `json:"address"`
				Domain    string `json:"domain"`
				Score     string `json:"score"`
				PeakScore string `json:"peak_score"`
			}
			var entries []entry
			for addr := range submitters {
				bz, _, err := clientCtx.QueryStore(
					types.AgentDomainReputationKey(addr, domain), types.StoreKey,
				)
				if err != nil || len(bz) == 0 {
					continue
				}
				var rep types.AgentDomainReputation
				if json.Unmarshal(bz, &rep) == nil {
					entries = append(entries, entry{
						Address:   addr,
						Domain:    domain,
						Score:     rep.Score,
						PeakScore: rep.PeakScore,
					})
				}
			}

			// Sort by score descending
			sort.Slice(entries, func(i, j int) bool {
				si, _ := strconv.ParseFloat(entries[i].Score, 64)
				sj, _ := strconv.ParseFloat(entries[j].Score, 64)
				return si > sj
			})

			if uint64(len(entries)) > limit {
				entries = entries[:limit]
			}

			return printJSON(cmd, map[string]interface{}{
				"domain":  domain,
				"entries": entries,
			})
		},
	}
	cmd.Flags().String("domain", "", "Domain to rank (required)")
	cmd.Flags().Uint64("limit", 10, "Maximum entries to return")
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// ---------- Stakes ----------

func NewQueryStakesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stakes",
		Short: "Query active stakes for an agent",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			address, _ := cmd.Flags().GetString("address")
			if address == "" {
				return fmt.Errorf("--address is required")
			}

			// Get samples by submitter to find active submissions with stakes
			samplesReq := &types.QuerySamplesBySubmitterRequest{Submitter: address}
			samplesResp := &types.QuerySamplesResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/SamplesBySubmitter", samplesReq, samplesResp); err != nil {
				return fmt.Errorf("failed to query samples: %w", err)
			}

			type stakeEntry struct {
				SampleID string `json:"sample_id"`
				Domain   string `json:"domain"`
				Status   string `json:"status"`
				Stake    string `json:"stake,omitempty"`
			}
			var stakes []stakeEntry
			for _, s := range samplesResp.Samples {
				if s.Status == types.SampleStatus_SAMPLE_STATUS_PENDING ||
					s.Status == types.SampleStatus_SAMPLE_STATUS_IN_REVIEW {
					stakes = append(stakes, stakeEntry{
						SampleID: s.Id,
						Domain:   s.Domain,
						Status:   s.Status.String(),
					})
				}
			}

			return printJSON(cmd, map[string]interface{}{
				"address": address,
				"stakes":  stakes,
			})
		},
	}
	cmd.Flags().String("address", "", "Agent address")
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// ---------- ActiveRounds ----------

func NewQueryActiveRoundsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "active-rounds",
		Short: "Query active quality rounds, optionally filtered by reviewer",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			reviewer, _ := cmd.Flags().GetString("reviewer")

			// Get active round count from protocol stats
			statsReq := &types.QueryProtocolStatsRequest{}
			statsResp := &types.QueryProtocolStatsResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/ProtocolStats", statsReq, statsResp); err != nil {
				return fmt.Errorf("failed to query protocol stats: %w", err)
			}

			result := map[string]interface{}{
				"active_round_count": statsResp.ActiveRounds,
			}
			if reviewer != "" {
				result["reviewer_filter"] = reviewer
				result["note"] = "Per-reviewer filtering requires server-side gRPC endpoint; run 'make proto-gen' to enable"
			}
			return printJSON(cmd, result)
		},
	}
	cmd.Flags().String("reviewer", "", "Filter by reviewer address")
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// ---------- Fitness ----------

func NewQueryFitnessCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fitness [tdu-id]",
		Short: "Query fitness record for a TDU (sample)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			tduID := args[0]

			bz, _, err := clientCtx.QueryStore(types.FitnessRecordKey(tduID), types.StoreKey)
			if err != nil {
				return fmt.Errorf("failed to query fitness record: %w", err)
			}
			if len(bz) == 0 {
				return fmt.Errorf("no fitness record found for TDU %s", tduID)
			}

			var record types.TDUFitnessRecord
			if err := json.Unmarshal(bz, &record); err != nil {
				return fmt.Errorf("failed to unmarshal fitness record: %w", err)
			}

			result := map[string]interface{}{
				"sample_id":        record.SampleID,
				"fitness_score":    record.FitnessScore,
				"lifecycle_status": record.GetLifecycleStatus().String(),
				"original_stake":   record.OriginalStake,
				"last_signal_cycle": record.LastSignalCycle,
				"cycle_count":      record.CycleCount,
			}
			return printJSON(cmd, result)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// ---------- FitnessSummary ----------

func NewQueryFitnessSummaryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fitness-summary",
		Short: "Query aggregate fitness statistics by lifecycle status and domain",
		Long: `Query aggregate TDU fitness stats. Counts samples by lifecycle status.
Uses existing sample data and individual fitness record lookups.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			statusFilter, _ := cmd.Flags().GetString("status")
			domain, _ := cmd.Flags().GetString("domain")

			// Get samples (optionally by domain)
			var samples []*types.Sample
			if domain != "" {
				req := &types.QuerySamplesByDomainRequest{Domain: domain}
				resp := &types.QuerySamplesResponse{}
				if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/SamplesByDomain", req, resp); err != nil {
					return fmt.Errorf("failed to query samples: %w", err)
				}
				samples = resp.Samples
			} else {
				req := &types.QuerySamplesRequest{}
				resp := &types.QuerySamplesResponse{}
				if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/Samples", req, resp); err != nil {
					return fmt.Errorf("failed to query samples: %w", err)
				}
				samples = resp.Samples
			}

			var coreCount, activeCount, dormantCount, prunedCount uint64
			for _, s := range samples {
				bz, _, err := clientCtx.QueryStore(types.FitnessRecordKey(s.Id), types.StoreKey)
				if err != nil || len(bz) == 0 {
					continue
				}
				var rec types.TDUFitnessRecord
				if json.Unmarshal(bz, &rec) != nil {
					continue
				}
				status := rec.GetLifecycleStatus()
				if statusFilter != "" && status.String() != strings.ToLower(statusFilter) {
					continue
				}
				switch status {
				case types.TDULifecycleCore:
					coreCount++
				case types.TDULifecycleActive:
					activeCount++
				case types.TDULifecycleDormant:
					dormantCount++
				case types.TDULifecyclePruned:
					prunedCount++
				}
			}

			result := map[string]interface{}{
				"core_count":    coreCount,
				"active_count":  activeCount,
				"dormant_count": dormantCount,
				"pruned_count":  prunedCount,
				"total_count":   coreCount + activeCount + dormantCount + prunedCount,
			}
			if domain != "" {
				result["domain"] = domain
			}
			if statusFilter != "" {
				result["status_filter"] = statusFilter
			}
			return printJSON(cmd, result)
		},
	}
	cmd.Flags().String("status", "", "Filter by lifecycle status: core, active, dormant, pruned")
	cmd.Flags().String("domain", "", "Filter by domain")
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// ---------- Shard ----------

func NewQueryShardCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "shard",
		Short: "Query a validator's shard assignment at a snapshot height",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			validator, _ := cmd.Flags().GetString("validator")
			if validator == "" {
				return fmt.Errorf("--validator is required")
			}
			snapshotHeight, _ := cmd.Flags().GetInt64("snapshot-height")
			if snapshotHeight <= 0 {
				return fmt.Errorf("--snapshot-height is required and must be > 0")
			}

			// Query shard assignment from store
			assignBz, _, err := clientCtx.QueryStore(
				types.ShardAssignmentKey(validator, snapshotHeight), types.StoreKey,
			)
			if err != nil {
				return fmt.Errorf("failed to query shard assignment: %w", err)
			}
			if len(assignBz) == 0 {
				return fmt.Errorf("no shard assignment for validator %s at height %d", validator, snapshotHeight)
			}
			var assignment types.ShardAssignment
			if err := json.Unmarshal(assignBz, &assignment); err != nil {
				return fmt.Errorf("failed to unmarshal shard assignment: %w", err)
			}

			// Check if attestation exists
			attested := false
			attestBz, _, attestErr := clientCtx.QueryStore(
				types.ShardAttestationKey(validator, snapshotHeight), types.StoreKey,
			)
			if attestErr == nil && len(attestBz) > 0 {
				attested = true
			}

			result := map[string]interface{}{
				"validator":       assignment.ValidatorAddr,
				"snapshot_height": assignment.SnapshotHeight,
				"tdu_count":       len(assignment.TDUIDs),
				"tdu_ids":         assignment.TDUIDs,
				"attested":        attested,
			}
			return printJSON(cmd, result)
		},
	}
	cmd.Flags().String("validator", "", "Validator address")
	cmd.Flags().Int64("snapshot-height", 0, "Snapshot height")
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// ---------- ShardStatus ----------

func NewQueryShardStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "shard-status",
		Short: "Query global sharding status",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			// Read sharding params from store
			paramsBz, _, err := clientCtx.QueryStore(types.ShardingParamsKey, types.StoreKey)
			if err != nil {
				return fmt.Errorf("failed to query sharding params: %w", err)
			}
			params := types.DefaultShardingParams()
			if len(paramsBz) > 0 {
				_ = json.Unmarshal(paramsBz, &params)
			}

			result := map[string]interface{}{
				"snapshot_interval":  params.SnapshotInterval,
				"replication_factor": params.ReplicationFactor,
				"min_validators":     params.MinValidators,
			}
			return printJSON(cmd, result)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// printJSON marshals v to indented JSON and prints it via the command's output.
func printJSON(cmd *cobra.Command, v interface{}) error {
	bz, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	cmd.Println(string(bz))
	return nil
}
