package cli

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// parseConsentType maps a CLI string to a ConsentType enum value.
func parseConsentType(s string) (types.ConsentType, error) {
	switch strings.ToLower(s) {
	case "", "unspecified":
		return types.ConsentType_CONSENT_TYPE_UNSPECIFIED, nil
	case "self", "self_authored":
		return types.ConsentType_CONSENT_TYPE_SELF_AUTHORED, nil
	case "opt_in", "optin":
		return types.ConsentType_CONSENT_TYPE_OPT_IN, nil
	case "public_license", "public":
		return types.ConsentType_CONSENT_TYPE_PUBLIC_LICENSE, nil
	case "platform_tos", "tos":
		return types.ConsentType_CONSENT_TYPE_PLATFORM_TOS, nil
	case "fair_use", "fairuse":
		return types.ConsentType_CONSENT_TYPE_FAIR_USE, nil
	default:
		return 0, fmt.Errorf("unknown consent type %q: must be self, optin, public, tos, or fairuse", s)
	}
}

// parseContestType maps a CLI string to a ContestType enum value.
func parseContestType(s string) (types.ContestType, error) {
	switch strings.ToLower(s) {
	case "", "unspecified":
		return types.ContestType_CONTEST_TYPE_UNSPECIFIED, nil
	case "consent":
		return types.ContestType_CONTEST_TYPE_CONSENT, nil
	case "quality":
		return types.ContestType_CONTEST_TYPE_QUALITY, nil
	case "duplicate":
		return types.ContestType_CONTEST_TYPE_DUPLICATE, nil
	case "toxic":
		return types.ContestType_CONTEST_TYPE_TOXIC, nil
	case "copyright":
		return types.ContestType_CONTEST_TYPE_COPYRIGHT, nil
	default:
		return 0, fmt.Errorf("unknown contest type %q: must be consent, quality, duplicate, toxic, or copyright", s)
	}
}

// GetTxCmd returns the root transaction command for the knowledge module.
func GetTxCmd() *cobra.Command {
	txCmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "Knowledge module transaction commands",
		DisableFlagParsing:         false,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	txCmd.AddCommand(
		NewSubmitDataCmd(),
		NewSubmitThreadCmd(),
		NewSubmitCommitmentCmd(),
		NewSubmitRevealCmd(),
		NewContestSampleCmd(),
		NewSponsorSampleCmd(),
		NewProposeDomainCmd(),
		NewEndorseDomainProposalCmd(),
		NewCreateDatasetCmd(),
		NewAccessDatasetCmd(),
		NewAccessSampleCmd(),
		NewReportDemandCmd(),
		NewFundBountyCmd(),
		NewRateSampleCmd(),
		NewAddScrapedSourceCmd(),
		NewRemoveScrapedSourceCmd(),
		NewProposeResearchFundCmd(),
		NewVoteResearchProposalCmd(),
		NewExecuteResearchProposalCmd(),
		NewAddSampleCmd(),
	)

	return txCmd
}

