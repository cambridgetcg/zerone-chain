# R24-4 — Docker & Cross-Compilation: Reproducible Builds

## Context

No Dockerfile exists. No cross-compilation targets. Every operator must build from source on their target machine. This is the single biggest barrier to external participation — most VPS operators want a binary or a Docker image, not a Go toolchain setup.

## Task

### 1. Cross-Compilation Targets

Add to `Makefile`:

```makefile
# ── Cross-compile targets ──────────────────────────────────────────────

build-linux-amd64:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o build/zeroned-linux-amd64 ./cmd/zeroned

build-linux-arm64:
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o build/zeroned-linux-arm64 ./cmd/zeroned

build-darwin-arm64:
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o build/zeroned-darwin-arm64 ./cmd/zeroned

build-all: build-linux-amd64 build-linux-arm64 build-darwin-arm64

release: build-all
	@echo "Binaries built:"
	@ls -la build/zeroned-*
	@echo ""
	@cd build && for f in zeroned-*; do sha256sum "$$f" > "$$f.sha256"; done
	@echo "Checksums:"
	@cat build/*.sha256
```

**Verify:**
- [ ] `make build-linux-amd64` produces a static binary
- [ ] Binary runs on Linux amd64 (test in Docker or on a VPS)
- [ ] `CGO_ENABLED=0` works without issues (some Cosmos SDK deps may need CGO)
- [ ] If CGO is required, document it and provide alternative (Docker build)

**Issues to look for:**
- `CGO_ENABLED=0` may fail if the app uses `github.com/cosmos/ledger-cosmos-go` or SQLite
- If CGO is needed, cross-compilation becomes harder — Docker is the answer
- File sizes — how big is the binary? Compress with `upx` if > 100MB?

### 2. Dockerfile

Create `Dockerfile`:

```dockerfile
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

ENTRYPOINT ["zeroned"]
CMD ["start"]
```

**Verify:**
- [ ] `docker build -t zerone:latest .` succeeds
- [ ] `docker run zerone:latest version` returns correct version
- [ ] Image size is reasonable (< 200MB for runtime)
- [ ] Binary inside container works (test with `docker run zerone:latest init test-chain`)

### 3. Docker Compose for Development

Create `docker-compose.yml`:

```yaml
version: '3.8'

services:
  zeroned:
    build: .
    container_name: zerone-node
    ports:
      - "26656:26656"  # P2P
      - "26657:26657"  # RPC
      - "1317:1317"    # REST
      - "9090:9090"    # gRPC
    volumes:
      - zeroned-data:/root/.zeroned
    command: start --home /root/.zeroned
    restart: unless-stopped

volumes:
  zeroned-data:
```

**Verify:**
- [ ] `docker compose up` boots a node
- [ ] State persists across restarts (volume mount)
- [ ] Ports accessible from host

### 4. Multi-Stage Validator Image

Create `Dockerfile.validator` (includes Cosmovisor):

```dockerfile
FROM golang:1.24-bookworm AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN make build
RUN go install cosmossdk.io/tools/cosmovisor/cmd/cosmovisor@latest

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates curl jq \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/build/zeroned /usr/local/bin/zeroned
COPY --from=builder /go/bin/cosmovisor /usr/local/bin/cosmovisor

ENV DAEMON_NAME=zeroned
ENV DAEMON_HOME=/root/.zeroned
ENV DAEMON_ALLOW_DOWNLOAD_BINARIES=true
ENV DAEMON_RESTART_AFTER_UPGRADE=true

EXPOSE 26656 26657 1317 9090

ENTRYPOINT ["cosmovisor"]
CMD ["run", "start"]
```

### 5. GitHub Actions / CI (Optional)

If Codeberg supports CI, create `.woodpecker.yml` or `.github/workflows/release.yml`:

```yaml
name: Build & Release
on:
  push:
    tags: ['v*']

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.24'
      - run: make test
      - run: make build-all
      - run: make release
```

### 6. Reproducible Build Verification

```bash
# Build twice, compare SHA256
make clean && make build-linux-amd64
sha256sum build/zeroned-linux-amd64 > /tmp/build1.sha256

make clean && make build-linux-amd64
sha256sum build/zeroned-linux-amd64 > /tmp/build2.sha256

diff /tmp/build1.sha256 /tmp/build2.sha256
```

**Verify:**
- [ ] Same source + same Go version = same binary hash
- [ ] If not reproducible: identify why (timestamps? build paths?) and fix with `-trimpath`

Add to LDFLAGS:
```makefile
LDFLAGS := -X ... -s -w  # strip debug symbols
```
And build flag:
```makefile
go build -trimpath -ldflags "$(LDFLAGS)" ...
```

### 7. Documentation

Update `docs/VALIDATOR-GUIDE.md` with Docker options:

```markdown
### Option B: Docker (easiest)

docker pull ghcr.io/zerone-chain/zerone:latest
docker run -v ~/.zeroned:/root/.zeroned zerone:latest init my-node

### Option C: Pre-built binary

Download from releases:
curl -L https://github.com/zerone-chain/zerone/releases/download/v0.1.0/zeroned-linux-amd64 -o zeroned
chmod +x zeroned
sudo mv zeroned /usr/local/bin/
```

## Exit Criteria

1. `make build-linux-amd64` produces a working binary
2. Dockerfile builds and `docker run zerone:latest version` works
3. Docker Compose boots a node with persistent storage
4. Validator image includes Cosmovisor
5. Builds are reproducible (same hash across builds)
6. VALIDATOR-GUIDE updated with Docker/binary options
7. All new files committed

## Commit Convention

```
feat(build): cross-compilation targets for linux/darwin amd64/arm64
feat(docker): Dockerfile + docker-compose for node deployment
feat(docker): validator image with Cosmovisor
docs(validator): add Docker and pre-built binary installation options
```
