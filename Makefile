.PHONY: build install test lint proto-gen clean pr-check

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

clean:
	rm -rf build/

pr-check: lint test build
	@echo "PR check passed"
