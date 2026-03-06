# T4-2 — Model Lifecycle Management

## Goal

Build the model lifecycle system: hot-swap model updates without downtime, A/B testing between model versions, and rollback capability.

## Deliverables

### 1. Hot-Swap Protocol

When a new model version is ready (from T2 training pipeline):
1. New adapter weights copied to inference server
2. vLLM loads new adapter (warm loading — no base model reload)
3. Gateway gradually routes traffic: 100% old → canary (10% new) → ramp → 100% new
4. Old adapter unloaded after drain period
5. Total swap time target: < 60 seconds for adapter, < 5 minutes for base model change

### 2. A/B Testing

- Route configurable % of traffic to model A vs model B
- Track per-model metrics: latency, token throughput, user satisfaction (if feedback endpoint exists)
- Automated promotion: if model B outperforms A on eval metrics for N hours, promote B

### 3. Rollback

- Keep last N model versions available for instant rollback
- Rollback trigger: manual CLI command or automated (error rate spike)
- Rollback is just a symlink swap + adapter reload

### 4. Lifecycle CLI

```bash
model deploy --adapter zerone-technical-v1.1.0 --canary 10
model promote --adapter zerone-technical-v1.1.0
model rollback --to zerone-technical-v1.0.0
model list --serving    # Show currently loaded models
model benchmark --adapter zerone-technical-v1.1.0 --dataset eval-set
```

### 5. Notification Integration

- On successful deployment: notify via webhook (or on-chain event)
- On rollback: alert with reason
- On eval failure: block promotion, alert

## Output

- Lifecycle management scripts/service
- Integration with T2 training pipeline (auto-deploy on training completion)
- Integration with T3 gateway (traffic routing controls)
- Runbook for manual operations
