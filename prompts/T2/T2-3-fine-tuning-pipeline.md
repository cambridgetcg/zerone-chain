# T2-3 — Fine-Tuning Pipeline

## Goal

Build the automated fine-tuning pipeline that takes transformed training data and produces fine-tuned model weights ready for serving. Support LoRA/QLoRA for efficient fine-tuning on consumer/prosumer GPUs.

## Deliverables

### 1. Training Configuration

YAML-based training configs:
```yaml
name: "zerone-technical-v1"
base_model: "meta-llama/Llama-3.1-8B-Instruct"
method: "qlora"  # lora, qlora, full
dataset:
  path: "/data/training/v1.0.0/"
  format: "chat"  # chat, completion, dpo
hyperparameters:
  learning_rate: 2e-4
  batch_size: 4
  gradient_accumulation_steps: 8
  epochs: 3
  max_seq_length: 4096
  warmup_ratio: 0.03
lora:
  r: 64
  alpha: 128
  dropout: 0.05
  target_modules: ["q_proj", "k_proj", "v_proj", "o_proj", "gate_proj", "up_proj", "down_proj"]
quantization:
  bits: 4
  quant_type: "nf4"
  double_quant: true
output_dir: "/models/zerone-technical-v1/"
```

### 2. Training Runner

Python script using HuggingFace transformers + PEFT + bitsandbytes:
- Load base model with quantization config
- Apply LoRA adapters
- Train on transformed dataset
- Save adapter weights + merged model
- Log training metrics (loss, learning rate, grad norm) to W&B or local files
- Checkpoint every N steps (resumable training)

### 3. Model Registry

Local registry tracking all trained models:
```json
{
  "model_id": "zerone-technical-v1.0.0",
  "base_model": "meta-llama/Llama-3.1-8B-Instruct",
  "dataset_version": "v1.0.0",
  "method": "qlora",
  "domain_focus": "technical",
  "training_config": "configs/technical-v1.yaml",
  "metrics": {
    "final_loss": 0.42,
    "eval_loss": 0.45,
    "perplexity": 1.56
  },
  "benchmark_scores": {},
  "weights_path": "/models/zerone-technical-v1/",
  "created_at": "2026-03-05T12:00:00Z",
  "status": "ready"  // training, ready, serving, archived
}
```

### 4. Evaluation Suite

After training, automatically evaluate:
- **Perplexity** on held-out test set
- **Domain-specific benchmarks**: Does the model know what the ToK taught it?
- **Regression check**: Compare against base model on standard benchmarks (MMLU, etc.) to ensure fine-tuning didn't degrade general capability
- **A/B comparison**: If previous fine-tuned version exists, compare new vs old

### 5. Continuous Training Loop

Design for automation:
1. Dataset exporter creates new snapshot (triggered by N new approved samples)
2. Transform pipeline generates training data
3. Fine-tuning runs with incremental training (continue from last checkpoint, not from scratch)
4. Evaluation runs automatically
5. If eval passes thresholds → model promoted to "ready"
6. Inference server notified to hot-swap to new model

### 6. CLI

```bash
train run --config configs/technical-v1.yaml
train resume --checkpoint /models/zerone-technical-v1/checkpoint-5000/
train eval --model zerone-technical-v1.0.0
train promote --model zerone-technical-v1.0.0  # Mark as ready for serving
train list                                      # Show all models in registry
```

## Output

- Python package at `services/training-pipeline/`
- Example training configs at `services/training-pipeline/configs/`
- Dockerfile with CUDA support for GPU training
- README with hardware requirements and usage
