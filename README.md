# SMTS (Secure Message Transport System)

A Golang service for secure message transport between isolated corporate networks using NATS JetStream as the message backbone.

## Overview

SMTS provides secure message transport between isolated corporate networks (EXT and INT) with the following features:

- **Unified Codebase**: Single codebase for both EXT and INT deployments
- **Embedded NATS JetStream**: Self-contained message broker
- **JSON Raw Type**: Flexible message format with raw JSON body support
- **DLP Validation**: Data Loss Prevention integration for INT network
- **ArtemisMQ Integration**: Cross-network message flow via ArtemisMQ
- **Configuration-Driven**: Behavior determined by YAML configuration files

## Architecture

### Network Deployments

- **EXT SMTS**: Deployed in external corporate network
- **INT SMTS**: Deployed in secured internal network

### Message Flow

#### EXT Network:
```
NATS JetStream (EXT) → EXT SMTS → Corporate API Server (/topic_name)
```

#### INT Network:
```
NATS JetStream (INT) → INT SMTS → DLP Validation (/validate) → Corporate API Server (/topic_name)
```

#### Cross-Network Flow:
```
Corporate API Server → ArtemisMQ → INT SMTS → NATS JetStream (INT)
```

## Quick Start

### Prerequisites

- Go 1.25
- Docker (for containerized deployment)

### Installation

1. Clone the repository:
```bash
git clone <repository-url>
cd smts
```

2. Install dependencies:
```bash
go mod download
```

3. Build the applications:
```bash
# Build EXT deployment
go build -o bin/ext-smts ./cmd/ext-smts

# Build INT deployment
go build -o bin/int-smts ./cmd/int-smts
```

### Configuration

#### Environment Variables

```bash
# Required for both deployments
export API_KEY=corporate_api_key_here

# Required for INT deployment only
export ARTEMIS_USER=artemis_username
export ARTEMIS_PASSWORD=artemis_password

# Optional overrides
export NATS_HOST=localhost
export NATS_PORT=4222
export LOG_LEVEL=info
```

#### Configuration Files

- `configs/ext-config.yaml` - EXT network configuration
- `configs/int-config.yaml` - INT network configuration  
- `configs/topics.yaml` - Topics and privileges configuration

### Running the Service

#### EXT Deployment
```bash
./bin/ext-smts --config configs/ext-config.yaml
```

#### INT Deployment
```bash
./bin/int-smts --config configs/int-config.yaml
```

#### Health Check
```bash
./bin/ext-smts --health-check --config configs/ext-config.yaml
```

## Configuration Details

### Main Configuration Structure

```yaml
deployment:
  type: "ext"  # or "int"
  name: "smts-ext-prod"
  environment: "production"

nats:
  embedded: true
  host: "localhost"
  port: 4222
  stream:
    name: "SMTS_EXT"
    subjects: ["monterra.>", "pact_update.>"]
    retention: "workqueue"
    max_age: "24h"
    storage: "file"
    replicas: 1

api:
  base_url: "https://api.corporate.com"
  timeout: "30s"
  auth:
    type: "api_key"
    api_key: "${API_KEY}"
```

### Topics Configuration

```yaml
topics:
  monterra:
    description: "Monterra events topic"
    permissions:
      read: ["ext_reader", "int_reader"]
      write: ["ext_writer", "int_writer"]

roles:
  ext_reader:
    description: "EXT network message reader"
    topics: ["monterra", "pact_update"]
```

## Message Format

### JSON Message Structure

```json
{
  "id": "uuid-v4",
  "timestamp": "2025-09-23T16:34:17Z",
  "topic": "monterra.event",
  "source": "ext_smts",
  "headers": {
    "content-type": "application/json",
    "correlation-id": "uuid-v4"
  },
  "body": "base64_encoded_raw_data"
}
```

## API Integration

### Corporate API Endpoints

#### POST /topic_name
- **Authentication**: API Key (X-API-Key header)
- **Request Body**: Raw message data
- **Response**: 200 OK on success

#### POST /validate (DLP Endpoint)
- **Authentication**: API Key (X-API-Key header)
- **Request Body**: Message content for validation
- **Response**: Approval/Rejection with reasons

## Health Monitoring

### Health Endpoints

- `GET /health` - Comprehensive health check
- `GET /ready` - Readiness probe
- `GET /live` - Liveness probe
- `GET /metrics` - Metrics endpoint (future)

### Health Check Response

```json
{
  "status": "healthy",
  "timestamp": "2025-09-23T16:34:17Z",
  "uptime": "5m30s",
  "version": "1.0.0",
  "deployment": {
    "type": "ext",
    "name": "smts-ext-prod",
    "environment": "production"
  },
  "checks": {
    "process": {"status": "healthy", "details": "Process is running"},
    "uptime": {"status": "healthy", "details": "5m30s", "seconds": 330},
    "overall": {"status": "healthy", "details": "Overall system health"}
  }
}
```

## Deployment

### Docker Deployment

#### EXT Deployment
```bash
docker build -t smts-ext .
docker run -d \
  --name smts-ext \
  -p 8080:8080 \
  -e API_KEY=your_api_key \
  -v $(pwd)/configs:/app/configs \
  smts-ext
```

#### INT Deployment
```bash
docker build -t smts-int .
docker run -d \
  --name smts-int \
  -p 8080:8080 \
  -e API_KEY=your_api_key \
  -e ARTEMIS_USER=artemis_user \
  -e ARTEMIS_PASSWORD=artemis_password \
  -v $(pwd)/configs:/app/configs \
  smts-int
```

### Kubernetes Deployment

See `deployments/k8s/` directory for Kubernetes manifests.

## Monitoring and Logging

### Logging

- **Format**: Structured JSON logging
- **Levels**: debug, info, warn, error
- **Output**: stdout or file

### Metrics

- Prometheus metrics endpoint at `/metrics`
- Custom metrics for message processing, errors, and performance

## Troubleshooting

### Common Issues

1. **NATS Connection Failed**
   - Check NATS server availability
   - Verify host and port configuration
   - Check firewall settings

2. **API Authentication Failed**
   - Verify API_KEY environment variable
   - Check corporate API endpoint accessibility
   - Validate API key permissions

3. **DLP Validation Errors**
   - Check DLP service availability
   - Verify message format and content
   - Review DLP policy configuration

### Debug Mode

Enable debug logging for detailed troubleshooting:

```yaml
logging:
  level: "debug"
  format: "console"  # Human-readable format for debugging
```

## Development

### Building from Source

```bash
# Build both deployments
make build

# Run tests
make test

# Run linting
make lint
```

### Testing

```bash
# Unit tests
go test ./...

# Integration tests
go test -tags=integration ./...

# End-to-end tests
go test -tags=e2e ./...
```

## Security Considerations

- All external calls use TLS encryption
- API key authentication for corporate endpoints
- DLP validation for INT-bound messages
- No sensitive data in logs
- Topic-level permission enforcement
