# ── Build stage ──────────────────────────────────────────────────────────
FROM golang:1.24-bookworm AS builder

WORKDIR /app

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Build
COPY . .
RUN make build

# ── Runtime stage ────────────────────────────────────────────────────────
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates curl jq \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/build/zeroned /usr/local/bin/zeroned

# Default ports: P2P=26656, RPC=26657, REST=1317, gRPC=9090
EXPOSE 26656 26657 1317 9090

CMD ["zeroned", "start"]
