# SMTS (Secure Message Transport System) Architecture

## System Overview

SMTS is a Golang service designed for secure message transport between isolated corporate networks (EXT and INT) using NATS JetStream as the message backbone.

## Architecture Components

### 1. Network Deployments
- **EXT SMTS**: Deployed in external corporate network
- **INT SMTS**: Deployed in secured internal network
- **Unified Codebase**: Single codebase with configuration-driven behavior

### 2. Message Flow

#### EXT Network Flow:
```
NATS JetStream (EXT) → EXT SMTS → Corporate API Server (/topic_name)
```

#### INT Network Flow:
```
NATS JetStream (INT) → INT SMTS → DLP Validation (/validate) → Corporate API Server (/topic_name)
```

#### Cross-Network Flow:
```
Corporate API Server → ArtemisMQ → INT SMTS → NATS JetStream (INT)
```

## NATS JetStream Configuration

### Stream Configuration
```yaml
streams:
  ext_stream:
    name: "SMTS_EXT"
    subjects: ["monterra.>", "pact_update.>"]
    retention: workqueue
    max_age: 24h
    storage: file
    replicas: 3

  int_stream:
    name: "SMTS_INT" 
    subjects: ["monterra.>", "pact_update.>"]
    retention: workqueue
    max_age: 24h
    storage: file
    replicas: 3
```

### Consumer Configuration
```yaml
consumers:
  ext_consumer:
    stream: "SMTS_EXT"
    durable_name: "SMTS_EXT_CONSUMER"
    filter_subject: "monterra.>"
    ack_policy: explicit
    deliver_policy: all

  int_consumer:
    stream: "SMTS_INT"
    durable_name: "SMTS_INT_CONSUMER" 
    filter_subject: "pact_update.>"
    ack_policy: explicit
    deliver_policy: all
```

## Message Schema

### JSON Raw Message Format
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

## API Specifications

### Corporate API Server Endpoints

#### POST /topic_name
- **Authentication**: API Key (X-API-Key header)
- **Request Body**: Raw message data
- **Response**: 
  - 200: Message accepted
  - 400: Bad request
  - 401: Unauthorized
  - 500: Internal server error

#### POST /validate (DLP Endpoint)
- **Authentication**: API Key (X-API-Key header)
- **Request Body**: Message content for validation
- **Response**:
  - 200: Validation passed
  - 400: Validation failed (with reasons)
  - 401: Unauthorized

## Configuration Management

### Topics and Privileges Config
```yaml
topics:
  monterra:
    read_roles: ["ext_reader", "int_reader"]
    write_roles: ["ext_writer", "int_writer"]
    description: "Monterra events topic"

  pact_update:
    read_roles: ["ext_reader", "int_reader"] 
    write_roles: ["ext_writer", "int_writer"]
    description: "Pact update events topic"

roles:
  ext_reader: ["monterra", "pact_update"]
  ext_writer: ["monterra", "pact_update"]
  int_reader: ["monterra", "pact_update"]
  int_writer: ["monterra", "pact_update"]
```

## Project Structure
```
smts/
├── cmd/
│   ├── ext-smts/
│   └── int-smts/
├── internal/
│   ├── config/
│   ├── nats/
│   ├── api/
│   ├── dlp/
│   ├── artemis/
│   └── message/
├── pkg/
│   ├── utils/
│   └── types/
├── configs/
│   ├── ext-config.yaml
│   └── int-config.yaml
└── deployments/
    ├── docker-compose.ext.yml
    └── docker-compose.int.yml
```

## Technology Stack
- **Language**: Go 1.25
- **Message Broker**: NATS JetStream (embedded)
- **Queue**: ArtemisMQ (for INT network integration)
- **Configuration**: YAML with Viper
- **Logging**: Structured logging with Zap
- **Testing**: Go testing framework with Testify