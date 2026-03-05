# ZERONE Inference Layer — Local Development

Local development stack for the ZERONE off-chain inference layer.
For full system design, see [ARCHITECTURE.md](./ARCHITECTURE.md).

## Quick Start

```bash
# 1. Copy environment config
cp .env.example .env

# 2. Start the stack (CPU-only dev mode, no GPU required)
docker compose --profile cpu up -d

# 3. Verify all services are healthy
docker compose ps

# 4. Test the API
curl http://localhost:8080/healthz
```

### GPU Mode

If you have an NVIDIA GPU with [nvidia-container-toolkit](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/install-guide.html) installed:

```bash
# Download a base model first
mkdir -p models/base
# (place Llama-3.1-8B-Instruct weights in models/base/Llama-3.1-8B-Instruct)

# Start with GPU inference
docker compose --profile gpu up -d
```

## Architecture Overview

```
                    ┌──────────────┐
  clients ────────▶ │ API Gateway  │ :8080 (only exposed port)
                    └──────┬───────┘
                           │
              ┌────────────┼────────────┐
              │            │            │
      ┌───────▼──┐  ┌──────▼──┐  ┌─────▼────────┐
      │ Inference│  │ Payment │  │ Blind        │
      │ Server   │  │ Bridge  │  │ Storage Node │
      │ :8000    │  │ :9300   │  │ :9400        │
      └──────────┘  └────┬────┘  └──────────────┘
                         │
              ┌──────────┼──────────┐
              │          │          │
      ┌───────▼──┐ ┌────▼───┐ ┌───▼────────────┐
      │PostgreSQL│ │ Redis  │ │ Chain Mock     │
      │ :5432    │ │ :6379  │ │ :9090          │
      └──────────┘ └────────┘ └────────────────┘
              │
      ┌───────▼──────────┐
      │ Dataset Exporter │ (syncs chain → PostgreSQL)
      └──────────────────┘
```

All inter-service communication happens on an internal Docker network.
Only the API Gateway is exposed to the host.

## Services

| Service | Port | Description |
|---------|------|-------------|
| **api-gateway** | 8080 | OpenAI-compatible API with auth, rate limiting, metering |
| **inference** (GPU) | 8000 (internal) | vLLM server with LoRA hot-swap |
| **inference-stub** (CPU) | 8000 (internal) | Canned-response stub for dev without GPU |
| **dataset-exporter** | 9200 (metrics) | Syncs approved samples from chain to PostgreSQL |
| **payment-bridge** | 9300 (gRPC) | Off-chain metering ledger + on-chain settlement |
| **blind-storage** | 9400 | Encrypted dataset chunk storage node |
| **chain-mock** | 9090 | Stub for ZERONE chain gRPC (logs all requests) |
| **training-pipeline** | — | Batch: transforms data + fine-tunes models (on-demand) |

### Infrastructure

| Service | Port | Purpose |
|---------|------|---------|
| PostgreSQL | 5432 | Metering ledger, dataset staging, account balances |
| Redis | 6379 | Session cache, rate limiting, off-chain ledger |
| NATS | 4222 | Event bus (JetStream enabled) |
| MinIO | 9000 (API), 9001 (console) | S3-compatible object storage for models + datasets |

## Profiles

The compose file uses profiles to control optional services:

```bash
# Core stack only (infra + app services + CPU stub)
docker compose --profile cpu up -d

# Core stack with real GPU inference
docker compose --profile gpu up -d

# Add monitoring (Prometheus + Grafana)
docker compose --profile cpu --profile monitoring up -d

# Run training pipeline
docker compose --profile training run training-pipeline transform --help
```

## API Usage Examples

### Chat Completion

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test-api-key" \
  -d '{
    "model": "zerone-dev-stub",
    "messages": [
      {"role": "system", "content": "You are a helpful assistant."},
      {"role": "user", "content": "What is ZERONE?"}
    ],
    "max_tokens": 256
  }'
```

### Streaming Chat Completion

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test-api-key" \
  -d '{
    "model": "zerone-dev-stub",
    "messages": [
      {"role": "user", "content": "Explain knowledge curation."}
    ],
    "stream": true
  }'
```

### List Models

```bash
curl http://localhost:8080/v1/models \
  -H "Authorization: Bearer test-api-key"
```

### Health Check

