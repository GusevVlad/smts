# SMTS REST API Examples

This document provides curl commands and bash scripts for interacting with SMTS via REST API to write and read messages.

## Architecture Overview

SMTS implements bidirectional message flows between EXT and INT networks:

### Flow 1: EXT → INT Message Flow
**Path:** EXT Client → EXT-SMTS → Corporate API → ArtemisMQ → INT-SMTS → INT Client

### Flow 2: INT → EXT Message Flow
**Path:** INT Client → INT-SMTS → DLP → ArtemisMQ → Corporate API → EXT-SMTS → EXT Client

### Key Features:
- **Durable NATS Queues**: All NATS queues are durable with workqueue retention
- **FIFO Processing**: Messages processed in strict first-in-first-out order
- **Topic-Based Grouping**: Messages grouped by topic names in both input/output queues
- **DLP Integration**: Risk-based validation for INT → EXT flow
- **Confirmation Mechanism**: Clients confirm receipt before message deletion

Both deployments use embedded NATS JetStream for message queuing and provide REST API endpoints for message operations.

## Curl Commands

### 1. Send Message via EXT SMTS (EXT → INT Flow)

```bash
# Send a Monterra event with DORA metrics via EXT SMTS
curl -X POST "http://localhost:8080/send/monterra.event" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-api-key-here" \
  -d '{
    "id": "'$(date +%Y%m%d%H%M%S)-$(openssl rand -hex 4)'",
    "timestamp": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'",
    "topic": "monterra.event",
    "source": "ext-client",
    "headers": {
      "content-type": "application/json",
      "correlation-id": "'$(openssl rand -hex 8)'"
    },
    "body": {
      "event_type": "dora_metrics",
      "deployment_frequency": {
        "daily": 3.2,
        "weekly": 22.4,
        "monthly": 96.0,
        "trend": "improving"
      },
      "lead_time_for_changes": {
        "average_hours": 12.5,
        "median_hours": 8.2,
        "p95_hours": 36.7,
        "trend": "stable"
      },
      "change_failure_rate": {
        "percentage": 2.8,
        "failed_deployments": 3,
        "total_deployments": 107,
        "trend": "improving"
      },
      "time_to_restore_service": {
        "average_minutes": 45.2,
        "median_minutes": 28.7,
        "p95_minutes": 126.4,
        "trend": "improving"
      },
      "environment": "production",
      "team": "platform-engineering",
      "period": {
        "start": "'$(date -u -d '-7 days' +%Y-%m-%dT%H:%M:%SZ)'",
        "end": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'"
      }
    }
  }'
```

### 2. Send Message via INT SMTS

```bash
# Send a Pact update event via INT SMTS
curl -X POST "https://api.corporate.com/pact_update.event" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-api-key-here" \
  -H "X-SMTS-Message-ID: $(date +%Y%m%d%H%M%S)-$(openssl rand -hex 4)" \
  -H "X-SMTS-Timestamp: $(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  -H "X-SMTS-Source: int-client" \
  -H "smts-role: int_writer" \
  -d '{
    "pact_id": "pact-789",
    "version": "2.1.0",
    "update_type": "terms_change",
    "effective_date": "'$(date -u -d '+30 days' +%Y-%m-%dT%H:%M:%SZ)'",
    "changes": [
      "Updated section 4.2",
      "Added new clause 7.1"
    ]
  }'
```

### 2. Receive Messages via EXT SMTS (INT → EXT Flow)

```bash
# Receive messages from EXT SMTS
curl -X GET "http://localhost:8080/receive/monterra.event?count=5" \
  -H "X-API-Key: your-api-key-here"

# Confirm receipt of messages
curl -X POST "http://localhost:8080/confirm/monterra.event" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-api-key-here" \
  -d '{
    "message_ids": ["20251002163439-abcd1234", "20251002163440-efgh5678"]
  }'
```

### 3. Send Message via INT SMTS (INT → EXT Flow)

```bash
# Send a Pact update event via INT SMTS
curl -X POST "http://localhost:8080/send/pact_update.event" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-api-key-here" \
  -d '{
    "id": "'$(date +%Y%m%d%H%M%S)-$(openssl rand -hex 4)'",
    "timestamp": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'",
    "topic": "pact_update.event",
    "source": "int-client",
    "headers": {
      "content-type": "application/json",
      "correlation-id": "'$(openssl rand -hex 8)'"
    },
    "body": {
      "pact_id": "pact-789",
      "version": "2.1.0",
      "update_type": "terms_change",
      "effective_date": "'$(date -u -d '+30 days' +%Y-%m-%dT%H:%M:%SZ)'",
      "changes": [
        "Updated section 4.2",
        "Added new clause 7.1"
      ]
    }
  }'
```

