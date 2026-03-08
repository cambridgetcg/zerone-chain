package types

import (
	"encoding/json"
	"fmt"

	sdkmath "cosmossdk.io/math"
)

// ─── R51: Agent-as-Consumer Types ───────────────────────────────────────────
//
// Agents access models through the API payment layer. They don't contain
// models — they rent them. This closes the recursive self-improvement loop:
//
//   earn ZRN → pay for API → use best model → do better work → earn more ZRN
//              ↑                                                     │
//              └─────────────────────────────────────────────────────┘
//
// The agent is an economic actor. The model is a service.
// Natural selection: agents that can't afford to think, die.

// ─── Agent API Configuration ────────────────────────────────────────────────

// AgentAPIConfig stores an agent's API access configuration.
// Created automatically during agent promotion (ProvisionAgentAPI).
type AgentAPIConfig struct {
	AgentID        string `json:"agent_id"`
	APIKeyHash     string `json:"api_key_hash"`       // bound to agent's derived wallet
	Wallet         string `json:"wallet"`              // agent's on-chain address

	// Model selection preferences.
	AutoSelect       bool   `json:"auto_select"`        // true = pick best model per task
	PreferredModelID string `json:"preferred_model_id"` // manual override (empty = auto)
	MaxTokenBudget   uint64 `json:"max_token_budget"`   // per-task token limit (0 = unlimited)

	// Budget management.
	ReplenishRate string `json:"replenish_rate"` // fraction of earnings auto-deposited as credits
	MinBalance    string `json:"min_balance"`    // alert/suspend threshold (uzrn)

	// Cumulative statistics.
	TotalCalls      uint64 `json:"total_calls"`
	TotalTokensUsed uint64 `json:"total_tokens_used"`
	TotalSpent      string `json:"total_spent"` // uzrn lifetime API spend

	// Current state.
	LastModelUsed string `json:"last_model_used"`
	LastCallBlock uint64 `json:"last_call_block"`
	CreatedAt     uint64 `json:"created_at"`
}

// GetReplenishRate parses the auto-replenish fraction.
func (c *AgentAPIConfig) GetReplenishRate() sdkmath.LegacyDec {
	if c.ReplenishRate == "" {
		return DefaultReplenishRate
	}
	d, err := sdkmath.LegacyNewDecFromStr(c.ReplenishRate)
	if err != nil {
		return DefaultReplenishRate
	}
	return d
}

// GetTotalSpent parses total API spend.
func (c *AgentAPIConfig) GetTotalSpent() sdkmath.Int {
	if c.TotalSpent == "" {
		return sdkmath.ZeroInt()
	}
	i, ok := sdkmath.NewIntFromString(c.TotalSpent)
	if !ok {
		return sdkmath.ZeroInt()
	}
	return i
}

// GetMinBalance parses the minimum balance threshold.
func (c *AgentAPIConfig) GetMinBalance() sdkmath.Int {
	if c.MinBalance == "" {
		return DefaultAgentMinAPIBalance
	}
	i, ok := sdkmath.NewIntFromString(c.MinBalance)
	if !ok {
		return DefaultAgentMinAPIBalance
	}
	return i
}

func (c AgentAPIConfig) Marshal() ([]byte, error)    { return json.Marshal(c) }
func (c *AgentAPIConfig) Unmarshal(bz []byte) error   { return json.Unmarshal(bz, c) }

// ─── Agent Profitability ────────────────────────────────────────────────────

// AgentProfitability tracks whether an agent is economically viable.
// An agent that spends more on API access than it earns from work is
// living on borrowed time. Natural selection handles the rest.
type AgentProfitability struct {
	AgentID string `json:"agent_id"`

	// Revenue streams.
	CurationRewards    string `json:"curation_rewards"`    // from TDU submissions accepted
	ReviewRewards      string `json:"review_rewards"`      // from quality round participation
	BountyRewards      string `json:"bounty_rewards"`      // from competitive bounties
	AttributionRewards string `json:"attribution_rewards"` // from training impact (R50 Loop 3)
	TotalEarned        string `json:"total_earned"`

	// Costs.
	APISpend   string `json:"api_spend"`   // model access via API
	StakeSpend string `json:"stake_spend"` // consensus/review staking costs
	TotalSpent string `json:"total_spent"`

	// Bottom line.
	NetProfitLoss string `json:"net_profit_loss"` // earned - spent
	ProfitRatio   string `json:"profit_ratio"`    // earned / spent (>1 = profitable)

	// Trend over last N epochs.
	EpochHistory []EpochPnL `json:"epoch_history"`
	Trend        string     `json:"trend"` // "improving" | "stable" | "declining"

	// Survival metrics.
	EstimatedRunway uint64 `json:"estimated_runway"` // blocks until ZRN runs out at current burn rate
	SolventSince    uint64 `json:"solvent_since"`    // block height (reset if ever went to 0)

	ComputedAt uint64 `json:"computed_at"` // last calculation block
}

