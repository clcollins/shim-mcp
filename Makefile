unexport GOFLAGS

GOOS ?= linux
GOARCH ?= amd64
GOPATH := $(shell go env GOPATH | awk -F: '{print $1}')
BIN_DIR := $(GOPATH)/bin
HOME ?= $(shell echo ~)

GOLANGCI_LINT_VERSION = v2.12.2
GORELEASER_VERSION = v2.8.2

CONTAINER_SUBSYS ?= podman
CI_IMAGE ?= shim-mcp-ci:local
REGISTRY ?= quay.io
IMAGE_REPO ?= clcollins
IMAGE_NAME ?= shim-mcp
IMAGE_TAG ?= latest
IMAGE_REF = $(REGISTRY)/$(IMAGE_REPO)/$(IMAGE_NAME):$(IMAGE_TAG)

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS = -X github.com/clcollins/shim-mcp/internal/cli.Version=$(VERSION) \
          -X github.com/clcollins/shim-mcp/internal/cli.Commit=$(COMMIT) \
          -X github.com/clcollins/shim-mcp/internal/cli.BuildDate=$(BUILD_DATE)

export CGO_ENABLED = 0

-include .env

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help message
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

# --- Build ---

.PHONY: build
build: ## Build the application with goreleaser
	goreleaser build --snapshot --clean --single-target

.PHONY: install
install: ## Install the application to $(GOPATH)/bin
	go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/shim-mcp ./cmd/shim-mcp

.PHONY: install-local
install-local: build ## Install locally to ~/.local/bin
	cp dist/*/shim-mcp $(HOME)/.local/bin/shim-mcp

.PHONY: clean
clean: ## Clean up build artifacts
	rm -rf bin/ dist/ coverage.out

# --- Go checks (native) ---

.PHONY: tidy
tidy: ## Tidy up go modules
	go mod tidy

.PHONY: tidy-check
tidy-check: ## Verify go.mod and go.sum are tidy
	go mod tidy
	git diff --exit-code go.mod go.sum

.PHONY: fmt
fmt: ## Format the code
	gofmt -s -l -w cmd internal

.PHONY: fmt-check
fmt-check: ## Check code formatting (CI-friendly)
	@test -z "$$(gofmt -s -l cmd internal)" || (echo "Unformatted files:"; gofmt -s -l cmd internal; exit 1)

.PHONY: vet
vet: ## Run go vet
	go vet ./...

.PHONY: getlint
getlint: ## Install golangci-lint if not already installed
	@which golangci-lint >/dev/null 2>&1 || go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

.PHONY: lint
lint: getlint ## Run golangci-lint
	golangci-lint run --timeout 5m

.PHONY: test
test: ## Run unit tests
	go test ./... -v -count=1 $(TESTOPTS)

.PHONY: test-race
test-race: ## Run tests with race detector
	CGO_ENABLED=1 go test -race ./... -count=1 $(TESTOPTS)

.PHONY: coverage
coverage: ## Generate test coverage report
	go test ./... -coverprofile=coverage.out -covermode=atomic
	@echo "Coverage summary:"
	go tool cover -func=coverage.out
	@rm -f coverage.out

.PHONY: test-all
test-all: fmt-check vet lint test test-race ## Run all native Go checks
	@echo "All native checks passed."

# --- Non-Go checks (run inside CI container or with tools installed) ---

.PHONY: yaml-lint
yaml-lint: ## Lint YAML files
	yamllint -c .yamllint.yaml .

.PHONY: markdown-lint
markdown-lint: ## Lint Markdown files
	markdownlint-cli2 "**/*.md" "#vendor" "#node_modules"

.PHONY: makefile-lint
makefile-lint: ## Lint Makefile
	checkmake Makefile

.PHONY: docs-check
docs-check: ## Verify plan documents exist in docs/plans/
	@test -n "$$(find docs/plans/ -name '*.md' -not -name 'TEMPLATE.md')" || \
		(echo "ERROR: No plan documents found in docs/plans/"; exit 1)
	@echo "Plan document(s) found."

.PHONY: containerfile-check
containerfile-check: ## Validate Containerfile base image tags and registries
	.github/scripts/check-containerfile-tags.sh Containerfile test/Containerfile.ci

# --- Containerized CI ---

.PHONY: ci-build
ci-build: ## Build the CI container image
	$(CONTAINER_SUBSYS) build \
		--tag $(CI_IMAGE) \
		--file test/Containerfile.ci .

.PHONY: ci-checks
ci-checks: tidy-check fmt-check vet lint test yaml-lint markdown-lint docs-check containerfile-check ## Run all CI checks (inside CI container)

.PHONY: ci-all
ci-all: ci-build ## Build CI container and run all checks inside it
	$(CONTAINER_SUBSYS) run --rm \
		-v $(CURDIR):/src:Z \
		-w /src \
		$(CI_IMAGE) \
		make ci-checks

# --- Container image ---

.PHONY: image-build
image-build: ## Build the application container image
	$(CONTAINER_SUBSYS) build \
		--tag $(IMAGE_REF) \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--file Containerfile .

# --- Release ---

.PHONY: ensure-goreleaser
ensure-goreleaser: ## Ensure goreleaser is installed
	@which goreleaser >/dev/null 2>&1 || go install github.com/goreleaser/goreleaser/v2@$(GORELEASER_VERSION)

.PHONY: release
release: ensure-goreleaser ## Create a release using goreleaser
	GITHUB_TOKEN=$$(jq -r .goreleaser_token ~/.config/goreleaser/goreleaser_token) && \
	export GITHUB_TOKEN && \
	goreleaser release --clean --parallelism 1 $(if $(RELEASE_NOTES),--release-notes $(RELEASE_NOTES),)
