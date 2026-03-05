#!/usr/bin/env bash
set -euo pipefail

URL="${1:-http://localhost:8000}"

echo "Checking inference server health at $URL..."

# Health endpoint
if curl -sf "$URL/health" > /dev/null 2>&1; then
    echo "Health: OK"
else
    echo "Health: FAIL"
    exit 1
fi

# List models
echo ""
echo "Available models:"
curl -s "$URL/v1/models" | python3 -m json.tool 2>/dev/null || echo "(unable to parse)"

# GPU metrics if available
echo ""
echo "GPU utilization:"
nvidia-smi --query-gpu=name,utilization.gpu,memory.used,memory.total --format=csv,noheader 2>/dev/null || echo "(nvidia-smi not available)"