### 4. Receive Messages via INT SMTS (EXT → INT Flow)

```bash
# Receive messages from INT SMTS
curl -X GET "http://localhost:8080/receive/pact_update.event?count=5" \
  -H "X-API-Key: your-api-key-here"

# Confirm receipt of messages
curl -X POST "http://localhost:8080/confirm/pact_update.event" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-api-key-here" \
  -d '{
    "message_ids": ["20251002163441-ijkl9012", "20251002163442-mnop3456"]
  }'
```

### 5. Health Check

```bash
# Check SMTS health status
curl -X GET "http://localhost:8080/health" \
  -H "X-API-Key: your-api-key-here"
```

## Bash Scripts

### 1. External Client Sending Message

```bash
#!/bin/bash
# ext-client-send.sh - External client sending message to EXT SMTS

# Configuration
API_BASE_URL="https://api.corporate.com"
API_KEY="your-ext-api-key-here"
TOPIC="monterra.event"

# Generate unique message ID
MESSAGE_ID=$(date +%Y%m%d%H%M%S)-$(openssl rand -hex 4)
TIMESTAMP=$(date -u +%Y-%m-%dT%H:%M:%SZ)

# Message payload
PAYLOAD=$(cat <<EOF
{
  "event_type": "payment_processed",
  "transaction_id": "txn-$(openssl rand -hex 8)",
  "amount": 299.99,
  "currency": "USD",
  "customer_id": "cust-789",
  "timestamp": "$TIMESTAMP",
  "payment_method": "credit_card",
  "status": "completed"
}
EOF
)

echo "Sending message via EXT SMTS..."
echo "Message ID: $MESSAGE_ID"
echo "Topic: $TOPIC"

# Send the message
RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$API_BASE_URL/$TOPIC" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: $API_KEY" \
  -H "X-SMTS-Message-ID: $MESSAGE_ID" \
  -H "X-SMTS-Timestamp: $TIMESTAMP" \
  -H "X-SMTS-Source: ext-client" \
  -H "smts-role: ext_writer" \
  -d "$PAYLOAD")

# Extract HTTP status code
HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
RESPONSE_BODY=$(echo "$RESPONSE" | head -n -1)

echo "HTTP Status: $HTTP_CODE"
echo "Response: $RESPONSE_BODY"

if [ "$HTTP_CODE" -eq 200 ] || [ "$HTTP_CODE" -eq 201 ]; then
    echo "✅ Message sent successfully!"
else
    echo "❌ Failed to send message"
    exit 1
fi
```

### 2. Internal Client Receiving Message (Simulation)

```bash
#!/bin/bash
# int-client-receive.sh - Internal client simulating message reception

# Configuration
API_BASE_URL="https://api.corporate.com"
API_KEY="your-int-api-key-here"

echo "Simulating INT client receiving messages from INT SMTS..."
echo "Note: In production, INT SMTS would push messages to internal systems"

# Simulate receiving different types of messages
MESSAGE_TYPES=("monterra.event" "pact_update.event")

for topic in "${MESSAGE_TYPES[@]}"; do
    echo ""
    echo "📨 Checking for messages on topic: $topic"
    
    # In a real scenario, this would be a webhook or message queue consumer
    # For demonstration, we'll simulate receiving a message
    MESSAGE_ID=$(date +%Y%m%d%H%M%S)-$(openssl rand -hex 4)
    TIMESTAMP=$(date -u +%Y-%m-%dT%H:%M:%SZ)
    
    # Simulate message content based on topic
    case $topic in
        "monterra.event")
            PAYLOAD=$(cat <<EOF
{
  "message_id": "$MESSAGE_ID",
  "topic": "$topic",
  "timestamp": "$TIMESTAMP",
  "data": {
    "event_type": "user_activity",
    "user_id": "int-user-456",
    "action": "file_download",
    "file_name": "quarterly_report.pdf",
    "file_size": 5242880
  }
}
EOF
            )
            ;;
        "pact_update.event")
            PAYLOAD=$(cat <<EOF
{
  "message_id": "$MESSAGE_ID",
  "topic": "$topic",
  "timestamp": "$TIMESTAMP",
  "data": {
    "pact_id": "int-pact-123",
    "update_type": "compliance_update",
    "regulation": "GDPR",
    "effective_date": "$(date -u -d '+60 days' +%Y-%m-%dT%H:%M:%SZ)",
    "required_actions": ["update_privacy_policy", "obtain_consent"]
  }
}
EOF
            )
            ;;
    esac
    
    echo "Received message:"
    echo "$PAYLOAD" | jq '.' 2>/dev/null || echo "$PAYLOAD"
    echo "✅ Message processed successfully"
done

echo ""
echo "🎉 All messages processed by INT client"
```

