package types

import (
	"encoding/json"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// ─── API Key Record ─────────────────────────────────────────────────────────

// APIKeyRecord maps an API key hash to a wallet address on-chain.
type APIKeyRecord struct {
	KeyHash        string `json:"key_hash"`
	Wallet         string `json:"wallet"`
	CreatedAtBlock int64  `json:"created_at_block"`
	Revoked        bool   `json:"revoked"`
	RateLimitTier  string `json:"rate_limit_tier"`
}

func (r *APIKeyRecord) MarshalJSON() ([]byte, error) {
	type alias APIKeyRecord
	return json.Marshal((*alias)(r))
}

func (r *APIKeyRecord) UnmarshalJSON(bz []byte) error {
	type alias APIKeyRecord
	return json.Unmarshal(bz, (*alias)(r))
}

// ─── API Balance ────────────────────────────────────────────────────────────

// APIBalance tracks a wallet's prepaid API credit balance.
type APIBalance struct {
	Wallet           string `json:"wallet"`
	Balance          string `json:"balance"`
	TotalDeposited   string `json:"total_deposited"`
	TotalConsumed    string `json:"total_consumed"`
	LastDepositBlock int64  `json:"last_deposit_block"`
	LastUsageBlock   int64  `json:"last_usage_block"`
}

func (b *APIBalance) MarshalJSON() ([]byte, error) {
	type alias APIBalance
	return json.Marshal((*alias)(b))
}

func (b *APIBalance) UnmarshalJSON(bz []byte) error {
	type alias APIBalance
	return json.Unmarshal(bz, (*alias)(b))
}

// ─── API Usage Record ──────────────────────────────────────────────────────

// APIUsageRecord aggregates per-epoch usage for a wallet.
type APIUsageRecord struct {
	Wallet       string `json:"wallet"`
	Epoch        uint64 `json:"epoch"`
	InputTokens  uint64 `json:"input_tokens"`
	OutputTokens uint64 `json:"output_tokens"`
	RequestCount uint64 `json:"request_count"`
	TotalCost    string `json:"total_cost"`
	ModelUsed    string `json:"model_used"`
}

func (u *APIUsageRecord) MarshalJSON() ([]byte, error) {
	type alias APIUsageRecord
	return json.Marshal((*alias)(u))
}

func (u *APIUsageRecord) UnmarshalJSON(bz []byte) error {
	type alias APIUsageRecord
	return json.Unmarshal(bz, (*alias)(u))
}

// ─── API Revenue Params ─────────────────────────────────────────────────────

// APIRevenueParams holds the 5-way revenue split and token pricing for API revenue.
type APIRevenueParams struct {
	TrainingShareBPS  uint64 `json:"training_share_bps"`  // 4000 = 40%
	InfraShareBPS     uint64 `json:"infra_share_bps"`     // 2500 = 25%
	SubmitterShareBPS uint64 `json:"submitter_share_bps"` // 2000 = 20%
	ProtocolShareBPS  uint64 `json:"protocol_share_bps"`  // 1000 = 10%
	ResearchShareBPS  uint64 `json:"research_share_bps"`  // 500 = 5%
	// Token pricing (per 1000 tokens, uzrn)
	PricePerInputToken  string `json:"price_per_input_token,omitempty"`  // default: "1"
	PricePerOutputToken string `json:"price_per_output_token,omitempty"` // default: "3"
}

func DefaultAPIRevenueParams() APIRevenueParams {
	return APIRevenueParams{
		TrainingShareBPS:    4_000, // 40%
		InfraShareBPS:       2_500, // 25%
		SubmitterShareBPS:   2_000, // 20%
		ProtocolShareBPS:    1_000, // 10%
		ResearchShareBPS:    500,   // 5%
		PricePerInputToken:  "1",   // 1 uzrn per 1000 input tokens
		PricePerOutputToken: "3",   // 3 uzrn per 1000 output tokens
	}
}

func (p *APIRevenueParams) Validate() error {
	total := p.TrainingShareBPS + p.InfraShareBPS + p.SubmitterShareBPS +
		p.ProtocolShareBPS + p.ResearchShareBPS
	if total != 10_000 {
		return fmt.Errorf("API revenue shares must sum to 10000 (100%%), got %d", total)
	}
	return nil
}

func (p *APIRevenueParams) MarshalJSON() ([]byte, error) {
	type alias APIRevenueParams
	return json.Marshal((*alias)(p))
}

func (p *APIRevenueParams) UnmarshalJSON(bz []byte) error {
	type alias APIRevenueParams
	return json.Unmarshal(bz, (*alias)(p))
}

// ─── API Usage Batch (tx message sub-type) ──────────────────────────────────

// APIUsageBatch represents a single batch entry in MsgRecordAPIUsage.
type APIUsageBatch struct {
	APIKeyHash   string `json:"api_key_hash"`
	InputTokens  uint64 `json:"input_tokens"`
	OutputTokens uint64 `json:"output_tokens"`
	RequestCount uint64 `json:"request_count"`
	ModelUsed    string `json:"model_used"`
}

// ─── Message Types ──────────────────────────────────────────────────────────

// MsgCreateAPIKey registers an API key hash on-chain, binding it to a wallet.
type MsgCreateAPIKey struct {
	Owner         string `json:"owner"`
	KeyHash       string `json:"key_hash"`
	RateLimitTier string `json:"rate_limit_tier"`
}

func (m *MsgCreateAPIKey) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Owner); err != nil {
		return fmt.Errorf("invalid owner address: %w", err)
	}
	if m.KeyHash == "" {
		return fmt.Errorf("key_hash is required")
	}
	if len(m.KeyHash) != 64 {
		return fmt.Errorf("key_hash must be 64 hex characters (SHA-256)")
	}
	return nil
}

