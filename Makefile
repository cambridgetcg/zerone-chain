.PHONY: build install test lint proto-gen proto-swagger-gen proto-check creed-check clean pr-check cosmovisor-init boot-test genesis-check \
       build-linux-amd64 build-linux-arm64 build-darwin-arm64 build-all release

VERSION := $(shell git describe --tags --always 2>/dev/null || echo "dev")
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

LDFLAGS := -s -w \
           -X github.com/cosmos/cosmos-sdk/version.Name=zerone \
           -X github.com/cosmos/cosmos-sdk/version.AppName=zeroned \
           -X github.com/cosmos/cosmos-sdk/version.Version=$(VERSION) \
           -X github.com/cosmos/cosmos-sdk/version.Commit=$(COMMIT)

build:
	mkdir -p build
	go build -trimpath -ldflags "$(LDFLAGS)" -o build/zeroned ./cmd/zeroned

install:
	go install -ldflags "$(LDFLAGS)" ./cmd/zeroned

test:
	go test ./... -count=1 -timeout 300s

lint:
	go vet ./...

proto-gen:
	cd proto && buf generate

proto-swagger-gen:
	@echo "Generating Swagger from proto files..."
	cd proto && buf generate --template buf.gen.swagger.yaml
	go run scripts/merge_swagger.go
	rm -rf tmp-swagger-gen

proto-check:
	@bash scripts/proto-audit.sh

creed-check:
	@bash scripts/check_creed_hash.sh
	@bash scripts/check_useful_work_hash.sh
	@bash scripts/check_tok_substrate_hash.sh
	@bash scripts/check_strange_loop_hash.sh
	@bash scripts/check_sub_creed_hashes.sh
	@bash scripts/check_recursion_doctrine_hash.sh
	@bash scripts/check_phase_1_spec_hash.sh
	@bash scripts/check_specs_and_plans_hashes.sh

recursion-check:
	@bash scripts/recursion-check.sh

# ── Cross-compile targets ──────────────────────────────────────────────

build-linux-amd64:
	mkdir -p build
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o build/zeroned-linux-amd64 ./cmd/zeroned

build-linux-arm64:
	mkdir -p build
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o build/zeroned-linux-arm64 ./cmd/zeroned

build-darwin-arm64:
	mkdir -p build
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o build/zeroned-darwin-arm64 ./cmd/zeroned

build-all: build-linux-amd64 build-linux-arm64 build-darwin-arm64

release: build-all
	@echo "Binaries built:"
	@ls -la build/zeroned-*
	@echo ""
	@cd build && for f in zeroned-*; do shasum -a 256 "$$f" > "$$f.sha256"; done
	@echo "Checksums:"
	@cat build/*.sha256

clean:
	rm -rf build/

pr-check: lint test proto-check creed-check build
	@echo "PR check passed"

cosmovisor-init: build
	@mkdir -p cosmovisor/genesis/bin
	@mkdir -p cosmovisor/upgrades/v1.0.0-testnet/bin
	@cp build/zeroned cosmovisor/genesis/bin/zeroned
	@echo "Cosmovisor initialized with zeroned at cosmovisor/genesis/bin/"

boot-test: build
	@./scripts/boot-test.sh build/zeroned

genesis-check:
	@go run tools/genesis-check/main.go --genesis $(GENESIS)
