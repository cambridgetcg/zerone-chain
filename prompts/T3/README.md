# T3 — ZRN Payment Integration & API Gateway

**Goal:** Build the API gateway and payment bridge that connects inference requests to ZRN on-chain payments. Drop-in OpenAI-compatible API, metered and paid in ZRN.

## Sessions (3)

| # | File | Scope |
|---|------|-------|
| T3-1 | T3-1-payment-bridge.md | Go service bridging off-chain usage metering with on-chain ZRN settlement |
| T3-2 | T3-2-api-gateway.md | OpenAI-compatible API gateway with auth, rate limiting, and usage metering |
| T3-3 | T3-3-billing-integration.md | Wire gateway to x/billing module, handle deposits/withdrawals/disputes |

## Run Order

Sequential: T3-1 → T3-2 → T3-3

## Prerequisites

- T1 architecture document
- ZERONE chain running (testnet)

## Exit Criteria

1. API gateway serves OpenAI-compatible endpoints
2. Requests authenticated via wallet signature or API key
3. Usage metered per-token and deducted from ZRN balance
4. Periodic batch settlement on-chain
5. Balance checks prevent over-spending
