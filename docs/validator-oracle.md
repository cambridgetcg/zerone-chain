# Validator Evaluation Oracle

## What It Does

The **zerone-oracle** is an advisory sidecar process that helps validators make informed decisions when voting on knowledge claims. Instead of blindly accepting all claims, validators can consult the oracle for a fact-checking verdict before casting their vote.

**Important:** The oracle is purely advisory. It does not participate in consensus. If the oracle is down, slow, or misconfigured, the validator falls back to default behavior (accept all claims). Oracle failure never blocks the chain.

## How It Works

The oracle provides two evaluation tiers:

### Tier 1: Static (No External Dependencies)

Checks new claims against the 777 genesis axioms embedded in the binary:
- **Numerical contradiction:** Extracts numbers from the claim and matching axioms, flags mismatches (e.g., "speed of light is 100 m/s" contradicts the axiom stating ~3x10^8 m/s)
- **Explicit negation:** Detects negation words ("not", "never", "false") that reverse the meaning of a matching axiom

**Limitations:** Tier 1 does NOT catch semantic contradictions (e.g., "the sun orbits the earth"), paraphrased falsehoods, or claims in domains with no matching axioms. It is a simple filter, not a general fact-checker.

### Tier 2: LLM (Requires API Key)

Sends the claim to Claude for fact-checking with a structured prompt. Returns a verdict with confidence and reasoning. Results are cached (LRU, 1000 entries) to avoid repeated API calls.

**Timeout:** LLM calls have a strict 2-second timeout. If Claude doesn't respond in time, the oracle returns "uncertain."

## Quick Start

### 1. Build the oracle

```bash
cd /path/to/zerone
go build -o build/zerone-oracle ./cmd/oracle/
```

### 2. Start the sidecar (Tier 1 only)

```bash
./build/zerone-oracle --port 8081 --tier static
```

### 3. Start the sidecar (with LLM)

```bash
./build/zerone-oracle --port 8081 --tier llm \
  --llm-api-key "sk-ant-..." \
  --llm-model "claude-sonnet-4-5-20250514"
```

### 4. Enable in validator config

Add to your `app.toml`:

```toml
[oracle]
enabled = true
endpoint = "http://localhost:8081"
timeout = "2s"
min-confidence = 0.6
```

### 5. Restart your validator

The validator will now query the oracle during vote extension commit phase.

## Configuration Reference

### Oracle Sidecar Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--port` | `8081` | HTTP server port |
| `--tier` | `static` | Evaluation tier: `static` or `llm` |
| `--llm-api-key` | (empty) | Anthropic API key (required for `llm` tier) |
| `--llm-api-url` | `https://api.anthropic.com` | Anthropic API base URL |
| `--llm-model` | `claude-sonnet-4-5-20250514` | Model ID for LLM evaluation |
| `--llm-max-tokens` | `500` | Max response tokens |
| `--llm-timeout` | `2s` | LLM API call timeout |

### Validator app.toml

| Key | Default | Description |
|-----|---------|-------------|
| `oracle.enabled` | `false` | Enable oracle queries |
| `oracle.endpoint` | `http://localhost:8081` | Oracle sidecar URL |
| `oracle.timeout` | `2s` | Max wait for oracle response |
| `oracle.min-confidence` | `0.6` | Minimum confidence to act on verdict |

## API Endpoints

### POST /evaluate

Evaluate a claim.

```json
Request:  {"claim": "Water boils at 100C", "domain": "physics", "claim_type": "assertion"}
Response: {"verdict": "accept", "confidence": 0.75, "reasoning": "consistent with axiom PHYS-..."}
```

Verdicts: `accept`, `reject`, `uncertain`

Confidence: 0.0 to 1.0

### POST /prefetch

Pre-warm the cache for an upcoming evaluation. Same request body as `/evaluate`. Returns immediately (HTTP 202). The evaluation runs in the background and caches the result for a subsequent `/evaluate` call.

### GET /health

Health check.

```json
Response: {"status": "ok", "tier": "static"}
```

## Safety Guarantees

- **Oracle down:** Validator falls back to `accept` with default confidence (600,000 BPS). No consensus impact.
- **Oracle slow:** 2-second hard timeout. Falls back to default.
- **Oracle wrong:** Verdicts below the confidence threshold (default 0.6) are treated as "uncertain," which maps to accept.
- **Oracle disabled:** `oracle.enabled = false` (default). Zero behavior change from pre-oracle code.
- **Oracle crash:** The sidecar is a separate process. A crash in the oracle cannot affect the validator node.

## Performance

- Static evaluation: <1ms per claim
- LLM evaluation: 500ms-2s (first call), <1ms (cache hit)
- Cache pre-warming via `/prefetch` recommended for LLM tier
- Localhost HTTP overhead: ~1ms

## How the Combine Logic Works

When both tiers are active (`--tier llm`):

1. Static evaluation always runs first
2. If static returns a high-confidence verdict (>0.7, accept or reject), it short-circuits
3. If static is uncertain and LLM is available, the LLM result is used
4. If the LLM fails or times out, the static result is used
5. If only static is active (`--tier static`), the LLM step is skipped entirely

The validator applies an additional confidence threshold (default 0.6). Any verdict below this threshold is treated as "uncertain" and mapped to the default accept behavior. This protects validators from being slashed based on weak oracle signals.
