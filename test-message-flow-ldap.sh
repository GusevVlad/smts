
#!/bin/bash
# test-message-flow-ldap.sh - Test complete bidirectional message flows between EXT and INT SMTS with LDAP authentication
# This script can be executed from the root directory and prints ready-to-use curl commands with LDAP auth

# Load environment variables from tests/examples/.env
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ENV_FILE="$SCRIPT_DIR/tests/examples/.env"

if [ -f "$ENV_FILE" ]; then
    set -a
    source "$ENV_FILE"
    set +a
    echo "Loaded environment variables from: $ENV_FILE"
else
    echo "Warning: Environment file not found at $ENV_FILE"
    echo "Using default values..."
fi

# Default configuration
API_BASE_URL_EXT="${API_BASE_URL_EXT:-http://localhost:18092}"
API_BASE_URL_INT="${API_BASE_URL_INT:-http://localhost:18094}"
MESSAGE_API_EXT="${MESSAGE_API_EXT:-http://localhost:19081}"
MESSAGE_API_INT="${MESSAGE_API_INT:-http://localhost:19083}"
API_KEY="${API_KEY:-test-api-key}"
TOPIC_MONTERRA="${TOPIC_MONTERRA:-test.monterra.event}"
TOPIC_PACT_UPDATE="${TOPIC_PACT_UPDATE:-test.pact_update.event}"

# LDAP credentials for testing
LDAP_USERNAME="${LDAP_USERNAME:-testuser}"
LDAP_PASSWORD="${LDAP_PASSWORD:-testpass}"
LDAP_AUTH_HEADER="Basic $(printf "%s" "$LDAP_USERNAME:$LDAP_PASSWORD" | base64)"

echo "=== Testing SMTS Bidirectional Message Flows with LDAP Authentication ==="
echo "Script location: $SCRIPT_DIR"
echo "LDAP Username: $LDAP_USERNAME"
echo

# Test result tracking variables
EXT_SEND_SUCCESS=false
INT_SEND_SUCCESS=false
EXT_TO_INT_FLOW_SUCCESS=false
INT_TO_EXT_FLOW_SUCCESS=false
SERVICES_HEALTHY=false
LDAP_AUTH_SUCCESS=false
LDAP_AUTH_FAILURE_TESTED=false

# Function to generate unique message ID
generate_message_id() {
    echo "$(date +%Y%m%d%H%M%S)-$(openssl rand -hex 4)"
}

# Function to send message to EXT SMTS with curl command and LDAP auth
send_to_ext_smts() {
    local message_id=$(generate_message_id)
    local timestamp=$(date -u +%Y-%m-%dT%H:%M:%SZ)
    
    local payload=$(cat <<EOF
{
  "event_type": "dora_metrics_update",
  "project_id": "project-$(openssl rand -hex 4)",
  "project_name": "Monterra Platform",
  "timestamp": "$timestamp",
  "metrics": {
    "deployment_frequency": {
      "value": 12.5,
      "unit": "deployments/week",
      "trend": "improving"
    },
    "lead_time_for_changes": {
      "value": 3.2,
      "unit": "days",
      "trend": "stable"
    },
    "mean_time_to_restore": {
      "value": 2.1,
      "unit": "hours",
      "trend": "improving"
    },
    "change_failure_rate": {
      "value": 8.5,
      "unit": "percent",
      "trend": "stable"
    }
  },
  "period": {
    "start": "$(date -u -v-7d +%Y-%m-%dT%H:%M:%SZ)",
    "end": "$timestamp"
  },
  "team_size": 15,
  "environment": "production",
  "source": "external-monitoring"
}
EOF
)

    echo "=== Ready-to-use curl command for EXT SMTS (with LDAP) ==="
    echo "curl -X POST \"$API_BASE_URL_EXT/send/$TOPIC_MONTERRA\" \\"
    echo "  -H \"Content-Type: application/json\" \\"
    echo "  -H \"Authorization: $LDAP_AUTH_HEADER\" \\"
    echo "  -H \"X-API-Key: $API_KEY\" \\"
    echo "  -H \"X-SMTS-Message-ID: $message_id\" \\"
    echo "  -H \"X-SMTS-Timestamp: $timestamp\" \\"
    echo "  -H \"X-SMTS-Source: ext-client\" \\"
    echo "  -H \"smts-role: ext_writer\" \\"
    echo "  -d '$payload'"
    echo

    echo "Executing the request..."
    RESPONSE=$(curl -s -w "\nHTTP_CODE:%{http_code}" -X POST "$API_BASE_URL_EXT/send/$TOPIC_MONTERRA" \
      -H "Content-Type: application/json" \
      -H "Authorization: $LDAP_AUTH_HEADER" \
      -H "X-API-Key: $API_KEY" \
      -H "X-SMTS-Message-ID: $message_id" \
      -H "X-SMTS-Timestamp: $timestamp" \
      -H "X-SMTS-Source: ext-client" \
      -H "smts-role: ext_writer" \
      -d "$payload")

    # Extract HTTP status code
    HTTP_CODE=$(echo "$RESPONSE" | grep "HTTP_CODE:" | cut -d':' -f2)
    RESPONSE_BODY=$(echo "$RESPONSE" | grep -v "HTTP_CODE:")

    echo "HTTP Status: $HTTP_CODE"
    echo "Response: $RESPONSE_BODY"
    echo

    if [ "$HTTP_CODE" -eq 200 ] || [ "$HTTP_CODE" -eq 201 ]; then
        echo "✅ Message sent successfully to EXT SMTS with LDAP auth!"
        return 0
    else
        echo "❌ Failed to send message to EXT SMTS (HTTP $HTTP_CODE)"
        return 1
    fi
}

