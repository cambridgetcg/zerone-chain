#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR/.."

# Ensure model directory structure exists
mkdir -p models/base models/adapters

# Check for base model
if [ ! -d "models/base/Llama-3.1-8B-Instruct" ]; then
    echo "Base model not found at models/base/Llama-3.1-8B-Instruct"
    echo "Download with: huggingface-cli download meta-llama/Llama-3.1-8B-Instruct --local-dir models/base/Llama-3.1-8B-Instruct"
    exit 1
fi

# Set active adapter symlink if not set
if [ ! -L "models/active" ] && [ -d "models/adapters" ]; then
    FIRST_ADAPTER=$(ls models/adapters/ 2>/dev/null | head -1)
    if [ -n "$FIRST_ADAPTER" ]; then
        ln -sf "adapters/$FIRST_ADAPTER" models/active
        echo "Active adapter set to: $FIRST_ADAPTER"
    fi
fi

echo "Starting inference server..."
docker compose up -d inference

echo "Waiting for health check..."
for i in $(seq 1 60); do
    if curl -sf http://localhost:8000/health > /dev/null 2>&1; then
        echo "Inference server is healthy!"
        exit 0
    fi
    sleep 5
done

echo "Timeout waiting for inference server to become healthy"
docker compose logs inference | tail -20
exit 1