### 3. Bidirectional Communication Script

```bash
#!/bin/bash
# smts-bidirectional-demo.sh - Demonstrates EXT→INT and INT→EXT communication

# Configuration
EXT_API_URL="https://api.corporate.com"
INT_API_URL="https://api.corporate.com"  # Different URL in production
EXT_API_KEY="your-ext-api-key"
INT_API_KEY="your-int-api-key"

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${BLUE}🚀 SMTS Bidirectional Communication Demo${NC}"
echo "=========================================="

# Function to send message
send_message() {
    local deployment=$1
    local topic=$2
    local payload=$3
    local api_key=$4
    local source=$5
    
    local message_id=$(date +%Y%m%d%H%M%S)-$(openssl rand -hex 4)
    local timestamp=$(date -u +%Y-%m-%dT%H:%M:%SZ)
    
    echo -e "${YELLOW}Sending $deployment message to $topic...${NC}"
    
    curl -s -X POST "$EXT_API_URL/$topic" \
      -H "Content-Type: application/json" \
      -H "X-API-Key: $api_key" \
      -H "X-SMTS-Message-ID: $message_id" \
      -H "X-SMTS-Timestamp: $timestamp" \
      -H "X-SMTS-Source: $source" \
      -H "smts-role: ${deployment}_writer" \
      -d "$payload" > /dev/null
    
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✅ $deployment message sent successfully!${NC}"
        echo "   Message ID: $message_id"
        echo "   Topic: $topic"
    else
        echo -e "${RED}❌ Failed to send $deployment message${NC}"
    fi
}

# EXT → INT Communication
echo ""
echo -e "${BLUE}EXT → INT Communication${NC}"
echo "------------------------"

# EXT client sends Monterra event
EXT_PAYLOAD=$(cat <<EOF
{
  "event_type": "external_data_received",
  "source_system": "partner_api",
  "data_type": "customer_feedback",
  "record_count": 250,
  "timestamp": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "metadata": {
    "partner_id": "partner-xyz",
    "data_format": "json",
    "encryption": "aes-256"
  }
}
EOF
)

send_message "ext" "monterra.event" "$EXT_PAYLOAD" "$EXT_API_KEY" "ext-demo-client"

# INT → EXT Communication  
echo ""
echo -e "${BLUE}INT → EXT Communication${NC}"
echo "------------------------"

# INT client sends Pact update
INT_PAYLOAD=$(cat <<EOF
{
  "pact_id": "demo-pact-2025",
  "version": "1.0.0",
  "update_type": "security_patch",
  "severity": "high",
  "description": "Critical security vulnerability patch",
  "affected_versions": ["1.0.0", "1.1.0"],
  "patch_notes": [
    "Fixed authentication bypass vulnerability",
    "Enhanced input validation",
    "Updated dependency versions"
  ],
  "deployment_instructions": "Immediate deployment required",
  "rollback_plan": "Revert to version 0.9.5 if issues occur"
}
EOF
)

send_message "int" "pact_update.event" "$INT_PAYLOAD" "$INT_API_KEY" "int-demo-client"

echo ""
echo -e "${GREEN}🎉 Bidirectional communication demo completed!${NC}"
echo ""
echo "Summary:"
echo "• EXT client → monterra.event → INT systems"
echo "• INT client → pact_update.event → EXT systems"
echo ""
echo "In production, SMTS handles:"
echo "• Message routing between EXT and INT networks"
echo "• DLP validation for INT-bound messages"
echo "• Authentication and authorization"
echo "• Retry logic and error handling"
```

### 4. Complete Test Script

