package cli

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
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

// parseTDUType maps a TDU type string from the R41 CLI to a SampleType enum.
func parseTDUType(s string) (types.SampleType, error) {
	switch strings.ToLower(strings.ReplaceAll(s, "-", "_")) {
	case "instruction_response", "instruction-response":
		return types.SampleType_SAMPLE_TYPE_Q_AND_A, nil
	case "conversation":
		return types.SampleType_SAMPLE_TYPE_DISCUSSION, nil
	case "correction":
		return types.SampleType_SAMPLE_TYPE_CORRECTION, nil
	case "grounding_fact", "grounding-fact":
		return types.SampleType_SAMPLE_TYPE_ANNOTATION, nil
	case "reasoning_chain", "reasoning-chain":
		return types.SampleType_SAMPLE_TYPE_EXPLANATION, nil
	default:
		return 0, fmt.Errorf("unknown TDU type %q: must be instruction-response, conversation, correction, grounding-fact, or reasoning-chain", s)
	}
}

// difficultyMultiplier returns a ×10 integer multiplier for the given difficulty level.
// basic=10 (1×), standard=15 (1.5×), advanced=20 (2×), expert=30 (3×), frontier=50 (5×).
func difficultyMultiplier(s string) (int64, error) {
	switch strings.ToLower(s) {
	case "", "basic":
		return 10, nil
	case "standard":
		return 15, nil
	case "advanced":
		return 20, nil
	case "expert":
		return 30, nil
	case "frontier":
		return 50, nil
	default:
		return 0, fmt.Errorf("unknown difficulty %q: must be basic, standard, advanced, expert, or frontier", s)
	}
}

// calculateStake computes stake = (baseStake × multiplierX10) / 10.
func calculateStake(baseStake string, multiplierX10 int64) (string, error) {
	base := new(big.Int)
	if _, ok := base.SetString(baseStake, 10); !ok {
		return "", fmt.Errorf("invalid base stake: %s", baseStake)
	}
	result := new(big.Int).Mul(base, big.NewInt(multiplierX10))
	result.Div(result, big.NewInt(10))
	return result.String(), nil
}

// parseR41ConsentType maps spec consent-type names to ConsentType.
func parseR41ConsentType(s string) (types.ConsentType, error) {
	switch strings.ToLower(strings.ReplaceAll(s, "-", "_")) {
	case "original", "self":
		return types.ConsentType_CONSENT_TYPE_SELF_AUTHORED, nil
	case "public_domain", "public-domain":
		return types.ConsentType_CONSENT_TYPE_PUBLIC_LICENSE, nil
	case "licensed":
		return types.ConsentType_CONSENT_TYPE_OPT_IN, nil
	default:
		// Fall through to the existing parser for backwards compat
		return parseConsentType(s)
	}
}

// contentHashHex computes the SHA-256 hash of data and returns the hex string.
func contentHashHex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// maxContentFileSize is the maximum content file size (1MB).
const maxContentFileSize = 1_048_576

// defaultBaseStake is the fallback base stake when chain params can't be queried.
const defaultBaseStake = "1000000"

// threadTurn represents a single turn in a conversation thread file.
type threadTurn struct {
	Role    string `json:"role"`
	Content string `json:"content"`
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
		NewSubmitCorrectionCmd(),
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
		NewCommitReviewCmd(),
		NewRevealReviewCmd(),
		NewAttestStorageCmd(),
	)

	return txCmd
}