// NewSubmitDataCmd creates a CLI command for MsgSubmitData.
func NewSubmitDataCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "submit-data [content] [domain] [stake]",
		Short: "Submit training data for quality review",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			sampleTypeStr, _ := cmd.Flags().GetString("sample-type")
			sampleType, err := parseSampleType(sampleTypeStr)
			if err != nil {
				return err
			}

			sourceUri, _ := cmd.Flags().GetString("source-uri")
			sourcePlatform, _ := cmd.Flags().GetString("source-platform")
			sourceTimestamp, _ := cmd.Flags().GetUint64("source-timestamp")
			originalAuthor, _ := cmd.Flags().GetString("original-author")
			license, _ := cmd.Flags().GetString("license")
			language, _ := cmd.Flags().GetString("language")
			parentSubmissionId, _ := cmd.Flags().GetString("parent-submission-id")
			threadId, _ := cmd.Flags().GetString("thread-id")
			sponsored, _ := cmd.Flags().GetBool("sponsored")

			tagsStr, _ := cmd.Flags().GetString("tags")
			var tags []string
			if tagsStr != "" {
				tags = strings.Split(tagsStr, ",")
			}

			contextIdsStr, _ := cmd.Flags().GetString("context-ids")
			var contextIds []string
			if contextIdsStr != "" {
				contextIds = strings.Split(contextIdsStr, ",")
			}

			// Build consent proof from flags
			var consent *types.ConsentProof
			consentTypeStr, _ := cmd.Flags().GetString("consent-type")
			if consentTypeStr != "" {
				consentType, err := parseConsentType(consentTypeStr)
				if err != nil {
					return err
				}
				proofUri, _ := cmd.Flags().GetString("consent-proof-uri")
				authorSig, _ := cmd.Flags().GetString("consent-author-signature")
				consentTimestamp, _ := cmd.Flags().GetUint64("consent-timestamp")
				consentTerms, _ := cmd.Flags().GetString("consent-terms")
				consent = &types.ConsentProof{
					Type:             consentType,
					ProofUri:         proofUri,
					AuthorSignature:  authorSig,
					ConsentTimestamp: consentTimestamp,
					ConsentTerms:     consentTerms,
				}
			}

			msg := &types.MsgSubmitData{
				Submitter:          clientCtx.GetFromAddress().String(),
				Content:            args[0],
				SampleType:         sampleType,
				Domain:             args[1],
				SourceUri:          sourceUri,
				SourcePlatform:     sourcePlatform,
				SourceTimestamp:    sourceTimestamp,
				Consent:            consent,
				OriginalAuthor:     originalAuthor,
				License:            license,
				Tags:               tags,
				Language:           language,
				Stake:              args[2],
				ParentSubmissionId: parentSubmissionId,
				ThreadId:           threadId,
				ContextIds:         contextIds,
				Sponsored:          sponsored,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	cmd.Flags().String("sample-type", "", "Sample type: discussion, debate, explanation, troubleshoot, review, tutorial, opinion, narrative, qanda, creative, annotation, correction")
	cmd.Flags().String("source-uri", "", "URI of the original source")
	cmd.Flags().String("source-platform", "", "Platform name (e.g. reddit, stackoverflow)")
	cmd.Flags().Uint64("source-timestamp", 0, "Unix timestamp of original content")
	cmd.Flags().String("original-author", "", "Original author identifier")
	cmd.Flags().String("license", "", "Content license (e.g. CC-BY-4.0)")
	cmd.Flags().String("language", "", "Language code (e.g. en, es)")
	cmd.Flags().String("tags", "", "Comma-separated tags")
	cmd.Flags().String("parent-submission-id", "", "Parent submission ID (for thread replies)")
	cmd.Flags().String("thread-id", "", "Thread ID to append to")
	cmd.Flags().String("context-ids", "", "Comma-separated context submission IDs")
	cmd.Flags().Bool("sponsored", false, "Request bootstrap fund sponsorship")
	cmd.Flags().String("consent-type", "", "Consent type: self, optin, public, tos, fairuse")
	cmd.Flags().String("consent-proof-uri", "", "URI to consent evidence")
	cmd.Flags().String("consent-author-signature", "", "Cryptographic author consent signature")
	cmd.Flags().Uint64("consent-timestamp", 0, "Unix timestamp of consent")
	cmd.Flags().String("consent-terms", "", "Description of what was consented to")
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewSubmitThreadCmd creates a CLI command for MsgSubmitThread.
func NewSubmitThreadCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "submit-thread [domain] [stake]",
		Short: "Submit a conversation thread (items provided via --items JSON file or stdin)",
		Long: `Submit a multi-turn conversation thread. The thread items are embedded MsgSubmitData messages.
Due to the complexity of nested messages, this command accepts domain and stake as arguments.
Individual thread items should be submitted via submit-data with --thread-id after creating the thread.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			threadId, _ := cmd.Flags().GetString("thread-id")

			msg := &types.MsgSubmitThread{
				Submitter: clientCtx.GetFromAddress().String(),
				Domain:    args[0],
				Stake:     args[1],
				ThreadId:  threadId,
				Items:     nil, // Items are added via individual submit-data calls with --thread-id
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	cmd.Flags().String("thread-id", "", "Explicit thread ID (auto-generated if omitted)")
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewSubmitCommitmentCmd creates a CLI command for MsgSubmitCommitment.
func NewSubmitCommitmentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "submit-commitment [round-id] [commit-hash-hex]",
		Short: "Submit a quality review commitment (commit-reveal phase 1)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			commitHash, err := hex.DecodeString(args[1])
			if err != nil {
				return fmt.Errorf("invalid commit hash hex: %w", err)
			}

			msg := &types.MsgSubmitCommitment{
				Verifier:   clientCtx.GetFromAddress().String(),
				RoundId:    args[0],
				CommitHash: commitHash,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewSubmitRevealCmd creates a CLI command for MsgSubmitReveal.
func NewSubmitRevealCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "submit-reveal [round-id] [overall-quality] [salt-hex]",
		Short: "Submit a quality review reveal (commit-reveal phase 2)",
		Long: `Reveal quality scores for a sample review round.
overall-quality is a uint64 score (0-1000000). Salt is the hex-encoded salt used in commitment.
Additional score dimensions can be set via flags.`,
		Args: cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			overallQuality, err := strconv.ParseUint(args[1], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid overall-quality: %w", err)
			}

			salt, err := hex.DecodeString(args[2])
			if err != nil {
				return fmt.Errorf("invalid salt hex: %w", err)
			}

			reasoningDepth, _ := cmd.Flags().GetUint64("reasoning-depth")
			novelty, _ := cmd.Flags().GetUint64("novelty")
			toxicity, _ := cmd.Flags().GetUint64("toxicity")
			factualAccuracy, _ := cmd.Flags().GetUint64("factual-accuracy")
			consentValid, _ := cmd.Flags().GetBool("consent-valid")
			duplicate, _ := cmd.Flags().GetBool("duplicate")
			notes, _ := cmd.Flags().GetString("notes")

			scores := &types.QualityVote{
				OverallQuality:  overallQuality,
				ReasoningDepth:  reasoningDepth,
				Novelty:         novelty,
				Toxicity:        toxicity,
				FactualAccuracy: factualAccuracy,
				ConsentValid:    consentValid,
				Duplicate:       duplicate,
				Notes:           notes,
			}

			msg := &types.MsgSubmitReveal{
				Verifier: clientCtx.GetFromAddress().String(),
				RoundId:  args[0],
				Scores:   scores,
				Salt:     salt,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	cmd.Flags().Uint64("reasoning-depth", 0, "Reasoning depth score (0-1000000)")
	cmd.Flags().Uint64("novelty", 0, "Novelty score (0-1000000)")
	cmd.Flags().Uint64("toxicity", 0, "Toxicity score (0-1000000, higher = more toxic)")
	cmd.Flags().Uint64("factual-accuracy", 0, "Factual accuracy score (0-1000000)")
	cmd.Flags().Bool("consent-valid", true, "Whether consent proof is valid")
	cmd.Flags().Bool("duplicate", false, "Whether sample is a duplicate")
	cmd.Flags().String("notes", "", "Reviewer notes")
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewContestSampleCmd creates a CLI command for MsgContestSample.
func NewContestSampleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "contest-sample [sample-id] [stake] [reason]",
		Short: "Contest a sample's quality score or consent validity",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			contestTypeStr, _ := cmd.Flags().GetString("contest-type")
			contestType, err := parseContestType(contestTypeStr)
			if err != nil {
				return err
			}

			msg := &types.MsgContestSample{
				Challenger:  clientCtx.GetFromAddress().String(),
				SampleId:    args[0],
				Stake:       args[1],
				Reason:      args[2],
				ContestType: contestType,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	cmd.Flags().String("contest-type", "", "Contest type: consent, quality, duplicate, toxic, copyright")
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewSponsorSampleCmd creates a CLI command for MsgSponsorSample.
func NewSponsorSampleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sponsor-sample [sample-id] [amount] [duration-blocks]",
		Short: "Sponsor a sample with staking support",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			durationBlocks, err := strconv.ParseUint(args[2], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid duration-blocks: %w", err)
			}

			msg := &types.MsgSponsorSample{
				Sponsor:        clientCtx.GetFromAddress().String(),
				SampleId:       args[0],
				Amount:         args[1],
				DurationBlocks: durationBlocks,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewProposeDomainCmd creates a CLI command for MsgProposeDomain.
func NewProposeDomainCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "propose-domain [name] [description] [stratum] [stake]",
		Short: "Propose a new knowledge domain",
		Args:  cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := &types.MsgProposeDomain{
				Proposer:    clientCtx.GetFromAddress().String(),
				Name:        args[0],
				Description: args[1],
				Stratum:     args[2],
				Stake:       args[3],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewEndorseDomainProposalCmd creates a CLI command for MsgEndorseDomainProposal.
func NewEndorseDomainProposalCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "endorse-domain [proposal-id]",
		Short: "Endorse a domain proposal",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := &types.MsgEndorseDomainProposal{
				Endorser:   clientCtx.GetFromAddress().String(),
				ProposalId: args[0],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewCreateDatasetCmd creates a CLI command for MsgCreateDataset.
func NewCreateDatasetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create-dataset [name] [description] [domain]",
		Short: "Create a curated dataset from samples",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			license, _ := cmd.Flags().GetString("license")
			filterLanguage, _ := cmd.Flags().GetString("filter-language")
			minQuality, _ := cmd.Flags().GetUint64("min-quality")
			pricePerSample, _ := cmd.Flags().GetString("price-per-sample")
			bulkPrice, _ := cmd.Flags().GetString("bulk-price")

			filterTypeStr, _ := cmd.Flags().GetString("filter-type")
			filterType, err := parseSampleType(filterTypeStr)
			if err != nil {
				return err
			}

			filterTagsStr, _ := cmd.Flags().GetString("filter-tags")
			var filterTags []string
			if filterTagsStr != "" {
				filterTags = strings.Split(filterTagsStr, ",")
			}

			msg := &types.MsgCreateDataset{
				Curator:        clientCtx.GetFromAddress().String(),
				Name:           args[0],
				Description:    args[1],
				Domain:         args[2],
				License:        license,
				FilterType:     filterType,
				FilterLanguage: filterLanguage,
				FilterTags:     filterTags,
				MinQuality:     minQuality,
				PricePerSample: pricePerSample,
				BulkPrice:      bulkPrice,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	cmd.Flags().String("license", "", "Dataset license")
	cmd.Flags().String("filter-type", "", "Filter by sample type")
	cmd.Flags().String("filter-language", "", "Filter by language code")
	cmd.Flags().String("filter-tags", "", "Comma-separated tags to filter by")
	cmd.Flags().Uint64("min-quality", 0, "Minimum quality score threshold")
	cmd.Flags().String("price-per-sample", "0", "Price per sample access (uzrn)")
	cmd.Flags().String("bulk-price", "0", "Bulk access price (uzrn)")
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewAccessDatasetCmd creates a CLI command for MsgAccessDataset.
func NewAccessDatasetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "access-dataset [dataset-id] [max-payment]",
		Short: "Purchase access to a dataset",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := &types.MsgAccessDataset{
				Consumer:   clientCtx.GetFromAddress().String(),
				DatasetId:  args[0],
				MaxPayment: args[1],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewAccessSampleCmd creates a CLI command for MsgAccessSample.
func NewAccessSampleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "access-sample [sample-id] [max-payment]",
		Short: "Purchase access to a single sample",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := &types.MsgAccessSample{
				Consumer:   clientCtx.GetFromAddress().String(),
				SampleId:   args[0],
				MaxPayment: args[1],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewReportDemandCmd creates a CLI command for MsgReportDemand.
func NewReportDemandCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "report-demand [domain] [subject] [queries] [fulfilled] [unfulfilled]",
		Short: "Report training data demand signals",
		Long:  `Report demand for training data in a specific domain. Queries, fulfilled, and unfulfilled are counts.`,
		Args:  cobra.ExactArgs(5),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			queries, err := strconv.ParseUint(args[2], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid queries count: %w", err)
			}
			fulfilled, err := strconv.ParseUint(args[3], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid fulfilled count: %w", err)
			}
			unfulfilled, err := strconv.ParseUint(args[4], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid unfulfilled count: %w", err)
			}

			report := &types.DemandReport{
				Domain:      args[0],
				Subject:     args[1],
				Queries:     queries,
				Fulfilled:   fulfilled,
				Unfulfilled: unfulfilled,
			}

			msg := &types.MsgReportDemand{
				Reporter: clientCtx.GetFromAddress().String(),
				Reports:  []*types.DemandReport{report},
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewFundBountyCmd creates a CLI command for MsgFundBounty.
func NewFundBountyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fund-bounty [domain] [topic] [amount] [expires-blocks]",
		Short: "Fund a data collection bounty",
		Args:  cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			expiresBlocks, err := strconv.ParseUint(args[3], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid expires-blocks: %w", err)
			}

			preferredTypeStr, _ := cmd.Flags().GetString("preferred-type")
			preferredType, err := parseSampleType(preferredTypeStr)
			if err != nil {
				return err
			}

			language, _ := cmd.Flags().GetString("language")

			msg := &types.MsgFundBounty{
				Funder:        clientCtx.GetFromAddress().String(),
				Domain:        args[0],
				Topic:         args[1],
				PreferredType: preferredType,
				Language:      language,
				Amount:        args[2],
				ExpiresBlocks: expiresBlocks,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	cmd.Flags().String("preferred-type", "", "Preferred sample type")
	cmd.Flags().String("language", "", "Preferred language code")
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewRateSampleCmd creates a CLI command for MsgRateSample.
func NewRateSampleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rate-sample [sample-id] [useful:true/false]",
		Short: "Rate a sample's usefulness",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			useful, err := strconv.ParseBool(args[1])
			if err != nil {
				return fmt.Errorf("invalid useful flag: %w", err)
			}

			memo, _ := cmd.Flags().GetString("memo")

			msg := &types.MsgRateSample{
				Rater:    clientCtx.GetFromAddress().String(),
				SampleId: args[0],
				Useful:   useful,
				Memo:     memo,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	cmd.Flags().String("memo", "", "Optional rating memo")
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewAddScrapedSourceCmd creates a CLI command for MsgAddScrapedSource.
func NewAddScrapedSourceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add-scraped-source [platform] [domain] [description]",
		Short: "Register a scraped data source (authority only)",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			noveltyPenalty, _ := cmd.Flags().GetUint64("novelty-penalty")

			msg := &types.MsgAddScrapedSource{
				Authority:      clientCtx.GetFromAddress().String(),
				Platform:       args[0],
				Domain:         args[1],
				Description:    args[2],
				NoveltyPenalty: noveltyPenalty,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	cmd.Flags().Uint64("novelty-penalty", 0, "Novelty penalty percentage (0-100)")
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewRemoveScrapedSourceCmd creates a CLI command for MsgRemoveScrapedSource.
func NewRemoveScrapedSourceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove-scraped-source [id]",
		Short: "Remove a scraped data source (authority only)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := &types.MsgRemoveScrapedSource{
				Authority: clientCtx.GetFromAddress().String(),
				Id:        args[0],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewProposeResearchFundCmd creates a CLI command for MsgProposeResearchFund.
func NewProposeResearchFundCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "propose-research-fund [title] [description] [amount] [recipient] [voting-period-blocks]",
		Short: "Propose a research fund allocation",
		Args:  cobra.ExactArgs(5),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			votingPeriod, err := strconv.ParseUint(args[4], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid voting-period-blocks: %w", err)
			}

			msg := &types.MsgProposeResearchFund{
				Proposer:           clientCtx.GetFromAddress().String(),
				Title:              args[0],
				Description:        args[1],
				Amount:             args[2],
				Recipient:          args[3],
				VotingPeriodBlocks: votingPeriod,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewVoteResearchProposalCmd creates a CLI command for MsgVoteResearchProposal.
func NewVoteResearchProposalCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vote-research-proposal [proposal-id] [vote:true/false]",
		Short: "Vote on a research fund proposal",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			vote, err := strconv.ParseBool(args[1])
			if err != nil {
				return fmt.Errorf("invalid vote: %w", err)
			}

			msg := &types.MsgVoteResearchProposal{
				Voter:      clientCtx.GetFromAddress().String(),
				ProposalId: args[0],
				Vote:       vote,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewExecuteResearchProposalCmd creates a CLI command for MsgExecuteResearchProposal.
func NewExecuteResearchProposalCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "execute-research-proposal [proposal-id]",
		Short: "Execute an approved research fund proposal (authority only)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := &types.MsgExecuteResearchProposal{
				Authority:  clientCtx.GetFromAddress().String(),
				ProposalId: args[0],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewAddSampleCmd creates a CLI command for MsgAddSample.
func NewAddSampleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add-sample [content] [domain] [quality-score]",
		Short: "Add a sample directly (authority only)",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			qualityScore, err := strconv.ParseUint(args[2], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid quality-score: %w", err)
			}

			sampleTypeStr, _ := cmd.Flags().GetString("sample-type")
			sampleType, err := parseSampleType(sampleTypeStr)
			if err != nil {
				return err
			}

			sourceUri, _ := cmd.Flags().GetString("source-uri")
			originalAuthor, _ := cmd.Flags().GetString("original-author")
			license, _ := cmd.Flags().GetString("license")

			msg := &types.MsgAddSample{
				Authority:      clientCtx.GetFromAddress().String(),
				Content:        args[0],
				SampleType:     sampleType,
				Domain:         args[1],
				SourceUri:      sourceUri,
				OriginalAuthor: originalAuthor,
				License:        license,
				QualityScore:   qualityScore,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	cmd.Flags().String("sample-type", "", "Sample type: discussion, debate, explanation, etc.")
	cmd.Flags().String("source-uri", "", "URI of the original source")
	cmd.Flags().String("original-author", "", "Original author identifier")
	cmd.Flags().String("license", "", "Content license")
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}