```bash
#!/bin/bash
# smts-complete-test.sh - Complete test of SMTS REST API

set -e  # Exit on any error

# Configuration
API_BASE_URL="${API_BASE_URL:-https://api.corporate.com}"
API_KEY="${API_KEY:-your-api-key-here}"
TEST_TIMEOUT=30

# Test results
PASSED=0
FAILED=0

# Helper functions
log() {
    echo "[$(date +%Y-%m-%d\ %H:%M:%S)] $1"
}

pass() {
    log "✅ PASS: $1"
    ((PASSED++))
}

fail() {
    log "❌ FAIL: $1"
    ((FAILED++))
}

test_health() {
    log "Testing health endpoint..."
    if curl -s -f -X GET "$API_BASE_URL/health" \
       -H "X-API-Key: $API_KEY" > /dev/null; then
        pass "Health check"
    else
        fail "Health check"
    fi
}

test_message_delivery() {
    local topic=$1
    local role=$2
    local source=$3
    
    log "Testing message delivery to $topic..."
    
    local message_id=$(date +%Y%m%d%H%M%S)-$(openssl rand -hex 4)
    local timestamp=$(date -u +%Y-%m-%dT%H:%M:%SZ)
    
    local payload=$(cat <<EOF
{
  "test_id": "$message_id",
  "test_timestamp": "$timestamp",
  "test_type": "api_integration",
  "deployment": "$role",
  "data": {
    "sample_field": "test_value_$(openssl rand -hex 4)",
    "numeric_value": 42,
    "boolean_value": true
  }
}
EOF
)
    
    local response=$(curl -s -w "\n%{http_code}" -X POST "$API_BASE_URL/$topic" \
      -H "Content-Type: application/json" \
      -H "X-API-Key: $API_KEY" \
      -H "X-SMTS-Message-ID: $message_id" \
      -H "X-SMTS-Timestamp: $timestamp" \
      -H "X-SMTS-Source: $source" \
      -H "smts-role: ${role}_writer" \
      -d "$payload")
    
    local http_code=$(echo "$response" | tail -n1)
    
    if [ "$http_code" -eq 200 ] || [ "$http_code" -eq 201 ]; then
        pass "Message delivery to $topic"
    else
        fail "Message delivery to $topic (HTTP $http_code)"
    fi
}

# Main test execution
echo "Starting SMTS REST API Tests"
echo "============================"
echo "API Base URL: $API_BASE_URL"
echo "Timeout: ${TEST_TIMEOUT}s"
echo ""

# Run tests
test_health
test_message_delivery "monterra.event" "ext" "test-ext-client"
test_message_delivery "pact_update.event" "int" "test-int-client"

# Summary
echo ""
echo "Test Summary"
echo "============"
echo "✅ PASSED: $PASSED"
echo "❌ FAILED: $FAILED"
echo ""

if [ $FAILED -eq 0 ]; then
    echo "🎉 All tests passed!"
    exit 0
else
    echo "💥 Some tests failed!"
    exit 1
fi
```

## Usage Instructions

1. **Make scripts executable:**
   ```bash
   chmod +x *.sh
   ```

2. **Set environment variables:**
   ```bash
   export API_BASE_URL="https://your-smts-api.com"
   export API_KEY="your-actual-api-key"
   ```

3. **Run individual scripts:**
   ```bash
   ./ext-client-send.sh
   ./int-client-receive.sh
   ./smts-bidirectional-demo.sh
   ./smts-complete-test.sh
   ```

## Message Flow

### Flow 1: EXT → INT
1. **EXT Client** sends message via POST /send/{topic_name} to EXT-SMTS
2. **EXT-SMTS** places message in output queue (grouped by topic) in embedded NATS
3. Messages processed in FIFO order and sent to **Corporate API** /topic_name endpoint
4. **Corporate API** pushes messages to **ArtemisMQ** queue
5. **INT-SMTS** pulls messages from ArtemisMQ and collects in input queue (grouped by topic)
6. **INT Client** reads messages via GET /receive/{topic_name}?count=n
7. **INT Client** confirms receipt via POST /confirm/{topic_name}, messages deleted from NATS

### Flow 2: INT → EXT
1. **INT Client** sends message via POST /send/{topic_name} to INT-SMTS
2. **INT-SMTS** places message in output queue (grouped by topic) in embedded NATS
3. Messages processed in FIFO order and sent to **DLP Server** for risk check
4. **Approved** messages pushed to **ArtemisMQ** via STOMP, **Rejected** messages logged to incidents.log
5. **Corporate API** consumes messages from ArtemisMQ (FIFO)
6. **Corporate API** sends messages via POST /corp_message/{topic_name} to EXT-SMTS
7. **EXT-SMTS** stores messages in input queue (grouped by topic)
8. **EXT Client** reads messages via GET /receive/{topic_name}?count=n
9. **EXT Client** confirms receipt via POST /confirm/{topic_name}, messages deleted from NATS

## Security Headers

- `X-API-Key`: Authentication token
- `X-SMTS-Message-ID`: Unique message identifier
- `X-SMTS-Timestamp`: Message creation timestamp (RFC3339)
- `X-SMTS-Source`: Message source identifier
- `smts-role`: Role-based access control (ext_writer/int_writer)

## Response Codes

- `200 OK`: Message delivered successfully
- `201 Created`: Message accepted for processing
- `400 Bad Request`: Invalid message format
- `401 Unauthorized`: Invalid API key
- `403 Forbidden`: Insufficient permissions
- `500 Internal Server Error`: Server error