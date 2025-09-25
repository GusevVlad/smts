# SMTS Makefile

# Build configuration
BIN_DIR := bin
EXT_BINARY := $(BIN_DIR)/ext-smts
INT_BINARY := $(BIN_DIR)/int-smts
TEST_BINARY := $(BIN_DIR)/smts-test

# Go configuration
GO := go
GO_MODULE := github.com/corporate/smts
GO_BUILD_FLAGS := -v
GO_TEST_FLAGS := -v -race -cover
GO_LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

# Version information
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Test configuration
TEST_TIMEOUT := 10m
UNIT_TEST_PATTERN := ./tests/unit/...
INTEGRATION_TEST_PATTERN := ./tests/integration/...
E2E_TEST_PATTERN := ./tests/e2e/...

# Default target
.DEFAULT_GOAL := help

.PHONY: help
help: ## Display this help message
	@echo "SMTS Build and Test Targets"
	@echo ""
	@echo "Usage:"
	@echo "  make <target>"
	@echo ""
	@echo "Targets:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Build targets
.PHONY: build
build: build-ext build-int ## Build both EXT and INT SMTS binaries

.PHONY: build-ext
build-ext: ## Build EXT SMTS binary
	@echo "Building EXT SMTS..."
	@mkdir -p $(BIN_DIR)
	$(GO) build $(GO_BUILD_FLAGS) -ldflags "$(GO_LDFLAGS)" -o $(EXT_BINARY) ./cmd/ext-smts

.PHONY: build-int
build-int: ## Build INT SMTS binary
	@echo "Building INT SMTS..."
	@mkdir -p $(BIN_DIR)
	$(GO) build $(GO_BUILD_FLAGS) -ldflags "$(GO_LDFLAGS)" -o $(INT_BINARY) ./cmd/int-smts

.PHONY: clean
clean: ## Clean build artifacts
	@echo "Cleaning build artifacts..."
	@rm -rf $(BIN_DIR)
	@rm -rf coverage.out
	@rm -rf test-results

# Test targets
.PHONY: test
test: test-unit test-integration ## Run all tests

.PHONY: test-unit
test-unit: ## Run unit tests
	@echo "Running unit tests..."
	@mkdir -p test-results
	$(GO) test $(GO_TEST_FLAGS) -timeout=$(TEST_TIMEOUT) -coverprofile=test-results/unit-coverage.out $(UNIT_TEST_PATTERN)

.PHONY: test-integration
test-integration: ## Run integration tests
	@echo "Running integration tests..."
	@mkdir -p test-results
	$(GO) test $(GO_TEST_FLAGS) -timeout=$(TEST_TIMEOUT) -coverprofile=test-results/integration-coverage.out $(INTEGRATION_TEST_PATTERN)

.PHONY: test-coverage
test-coverage: ## Generate test coverage report
	@echo "Generating coverage report..."
	@mkdir -p test-results
	$(GO) test $(GO_TEST_FLAGS) -coverprofile=test-results/coverage.out ./...
	$(GO) tool cover -html=test-results/coverage.out -o test-results/coverage.html
	@echo "Coverage report generated: test-results/coverage.html"

.PHONY: test-race
test-race: ## Run tests with race detector
	@echo "Running tests with race detector..."
	$(GO) test -race -timeout=$(TEST_TIMEOUT) ./...

.PHONY: test-bench
test-bench: ## Run benchmark tests
	@echo "Running benchmark tests..."
	$(GO) test -bench=. -benchmem ./...

# Development targets
.PHONY: deps
deps: ## Download dependencies
	@echo "Downloading dependencies..."
	$(GO) mod download
	$(GO) mod verify

.PHONY: tidy
tidy: ## Tidy go.mod
	@echo "Tidying go.mod..."
	$(GO) mod tidy

.PHONY: vet
vet: ## Run go vet
	@echo "Running go vet..."
	$(GO) vet ./...

.PHONY: lint
lint: ## Run golangci-lint
	@echo "Running golangci-lint..."
	@if command -v golangci-lint >/dev/null; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed, skipping..."; \
		echo "Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

.PHONY: fmt
fmt: ## Format code
	@echo "Formatting code..."
	$(GO) fmt ./...

.PHONY: check
check: vet lint ## Run all code quality checks

# Run targets
.PHONY: run-ext
run-ext: build-ext ## Run EXT SMTS
	@echo "Running EXT SMTS..."
	$(EXT_BINARY) --config configs/ext-config.yaml

.PHONY: run-int
run-int: build-int ## Run INT SMTS
	@echo "Running INT SMTS..."
	$(INT_BINARY) --config configs/int-config.yaml

.PHONY: run-ext-test
run-ext-test: build-ext ## Run EXT SMTS with test config
	@echo "Running EXT SMTS with test config..."
	$(EXT_BINARY) --config tests/configs/ext-test-config.yaml

.PHONY: run-int-test
run-int-test: build-int ## Run INT SMTS with test config
	@echo "Running INT SMTS with test config..."
	$(INT_BINARY) --config tests/configs/int-test-config.yaml

# Docker targets
.PHONY: docker-build
docker-build: ## Build Docker images
	@echo "Building Docker images..."
	docker build -t smts-ext:latest -f Dockerfile --build-arg BINARY=ext-smts .
	docker build -t smts-int:latest -f Dockerfile --build-arg BINARY=int-smts .

.PHONY: docker-test
docker-test: docker-test-unit docker-test-integration ## Run all tests in Docker

