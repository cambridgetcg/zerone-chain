# R44-1 — API Revenue Model

## Objective

Formalize the API revenue model: how API usage translates to on-chain payments, how revenue splits between stakeholders, and how the API gateway bridges off-chain requests to on-chain access transactions.

## Context

Existing infrastructure:
- `services/api-gateway/` — OpenAI-compatible API (1,582 lines) with auth, rate limiting, inference pool
- `services/payment-bridge/` — payment bridge service (552 lines)
- `x/knowledge/keeper/access.go` — on-chain AccessSample/AccessDataset with pricing
- `x/knowledge/keeper/revenue.go` — epoch revenue distribution (submitter/validator/protocol split)
- Params: `access_fee_per_sample`, `submitter_revenue_share_bps` (55%), `validator_revenue_share_bps` (22%), quality multipliers

## What's Missing

### 1. Per-Token API Pricing
The API gateway charges per request, but doesn't map to per-token pricing:
- Add `price_per_input_token` and `price_per_output_token` to knowledge params (uzrn)
- Default: 1 uzrn per 1000 input tokens, 3 uzrn per 1000 output tokens
- API gateway: after inference, compute token count × price, record payment

### 2. API Key → Wallet Binding
- API keys must be linked to a zerone wallet address
- `MsgCreateAPIKey`: registers an API key hash on-chain, binds to wallet
- `MsgRevokeAPIKey`: deactivates a key
- On-chain: `APIKeyRecord { key_hash, wallet, created_at, revoked, rate_limit_tier }`

### 3. Prepaid Balance / Credit System
- Users deposit ZRN into a prepaid balance: `MsgDepositAPICredits`
- API calls deduct from prepaid balance (no per-call tx needed)
- Low balance warning at 10% remaining
- Zero balance → 402 Payment Required
- `MsgWithdrawAPICredits`: withdraw unused credits

### 4. Usage Metering Bridge
Connect API gateway → chain:
- Gateway batches usage records every N seconds (configurable, default 30s)
- Batch includes: `{ api_key_hash, input_tokens, output_tokens, request_count, model_used }`
- Payment bridge submits `MsgRecordAPIUsage` on-chain
- On-chain: deduct from prepaid balance, record revenue for distribution

### 5. Revenue Split Enhancement  
Current split: submitter 55% / validator 22% / protocol 23%
Add API revenue split:
- Model training contributors: 40% (distributed by TDU fitness scores)
- Infrastructure (validators): 25%
- Submitters (data providers): 20%
- Protocol treasury: 10%
- Research fund: 5%
- These are configurable via params

### 6. Model Attribution
When an API request uses a fine-tuned model:
- Track which LoRA adapter served the response
- Map adapter → training run → dataset → individual TDUs
- Revenue for that request flows to the TDUs that trained that model
- This closes the loop: good data → better model → more API usage → more revenue → data providers paid

## Implementation

### On-chain (x/knowledge/)

New messages in `msg_server.go`:
- `MsgCreateAPIKey` / `MsgRevokeAPIKey`
- `MsgDepositAPICredits` / `MsgWithdrawAPICredits`  
- `MsgRecordAPIUsage` (batch usage recording from payment bridge)

New state:
- `APIKeyRecord` — key hash → wallet mapping
- `APIBalance` — wallet → prepaid balance
- `UsageRecord` — per-epoch usage aggregates per wallet

New params:
- `price_per_input_token`, `price_per_output_token`
- `api_revenue_*_share_bps` for the 5-way split

### Off-chain (services/)

Update `api-gateway`:
- Token counting after inference response
- Usage batching and submission to payment bridge
- Balance checking before serving requests

Update `payment-bridge`:
- Batch `MsgRecordAPIUsage` submission
- Balance query for the gateway

## Tests

- Test: API key creation binds to wallet
- Test: credit deposit increases balance
- Test: usage recording deducts correct amount (input + output tokens)
- Test: zero balance blocks further API calls
- Test: revenue distribution splits correctly across 5 recipients
- Test: model attribution traces revenue to correct TDUs
- Test: withdrawal returns unused credits
- Test: revoked key cannot be used

## Key Files

- `x/knowledge/keeper/api_revenue.go` — NEW
- `x/knowledge/keeper/msg_server.go` — add handlers
- `x/knowledge/types/api.go` — NEW types
- `services/api-gateway/internal/handler/handler.go` — update
- `services/payment-bridge/internal/` — update

## Constraints

- All monetary amounts in uzrn
- API pricing must be competitive with centralized alternatives
- Batch usage recording to minimize on-chain tx count
- Credit system avoids per-call blockchain transactions (latency)
