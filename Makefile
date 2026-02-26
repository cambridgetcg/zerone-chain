.PHONY: build install test lint proto-gen proto-swagger-gen clean pr-check cosmovisor-init boot-test genesis-check

VERSION := $(shell git describe --tags --always 2>/dev/null || echo "dev")
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

LDFLAGS := -X github.com/cosmos/cosmos-sdk/version.Name=zerone \
           -X github.com/cosmos/cosmos-sdk/version.AppName=zeroned \
           -X github.com/cosmos/cosmos-sdk/version.Version=$(VERSION) \
           -X github.com/cosmos/cosmos-sdk/version.Commit=$(COMMIT)

build:
	mkdir -p build
	go build -ldflags "$(LDFLAGS)" -o build/zeroned ./cmd/zeroned

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

clean:
	rm -rf build/

pr-check: lint test build
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