// IsProfitable returns true if the agent earns more than it spends.
func (p *AgentProfitability) IsProfitable() bool {
	ratio, err := sdkmath.LegacyNewDecFromStr(p.ProfitRatio)
	if err != nil {
		return false
	}
	return ratio.GTE(sdkmath.LegacyOneDec())
}

// GetNetProfitLoss parses the net P&L.
func (p *AgentProfitability) GetNetProfitLoss() sdkmath.Int {
	if p.NetProfitLoss == "" {
		return sdkmath.ZeroInt()
	}
	i, ok := sdkmath.NewIntFromString(p.NetProfitLoss)
	if !ok {
		return sdkmath.ZeroInt()
	}
	return i
}

func (p AgentProfitability) Marshal() ([]byte, error)  { return json.Marshal(p) }
func (p *AgentProfitability) Unmarshal(bz []byte) error { return json.Unmarshal(bz, p) }

// EpochPnL is one epoch's profit & loss summary for an agent.
type EpochPnL struct {
	Epoch     uint64 `json:"epoch"`
	Earned    string `json:"earned"`
	Spent     string `json:"spent"`
	Net       string `json:"net"`
	ModelUsed string `json:"model_used"` // which model the agent primarily used
}

// ─── Model Selection ────────────────────────────────────────────────────────

// ModelSelection is the result of an agent choosing which model to use for a task.
type ModelSelection struct {
	ModelID        string `json:"model_id"`
	Domain         string `json:"domain"`
	BenchmarkScore string `json:"benchmark_score"`
	EstimatedCost  string `json:"estimated_cost"` // uzrn for this task
	Reason         string `json:"reason"`          // why this model was chosen
}

func (s ModelSelection) Marshal() ([]byte, error)  { return json.Marshal(s) }
func (s *ModelSelection) Unmarshal(bz []byte) error { return json.Unmarshal(bz, s) }

// ─── Agent API Call Record ──────────────────────────────────────────────────

// AgentAPICall records a single API invocation by an agent during task execution.
// Stored for audit trail and profitability computation.
type AgentAPICall struct {
	CallID       string `json:"call_id"`
	AgentID      string `json:"agent_id"`
	ModelID      string `json:"model_id"`
	TaskID       string `json:"task_id"`       // which task this call served
	InputTokens  uint64 `json:"input_tokens"`
	OutputTokens uint64 `json:"output_tokens"`
	Cost         string `json:"cost"`          // uzrn deducted
	BlockHeight  uint64 `json:"block_height"`
}

func (c AgentAPICall) Marshal() ([]byte, error)  { return json.Marshal(c) }
func (c *AgentAPICall) Unmarshal(bz []byte) error { return json.Unmarshal(bz, c) }

// ─── Consumer Parameters ────────────────────────────────────────────────────

// AgentConsumerParams governs the agent-as-consumer system.
type AgentConsumerParams struct {
	// What fraction of promotion stake becomes initial API credits.
	InitialCreditFraction string `json:"initial_credit_fraction"` // default: "0.30" = 30%

	// Default auto-replenish rate for new agents.
	DefaultReplenishRate string `json:"default_replenish_rate"` // default: "0.30" = 30% of rewards

	// Grace period before suspending zero-balance agents.
	SuspensionGraceBlocks uint64 `json:"suspension_grace_blocks"` // default: 1000

	// Minimum API balance before agent enters "low fuel" state.
	MinOperatingBalance string `json:"min_operating_balance"` // default: "1000000" = 1 ZRN

	// Agent tier rate limit (agents get higher throughput than free tier).
	AgentRateLimitTier string `json:"agent_rate_limit_tier"` // default: "agent"

	// Self-reinforcement prevention: can an agent use a model trained on its own data?
	AllowSelfReinforcement bool `json:"allow_self_reinforcement"` // default: false
}

