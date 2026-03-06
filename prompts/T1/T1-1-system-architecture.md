# T1-1 — System Architecture

## Goal

Design the complete system architecture for the ZERONE inference layer. This is the off-chain service stack that sits alongside the ZERONE blockchain, consuming training data from the on-chain knowledge module and serving fine-tuned model inference paid in ZRN.

## Context

### What Already Exists (On-Chain)
- **x/knowledge module** — training data collection with:
  - Samples (validated training data units with quality tiers: gold/silver/bronze)
  - Submissions (raw data awaiting quality validation)
  - QualityRounds (commit-reveal validation by validators)
  - Datasets (curated collections filtered by domain/type/quality/language)
  - DataBounties (rewards for filling training data gaps)
  - TrainingDemand (aggregate demand signals per domain/topic)
  - Domains (data categories: technical, culture, science, etc.)
  - 12 SampleTypes (discussion, debate, explanation, tutorial, Q&A, etc.)
- **x/billing module** — on-chain payment/metering
- **x/compute_pool module** — compute resource management

### What We're Building (Off-Chain)
An inference service that:
1. Exports approved training data from the knowledge module
2. Fine-tunes open-source base models (Llama, Mistral, Qwen, etc.)
3. Serves inference via API
4. Accepts ZRN as payment
5. Distributes training data via blind storage for dataset buyers
6. Continuously retrains as the ToK grows

## Deliverables

### 1. Component Diagram

Define and describe each component:

- **Dataset Exporter** — Watches the chain, exports approved Samples into training-ready formats (instruction pairs, conversation, raw text). Transforms based on SampleType.
- **Training Pipeline** — Takes exported datasets, runs fine-tuning jobs on base models. Manages model versioning. Triggers on dataset snapshots (e.g., every N new approved samples or on-demand).
- **Model Registry** — Stores trained model weights with metadata (base model, dataset version, domain focus, training config, benchmark scores). Tracks which model is active for serving.
- **Inference Server** — Serves the active model via API. Use vLLM or TGI (text-generation-inference) in Docker containers. Handles request routing, batching, streaming responses.
- **API Gateway** — Public-facing API that handles:
  - Authentication (wallet signature or API key linked to ZRN deposit)
  - Rate limiting
  - Request routing to inference server
  - Usage metering (tokens consumed per request)
  - ZRN balance checking and deduction
- **Payment Bridge** — Connects API gateway to the ZERONE chain:
  - Monitors prepaid ZRN deposits (escrow accounts on-chain)
  - Decrements balances per API call off-chain
  - Settles periodically on-chain (batch settlement)
  - Handles disputes and refunds
- **Blind Storage Network** — Distributes encrypted dataset chunks to storage nodes (see T1-2 for detailed design)
- **Monitoring & Metrics** — Tracks model performance, API latency, usage stats, revenue

### 2. Data Flow Diagrams

Document these flows:
1. **Submission → Training**: Sample approved on-chain → Exporter picks it up → transforms to training format → added to dataset staging area
2. **Training → Serving**: Dataset snapshot created → fine-tuning job runs → model registered → promoted to active → inference server loads it
3. **API Request → Response**: Client sends request + auth → gateway validates ZRN balance → routes to inference server → response streamed → usage metered → balance decremented
4. **Settlement**: Off-chain usage ledger → periodic batch → on-chain settlement tx → revenue distributed (to storage nodes, data contributors, protocol)
5. **Dataset Purchase**: Buyer deposits ZRN → requests dataset access → blind storage releases chunks proportional to payment → buyer reassembles

### 3. Technology Choices

For each component, specify:
- Language/framework
- Deployment (Docker container, k8s, bare metal)
- Storage (PostgreSQL, Redis, S3, IPFS)
- Communication (gRPC, REST, message queue)

Recommended stack:
- **Inference**: vLLM (best throughput for open models) in Docker
- **API Gateway**: Go service (matches chain codebase, high perf)
- **Training Pipeline**: Python (PyTorch, HuggingFace transformers, LoRA/QLoRA)
- **Dataset Exporter**: Go service subscribing to chain events
- **Payment Bridge**: Go service with Cosmos SDK client
- **Message Queue**: NATS or Redis Streams (lightweight, fast)
- **Storage**: PostgreSQL (metadata), S3-compatible (model weights, datasets), Redis (session/cache)

### 4. Deployment Topology

- Where does each component run?
- GPU requirements (inference vs training)
- Scaling strategy (horizontal for inference, vertical for training)
- Start with single-node, design for multi-node

### 5. Security Model

- How is the API gateway authenticated?
- How do we prevent inference theft (prompt injection to dump training data)?
- How do we protect model weights?
- How do we verify storage nodes aren't serving garbage?

### 6. Interface Contracts

Define the API surface:
```
POST /v1/chat/completions    — OpenAI-compatible chat endpoint
POST /v1/completions         — OpenAI-compatible completion endpoint
GET  /v1/models              — List available models
GET  /v1/balance             — Check ZRN balance
POST /v1/deposit             — Get deposit address for ZRN
GET  /v1/usage               — Usage history
POST /v1/datasets/purchase   — Initiate dataset purchase (blind storage)
GET  /v1/datasets             — List available datasets
```

OpenAI-compatible endpoints ensure drop-in replacement for existing tooling.

## Output

A single architecture document: `docs/inference-layer/ARCHITECTURE.md` in the zerone repo.
Include diagrams as ASCII art or mermaid blocks.
This document becomes the reference for T2-T5 implementation.