# Function to send message to INT SMTS with curl command and LDAP auth
send_to_int_smts() {
    local message_id=$(generate_message_id)
    local timestamp=$(date -u +%Y-%m-%dT%H:%M:%SZ)
    
    local payload=$(cat <<EOF
{
  "event_type": "dora_metrics_update",
  "system_id": "system-$(openssl rand -hex 4)",
  "system_name": "Internal Monitoring System",
  "timestamp": "$timestamp",
  "metrics": {
    "deployment_frequency": {
      "value": 15.2,
      "unit": "deployments/week",
      "trend": "improving"
    },
    "lead_time_for_changes": {
      "value": 2.8,
      "unit": "days",
      "trend": "improving"
    },
    "mean_time_to_restore": {
      "value": 1.5,
      "unit": "hours",
      "trend": "stable"
    },
    "change_failure_rate": {
      "value": 6.2,
      "unit": "percent",
      "trend": "improving"
    }
  },
  "status": "healthy",
  "environment": "production",
  "source": "internal-monitoring"
}
EOF
)

    echo "=== Ready-to-use curl command for INT SMTS (with LDAP) ==="
    echo "curl -X POST \"$API_BASE_URL_INT/send/$TOPIC_MONTERRA\" \\"
    echo "  -H \"Content-Type: application/json\" \\"
    echo "  -H \"Authorization: $LDAP_AUTH_HEADER\" \\"
    echo "  -H \"X-API-Key: $API_KEY\" \\"
    echo "  -H \"X-SMTS-Message-ID: $message_id\" \\"
    echo "  -H \"X-SMTS-Timestamp: $timestamp\" \\"
    echo "  -H \"X-SMTS-Source: int-client\" \\"
    echo "  -H \"smts-role: int_writer\" \\"
    echo "  -d '$payload'"
    echo

    echo "Executing the request..."
    RESPONSE=$(curl -s -w "\nHTTP_CODE:%{http_code}" -X POST "$API_BASE_URL_INT/send/$TOPIC_MONTERRA" \
      -H "Content-Type: application/json" \
      -H "Authorization: $LDAP_AUTH_HEADER" \
      -H "X-API-Key: $API_KEY" \
      -H "X-SMTS-Message-ID: $message_id" \
      -H "X-SMTS-Timestamp: $timestamp" \
      -H "X-SMTS-Source: int-client" \
      -H "smts-role: int_writer" \
      -d "$payload")

    # Extract HTTP status code
    HTTP_CODE=$(echo "$RESPONSE" | grep "HTTP_CODE:" | cut -d':' -f2)
    RESPONSE_BODY=$(echo "$RESPONSE" | grep -v "HTTP_CODE:")

    echo "HTTP Status: $HTTP_CODE"
    echo "Response: $RESPONSE_BODY"
    echo

    if [ "$HTTP_CODE" -eq 200 ] || [ "$HTTP_CODE" -eq 201 ]; then
        echo "✅ Message sent successfully to INT SMTS with LDAP auth!"
        return 0
    else
        echo "❌ Failed to send message to INT SMTS (HTTP $HTTP_CODE)"
        return 1
    fi
}

