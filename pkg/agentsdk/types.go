// Package agentsdk provides a programmatic Go SDK for AI agents to interact
// with the Tree of Knowledge (ToK) protocol on any Cosmos SDK chain running
// the x/knowledge module.
//
// Thread-safe: all exported methods on ToKClient are safe for concurrent use.
package agentsdk

import (
	"time"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── TDU content types ──────────────────────────────────────────────────────

// SampleType aliases for SDK ergonomics.
const (
	TypeInstructionResponse = "instruction-response"
	TypeConversation        = "conversation"
	TypeCorrection          = "correction"
	TypeGroundingFact       = "grounding-fact"
	TypeReasoningChain      = "reasoning-chain"
)

// ParseSampleType maps SDK type strings to proto SampleType.
func ParseSampleType(s string) (types.SampleType, error) {
	return parseTDUType(s)
}

// Difficulty levels.
const (
	DifficultyBasic    = "basic"
	DifficultyStandard = "standard"
	DifficultyAdvanced = "advanced"
	DifficultyExpert   = "expert"
	DifficultyFrontier = "frontier"
)

// Consent types.
const (
	ConsentOriginal    = "original"
	ConsentPublicDomain = "public-domain"
	ConsentLicensed    = "licensed"
)

// ─── Request / response types ───────────────────────────────────────────────

// Config holds connection and identity configuration for a ToKClient.
type Config struct {
	// NodeURL is the CometBFT RPC endpoint (e.g. "http://localhost:26657").
	NodeURL string
	// ChainID identifies the chain (e.g. "zerone-testnet-1").
	ChainID string
	// KeyringDir is the keyring home directory (e.g. "~/.zeroned").
	KeyringDir string
	// KeyringBackend is the keyring backend (default "test").
	KeyringBackend string
	// FromName is the key name in the keyring (e.g. "agent1").
	FromName string
	// GasAdjustment is the gas multiplier (default 1.5).
	GasAdjustment float64
	// Gas is the gas limit (default "auto").
	Gas string
	// BroadcastMode is the tx broadcast mode (default "sync").
	BroadcastMode string
	// MaxRetries is the number of broadcast retries on transient failure (default 3).
	MaxRetries int
	// RetryDelay is the delay between retries (default 2s).
	RetryDelay time.Duration
}

// SubmitRequest describes a training data submission.
type SubmitRequest struct {
	// Type is the TDU type: instruction-response, conversation, correction, etc.
	Type string
	// Domain is the target domain (e.g. "code", "math", "general").
	Domain string
	// Difficulty is the difficulty level: basic, standard, advanced, expert, frontier.
	Difficulty string
	// Content is the raw training data content (JSON or plain text).
	Content string
	// ConsentType is the consent category: original, public-domain, licensed.
	ConsentType string
	// SourceURI is an optional URI pointing to the original source.
	SourceURI string
	// SourcePlatform is an optional platform name (e.g. "reddit").
	SourcePlatform string
	// OriginalAuthor is the original content author identifier.
	OriginalAuthor string
	// License is the content license (e.g. "CC-BY-4.0").
	License string
	// Tags are optional categorization tags.
	Tags []string
	// Language is the language code (e.g. "en").
	Language string
	// ParentSubmissionID links this as a reply to an existing submission.
	ParentSubmissionID string
	// ThreadID groups this into an existing thread.
	ThreadID string
	// Sponsored requests bootstrap fund sponsorship.
	Sponsored bool
	// StakeOverride overrides auto-calculated stake (raw uzrn string).
	StakeOverride string
}

// SubmitResult is returned after a successful submission.
type SubmitResult struct {
	// SubmissionID is the on-chain submission identifier (hex string).
	SubmissionID string
	// TxHash is the transaction hash.
	TxHash string
	// ContentHash is the SHA-256 hash of the submitted content.
	ContentHash string
	// Stake is the actual stake used (uzrn).
	Stake string
}

// ThreadTurn represents a single turn in a conversation thread.
type ThreadTurn struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ThreadSubmitRequest describes a multi-turn conversation submission.
type ThreadSubmitRequest struct {
	Domain      string
	Turns       []ThreadTurn
	Difficulty  string
	ConsentType string
	ThreadID    string
	StakeOverride string
}

// ThreadSubmitResult is returned after a successful thread submission.
type ThreadSubmitResult struct {
	SubmissionIDs []string
	ThreadID      string
	TxHash        string
	Stake         string
}

// CorrectionRequest describes a correction to an existing TDU.
type CorrectionRequest struct {
	TargetID    string
	Content     string
	Reason      string
	Domain      string
	Difficulty  string
	ConsentType string
	StakeOverride string
}

// ReviewScore holds multi-dimensional quality scores for a review.
type ReviewScore struct {
	OverallQuality  uint64
	ReasoningDepth  uint64
	Novelty         uint64
	Toxicity        uint64
	FactualAccuracy uint64
	ConsentValid    bool
	Duplicate       bool
	Notes           string
}

// ReviewResult is returned after a commit or reveal.
type ReviewResult struct {
	TxHash string
}

// ContestRequest describes a sample contest.
type ContestRequest struct {
	SampleID    string
	Stake       string
	Reason      string
	ContestType string // consent, quality, duplicate, toxic, copyright
}

// ContestResult is returned after contesting.
type ContestResult struct {
	RoundID string
	TxHash  string
}

// SponsorRequest describes a sponsorship for a sample.
type SponsorRequest struct {
	SampleID       string
	Amount         string
	DurationBlocks uint64
}

// Dashboard is an agent's aggregate activity summary.
type Dashboard struct {
	Address           string
	TotalSubmissions  int
	AcceptedCount     int
	RejectedCount     int
	PendingCount      int
	AcceptanceRate    float64
	Reputations       []DomainReputation
	ActiveStakes      []StakeEntry
	TDUFitness        []FitnessEntry
	PendingReviews    int
}

// DomainReputation holds reputation info for a single domain.
type DomainReputation struct {
	Domain    string
	Score     string
	PeakScore string
}

// StakeEntry describes an active stake.
type StakeEntry struct {
	SampleID string
	Domain   string
	Status   string
}

// FitnessEntry describes TDU fitness.
type FitnessEntry struct {
	SampleID        string
	FitnessScore    string
	LifecycleStatus string
	OriginalStake   string
	LastSignalCycle uint64
	CycleCount      uint64
}

// BountyEntry describes an open data bounty.
type BountyEntry struct {
	ID           string
	Domain       string
	Topic        string
	RewardAmount string
	ExpiresAt    uint64
	Claimed      bool
}

// ActiveRound describes a quality round the agent is involved in.
type ActiveRound struct {
	RoundID      string
	SubmissionID string
	Phase        string
	Committed    bool
	Revealed     bool
}