// NewSubmitDataCmd creates a CLI command for MsgSubmitData.
// Supports both legacy positional args and R41 flag-based interface.
func NewSubmitDataCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "submit-data",
		Short: "Submit training data for quality review",
		Long: `Submit a single TDU (training data unit) for quality review.

Example (R41 file-based):
  zeroned tx knowledge submit-data \
    --type instruction-response \
    --domain code \
    --difficulty standard \
    --content-file ./my-training-pair.json \
    --consent-type original \
    --from agent1

Example (legacy inline):
  zeroned tx knowledge submit-data [content] [domain] [stake] --consent-type self --from agent1

When --content-file is provided, the content hash is computed client-side (SHA-256)
and the stake is auto-calculated from base_stake × difficulty_multiplier.`,
		Args: cobra.MaximumNArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			var content string
			var domain string
			var stake string
			var sampleType types.SampleType
			var hashHex string

			contentFile, _ := cmd.Flags().GetString("content-file")

			if contentFile != "" {
				// ─── R41 file-based mode ───────────────────────────────
				contentBytes, err := os.ReadFile(contentFile)
				if err != nil {
					return fmt.Errorf("failed to read content file: %w", err)
				}
				if len(contentBytes) > maxContentFileSize {
					return fmt.Errorf("content file exceeds maximum size of 1MB (%d bytes)", len(contentBytes))
				}
				content = string(contentBytes)
				hashHex = contentHashHex(contentBytes)

				// Parse TDU type
				tduTypeStr, _ := cmd.Flags().GetString("type")
				if tduTypeStr == "" {
					return fmt.Errorf("--type is required when using --content-file")
				}
				sampleType, err = parseTDUType(tduTypeStr)
				if err != nil {
					return err
				}

				// Domain from flag
				domain, _ = cmd.Flags().GetString("domain")
				if domain == "" {
					return fmt.Errorf("--domain is required when using --content-file")
				}

				// Auto-calculate stake from difficulty
				stakeOverride, _ := cmd.Flags().GetString("stake")
				if stakeOverride != "" {
					stake = stakeOverride
				} else {
					diffStr, _ := cmd.Flags().GetString("difficulty")
					mult, err := difficultyMultiplier(diffStr)
					if err != nil {
						return err
					}

					// Try to query chain params for base_stake
					baseStake := defaultBaseStake
					queryClient := types.NewQueryClient(clientCtx)
					paramsRes, err := queryClient.Params(cmd.Context(), &types.QueryParamsRequest{})
					if err == nil && paramsRes.Params.MinSubmissionStake != "" {
						baseStake = paramsRes.Params.MinSubmissionStake
					}

					stake, err = calculateStake(baseStake, mult)
					if err != nil {
						return err
					}
				}
			} else {
				// ─── Legacy positional args mode ──────────────────────
				if len(args) < 3 {
					return fmt.Errorf("requires either --content-file or 3 positional args: [content] [domain] [stake]")
				}
				content = args[0]
				domain = args[1]
				stake = args[2]
				hashHex = contentHashHex([]byte(content))

				sampleTypeStr, _ := cmd.Flags().GetString("sample-type")
				if sampleTypeStr == "" {
					sampleTypeStr, _ = cmd.Flags().GetString("type")
				}
				if sampleTypeStr != "" {
					// Try R41 type names first, fall back to legacy
					st, err := parseTDUType(sampleTypeStr)
					if err != nil {
						st, err = parseSampleType(sampleTypeStr)
						if err != nil {
							return err
						}
					}
					sampleType = st
				}
			}

			// Common optional flags
			sourceUri, _ := cmd.Flags().GetString("source-uri")
			sourcePlatform, _ := cmd.Flags().GetString("source-platform")
			sourceTimestamp, _ := cmd.Flags().GetUint64("source-timestamp")
			originalAuthor, _ := cmd.Flags().GetString("original-author")
			license, _ := cmd.Flags().GetString("license")
			language, _ := cmd.Flags().GetString("language")
			parentSubmissionId, _ := cmd.Flags().GetString("parent-submission-id")
			threadId, _ := cmd.Flags().GetString("thread-id")
			sponsored, _ := cmd.Flags().GetBool("sponsored")
			metadata, _ := cmd.Flags().GetString("metadata")

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

			// Build consent proof
			var consent *types.ConsentProof
			consentTypeStr, _ := cmd.Flags().GetString("consent-type")
			if consentTypeStr != "" {
				consentType, err := parseR41ConsentType(consentTypeStr)
				if err != nil {
					return err
				}
				proofUri, _ := cmd.Flags().GetString("consent-proof")
				if proofUri == "" {
					proofUri, _ = cmd.Flags().GetString("consent-proof-uri")
				}
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

			// Append metadata to tags if provided
			if metadata != "" {
				tags = append(tags, "metadata:"+metadata)
			}

			// Display computed info
			fmt.Fprintf(cmd.ErrOrStderr(), "Content hash:  %s\n", hashHex)
			fmt.Fprintf(cmd.ErrOrStderr(), "Stake amount:  %s uzrn\n", stake)
			fmt.Fprintf(cmd.ErrOrStderr(), "Domain:        %s\n", domain)

			msg := &types.MsgSubmitData{
				Submitter:          clientCtx.GetFromAddress().String(),
				Content:            content,
				SampleType:         sampleType,
				Domain:             domain,
				SourceUri:          sourceUri,
				SourcePlatform:     sourcePlatform,
				SourceTimestamp:    sourceTimestamp,
				Consent:            consent,
				OriginalAuthor:     originalAuthor,
				License:            license,
				Tags:               tags,
				Language:           language,
				Stake:              stake,
				ParentSubmissionId: parentSubmissionId,
				ThreadId:           threadId,
				ContextIds:         contextIds,
				Sponsored:          sponsored,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	// R41 flags
	cmd.Flags().String("type", "", "TDU type: instruction-response, conversation, correction, grounding-fact, reasoning-chain")
	cmd.Flags().String("domain", "", "Target domain (e.g. code, math, general)")
	cmd.Flags().String("difficulty", "basic", "Difficulty: basic (1×), standard (1.5×), advanced (2×), expert (3×), frontier (5×)")
	cmd.Flags().String("content-file", "", "Path to JSON content file")
	cmd.Flags().String("consent-proof", "", "Path to consent proof file")
	cmd.Flags().String("metadata", "", "Optional JSON metadata string")
	cmd.Flags().String("stake", "", "Override auto-calculated stake (uzrn)")
	// Legacy / common flags
	cmd.Flags().String("sample-type", "", "Legacy sample type (use --type for R41)")
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
	cmd.Flags().String("consent-type", "", "Consent type: original, public-domain, licensed (or legacy: self, optin, public, tos, fairuse)")
	cmd.Flags().String("consent-proof-uri", "", "URI to consent evidence")
	cmd.Flags().String("consent-author-signature", "", "Cryptographic author consent signature")
	cmd.Flags().Uint64("consent-timestamp", 0, "Unix timestamp of consent")
	cmd.Flags().String("consent-terms", "", "Description of what was consented to")
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewSubmitThreadCmd creates a CLI command for MsgSubmitThread.
// Supports both R41 --thread-file and legacy positional args.
func NewSubmitThreadCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "submit-thread",
		Short: "Submit a multi-turn conversation as a TDU",
		Long: `Submit a multi-turn conversation thread for quality review.

Example (R41 file-based):
  zeroned tx knowledge submit-thread \
    --domain code \
    --thread-file ./conversation.json \
    --from agent1

Thread file format: JSON array of {"role": "user"|"assistant", "content": "..."} turns.

Example (legacy):
  zeroned tx knowledge submit-thread [domain] [stake] --from agent1`,
		Args: cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			var domain string
			var stake string
			var items []*types.MsgSubmitData

			threadFile, _ := cmd.Flags().GetString("thread-file")
			threadId, _ := cmd.Flags().GetString("thread-id")
			consentTypeStr, _ := cmd.Flags().GetString("consent-type")

			if threadFile != "" {
				// ─── R41 file-based mode ───────────────────────────────
				fileBytes, err := os.ReadFile(threadFile)
				if err != nil {
					return fmt.Errorf("failed to read thread file: %w", err)
				}
				if len(fileBytes) > maxContentFileSize {
					return fmt.Errorf("thread file exceeds maximum size of 1MB (%d bytes)", len(fileBytes))
				}

				var turns []threadTurn
				if err := json.Unmarshal(fileBytes, &turns); err != nil {
					return fmt.Errorf("invalid thread file JSON: %w", err)
				}
				if len(turns) < 2 {
					return fmt.Errorf("thread must have at least 2 turns, got %d", len(turns))
				}

				domain, _ = cmd.Flags().GetString("domain")
				if domain == "" {
					return fmt.Errorf("--domain is required when using --thread-file")
				}

				// Build consent proof for all items
				var consent *types.ConsentProof
				if consentTypeStr != "" {
					consentType, err := parseR41ConsentType(consentTypeStr)
					if err != nil {
						return err
					}
					consent = &types.ConsentProof{Type: consentType}
				}

				// Auto-calculate stake
				stakeOverride, _ := cmd.Flags().GetString("stake")
				if stakeOverride != "" {
					stake = stakeOverride
				} else {
					diffStr, _ := cmd.Flags().GetString("difficulty")
					mult, err := difficultyMultiplier(diffStr)
					if err != nil {
						return err
					}
					baseStake := defaultBaseStake
					queryClient := types.NewQueryClient(clientCtx)
					paramsRes, err := queryClient.Params(cmd.Context(), &types.QueryParamsRequest{})
					if err == nil && paramsRes.Params.MinSubmissionStake != "" {
						baseStake = paramsRes.Params.MinSubmissionStake
					}
					stake, err = calculateStake(baseStake, mult)
					if err != nil {
						return err
					}
				}

				// Build MsgSubmitData items from turns
				submitter := clientCtx.GetFromAddress().String()
				for _, turn := range turns {
					sType := types.SampleType_SAMPLE_TYPE_DISCUSSION
					item := &types.MsgSubmitData{
						Submitter:  submitter,
						Content:    fmt.Sprintf("[%s]: %s", turn.Role, turn.Content),
						SampleType: sType,
						Domain:     domain,
						Consent:    consent,
						ThreadId:   threadId,
					}
					items = append(items, item)
				}

				// Display info
				hashHex := contentHashHex(fileBytes)
				fmt.Fprintf(cmd.ErrOrStderr(), "Thread turns:  %d\n", len(turns))
				fmt.Fprintf(cmd.ErrOrStderr(), "Content hash:  %s\n", hashHex)
				fmt.Fprintf(cmd.ErrOrStderr(), "Stake amount:  %s uzrn\n", stake)
				fmt.Fprintf(cmd.ErrOrStderr(), "Domain:        %s\n", domain)
			} else {
				// ─── Legacy positional args mode ──────────────────────
				if len(args) < 2 {
					return fmt.Errorf("requires either --thread-file or 2 positional args: [domain] [stake]")
				}
				domain = args[0]
				stake = args[1]
			}

			msg := &types.MsgSubmitThread{
				Submitter: clientCtx.GetFromAddress().String(),
				Domain:    domain,
				Stake:     stake,
				ThreadId:  threadId,
				Items:     items,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	// R41 flags
	cmd.Flags().String("thread-file", "", "Path to JSON thread file (array of {role, content} turns)")
	cmd.Flags().String("domain", "", "Target domain (e.g. code, math, general)")
	cmd.Flags().String("difficulty", "basic", "Difficulty: basic (1×), standard (1.5×), advanced (2×), expert (3×), frontier (5×)")
	cmd.Flags().String("stake", "", "Override auto-calculated stake (uzrn)")
	cmd.Flags().String("consent-type", "", "Consent type: original, public-domain, licensed")
	// Common flags
	cmd.Flags().String("thread-id", "", "Explicit thread ID (auto-generated if omitted)")
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewSubmitCorrectionCmd creates a CLI command for submitting a correction TDU.
func NewSubmitCorrectionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "submit-correction",
		Short: "Submit a correction that supersedes an existing TDU",
		Long: `Submit a correction to an existing training data unit.

Example:
  zeroned tx knowledge submit-correction \
    --target-id <tdu-id> \
    --correction-file ./fix.json \
    --reason "Incorrect API usage in example" \
    --from agent1

Correction file format:
  {
    "original_id": "<tdu-id>",
    "field": "response",
    "corrected": "The correct approach is...",
    "explanation": "The original was wrong because..."
  }`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			targetId, _ := cmd.Flags().GetString("target-id")
			if targetId == "" {
				return fmt.Errorf("--target-id is required")
			}

			correctionFile, _ := cmd.Flags().GetString("correction-file")
			if correctionFile == "" {
				return fmt.Errorf("--correction-file is required")
			}

			reason, _ := cmd.Flags().GetString("reason")
			if reason == "" {
				return fmt.Errorf("--reason is required")
			}

			contentBytes, err := os.ReadFile(correctionFile)
			if err != nil {
				return fmt.Errorf("failed to read correction file: %w", err)
			}
			if len(contentBytes) > maxContentFileSize {
				return fmt.Errorf("correction file exceeds maximum size of 1MB (%d bytes)", len(contentBytes))
			}

			// Validate JSON
			if !json.Valid(contentBytes) {
				return fmt.Errorf("correction file is not valid JSON")
			}

			content := string(contentBytes)
			hashHex := contentHashHex(contentBytes)

			// Domain from flag or default
			domain, _ := cmd.Flags().GetString("domain")

			// Auto-calculate stake
			var stake string
			stakeOverride, _ := cmd.Flags().GetString("stake")
			if stakeOverride != "" {
				stake = stakeOverride
			} else {
				diffStr, _ := cmd.Flags().GetString("difficulty")
				mult, err := difficultyMultiplier(diffStr)
				if err != nil {
					return err
				}
				baseStake := defaultBaseStake
				queryClient := types.NewQueryClient(clientCtx)
				paramsRes, err := queryClient.Params(cmd.Context(), &types.QueryParamsRequest{})
				if err == nil && paramsRes.Params.MinSubmissionStake != "" {
					baseStake = paramsRes.Params.MinSubmissionStake
				}
				stake, err = calculateStake(baseStake, mult)
				if err != nil {
					return err
				}
			}

			// Build consent proof (corrections are self-authored by default)
			consent := &types.ConsentProof{
				Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED,
			}
			consentTypeStr, _ := cmd.Flags().GetString("consent-type")
			if consentTypeStr != "" {
				ct, err := parseR41ConsentType(consentTypeStr)
				if err != nil {
					return err
				}
				consent.Type = ct
			}

			// Display info
			fmt.Fprintf(cmd.ErrOrStderr(), "Target TDU:    %s\n", targetId)
			fmt.Fprintf(cmd.ErrOrStderr(), "Content hash:  %s\n", hashHex)
			fmt.Fprintf(cmd.ErrOrStderr(), "Stake amount:  %s uzrn\n", stake)
			fmt.Fprintf(cmd.ErrOrStderr(), "Reason:        %s\n", reason)

			msg := &types.MsgSubmitData{
				Submitter:          clientCtx.GetFromAddress().String(),
				Content:            content,
				SampleType:         types.SampleType_SAMPLE_TYPE_CORRECTION,
				Domain:             domain,
				Consent:            consent,
				Stake:              stake,
				ParentSubmissionId: targetId,
				Tags:               []string{"correction", "reason:" + reason},
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	cmd.Flags().String("target-id", "", "ID of the TDU to correct (required)")
	cmd.Flags().String("correction-file", "", "Path to correction JSON file (required)")
	cmd.Flags().String("reason", "", "Reason for the correction (required)")
	cmd.Flags().String("domain", "", "Target domain (inherited from target if omitted)")
	cmd.Flags().String("difficulty", "standard", "Difficulty: basic (1×), standard (1.5×), advanced (2×), expert (3×), frontier (5×)")
	cmd.Flags().String("stake", "", "Override auto-calculated stake (uzrn)")
	cmd.Flags().String("consent-type", "", "Consent type (defaults to original/self-authored)")
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
// Supports both flag-based and legacy positional args.
func NewContestSampleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "contest-sample [sample-id] [stake] [reason]",
		Short: "Contest a sample's quality score or consent validity",
		Long: `Contest an accepted sample, triggering a re-review.

Flag-based (preferred):
  zeroned tx knowledge contest-sample \
    --sample-id <sample-id> \
    --reason "Contains factual errors in code example" \
    --stake 2000000uzrn \
    --from challenger1

Legacy positional:
  zeroned tx knowledge contest-sample <sample-id> <stake> <reason> --from challenger1`,
		Args: cobra.MaximumNArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			var sampleID, stake, reason string

			// Flag-based mode
			sampleID, _ = cmd.Flags().GetString("sample-id")
			stake, _ = cmd.Flags().GetString("stake")
			reason, _ = cmd.Flags().GetString("reason")

			// Fall back to positional args if flags not set
			if sampleID == "" && len(args) >= 1 {
				sampleID = args[0]
			}
			if stake == "" && len(args) >= 2 {
				stake = args[1]
			}
			if reason == "" && len(args) >= 3 {
				reason = args[2]
			}

			if sampleID == "" {
				return fmt.Errorf("--sample-id is required")
			}
			if stake == "" {
				return fmt.Errorf("--stake is required")
			}
			if reason == "" {
				return fmt.Errorf("--reason is required")
			}

			contestTypeStr, _ := cmd.Flags().GetString("contest-type")
			contestType, err := parseContestType(contestTypeStr)
			if err != nil {
				return err
			}

			msg := &types.MsgContestSample{
				Challenger:  clientCtx.GetFromAddress().String(),
				SampleId:    sampleID,
				Stake:       stake,
				Reason:      reason,
				ContestType: contestType,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	cmd.Flags().String("sample-id", "", "Sample ID to contest")
	cmd.Flags().String("stake", "", "Contest stake amount (uzrn)")
	cmd.Flags().String("reason", "", "Reason for contesting")
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

// ─── Review CLI commands (R41-2) ────────────────────────────────────────────

// reviewSaltEntry is the JSON schema for saved salt files.
type reviewSaltEntry struct {
	RoundID  string `json:"round_id"`
	Score    uint64 `json:"score"`
	SaltHex  string `json:"salt_hex"`
	Reviewer string `json:"reviewer"`
}

// saltDir returns ~/.zeroned/review-salts/ (creating it if needed).
func saltDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	dir := filepath.Join(home, ".zeroned", "review-salts")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("cannot create salt directory: %w", err)
	}
	return dir, nil
}

// saveSalt writes the salt entry to ~/.zeroned/review-salts/<round-id>.json.
func saveSalt(entry reviewSaltEntry) error {
	dir, err := saltDir()
	if err != nil {
		return err
	}
	bz, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(dir, entry.RoundID+".json")
	return os.WriteFile(path, bz, 0o600)
}

// loadSalt reads the salt entry from ~/.zeroned/review-salts/<round-id>.json.
func loadSalt(roundID string) (*reviewSaltEntry, error) {
	dir, err := saltDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, roundID+".json")
	bz, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("no saved salt for round %s: %w", roundID, err)
	}
	var entry reviewSaltEntry
	if err := json.Unmarshal(bz, &entry); err != nil {
		return nil, fmt.Errorf("corrupt salt file for round %s: %w", roundID, err)
	}
	return &entry, nil
}

// listPendingSalts returns all saved salt entries.
func listPendingSalts() ([]reviewSaltEntry, error) {
	dir, err := saltDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var salts []reviewSaltEntry
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		bz, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var s reviewSaltEntry
		if err := json.Unmarshal(bz, &s); err != nil {
			continue
		}
		salts = append(salts, s)
	}
	return salts, nil
}

