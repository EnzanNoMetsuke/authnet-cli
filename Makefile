GOLANGCI_LINT_VERSION := v2.12.2
LOCAL_BIN := $(CURDIR)/bin
GOLANGCI_LINT := $(LOCAL_BIN)/golangci-lint
AUTHNET_BIN ?= $(CURDIR)/bin/authnet
VERSION ?= 0.0.0-dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || printf unknown)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
SCHEMA_VERSION ?= 0.1.0
CONTRACT_STATUS ?= pre-release
LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(BUILD_DATE) -X main.schemaVersion=$(SCHEMA_VERSION) -X main.contractStatus=$(CONTRACT_STATUS)
GOCACHE ?= $(CURDIR)/.cache/go-build
GOMODCACHE ?= $(CURDIR)/.cache/go-mod
GOLANGCI_LINT_CACHE ?= $(CURDIR)/.cache/golangci-lint

.PHONY: fmt test vet build release-snapshot release-check docs-check verify golangci-lint-install golangci-lint-full install-git-hooks

fmt:
	gofmt -w cmd internal

test:
	GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" go test ./...

vet:
	GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" go vet ./...

build:
	GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" go build -ldflags "$(LDFLAGS)" -o "$(AUTHNET_BIN)" ./cmd/authnet

release-snapshot:
	goreleaser release --snapshot --clean

release-check:
	goreleaser check

docs-check:
	./scripts/check-docs.sh

verify: fmt test vet build docs-check golangci-lint-full

golangci-lint-install:
	./scripts/install-golangci-lint.sh "$(GOLANGCI_LINT_VERSION)" "$(LOCAL_BIN)"

golangci-lint-full:
	@test -x "$(GOLANGCI_LINT)" || { echo "golangci-lint is missing. Run: make golangci-lint-install"; exit 1; }
	GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" GOLANGCI_LINT_CACHE="$(GOLANGCI_LINT_CACHE)" "$(GOLANGCI_LINT)" run --config .golangci.yml ./...

install-git-hooks:
	./scripts/install-git-hooks.sh
