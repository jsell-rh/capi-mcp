# Variables
BINARY_NAME := capi-mcp-server
VERSION ?= v0.1.0
BUILD_DIR := ./build
GO := go
GOFLAGS := -v
LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.buildDate=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)"

# Directories
CMD_DIR := ./cmd/server
INTERNAL_DIR := ./internal
PKG_DIR := ./pkg
TEST_DIR := ./test

# Tools
GOLANGCI_LINT_VERSION := v1.62.2

.PHONY: all build clean test lint fmt vet deps tools help

all: clean lint test build ## Run all targets

build: ## Build the binary
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)

clean: ## Clean build artifacts
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@$(GO) clean -testcache

test: ## Run unit tests
	@echo "Running tests..."
	$(GO) test $(GOFLAGS) -race -coverprofile=coverage.out -tags="!e2e" ./...

test-integration: ## Run integration tests
	@echo "Running integration tests..."
	$(GO) test $(GOFLAGS) -tags=integration $(TEST_DIR)/integration/...

test-e2e: ## Run end-to-end tests
	@echo "Running e2e tests..."
	$(GO) test $(GOFLAGS) -tags=e2e $(TEST_DIR)/e2e/...

lint: ## Run linters
	@echo "Running linters..."
	@echo "Formatting code..."
	$(GO) fmt ./...
	@echo "Checking format..."
	@if [ "$$(gofmt -s -l . | wc -l)" -gt 0 ]; then \
		echo "Code is not formatted properly:"; \
		gofmt -s -l .; \
		exit 1; \
	fi
	@echo "Running go vet..."
	$(GO) vet ./...

fmt: ## Format code
	@echo "Formatting code..."
	$(GO) fmt ./...

vet: ## Run go vet
	@echo "Running go vet..."
	$(GO) vet ./...

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	$(GO) mod download
	$(GO) mod tidy

tools: ## Install development tools
	@echo "Installing development tools..."
	@echo "Installing golangci-lint..."
	@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin $(GOLANGCI_LINT_VERSION)

docker-build: ## Build Docker image
	@echo "Building Docker image..."
	docker build -t $(BINARY_NAME):$(VERSION) -t $(BINARY_NAME):latest .

run: build ## Run the server locally
	@echo "Running server..."
	$(BUILD_DIR)/$(BINARY_NAME)

help: ## Display this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help