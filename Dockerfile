# Build stage
FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build both EXT and INT deployments
RUN go build -o bin/ext-smts ./cmd/ext-smts
RUN go build -o bin/int-smts ./cmd/int-smts

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create app user
RUN addgroup -S app && adduser -S app -G app

# Set working directory
WORKDIR /app

# Copy binaries from builder
COPY --from=builder /app/bin/ /app/bin/
COPY --from=builder /app/configs/ /app/configs/

# Create data directory for NATS storage
RUN mkdir -p /app/data && chown -R app:app /app

# Switch to app user
USER app

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD /app/bin/ext-smts --health-check --config /app/configs/ext-config.yaml || exit 1

# Default command (EXT deployment)
CMD ["/app/bin/ext-smts", "--config", "/app/configs/ext-config.yaml"]