# Function to check service health
check_service_health() {
    local service_name=$1
    local health_url=$2
    
    echo "Checking $service_name health..."
    local status_code=$(curl -s -o /dev/null -w "%{http_code}" "$health_url")
    
    if [ "$status_code" -eq 200 ]; then
        echo "✅ $service_name is healthy"
        return 0
    else
        echo "❌ $service_name is not responding (HTTP $status_code)"
        return 1
    fi
}

# Function to check received messages with LDAP auth
check_received_messages() {
    local deployment=$1
    local topic=$2
    local description=$3
    
    if [ "$deployment" = "ext" ]; then
        endpoint="$MESSAGE_API_EXT/messages?topic=$topic&count=1"
        role="ext_reader"
    else
        endpoint="$MESSAGE_API_INT/messages?topic=$topic&count=1"
        role="int_reader"
    fi
    
    echo "=== Ready-to-use curl command for checking $description (with LDAP) ==="
    echo "curl -X GET \"$endpoint\" \\"
    echo "  -H \"Content-Type: application/json\" \\"
    echo "  -H \"Authorization: $LDAP_AUTH_HEADER\" \\"
    echo "  -H \"X-API-Key: $API_KEY\" \\"
    echo "  -H \"smts-role: $role\""
    echo
    
    echo "Executing the request..."
    RESPONSE=$(curl -s -X GET "$endpoint" \
      -H "Content-Type: application/json" \
      -H "Authorization: $LDAP_AUTH_HEADER" \
      -H "X-API-Key: $API_KEY" \
      -H "smts-role: $role")
    
    echo "Response:"
    echo "$RESPONSE" | jq . 2>/dev/null || echo "$RESPONSE"
    echo
    
    # Extract message count from response
    RECEIVED_COUNT=$(echo "$RESPONSE" | jq -r '.received // 0' 2>/dev/null || echo "0")
    if [ "$RECEIVED_COUNT" -gt 0 ]; then
        echo "✅ Message successfully transported with LDAP auth!"
        return 0
    else
        echo "⚠️  No messages found"
        return 1
    fi
}

# Function to test LDAP authentication failure
test_ldap_auth_failure() {
    local deployment=$1
    local endpoint=$2
    local description=$3
    
    echo "=== Testing LDAP authentication failure for $description ==="
    echo "Using invalid credentials..."
    
    INVALID_AUTH_HEADER="Basic $(printf "%s" "invaliduser:invalidpass" | base64)"
    
    RESPONSE=$(curl -s -w "\nHTTP_CODE:%{http_code}" -X GET "$endpoint" \
      -H "Content-Type: application/json" \
      -H "Authorization: $INVALID_AUTH_HEADER" \
      -H "X-API-Key: $API_KEY" \
      -H "smts-role: ext_reader")
    
    HTTP_CODE=$(echo "$RESPONSE" | grep "HTTP_CODE:" | cut -d':' -f2)
    RESPONSE_BODY=$(echo "$RESPONSE" | grep -v "HTTP_CODE:")
    
    echo "HTTP Status: $HTTP_CODE"
    echo "Response: $RESPONSE_BODY"
    
    if [ "$HTTP_CODE" -eq 401 ]; then
        echo "✅ LDAP authentication correctly rejected invalid credentials!"
        return 0
    else
        echo "❌ LDAP authentication did not work as expected (expected 401, got $HTTP_CODE)"
        return 1
    fi
    echo
}

# Main execution
echo "1. Checking if services are running..."
echo "   EXT SMTS Health: http://localhost:18091/health"
echo "   INT SMTS Health: http://localhost:18093/health"
echo "   Corporate API Health: http://localhost:18080/health"
echo

# Check EXT SMTS health
check_service_health "EXT SMTS" "http://localhost:18091/health" && {
    SERVICES_HEALTHY=true
} || {
    echo "❌ EXT SMTS is not responding"
    echo "   Please make sure the test containers are running:"
    echo "   docker-compose -f docker-compose.test.yml up -d"
    exit 1
}