func (m *MsgCreateAPIKey) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Owner)
	return []sdk.AccAddress{addr}
}

func (*MsgCreateAPIKey) ProtoMessage()             {}
func (*MsgCreateAPIKey) Reset()                    {}
func (*MsgCreateAPIKey) String() string            { return "MsgCreateAPIKey" }
func (*MsgCreateAPIKey) XXX_MessageName() string   { return "zerone.knowledge.v1.MsgCreateAPIKey" }

type MsgCreateAPIKeyResponse struct {
	KeyHash string `json:"key_hash"`
}

func (*MsgCreateAPIKeyResponse) ProtoMessage()  {}
func (*MsgCreateAPIKeyResponse) Reset()         {}
func (*MsgCreateAPIKeyResponse) String() string { return "MsgCreateAPIKeyResponse" }

// MsgRevokeAPIKey deactivates an API key.
type MsgRevokeAPIKey struct {
	Owner   string `json:"owner"`
	KeyHash string `json:"key_hash"`
}

func (m *MsgRevokeAPIKey) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Owner); err != nil {
		return fmt.Errorf("invalid owner address: %w", err)
	}
	if m.KeyHash == "" {
		return fmt.Errorf("key_hash is required")
	}
	return nil
}

func (m *MsgRevokeAPIKey) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Owner)
	return []sdk.AccAddress{addr}
}

func (*MsgRevokeAPIKey) ProtoMessage()             {}
func (*MsgRevokeAPIKey) Reset()                    {}
func (*MsgRevokeAPIKey) String() string            { return "MsgRevokeAPIKey" }
func (*MsgRevokeAPIKey) XXX_MessageName() string   { return "zerone.knowledge.v1.MsgRevokeAPIKey" }

type MsgRevokeAPIKeyResponse struct{}

func (*MsgRevokeAPIKeyResponse) ProtoMessage()  {}
func (*MsgRevokeAPIKeyResponse) Reset()         {}
func (*MsgRevokeAPIKeyResponse) String() string { return "MsgRevokeAPIKeyResponse" }

// MsgDepositAPICredits deposits ZRN into a prepaid API balance.
type MsgDepositAPICredits struct {
	Depositor string `json:"depositor"`
	Amount    string `json:"amount"` // uzrn
}

func (m *MsgDepositAPICredits) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Depositor); err != nil {
		return fmt.Errorf("invalid depositor address: %w", err)
	}
	if m.Amount == "" || m.Amount == "0" {
		return fmt.Errorf("deposit amount must be > 0")
	}
	return nil
}

func (m *MsgDepositAPICredits) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Depositor)
	return []sdk.AccAddress{addr}
}

func (*MsgDepositAPICredits) ProtoMessage()             {}
func (*MsgDepositAPICredits) Reset()                    {}
func (*MsgDepositAPICredits) String() string            { return "MsgDepositAPICredits" }
func (*MsgDepositAPICredits) XXX_MessageName() string   { return "zerone.knowledge.v1.MsgDepositAPICredits" }