```bash
# API Gateway
curl http://localhost:8080/healthz

# Individual services (from inside the Docker network)
docker compose exec api-gateway wget -qO- http://inference:8000/health
docker compose exec api-gateway wget -qO- http://chain-mock:9090/health
```

### Dataset Marketplace — Browse Datasets

```bash
curl http://localhost:8080/v1/datasets \
  -H "Authorization: Bearer test-api-key"
```

### Dataset Marketplace — Purchase

```bash
curl -X POST http://localhost:8080/v1/datasets/purchase \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test-api-key" \
  -d '{
    "dataset_id": "ds-technical-001",
    "tier": "standard"
  }'
```

### Check Account Balance

```bash
curl http://localhost:8080/v1/balance \
  -H "Authorization: Bearer test-api-key"
```

### Buyer CLI (runs on host, talks to gateway)

```bash
# Build the buyer CLI
cd services/buyer-cli && go build -o buyer-cli . && cd -

# Browse available datasets
./services/buyer-cli/buyer-cli browse --gateway http://localhost:8080 --api-key test-api-key

# Show pricing tiers
./services/buyer-cli/buyer-cli pricing --gateway http://localhost:8080

# Purchase a dataset
./services/buyer-cli/buyer-cli purchase \
  --gateway http://localhost:8080 \
  --api-key test-api-key \
  --dataset ds-technical-001 \
  --tier standard
```

## Connecting to a Real Chain

By default, the stack uses `chain-mock` — a lightweight stub that logs all gRPC
requests from services. To connect to a real ZERONE testnet node:

1. Edit `.env`:
   ```
   CHAIN_GRPC=your-testnet-node:9090
   ```

2. Remove or stop the chain-mock service:
   ```bash
   docker compose stop chain-mock
   ```

3. Restart services that depend on the chain:
   ```bash
   docker compose restart dataset-exporter payment-bridge
   ```

Or run a local full node alongside the stack:
```bash
# The repo root Dockerfile builds zeroned
docker build -t zerone-node .
docker run -d --name zerone-node \
  --network zerone_zerone-internal \
  -p 26657:26657 \
  zerone-node start --home /root/.zeroned
```

## Development Workflow

### Rebuild a Single Service

```bash
# Rebuild and restart just the api-gateway
docker compose build api-gateway
docker compose up -d api-gateway

# Or in one step
docker compose up -d --build api-gateway
```

### View Logs

```bash
# All services
docker compose logs -f

# Single service
docker compose logs -f api-gateway

# Chain mock (see what services are calling)
docker compose logs -f chain-mock
```

### Run Training Pipeline

```bash
# Create a dataset snapshot
docker compose --profile training run training-pipeline \
  transform run --snapshot v1 --format chat --output /datasets/v1

# Run fine-tuning (requires GPU)
docker compose --profile training run training-pipeline \
  train run --config /app/configs/technical-v1.yaml
```

### Model Lifecycle (CLI, runs on host)

```bash
cd services/model-lifecycle && go build -o model-lifecycle . && cd -

# List deployed adapters
./services/model-lifecycle/model-lifecycle list --inference-url http://localhost:8000

# Deploy a new adapter with canary traffic
./services/model-lifecycle/model-lifecycle deploy \
  --adapter zerone-technical-v2 --canary 10 \
  --inference-url http://localhost:8000

# Promote to 100%
./services/model-lifecycle/model-lifecycle promote \
  --adapter zerone-technical-v2 \
  --inference-url http://localhost:8000
```

### Reset Everything

```bash
docker compose --profile cpu down -v   # stops all, removes volumes
docker compose --profile cpu up -d     # fresh start
```

## Port Reference

| Port | Service | Access |
|------|---------|--------|
| 8080 | API Gateway | 0.0.0.0 (public API) |
| 5432 | PostgreSQL | 127.0.0.1 only |
| 6379 | Redis | 127.0.0.1 only |
| 4222 | NATS | 127.0.0.1 only |
| 8222 | NATS monitoring | 127.0.0.1 only |
| 9000 | MinIO API | 127.0.0.1 only |
| 9001 | MinIO Console | 127.0.0.1 only (http://localhost:9001) |
| 9090 | Chain Mock | 127.0.0.1 only |
| 9091 | Prometheus | 127.0.0.1 only (monitoring profile) |
| 3000 | Grafana | 127.0.0.1 only (monitoring profile) |
