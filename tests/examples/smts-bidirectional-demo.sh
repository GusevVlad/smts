#!/bin/bash

# SMTS Bidirectional Message Flow Demo
# This script demonstrates sending messages between EXT and INT SMTS instances

set -e

echo "🚀 SMTS Bidirectional Message Flow Demo"
echo "========================================"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to log messages
log() {
    echo -e "${BLUE}[$(date +'%Y-%m-%d %H:%M:%S')]${NC} $1"
}

# Function to log success
success() {
    echo -e "${GREEN}✅ $1${NC}"
}

# Function to log error
error() {
    echo -e "${RED}❌ $1${NC}"
}

# Function to log warning
warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

# Check if services are running
check_services() {
    log "Checking if SMTS services are running..."
    
    if curl -s http://localhost:18091/health > /dev/null; then
        success "EXT SMTS health check passed"
    else
        error "EXT SMTS health check failed"
        exit 1
    fi
    
    if curl -s http://localhost:18093/health > /dev/null; then
        success "INT SMTS health check passed"
    else
        error "INT SMTS health check failed"
        exit 1
    fi
    
    if curl -s http://localhost:19081/messages?topic=test.monterra.event > /dev/null; then
        success "EXT Message API is accessible"
    else
        error "EXT Message API is not accessible"
        exit 1
    fi
    
    if curl -s http://localhost:19083/messages?topic=test.monterra.event > /dev/null; then
        success "INT Message API is accessible"
    else
        error "INT Message API is not accessible"
        exit 1
    fi
}

# Demo 1: Send message to EXT SMTS and receive via INT SMTS
demo_ext_to_int() {
    echo
    log "Demo 1: EXT → INT Message Flow"
    echo "-----------------------------"
    
    # Create test message
    MESSAGE_ID="ext-to-int-$(date +%s)"
    TIMESTAMP=$(date -u +%Y-%m-%dT%H:%M:%SZ)
    
    log "Sending message to EXT SMTS..."
    
    # Send message to EXT SMTS (this would normally go through corporate API)
    # For demo purposes, we'll simulate this by publishing directly to NATS
    # and then checking if it appears in INT SMTS
    
    # First, let's check current state
    log "Checking current messages in EXT SMTS..."
    EXT_MESSAGES=$(curl -s "http://localhost:19081/messages?topic=test.monterra.event&count=5")
    echo "$EXT_MESSAGES" | jq .
    
    log "Checking current messages in INT SMTS..."
    INT_MESSAGES=$(curl -s "http://localhost:19083/messages?topic=test.monterra.event&count=5")
    echo "$INT_MESSAGES" | jq .
    
    # Since we can't directly send via HTTP to the main server,
    # we'll use the message API send endpoint for testing
    log "Using Message API send endpoint for testing..."
    SEND_RESPONSE=$(curl -s -X POST "http://localhost:19081/send" \
        -H "Content-Type: application/json" \
        -d "{
            \"topic\": \"test.monterra.event\",
            \"message\": {
                \"event_type\": \"ext_to_int_demo\",
                \"message_id\": \"$MESSAGE_ID\",
                \"timestamp\": \"$TIMESTAMP\",
                \"direction\": \"EXT → INT\",
                \"test_data\": \"This message was sent from EXT and should be received by INT\"
            }
        }")
    
    echo "Send response: $SEND_RESPONSE"
    
    # Wait a moment for processing
    sleep 2
    
    # Check if message appears (note: this won't show real transportation yet)
    log "Checking for messages after send..."
    curl -s "http://localhost:19081/messages?topic=test.monterra.event&count=5" | jq .
}

# Demo 2: Send message to INT SMTS and receive via EXT SMTS
demo_int_to_ext() {
    echo
    log "Demo 2: INT → EXT Message Flow"
    echo "-----------------------------"
    
    MESSAGE_ID="int-to-ext-$(date +%s)"
    TIMESTAMP=$(date -u +%Y-%m-%dT%H:%M:%SZ)
    
    log "Using Message API send endpoint for INT SMTS testing..."
    SEND_RESPONSE=$(curl -s -X POST "http://localhost:19083/send" \
        -H "Content-Type: application/json" \
        -d "{
            \"topic\": \"test.monterra.event\",
            \"message\": {
                \"event_type\": \"int_to_ext_demo\",
                \"message_id\": \"$MESSAGE_ID\",
                \"timestamp\": \"$TIMESTAMP\",
                \"direction\": \"INT → EXT\",
                \"test_data\": \"This message was sent from INT and should be received by EXT\"
            }
        }")
    
    echo "Send response: $SEND_RESPONSE"
    
    # Wait a moment for processing
    sleep 2
    
    # Check if message appears
    log "Checking for messages after send..."
    curl -s "http://localhost:19083/messages?topic=test.monterra.event&count=5" | jq .
}

# Show ready-to-use curl commands
show_curl_commands() {
    echo
    log "Ready-to-use CURL Commands"
    echo "=========================="
    
    echo
    echo "1. Send message to EXT SMTS (via HTTP endpoint - if available):"
    cat << 'EOF'
curl -X POST "http://localhost:18081/test.monterra.event" \
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
EOF

    echo
    echo "2. Receive messages from EXT SMTS Message API:"
    echo 'curl -s "http://localhost:19081/messages?topic=test.monterra.event&count=5" | jq .'
    
    echo
    echo "3. Receive messages from INT SMTS Message API:"
    echo 'curl -s "http://localhost:19083/messages?topic=test.monterra.event&count=5" | jq .'
    
    echo
    echo "4. Send test message via Message API (EXT):"
    cat << 'EOF'
curl -X POST "http://localhost:19081/send" \
  -H "Content-Type: application/json" \
  -d '{
    "topic": "test.monterra.event",
    "message": {
      "event_type": "manual_test",
      "message_id": "manual-$(date +%s)",
      "timestamp": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'",
      "test": true
    }
  }'
EOF

    echo
    echo "5. Send test message via Message API (INT):"
    cat << 'EOF'
curl -X POST "http://localhost:19083/send" \
  -H "Content-Type: application/json" \
  -d '{
    "topic": "test.monterra.event",
    "message": {
      "event_type": "manual_test",
      "message_id": "manual-$(date +%s)",
      "timestamp": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'",
      "test": true
    }
  }'
EOF
}

# Main execution
main() {
    log "Starting SMTS Bidirectional Demo"
    
    check_services
    
    # Run demos
    demo_ext_to_int
    demo_int_to_ext
    
    # Show curl commands
    show_curl_commands
    
    echo
    success "Demo completed!"
    echo
    log "Note: For full bidirectional message transportation between isolated environments,"
    log "messages need to be sent through the corporate API endpoints that connect EXT and INT."
    log "This demo shows the Message API endpoints that can read from each environment's embedded NATS."
}

# Run main function
main "$@"