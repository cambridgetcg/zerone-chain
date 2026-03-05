#!/usr/bin/env bash
set -euo pipefail

URL="${1:-http://localhost:8000}"
CONCURRENCY="${2:-4}"
REQUESTS="${3:-20}"

echo "Benchmarking inference server at $URL"
echo "Concurrency: $CONCURRENCY, Requests: $REQUESTS"
echo ""

# Simple benchmark: measure time-to-first-token and total latency
for i in $(seq 1 "$REQUESTS"); do
    START=$(date +%s%N)
    RESPONSE=$(curl -s -w "\n%{time_total}" "$URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -d '{
            "model": "zerone-8b",
            "messages": [{"role": "user", "content": "What is 2+2?"}],
            "max_tokens": 50,
            "temperature": 0
        }' 2>/dev/null)
    END=$(date +%s%N)

    LATENCY_MS=$(( (END - START) / 1000000 ))
    echo "Request $i: ${LATENCY_MS}ms"
done &

wait
echo ""
echo "Benchmark complete."