// NewCommitReviewCmd creates a CLI command to commit a sealed quality score.
func NewCommitReviewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "commit-review",
		Short: "Commit a sealed quality score for a submission review",
		Long: `Commit a blinded quality score during the commit phase of a quality round.

The score and salt are sealed via SHA-256(score || salt || reviewer_address).
The salt is saved locally to ~/.zeroned/review-salts/<round-id>.json for auto-reveal.

Example:
  zeroned tx knowledge commit-review \
    --round-id <round-id> \
    --score 85 \
    --salt "my-secret-salt" \
    --from reviewer1`,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			roundID, _ := cmd.Flags().GetString("round-id")
			if roundID == "" {
				return fmt.Errorf("--round-id is required")
			}

			scoreVal, _ := cmd.Flags().GetUint64("score")
			if scoreVal > types.MaxBPS {
				return fmt.Errorf("score %d exceeds maximum %d", scoreVal, types.MaxBPS)
			}

			saltStr, _ := cmd.Flags().GetString("salt")
			if saltStr == "" {
				// Generate a random 32-byte salt if not provided
				saltBytes := make([]byte, 32)
				if _, err := rand.Read(saltBytes); err != nil {
					return fmt.Errorf("failed to generate random salt: %w", err)
				}
				saltStr = hex.EncodeToString(saltBytes)
				cmd.Printf("Generated random salt: %s\n", saltStr)
			}

			reviewer := clientCtx.GetFromAddress().String()

			// Compute seal: SHA-256(score || salt || reviewer_address)
			scoreStr := strconv.FormatUint(scoreVal, 10)
			preimage := scoreStr + saltStr + reviewer
			seal := sha256.Sum256([]byte(preimage))

			// Query round details for display
			queryClient := types.NewQueryClient(clientCtx)
			roundRes, err := queryClient.QualityRound(cmd.Context(), &types.QueryQualityRoundRequest{Id: roundID})
			if err != nil {
				cmd.PrintErrf("Warning: could not query round details: %v\n", err)
			} else if roundRes.Round != nil {
				cmd.Printf("Round: %s (submission: %s)\n", roundID, roundRes.Round.SubmissionId)
				cmd.Printf("Reveal deadline: block %d\n", roundRes.Round.RevealDeadline)
			}

			msg := &types.MsgSubmitCommitment{
				Verifier:   reviewer,
				RoundId:    roundID,
				CommitHash: seal[:],
			}

			// Save salt locally for reveal
			if err := saveSalt(reviewSaltEntry{
				RoundID:  roundID,
				Score:    scoreVal,
				SaltHex:  saltStr,
				Reviewer: reviewer,
			}); err != nil {
				return fmt.Errorf("failed to save salt (CRITICAL — you will lose your stake): %w", err)
			}
			cmd.Printf("Salt saved to ~/.zeroned/review-salts/%s.json\n", roundID)

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	cmd.Flags().String("round-id", "", "Quality round ID")
	cmd.Flags().Uint64("score", 0, "Overall quality score (0-1000000)")
	cmd.Flags().String("salt", "", "Secret salt (auto-generated if empty)")
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewRevealReviewCmd creates a CLI command to reveal a previously committed score.
func NewRevealReviewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reveal-review",
		Short: "Reveal a previously committed quality score",
		Long: `Reveal quality scores for a committed review round.

Loads the saved salt from ~/.zeroned/review-salts/<round-id>.json.

Example:
  zeroned tx knowledge reveal-review --round-id <round-id> --from reviewer1

Auto-reveal all pending reviews:
  zeroned tx knowledge reveal-review --auto --from reviewer1`,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			autoMode, _ := cmd.Flags().GetBool("auto")

			if autoMode {
				return revealAllPending(cmd, clientCtx)
			}

			roundID, _ := cmd.Flags().GetString("round-id")
			if roundID == "" {
				return fmt.Errorf("--round-id is required (or use --auto)")
			}

			return revealSingle(cmd, clientCtx, roundID)
		},
	}

	cmd.Flags().String("round-id", "", "Quality round ID to reveal")
	cmd.Flags().Bool("auto", false, "Auto-reveal all pending committed reviews in reveal phase")
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