// DefaultAgentConsumerParams returns sensible defaults.
func DefaultAgentConsumerParams() AgentConsumerParams {
	return AgentConsumerParams{
		InitialCreditFraction: "0.300000000000000000", // 30% of stake → API credits
		DefaultReplenishRate:  "0.300000000000000000", // 30% of rewards auto-deposited
		SuspensionGraceBlocks: 1_000,                  // ~1.5 hours at 5s blocks
		MinOperatingBalance:   "1000000",              // 1 ZRN
		AgentRateLimitTier:    "agent",
		AllowSelfReinforcement: false,
	}
}

// Validate checks parameter sanity.
func (p AgentConsumerParams) Validate() error {
	frac, err := sdkmath.LegacyNewDecFromStr(p.InitialCreditFraction)
	if err != nil || frac.IsNegative() || frac.GT(sdkmath.LegacyOneDec()) {
		return fmt.Errorf("initial_credit_fraction must be [0, 1], got %s", p.InitialCreditFraction)
	}
	rate, err := sdkmath.LegacyNewDecFromStr(p.DefaultReplenishRate)
	if err != nil || rate.IsNegative() || rate.GT(sdkmath.LegacyOneDec()) {
		return fmt.Errorf("default_replenish_rate must be [0, 1], got %s", p.DefaultReplenishRate)
	}
	return nil
}

func (p AgentConsumerParams) Marshal() ([]byte, error)  { return json.Marshal(p) }
func (p *AgentConsumerParams) Unmarshal(bz []byte) error { return json.Unmarshal(bz, p) }

// ─── Constants ──────────────────────────────────────────────────────────────

var (
	// DefaultReplenishRate — 30% of task rewards auto-deposited as API credits.
	DefaultReplenishRate = sdkmath.LegacyNewDecWithPrec(3, 1) // 0.3

	// DefaultAgentMinAPIBalance — 1 ZRN = 1_000_000 uzrn.
	DefaultAgentMinAPIBalance = sdkmath.NewInt(1_000_000)
)

// ─── Store Keys ─────────────────────────────────────────────────────────────

var (
	AgentAPIConfigPrefix      = []byte("agentapi/config/")
	AgentProfitabilityPrefix  = []byte("agentapi/profitability/")
	AgentAPICallPrefix        = []byte("agentapi/call/")
	AgentAPICallByTaskPrefix  = []byte("agentapi/call-by-task/")
	AgentConsumerParamsKey    = []byte("agentapi/params")
	AgentLowBalanceIndexKey   = []byte("agentapi/low-balance/")
	AgentAPICallSeqKey        = []byte("agentapi/call-seq")
)

// AgentAPIConfigKey returns the store key for an agent's API config.
func AgentAPIConfigKey(agentID string) []byte {
	return append(AgentAPIConfigPrefix, []byte(agentID)...)
}

// AgentProfitabilityKey returns the store key for an agent's P&L.
func AgentProfitabilityKey(agentID string) []byte {
	return append(AgentProfitabilityPrefix, []byte(agentID)...)
}

// AgentAPICallKey returns the store key for a specific API call record.
func AgentAPICallKey(callID string) []byte {
	return append(AgentAPICallPrefix, []byte(callID)...)
}

// AgentAPICallByTaskKey indexes calls by task ID for audit.
func AgentAPICallByTaskKey(taskID, callID string) []byte {
	return append(AgentAPICallByTaskPrefix, []byte(taskID+"/"+callID)...)
}

// AgentLowBalanceKey indexes agents in low-balance state.
func AgentLowBalanceKey(agentID string) []byte {
	return append(AgentLowBalanceIndexKey, []byte(agentID)...)
}

// ─── Events ─────────────────────────────────────────────────────────────────

const (
	EventAgentAPIProvisioned = "agent_api_provisioned"
	EventAgentAPICall        = "agent_api_call"
	EventAgentReplenished    = "agent_api_replenished"
	EventAgentLowBalance     = "agent_low_balance"
	EventAgentBankrupt       = "agent_bankrupt"

	AttributeAgentAPIKey   = "agent_api_key"
	AttributeModelSelected = "model_selected"
	AttributeAPICost       = "api_cost"
	AttributeEstRunway     = "estimated_runway"
)
