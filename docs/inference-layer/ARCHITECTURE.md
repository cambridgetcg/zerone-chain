# ZERONE Inference Layer — System Architecture

> Reference document for T2–T5 implementation.
> Describes the off-chain service stack that exports on-chain training data,
> fine-tunes open-source LLMs, serves inference, and settles payments in ZRN.

---

## Table of Contents

1. [Component Diagram](#1-component-diagram)
2. [Data Flow Diagrams](#2-data-flow-diagrams)
3. [Technology Choices](#3-technology-choices)
4. [Deployment Topology](#4-deployment-topology)
5. [Security Model](#5-security-model)
6. [Interface Contracts](#6-interface-contracts)
7. [Appendix: On-Chain Data Model Reference](#7-appendix-on-chain-data-model-reference)
8. [Blind Storage Protocol](#8-blind-storage-protocol)
9. [Token Economics](#9-token-economics)

---

## 1. Component Diagram

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                            ZERONE INFERENCE LAYER                            │
│                                                                             │
│  ┌──────────────┐   ┌──────────────┐   ┌──────────────┐                    │
│  │   Dataset     │──▶│   Training   │──▶│    Model     │                    │
│  │   Exporter    │   │   Pipeline   │   │   Registry   │                    │
│  └──────┬───────┘   └──────────────┘   └──────┬───────┘                    │
│         │                                      │                            │
│    chain events                          weight files                       │
│         │                                      │                            │
│  ┌──────┴───────┐                      ┌───────▼───────┐                    │
│  │   ZERONE      │                      │   Inference   │                    │
│  │   Chain       │◀─── settlement ─────│   Server      │                    │
│  │   (on-chain)  │                      │   (vLLM)      │                    │
│  └──────┬───────┘                      └───────▲───────┘                    │
│         │                                      │                            │
│    balance checks                         routing                           │
│         │                                      │                            │
│  ┌──────▼───────┐   ┌──────────────┐   ┌──────┴───────┐                    │
│  │   Payment     │◀──│   Metering   │◀──│     API      │◀── clients        │
│  │   Bridge      │   │   Ledger     │   │   Gateway    │                    │
│  └──────────────┘   └──────────────┘   └──────────────┘                    │
│                                                                             │
│  ┌──────────────┐   ┌──────────────┐                                       │
│  │   Blind       │   │  Monitoring  │                                       │
│  │   Storage     │   │  & Metrics   │                                       │
│  └──────────────┘   └──────────────┘                                       │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Component Descriptions

#### Dataset Exporter

Watches the ZERONE chain for approved samples and exports them into
training-ready formats. Subscribes to `NewBlock` events via CometBFT
WebSocket and queries the `x/knowledge` gRPC service.

Responsibilities:
- Subscribe to chain events (new samples reaching GOLD/SILVER/BRONZE status,
  completed quality rounds at prefix `0x57`)
- Query samples by domain (index `0x0B`), thread (index `0x0A`), submitter
  (index `0x0C`)
- Transform samples into training formats based on `SampleType`:
  - **Instruction pair** — Q_AND_A, TUTORIAL, EXPLANATION, TROUBLESHOOT
  - **Conversation/multi-turn** — DISCUSSION, DEBATE (thread-aware via
    `thread_id`, `parent_sample_id`, `thread_position`)
  - **Preference pair** — REVIEW, OPINION, ANNOTATION (quality score as
    preference signal)
  - **Raw text** — NARRATIVE, CREATIVE, CORRECTION
- Deduplicate via `content_hash` (index `0x0E`)
- Write transformed data to staging area in JSONL format
- Track export watermark (last processed `verdict_block`) for resumability

Data model awareness:
- 12 SampleTypes: DISCUSSION, DEBATE, EXPLANATION, TROUBLESHOOT, REVIEW,
  TUTORIAL, OPINION, NARRATIVE, Q_AND_A, CREATIVE, ANNOTATION, CORRECTION
- Quality tiers: gold (≥800K BPS), silver (≥600K), bronze (≥400K)
- Consent types affect weighting: SELF_AUTHORED (1.5×) → FAIR_USE (0.5×)
- Thread structure: `thread_id` groups samples; `thread_position` orders them;
  `thread_depth` indicates nesting

#### Training Pipeline

Takes exported datasets and runs fine-tuning jobs on open-source base models.
Manages model versioning and triggers on dataset milestones.

Responsibilities:
- Monitor staging area for new data past threshold (configurable: N new
  samples or on-demand trigger)
- Create dataset snapshots (frozen point-in-time copies)
- Run fine-tuning: LoRA/QLoRA on base models (Llama, Mistral, Qwen)
- Domain-focused training: separate adapters per knowledge domain
- Quality-weighted sampling: gold samples 3× weight, silver 2×, bronze 1×
  (mirrors on-chain `gold_quality_multiplier` = 30000, etc.)
- Consent-weighted sampling: self-authored 1.5×, fair-use 0.5× (mirrors
  on-chain consent multipliers)
- Track training runs with hyperparameters, loss curves, eval metrics
- Push trained weights to Model Registry on completion
- Manage GPU allocation (training jobs are batch, not latency-sensitive)

Trigger modes:
- **Threshold**: every N new approved samples (default: 1000)
- **Scheduled**: periodic (e.g., weekly)
- **On-demand**: manual trigger via admin API

#### Model Registry

Stores trained model weights with metadata. Tracks active serving model.

Responsibilities:
- Store model artifacts (weights, adapter files, tokenizer configs)
- Record metadata per model version:
  - Base model (e.g., `meta-llama/Llama-3.1-8B`)
  - Dataset snapshot ID and version
  - Domain focus (or "general")
  - Training config (LoRA rank, learning rate, epochs)
  - Eval metrics (perplexity, domain-specific benchmarks)
  - Sample count, quality distribution, consent distribution
- Track promotion history (which model is active for serving)
- Support rollback (instant switch to previous version)
- Garbage-collect old weights (configurable retention)

Storage layout:
```
models/
  {model_id}/
    metadata.json
    adapter/           # LoRA adapter weights
    base_model_ref     # pointer to base model (not duplicated)
    eval_results.json
```

#### Inference Server

Serves the active model via OpenAI-compatible API. Uses vLLM for high
throughput with continuous batching.

Responsibilities:
- Load active model from registry (base + LoRA adapter)
- Serve chat completions and text completions
- Continuous batching for throughput optimization
- Streaming responses (SSE)
- Token counting for metering (prompt + completion tokens)
- Health checks and readiness probes
- Hot-reload: swap LoRA adapter without restarting base model
- Report usage metrics to metering ledger

Configuration:
- `--model`: base model path
- `--lora-modules`: active adapter(s)
- `--tensor-parallel-size`: GPU count
- `--max-model-len`: context window
- `--gpu-memory-utilization`: VRAM target (default 0.9)

#### API Gateway

Public-facing service handling authentication, rate limiting, request routing,
and usage metering.

Responsibilities:
- **Authentication**: verify requests via:
  - API key (linked to ZRN deposit account)
  - Wallet signature (EIP-191 style with cosmos address)
- **Authorization**: check ZRN balance ≥ estimated request cost before routing
- **Rate limiting**: per-account token bucket (configurable burst/sustained)
- **Request routing**: forward to inference server, handle failover
- **Usage metering**: count tokens (prompt + completion) per request, write to
  metering ledger
- **Streaming proxy**: pass through SSE from inference server to client
- **Request validation**: max tokens, model availability, content filtering
- **Cost estimation**: estimate token cost pre-request using billing module
  pricing (base price + confidence/novelty/freshness adjustments from
  `x/billing` parameters)

#### Metering Ledger

Off-chain usage accounting between API calls and on-chain settlement.

Responsibilities:
- Record per-request usage: account, tokens, timestamp, model, cost
- Maintain running balances per account (prepaid ZRN minus consumed)
- Accumulate usage for batch settlement
- Provide usage history and analytics queries
- Write-ahead log for crash recovery
- Idempotent recording (request ID dedup)

Schema (PostgreSQL):
```sql
usage_events (
  id            BIGSERIAL PRIMARY KEY,
  request_id    UUID UNIQUE NOT NULL,
  account       TEXT NOT NULL,
  model         TEXT NOT NULL,
  prompt_tokens INT NOT NULL,
  completion_tokens INT NOT NULL,
  cost_uzrn     BIGINT NOT NULL,
  created_at    TIMESTAMPTZ DEFAULT now()
)

account_balances (
  account       TEXT PRIMARY KEY,
  deposit_uzrn  BIGINT NOT NULL DEFAULT 0,
  consumed_uzrn BIGINT NOT NULL DEFAULT 0,
  last_settled  TIMESTAMPTZ
)
```

#### Payment Bridge

Connects off-chain usage to on-chain settlement. Monitors deposits, performs
batch settlement, handles the financial boundary.

Responsibilities:
- **Deposit monitoring**: watch for `transfer` events to the inference escrow
  module account on-chain; credit off-chain balance
- **Batch settlement**: periodically (every N blocks or M minutes) submit
  settlement transactions on-chain:
  - Aggregate usage per account
  - Submit `MsgBatchQueryFacts` or custom settlement Msg
  - Revenue splits applied on-chain per `x/billing` RevenueSplit:
    - 55% provider (inference operator)
    - 22% protocol pools
    - 3.33% research fund
    - 19.67% development fund
- **Balance reconciliation**: compare off-chain ledger with on-chain escrow
- **Dispute handling**: flag discrepancies, pause accounts if needed
- **Refund processing**: return unused deposits on account closure

Settlement flow:
```
Off-chain metering ledger
  → aggregate unsettled usage per account
  → construct batch settlement tx
  → sign with operator key
  → broadcast to ZERONE chain
  → verify inclusion
  → mark usage as settled in ledger
  → emit settlement receipt
```

On-chain integration points:
- `x/billing.MsgBatchQueryFacts` — triggers payment distribution
- `x/compute_pool.MsgHeartbeat` — maintain provider active status
- `x/compute_pool` provider registration (service_type="inference")
- Deposit escrow via standard `x/bank` transfers

#### Blind Storage Network

Distributes encrypted training datasets to purchasers. Detailed design in
T1-2 (separate document).

Responsibilities:
- Encrypt dataset chunks with buyer-specific keys
- Distribute chunks across storage nodes (registered in `x/compute_pool`
  with service_type="storage")
- Release chunks proportional to payment received
- Verify storage node integrity via challenge-response
- Buyer reassembles full dataset from chunks

Integration:
- Dataset definitions from `x/knowledge` (filter criteria, sample counts)
- Pricing from `x/knowledge` (`price_per_sample`, `bulk_price`)
- Storage providers from `x/compute_pool` (service_type="storage")
- Payment via `x/billing` distribution

#### Monitoring & Metrics

Observability across the entire inference layer.

Responsibilities:
- Model performance: latency p50/p95/p99, throughput (tokens/sec),
  error rates, GPU utilization
- API metrics: requests/sec, active connections, auth failures,
  rate limit hits
- Financial metrics: revenue/hour, settlement lag, balance coverage,
  deposit inflow
- Training metrics: job duration, loss curves, eval scores per run
- Storage metrics: chunk availability, node health, retrieval latency
- Alerting: PagerDuty/Slack integration for SLA violations
- Dashboard: Grafana with Prometheus data source

---

## 2. Data Flow Diagrams

### Flow 1: Submission → Training

```
┌──────────┐    ┌─────────────┐    ┌──────────┐    ┌──────────────┐
│  ZERONE   │    │  Quality    │    │ Dataset  │    │  Staging     │
│  Chain    │    │  Round      │    │ Exporter │    │  Area (S3)   │
└────┬─────┘    └──────┬──────┘    └────┬─────┘    └──────┬───────┘
     │                 │                │                  │
     │  MsgSubmitData  │                │                  │
     ├────────────────▶│                │                  │
     │                 │                │                  │
     │  commit/reveal  │                │                  │
     │  (validators)   │                │                  │
     │◀───────────────▶│                │                  │
     │                 │                │                  │
     │  verdict: GOLD  │                │                  │
     │  (block event)  │                │                  │
     ├─────────────────┤                │                  │
     │                 │                │                  │
     │                 │  subscribe     │                  │
     │                 │  NewBlock      │                  │
     │                 ├───────────────▶│                  │
     │                 │                │                  │
     │                 │                │  query sample    │
     │◀─────────────────────────────────┤  by ID (gRPC)   │
     ├──────────────────────────────────▶                  │
     │                 │                │                  │
     │                 │                │  query thread    │
     │◀─────────────────────────────────┤  (if threaded)   │
     ├──────────────────────────────────▶                  │
     │                 │                │                  │
     │                 │                │  transform by    │
     │                 │                │  SampleType      │
     │                 │                │  ─────────────▶  │
     │                 │                │  write JSONL     │
     │                 │                │────────────────▶ │
     │                 │                │                  │
```

**Detail:**

1. Submitter sends `MsgSubmitData` with content, domain, sample_type, consent
   proof, stake (min 1 ZRN)
2. Chain creates Submission (status=PENDING), initiates QualityRound
3. Selected validators commit hashes, then reveal multi-dimensional
   QualityVotes (reasoning_depth, novelty, toxicity, factual_accuracy,
   consent_valid)
4. Aggregation produces verdict (GOLD/SILVER/BRONZE/REJECT) and creates
   Sample with quality scores
5. Dataset Exporter detects completed round via CompletedRoundIndex
   (prefix `0x57`, keyed by verdict_block)
6. Exporter queries full Sample via gRPC `knowledge.Sample(id)`
7. If sample is threaded, queries siblings via `knowledge.SamplesByThread`
   to preserve conversational context
8. Transforms content based on SampleType:
   - Q_AND_A → `{"instruction": <question>, "output": <answer>}`
   - DISCUSSION → multi-turn conversation array
   - TUTORIAL → `{"instruction": "Teach me about X", "output": <content>}`
   - etc.
9. Writes JSONL to S3 staging with metadata (quality_tier, domain, consent
   multiplier, fitness_score)
10. Updates export watermark to current verdict_block

### Flow 2: Training → Serving

```
┌──────────┐    ┌──────────────┐    ┌──────────┐    ┌──────────────┐
│ Staging  │    │  Training    │    │  Model   │    │  Inference   │
│ Area     │    │  Pipeline    │    │ Registry │    │  Server      │
└────┬─────┘    └──────┬───────┘    └────┬─────┘    └──────┬───────┘
     │                 │                 │                  │
     │  threshold met  │                 │                  │
     │  (N new samples)│                 │                  │
     ├────────────────▶│                 │                  │
     │                 │                 │                  │
     │  create snapshot│                 │                  │
     │◀────────────────┤                 │                  │
     │  (freeze data)  │                 │                  │
     ├────────────────▶│                 │                  │
     │                 │                 │                  │
     │                 │  quality-weight │                  │
     │                 │  sampling:      │                  │
     │                 │  gold=3× weight │                  │
     │                 │  silver=2×      │                  │
     │                 │  bronze=1×      │                  │
     │                 │                 │                  │
     │                 │  fine-tune      │                  │
     │                 │  (LoRA/QLoRA)   │                  │
     │                 │  ───────────    │                  │
     │                 │                 │                  │
     │                 │  eval + metrics │                  │
     │                 │  ───────────    │                  │
     │                 │                 │                  │
     │                 │  register model │                  │
     │                 ├────────────────▶│                  │
     │                 │                 │                  │
     │                 │                 │  promote to      │
     │                 │                 │  active          │
     │                 │                 ├─────────────────▶│
     │                 │                 │  (hot-reload     │
     │                 │                 │   LoRA adapter)  │
     │                 │                 │                  │
```

**Detail:**

1. Training Pipeline polls staging area. When new sample count since last
   snapshot ≥ threshold (default 1000), it triggers
2. Creates frozen snapshot: copies staging JSONL to versioned directory,
   records sample IDs and quality distribution
3. Applies quality-weighted sampling during training:
   - Gold samples: weight 3× (from `gold_quality_multiplier` = 30000)
   - Silver: 2× (`silver_quality_multiplier` = 20000)
   - Bronze: 1× (`bronze_quality_multiplier` = 10000)
   - Consent multiplier applied: self-authored 1.5×, fair-use 0.5×
4. Runs LoRA/QLoRA fine-tuning on base model (e.g., 4-bit quantized Llama-3.1-8B)
5. Evaluates on held-out set + domain-specific benchmarks
6. Registers model in Model Registry with full metadata
7. If eval metrics pass promotion criteria (configurable thresholds),
   promotes to active
8. Inference Server hot-reloads LoRA adapter (vLLM supports dynamic LoRA
   loading without restarting the base model)

### Flow 3: API Request → Response

```
┌──────────┐    ┌──────────────┐    ┌──────────┐    ┌──────────────┐
│  Client  │    │  API Gateway │    │ Metering │    │  Inference   │
│          │    │              │    │ Ledger   │    │  Server      │
└────┬─────┘    └──────┬───────┘    └────┬─────┘    └──────┬───────┘
     │                 │                 │                  │
     │  POST /v1/chat/ │                 │                  │
     │  completions    │                 │                  │
     │  + API key      │                 │                  │
     ├────────────────▶│                 │                  │
     │                 │                 │                  │
     │                 │  lookup account │                  │
     │                 │  + verify key   │                  │
     │                 │  ───────────    │                  │
     │                 │                 │                  │
     │                 │  check balance  │                  │
     │                 ├────────────────▶│                  │
     │                 │  balance OK     │                  │
     │                 │◀────────────────┤                  │
     │                 │                 │                  │
     │                 │  estimate cost  │                  │
     │                 │  (max_tokens ×  │                  │
     │                 │   price/token)  │                  │
     │                 │  ───────────    │                  │
     │                 │                 │                  │
     │                 │  reserve amount │                  │
     │                 ├────────────────▶│                  │
     │                 │                 │                  │
     │                 │  forward request│                  │
     │                 ├─────────────────────────────────▶ │
     │                 │                 │                  │
     │  stream SSE     │◀─────────────────────────────────┤│
     │◀────────────────┤                 │  stream tokens   │
     │  data: {...}    │                 │                  │
     │  data: {...}    │                 │                  │
     │  data: [DONE]   │                 │                  │
     │◀────────────────┤                 │                  │
     │                 │                 │                  │
     │                 │  record actual  │                  │
     │                 │  usage + cost   │                  │
     │                 ├────────────────▶│                  │
     │                 │  (release       │                  │
     │                 │   reservation   │                  │
     │                 │   overage)      │                  │
     │                 │                 │                  │
```

**Detail:**

1. Client sends OpenAI-compatible request with API key in `Authorization`
   header
2. Gateway looks up account by API key in Redis cache (fallback: PostgreSQL)
3. Gateway checks balance in metering ledger: `deposit_uzrn - consumed_uzrn`
   must exceed estimated cost
4. Cost estimation: `estimated_tokens × price_per_token_uzrn`
   - Price derived from `x/billing` base_query_price and adjustments
   - Reserve estimated amount (soft lock in ledger)
5. Gateway forwards to inference server via internal gRPC/HTTP
6. Inference server streams tokens via SSE; gateway proxies to client
7. After stream completes, gateway records actual usage:
   - Prompt tokens + completion tokens (from vLLM response metadata)
   - Actual cost = actual_tokens × price_per_token
   - Release reservation overage (reserved − actual)
8. Metering ledger updates `consumed_uzrn` for the account

### Flow 4: Settlement

```
┌──────────┐    ┌──────────────┐    ┌──────────┐
│ Metering │    │  Payment     │    │  ZERONE  │
│ Ledger   │    │  Bridge      │    │  Chain   │
└────┬─────┘    └──────┬───────┘    └────┬─────┘
     │                 │                 │
     │  settlement     │                 │
     │  tick (periodic)│                 │
     │◀────────────────┤                 │
     │                 │                 │
     │  unsettled      │                 │
     │  usage batch    │                 │
     ├────────────────▶│                 │
     │                 │                 │
     │                 │  construct      │
     │                 │  settlement tx  │
     │                 │  ───────────    │
     │                 │                 │
     │                 │  broadcast tx   │
     │                 ├────────────────▶│
     │                 │                 │
     │                 │                 │  apply revenue
     │                 │                 │  splits:
     │                 │                 │  55% provider
     │                 │                 │  22% protocol
     │                 │                 │  3.33% research
     │                 │                 │  19.67% dev fund
     │                 │                 │  ───────────
     │                 │                 │
     │                 │  tx confirmed   │
     │                 │◀────────────────┤
     │                 │                 │
     │  mark settled   │                 │
     │◀────────────────┤                 │
     │                 │                 │
```

**Detail:**

1. Payment Bridge runs a settlement ticker (configurable: every 100 blocks
   or every 10 minutes, whichever comes first)
2. Queries metering ledger for unsettled usage since last settlement,
   aggregated per account
3. Constructs batch settlement transaction referencing:
   - Total uzrn consumed per account
   - Usage proof (Merkle root of usage events for auditability)
4. Signs with operator key and broadcasts to ZERONE chain
5. On-chain `x/billing` processes payment distribution per RevenueSplit:
   - 55% to inference provider (operator)
   - 22% to protocol pools (knowledge pool + verification + treasury)
   - 3.33% to research fund
   - 19.67% to development fund
6. Bridge waits for tx confirmation (finality at 1 block on CometBFT)
7. Marks usage events as settled in metering ledger
8. Emits settlement receipt with tx hash for auditability

### Flow 5: Dataset Purchase (Blind Storage)

```
┌──────────┐    ┌──────────────┐    ┌──────────┐    ┌──────────────┐
│  Buyer   │    │  API Gateway │    │  ZERONE  │    │  Blind       │
│          │    │              │    │  Chain   │    │  Storage     │
└────┬─────┘    └──────┬───────┘    └────┬─────┘    └──────┬───────┘
     │                 │                 │                  │
     │  GET /v1/       │                 │                  │
     │  datasets       │                 │                  │
     ├────────────────▶│                 │                  │
     │                 │  query datasets │                  │
     │                 ├────────────────▶│                  │
     │                 │  dataset list   │                  │
     │  dataset list   │◀────────────────┤                  │
     │◀────────────────┤                 │                  │
     │                 │                 │                  │
     │  POST /v1/      │                 │                  │
     │  datasets/      │                 │                  │
     │  purchase       │                 │                  │
     ├────────────────▶│                 │                  │
     │                 │                 │                  │
     │                 │  MsgAccessDataset                  │
     │                 ├────────────────▶│                  │
     │                 │                 │                  │
     │                 │                 │  payment applied │
     │                 │                 │  (bulk_price or  │
     │                 │                 │   per-sample)    │
     │                 │                 │  ───────────     │
     │                 │                 │                  │
     │                 │  access granted │                  │
     │                 │◀────────────────┤                  │
     │                 │                 │                  │
     │                 │  request chunks │                  │
     │                 ├─────────────────────────────────▶ │
     │                 │                 │                  │
     │                 │  encrypted      │                  │
     │  download       │  chunks         │                  │
     │  chunks         │◀─────────────────────────────────┤│
     │◀────────────────┤                 │                  │
     │                 │                 │                  │
     │  reassemble +   │                 │                  │
     │  decrypt locally│                 │                  │
     │  ───────────    │                 │                  │
```

**Detail:**

1. Buyer queries available datasets via API (proxied to `x/knowledge`
   `Datasets` gRPC query)
2. Each dataset has filter criteria (domain, sample_type, language, tags,
   min_quality) and pricing (price_per_sample, bulk_price)
3. Buyer initiates purchase via `POST /v1/datasets/purchase`
4. Gateway submits `MsgAccessDataset` on-chain (or buyer signs directly)
5. Chain applies payment: `bulk_price` if buying entire dataset, else
   `price_per_sample × sample_count`
6. Revenue distributed per on-chain rules (submitter 55%, validator 22%,
   consent multipliers applied)
7. Blind Storage releases encrypted chunks to buyer:
   - Dataset is pre-chunked and encrypted per-buyer (AES-256-GCM with
     buyer-specific key)
   - Chunks distributed across storage nodes
   - Buyer downloads from multiple nodes in parallel
8. Buyer reassembles and decrypts locally

---

## 3. Technology Choices

### Component Matrix

| Component | Language | Framework | Deployment | Storage | Communication |
|-----------|----------|-----------|------------|---------|---------------|
| Dataset Exporter | Go | Cosmos SDK client, CometBFT RPC | Docker | PostgreSQL (watermarks), S3 (JSONL) | gRPC to chain, NATS publish |
| Training Pipeline | Python | PyTorch, HuggingFace transformers, PEFT (LoRA) | Docker (GPU) | S3 (datasets, weights), PostgreSQL (job metadata) | NATS subscribe, S3 read/write |
| Model Registry | Go | — | Docker | PostgreSQL (metadata), S3 (weights) | gRPC internal API |
| Inference Server | Python | vLLM | Docker (GPU) | Local SSD (model cache), S3 (weight source) | OpenAI-compatible HTTP |
| API Gateway | Go | net/http, grpc-go | Docker | Redis (sessions, rate limits), PostgreSQL (accounts) | HTTPS (external), gRPC (internal) |
| Metering Ledger | Go | — | Docker | PostgreSQL (usage records, balances) | gRPC internal API |
| Payment Bridge | Go | Cosmos SDK client | Docker | PostgreSQL (settlement records) | gRPC to chain, gRPC to ledger |
| Blind Storage | Go | — | Docker | S3 (encrypted chunks) | gRPC (internal), HTTPS (download) |
| Monitoring | — | Prometheus + Grafana | Docker | Prometheus TSDB | Prometheus scrape, Grafana dashboards |

### Rationale

**Go for services** — matches the chain codebase (x/knowledge, x/billing,
x/compute_pool are all Go), shares proto-generated types, low memory
footprint, excellent concurrency for I/O-bound services.

**Python for ML** — PyTorch and HuggingFace ecosystem have no Go equivalent.
vLLM is Python-native and provides the best open-source inference throughput
(continuous batching, PagedAttention, tensor parallelism).

**vLLM over TGI** — vLLM offers:
- Dynamic LoRA loading (swap adapters without restart)
- Better throughput with PagedAttention
- Native OpenAI-compatible API server
- Active development and broad model support

**PostgreSQL** — ACID transactions for financial data (metering, settlement).
Rich querying for analytics. Proven at scale. Single database engine reduces
operational complexity.

**Redis** — Session cache, API key lookup, rate limiting (token bucket).
Ephemeral data that can be rebuilt from PostgreSQL on failure.

**S3-compatible storage** — Model weights (GB-scale), training datasets,
encrypted chunks. MinIO for self-hosted, AWS S3 for cloud.

**NATS** — Lightweight message queue for event-driven communication between
services. Simpler than Kafka, sufficient for our throughput needs. JetStream
mode for persistence when needed (e.g., training triggers must not be lost).

### Dependency Versions (Target)

| Dependency | Version | Notes |
|------------|---------|-------|
| Go | 1.24+ | Match chain codebase |
| Python | 3.11+ | vLLM requirement |
| vLLM | 0.6.x+ | Dynamic LoRA support |
| PyTorch | 2.4+ | CUDA 12.x |
| transformers | 4.45+ | Latest model support |
| PEFT | 0.13+ | LoRA/QLoRA |
| PostgreSQL | 16 | — |
| Redis | 7.x | — |
| NATS | 2.10+ | JetStream |
| MinIO | latest | S3-compatible |
| Prometheus | 2.x | — |
| Grafana | 10.x | — |

---

## 4. Deployment Topology

### Single-Node (Development / MVP)

```
┌─────────────────────────────────────────────────────────────┐
│  Single Server (GPU)                                        │
│                                                             │
│  ┌─────────────────────────────────┐  ┌──────────────────┐ │
│  │ Docker Compose                  │  │ GPU: 1× A100 80G │ │
│  │                                 │  │ or 1× H100 80G   │ │
│  │  ┌────────────┐ ┌────────────┐ │  └──────────────────┘ │
│  │  │ API Gateway│ │ Dataset    │ │                        │
│  │  │ :8080      │ │ Exporter   │ │  ┌──────────────────┐ │
│  │  └────────────┘ └────────────┘ │  │ CPU: 16+ cores   │ │
│  │                                 │  │ RAM: 128 GB      │ │
│  │  ┌────────────┐ ┌────────────┐ │  │ Disk: 2 TB NVMe  │ │
│  │  │ Inference  │ │ Training   │ │  └──────────────────┘ │
│  │  │ Server     │ │ Pipeline   │ │                        │
│  │  │ (vLLM)     │ │ (batch)    │ │  ┌──────────────────┐ │
│  │  │ :8000      │ │            │ │  │ Network:         │ │
│  │  └────────────┘ └────────────┘ │  │ 1 Gbps+          │ │
│  │                                 │  │ Static IP        │ │
│  │  ┌────────────┐ ┌────────────┐ │  └──────────────────┘ │
│  │  │ Payment    │ │ Model      │ │                        │
│  │  │ Bridge     │ │ Registry   │ │                        │
│  │  └────────────┘ └────────────┘ │                        │
│  │                                 │                        │
│  │  ┌────────────┐ ┌────────────┐ │                        │
│  │  │ PostgreSQL │ │ Redis      │ │                        │
│  │  │ :5432      │ │ :6379      │ │                        │
│  │  └────────────┘ └────────────┘ │                        │
│  │                                 │                        │
│  │  ┌────────────┐ ┌────────────┐ │                        │
│  │  │ NATS       │ │ MinIO (S3) │ │                        │
│  │  │ :4222      │ │ :9000      │ │                        │
│  │  └────────────┘ └────────────┘ │                        │
│  │                                 │                        │
│  │  ┌────────────┐ ┌────────────┐ │                        │
│  │  │ Prometheus │ │ Grafana    │ │                        │
│  │  │ :9090      │ │ :3000      │ │                        │
│  │  └────────────┘ └────────────┘ │                        │
│  └─────────────────────────────────┘                        │
└─────────────────────────────────────────────────────────────┘
```

**GPU sharing**: Training and inference share the GPU but do not run
simultaneously. Training runs during low-traffic hours (scheduled batch).
Inference takes priority — training jobs are preemptable.

### Multi-Node (Production)

```
┌─────────────────────┐    ┌─────────────────────┐    ┌─────────────────┐
│  Inference Nodes     │    │  Training Node       │    │  Service Node    │
│  (horizontally       │    │  (vertically         │    │  (stateless)     │
│   scaled)            │    │   scaled)            │    │                  │
│                      │    │                      │    │  API Gateway ×2  │
│  vLLM instance ×N    │    │  4–8× A100/H100     │    │  Payment Bridge  │
│  1× GPU each (8B)    │    │  Multi-GPU training  │    │  Dataset Exporter│
│  or 2× GPU (70B)     │    │  (FSDP / DeepSpeed)  │    │  Model Registry  │
│                      │    │                      │    │  Metering Ledger │
│  Load balanced by    │    │  Batch only, not     │    │                  │
│  API Gateway         │    │  latency-sensitive   │    │  PostgreSQL (HA) │
└─────────────────────┘    └─────────────────────┘    │  Redis Sentinel  │
                                                       │  NATS cluster    │
                                                       │  MinIO cluster   │
                                                       └─────────────────┘
```

### Scaling Strategy

| Component | Scaling | Trigger |
|-----------|---------|---------|
| Inference Server | Horizontal (add GPU nodes) | Request queue depth > threshold, p95 latency > 2s |
| Training Pipeline | Vertical (more GPUs per job) | Larger models, more data |
| API Gateway | Horizontal (stateless, behind LB) | Requests/sec > capacity |
| Dataset Exporter | Single instance (leader election) | N/A — chain event rate is bounded |
| Payment Bridge | Single instance (leader election) | N/A — settlement is periodic |
| PostgreSQL | Vertical + read replicas | Query load |
| Redis | Sentinel (HA) | N/A — primarily cache |

### GPU Requirements

| Workload | Minimum | Recommended | Notes |
|----------|---------|-------------|-------|
| Inference (8B model, 4-bit) | 1× A100 40G | 1× A100 80G | ~6 GB VRAM for 8B Q4 + KV cache |
| Inference (70B model, 4-bit) | 2× A100 80G | 4× A100 80G | Tensor parallel across GPUs |
| Training (8B LoRA) | 1× A100 80G | 2× A100 80G | LoRA rank 64, batch size 8 |
| Training (70B QLoRA) | 4× A100 80G | 8× A100 80G | QLoRA 4-bit base + LoRA adapters |

---

## 5. Security Model

### 5.1 API Authentication

**Two authentication methods:**

**Method A: API Key**
- User deposits ZRN on-chain to inference escrow address
- Receives API key (256-bit random, stored as SHA-256 hash in DB)
- Key included in `Authorization: Bearer <key>` header
- Key maps to on-chain account address + off-chain balance

**Method B: Wallet Signature**
- Client signs a challenge nonce with their cosmos private key
- Signature verified against on-chain account
- Session token issued (JWT, 1-hour TTL)
- Used for programmatic access from chain-aware clients

**Key management:**
- API keys stored as SHA-256 hashes (never plaintext)
- Rate-limited key generation (max 5 keys per account)
- Key revocation via API call
- Separate read-only keys for balance/usage queries

### 5.2 Inference Theft Prevention

**Prompt injection to extract training data:**
- System prompt is fixed and not overridable by user messages
- Output filtering: detect and block responses that contain verbatim training
  samples (n-gram matching against a sample of the training set)
- Rate limiting: cap tokens per minute to prevent bulk extraction
- No access to raw training data via inference API (separate dataset
  purchase flow via blind storage)

**Model weight protection:**
- Weights stored in S3 with server-side encryption (AES-256)
- Inference server runs in a locked-down container:
  - No SSH access
  - No outbound network except API gateway (egress firewall)
  - Read-only filesystem except model cache directory
- Model weights are LoRA adapters (small, domain-specific); base model is
  publicly available — the adapter alone has limited standalone value

### 5.3 Storage Node Verification

**Challenge-response protocol:**
- Orchestrator periodically sends random chunk IDs to storage nodes
- Node must return the correct chunk content (verified by hash)
- Failure to respond correctly within timeout → slash storage provider
  stake (via `x/compute_pool` jailing mechanism)
- Probabilistic auditing: check random subset, not every chunk every time

**Chunk integrity:**
- Each chunk has a SHA-256 hash stored in metadata DB
- Buyer verifies chunk hashes before reassembly
- Merkle tree of all chunk hashes committed on-chain for dispute resolution

### 5.4 Payment Security

**Double-spend prevention:**
- Balance reservations are atomic (PostgreSQL transaction)
- Settlement is idempotent (unique settlement_id per batch)
- On-chain escrow prevents spending beyond deposited amount

**Operator key protection:**
- Settlement signing key stored in hardware security module (HSM) or
  encrypted keyfile with passphrase
- Key rotation supported (new key registered on-chain)
- Multi-sig option for high-value settlements

### 5.5 Network Security

- All external traffic over TLS 1.3
- Internal service-to-service communication over mTLS
- API Gateway is the only publicly exposed service
- PostgreSQL, Redis, NATS accessible only from internal network
- Inference server accessible only from API Gateway (network policy)

### 5.6 Data Privacy

- User prompts are not logged by default (opt-in for debugging)
- Usage records contain token counts and costs, not prompt content
- Training data on-chain is public (consent-verified); no additional
  privacy concern for the exported dataset
- Blind storage encryption ensures only the buyer can read purchased data

---

## 6. Interface Contracts

### 6.1 Public API (OpenAI-Compatible)

All endpoints served from the API Gateway at `https://api.zerone.network/`.

#### Chat Completions

```
POST /v1/chat/completions
Authorization: Bearer <api_key>
Content-Type: application/json

{
  "model": "zerone-8b",
  "messages": [
    {"role": "system", "content": "You are a helpful assistant."},
    {"role": "user", "content": "Explain quantum computing."}
  ],
  "max_tokens": 1024,
  "temperature": 0.7,
  "stream": true
}

Response (streaming):
data: {"id":"chatcmpl-abc123","object":"chat.completion.chunk","created":1709654400,"model":"zerone-8b","choices":[{"index":0,"delta":{"content":"Quantum"},"finish_reason":null}]}
data: {"id":"chatcmpl-abc123","object":"chat.completion.chunk","created":1709654400,"model":"zerone-8b","choices":[{"index":0,"delta":{"content":" computing"},"finish_reason":null}]}
...
data: {"id":"chatcmpl-abc123","object":"chat.completion.chunk","created":1709654400,"model":"zerone-8b","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":25,"completion_tokens":312,"total_tokens":337}}
data: [DONE]
```

#### Text Completions

```
POST /v1/completions
Authorization: Bearer <api_key>
Content-Type: application/json

{
  "model": "zerone-8b",
  "prompt": "The meaning of decentralized AI is",
  "max_tokens": 256,
  "temperature": 0.7
}

Response:
{
  "id": "cmpl-xyz789",
  "object": "text_completion",
  "created": 1709654400,
  "model": "zerone-8b",
  "choices": [{
    "text": " that no single entity controls...",
    "index": 0,
    "finish_reason": "stop"
  }],
  "usage": {
    "prompt_tokens": 8,
    "completion_tokens": 64,
    "total_tokens": 72
  }
}
```

#### List Models

```
GET /v1/models
Authorization: Bearer <api_key>

Response:
{
  "object": "list",
  "data": [
    {
      "id": "zerone-8b",
      "object": "model",
      "created": 1709654400,
      "owned_by": "zerone-network",
      "meta": {
        "base_model": "meta-llama/Llama-3.1-8B-Instruct",
        "dataset_version": "snapshot-2026-03-01",
        "domain_focus": "general",
        "sample_count": 45230,
        "quality_distribution": {"gold": 12450, "silver": 21300, "bronze": 11480},
        "training_date": "2026-03-02T00:00:00Z"
      }
    },
    {
      "id": "zerone-8b-science",
      "object": "model",
      "created": 1709654400,
      "owned_by": "zerone-network",
      "meta": {
        "base_model": "meta-llama/Llama-3.1-8B-Instruct",
        "dataset_version": "snapshot-2026-03-01",
        "domain_focus": "science",
        "sample_count": 8920,
        "quality_distribution": {"gold": 3200, "silver": 4100, "bronze": 1620},
        "training_date": "2026-03-02T00:00:00Z"
      }
    }
  ]
}
```

### 6.2 Account Management API

#### Check Balance

```
GET /v1/balance
Authorization: Bearer <api_key>

Response:
{
  "account": "zerone1abc...xyz",
  "deposit_uzrn": "50000000",
  "consumed_uzrn": "12340000",
  "available_uzrn": "37660000",
  "pending_settlement_uzrn": "2100000",
  "last_settlement_block": 1523400,
  "last_settlement_tx": "A1B2C3..."
}
```

#### Get Deposit Address

```
POST /v1/deposit
Authorization: Bearer <api_key>

Response:
{
  "deposit_address": "zerone1escrow...addr",
  "memo": "deposit:zerone1abc...xyz",
  "min_deposit_uzrn": "1000000",
  "instructions": "Send ZRN to the deposit address with the memo field. Deposits are credited within 1 block (~6 seconds)."
}
```

#### Usage History

```
GET /v1/usage?start=2026-03-01T00:00:00Z&end=2026-03-05T00:00:00Z&limit=100
Authorization: Bearer <api_key>

Response:
{
  "object": "list",
  "data": [
    {
      "request_id": "req_abc123",
      "model": "zerone-8b",
      "prompt_tokens": 25,
      "completion_tokens": 312,
      "total_tokens": 337,
      "cost_uzrn": "33700",
      "created_at": "2026-03-04T14:23:01Z"
    }
  ],
  "has_more": false,
  "total_cost_uzrn": "1250000",
  "total_tokens": 125000
}
```

### 6.3 Dataset API

#### List Datasets

```
GET /v1/datasets?domain=science&min_quality=600000
Authorization: Bearer <api_key>

Response:
{
  "object": "list",
  "data": [
    {
      "id": "dataset_001",
      "name": "Science Explanations Gold+Silver",
      "description": "High-quality science explanations and tutorials",
      "domain": "science",
      "sample_count": 8920,
      "total_tokens": 4500000,
      "quality_distribution": {"gold": 3200, "silver": 5720},
      "filter": {
        "domain": "science",
        "sample_type": "EXPLANATION",
        "min_quality": 600000,
        "language": "en"
      },
      "pricing": {
        "price_per_sample_uzrn": "100000",
        "bulk_price_uzrn": "500000000",
        "bulk_discount_pct": 44
      },
      "license": "CC-BY-SA-4.0",
      "created_at": "2026-03-01T00:00:00Z"
    }
  ]
}
```

#### Purchase Dataset

```
POST /v1/datasets/purchase
Authorization: Bearer <api_key>
Content-Type: application/json

{
  "dataset_id": "dataset_001",
  "use_bulk_price": true
}

Response:
{
  "purchase_id": "purchase_abc123",
  "dataset_id": "dataset_001",
  "payment_uzrn": "500000000",
  "sample_count": 8920,
  "status": "processing",
  "download_ready_at": "2026-03-04T14:30:00Z",
  "download_url": "https://api.zerone.network/v1/datasets/download/purchase_abc123",
  "chunks": 45,
  "encryption": "AES-256-GCM",
  "settlement_tx": "D4E5F6..."
}
```

### 6.4 Internal gRPC Services

Services communicate internally via gRPC with protobuf. Key interfaces:

```protobuf
// Model Registry
service ModelRegistry {
  rpc RegisterModel(RegisterModelRequest) returns (RegisterModelResponse);
  rpc PromoteModel(PromoteModelRequest) returns (PromoteModelResponse);
  rpc GetActiveModel(GetActiveModelRequest) returns (Model);
  rpc ListModels(ListModelsRequest) returns (ListModelsResponse);
  rpc RollbackModel(RollbackModelRequest) returns (RollbackModelResponse);
}

// Metering Ledger
service MeteringLedger {
  rpc RecordUsage(RecordUsageRequest) returns (RecordUsageResponse);
  rpc GetBalance(GetBalanceRequest) returns (Balance);
  rpc ReserveBalance(ReserveBalanceRequest) returns (ReserveBalanceResponse);
  rpc ReleaseReservation(ReleaseReservationRequest) returns (ReleaseReservationResponse);
  rpc GetUnsettled(GetUnsettledRequest) returns (UnsettledBatch);
  rpc MarkSettled(MarkSettledRequest) returns (MarkSettledResponse);
  rpc CreditDeposit(CreditDepositRequest) returns (CreditDepositResponse);
}

// Dataset Exporter (events)
// Publishes to NATS subjects:
//   zerone.export.sample.new    — new sample exported
//   zerone.export.snapshot.ready — dataset snapshot created
//   zerone.export.watermark     — export watermark updated
```

### 6.5 Pricing

Token pricing derives from on-chain `x/billing` parameters:

```
price_per_token = access_fee_per_sample / avg_tokens_per_sample
                ≈ 100,000 uzrn / 500 tokens
                = 200 uzrn per token (0.0002 ZRN)
```

Adjustments from `x/billing` pricing model:
- Confidence: ±20% (high-confidence domains cost less)
- Freshness: +10% premium for recently added knowledge
- Volume discount: negotiable for high-volume accounts

Settlement converts token usage to on-chain payments at the current rate.

---

## 7. Appendix: On-Chain Data Model Reference

### Knowledge Module (x/knowledge)

**Core types consumed by the inference layer:**

| Type | Key Prefix | Fields Used by Exporter |
|------|-----------|------------------------|
| Sample | `0x01` | id, content, sample_type, domain, quality_score, quality_tier, thread_id, parent_sample_id, thread_position, language, tags, consent.type, fitness_score |
| Submission | `0x02` | id, content, sample_type, domain, status, content_hash, quality_round_id |
| QualityRound | `0x03` | id, submission_id, phase, verdict, aggregate_scores |
| Domain | `0x04` | name, description, status, stratum, parent_domain, depth, fact_count |
| Dataset | `0x05` | id, name, domain, filter_type, filter_language, filter_tags, min_quality, price_per_sample, bulk_price, sample_count |
| TrainingDemand | `0x06` | domain, subject, query_count, unfulfilled_count, bounty_pool |
| DataBounty | `0x07` | id, domain, subject, reward_amount, claimed |
| CompletedRound | `0x57` | verdict_block (index key), domain, duration_blocks |

**Indexes used for export queries:**

| Index | Key Prefix | Use |
|-------|-----------|-----|
| DomainSampleIndex | `0x0B` | Iterate all samples in a domain |
| ThreadIndex | `0x0A` | Reconstruct conversation threads |
| ContentHashIndex | `0x0E` | Deduplication |
| NicheIndex | `0x0D` | Domain/topic grouping |
| CompletedRoundIndex | `0x57` | Detect newly approved samples by block range |

**12 SampleTypes and their training format mapping:**

| SampleType | Training Format | Notes |
|------------|----------------|-------|
| Q_AND_A | Instruction pair | `{"instruction": Q, "output": A}` |
| TUTORIAL | Instruction pair | `{"instruction": "Teach: <topic>", "output": content}` |
| EXPLANATION | Instruction pair | `{"instruction": "Explain: <topic>", "output": content}` |
| TROUBLESHOOT | Instruction pair | `{"instruction": "Troubleshoot: <problem>", "output": content}` |
| DISCUSSION | Multi-turn conversation | Ordered by thread_position within thread_id |
| DEBATE | Multi-turn conversation | Ordered by thread_position, multiple perspectives |
| REVIEW | Preference pair | Content + quality_score as preference signal |
| OPINION | Preference pair | Content + quality_score as preference signal |
| ANNOTATION | Preference pair | Correction/annotation of existing content |
| NARRATIVE | Raw text | Direct content, high-quality prose |
| CREATIVE | Raw text | Creative writing samples |
| CORRECTION | Raw text | Error corrections (can pair with original for preference) |

**Quality tier thresholds (from module params):**

| Tier | Threshold (BPS) | Training Weight |
|------|----------------|----------------|
| Gold | ≥ 800,000 | 3× |
| Silver | ≥ 600,000 | 2× |
| Bronze | ≥ 400,000 | 1× |

**Consent multipliers (from module params):**

| Consent Type | Multiplier | Training Weight |
|-------------|-----------|----------------|
| SELF_AUTHORED | 1.5× (15000) | Highest quality signal |
| OPT_IN | 1.3× (13000) | Strong consent |
| PUBLIC_LICENSE | 1.0× (10000) | Baseline |
| PLATFORM_TOS | 0.8× (8000) | Weaker consent |
| FAIR_USE | 0.5× (5000) | Lowest weight |

### Billing Module (x/billing)

**Types relevant to the inference layer:**

| Type | Purpose |
|------|---------|
| Provider | Registered inference provider (address, domains, stake, revenue) |
| QueryQuote | Per-query pricing with confidence/novelty/freshness adjustments |
| PaymentDistribution | Revenue split: 55% provider, 22% protocol, 3.33% research, 19.67% dev |
| DynamicPricingConfig | Oracle-driven USD-pegged pricing with TWAP fallback |

**Revenue split (RevenueSplit, 1M BPS total):**
- `contributor_bps`: 550,000 (55%) → inference provider
- `protocol_bps`: 220,000 (22%) → protocol pools
- `research_bps`: 33,300 (3.33%) → research fund
- `development_bps`: 196,700 (19.67%) → development fund

### Compute Pool Module (x/compute_pool)

**Types relevant to the inference layer:**

| Type | Purpose |
|------|---------|
| ComputeProvider | Registered provider with service_type ("inference"/"verification"/"storage"), endpoint, price_per_cu, stake, SLA metrics |
| ComputeCredit | Earned credits redeemable 1:1 for uzrn |

**Provider lifecycle:** active → jailed (missed heartbeat) → active (heartbeat resumes) or unbonding → exited

**SLA requirements:**
- Uptime: ≥ 90% (`min_uptime_bps` = 900,000)
- Latency: ≤ 5000ms (`max_latency_ms`)
- Heartbeat: every 100 blocks (`heartbeat_interval_blocks`)

---

## 8. Blind Storage Protocol

The Blind Storage Protocol distributes encrypted training data across a
decentralized storage network so that no single node can reconstruct a useful
dataset, only buyers who pay sufficient ZRN can reassemble the data, and storage
nodes earn ZRN for honest hosting.

### 8.1 Overview

```
┌────────────────────────────────────────────────────────────────────────────┐
│                       BLIND STORAGE PROTOCOL                               │
│                                                                            │
│   Dataset               Chunking &              Distribution &             │
│   Snapshot              Encryption               Storage                   │
│                                                                            │
│  ┌──────────┐    ┌──────────────────┐    ┌──────────────────────────────┐  │
│  │ Approved  │    │  Reed-Solomon    │    │  VRF-Assigned Storage Nodes  │  │
│  │ Samples   │───▶│  Erasure Coding  │───▶│                              │  │
│  │ (JSONL)   │    │  (K data + M     │    │  Node A: [C1, C7, C15, ...]  │  │
│  │           │    │   parity chunks) │    │  Node B: [C2, C9, C11, ...]  │  │
│  └──────────┘    │                  │    │  Node C: [C3, C5, C18, ...]  │  │
│                  │  AES-256-GCM     │    │  ...                         │  │
│                  │  per-chunk keys   │    │  Node N: [C4, C12, C20, ...] │  │
│                  │  (HKDF-derived)   │    │                              │  │
│                  └──────────────────┘    └──────────────────────────────┘  │
│                                                                            │
│   Purchase &             Key Release             Reconstruction            │
│   Payment                                                                  │
│                                                                            │
│  ┌──────────┐    ┌──────────────────┐    ┌──────────────────────────────┐  │
│  │ Buyer     │    │  Shamir Secret   │    │  Buyer reassembles locally:  │  │
│  │ deposits  │───▶│  Sharing:        │───▶│                              │  │
│  │ ZRN       │    │  T-of-N key      │    │  1. Collect K+ chunks        │  │
│  │ on-chain  │    │  shares released │    │  2. Recover master key       │  │
│  └──────────┘    │  proportional to │    │  3. Derive chunk keys (HKDF) │  │
│                  │  payment amount  │    │  4. Decrypt all chunks       │  │
│                  └──────────────────┘    │  5. RS-decode → full dataset │  │
│                                          └──────────────────────────────┘  │
└────────────────────────────────────────────────────────────────────────────┘
```

**Design principles:**
- **No single point of trust** — no node, operator, or key-holder can
  unilaterally access the dataset
- **Threshold economics** — partial payment yields useless fragments;
  full payment unlocks reconstruction
- **Cryptographic separation** — storage nodes hold ciphertext only;
  key shares are managed independently
- **Resilience** — erasure coding tolerates node failures without data loss

---

### 8.2 Chunking Strategy

#### 8.2.1 Reed-Solomon Erasure Coding

The protocol uses Reed-Solomon (RS) erasure coding to split dataset snapshots
into chunks. RS coding provides two critical properties: (1) any K of K+M
total chunks suffice to reconstruct the full dataset, and (2) fewer than K
chunks are computationally useless for reconstruction.

```
Dataset Snapshot (e.g., 500 MB JSONL)
         │
         ▼
┌─────────────────────────────────┐
│  Reed-Solomon Encoder           │
│                                 │
│  Input:  500 MB contiguous data │
│  K = 16  (data chunks)         │
│  M = 8   (parity chunks)       │
│  Total = 24 chunks             │
│                                 │
│  Each chunk ≈ 31.25 MB         │
│  (500 MB / 16 data chunks)     │
│                                 │
│  Any 16 of 24 → full recovery  │
│  Fewer than 16 → nothing       │
└─────────────────────────────────┘
         │
         ▼
  C₀  C₁  C₂ ... C₁₅  P₀  P₁ ... P₇
  ─── data chunks ───  ── parity ──
```

**Parameters (configurable per dataset version):**

| Parameter | Default | Range | Description |
|-----------|---------|-------|-------------|
| `K` (data chunks) | 16 | 4–64 | Number of data chunks. Higher = more granular distribution |
| `M` (parity chunks) | 8 | 2–32 | Redundancy chunks. Higher = more fault tolerance |
| `chunk_target_size` | 4 MB | 1–10 MB | Target chunk size; K is adjusted to match |
| `rs_field` | GF(2^8) | — | Galois field for RS encoding |

**Chunk sizing algorithm:**

```
Given:
  dataset_size = size of JSONL snapshot in bytes
  chunk_target = 4 MB (configurable)

Compute:
  K = ceil(dataset_size / chunk_target)
  K = clamp(K, 4, 64)
  M = ceil(K / 2)
  M = clamp(M, 2, 32)
  actual_chunk_size = ceil(dataset_size / K)
```

For typical datasets:

| Dataset Size | K | M | Total Chunks | Chunk Size | Fault Tolerance |
|-------------|---|---|-------------|------------|-----------------|
| 50 MB | 13 | 7 | 20 | ~3.8 MB | 7 node failures |
| 500 MB | 64 | 32 | 96 | ~7.8 MB | 32 node failures |
| 5 GB | 64 | 32 | 96 | ~80 MB | 32 node failures |

#### 8.2.2 Cross-Sample Interleaving

Before RS encoding, the raw JSONL is preprocessed to prevent individual chunks
from containing complete, independently valuable samples:

1. **Serialize** all samples into a contiguous byte stream (JSONL, newline-delimited)
2. **Shuffle** the byte stream with a deterministic permutation seeded by
   `SHA-256(dataset_version_id)` — this ensures samples are scattered across
   the byte stream before chunking
3. **RS-encode** the shuffled stream into K+M chunks

This means even a single data chunk contains interleaved byte fragments from
many different samples, rendering it useless without the full reconstruction
threshold.

#### 8.2.3 Dataset Versioning

Each dataset snapshot produces a new set of chunks. The on-chain `Dataset`
record (prefix `0x05`) is extended with:

| Field | Type | Description |
|-------|------|-------------|
| `version` | uint64 | Monotonically increasing version number |
| `chunk_merkle_root` | bytes | Root of Merkle tree over all chunk hashes |
| `k_threshold` | uint32 | Data chunk count (RS K parameter) |
| `m_parity` | uint32 | Parity chunk count (RS M parameter) |
| `total_chunks` | uint32 | K + M |
| `chunk_size_bytes` | uint64 | Size of each chunk (last may be shorter) |
| `snapshot_block` | int64 | Block height at which snapshot was taken |

Old versions are retained for a configurable retention period
(`dataset_version_retention = 30 days`). Storage nodes may prune expired
versions after on-chain confirmation.

---

### 8.3 Encryption Scheme

#### 8.3.1 Key Hierarchy

```
                    ┌─────────────────────────┐
                    │  Master Dataset Key      │
                    │  (256-bit, per version)   │
                    │                           │
                    │  Generated: CSPRNG        │
                    │  Stored: NEVER in full    │
                    │  Split: Shamir T-of-N     │
                    └────────────┬──────────────┘
                                 │
                    ┌────────────┴──────────────┐
                    │  HKDF-SHA256 derivation    │
                    │  salt = dataset_version_id │
                    └────────────┬──────────────┘
                                 │
           ┌─────────┬──────────┼──────────┬─────────┐
           ▼         ▼          ▼          ▼         ▼
        ┌─────┐  ┌─────┐   ┌─────┐   ┌─────┐  ┌─────┐
        │ CK₀ │  │ CK₁ │   │ CK₂ │   │ ... │  │CKₙ₋₁│
        └─────┘  └─────┘   └─────┘   └─────┘  └─────┘
        Chunk    Chunk      Chunk               Chunk
        Key 0    Key 1      Key 2               Key N-1

   CKᵢ = HKDF-Expand(master_key, "zerone-bsp-chunk" || i || version_id, 32)
```

**Key derivation:**

```
master_key       = random(32 bytes)                       // CSPRNG
salt             = SHA-256(dataset_version_id)
prk              = HKDF-Extract(salt, master_key)         // RFC 5869
chunk_key[i]     = HKDF-Expand(prk, "zerone-bsp-chunk" || uint32_be(i) || version_id, 32)
chunk_nonce[i]   = HKDF-Expand(prk, "zerone-bsp-nonce" || uint32_be(i) || version_id, 12)
```

Each chunk is encrypted independently:

```
encrypted_chunk[i] = AES-256-GCM.Encrypt(
    key   = chunk_key[i],
    nonce = chunk_nonce[i],
    plaintext = raw_chunk[i],
    aad   = chunk_metadata[i]   // chunk_index || dataset_version || merkle_path
)
```

The `aad` (additional authenticated data) binds each ciphertext to its
metadata, preventing chunk substitution attacks.

#### 8.3.2 Shamir's Secret Sharing

The master dataset key is split using Shamir's Secret Sharing (SSS) over
GF(2^256):

```
master_key → SSS.Split(master_key, T, N)
           → [share₀, share₁, ..., shareₙ₋₁]

Where:
  T = reconstruction threshold (e.g., T = 12 of N = 20)
  N = total key shares
  Any T shares → recover master_key
  Fewer than T shares → information-theoretically secure
```

**Share distribution:**

Shares are held by **Key Custodians** — a distinct role from storage nodes.
Key custodians are validators or specially staked entities registered on-chain
via `x/compute_pool` with `service_type = "key_custodian"`.

| Parameter | Default | Description |
|-----------|---------|-------------|
| `sss_total_shares` (N) | 20 | Total key shares generated |
| `sss_threshold` (T) | 12 | Shares needed to recover master key |
| `custodian_min_stake` | 10,000 ZRN | Minimum stake to hold key shares |
| `shares_per_custodian` | 1 | Each custodian holds exactly 1 share |

**Why separate custodians from storage nodes:**
- Storage nodes hold encrypted ciphertext — they cannot decrypt without keys
- Key custodians hold key shares — they cannot access data without ciphertext
- An attacker must compromise both T custodians AND enough storage nodes to
  collect K chunks — this is a much harder attack than compromising either
  group alone

#### 8.3.3 Payment-Gated Key Release

When a buyer pays for dataset access, key shares are released proportionally:

```
payment_fraction = payment_amount / dataset_full_price
shares_released  = floor(payment_fraction × N)

If shares_released >= T:
    buyer can reconstruct master_key
    → derive all chunk keys via HKDF
    → decrypt all chunks
    → RS-decode to reconstruct dataset
Else:
    buyer holds < T shares
    → cannot recover master_key
    → cannot decrypt any chunks
    → payment is wasted (partial purchases are non-refundable)
```

The minimum viable payment to reach threshold:

```
min_payment = ceil(T / N) × dataset_full_price
            = ceil(12 / 20) × dataset_full_price
            = 0.60 × dataset_full_price
```

This provides a **40% discount ceiling** — buyers must pay at least 60% of
the full price to reconstruct. The `bulk_price` in the on-chain Dataset record
should be set at or above this minimum.

**Share release protocol:**

1. Buyer submits `MsgAccessDataset` on-chain with payment
2. Chain records `DatasetAccess` entry: `{buyer, dataset_id, version, payment, shares_entitled}`
3. Buyer contacts key custodians with on-chain proof of payment
4. Each custodian verifies the `DatasetAccess` record on-chain
5. If `shares_entitled >= 1` for this custodian's index, custodian releases
   their share encrypted to the buyer's public key (ECIES with buyer's
   cosmos secp256k1 key)
6. Buyer collects T+ shares, reconstructs master key locally

---

### 8.4 Distribution Protocol

#### 8.4.1 Storage Node Registration

Storage nodes register on-chain via `x/compute_pool` with
`service_type = "storage"`:

```
MsgRegisterProvider {
    address:      "zerone1storage...",
    service_type: "storage",
    endpoint:     "https://storage-node-1.example.com:9443",
    stake:        100000000000,  // 100,000 ZRN minimum
    capacity_gb:  500,           // advertised storage capacity
    regions:      ["us-east", "eu-west"],  // geographic regions
}
```

**Registration requirements:**

| Requirement | Value | Rationale |
|-------------|-------|-----------|
| Minimum stake | 100,000 ZRN | Sybil resistance; slashable collateral |
| Capacity proof | Pass initial challenge | Must serve a test chunk within SLA |
| Heartbeat | Every 100 blocks | Liveness; missed → jailed |
| Bandwidth SLA | ≥ 50 Mbps sustained | Buyers need reasonable download speed |
| Uptime SLA | ≥ 95% | Higher than inference (90%) due to data criticality |

#### 8.4.2 VRF-Based Chunk Assignment

Chunks are assigned to storage nodes using the knowledge module's existing VRF
infrastructure, ensuring unpredictable, verifiable, and deterministic assignment:

```
For each chunk i in dataset version v:
    seed = VRF.Prove(validator_key, dataset_version_id || uint32_be(i))
    assignment = DeterministicShuffle(active_storage_nodes, seed)
    replicas[i] = assignment[0:R]  // first R nodes in shuffled order
```

**Replication factor (R):**

| Parameter | Default | Description |
|-----------|---------|-------------|
| `replication_factor` | 3 | Each chunk stored on R nodes |
| `min_region_diversity` | 2 | Replicas must span ≥ 2 geographic regions |
| `max_chunks_per_node` | 500 | Prevent overloading a single node |

**Assignment constraints:**
- No two replicas of the same chunk on the same node
- Geographic diversity: replicas spread across regions
- Capacity-aware: nodes not assigned beyond their `capacity_gb`
- Stake-weighted: higher-stake nodes receive proportionally more chunks
  (incentivizes staking, provides better economic security)

**Reassignment triggers:**
- Node exits or is jailed → chunks reassigned to next node in VRF shuffle
- New node joins → gradual rebalancing over `rebalance_period` (1000 blocks)
- Chunk audit failure → immediate reassignment + slashing

#### 8.4.3 Chunk Upload Flow

```
┌───────────┐    ┌─────────────┐    ┌──────────┐    ┌──────────────┐
│  Dataset   │    │  BSP         │    │  ZERONE  │    │  Storage     │
│  Exporter  │    │  Coordinator │    │  Chain   │    │  Nodes       │
└─────┬─────┘    └──────┬──────┘    └────┬─────┘    └──────┬───────┘
      │                 │                │                  │
      │  new snapshot   │                │                  │
      │  ready (NATS)   │                │                  │
      ├────────────────▶│                │                  │
      │                 │                │                  │
      │                 │  query active  │                  │
      │                 │  storage nodes │                  │
      │                 ├───────────────▶│                  │
      │                 │  node list     │                  │
      │                 │◀───────────────┤                  │
      │                 │                │                  │
      │                 │  RS-encode +   │                  │
      │                 │  encrypt +     │                  │
      │                 │  compute VRF   │                  │
      │                 │  assignments   │                  │
      │                 │  ───────────   │                  │
      │                 │                │                  │
      │                 │  register      │                  │
      │                 │  chunk manifest│                  │
      │                 ├───────────────▶│                  │
      │                 │  (merkle root, │                  │
      │                 │   assignments) │                  │
      │                 │                │                  │
      │                 │  upload chunks │                  │
      │                 │  to assigned   │                  │
      │                 │  nodes (gRPC)  │                  │
      │                 ├─────────────────────────────────▶│
      │                 │                │                  │
      │                 │                │  confirm receipt │
      │                 │                │◀─────────────────┤
      │                 │                │  (signed hash)   │
      │                 │                │                  │
```

The **BSP Coordinator** is a service within the inference layer (runs alongside
the Dataset Exporter). It orchestrates chunk creation and upload but does not
retain decrypted data or key material after upload completes.

#### 8.4.4 Proof of Storage

Storage nodes must periodically prove they still hold their assigned chunks.
The protocol uses a challenge-response mechanism:

```
┌───────────┐              ┌──────────────┐
│  On-Chain  │              │  Storage     │
│  Verifier  │              │  Node        │
└─────┬─────┘              └──────┬───────┘
      │                           │
      │  Challenge:               │
      │  chunk_id = C₇            │
      │  offset = 1048576         │
      │  length = 4096            │
      │  nonce = 0xABCD...        │
      ├──────────────────────────▶│
      │                           │
      │                           │  Read 4096 bytes
      │                           │  at offset 1048576
      │                           │  from chunk C₇
      │                           │  ───────────
      │                           │
      │  Response:                │
      │  data_hash = SHA-256(     │
      │    nonce || chunk_data    │
      │    [offset:offset+length])│
      │  merkle_proof = [...]     │
      │◀──────────────────────────┤
      │                           │
      │  Verify:                  │
      │  1. Check merkle_proof    │
      │     against on-chain root │
      │  2. Check data_hash       │
      │     against expected      │
      │  ───────────              │
      │                           │
```

**Challenge parameters:**

| Parameter | Default | Description |
|-----------|---------|-------------|
| `challenge_interval` | 200 blocks | How often challenges are issued |
| `challenge_batch_size` | 5 | Chunks challenged per node per interval |
| `response_timeout` | 20 blocks | Blocks allowed to respond |
| `challenge_byte_range` | 4096 | Bytes to prove per challenge |

**Verification:**

Challenges are generated deterministically from block hash + node address,
so no centralized challenger is needed. Any full node can verify responses.

```
challenge_seed = SHA-256(block_hash || node_address || challenge_epoch)
chunk_index    = challenge_seed[0:4] mod num_assigned_chunks(node)
byte_offset    = challenge_seed[4:8] mod (chunk_size - challenge_byte_range)
```

The node must respond with:
1. The raw bytes at the challenged range
2. A Merkle proof linking those bytes to the chunk's Merkle root (stored
   on-chain in the chunk manifest)

**Failure consequences:**
- 1 missed challenge: warning, no penalty
- 2 consecutive misses: jailed (50 blocks), chunk reassigned to backup node
- 3 consecutive misses: slashed 1% of stake, chunk permanently reassigned
- Serving incorrect data (bad proof): slashed 5% of stake, jailed 500 blocks

---

### 8.5 Access Control & Payment

#### 8.5.1 Two-Tier Access Model

```
┌───────────────────────────────────────────────────────┐
│  Tier 1: API Inference (Low Tier)                      │
│                                                        │
│  • Pay per API call in ZRN                             │
│  • Never touch raw training data                       │
│  • OpenAI-compatible /v1/chat/completions              │
│  • Cost: ~200 uzrn per token                           │
│  • Use case: applications, chatbots, assistants        │
│                                                        │
│  Access: API key + ZRN balance                         │
│  Protection: model serves inference, data stays hidden │
└───────────────────────────────────────────────────────┘

┌───────────────────────────────────────────────────────┐
│  Tier 2: Dataset Access (High Tier)                    │
│                                                        │
│  • Pay lump sum in ZRN to unlock dataset chunks        │
│  • Download encrypted chunks from storage nodes        │
│  • Reconstruct full dataset locally                    │
│  • Cost: bulk_price (e.g., 500 ZRN for 8920 samples)  │
│  • Use case: fine-tuning own models, research          │
│                                                        │
│  Access: on-chain payment + key share collection       │
│  Protection: threshold encryption + erasure coding     │
└───────────────────────────────────────────────────────┘
```

#### 8.5.2 Dataset Purchase Flow

```
┌──────────┐    ┌──────────────┐    ┌──────────┐    ┌─────────┐    ┌──────────┐
│  Buyer   │    │  API Gateway │    │  ZERONE  │    │  Key    │    │  Storage │
│          │    │              │    │  Chain   │    │ Custo-  │    │  Nodes   │
│          │    │              │    │          │    │  dians  │    │          │
└────┬─────┘    └──────┬───────┘    └────┬─────┘    └────┬────┘    └────┬─────┘
     │                 │                 │               │              │
     │  ① Browse       │                 │               │              │
     │  datasets       │                 │               │              │
     │  GET /v1/       │                 │               │              │
     │  datasets       │                 │               │              │
     ├────────────────▶│  query chain    │               │              │
     │                 ├────────────────▶│               │              │
     │  dataset list   │◀────────────────┤               │              │
     │◀────────────────┤                 │               │              │
     │                 │                 │               │              │
     │  ② Purchase     │                 │               │              │
     │  POST /v1/      │                 │               │              │
     │  datasets/      │                 │               │              │
     │  purchase       │                 │               │              │
     ├────────────────▶│                 │               │              │
     │                 │  MsgAccessDataset               │              │
     │                 ├────────────────▶│               │              │
     │                 │                 │               │              │
     │                 │                 │  debit buyer  │              │
     │                 │                 │  record access│              │
     │                 │                 │  compute      │              │
     │                 │                 │  shares_due   │              │
     │                 │                 │  ──────────   │              │
     │                 │                 │               │              │
     │                 │  access_ticket  │               │              │
     │  purchase_id +  │◀────────────────┤               │              │
     │  access_ticket  │                 │               │              │
     │◀────────────────┤                 │               │              │
     │                 │                 │               │              │
     │  ③ Collect key shares            │               │              │
     │  (direct P2P, authenticated)      │               │              │
     ├──────────────────────────────────────────────────▶│              │
     │                 │                 │               │              │
     │                 │                 │  verify       │              │
     │                 │                 │  DatasetAccess│              │
     │                 │                 │◀──────────────┤              │
     │                 │                 ├──────────────▶│              │
     │                 │                 │               │              │
     │  ECIES-encrypted share           │               │              │
     │◀──────────────────────────────────────────────────┤              │
     │  (repeat for T custodians)       │               │              │
     │                 │                 │               │              │
     │  ④ Reconstruct master key locally│               │              │
     │  SSS.Combine(shares[0:T])        │               │              │
     │  ───────────    │                 │               │              │
     │                 │                 │               │              │
     │  ⑤ Download encrypted chunks     │               │              │
     │  (present access_ticket)         │               │              │
     ├──────────────────────────────────────────────────────────────────▶
     │                 │                 │               │              │
     │                 │                 │  verify       │              │
     │                 │                 │  access_ticket│              │
     │                 │                 │◀──────────────────────────────┤
     │                 │                 ├──────────────────────────────▶│
     │                 │                 │               │              │
     │  encrypted chunks (parallel download from R replicas)           │
     │◀──────────────────────────────────────────────────────────────────┤
     │                 │                 │               │              │
     │  ⑥ Local reconstruction          │               │              │
     │  - Derive chunk keys (HKDF)      │               │              │
     │  - Decrypt each chunk            │               │              │
     │  - RS-decode K chunks → dataset  │               │              │
     │  - Reverse shuffle permutation   │               │              │
     │  - Verify content hash           │               │              │
     │  ───────────    │                 │               │              │
     │                 │                 │               │              │
```

#### 8.5.3 Access Ticket

The on-chain `DatasetAccess` record serves as the access ticket:

```
DatasetAccess {
    buyer:            "zerone1buyer...",
    dataset_id:       "dataset_001",
    version:          3,
    payment_uzrn:     500000000000,       // 500 ZRN
    shares_entitled:  20,                 // all shares (full price paid)
    access_ticket_id: SHA-256(buyer || dataset_id || version || block_hash),
    granted_block:    1523400,
    expires_block:    1823400,            // 300,000 blocks ≈ 21 days
    status:           ACTIVE,
}
```

Storage nodes verify access tickets by querying the chain (or a trusted
cache) for the `DatasetAccess` record. The ticket is valid if:

1. `status == ACTIVE`
2. `current_block < expires_block`
3. Buyer's signature matches the `buyer` address
4. Requested chunk belongs to the specified `dataset_id` and `version`

#### 8.5.4 Partial Purchase Prevention

Below the threshold T, the buyer cannot reconstruct the master key or decrypt
any chunks. The protocol intentionally prevents partial utility:

- **Erasure coding** — fewer than K plaintext chunks cannot reconstruct the
  dataset even if somehow decrypted
- **Shamir threshold** — fewer than T key shares reveal zero information
  about the master key (information-theoretic security)
- **Cross-sample interleaving** — even a single decrypted chunk contains
  scrambled byte fragments from many samples, not complete records
- **No partial refunds** — on-chain `DatasetAccess` is non-refundable to
  prevent abuse (pay, copy shares, refund)

The pricing should be set so that `bulk_price >= (T/N) × full_value` to
ensure the threshold is economically meaningful.

---

### 8.6 Incentive Structure

#### 8.6.1 Storage Rewards

Storage nodes earn ZRN from three sources:

```
reward_per_epoch(node) = base_storage_reward
                       + serving_reward
                       + challenge_bonus

Where:
  base_storage_reward = chunks_held × chunk_size × rate_per_byte_per_block
  serving_reward      = chunks_served × serving_fee_per_chunk
  challenge_bonus     = challenges_passed × challenge_reward
```

**Reward parameters:**

| Parameter | Default | Description |
|-----------|---------|-------------|
| `storage_rate_per_byte_block` | 1 uzrn | Per byte per 100 blocks |
| `serving_fee_per_chunk` | 10,000 uzrn | Per chunk served to a buyer |
| `challenge_reward` | 5,000 uzrn | Per proof-of-storage challenge passed |
| `reward_epoch` | 100 blocks | How often base rewards are distributed |

**Revenue split on dataset purchases:**

When a buyer purchases dataset access, the payment is distributed:

| Recipient | Share | Description |
|-----------|-------|-------------|
| Data submitters | 55% | Split among original sample submitters, weighted by quality tier and consent multiplier |
| Storage nodes | 15% | Split among nodes holding chunks of this dataset version |
| Key custodians | 5% | Split among custodians who released shares |
| Protocol pools | 15% | Knowledge pool + verification + treasury |
| Research fund | 3.33% | Long-term research |
| Development fund | 6.67% | Infrastructure development |

Storage node revenue from a dataset purchase:

```
node_share = (purchase_payment × 0.15) × (node_chunks / total_chunks)
```

#### 8.6.2 Slashing Conditions

| Violation | Slash Amount | Jail Duration | Additional |
|-----------|-------------|---------------|------------|
| Proof-of-storage failure (3 consecutive) | 1% of stake | 500 blocks | Chunks reassigned |
| Serving without valid ticket | 5% of stake | 1000 blocks | Reported by buyer or auditor |
| Serving corrupted data | 5% of stake | 1000 blocks | Bad Merkle proof |
| Extended downtime (>1000 blocks) | 0.5% of stake | Until heartbeat resumes | Chunks reassigned to backups |
| Collusion (proven) | 100% of stake | Permanent ban | Requires governance action |

**Slash detection:**

- Proof-of-storage failures: automated on-chain challenge-response
- Serving without ticket: buyer includes proof of unauthorized access in
  `MsgReportStorageViolation`
- Corrupted data: buyer submits chunk hash + Merkle proof showing mismatch
  with on-chain root

#### 8.6.3 Bootstrap Incentives

Before paying customers exist, storage nodes are incentivized via:

1. **Inflation rewards** — a portion of block inflation directed to storage
   providers during the bootstrap phase (first 480,000 blocks, matching the
   gas-free bootstrap period)
2. **Reduced stake requirement** — 10,000 ZRN during bootstrap (vs. 100,000
   at maturity)
3. **Guaranteed challenge rewards** — challenge rewards paid from protocol
   treasury during bootstrap, regardless of buyer activity

---

### 8.7 Threat Model

#### 8.7.1 Sybil Storage Nodes

**Attack:** One entity registers many storage nodes to accumulate all chunks
of a dataset.

**Mitigations:**
- **Stake requirement** — 100,000 ZRN per node makes Sybil attacks expensive
  (20 nodes = 2M ZRN at risk)
- **VRF assignment** — chunk assignment is deterministic and unpredictable;
  attacker cannot influence which chunks go to their nodes
- **Geographic diversity scoring** — replicas must span ≥ 2 regions;
  nodes in the same region/AS number get lower assignment priority
- **Capacity-proportional assignment** — nodes must prove storage capacity
  before receiving chunk assignments
- **Separation of concerns** — even if an attacker controls all storage nodes,
  they only hold ciphertext; key shares are with custodians

**Residual risk:** An attacker controlling all storage nodes AND T custodians
could reconstruct the dataset. This requires compromising 20+ entities with
significant combined stake (>2M ZRN storage + 200K ZRN custodian stakes).

#### 8.7.2 Collusion (Storage Nodes Share Chunks)

**Attack:** Storage nodes share their chunks with each other to reconstruct
the dataset without paying.

**Mitigations:**
- **Chunks are encrypted** — nodes hold ciphertext only; sharing ciphertext
  among colluding storage nodes gains nothing without key shares
- **Key shares are with custodians** — a completely separate group with their
  own staking and incentives
- **No cross-role** — a single entity cannot be both a storage node and a key
  custodian (enforced on-chain by address blacklist per role)

#### 8.7.3 Free-Riding (Access Without Payment)

**Attack:** Someone obtains chunks without paying by spoofing access tickets
or bypassing verification.

**Mitigations:**
- **On-chain access tickets** — `DatasetAccess` is an on-chain record;
  storage nodes verify it against chain state (cannot be forged)
- **Buyer signature verification** — chunk requests must be signed by the
  buyer's private key matching the `DatasetAccess.buyer` address
- **Serving audit** — storage nodes log all chunk serves with buyer signatures;
  random audits compare serve logs against on-chain access records
- **Whistleblower reward** — anyone proving unauthorized serving earns 50%
  of the slash amount

#### 8.7.4 Data Poisoning

**Attack:** Malicious storage node serves corrupted chunks to a buyer.

**Mitigations:**
- **Merkle root on-chain** — each chunk's SHA-256 hash is a leaf in a Merkle
  tree whose root is stored on-chain in the chunk manifest
- **Buyer-side verification** — buyer computes hash of each downloaded chunk
  and verifies against the Merkle proof before attempting decryption
- **Redundant downloads** — buyer can request the same chunk from multiple
  replicas (R=3); if one is corrupted, try another
- **Slash on proof** — buyer submits `MsgReportCorruptedChunk` with the bad
  data hash + expected hash from Merkle tree → node is slashed

#### 8.7.5 Threshold Gaming

**Attack:** Buyer pays just below threshold, hoping to infer missing data from
partial chunks.

**Mitigations:**
- **RS coding is all-or-nothing below K** — with fewer than K chunks, the
  mathematical reconstruction is impossible (not just hard — the system of
  equations is underdetermined)
- **Shamir's is information-theoretically secure** — T-1 shares reveal zero
  bits of the master key; this is proven, not an assumption
- **Cross-sample interleaving** — even if a buyer somehow decrypted a partial
  chunk, the byte-level shuffle means it contains fragments of many samples
  interleaved, not readable records
- **Economic deterrent** — partial payment is non-refundable; paying 59% and
  getting nothing is a strong disincentive

#### 8.7.6 Key Custodian Compromise

**Attack:** Attacker compromises T key custodians to recover the master key.

**Mitigations:**
- **High custodian stake** — 10,000 ZRN per custodian; compromising T=12
  requires 120,000 ZRN at risk
- **Custodian rotation** — key shares are regenerated on a configurable
  schedule (`key_rotation_interval = 10,000 blocks`); old shares become
  invalid for new dataset versions
- **HSM requirement** — custodians are encouraged (and may be required by
  governance) to store shares in hardware security modules
- **Monitoring** — unusual share release patterns (many releases without
  corresponding on-chain payments) trigger alerts and custodian jailing

#### 8.7.7 Coordinator Compromise

**Attack:** The BSP Coordinator is compromised, leaking the master key during
chunk encryption.

**Mitigations:**
- **Ephemeral key material** — the coordinator generates the master key,
  splits it via SSS, distributes shares, then securely erases the master key
  from memory; the key exists in the coordinator for seconds, not permanently
- **Attestation** — in production, the coordinator runs in a TEE (Trusted
  Execution Environment, e.g., Intel SGX / AMD SEV) to prevent key extraction
  even if the host is compromised
- **Audit log** — all coordinator actions are logged and signed; anomalous
  behavior (e.g., encrypting the same dataset twice) triggers alerts

---

### 8.8 Worked Example

**Scenario:** A science dataset with 8,920 approved samples (500 MB JSONL)
is chunked, distributed, purchased, and reconstructed.

#### Step 1: Dataset Snapshot

```
Dataset: "Science Explanations Gold+Silver"
  ID:           dataset_001
  Domain:       science
  Samples:      8,920
  Size:         500 MB (JSONL)
  Quality:      3,200 gold + 5,720 silver
  Version:      3
  Snapshot at:  block 1,500,000
```

#### Step 2: Chunking (RS Erasure Coding)

```
Input:  500 MB JSONL
Target chunk size: 4 MB

K = ceil(500 MB / 4 MB) = 125 → clamped to 64 (max)
M = ceil(64 / 2) = 32
Total chunks = 64 + 32 = 96
Actual chunk size = 500 MB / 64 ≈ 7.8 MB each

Before RS encoding:
  1. Serialize 8,920 samples → 500 MB contiguous byte stream
  2. Shuffle with seed = SHA-256("dataset_001_v3") → interleaved bytes
  3. RS-encode → 64 data chunks + 32 parity chunks = 96 chunks
  4. Total stored data = 96 × 7.8 MB ≈ 750 MB (1.5× overhead for parity)
```

#### Step 3: Encryption

```
master_key = random(32 bytes)  // e.g., 0xA3F1...9B2E

For each chunk i in 0..95:
  chunk_key[i] = HKDF(master_key, "zerone-bsp-chunk" || i || "dataset_001_v3")
  chunk_nonce[i] = HKDF(master_key, "zerone-bsp-nonce" || i || "dataset_001_v3")
  encrypted[i] = AES-256-GCM(chunk_key[i], chunk_nonce[i], raw_chunk[i])

Shamir split:
  master_key → SSS.Split(key, T=12, N=20)
  → 20 key shares distributed to 20 key custodians

Master key securely erased from coordinator memory.
```

#### Step 4: Distribution

```
Active storage nodes: 30 nodes (each staked ≥ 100,000 ZRN)
Replication factor: R = 3
Total chunk-replicas: 96 × 3 = 288 assignments

VRF assignment (example):
  Chunk C₀  → Node 7, Node 19, Node 2   (3 replicas, ≥ 2 regions)
  Chunk C₁  → Node 3, Node 22, Node 11
  Chunk C₂  → Node 15, Node 8, Node 27
  ...
  Chunk C₉₅ → Node 1, Node 14, Node 25

Each node holds: 288 / 30 ≈ 9-10 chunk-replicas
Storage per node: 9.6 × 7.8 MB ≈ 75 MB
Total network storage: 750 MB × 3 = 2.25 GB

On-chain chunk manifest:
  merkle_root = MerkleRoot([SHA-256(encrypted[0]), ..., SHA-256(encrypted[95])])
  Stored in Dataset record with K=64, M=32, chunk_size=7.8 MB
```

#### Step 5: Purchase

```
Buyer: zerone1buyer...abc
Dataset: dataset_001, version 3
Bulk price: 500 ZRN (500,000,000,000 uzrn)

On-chain:
  MsgAccessDataset {
    buyer: "zerone1buyer...abc",
    dataset_id: "dataset_001",
    version: 3,
    payment: 500000000000 uzrn
  }

Result:
  DatasetAccess {
    buyer: "zerone1buyer...abc",
    shares_entitled: 20  (full price → all shares)
    access_ticket_id: 0x7F3A...
    expires_block: 1800000
  }

Revenue distribution (500 ZRN):
  Data submitters:  275.0 ZRN (55%)  → split among 8,920 sample submitters
  Storage nodes:     75.0 ZRN (15%)  → split among 30 nodes proportional to chunks held
  Key custodians:    25.0 ZRN (5%)   → split among 20 custodians
  Protocol pools:    75.0 ZRN (15%)
  Research fund:     16.65 ZRN (3.33%)
  Development fund:  33.35 ZRN (6.67%)
```

#### Step 6: Key Share Collection

```
Buyer contacts 20 key custodians (endpoints registered on-chain):

For each custodian c in 0..19:
  1. Buyer sends: {access_ticket_id, buyer_pubkey, signature}
  2. Custodian verifies on-chain: DatasetAccess exists, shares_entitled ≥ 1
  3. Custodian encrypts their share to buyer's public key:
     encrypted_share[c] = ECIES.Encrypt(buyer_pubkey, share[c])
  4. Returns encrypted_share[c] to buyer

Buyer decrypts all 20 shares with their private key.
Buyer runs: master_key = SSS.Combine(shares[0..19])
Verifies: master_key produces the expected Merkle root when deriving chunk keys.
```

#### Step 7: Chunk Download

```
Buyer needs any 64 of 96 chunks (K=64).
Requests chunks in parallel from storage nodes:

For each chunk i in 0..95 (stop after 64 successful downloads):
  1. Select fastest replica from assignment list
  2. Send: {chunk_id: i, access_ticket_id, buyer_signature}
  3. Storage node verifies ticket on-chain
  4. Receives encrypted_chunk[i]
  5. Verify: SHA-256(encrypted_chunk[i]) matches Merkle proof
  6. If corrupted, try next replica

Download speed: 96 chunks × 7.8 MB = 750 MB total
At 50 Mbps per node, 10 parallel downloads: ~12 seconds
```

#### Step 8: Local Reconstruction

```
1. Derive chunk keys:
   For each chunk i:
     chunk_key[i] = HKDF(master_key, "zerone-bsp-chunk" || i || "dataset_001_v3")

2. Decrypt chunks:
   For each downloaded chunk i:
     plaintext[i] = AES-256-GCM.Decrypt(chunk_key[i], chunk_nonce[i], encrypted[i])

3. RS-decode:
   dataset_shuffled = ReedSolomon.Decode(plaintext[0..63], K=64)
   // Any 64 of 96 chunks suffice

4. Reverse shuffle:
   dataset = InverseShuffle(dataset_shuffled, seed=SHA-256("dataset_001_v3"))

5. Verify:
   assert SHA-256(dataset) == expected_hash  // from on-chain Dataset record

Result: 500 MB JSONL with 8,920 science samples, ready for fine-tuning.
```

#### End-to-End Security Summary

```
┌──────────────────────┬──────────────────────────────────────────────┐
│ What the attacker     │ What they'd need to break it                 │
│ controls              │                                              │
├──────────────────────┼──────────────────────────────────────────────┤
│ 1 storage node        │ Nothing — holds ciphertext only              │
│ All 30 storage nodes  │ Still nothing — no key shares                │
│ 11 key custodians     │ Nothing — below threshold T=12               │
│ 12+ key custodians    │ Can reconstruct master key, but need chunks  │
│                       │ from storage nodes (who verify tickets)      │
│ 12 custodians +       │ Full access — but requires 120K ZRN custo-  │
│ valid access ticket   │ dian stake + 500 ZRN purchase (legitimate)   │
│ 12 custodians +       │ Full access — costs >3M ZRN in colluding    │
│ 30 storage nodes      │ stakes; all are slashable if detected        │
└──────────────────────┴──────────────────────────────────────────────┘
```

---

### 8.9 Implementation Considerations

#### 8.9.1 Storage Backend

The protocol uses a **custom P2P network** rather than IPFS or Arweave:

- **IPFS** — content-addressed, but no payment gating; anyone who knows the
  CID can retrieve data. Not suitable for monetized access.
- **Arweave** — permanent storage, but immutable and publicly accessible by
  design. Cannot enforce access control.
- **Custom P2P** — storage nodes are registered on-chain, serve chunks only
  with valid access tickets, and are economically incentivized via staking
  and rewards. Full control over access policy.

Storage nodes use S3-compatible local storage (MinIO) for chunk persistence,
with the gRPC service layer handling authentication and access control.

#### 8.9.2 Minimum Network Size

| Phase | Storage Nodes | Key Custodians | Rationale |
|-------|--------------|----------------|-----------|
| Bootstrap (MVP) | 5 | 5 (T=3) | Minimum viable distribution |
| Early growth | 15 | 10 (T=7) | Reasonable fault tolerance |
| Production | 30+ | 20 (T=12) | Full security model |

At bootstrap, parameters are relaxed: K=4, M=2, R=2 to function with fewer
nodes. These are tightened by governance as the network grows.

#### 8.9.3 Dual-Role Nodes

Storage nodes **may also serve as inference nodes** (dual role) by registering
with both `service_type = "storage"` and `service_type = "inference"` in
`x/compute_pool`. However:

- Separate stake is required for each role
- SLA metrics are tracked independently
- Chunk storage and inference serving use separate resources (disk vs. GPU)
- The same machine can run both services if hardware supports it

#### 8.9.4 BSP Coordinator Deployment

The BSP Coordinator runs as a service within the inference layer alongside the
Dataset Exporter. It:

- Listens for `zerone.export.snapshot.ready` NATS events
- Performs RS encoding, encryption, SSS key splitting, and chunk upload
- Does NOT persist key material after upload
- Can be run by any operator (not a privileged role); the on-chain Merkle root
  is the source of truth

---

## 9. Token Economics

This section defines the complete ZRN token flow through the inference layer —
how every participant earns, spends, and stakes ZRN. All values reference
existing on-chain parameters from `x/knowledge`, `x/billing`, and
`x/compute_pool` unless otherwise noted.

### 9.1 Participants

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                         ZERONE AGENT ECONOMY                                │
│                                                                            │
│  ┌───────────────┐  ┌───────────────┐  ┌───────────────┐                   │
│  │ Data          │  │ Data          │  │ Storage       │                   │
│  │ Submitters    │  │ Reviewers     │  │ Nodes         │                   │
│  │               │  │ (validators)  │  │               │                   │
│  │ Create value  │  │ Verify value  │  │ Host value    │                   │
│  └───────┬───────┘  └───────┬───────┘  └───────┬───────┘                   │
│          │                  │                   │                           │
│          │     ┌────────────┴───────────┐       │                           │
│          │     │    ZERONE CHAIN        │       │                           │
│          └────▶│    (settlement layer)  │◀──────┘                           │
│                │                        │                                   │
│          ┌────▶│    ZRN flows through   │◀──────┐                           │
│          │     │    on-chain modules    │       │                           │
│          │     └────────────┬───────────┘       │                           │
│          │                  │                   │                           │
│  ┌───────┴───────┐  ┌──────┴────────┐  ┌──────┴────────┐                   │
│  │ API           │  │ Dataset       │  │ Inference     │                   │
│  │ Consumers     │  │ Buyers        │  │ Operators     │                   │
│  │               │  │               │  │ (GPU)         │                   │
│  │ Pay for       │  │ Pay for       │  │ Serve         │                   │
│  │ inference     │  │ training data │  │ models        │                   │
│  └───────────────┘  └───────────────┘  └───────────────┘                   │
│                                                                            │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │                     Protocol Treasury                                │  │
│  │  Research Fund (3.33%) │ Development Fund (19.67%) │ Treasury (4.4%) │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────────────────┘
```

| Participant | Role | Earns ZRN | Spends/Stakes ZRN |
|-------------|------|-----------|-------------------|
| Data Submitter | Submits training samples to ToK | Revenue share from inference + dataset sales | Stakes 1 ZRN per submission |
| Data Reviewer | Validates submission quality | 22% of dataset revenue; citation rewards | Slash risk for inaccurate reviews |
| Storage Node | Hosts encrypted dataset chunks | Storage serving fees | Stakes 100+ ZRN (slashable) |
| API Consumer | Pays for model inference | — | Pays per 1K tokens |
| Dataset Buyer | Purchases raw training data | — | Pays per-sample or bulk price |
| Inference Operator | Runs GPU inference servers | 55% of inference revenue | Stakes 100 ZRN (slashable) |
| Protocol Treasury | Receives protocol fees | Research + dev + treasury shares | Funds bounties, grants, operations |

---

### 9.2 API Inference Pricing

#### 9.2.1 Unit of Pricing

All inference is priced **per 1K tokens** (input + output combined), matching
industry standard. The base price is denominated in **uzrn** (1 ZRN = 1,000,000
uzrn).

#### 9.2.2 Model Tier Pricing

Different model sizes carry different compute costs. Pricing reflects the GPU
resources required for inference.

| Model Tier | Example | Price per 1K Tokens (uzrn) | Price per 1K Tokens (ZRN) | Rationale |
|------------|---------|---------------------------|--------------------------|-----------|
| Small (≤8B) | zerone-8b | 100,000 | 0.1 ZRN | 1× A100, high throughput |
| Medium (8B–34B) | zerone-13b | 400,000 | 0.4 ZRN | 1–2× A100, moderate throughput |
| Large (≥70B) | zerone-70b | 1,000,000 | 1.0 ZRN | 4× A100, lower throughput |
| Domain-specialist | zerone-8b-science | 150,000 | 0.15 ZRN | Same GPU as small + domain premium |

**Derivation from on-chain parameters:**

The `x/billing` base query price is 1,000,000 uzrn (1 ZRN) per fact query, which
maps to approximately 500 tokens of output. Normalizing:

```
1,000,000 uzrn / 500 tokens × 1000 = 2,000,000 uzrn per 1K tokens (at-cost)
```

Inference pricing is set **below** the per-fact price because inference amortizes
training data across millions of queries, while dataset access is a direct data
purchase. The 100,000 uzrn/1K tokens base rate for 8B models represents a ~20×
cost reduction compared to individual fact queries — this is the value proposition
of the trained model.

#### 9.2.3 Dynamic Pricing (Demand-Based)

The `x/billing` module supports dynamic pricing via a 3-tier oracle:

1. **Manual governance override** — `ManualZrnPriceUsd` for emergency adjustments
2. **TWAP oracle** — time-weighted average from liquidity pool (1000-block window)
3. **Fallback** — fixed `BaseQueryPrice` when oracles unavailable

**Surge pricing** applies when inference queue depth exceeds capacity:

| Queue Utilization | Price Multiplier | Trigger |
|-------------------|-----------------|---------|
| 0–70% | 1.0× (base) | Normal load |
| 70–90% | 1.25× | Moderate congestion |
| 90–100% | 1.5× | High congestion |
| Queueing (> capacity) | 2.0× | Backpressure |

Surge pricing is enforced at the API Gateway (metering ledger records actual
price paid). Multipliers are configurable via operator settings, not on-chain
governance, since they reflect off-chain infrastructure capacity.

**Confidence and freshness adjustments** (from `x/billing` pricing engine):
- Queries touching high-confidence domains (>850K BPS): −20% discount
- Queries touching low-confidence domains (<500K BPS): +20% surcharge
- Queries touching fresh knowledge (within 1000 blocks): +10% freshness premium

#### 9.2.4 Free Tier (Bootstrap)

During network bootstrap (first 12 months or until ToK reaches 100K approved
samples, whichever comes first):

- **100K free tokens per account** per month (funded by protocol treasury)
- Free tier accounts require wallet authentication (no anonymous free usage)
- Free-tier usage still generates revenue-share events on-chain (treasury pays)
- Converts to paid-only automatically after bootstrap period ends

#### 9.2.5 Bulk Prepaid Packages

High-volume consumers can prepay for discounted inference:

| Package | Tokens | Price (ZRN) | Discount | Effective per 1K Tokens |
|---------|--------|-------------|----------|------------------------|
| Starter | 1M | 80 | 20% | 80,000 uzrn |
| Growth | 10M | 700 | 30% | 70,000 uzrn |
| Enterprise | 100M | 5,000 | 50% | 50,000 uzrn |

Prepaid packages are locked as on-chain deposits. Unused tokens do not expire
but are non-refundable (can be transferred between accounts owned by the same
wallet).

---

### 9.3 Dataset Access Pricing

Dataset pricing uses the existing `x/knowledge` access payment system.

#### 9.3.1 Per-Sample Pricing

Base fee: **100,000 uzrn** (0.1 ZRN) per sample (`AccessFeePerSample`).

Quality-tier multipliers scale the base fee:

| Quality Tier | Multiplier (BPS) | Effective Price per Sample |
|--------------|------------------|---------------------------|
| Gold (≥800K) | 30,000 (3×) | 300,000 uzrn (0.3 ZRN) |
| Silver (≥600K) | 20,000 (2×) | 200,000 uzrn (0.2 ZRN) |
| Bronze (≥400K) | 10,000 (1×) | 100,000 uzrn (0.1 ZRN) |

#### 9.3.2 Bulk Dataset Pricing

Full dataset access applies a **20% discount** (`BulkDiscountBPS` = 200,000):

```
bulk_price = sum(per_sample_prices) × (1 - 200,000 / 1,000,000)
           = sum(per_sample_prices) × 0.80
```

Curators may set an explicit `BulkPrice` on their datasets. If set, it overrides
the calculated discount.

**Example:** A dataset with 5,000 gold + 3,000 silver + 2,000 bronze samples:

```
Per-sample total = (5,000 × 300,000) + (3,000 × 200,000) + (2,000 × 100,000)
                 = 1,500,000,000 + 600,000,000 + 200,000,000
                 = 2,300,000,000 uzrn (2,300 ZRN)

Bulk price       = 2,300,000,000 × 0.80
                 = 1,840,000,000 uzrn (1,840 ZRN)

Savings          = 460 ZRN (20% discount)
```

#### 9.3.3 Domain Pricing Variation

Rarer or higher-value domains naturally cost more because they contain a higher
proportion of gold-tier samples (reflecting genuine scarcity). No additional
domain premium is applied — the quality multiplier already captures value
differentiation.

Domains with active `DataBounty` rewards (auto-bounty = 10 ZRN when demand hits
100 unfulfilled queries) signal market demand and attract more submissions,
increasing supply and moderating prices organically.

#### 9.3.4 Minimum Viable Fine-Tune Threshold

To fine-tune a useful LoRA adapter, a buyer needs approximately:

| Model Size | Minimum Samples | Recommended Samples | Min Cost (bulk, mixed quality) |
|------------|-----------------|---------------------|-------------------------------|
| 8B | 500 | 2,000+ | ~80 ZRN |
| 13B | 1,000 | 5,000+ | ~160 ZRN |
| 70B | 2,000 | 10,000+ | ~320 ZRN |

These thresholds match the Blind Storage Protocol's design: partial payment
yields useless fragments. The BSP threshold parameter (K-of-N chunks required)
can be aligned to require payment for at least the minimum viable sample count.

#### 9.3.5 Subscription Access

Ongoing access to new data as the ToK grows:

- **Domain subscription**: buyer pays a flat monthly rate (set by governance) to
  receive all new approved samples in a domain
- Priced at a discount to per-sample (approximately 60% of expected per-sample
  cost based on historical submission rate)
- Subscriptions are on-chain recurring payments via `x/billing` (auto-deduct
  from deposit each epoch)
- Subscribers get priority access to Blind Storage chunks (pre-encrypted with
  their key on approval)

---

### 9.4 Revenue Distribution

#### 9.4.1 API Inference Revenue Flow

```
                        API Consumer pays 100,000 uzrn (per 1K tokens)
                                       │
                                       ▼
                          ┌────────────────────────┐
                          │   x/billing Settlement  │
                          │   (on-chain)            │
                          └────────────┬───────────┘
                                       │
              ┌────────────────────────┼────────────────────────┐
              │                        │                        │
              ▼                        ▼                        ▼
    ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
    │  Contributor     │    │  Protocol        │    │  Research +     │
    │  (55%)           │    │  (22%)           │    │  Development    │
    │  55,000 uzrn     │    │  22,000 uzrn     │    │  (23%)          │
    └────────┬────────┘    └────────┬────────┘    │  23,000 uzrn    │
             │                      │              └────────┬────────┘
             │                      │                       │
             ▼                      ▼                       ▼
   ┌──────────────────┐  ┌──────────────────┐  ┌──────────────────┐
   │ Inference        │  │ Citation Pool    │  │ Research Fund    │
   │ Operator         │  │ (50% of proto)   │  │ 3.33% = 3,330   │
   │ (runs GPUs)      │  │ 11,000 uzrn      │  │                  │
   │                  │  │ → data submitters │  │ Development Fund │
   │ Receives 55%     │  │   weighted by     │  │ 19.67% = 19,670 │
   │ for serving      │  │   confidence ×    │  │                  │
   │ the model        │  │   log2(citations) │  └──────────────────┘
   └──────────────────┘  │                  │
                         │ Verification     │
                         │ Pool (30%)       │
                         │ 6,600 uzrn       │
                         │ → reviewers      │
                         │                  │
                         │ Treasury (20%)   │
                         │ 4,400 uzrn       │
                         │ → governance     │
                         └──────────────────┘
```

**Breakdown per 100,000 uzrn of inference revenue:**

| Recipient | Share | Amount (uzrn) | Source Module |
|-----------|-------|---------------|---------------|
| Inference Operator | 55% | 55,000 | `x/billing` ContributorBps |
| Data Submitters (citation pool) | 11% | 11,000 | `x/billing` ProtocolBps → KnowledgePoolShareOfProtocolBps (50% of 22%) |
| Data Reviewers (verification pool) | 6.6% | 6,600 | `x/billing` ProtocolBps → VerificationPoolShareOfProtocolBps (30% of 22%) |
| Protocol Treasury | 4.4% | 4,400 | `x/billing` ProtocolBps → remainder (20% of 22%) |
| Research Fund | 3.33% | 3,330 | `x/billing` ResearchBps |
| Development Fund | 19.67% | 19,670 | `x/billing` DevelopmentBps |

**Citation pool distribution** uses confidence-weighted logarithmic scaling:

```
weight(sample) = confidence_score × floor(log2(citations + 1) + 1)
```

This rewards submitters whose data is frequently cited (used in inference) and
whose samples have high confidence scores, creating a direct link between data
quality and earnings.

#### 9.4.2 Dataset Purchase Revenue Flow

```
                     Dataset Buyer pays 1,840,000,000 uzrn (bulk purchase)
                                       │
                                       ▼
                          ┌────────────────────────┐
                          │   x/knowledge Access    │
                          │   Payment (on-chain)    │
                          └────────────┬───────────┘
                                       │
              ┌────────────────────────┼────────────────────────┐
              │                        │                        │
              ▼                        ▼                        ▼
    ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
    │  Curator         │    │  Data            │    │  Protocol       │
    │  Commission      │    │  Submitters      │    │  (23%)          │
    │  (95% of fee)    │    │  (5% of fee,     │    │                 │
    │                  │    │   quality-        │    │  Split per      │
    │  For curating    │    │   weighted)       │    │  x/knowledge    │
    │  the dataset     │    │                  │    │  revenue params │
    └────────┬────────┘    └────────┬────────┘    └────────┬────────┘
             │                      │                       │
             ▼                      ▼                       ▼
   Revenue to curator      Revenue split among      Research (7%)
   via x/knowledge         sample submitters       Founder (8%)
   CuratorCommissionBPS    in the purchased        AI Ops (3%)
   = 950,000 (95%)         dataset, weighted by    Protocol remainder
                           quality tier + consent
```

**Dataset revenue distribution is two-stage:**

**Stage 1 — Fee split (on purchase):**

The `x/knowledge` access payment module applies:
- **95%** to curator commission (`CuratorCommissionBPS` = 950,000)
- **5%** distributed to sample submitters in the dataset

**Stage 2 — Submitter share distribution (per epoch):**

The 5% submitter pool is distributed via the `x/knowledge` revenue queue:
- **55%** to submitters (`SubmitterRevenueShareBps` = 5,500)
- **22%** to validators who reviewed those samples (`ValidatorRevenueShareBps` = 2,200)
- **23%** to protocol (split: 7% research, 8% founder, 3% AI ops, remainder treasury)

**Consent multipliers adjust submitter revenue:**

| Consent Type | Multiplier | Effect on Submitter Revenue |
|-------------|-----------|---------------------------|
| SELF_AUTHORED | 1.5× (15,000 BPS) | +50% bonus (highest quality signal) |
| OPT_IN | 1.3× (13,000 BPS) | +30% bonus |
| PUBLIC_LICENSE | 1.0× (10,000 BPS) | Baseline (no adjustment) |
| PLATFORM_TOS | 0.8× (8,000 BPS) | −20% penalty |
| FAIR_USE | 0.5× (5,000 BPS) | −50% penalty |

Delta from consent multiplier application flows to the protocol share.

---

### 9.5 Staking Requirements

#### 9.5.1 Staking Overview

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         STAKING REQUIREMENTS                           │
│                                                                        │
│  Role               Minimum Stake    Slashable?   Slash Conditions     │
│  ─────────────────  ──────────────   ──────────   ──────────────────── │
│  Data Submitter     1 ZRN / claim    Yes          Rejected submission  │
│  Data Reviewer      Validator stake  Yes          Outlier / missed     │
│  Storage Node       100 ZRN          Yes          Downtime / data loss │
│  Inference Operator 100 ZRN          Yes          Downtime / bad resp  │
│  Dataset Curator    10 ZRN / dataset No           —                    │
│  API Consumer       Deposit (prepay) No           —                    │
│                                                                        │
│  Note: "Validator stake" = existing CometBFT staking via x/staking     │
└─────────────────────────────────────────────────────────────────────────┘
```

#### 9.5.2 Data Submitters

- **Amount**: 1,000,000 uzrn (1 ZRN) per submission (`MinSubmissionStake`)
- **Lock period**: duration of quality round (~8 blocks: 4 commit + 4 reveal)
- **Return**: stake returned after quality round completes regardless of verdict
- **Slash**: if submission is toxic (>200K BPS toxicity score), stake is burned

The low stake serves as a Sybil deterrent — submitting 1000 junk entries costs
1000 ZRN, while the reviewer system catches and rejects low-quality data.

#### 9.5.3 Data Reviewers (Validators)

Reviewers are existing chain validators who participate in quality rounds.

- **Amount**: existing validator stake (from `x/staking`/`x/zerone_staking`)
- **Slash conditions** (from `x/knowledge` params):
  - Wrong validation (>20% deviation from median): 5% of stake (`WrongValidationSlashBps` = 50,000)
  - Missed reveal (committed but didn't reveal): 10% of stake (`MissedRevealSlashBps` = 100,000)
  - Equivocation (duplicate votes): 20% of stake (`EquivocationSlashBps` = 200,000)
- **Selection**: VRF-based random selection, 3–22 validators per round
- **Reward**: equal share of the 22% validator revenue from samples they reviewed

#### 9.5.4 Storage Nodes

Registered in `x/compute_pool` with `service_type="storage"`.

- **Amount**: 100,000,000 uzrn (100 ZRN) minimum (`MinProviderStake`)
- **SLA requirements**: ≥90% uptime, ≤5000ms latency, heartbeat every 100 blocks
- **Slash conditions**: jailed on missed heartbeat; stake slashed for verified
  data loss (failed challenge-response audit)
- **Reward**: proportional share of storage serving fees from Blind Storage
  downloads (paid by dataset buyers)

#### 9.5.5 Inference Operators

Registered in `x/compute_pool` with `service_type="inference"`.

- **Amount**: 100,000,000 uzrn (100 ZRN) minimum (`MinProviderStake`)
- **SLA requirements**: ≥90% uptime, ≤5000ms latency, heartbeat every 100 blocks
- **Slash conditions**: jailed on missed heartbeat; stake slashed for serving
  incorrect model (model hash mismatch with registry)
- **Reward**: 55% of all inference revenue from queries they serve

#### 9.5.6 Dataset Curators

- **Amount**: 10,000,000 uzrn (10 ZRN) per dataset created
- **Not slashable**: curator quality is market-driven (bad datasets get no buyers)
- **Return**: locked while dataset is active; returned if dataset is deactivated
- **Reward**: 95% of dataset access fees (`CuratorCommissionBPS`)

#### 9.5.7 API Consumers

- **Deposit (not stake)**: minimum 1,000,000 uzrn (1 ZRN) to activate account
- **Not slashable**: deposits are consumed by usage, not locked
- **Balance**: `deposit_uzrn - consumed_uzrn` must cover estimated request cost
- **Refund**: unused deposits returnable via on-chain withdrawal

---

### 9.6 ZRN Flow Diagram

Complete token flow showing all ZRN movements between participants:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         COMPLETE ZRN TOKEN FLOW                            │
│                                                                            │
│                                                                            │
│  EARNING SIDE                              SPENDING SIDE                   │
│  ──────────                                ────────────                    │
│                                                                            │
│  ┌──────────────┐                          ┌──────────────┐                │
│  │ Data         │◀────── citation ────────│ API          │                │
│  │ Submitters   │        rewards           │ Consumers    │                │
│  │              │        (11%)             │              │                │
│  │              │◀─── dataset revenue ────│ Dataset      │                │
│  │              │     (5% × 55%)           │ Buyers       │                │
│  └──────────────┘                          └──────┬───────┘                │
│                                                   │                        │
│  ┌──────────────┐                                 │ deposit                │
│  │ Reviewers    │◀── verification pool ──┐       │ ZRN                    │
│  │ (validators) │    (6.6%)              │       │                        │
│  │              │◀── dataset review ──┐  │       ▼                        │
│  │              │    (5% × 22%)       │  │ ┌──────────────┐                │
│  └──────────────┘                     │  │ │  Settlement  │                │
│                                       │  │ │  Layer       │                │
│  ┌──────────────┐                     │  │ │              │                │
│  │ Inference    │◀── provider ────────┤  │ │  x/billing   │                │
│  │ Operators    │    revenue           │  │ │  x/knowledge │                │
│  │ (GPU)        │    (55%)             │  │ │              │                │
│  └──────────────┘                     │  │ └──────────────┘                │
│                                       │  │                                 │
│  ┌──────────────┐                     │  │                                 │
│  │ Storage      │◀── serving fees ────┘  │                                 │
│  │ Nodes        │    (from BSP)          │                                 │
│  └──────────────┘                        │                                 │
│                                          │                                 │
│  ┌──────────────┐                        │                                 │
│  │ Dataset      │◀── curator ────────────┘                                 │
│  │ Curators     │    commission                                            │
│  │              │    (95% of dataset fee)                                   │
│  └──────────────┘                                                          │
│                                                                            │
│  ┌──────────────┐                                                          │
│  │ Protocol     │◀── treasury (4.4%) + research (3.33%) + dev (19.67%)     │
│  │ Treasury     │    + knowledge research tax (7%) + founder (8%)          │
│  └──────────────┘                                                          │
│                                                                            │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

### 9.7 Incentive Alignment Analysis

#### 9.7.1 Data Submitter Loop

```
Submit quality data ──▶ Data approved (GOLD/SILVER) ──▶ Sample enters training set
       ▲                                                          │
       │                                                          ▼
       │                                               Model serves inference
       │                                                          │
       │                                                          ▼
       └──────────── Earn ongoing citation revenue ◀── API consumers pay per query
```

**Positive incentives:**
- Gold samples earn 3× revenue weight versus bronze → quality is directly rewarded
- Self-authored content earns 1.5× consent bonus → original work is valued most
- Citation-weighted distribution means popular/useful data earns more over time
- Ongoing passive income: once submitted, a sample earns forever as long as the
  model serves queries

**Negative deterrents:**
- 1 ZRN stake per submission → spam is costly
- Toxic content auto-rejected and stake burned → harmful content is punished
- Quality rounds with 3–22 validators → hard to game approval

#### 9.7.2 Data Reviewer Loop

```
Review submissions accurately ──▶ Build reputation ──▶ Selected for more rounds
       ▲                                                        │
       │                                                        ▼
       │                                               Earn verification pool
       │                                               revenue (6.6% of inference)
       │                                                        │
       └─────────────── Maintain validator stake ◀──────────────┘
```

**Positive incentives:**
- Consistent, accurate reviews → selected for more quality rounds → more revenue
- Verification pool distributes equally among revealing validators → reliable
  participation is rewarded

**Negative deterrents:**
- Outlier scores (>20% deviation from median): 5% stake slash
- Missed reveals: 10% stake slash
- Equivocation: 20% stake slash
- VRF selection prevents validator collusion (cannot predict who reviews what)

#### 9.7.3 Storage Node Loop

```
Store chunks reliably ──▶ Pass challenge audits ──▶ Serve downloads to buyers
       ▲                                                      │
       │                                                      ▼
       │                                             Earn serving fees
       │                                                      │
       └──────────── Maintain uptime SLA ◀────────────────────┘
```

**Positive incentives:**
- More chunks stored → more serving fee opportunities
- High uptime → stays active (not jailed) → continuous earnings
- Erasure coding means each node only stores a fraction of the data → storage
  costs are manageable

**Negative deterrents:**
- Missed heartbeat → jailed → no revenue
- Failed challenge-response → stake slashed
- 100 ZRN minimum stake → significant commitment

#### 9.7.4 API Consumer Loop

```
Pay ZRN ──▶ Get inference ──▶ Model improves (more training data) ──▶ Better results
   ▲                                                                       │
   │                                                                       ▼
   └────────────────────── Willing to keep paying ◀────────────────────────┘
```

**Flywheel effect:** Consumer payments fund submitter rewards → more/better data →
better model → more consumers → more payments → cycle accelerates.

#### 9.7.5 Dataset Buyer Loop

```
Pay ZRN ──▶ Get training data ──▶ Fine-tune own model ──▶ Create value
   ▲                                                          │
   │                                                          ▼
   └───── Need updated/expanded data ◀──── Model needs refresh ◀──┘
```

**Recurring demand:** Models degrade without fresh data. Subscription access
(Section 9.3.5) captures this recurring demand at a discount.

---

### 9.8 Bootstrap Sequence

The cold-start problem: no data → no model → no revenue → no incentive to submit.

```
Phase 1: SEED                    Phase 2: TRAIN                Phase 3: LAUNCH
(blocks 0 – 480K)               (after 1K+ samples)           (model serving)

┌──────────────┐                ┌──────────────┐              ┌──────────────┐
│ Gas-free      │               │ Protocol-    │              │ Free tier    │
│ submissions   │──────────────▶│ funded first │─────────────▶│ (100K tokens │
│               │  reach 1K+    │ fine-tune    │   model      │  / month)    │
│ DataBounty    │  approved     │              │   deployed   │              │
│ rewards       │  samples      │ Dev fund     │              │ Revenue      │
│ (10 ZRN each) │               │ pays GPU     │              │ starts       │
│               │               │ costs        │              │ flowing      │
│ BootstrapGas  │               │              │              │              │
│ FreeTypes     │               │ Domain-      │              │ Submitter    │
│ includes      │               │ specialist   │              │ flywheel     │
│ MsgSubmitClaim│               │ adapters     │              │ begins       │
└──────────────┘               └──────────────┘              └──────────────┘
```

**Phase 1 — Seed Data (blocks 0 to ~480,000)**

Already implemented in `x/knowledge`:
- `BootstrapGasFreeTypes` includes `MsgSubmitClaim` — zero gas cost for early
  submitters (first 480K blocks ≈ 33 days at 6s blocks)
- `DataBounty` system: auto-bounties of 10 ZRN triggered when demand for a
  domain/topic hits 100 unfulfilled queries (`AutoBountyThreshold`)
- `TrainingDemand` tracking: records which domains lack data, directing
  submissions to highest-value areas

**Phase 2 — Protocol-Funded Training**

- Development fund (19.67% of all protocol revenue) accumulates during seed phase
- First fine-tune job funded entirely from development fund
- Target: 1,000+ approved samples for initial 8B LoRA adapter
- Multiple domain-specialist adapters trained in parallel if domain-specific
  data reaches threshold

**Phase 3 — Launch & Free Tier**

- Model deployed to inference server
- Free tier activated: 100K tokens/month per account, funded by protocol treasury
- Early users validate quality → word of mouth → organic demand
- First paying customers arrive → revenue flows to submitters → flywheel begins

**Phase 4 — Flywheel (self-sustaining)**

- Revenue exceeds protocol costs → treasury grows → more bounties → more data
- Model quality improves → more consumers → more revenue → accelerating cycle
- Domain specialists attract niche users → premium pricing → higher submitter
  rewards in those domains

---

### 9.9 Anti-Gaming Measures

#### 9.9.1 Wash Trading (Submit + Self-Review via Sybils)

**Attack:** Agent creates multiple accounts, submits junk data, and reviews own
submissions with Sybil validators to approve it.

**Defenses:**
- **Commit-reveal scheme**: validators commit hashed votes before revealing,
  preventing copying (quality_round.go)
- **VRF selection**: random validator selection per round — attacker cannot
  guarantee their Sybils are chosen (3–22 validators per round)
- **Stake requirements**: each Sybil validator needs full validator stake;
  each submission costs 1 ZRN — economic attack cost scales linearly
- **Outlier detection**: scores >20% from median are flagged; Sybil votes
  that differ from honest validators are detected and slashed
- **Minimum validators**: at least 3 validators per round — attacker needs to
  control majority of a random subset

**Economic analysis:** To reliably approve junk via Sybils, an attacker needs to
control >50% of selected validators. With 22 max validators per round and VRF
selection from the full validator set, this requires controlling a significant
fraction of total validator stake — far more expensive than the potential revenue
from junk data.

#### 9.9.2 Quantity Over Quality

**Attack:** Agent submits huge volumes of bronze-tier data to earn more total
revenue than fewer gold-tier submissions.

**Defenses:**
- **Quality tier weights**: gold = 3× revenue, silver = 2×, bronze = 1×
  - 1 gold sample earns as much as 3 bronze samples
  - But costs the same 1 ZRN to submit → gold is 3× more profitable per ZRN staked
- **Consent multipliers**: self-authored (1.5×) vs fair-use (0.5×) creates
  additional 3× revenue spread favoring original content
- **Citation-weighted distribution**: inference revenue flows to frequently-cited
  samples — high-quality data gets cited more → earns more over time
- **Ecology system**: low-energy samples get pruned after 10 grace epochs
  (`PruneGraceEpochs`), removing stale/unused bronze data

**Combined effect:** A gold self-authored sample earns 3 × 1.5 = 4.5× the
revenue of a bronze fair-use sample, while costing the same 1 ZRN stake.

#### 9.9.3 Free-Rider Inference (Training Data Extraction)

**Attack:** Extracting training data verbatim through carefully crafted model
queries.

**Defenses:**
- **Output filtering**: n-gram matching against training sample subset to detect
  verbatim reproduction (API Gateway)
- **Rate limiting**: per-account token bucket limits bulk extraction attempts
- **Differential privacy**: training uses quality-weighted sampling with noise —
  individual samples cannot be deterministically reconstructed
- **LoRA adapters**: the fine-tuned component is a small adapter; base model
  knowledge dominates — extracting ZERONE-specific training data from inference
  is information-theoretically difficult
- **Separate access paths**: raw data access is via Blind Storage (paid dataset
  purchase), not inference — the API is not a data retrieval mechanism

#### 9.9.4 Price Manipulation (ZRN Cornering)

**Attack:** Accumulating large ZRN positions to manipulate pricing.

**Defenses:**
- **Dynamic pricing oracle**: TWAP over 1000-block window smooths short-term
  price spikes (`TwapWindowBlocks`)
- **Price bounds**: `MinCostPerFact` (0.001 ZRN) and `MaxCostPerFact` (100 ZRN)
  create hard floors and ceilings
- **Staleness check**: prices older than 5000 blocks fall back to base rate
  (`StalenessBlocks`)
- **Market dynamics accepted**: within bounds, ZRN price reflects genuine supply
  and demand — this is a feature, not a bug

#### 9.9.5 Curator Abuse (Empty/Duplicate Datasets)

**Attack:** Creating datasets with duplicate or already-public samples to farm
95% curator commission.

**Defenses:**
- **Content hash deduplication**: `ContentHashIndex` (prefix `0x0E`) prevents
  identical samples from existing twice on-chain
- **Curator stake**: 10 ZRN per dataset → creating junk datasets has real cost
- **Market pressure**: buyers can see sample counts, quality distributions, and
  filter criteria before purchasing — empty or low-quality datasets get no buyers
- **Energy system**: samples that receive no access lose energy and eventually get
  pruned — datasets with pruned samples shrink automatically

---

### 9.10 Worked Example

**Scenario:** Agent A submits 100 gold-tier, self-authored samples to the science
domain. Over 30 days, those samples contribute to a model that serves 1M API
calls. Meanwhile, a dataset buyer purchases bulk access to the science domain
dataset.

#### Setup

- 100 samples, all GOLD quality (score ≥800K BPS)
- All SELF_AUTHORED consent (1.5× multiplier)
- Model tier: zerone-8b at 100,000 uzrn per 1K tokens
- Average tokens per API call: 500 (prompt + completion)
- Total ToK science samples: 10,000 (Agent A contributed 1%)
- Assume Agent A's samples are in the active training set

#### Revenue Source 1: API Inference

```
Total API revenue from 1M calls:
  1,000,000 calls × 500 tokens/call = 500,000,000 tokens = 500,000 × 1K tokens
  500,000 × 100,000 uzrn = 50,000,000,000 uzrn (50,000 ZRN)

Citation pool (11% of inference revenue):
  50,000,000,000 × 0.11 = 5,500,000,000 uzrn (5,500 ZRN)

Agent A's share of citation pool:
  Agent A has 100 gold samples out of 10,000 total science samples.
  Gold samples get 3× weight in citation distribution.
  Assume average domain sample is silver (2× weight).

  Agent A's weight: 100 samples × 3 (gold) × 1.5 (self-authored) = 450
  Domain total weight (approx): 10,000 samples × 2 (avg) × 1.0 (avg consent) = 20,000

  Agent A's share: 450 / 20,000 = 2.25%

  Agent A citation revenue: 5,500 ZRN × 2.25% = 123.75 ZRN
```

#### Revenue Source 2: Dataset Purchase

```
Buyer purchases bulk access to science domain (10,000 samples):
  Assume mixed quality: 2,000 gold, 5,000 silver, 3,000 bronze

  Per-sample total = (2,000 × 300,000) + (5,000 × 200,000) + (3,000 × 100,000)
                   = 600M + 1,000M + 300M = 1,900,000,000 uzrn

  Bulk price (20% discount): 1,900,000,000 × 0.80 = 1,520,000,000 uzrn (1,520 ZRN)

Stage 1 — Curator commission:
  95% to curator: 1,520 × 0.95 = 1,444 ZRN (curator keeps this)
  5% to sample submitters: 1,520 × 0.05 = 76 ZRN

Stage 2 — Submitter pool distribution (76 ZRN):
  Submitter share: 76 × 0.55 = 41.8 ZRN

  Agent A's share (100 of 10,000 samples, gold 3× weight, self-authored 1.5×):
    Agent A weight: 100 × 3 × 1.5 = 450
    Total weight: ~20,000 (same as above)
    Agent A's portion: 41.8 × (450/20,000) = 0.94 ZRN

  Reviewer share: 76 × 0.22 = 16.72 ZRN (split among reviewing validators)
  Protocol share: 76 × 0.23 = 17.48 ZRN
```

#### Total 30-Day Earnings for Agent A

```
┌──────────────────────────────────────────────────────────────┐
│  Agent A — 30-Day Revenue Summary                           │
│                                                              │
│  Cost to submit:                                             │
│    100 submissions × 1 ZRN stake = 100 ZRN (returned)       │
│    Net cost: 0 ZRN (gas-free during bootstrap)               │
│                                                              │
│  Revenue:                                                    │
│    API inference citation pool:     123.75 ZRN               │
│    Dataset purchase submitter pool:   0.94 ZRN               │
│                                     ─────────                │
│    Total earned:                    124.69 ZRN               │
│                                                              │
│  ROI: ∞ (zero net cost during bootstrap)                     │
│  Post-bootstrap ROI: 124.69 / 0 gas ≈ pure profit           │
│  (stakes are returned after each quality round)              │
│                                                              │
│  Note: This is ONGOING. As long as the model serves          │
│  queries using Agent A's training data, Agent A earns        │
│  citation revenue. Month 2 could yield similar or more       │
│  if API usage grows.                                         │
└──────────────────────────────────────────────────────────────┘
```

#### Key Takeaways from Example

1. **Quality pays**: Agent A's gold samples earn 3× per sample compared to bronze
2. **Original content pays**: self-authored 1.5× multiplier compounds with quality
3. **Inference dominates**: API citation revenue (123.75 ZRN) vastly exceeds
   dataset purchase revenue (0.94 ZRN) — the model amplifies data value
4. **Passive income**: once submitted, earnings are automatic and ongoing
5. **Curator incentive is separate**: the 95% curator commission goes to whoever
   assembled the dataset, not the raw submitters — curators add value by
   organizing and filtering

---

### 9.11 Parameter Summary

All economic parameters referenced in this section, with their source modules
and current values:

| Parameter | Value | Unit | Source |
|-----------|-------|------|--------|
| **Pricing** | | | |
| AccessFeePerSample | 100,000 | uzrn | x/knowledge |
| BaseQueryPrice | 1,000,000 | uzrn | x/billing |
| Inference 8B model | 100,000 | uzrn/1K tokens | API Gateway config |
| Inference 70B model | 1,000,000 | uzrn/1K tokens | API Gateway config |
| BulkDiscountBPS | 200,000 | BPS (20%) | x/knowledge |
| MinCostPerFact | 1,000 | uzrn | x/billing |
| MaxCostPerFact | 100,000,000 | uzrn | x/billing |
| **Revenue Splits** | | | |
| ContributorBps (billing) | 550,000 | BPS (55%) | x/billing |
| ProtocolBps (billing) | 220,000 | BPS (22%) | x/billing |
| ResearchBps | 33,300 | BPS (3.33%) | x/billing |
| DevelopmentBps | 196,700 | BPS (19.67%) | x/billing |
| KnowledgePoolShare | 500,000 | BPS (50% of protocol) | x/billing |
| VerificationPoolShare | 300,000 | BPS (30% of protocol) | x/billing |
| SubmitterRevenueShareBps | 5,500 | BPS (55%) | x/knowledge |
| ValidatorRevenueShareBps | 2,200 | BPS (22%) | x/knowledge |
| CuratorCommissionBPS | 950,000 | BPS (95%) | x/knowledge |
| ResearchTaxBps | 70,000 | BPS (7%) | x/knowledge |
| FounderShareBps | 80,000 | BPS (8%) | x/knowledge |
| AiOperationsShareBps | 30,000 | BPS (3%) | x/knowledge |
| **Quality** | | | |
| GoldThreshold | 800,000 | BPS | x/knowledge |
| SilverThreshold | 600,000 | BPS | x/knowledge |
| BronzeThreshold | 400,000 | BPS | x/knowledge |
| GoldQualityMultiplier | 30,000 | BPS (3×) | x/knowledge |
| SilverQualityMultiplier | 20,000 | BPS (2×) | x/knowledge |
| BronzeQualityMultiplier | 10,000 | BPS (1×) | x/knowledge |
| **Consent** | | | |
| SelfAuthoredMultiplier | 15,000 | BPS (1.5×) | x/knowledge |
| OptInMultiplier | 13,000 | BPS (1.3×) | x/knowledge |
| PublicLicenseMultiplier | 10,000 | BPS (1.0×) | x/knowledge |
| PlatformTosMultiplier | 8,000 | BPS (0.8×) | x/knowledge |
| FairUseMultiplier | 5,000 | BPS (0.5×) | x/knowledge |
| **Staking** | | | |
| MinSubmissionStake | 1,000,000 | uzrn (1 ZRN) | x/knowledge |
| MinProviderStake | 100,000,000 | uzrn (100 ZRN) | x/billing |
| WrongValidationSlashBps | 50,000 | BPS (5%) | x/knowledge |
| MissedRevealSlashBps | 100,000 | BPS (10%) | x/knowledge |
| EquivocationSlashBps | 200,000 | BPS (20%) | x/knowledge |
| **Bootstrap** | | | |
| AutoBountyThreshold | 100 | queries | x/knowledge |
| AutoBountyAmount | 10,000,000 | uzrn (10 ZRN) | x/knowledge |
| BootstrapGasFreeBlocks | 480,000 | blocks (~33 days) | x/knowledge |
| FreeTierTokensPerMonth | 100,000 | tokens | API Gateway config |
| **Dynamic Pricing** | | | |
| ConfidenceWeightBps | 200,000 | BPS (20%) | x/billing |
| FreshnessWeightBps | 100,000 | BPS (10%) | x/billing |
| ConfidenceThreshold | 500,000 | BPS | x/billing |
| FreshnessWindowBlocks | 1,000 | blocks | x/billing |
| TwapWindowBlocks | 1,000 | blocks | x/billing |
| StalenessBlocks | 5,000 | blocks | x/billing |

---

*This document is the authoritative reference for the ZERONE inference layer
architecture. Implementation details for each component are covered in
T2 (Dataset Exporter + Training Pipeline), T3 (Inference Server + API Gateway),
T4 (Payment Bridge + Metering), and T5 (Blind Storage + Monitoring).*