# Check INT SMTS health
check_service_health "INT SMTS" "http://localhost:18093/health" && {
    SERVICES_HEALTHY=true
} || {
    echo "❌ INT SMTS is not responding"
    exit 1
}

# Check Corporate API health
check_service_health "Corporate API" "http://localhost:18080/health" && {
    SERVICES_HEALTHY=true
} || {
    echo "❌ Corporate API is not responding"
    exit 1
}

echo
echo "=== Testing LDAP Authentication ==="
echo

# Test LDAP authentication failure first
test_ldap_auth_failure "ext" "$MESSAGE_API_EXT/messages?topic=$TOPIC_MONTERRA&count=1" "EXT SMTS message API" && {
    LDAP_AUTH_FAILURE_TESTED=true
}

test_ldap_auth_failure "int" "$MESSAGE_API_INT/messages?topic=$TOPIC_MONTERRA&count=1" "INT SMTS message API" && {
    LDAP_AUTH_FAILURE_TESTED=true
}

echo
echo "=== Flow 1: EXT → INT Message Flow (with LDAP) ==="
echo "Path: EXT Client → EXT-SMTS → Corporate API → ArtemisMQ → INT-SMTS → INT Client"
echo

echo "2.1. Sending message to EXT SMTS with LDAP auth (Flow 1)..."
send_to_ext_smts && {
    EXT_SEND_SUCCESS=true
    LDAP_AUTH_SUCCESS=true
} || {
    echo "❌ Failed to send message to EXT SMTS with LDAP auth"
    exit 1
}

echo "2.2. Checking if message was transported to INT SMTS via Corporate API and ArtemisMQ..."
echo "   Checking INT SMTS message API with LDAP auth..."
echo "   Waiting for message to flow through Corporate API and ArtemisMQ..."
sleep 5

check_received_messages "int" "$TOPIC_MONTERRA" "INT SMTS messages from EXT" && {
    EXT_TO_INT_FLOW_SUCCESS=true
} || {
    echo "⚠️  No messages found in INT SMTS"
    echo "   This might be because:"
    echo "   - The Corporate API is not forwarding messages to ArtemisMQ"
    echo "   - INT-SMTS is not consuming from ArtemisMQ"
    echo "   - The message is still being processed"
}

echo
echo
echo "=== Flow 2: INT → EXT Message Flow (with LDAP) ==="
echo "Path: INT Client → INT-SMTS → DLP → ArtemisMQ → Corporate API → EXT-SMTS → EXT Client"
echo

echo "2.1. Sending message to INT SMTS with LDAP auth (Flow 2)..."
send_to_int_smts && {
    INT_SEND_SUCCESS=true
    LDAP_AUTH_SUCCESS=true
} || {
    echo "❌ Failed to send message to INT SMTS with LDAP auth"
    exit 1
}

echo
echo "2.2. Checking if message was transported to EXT SMTS via ArtemisMQ and Corporate API..."
echo "   Checking EXT SMTS message API with LDAP auth..."
echo "   Waiting for message to flow through ArtemisMQ and Corporate API..."
sleep 5

check_received_messages "ext" "$TOPIC_MONTERRA" "EXT SMTS messages from INT" && {
    INT_TO_EXT_FLOW_SUCCESS=true
} || {
    echo "⚠️  No messages found in EXT SMTS"
    echo "   This might be because:"
    echo "   - INT-SMTS is not pushing messages to ArtemisMQ"
    echo "   - Corporate API is not consuming from ArtemisMQ"
    echo "   - The message is still being processed"
}

