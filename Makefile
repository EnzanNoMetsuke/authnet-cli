GOLANGCI_LINT_VERSION := v2.12.2
LOCAL_BIN := $(CURDIR)/bin
GOLANGCI_LINT := $(LOCAL_BIN)/golangci-lint
AUTHNET_BIN ?= $(CURDIR)/bin/authnet
GOCACHE ?= $(CURDIR)/.cache/go-build
GOMODCACHE ?= $(CURDIR)/.cache/go-mod
GOLANGCI_LINT_CACHE ?= $(CURDIR)/.cache/golangci-lint

.PHONY: fmt test vet build verify golangci-lint-install golangci-lint-full install-git-hooks

fmt:
	gofmt -w cmd internal

test:
	GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" go test ./...

vet:
	GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" go vet ./...

build:
	GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" go build -o "$(AUTHNET_BIN)" ./cmd/authnet

verify: fmt test vet build golangci-lint-full

golangci-lint-install:
	./scripts/install-golangci-lint.sh "$(GOLANGCI_LINT_VERSION)" "$(LOCAL_BIN)"

golangci-lint-full:
	@test -x "$(GOLANGCI_LINT)" || { echo "golangci-lint is missing. Run: make golangci-lint-install"; exit 1; }
	GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" GOLANGCI_LINT_CACHE="$(GOLANGCI_LINT_CACHE)" "$(GOLANGCI_LINT)" run --config .golangci.yml ./...

install-git-hooks:
	./scripts/install-git-hooks.sh
