#!/usr/bin/env bash
set -euo pipefail

if [ $# -lt 1 ]; then
    echo "Usage: $0 <adapter-name>"
    echo ""
    echo "Available adapters:"
    ls -1 "$(dirname "$0")/../models/adapters/" 2>/dev/null || echo "  (none)"
    exit 1
fi

ADAPTER_NAME="$1"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR/.."

ADAPTER_DIR="models/adapters/$ADAPTER_NAME"
if [ ! -d "$ADAPTER_DIR" ]; then
    echo "Adapter not found: $ADAPTER_DIR"
    exit 1
fi

echo "Switching active adapter to: $ADAPTER_NAME"
rm -f models/active
ln -sf "adapters/$ADAPTER_NAME" models/active

echo "Restarting inference server..."
docker compose restart inference

echo "Waiting for health check..."
for i in $(seq 1 60); do
    if curl -sf http://localhost:8000/health > /dev/null 2>&1; then
        echo "Model swap complete. Active adapter: $ADAPTER_NAME"
        exit 0
    fi
    sleep 5
done

echo "Timeout waiting for inference server after model swap"
exit 1