type MsgDepositAPICreditsResponse struct {
	NewBalance string `json:"new_balance"`
}

func (*MsgDepositAPICreditsResponse) ProtoMessage()  {}
func (*MsgDepositAPICreditsResponse) Reset()         {}
func (*MsgDepositAPICreditsResponse) String() string { return "MsgDepositAPICreditsResponse" }

// MsgWithdrawAPICredits withdraws unused API credits.
type MsgWithdrawAPICredits struct {
	Wallet string `json:"wallet"`
	Amount string `json:"amount"` // uzrn
}

func (m *MsgWithdrawAPICredits) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Wallet); err != nil {
		return fmt.Errorf("invalid wallet address: %w", err)
	}
	if m.Amount == "" || m.Amount == "0" {
		return fmt.Errorf("withdrawal amount must be > 0")
	}
	return nil
}

func (m *MsgWithdrawAPICredits) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Wallet)
	return []sdk.AccAddress{addr}
}

func (*MsgWithdrawAPICredits) ProtoMessage()             {}
func (*MsgWithdrawAPICredits) Reset()                    {}
func (*MsgWithdrawAPICredits) String() string            { return "MsgWithdrawAPICredits" }
func (*MsgWithdrawAPICredits) XXX_MessageName() string   { return "zerone.knowledge.v1.MsgWithdrawAPICredits" }

type MsgWithdrawAPICreditsResponse struct {
	RemainingBalance string `json:"remaining_balance"`
}

func (*MsgWithdrawAPICreditsResponse) ProtoMessage()  {}
func (*MsgWithdrawAPICreditsResponse) Reset()         {}
func (*MsgWithdrawAPICreditsResponse) String() string { return "MsgWithdrawAPICreditsResponse" }

// MsgRecordAPIUsage records batched API usage from the payment bridge.
type MsgRecordAPIUsage struct {
	Bridge  string           `json:"bridge"`
	Batches []*APIUsageBatch `json:"batches"`
}

func (m *MsgRecordAPIUsage) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Bridge); err != nil {
		return fmt.Errorf("invalid bridge address: %w", err)
	}
	if len(m.Batches) == 0 {
		return fmt.Errorf("at least one usage batch is required")
	}
	for i, b := range m.Batches {
		if b.APIKeyHash == "" {
			return fmt.Errorf("batch %d: api_key_hash is required", i)
		}
		if b.InputTokens == 0 && b.OutputTokens == 0 {
			return fmt.Errorf("batch %d: must have at least input or output tokens", i)
		}
	}
	return nil
}

func (m *MsgRecordAPIUsage) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Bridge)
	return []sdk.AccAddress{addr}
}

func (*MsgRecordAPIUsage) ProtoMessage()             {}
func (*MsgRecordAPIUsage) Reset()                    {}
func (*MsgRecordAPIUsage) String() string            { return "MsgRecordAPIUsage" }
func (*MsgRecordAPIUsage) XXX_MessageName() string   { return "zerone.knowledge.v1.MsgRecordAPIUsage" }

type MsgRecordAPIUsageResponse struct {
	TotalDeducted    string `json:"total_deducted"`
	BatchesProcessed uint64 `json:"batches_processed"`
}

func (*MsgRecordAPIUsageResponse) ProtoMessage()  {}
func (*MsgRecordAPIUsageResponse) Reset()         {}
func (*MsgRecordAPIUsageResponse) String() string { return "MsgRecordAPIUsageResponse" }

// ─── Event constants ────────────────────────────────────────────────────────

const (
	EventAPIKeyCreated       = "api_key_created"
	EventAPIKeyRevoked       = "api_key_revoked"
	EventAPICreditsDeposited = "api_credits_deposited"
	EventAPICreditsWithdrawn = "api_credits_withdrawn"
	EventAPIUsageRecorded    = "api_usage_recorded"
	EventAPIRevenueDistributed = "api_revenue_distributed"

	AttributeAPIKeyHash    = "key_hash"
	AttributeWallet        = "wallet"
	AttributeDepositAmount = "deposit_amount"
	AttributeNewBalance    = "new_balance"
	AttributeInputTokens   = "input_tokens"
	AttributeOutputTokens  = "output_tokens"
	AttributeTotalCost     = "total_cost"
)
