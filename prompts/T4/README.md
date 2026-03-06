# T4 — Inference Server & Model Serving

**Goal:** Deploy and configure the inference server stack — vLLM serving fine-tuned models with hot-swap capability for continuous model updates.

## Sessions (2)

| # | File | Scope |
|---|------|-------|
| T4-1 | T4-1-inference-server.md | vLLM deployment, model loading, streaming, health checks |
| T4-2 | T4-2-model-lifecycle.md | Hot-swap model updates, A/B testing, rollback, multi-model serving |

## Run Order

Sequential: T4-1 → T4-2

## Prerequisites

- T2 (trained model available)
- T3 (API gateway to route requests)

## Exit Criteria

1. vLLM serves fine-tuned model with OpenAI-compatible API
2. Streaming responses work end-to-end
3. Model hot-swap completes without downtime
4. Multi-model serving routes by model ID