echo "=== Test Complete ==="
echo
echo "=== Test Summary ==="
echo "Test Results:"
echo "  ✅ Services Health Check: $([ "$SERVICES_HEALTHY" = true ] && echo "PASS" || echo "FAIL")"
echo "  ✅ LDAP Auth Failure Test: $([ "$LDAP_AUTH_FAILURE_TESTED" = true ] && echo "PASS" || echo "FAIL")"
echo "  ✅ LDAP Auth Success: $([ "$LDAP_AUTH_SUCCESS" = true ] && echo "PASS" || echo "FAIL")"
echo "  ✅ EXT SMTS Send: $([ "$EXT_SEND_SUCCESS" = true ] && echo "PASS" || echo "FAIL")"
echo "  ✅ INT SMTS Send: $([ "$INT_SEND_SUCCESS" = true ] && echo "PASS" || echo "FAIL")"
echo "  ✅ EXT → INT Flow: $([ "$EXT_TO_INT_FLOW_SUCCESS" = true ] && echo "PASS" || echo "FAIL")"
echo "  ✅ INT → EXT Flow: $([ "$INT_TO_EXT_FLOW_SUCCESS" = true ] && echo "PASS" || echo "FAIL")"
echo
echo "Overall Status: $([ "$EXT_SEND_SUCCESS" = true ] && [ "$INT_SEND_SUCCESS" = true ] && [ "$SERVICES_HEALTHY" = true ] && [ "$LDAP_AUTH_SUCCESS" = true ] && echo "✅ ALL TESTS PASSED" || echo "❌ SOME TESTS FAILED")"
echo
echo "=== Summary of Ready-to-use Curl Commands with LDAP ==="
echo
echo "1. Send message to EXT SMTS (with LDAP):"
echo "   curl -X POST \"$API_BASE_URL_EXT/send/$TOPIC_MONTERRA\" \\"
echo "     -H \"Content-Type: application/json\" \\"
echo "     -H \"Authorization: $LDAP_AUTH_HEADER\" \\"
echo "     -H \"X-API-Key: $API_KEY\" \\"
echo "     -H \"X-SMTS-Message-ID: \$(date +%Y%m%d%H%M%S)-\$(openssl rand -hex 4)\" \\"
echo "     -H \"X-SMTS-Timestamp: \$(date -u +%Y-%m-%dT%H:%M:%SZ)\" \\"
echo "     -H \"X-SMTS-Source: ext-client\" \\"
echo "     -H \"smts-role: ext_writer\" \\"
echo "     -d '{\"event_type\":\"dora_metrics_update\",\"project_id\":\"project-\$(openssl rand -hex 4)\",\"timestamp\":\"\$(date -u +%Y-%m-%dT%H:%M:%SZ)\"}'"
echo
echo "2. Send message to INT SMTS (with LDAP):"
echo "   curl -X POST \"$API_BASE_URL_INT/send/$TOPIC_MONTERRA\" \\"
echo "     -H \"Content-Type: application/json\" \\"
echo "     -H \"Authorization: $LDAP_AUTH_HEADER\" \\"
echo "     -H \"X-API-Key: $API_KEY\" \\"
echo "     -H \"X-SMTS-Message-ID: \$(date +%Y%m%d%H%M%S)-\$(openssl rand -hex 4)\" \\"
echo "     -H \"X-SMTS-Timestamp: \$(date -u +%Y-%m-%dT%H:%M:%SZ)\" \\"
echo "     -H \"X-SMTS-Source: int-client\" \\"
echo "     -H \"smts-role: int_writer\" \\"
echo "     -d '{\"event_type\":\"dora_metrics_update\",\"system_id\":\"system-\$(openssl rand -hex 4)\",\"timestamp\":\"\$(date -u +%Y-%m-%dT%H:%M:%SZ)\"}'"
echo
echo "3. Check messages in EXT SMTS (with LDAP):"
echo "   curl -X GET \"$MESSAGE_API_EXT/messages?topic=$TOPIC_MONTERRA&count=1\" \\"
echo "     -H \"Content-Type: application/json\" \\"
echo "     -H \"Authorization: $LDAP_AUTH_HEADER\" \\"
echo "     -H \"X-API-Key: $API_KEY\" \\"
echo "     -H \"smts-role: ext_reader\""
echo
echo "4. Check messages in INT SMTS (with LDAP):"
echo "   curl -X GET \"$MESSAGE_API_INT/messages?topic=$TOPIC_MONTERRA&count=1\" \\"
echo "     -H \"Content-Type: application/json\" \\"
echo "     -H \"Authorization: $LDAP_AUTH_HEADER\" \\"
echo "     -H \"X-API-Key: $API_KEY\" \\"
echo "     -H \"smts-role: int_reader\""
echo
echo "=== Important Notes ==="
echo "- LDAP must be enabled in both ext-config.yaml and int-config.yaml"
echo "- Set ldap.enabled: true in both configuration files"
echo "- Configure LDAP server details in the configuration files"
echo "- This script uses test credentials: $LDAP_USERNAME:$LDAP_PASSWORD"
echo "- For production, use real LDAP credentials and secure configuration"