package types

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	sdkmath "cosmossdk.io/math"
)

// ─── Agent Status ───────────────────────────────────────────────────────────

// AgentStatus represents the lifecycle state of a promoted agent.
type AgentStatus string

const (
	AgentStatusActive    AgentStatus = "active"
	AgentStatusSuspended AgentStatus = "suspended" // poor performance
	AgentStatusRetired   AgentStatus = "retired"   // source model deprecated
)

// ValidAgentStatuses is the set of valid agent statuses.
var ValidAgentStatuses = map[AgentStatus]bool{
	AgentStatusActive:    true,
	AgentStatusSuspended: true,
	AgentStatusRetired:   true,
}

// ─── Agent Identity ─────────────────────────────────────────────────────────

// AgentIdentity represents an autonomous agent derived from a trained model.
// The recursive loop: data → training → model → agent → better data → repeat.
// Stored as JSON under AgentIdentityPrefix + agentID.
type AgentIdentity struct {
	// Identity
	AgentID    string `json:"agent_id"`    // deterministic from model hash
	ModelID    string `json:"model_id"`    // source model that was promoted
	Address    string `json:"address"`     // on-chain wallet (derived from model hash)
	Domain     string `json:"domain"`      // inherited from model's training domain
	Generation uint64 `json:"generation"`  // recursive depth (0 = human-trained, 1+ = agent-trained)

	// Capabilities
	CanSubmit bool `json:"can_submit"` // can submit TDUs
	CanReview bool `json:"can_review"` // can review TDUs (benchmark ≥ 0.6)
	CanTrain  bool `json:"can_train"`  // can initiate training (requires validator backing)

	// Performance
	Reputation    string `json:"reputation"`     // sdkmath.LegacyDec [0, 1]
	TasksComplete uint64 `json:"tasks_complete"` // total on-chain actions
	EarningsTotal string `json:"earnings_total"` // sdkmath.Int uzrn

	// Lifecycle
	Status      AgentStatus `json:"status"`
	PromotedAt  int64       `json:"promoted_at"`  // block height
	SuspendedAt int64       `json:"suspended_at"` // block height (0 if not suspended)
	SponsorAddr string      `json:"sponsor_addr"` // who paid for promotion
	InitialStake string     `json:"initial_stake"` // sdkmath.Int uzrn

	// Lineage
	ParentAgentID string   `json:"parent_agent_id"` // if model was trained by agent-curated data
	Lineage       []string `json:"lineage"`         // full ancestry chain
}

// GetReputation parses the reputation score.
func (a *AgentIdentity) GetReputation() sdkmath.LegacyDec {
	if a.Reputation == "" {
		return sdkmath.LegacyZeroDec()
	}
	d, err := sdkmath.LegacyNewDecFromStr(a.Reputation)
	if err != nil {
		return sdkmath.LegacyZeroDec()
	}
	return d
}

// SetReputation stores the reputation, clamped to [0, 1].
func (a *AgentIdentity) SetReputation(rep sdkmath.LegacyDec) {
	if rep.GT(sdkmath.LegacyOneDec()) {
		rep = sdkmath.LegacyOneDec()
	}
	if rep.LT(sdkmath.LegacyZeroDec()) {
		rep = sdkmath.LegacyZeroDec()
	}
	a.Reputation = rep.String()
}

// GetEarningsTotal parses the total earnings.
func (a *AgentIdentity) GetEarningsTotal() sdkmath.Int {
	if a.EarningsTotal == "" {
		return sdkmath.ZeroInt()
	}
	i, ok := sdkmath.NewIntFromString(a.EarningsTotal)
	if !ok {
		return sdkmath.ZeroInt()
	}
	return i
}

// GetInitialStake parses the initial stake amount.
func (a *AgentIdentity) GetInitialStake() sdkmath.Int {
	if a.InitialStake == "" {
		return sdkmath.ZeroInt()
	}
	i, ok := sdkmath.NewIntFromString(a.InitialStake)
	if !ok {
		return sdkmath.ZeroInt()
	}
	return i
}

// ─── Promotion Criteria ─────────────────────────────────────────────────────

var (
	// AgentMinBenchmarkScore — higher bar than publishing (0.6 vs 0.3).
	AgentMinBenchmarkScore = sdkmath.LegacyNewDecWithPrec(6, 1) // 0.6
	// AgentMinTDUCount — trained on substantial data.
	AgentMinTDUCount uint64 = 50
	// AgentMinStake — skin in the game (10 ZRN = 10_000_000 uzrn).
	AgentMinStake = sdkmath.NewInt(10_000_000)
	// AgentMaxGeneration — prevent infinite recursion.
	AgentMaxGeneration uint64 = 10
	// AgentSuspensionThreshold — auto-suspend below this reputation.
	AgentSuspensionThreshold = sdkmath.LegacyNewDecWithPrec(2, 1) // 0.2
	// AgentReinstateBlocks — blocks before suspended agent can be reinstated.
	AgentReinstateBlocks int64 = 1000
)

// ─── Deterministic Address Derivation ───────────────────────────────────────

// DeriveAgentAddress generates a deterministic address from a model hash.
// Format: first 20 bytes of SHA-256("zerone-agent:" + modelHash).
func DeriveAgentAddress(modelHash string) string {
	input := "zerone-agent:" + modelHash
	hash := sha256.Sum256([]byte(input))
	return hex.EncodeToString(hash[:20])
}

// DeriveAgentID generates a deterministic agent ID from a model ID.
func DeriveAgentID(modelID string) string {
	input := "agent:" + modelID
	hash := sha256.Sum256([]byte(input))
	return hex.EncodeToString(hash[:])
}

// ─── Messages ───────────────────────────────────────────────────────────────

// MsgPromoteModel requests promotion of a trained model to an autonomous agent.
type MsgPromoteModel struct {
	Sponsor string `json:"sponsor"` // who pays the stake
	ModelID string `json:"model_id"`
	Stake   string `json:"stake"` // sdkmath.Int uzrn
}

// ValidateBasic performs stateless validation.
func (msg *MsgPromoteModel) ValidateBasic() error {
	if msg.Sponsor == "" {
		return ErrUnauthorized.Wrap("sponsor is required")
	}
	if msg.ModelID == "" {
		return fmt.Errorf("model ID is required")
	}
	stake, ok := sdkmath.NewIntFromString(msg.Stake)
	if !ok || !stake.IsPositive() {
		return fmt.Errorf("invalid stake amount")
	}
	return nil
}

// MsgPromoteModelResponse is returned after promotion.
type MsgPromoteModelResponse struct {
	AgentID string `json:"agent_id"`
	Address string `json:"address"`
}

// ─── Events ─────────────────────────────────────────────────────────────────

const (
	EventAgentPromoted  = "agent_promoted"
	EventAgentSuspended = "agent_suspended"
	EventAgentRetired   = "agent_retired"
	EventAgentReinstated = "agent_reinstated"

	AttributeAgentID         = "agent_id"
	AttributeAgentGeneration = "agent_generation"
	AttributeAgentSponsor    = "agent_sponsor"
	AttributeAgentAddress    = "agent_address"
)