.PHONY: docker-test-unit
docker-test-unit: ## Run unit tests in Docker
	@echo "Running unit tests in Docker..."
	docker build -t smts-test-unit:latest -f Dockerfile.test --target test-builder .
	docker run --rm smts-test-unit:latest /app/bin/run-unit-tests.sh

.PHONY: docker-test-integration
docker-test-integration: ## Run integration tests in Docker
	@echo "Running integration tests in Docker..."
	docker build -t smts-test-integration:latest -f Dockerfile.test --target test-builder .
	docker run --rm smts-test-integration:latest /app/bin/run-integration-tests.sh

.PHONY: docker-test-ext
docker-test-ext: ## Run EXT SMTS integration tests in Docker
	@echo "Running EXT SMTS integration tests in Docker..."
	docker build -t smts-test-ext:latest -f Dockerfile.test --target test-builder .
	docker run --rm smts-test-ext:latest /app/bin/run-ext-tests.sh

.PHONY: docker-test-int
docker-test-int: ## Run INT SMTS integration tests in Docker
	@echo "Running INT SMTS integration tests in Docker..."
	docker build -t smts-test-int:latest -f Dockerfile.test --target test-builder .
	docker run --rm smts-test-int:latest /app/bin/run-int-tests.sh

.PHONY: docker-test-full
docker-test-full: ## Run complete test suite in Docker
	@echo "Running complete test suite in Docker..."
	docker build -t smts-test-full:latest -f Dockerfile.test .
	docker run --rm smts-test-full:latest

# Docker Compose test targets
.PHONY: docker-compose-test
docker-compose-test: docker-compose-test-up docker-compose-test-down ## Run tests with docker-compose

.PHONY: docker-compose-test-up
docker-compose-test-up: ## Start test services and run tests
	@echo "Starting test services with docker-compose..."
	docker-compose -f docker-compose.test.yml up --build --abort-on-container-exit --exit-code-from smts-test-runner

.PHONY: docker-compose-test-down
docker-compose-test-down: ## Stop and remove test services
	@echo "Stopping test services..."
	docker-compose -f docker-compose.test.yml down -v

.PHONY: docker-compose-test-ext
docker-compose-test-ext: ## Run EXT SMTS tests with docker-compose
	@echo "Running EXT SMTS tests with docker-compose..."
	docker-compose -f docker-compose.test.yml up --build --abort-on-container-exit smts-ext-test

.PHONY: docker-compose-test-int
docker-compose-test-int: ## Run INT SMTS tests with docker-compose
	@echo "Running INT SMTS tests with docker-compose..."
	docker-compose -f docker-compose.test.yml up --build --abort-on-container-exit smts-int-test

.PHONY: docker-compose-test-services
docker-compose-test-services: ## Start test services only (for manual testing)
	@echo "Starting test services..."
	docker-compose -f docker-compose.test.yml up -d --build nats-test artemis-test api-mock dlp-mock

.PHONY: docker-compose-test-clean
docker-compose-test-clean: ## Clean up test services and volumes
	@echo "Cleaning up test services..."
	docker-compose -f docker-compose.test.yml down -v --rmi local

# CI/CD targets
.PHONY: ci-setup
ci-setup: deps tidy ## Setup for CI
	@echo "CI setup complete"

.PHONY: ci-test
ci-test: test check ## Run CI tests
	@echo "CI tests complete"

.PHONY: ci-build
ci-build: build check ## CI build
	@echo "CI build complete"

# Utility targets
.PHONY: version
version: ## Display version information
	@echo "Version: $(VERSION)"
	@echo "Commit: $(COMMIT)"
	@echo "Build Date: $(DATE)"

.PHONY: generate
generate: ## Generate code
	@echo "Generating code..."
	$(GO) generate ./...

.PHONY: docs
docs: ## Generate documentation
	@echo "Generating documentation..."
	@if command -v godoc >/dev/null; then \
		godoc -http=:6060 & \
		echo "Documentation server started at http://localhost:6060"; \
		echo "Press Ctrl+C to stop"; \
		wait; \
	else \
		echo "godoc not installed, skipping..."; \
	fi

# Test data targets
.PHONY: test-data
test-data: ## Generate test data
	@echo "Generating test data..."
	@mkdir -p test-data
	@echo "Test data generation complete"

.PHONY: clean-test-data
clean-test-data: ## Clean test data
	@echo "Cleaning test data..."
	@rm -rf test-data

# Install dependencies for development
.PHONY: dev-deps
dev-deps: ## Install development dependencies
	@echo "Installing development dependencies..."
	$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	$(GO) install github.com/stretchr/testify/assert@latest
	$(GO) install golang.org/x/tools/cmd/godoc@latest
	@echo "Development dependencies installed"

# Quick development targets
.PHONY: quick-test
quick-test: ## Run quick tests (no race detector, shorter timeout)
	@echo "Running quick tests..."
	$(GO) test -v -timeout=5m ./tests/unit/...

.PHONY: watch-test
watch-test: ## Watch for changes and run tests
	@echo "Watching for changes..."
	@if command -v reflex >/dev/null; then \
		reflex -r '\.go$$' -s -- make quick-test; \
	else \
		echo "reflex not installed, install with: go install github.com/cespare/reflex@latest"; \
		echo "Running once instead..."; \
		make quick-test; \
	fi

.PHONY: debug-test
debug-test: ## Run tests with debug output
	@echo "Running tests with debug output..."
	$(GO) test -v -timeout=$(TEST_TIMEOUT) -tags=debug ./...