# SMTS Project Structure and Configuration

## Project Directory Layout

```
smts/
├── cmd/                          # Application entry points
│   ├── ext-smts/                 # EXT network deployment
│   │   └── main.go
│   └── int-smts/                 # INT network deployment
│       └── main.go
├── internal/                     # Private application code
│   ├── config/                   # Configuration management
│   │   ├── config.go
│   │   ├── loader.go
│   │   └── validator.go
│   ├── nats/                     # NATS JetStream integration
│   │   ├── client.go
│   │   ├── stream.go
│   │   ├── consumer.go
│   │   └── publisher.go
│   ├── api/                      # Corporate API client
│   │   ├── client.go
│   │   ├── delivery.go
│   │   └── auth.go
│   ├── dlp/                      # DLP validation integration
│   │   ├── client.go
│   │   ├── validator.go
│   │   └── types.go
│   ├── artemis/                  # ArtemisMQ integration
│   │   ├── consumer.go
│   │   ├── connection.go
│   │   └── handler.go
│   ├── message/                  # Message processing
│   │   ├── processor.go
│   │   ├── router.go
│   │   ├── validator.go
│   │   └── types.go
│   └── server/                   # Service orchestration
│       ├── server.go
│       ├── health.go
│       └── lifecycle.go
├── pkg/                          # Public reusable packages
│   ├── utils/                    # Utility functions
│   │   ├── logger.go
│   │   ├── crypto.go
│   │   └── retry.go
│   └── types/                    # Common data types
│       ├── message.go
│       ├── config.go
│       └── errors.go
├── configs/                      # Configuration files
│   ├── ext-config.yaml           # EXT network configuration
│   ├── int-config.yaml           # INT network configuration
│   └── topics.yaml               # Topics and privileges
├── deployments/                  # Deployment configurations
│   ├── docker-compose.ext.yml    # EXT deployment
│   ├── docker-compose.int.yml    # INT deployment
│   ├── Dockerfile
│   └── k8s/                      # Kubernetes manifests
├── scripts/                      # Build and deployment scripts
│   ├── build.sh
│   ├── deploy.sh
│   └── test.sh
├── docs/                         # Documentation
│   ├── api.md
│   ├── deployment.md
│   └── troubleshooting.md
└── tests/                        # Test files
    ├── unit/
    ├── integration/
    └── e2e/
```

## Configuration Specifications

### Main Configuration (ext-config.yaml / int-config.yaml)

```yaml
# Deployment Configuration
deployment:
  type: "ext"  # or "int"
  name: "smts-ext-prod"
  environment: "production"

# NATS Configuration
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
    replicas: 3
  consumer:
    durable_name: "SMTS_EXT_CONSUMER"
    ack_policy: "explicit"
    deliver_policy: "all"

# Corporate API Configuration
api:
  base_url: "https://api.corporate.com"
  timeout: "30s"
  retry:
    max_attempts: 3
    backoff: "2s"
  auth:
    type: "api_key"
    api_key: "${API_KEY}"

# DLP Configuration (INT only)
dlp:
  enabled: false  # true for INT deployment
  endpoint: "https://dlp.corporate.com/validate"
  timeout: "10s"
  retry:
    max_attempts: 2
    backoff: "1s"

# ArtemisMQ Configuration (INT only)
artemis:
  enabled: false  # true for INT deployment
  host: "artemis.corporate.com"
  port: 61616
  queue: "SMTS_INT_QUEUE"
  username: "${ARTEMIS_USER}"
  password: "${ARTEMIS_PASSWORD}"

# Logging Configuration
logging:
  level: "info"
  format: "json"
  output: "stdout"

# Health Check Configuration
health:
  port: 8080
  path: "/health"
  interval: "30s"
```

### Topics and Privileges Configuration (topics.yaml)

```yaml
# Topic Definitions
topics:
  monterra:
    description: "Monterra events topic"
    patterns:
      - "monterra.>"
    permissions:
      read:
        - "ext_reader"
        - "int_reader"
      write:
        - "ext_writer" 
        - "int_writer"

  pact_update:
    description: "Pact update events topic"
    patterns:
      - "pact_update.>"
    permissions:
      read:
        - "ext_reader"
        - "int_reader"
      write:
        - "ext_writer"
        - "int_writer"

# Role Definitions
roles:
  ext_reader:
    description: "EXT network message reader"
    topics:
      - "monterra"
      - "pact_update"

  ext_writer:
    description: "EXT network message writer"
    topics:
      - "monterra"
      - "pact_update"

  int_reader:
    description: "INT network message reader"
    topics:
      - "monterra"
      - "pact_update"

  int_writer:
    description: "INT network message writer"
    topics:
      - "monterra"
      - "pact_update"
```

## Message Schema Definition

### Message Types (pkg/types/message.go)

```go
package types

import (
    "encoding/json"
    "time"
)

// Message represents the unified message format
type Message struct {
    ID        string            `json:"id"`
    Timestamp time.Time         `json:"timestamp"`
    Topic     string            `json:"topic"`
    Source    string            `json:"source"`
    Headers   map[string]string `json:"headers"`
    Body      json.RawMessage   `json:"body"` // JSON raw type
}

// DeliveryResult represents message delivery outcome
type DeliveryResult struct {
    Success    bool      `json:"success"`
    MessageID  string    `json:"message_id"`
    Timestamp  time.Time `json:"timestamp"`
    Error      string    `json:"error,omitempty"`
    RetryCount int       `json:"retry_count,omitempty"`
}

// DLPValidationRequest represents DLP validation request
type DLPValidationRequest struct {
    MessageID string          `json:"message_id"`
    Topic     string          `json:"topic"`
    Content   json.RawMessage `json:"content"`
    Metadata  map[string]any  `json:"metadata"`
}

// DLPValidationResponse represents DLP validation result
type DLPValidationResponse struct {
    Approved bool     `json:"approved"`
    MessageID string  `json:"message_id"`
    Reasons  []string `json:"reasons,omitempty"`
    Error    string   `json:"error,omitempty"`
}
```

## Environment Variables

```bash
# Required for both deployments
API_KEY=corporate_api_key_here

# Required for INT deployment only
ARTEMIS_USER=artemis_username
ARTEMIS_PASSWORD=artemis_password
DLP_API_KEY=dlp_api_key

# Optional configuration overrides
NATS_HOST=localhost
NATS_PORT=4222
LOG_LEVEL=info
```

## Build and Deployment

The project uses Go modules and can be built with:

```bash
# Build EXT deployment
go build -o bin/ext-smts ./cmd/ext-smts

# Build INT deployment  
go build -o bin/int-smts ./cmd/int-smts

# Run with configuration
./bin/ext-smts --config configs/ext-config.yaml