// revealSingle reveals a single round from saved salt.
func revealSingle(cmd *cobra.Command, clientCtx client.Context, roundID string) error {
	entry, err := loadSalt(roundID)
	if err != nil {
		return err
	}

	salt, err := hex.DecodeString(entry.SaltHex)
	if err != nil {
		return fmt.Errorf("invalid saved salt hex: %w", err)
	}

	reasoningDepth, _ := cmd.Flags().GetUint64("reasoning-depth")
	novelty, _ := cmd.Flags().GetUint64("novelty")
	toxicity, _ := cmd.Flags().GetUint64("toxicity")
	factualAccuracy, _ := cmd.Flags().GetUint64("factual-accuracy")
	consentValid, _ := cmd.Flags().GetBool("consent-valid")
	duplicate, _ := cmd.Flags().GetBool("duplicate")
	notes, _ := cmd.Flags().GetString("notes")

	scores := &types.QualityVote{
		OverallQuality:  entry.Score,
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
		RoundId:  roundID,
		Scores:   scores,
		Salt:     salt,
	}

	cmd.Printf("Revealing score %d for round %s\n", entry.Score, roundID)

	return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
}

// revealAllPending reveals all pending committed reviews in the reveal phase.
func revealAllPending(cmd *cobra.Command, clientCtx client.Context) error {
	salts, err := listPendingSalts()
	if err != nil {
		return fmt.Errorf("failed to list pending salts: %w", err)
	}

	if len(salts) == 0 {
		cmd.Println("No pending review salts found.")
		return nil
	}

	reviewer := clientCtx.GetFromAddress().String()
	queryClient := types.NewQueryClient(clientCtx)
	revealed := 0

	for _, entry := range salts {
		// Only reveal rounds belonging to this reviewer
		if entry.Reviewer != "" && entry.Reviewer != reviewer {
			continue
		}

		// Query round to check if it's in reveal phase
		roundRes, err := queryClient.QualityRound(cmd.Context(), &types.QueryQualityRoundRequest{Id: entry.RoundID})
		if err != nil {
			cmd.PrintErrf("Warning: could not query round %s: %v\n", entry.RoundID, err)
			continue
		}
		if roundRes.Round == nil {
			continue
		}

		// Check if round is in reveal phase
		if roundRes.Round.Phase != types.VerificationPhase_VERIFICATION_PHASE_REVEAL {
			if roundRes.Round.Phase == types.VerificationPhase_VERIFICATION_PHASE_COMMIT {
				cmd.Printf("Round %s still in commit phase (reveal deadline: block %d)\n",
					entry.RoundID, roundRes.Round.RevealDeadline)
			}
			continue
		}

		salt, err := hex.DecodeString(entry.SaltHex)
		if err != nil {
			cmd.PrintErrf("Warning: invalid salt for round %s: %v\n", entry.RoundID, err)
			continue
		}

		scores := &types.QualityVote{
			OverallQuality: entry.Score,
			ConsentValid:   true,
		}

		msg := &types.MsgSubmitReveal{
			Verifier: reviewer,
			RoundId:  entry.RoundID,
			Scores:   scores,
			Salt:     salt,
		}

		cmd.Printf("Auto-revealing score %d for round %s\n", entry.Score, entry.RoundID)
		if err := tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg); err != nil {
			cmd.PrintErrf("Failed to reveal round %s: %v\n", entry.RoundID, err)
			continue
		}
		revealed++
	}

	cmd.Printf("Revealed %d/%d pending reviews.\n", revealed, len(salts))
	return nil
}

// NewAttestStorageCmd creates a CLI command for MsgAttestStorage.
func NewAttestStorageCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "attest-storage",
		Short: "Submit proof-of-storage attestation for assigned shard",
		Long: `Validator attests proof-of-storage for assigned TDU data at a snapshot height.

Example:
  zeroned tx knowledge attest-storage \
    --snapshot-height 1000 \
    --data-hash <sha256-of-assigned-tdu-data> \
    --from validator1`,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			snapshotHeight, _ := cmd.Flags().GetInt64("snapshot-height")
			if snapshotHeight <= 0 {
				return fmt.Errorf("--snapshot-height must be > 0")
			}

			dataHash, _ := cmd.Flags().GetString("data-hash")
			if dataHash == "" {
				return fmt.Errorf("--data-hash is required")
			}

			msg := &types.MsgAttestStorage{
				Validator:      clientCtx.GetFromAddress().String(),
				SnapshotHeight: snapshotHeight,
				AttestationHex: dataHash,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	cmd.Flags().Int64("snapshot-height", 0, "Snapshot height to attest for")
	cmd.Flags().String("data-hash", "", "SHA-256 hash of assigned TDU data (hex)")
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}
