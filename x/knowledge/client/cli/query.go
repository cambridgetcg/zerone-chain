package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// parseClaimTypeQuery maps a CLI string to a ClaimType enum for query filtering.
func parseClaimTypeQuery(s string) (types.ClaimType, error) {
	if s == "" {
		return types.ClaimType_CLAIM_TYPE_UNSPECIFIED, nil
	}
	switch strings.ToLower(s) {
	case "assertion":
		return types.ClaimType_CLAIM_TYPE_ASSERTION, nil
	case "relation":
		return types.ClaimType_CLAIM_TYPE_RELATION, nil
	case "definition":
		return types.ClaimType_CLAIM_TYPE_DEFINITION, nil
	case "constraint":
		return types.ClaimType_CLAIM_TYPE_CONSTRAINT, nil
	case "negation":
		return types.ClaimType_CLAIM_TYPE_NEGATION, nil
	case "observation":
		return types.ClaimType_CLAIM_TYPE_OBSERVATION, nil
	default:
		return 0, fmt.Errorf("unknown claim type %q: must be assertion, relation, definition, constraint, negation, or observation", s)
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
		NewQueryFactCmd(),
		NewQueryFactsCmd(),
		NewQueryFactsByDomainCmd(),
		NewQueryFactsBySubmitterCmd(),
		NewQueryClaimCmd(),
		NewQueryPendingClaimsCmd(),
		NewQueryVerificationRoundCmd(),
		NewQueryDomainCmd(),
		NewQueryDomainsCmd(),
		NewQueryFactConfidenceCmd(),
		NewQueryFactCitationCountCmd(),
		NewQueryFactRelationsCmd(),
		NewQueryFactsBySubjectCmd(),
		NewQueryFactsByTagCmd(),
		NewQueryFactByCanonicalCmd(),
		NewQueryBootstrapFundStatusCmd(),
		NewQueryFactsAtRiskCmd(),
		NewQueryCheckNoveltyCmd(),
		NewQueryCommonKnowledgeCmd(),
		NewQueryBountiesCmd(),
		NewQueryDemandSignalsCmd(),
		NewQueryDemandGapsCmd(),
		NewQueryFactLineageCmd(),
		NewQueryFactProgenyCmd(),
		NewQueryNicheInfoCmd(),
		NewQueryNichesByDomainCmd(),
		NewQueryDomainDiversityCmd(),
		NewQueryDomainDiversityHistoryCmd(),
		NewQueryValidatorIndependenceCmd(),
		NewQueryConformityAlertsCmd(),
		NewQueryVindicationPendingCmd(),
		NewQueryVindicationRecordCmd(),
		NewQueryMetabolismStatusCmd(),
		NewQueryDomainCapacityCmd(),
	)

	return queryCmd
}

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

func NewQueryFactCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fact [id]",
		Short: "Query a fact by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			req := &types.QueryFactRequest{Id: args[0]}
			resp := &types.QueryFactResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/Fact", req, resp); err != nil {
				return fmt.Errorf("failed to query fact: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func NewQueryFactsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "facts",
		Short: "Query facts with optional filters",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			domain, _ := cmd.Flags().GetString("domain")
			status, _ := cmd.Flags().GetString("status")
			category, _ := cmd.Flags().GetString("category")
			claimTypeStr, _ := cmd.Flags().GetString("claim-type")
			claimType, err := parseClaimTypeQuery(claimTypeStr)
			if err != nil {
				return err
			}
			req := &types.QueryFactsRequest{
				Domain:    domain,
				Status:    status,
				Category:  category,
				ClaimType: claimType,
			}
			resp := &types.QueryFactsResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/Facts", req, resp); err != nil {
				return fmt.Errorf("failed to query facts: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	cmd.Flags().String("domain", "", "Filter by domain")
	cmd.Flags().String("status", "", "Filter by status")
	cmd.Flags().String("category", "", "Filter by category")
	cmd.Flags().String("claim-type", "", "Filter by claim type: assertion, relation, definition, constraint, negation, observation")
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func NewQueryFactsByDomainCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "facts-by-domain [domain]",
		Short: "Query facts by domain",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			req := &types.QueryFactsByDomainRequest{Domain: args[0]}
			resp := &types.QueryFactsByDomainResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/FactsByDomain", req, resp); err != nil {
				return fmt.Errorf("failed to query facts by domain: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func NewQueryFactsBySubmitterCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "facts-by-submitter [submitter]",
		Short: "Query facts by submitter address",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			req := &types.QueryFactsBySubmitterRequest{Submitter: args[0]}
			resp := &types.QueryFactsBySubmitterResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/FactsBySubmitter", req, resp); err != nil {
				return fmt.Errorf("failed to query facts by submitter: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func NewQueryClaimCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "claim [id]",
		Short: "Query a claim by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			req := &types.QueryClaimRequest{Id: args[0]}
			resp := &types.QueryClaimResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/Claim", req, resp); err != nil {
				return fmt.Errorf("failed to query claim: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func NewQueryPendingClaimsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pending-claims",
		Short: "Query all pending claims",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			req := &types.QueryPendingClaimsRequest{}
			resp := &types.QueryPendingClaimsResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/PendingClaims", req, resp); err != nil {
				return fmt.Errorf("failed to query pending claims: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func NewQueryVerificationRoundCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "verification-round [id]",
		Short: "Query a verification round by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			req := &types.QueryVerificationRoundRequest{Id: args[0]}
			resp := &types.QueryVerificationRoundResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/VerificationRound", req, resp); err != nil {
				return fmt.Errorf("failed to query verification round: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

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

func NewQueryFactConfidenceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fact-confidence [id]",
		Short: "Query the confidence score of a fact",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			req := &types.QueryFactConfidenceRequest{Id: args[0]}
			resp := &types.QueryFactConfidenceResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/FactConfidence", req, resp); err != nil {
				return fmt.Errorf("failed to query fact confidence: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func NewQueryFactCitationCountCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fact-citation-count [id]",
		Short: "Query the citation count of a fact",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			req := &types.QueryFactCitationCountRequest{Id: args[0]}
			resp := &types.QueryFactCitationCountResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/FactCitationCount", req, resp); err != nil {
				return fmt.Errorf("failed to query fact citation count: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// parseRelationTypeQuery maps a CLI string to a RelationType enum for query filtering.
func parseRelationTypeQuery(s string) (types.RelationType, error) {
	if s == "" {
		return types.RelationType_RELATION_TYPE_UNSPECIFIED, nil
	}
	switch strings.ToLower(s) {
	case "supports":
		return types.RelationType_RELATION_TYPE_SUPPORTS, nil
	case "contradicts":
		return types.RelationType_RELATION_TYPE_CONTRADICTS, nil
	case "requires":
		return types.RelationType_RELATION_TYPE_REQUIRES, nil
	case "refines":
		return types.RelationType_RELATION_TYPE_REFINES, nil
	case "generalizes":
		return types.RelationType_RELATION_TYPE_GENERALIZES, nil
	case "supersedes":
		return types.RelationType_RELATION_TYPE_SUPERSEDES, nil
	default:
		return 0, fmt.Errorf("unknown relation type %q: must be supports, contradicts, requires, refines, generalizes, or supersedes", s)
	}
}

func NewQueryFactRelationsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fact-relations [fact-id]",
		Short: "Query typed relations for a fact (knowledge graph edges)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			typeStr, _ := cmd.Flags().GetString("type")
			relType, err := parseRelationTypeQuery(typeStr)
			if err != nil {
				return err
			}
			direction, _ := cmd.Flags().GetString("direction")

			req := &types.QueryFactRelationsRequest{
				FactId:    args[0],
				Relation:  relType,
				Direction: direction,
			}
			resp := &types.QueryFactRelationsResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/FactRelations", req, resp); err != nil {
				return fmt.Errorf("failed to query fact relations: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	cmd.Flags().String("type", "", "Filter by relation type: supports, contradicts, requires, refines, generalizes, supersedes")
	cmd.Flags().String("direction", "both", "Direction: outgoing, incoming, or both")
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func NewQueryFactsBySubjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "facts-by-subject [domain] [subject]",
		Short: "Query facts by structured subject within a domain",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			req := &types.QueryFactsBySubjectRequest{Domain: args[0], Subject: args[1]}
			resp := &types.QueryFactsBySubjectResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/FactsBySubject", req, resp); err != nil {
				return fmt.Errorf("failed to query facts by subject: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func NewQueryFactsByTagCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "facts-by-tag [tag]",
		Short: "Query facts by a searchable tag",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			req := &types.QueryFactsByTagRequest{Tag: args[0]}
			resp := &types.QueryFactsByTagResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/FactsByTag", req, resp); err != nil {
				return fmt.Errorf("failed to query facts by tag: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func NewQueryFactByCanonicalCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fact-by-canonical [canonical-form-or-hash]",
		Short: "Query a fact by canonical form or canonical hash",
		Long:  "Looks up a fact by its canonical form (auto-hashed server-side) or by SHA-256 hex hash directly.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			input := args[0]
			req := &types.QueryFactByCanonicalRequest{}
			// If input looks like a hex hash (64 chars, all hex), treat as hash
			if len(input) == 64 && isHex(input) {
				req.CanonicalHash = input
			} else {
				req.CanonicalForm = input
			}
			resp := &types.QueryFactByCanonicalResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/FactByCanonical", req, resp); err != nil {
				return fmt.Errorf("failed to query fact by canonical: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// NewQueryBootstrapFundStatusCmd creates a CLI command for querying bootstrap fund status.
func NewQueryBootstrapFundStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bootstrap-fund-status",
		Short: "Query the knowledge bootstrap fund status",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			req := &types.QueryBootstrapFundStatusRequest{}
			resp := &types.QueryBootstrapFundStatusResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/BootstrapFundStatus", req, resp); err != nil {
				return fmt.Errorf("failed to query bootstrap fund status: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// NewQueryFactsAtRiskCmd creates a CLI command for querying facts at risk of expiry.
func NewQueryFactsAtRiskCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "facts-at-risk",
		Short: "Query facts whose energy has reached zero (at-risk of expiry)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			domain, _ := cmd.Flags().GetString("domain")
			limit, _ := cmd.Flags().GetUint64("limit")
			req := &types.QueryFactsAtRiskRequest{
				Domain: domain,
				Limit:  limit,
			}
			resp := &types.QueryFactsAtRiskResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/FactsAtRisk", req, resp); err != nil {
				return fmt.Errorf("failed to query facts at risk: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	cmd.Flags().String("domain", "", "Filter by domain")
	cmd.Flags().Uint64("limit", 50, "Max results")
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// isHex returns true if s contains only hexadecimal characters.
func isHex(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// NewQueryCheckNoveltyCmd creates a CLI command for checking novelty before submission.
func NewQueryCheckNoveltyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check-novelty [domain] [subject] [content]",
		Short: "Check novelty score for a claim before submission (free, no tx required)",
		Long:  "Preview the novelty score a claim would receive. Checks against common knowledge registry and existing facts.",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			req := &types.QueryCheckNoveltyRequest{
				Domain:  args[0],
				Subject: args[1],
				Content: args[2],
			}
			resp := &types.QueryCheckNoveltyResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/CheckNovelty", req, resp); err != nil {
				return fmt.Errorf("failed to check novelty: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// NewQueryCommonKnowledgeCmd creates a CLI command for querying the common knowledge registry.
func NewQueryCommonKnowledgeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "common-knowledge",
		Short: "Query the common knowledge registry",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			domain, _ := cmd.Flags().GetString("domain")
			req := &types.QueryCommonKnowledgeRequest{Domain: domain}
			resp := &types.QueryCommonKnowledgeResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/CommonKnowledge", req, resp); err != nil {
				return fmt.Errorf("failed to query common knowledge: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	cmd.Flags().String("domain", "", "Filter by domain")
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// NewQueryBountiesCmd creates a CLI command for querying active knowledge bounties.
func NewQueryBountiesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bounties",
		Short: "Query active knowledge bounties",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			domain, _ := cmd.Flags().GetString("domain")
			req := &types.QueryActiveBountiesRequest{Domain: domain}
			resp := &types.QueryActiveBountiesResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/ActiveBounties", req, resp); err != nil {
				return fmt.Errorf("failed to query active bounties: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	cmd.Flags().String("domain", "", "Filter by domain")
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// NewQueryDemandSignalsCmd creates a CLI command for querying demand signals.
func NewQueryDemandSignalsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "demand-signals",
		Short: "Query demand signals",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			domain, _ := cmd.Flags().GetString("domain")
			minUnfulfilled, _ := cmd.Flags().GetUint64("min-unfulfilled")
			req := &types.QueryDemandSignalsRequest{
				Domain:         domain,
				MinUnfulfilled: minUnfulfilled,
			}
			resp := &types.QueryDemandSignalsResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/DemandSignals", req, resp); err != nil {
				return fmt.Errorf("failed to query demand signals: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	cmd.Flags().String("domain", "", "Filter by domain")
	cmd.Flags().Uint64("min-unfulfilled", 0, "Minimum unfulfilled count")
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// NewQueryDemandGapsCmd creates a CLI command for querying top knowledge gaps.
func NewQueryDemandGapsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "demand-gaps",
		Short: "Query top knowledge gaps sorted by unfulfilled demand",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			limit, _ := cmd.Flags().GetUint64("limit")
			req := &types.QueryTopDemandGapsRequest{Limit: limit}
			resp := &types.QueryTopDemandGapsResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/TopDemandGaps", req, resp); err != nil {
				return fmt.Errorf("failed to query demand gaps: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	cmd.Flags().Uint64("limit", 20, "Max results")
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func NewQueryFactLineageCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fact-lineage [fact-id]",
		Short: "Query the lineage (ancestors) of a fact",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			depth, _ := cmd.Flags().GetUint64("depth")
			req := &types.QueryFactLineageRequest{FactId: args[0], Depth: depth}
			resp := &types.QueryFactLineageResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/FactLineage", req, resp); err != nil {
				return fmt.Errorf("failed to query fact lineage: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	cmd.Flags().Uint64("depth", 0, "How far up to trace (0 = to root)")
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func NewQueryFactProgenyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fact-progeny [fact-id]",
		Short: "Query the progeny (descendants) of a fact",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			depth, _ := cmd.Flags().GetUint64("depth")
			req := &types.QueryFactProgenyRequest{FactId: args[0], Depth: depth}
			resp := &types.QueryFactProgenyResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/FactProgeny", req, resp); err != nil {
				return fmt.Errorf("failed to query fact progeny: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	cmd.Flags().Uint64("depth", 0, "How deep to traverse (0 = default 3)")
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func NewQueryNicheInfoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "niche-info [niche-key]",
		Short: "Query information about a knowledge niche",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			req := &types.QueryNicheInfoRequest{NicheKey: args[0]}
			resp := &types.QueryNicheInfoResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/NicheInfo", req, resp); err != nil {
				return fmt.Errorf("failed to query niche info: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func NewQueryNichesByDomainCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "niches-by-domain [domain]",
		Short: "Query all niches within a domain",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			req := &types.QueryNichesByDomainRequest{Domain: args[0]}
			resp := &types.QueryNichesByDomainResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/NichesByDomain", req, resp); err != nil {
				return fmt.Errorf("failed to query niches by domain: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// NewQueryDomainDiversityCmd queries the current epoch diversity for a domain.
func NewQueryDomainDiversityCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "domain-diversity [domain]",
		Short: "Query consensus diversity for a domain (current epoch)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			req := &types.QueryDomainDiversityRequest{Domain: args[0]}
			resp := &types.QueryDomainDiversityResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/DomainDiversity", req, resp); err != nil {
				return fmt.Errorf("failed to query domain diversity: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// NewQueryDomainDiversityHistoryCmd queries historical diversity for a domain.
func NewQueryDomainDiversityHistoryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "domain-diversity-history [domain]",
		Short: "Query historical diversity for a domain",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			epochs, _ := cmd.Flags().GetUint64("epochs")
			req := &types.QueryDomainDiversityHistoryRequest{Domain: args[0], Epochs: epochs}
			resp := &types.QueryDomainDiversityHistoryResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/DomainDiversityHistory", req, resp); err != nil {
				return fmt.Errorf("failed to query domain diversity history: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	cmd.Flags().Uint64("epochs", 10, "Number of epochs to look back")
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// NewQueryValidatorIndependenceCmd queries a validator's independence score.
func NewQueryValidatorIndependenceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validator-independence [validator-addr]",
		Short: "Query a validator's independence score (how often they dissent)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			req := &types.QueryValidatorIndependenceRequest{Validator: args[0]}
			resp := &types.QueryValidatorIndependenceResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/ValidatorIndependence", req, resp); err != nil {
				return fmt.Errorf("failed to query validator independence: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// NewQueryConformityAlertsCmd queries active conformity alerts across domains.
func NewQueryConformityAlertsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "conformity-alerts",
		Short: "Query domains with active conformity alerts (sustained low diversity)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			req := &types.QueryConformityAlertsRequest{}
			resp := &types.QueryConformityAlertsResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/ConformityAlerts", req, resp); err != nil {
				return fmt.Errorf("failed to query conformity alerts: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// NewQueryVindicationPendingCmd queries pending vindication entries for a fact
// using a raw ABCI store query (no proto gRPC endpoint registered).
func NewQueryVindicationPendingCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vindication-pending [fact-id]",
		Short: "Query pending vindication entries for a fact",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			factId := args[0]
			key := types.VindicationPendingKey(factId)

			bz, _, err := clientCtx.QueryStore(key, types.StoreKey)
			if err != nil {
				return fmt.Errorf("failed to query vindication pending: %w", err)
			}
			if len(bz) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No pending vindication entries for fact %s\n", factId)
				return nil
			}

			var entries []types.VindicationEntry
			if err := json.Unmarshal(bz, &entries); err != nil {
				return fmt.Errorf("failed to unmarshal vindication entries: %w", err)
			}

			out, err := json.MarshalIndent(entries, "", "  ")
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(out))
			return nil
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// NewQueryVindicationRecordCmd queries vindication records for a specific fact and verifier
// using a raw ABCI store query (no proto gRPC endpoint registered).
func NewQueryVindicationRecordCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vindication-record [fact-id] [verifier]",
		Short: "Query vindication record for a fact and verifier",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			factId := args[0]
			verifier := args[1]
			key := types.VindicationRecordKey(factId, verifier)

			bz, _, err := clientCtx.QueryStore(key, types.StoreKey)
			if err != nil {
				return fmt.Errorf("failed to query vindication record: %w", err)
			}
			if len(bz) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No vindication record for fact %s verifier %s\n", factId, verifier)
				return nil
			}

			var record types.VindicationRecord
			if err := json.Unmarshal(bz, &record); err != nil {
				return fmt.Errorf("failed to unmarshal vindication record: %w", err)
			}

			out, err := json.MarshalIndent(record, "", "  ")
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(out))
			return nil
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// NewQueryDomainCapacityCmd creates a CLI command for querying domain carrying capacity and pressure.
func NewQueryDomainCapacityCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "domain-capacity [domain]",
		Short: "Query carrying capacity and pressure for a domain",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			req := &types.QueryDomainCapacityRequest{Domain: args[0]}
			resp := &types.QueryDomainCapacityResponse{}

			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/DomainCapacity", req, resp); err != nil {
				return fmt.Errorf("failed to query domain capacity: %w", err)
			}

			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// NewQueryMetabolismStatusCmd creates a CLI command for querying aggregate metabolism health statistics.
func NewQueryMetabolismStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "metabolism-status",
		Short: "Query aggregate metabolism health statistics",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			req := &types.QueryMetabolismStatusRequest{}
			resp := &types.QueryMetabolismStatusResponse{}

			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/MetabolismStatus", req, resp); err != nil {
				return err
			}

			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}
