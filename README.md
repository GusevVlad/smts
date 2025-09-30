# SMTS (Secure Message Transport System)

A Golang service for secure message transport between isolated corporate networks using NATS JetStream as the message backbone.

## Overview

SMTS provides secure message transport between isolated corporate networks (EXT and INT) with the following features:

- **Embedded NATS JetStream**: Self-contained message broker
- **JSON Raw Type**: Flexible message format with raw JSON body support
- **DLP Validation**: Data Loss Prevention integration for INT network
- **ArtemisMQ Integration**: Cross-network message flow via ArtemisMQ
- **Configuration-Driven**: Behavior determined by YAML configuration files
- **JSON raw**: type for flexible message content
- **Unified Codebase**: Single codebase for both EXT and INT deployments

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

## System Message Flow

```mermaid
flowchart TD
    subgraph EXT Network
        EXT_NATS[EXT NATS JetStream]
        EXT_SMTS[EXT SMTS Service]
    end

    subgraph INT Network  
        INT_NATS[INT NATS JetStream]
        INT_SMTS[INT SMTS Service]
        DLP[DLP Validation]
    end

    subgraph Corporate API
        API[Corporate API Server]
        ARTEMIS[ArtemisMQ]
    end

    %% EXT Network Flow
    EXT_NATS --> EXT_SMTS
    EXT_SMTS --> API

    %% INT Network Flow
    INT_NATS --> INT_SMTS
    INT_SMTS --> DLP
    DLP --> API

    %% Cross-Network Flow
    API --> ARTEMIS
    ARTEMIS --> INT_SMTS
    INT_SMTS --> INT_NATS
```

## Detailed EXT SMTS Flow

```mermaid
sequenceDiagram
    participant C as EXT Client
    participant N as EXT NATS
    participant S as EXT SMTS
    participant A as Corporate API

    C->>N: Publish message to topic
    N->>S: Consumer pulls message
    S->>S: Validate topic permissions
    S->>A: POST /topic_name with message
    A->>S: 200 OK
    S->>N: Acknowledge message
```

## Detailed INT SMTS Flow

```mermaid
sequenceDiagram
    participant C as INT Client
    participant N as INT NATS
    participant S as INT SMTS
    participant D as DLP Service
    participant A as Corporate API
    participant M as ArtemisMQ

    C->>N: Publish message to topic
    N->>S: Consumer pulls message
    S->>S: Validate topic permissions
    S->>D: POST /validate for DLP check
    D->>S: Validation result
    alt Validation Passed
        S->>A: POST /topic_name with message
        A->>S: 200 OK
        S->>N: Acknowledge message
    else Validation Failed
        S->>S: Log rejection
        S->>N: Acknowledge message (discard)
    end

    Note over A,M: Cross-network message flow
    A->>M: Push message to ArtemisMQ
    M->>S: INT SMTS consumes from Artemis
    S->>N: Publish to INT NATS stream
```

## Error Handling Flow

```mermaid
flowchart TD
    Start[Process Message] --> Validate{Validate Permissions}
    Validate -->|Invalid| LogError[Log Permission Error]
    Validate -->|Valid| Process
    
    subgraph Process
        Direction{Network Type}
        Direction -->|EXT| SendAPI[Send to Corporate API]
        Direction -->|INT| DLPCheck[DLP Validation]
        
        DLPCheck -->|Passed| SendAPI
        DLPCheck -->|Failed| LogDLPError[Log DLP Rejection]
    end

    SendAPI --> APIResult{API Response}
    APIResult -->|Success| Ack[Acknowledge Message]
    APIResult -->|Temporary Error| Retry[Retry with Backoff]
    APIResult -->|Permanent Error| DeadLetter[Dead Letter Queue]
    
    Retry -->|Max Retries| DeadLetter
    Retry -->|Success| Ack
    
    LogError --> Discard[Discard Message]
    LogDLPError --> Discard
    DeadLetter --> Finish[Finish Processing]
    Ack --> Finish
    Discard --> Finish
```

## Quick Start

### Prerequisites

- Go 1.25
- Docker (for containerized deployment)

### Installation

Запуск тестов (полный набор):

```bash
docker-compose -f docker-compose.test.yml build && docker-compose -f docker-compose.test.yml up -d
```
Результаты в папке /docker-logs

Build the applications:
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
  type: "int"  # or "ext"
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

dlp: # int only
  enabled: true
  endpoint: "https://dlp.corporate.com/validate"
  timeout: "10s"
  retry:
    max_attempts: 2
    backoff: "1s"

artemis:  # int only
  enabled: true
  host: "artemis"
  port: 61613
  queue: "SMTS_INT_QUEUE"
  username: "${ARTEMIS_USER}"
  password: "${ARTEMIS_PASSWORD}"
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
  "timestamp": "2025-09-29T16:34:17Z",
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

### Debug Mode

Enable debug logging for detailed troubleshooting:

```yaml
logging:
  level: "debug"
  format: "console"  # Human-readable format for debugging
```

## Health Monitoring

### Health Endpoints

- `GET /health` - Comprehensive health check
- `GET /ready` - Readiness probe
- `GET /live` - Liveness probe
- `GET /metrics` - Metrics endpoint (future)

### Health Check Request

```bash
#!/bin/bash
# health-check.sh
URL="http://localhost:8080/health"
response=$(curl -s -w "%{http_code}" $URL)
http_code=$(tail -n1 <<< "$response")
content=$(sed '$ d' <<< "$response")

if [ $http_code -eq 200 ]; then
    echo "Health check PASSED"
    exit 0
else
    echo "Health check FAILED: HTTP $http_code"
    echo "$content"
    exit 1
fi
```

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

## Security Considerations

- All external calls use TLS encryption
- API key authentication for corporate endpoints
- DLP validation for INT-bound messages
- No sensitive data in logs
- Topic-level permission enforcement
