# SMTS REST API Examples

This document provides comprehensive examples for using the SMTS (Secure Message Transportation System) REST APIs for bidirectional message flow between EXT and INT environments.

## Overview

SMTS provides two types of REST APIs:
1. **HTTP Publishing API** - Send messages to SMTS via HTTP endpoints
2. **Message API** - Read messages from SMTS via HTTP endpoints

## Service Ports

### EXT SMTS
- **Health Server**: `http://localhost:18091/health`
- **HTTP Publishing API**: `http://localhost:18092/{topic}`
- **Message API**: `http://localhost:19081/messages`

### INT SMTS
- **Health Server**: `http://localhost:18093/health`
- **HTTP Publishing API**: `http://localhost:18094/{topic}`
- **Message API**: `http://localhost:19083/messages`

## Quick Start Examples

### 1. Send Message to EXT SMTS

```bash
curl -X POST "http://localhost:18092/test.monterra.event" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: test-api-key" \
  -H "X-SMTS-Message-ID: test-$(date +%s)" \
  -H "X-SMTS-Timestamp: $(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  -H "X-SMTS-Source: demo-client" \
  -H "smts-role: ext_writer" \
  -d '{
    "event_type": "demo_message",
    "message": "Test message for bidirectional flow",
    "timestamp": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'"
  }'
```

### 2. Send Message to INT SMTS

```bash
curl -X POST "http://localhost:18094/test.monterra.event" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: test-api-key" \
  -H "X-SMTS-Message-ID: test-int-$(date +%s)" \
  -H "X-SMTS-Timestamp: $(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  -H "X-SMTS-Source: demo-client" \
  -H "smts-role: int_writer" \
  -d '{
    "event_type": "demo_message",
    "message": "Test message via INT HTTP endpoint",
    "timestamp": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'"
  }'
```

### 3. Receive Messages from EXT SMTS

```bash
# Get up to 5 messages from EXT SMTS
curl -s "http://localhost:19081/messages?topic=test.monterra.event&count=5" | jq .

# Get a single message from EXT SMTS
curl -s "http://localhost:19081/messages?topic=test.monterra.event" | jq .
```

### 4. Receive Messages from INT SMTS

```bash
# Get up to 5 messages from INT SMTS
curl -s "http://localhost:19083/messages?topic=test.monterra.event&count=5" | jq .

# Get a single message from INT SMTS
curl -s "http://localhost:19083/messages?topic=test.monterra.event" | jq .
```

## Complete Bidirectional Flow Examples

### Example 1: EXT → INT Message Flow

```bash
#!/bin/bash
echo "=== EXT → INT Message Flow Demo ==="

# 1. Send message to EXT SMTS
MESSAGE_ID="ext-to-int-$(date +%s)"
echo "Sending message to EXT SMTS: $MESSAGE_ID"

curl -X POST "http://localhost:18092/test.monterra.event" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: test-api-key" \
  -H "X-SMTS-Message-ID: $MESSAGE_ID" \
  -H "X-SMTS-Timestamp: $(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  -H "X-SMTS-Source: ext-client" \
  -H "smts-role: ext_writer" \
  -d '{
    "event_type": "ext_to_int_demo",
    "message_id": "'$MESSAGE_ID'",
    "direction": "EXT → INT",
    "data": {
      "project": "Monterra Platform",
      "metrics": {
        "deployment_frequency": 12.5,
        "lead_time": 3.2
      }
    },
    "timestamp": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'"
  }' | jq .

echo "Message sent to EXT SMTS. Check INT SMTS logs for message processing."
```

### Example 2: INT → EXT Message Flow

```bash
#!/bin/bash
echo "=== INT → EXT Message Flow Demo ==="

# 1. Send message to INT SMTS
MESSAGE_ID="int-to-ext-$(date +%s)"
echo "Sending message to INT SMTS: $MESSAGE_ID"

curl -X POST "http://localhost:18094/test.monterra.event" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: test-api-key" \
  -H "X-SMTS-Message-ID: $MESSAGE_ID" \
  -H "X-SMTS-Timestamp: $(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  -H "X-SMTS-Source: int-client" \
  -H "smts-role: int_writer" \
  -d '{
    "event_type": "int_to_ext_demo",
    "message_id": "'$MESSAGE_ID'",
    "direction": "INT → EXT",
    "data": {
      "validation": "DLP approved",
      "status": "processed",
      "details": {
        "dlp_scan": "clean",
        "artemis_delivery": "success"
      }
    },
    "timestamp": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'"
  }' | jq .

echo "Message sent to INT SMTS. Check EXT SMTS logs for message processing."
```

