# SMTS Makefile

# Variables
BIN_DIR = bin
CONFIG_DIR = configs
EXT_BINARY = $(BIN_DIR)/ext-smts
INT_BINARY = $(BIN_DIR)/int-smts
DOCKER_IMAGE_EXT = smts-ext
DOCKER_IMAGE_INT = smts-int
VERSION ?= 1.0.0

# Default target
.PHONY: all
all: build

# Build both EXT and INT deployments
.PHONY: build
build: $(EXT_BINARY) $(INT_BINARY)

# Build EXT deployment
$(EXT_BINARY):
	@echo "Building EXT SMTS..."
	@mkdir -p $(BIN_DIR)
	go build -o $(EXT_BINARY) ./cmd/ext-smts

# Build INT deployment
$(INT_BINARY):
	@echo "Building INT SMTS..."
	@mkdir -p $(BIN_DIR)
	go build -o $(INT_BINARY) ./cmd/int-smts

# Run tests
.PHONY: test
test:
	@echo "Running tests..."
	go test ./...

# Run integration tests
.PHONY: test-integration
test-integration:
	@echo "Running integration tests..."
	go test -tags=integration ./...

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning..."
	rm -rf $(BIN_DIR)
	go clean

# Format code
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Lint code
.PHONY: lint
lint:
	@echo "Linting code..."
	@if command -v golangci-lint >/dev/null; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed, skipping linting"; \
	fi

# Vet code
.PHONY: vet
vet:
	@echo "Vetting code..."
	go vet ./...

# Health check
.PHONY: health-check-ext
health-check-ext: $(EXT_BINARY)
	@echo "Running health check for EXT deployment..."
	./$(EXT_BINARY) --health-check --config $(CONFIG_DIR)/ext-config.yaml

.PHONY: health-check-int
health-check-int: $(INT_BINARY)
	@echo "Running health check for INT deployment..."
	./$(INT_BINARY) --health-check --config $(CONFIG_DIR)/int-config.yaml

# Docker builds
.PHONY: docker-build
docker-build: docker-build-ext docker-build-int

.PHONY: docker-build-ext
docker-build-ext:
	@echo "Building EXT Docker image..."
	docker build -t $(DOCKER_IMAGE_EXT):$(VERSION) -t $(DOCKER_IMAGE_EXT):latest .

.PHONY: docker-build-int
docker-build-int:
	@echo "Building INT Docker image..."
	docker build -t $(DOCKER_IMAGE_INT):$(VERSION) -t $(DOCKER_IMAGE_INT):latest .

# Docker run
.PHONY: docker-run-ext
docker-run-ext:
	@echo "Running EXT Docker container..."
	docker run -d \
		--name smts-ext \
		-p 8080:8080 \
		-e API_KEY=$${API_KEY} \
		-v $(PWD)/$(CONFIG_DIR):/app/$(CONFIG_DIR) \
		$(DOCKER_IMAGE_EXT):latest

.PHONY: docker-run-int
docker-run-int:
	@echo "Running INT Docker container..."
	docker run -d \
		--name smts-int \
		-p 8080:8080 \
		-e API_KEY=$${API_KEY} \
		-e ARTEMIS_USER=$${ARTEMIS_USER} \
		-e ARTEMIS_PASSWORD=$${ARTEMIS_PASSWORD} \
		-v $(PWD)/$(CONFIG_DIR):/app/$(CONFIG_DIR) \
		$(DOCKER_IMAGE_INT):latest

# Development
.PHONY: dev-ext
dev-ext: $(EXT_BINARY)
	@echo "Starting EXT SMTS in development mode..."
	./$(EXT_BINARY) --config $(CONFIG_DIR)/ext-config.yaml

.PHONY: dev-int
dev-int: $(INT_BINARY)
	@echo "Starting INT SMTS in development mode..."
	./$(INT_BINARY) --config $(CONFIG_DIR)/int-config.yaml

# Help
.PHONY: help
help:
	@echo "SMTS Makefile Targets:"
	@echo "  build           - Build both EXT and INT deployments"
	@echo "  test            - Run unit tests"
	@echo "  test-integration - Run integration tests"
	@echo "  clean           - Clean build artifacts"
	@echo "  fmt             - Format code"
	@echo "  lint            - Lint code (requires golangci-lint)"
	@echo "  vet             - Vet code"
	@echo "  health-check-ext - Health check for EXT deployment"
	@echo "  health-check-int - Health check for INT deployment"
	@echo "  docker-build    - Build both Docker images"
	@echo "  docker-run-ext  - Run EXT Docker container"
	@echo "  docker-run-int  - Run INT Docker container"
	@echo "  dev-ext         - Run EXT deployment in development"
	@echo "  dev-int         - Run INT deployment in development"
	@echo "  help            - Show this help message"