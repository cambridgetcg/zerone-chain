# T3-2 — API Gateway

## Goal

Build an OpenAI-compatible API gateway in Go that authenticates requests, checks ZRN balances, routes to inference servers, meters usage, and returns responses. Drop-in replacement for OpenAI API — existing tools (LangChain, LlamaIndex, etc.) work by changing the base URL.

## Deliverables

### 1. OpenAI-Compatible Endpoints

```
POST /v1/chat/completions     — Chat completion (streaming + non-streaming)
POST /v1/completions          — Text completion
GET  /v1/models               — List available models
```

Request/response formats must match OpenAI API spec exactly:
- Chat completions: messages array, temperature, max_tokens, stream, etc.
- Streaming: SSE format with `data: {...}\n\n` chunks
- Model listing: id, object, created, owned_by fields

### 2. Authentication

Two auth methods:
- **API Key**: `Authorization: Bearer zrn_<key>` — key linked to a wallet address + deposit
- **Wallet Signature**: Sign request hash with wallet private key, include in header — for agents with direct chain access

API key management:
- `POST /v1/keys/create` — Create API key (requires wallet signature)
- `DELETE /v1/keys/{key_id}` — Revoke key
- `GET /v1/keys` — List active keys
- Keys stored in PostgreSQL with bcrypt hash

### 3. Request Pipeline

```
Request → Auth → Balance Check → Rate Limit → Route → Inference → Meter → Respond
```

1. **Auth**: Validate API key or wallet signature
2. **Balance check**: Call payment bridge, reject if insufficient ZRN
3. **Rate limit**: Token bucket per API key (configurable limits per tier)
4. **Route**: Forward to inference server (vLLM) via internal gRPC/HTTP
5. **Inference**: Wait for response (or stream chunks)
6. **Meter**: Count input + output tokens, call payment bridge to deduct
7. **Respond**: Return OpenAI-compatible response with usage stats

### 4. ZERONE-Specific Endpoints

Beyond OpenAI compat:
```
GET  /v1/balance              — Check ZRN balance for authenticated user
GET  /v1/usage                — Usage history (tokens, cost, by model/day)
POST /v1/deposit              — Get deposit instructions (chain address + memo)
GET  /v1/datasets             — List available datasets for purchase
POST /v1/datasets/purchase    — Initiate dataset purchase via blind storage
GET  /v1/datasets/{id}/status — Check purchase/download progress
```

### 5. Inference Server Integration

- Connect to vLLM server via HTTP (OpenAI-compatible mode)
- Health checking: mark inference servers as healthy/unhealthy
- Load balancing: round-robin across multiple inference servers
- Model routing: different models on different servers

### 6. Observability

- Request logging (sanitized — no prompt content in logs)
- Prometheus metrics: request count, latency p50/p95/p99, tokens/sec, error rate, revenue
- Health endpoint: `GET /healthz`
- Ready endpoint: `GET /readyz` (healthy + inference server connected + payment bridge connected)

## Working Directory

`/Users/yournameisai/Desktop/zerone/services/api-gateway/`

## Output

- Go module with HTTP server (chi or stdlib)
- OpenAPI spec for all endpoints
- Integration tests against mock inference server + mock payment bridge
- Dockerfile
- Example client usage (curl, Python)