## Message API Test Endpoints

### Send Test Messages via Message API

```bash
# Send test message to EXT SMTS Message API
curl -X POST "http://localhost:19081/send" \
  -H "Content-Type: application/json" \
  -d '{
    "topic": "test.monterra.event",
    "message": {
      "event_type": "manual_test",
      "message_id": "manual-$(date +%s)",
      "timestamp": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'",
      "test": true,
      "data": "Test message via Message API"
    }
  }'

# Send test message to INT SMTS Message API
curl -X POST "http://localhost:19083/send" \
  -H "Content-Type: application/json" \
  -d '{
    "topic": "test.monterra.event",
    "message": {
      "event_type": "manual_test",
      "message_id": "manual-$(date +%s)",
      "timestamp": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'",
      "test": true,
      "data": "Test message via Message API"
    }
  }'
```

## Health Checks

```bash
# Check EXT SMTS health
curl -s http://localhost:18091/health | jq .

# Check INT SMTS health
curl -s http://localhost:18093/health | jq .
```

## Required Headers for HTTP Publishing

When sending messages via the HTTP Publishing API, the following headers are required:

- `Content-Type: application/json` - Message format
- `X-API-Key: test-api-key` - API key for authentication
- `X-SMTS-Message-ID: {unique-id}` - Unique message identifier
- `X-SMTS-Timestamp: {timestamp}` - Message timestamp (RFC3339)
- `X-SMTS-Source: {source}` - Message source identifier
- `smts-role: {role}` - Role for topic authorization (ext_writer/int_writer)

## Response Format

### Successful Message Delivery
```json
{
  "status": "delivered",
  "message_id": "test-1759415221",
  "topic": "test.monterra.event",
  "timestamp": "2025-10-02T14:27:01Z",
  "deployment": "ext"
}
```

### Message API Response
```json
{
  "api": "message-api",
  "count": 5,
  "deployment": "ext",
  "messages": [
    {
      "id": "test-1759415221",
      "topic": "test.monterra.event",
      "timestamp": "2025-10-02T14:27:01Z",
      "source": "demo-client",
      "headers": {
        "X-SMTS-Message-ID": "test-1759415221",
        "X-SMTS-Timestamp": "2025-10-02T14:27:01Z",
        "X-SMTS-Source": "demo-client",
        "smts-role": "ext_writer"
      },
      "body": {
        "event_type": "demo_message",
        "message": "Test message via new HTTP endpoint",
        "timestamp": "2025-10-02T14:27:01Z"
      }
    }
  ],
  "received": 1,
  "timestamp": "2025-10-02T14:29:40Z",
  "topic": "test.monterra.event"
}
```

## Available Scripts

### Individual Client Scripts
- `./ext-client-send.sh` - Send message to EXT SMTS
- `./int-client-send.sh` - Send message to INT SMTS
- `./ext-client-receive.sh` - Receive messages from EXT SMTS
- `./int-client-receive.sh` - Receive messages from INT SMTS

### Complete Flow Testing
- `./test-message-flow.sh` - Test complete message flow from EXT to INT SMTS

## Running the Complete Demo

Use the provided demo script to test the complete bidirectional flow:

```bash
# Make all scripts executable
chmod +x *.sh

# Test complete message flow
./test-message-flow.sh

# Or test individual components
./ext-client-send.sh
./int-client-receive.sh
```

## Troubleshooting

### Common Issues

1. **Port Connection Issues**: Ensure all services are running and ports are properly exposed
2. **Authentication Errors**: Verify the `X-API-Key` and `smts-role` headers are correct
3. **Topic Authorization**: Check that the role has write permissions for the topic
4. **Message Not Appearing**: Messages are processed by consumers and may not remain in the stream

### Checking Logs

```bash
# Check EXT SMTS logs
docker-compose -f docker-compose.test.yml logs smts-ext-test

# Check INT SMTS logs
docker-compose -f docker-compose.test.yml logs smts-int-test

# Check for specific message processing
docker-compose -f docker-compose.test.yml logs smts-ext-test | grep "test-1759415221"
```

## Architecture Notes

- Each SMTS instance has its own embedded NATS server for message queuing
- Messages sent to EXT SMTS are delivered to the corporate API (simulated by mock server)
- Messages sent to INT SMTS go through DLP validation before delivery
- The Message API reads from the embedded NATS streams for testing and monitoring
- Full bidirectional transportation requires corporate API connectivity between environments