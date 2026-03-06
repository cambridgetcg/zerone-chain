# T4-1 — Inference Server Setup

## Goal

Deploy vLLM to serve fine-tuned ZERONE models with OpenAI-compatible API, optimized for throughput and latency.

## Deliverables

### 1. vLLM Configuration

Docker Compose setup:
```yaml
services:
  inference:
    image: vllm/vllm-openai:latest
    runtime: nvidia
    environment:
      - MODEL_PATH=/models/active/
      - MAX_MODEL_LEN=4096
      - GPU_MEMORY_UTILIZATION=0.90
      - TENSOR_PARALLEL_SIZE=1
    volumes:
      - ./models:/models
    ports:
      - "8000:8000"  # Internal only — gateway is the public face
```

### 2. Model Loading

- Load LoRA adapters on top of base model (vLLM supports this natively)
- Support both merged models and adapter-only loading
- Model directory structure:
  ```
  /models/
  ├── base/
  │   └── Llama-3.1-8B-Instruct/
  ├── adapters/
  │   ├── zerone-technical-v1.0.0/
  │   └── zerone-general-v1.0.0/
  └── active -> adapters/zerone-technical-v1.0.0
  ```

### 3. Performance Tuning

- Continuous batching (vLLM default)
- PagedAttention for efficient memory
- Quantized inference (AWQ/GPTQ if using quantized weights)
- Benchmark: tokens/sec, time-to-first-token, throughput under concurrent load

### 4. Health & Monitoring

- Health check endpoint (vLLM built-in)
- Prometheus metrics export
- GPU utilization monitoring
- Request queue depth tracking

### 5. Multi-Model Serving

- vLLM can serve multiple LoRA adapters on the same base model
- Map model IDs to adapters: `zerone-technical` → technical adapter, `zerone-general` → general adapter
- Gateway routes by requested model name

## Working Directory

`/Users/yournameisai/Desktop/zerone/services/inference/`

## Output

- Docker Compose file for inference server
- Deployment scripts (start, stop, health check)
- Benchmark results on test hardware
- README with GPU requirements
