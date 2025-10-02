#!/bin/bash
# smts-complete-test.sh - Complete test of SMTS REST API

set -e  # Exit on any error

# Load environment variables
set -a
source .env
set +a

# Configuration
API_BASE_URL_EXT="${API_BASE_URL_EXT:-http://localhost:18080}"
API_BASE_URL_INT="${API_BASE_URL_INT:-http://localhost:18082}"
API_KEY="${API_KEY:-test-api-key}"
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
    log "Testing EXT health endpoint..."
    if curl -s -f -X GET "$HEALTH_EXT" > /dev/null; then
        pass "EXT Health check"
    else
        fail "EXT Health check"
    fi
    
    log "Testing INT health endpoint..."
    if curl -s -f -X GET "$HEALTH_INT" > /dev/null; then
        pass "INT Health check"
    else
        fail "INT Health check"
    fi
}

test_message_delivery() {
    local api_url=$1
    local topic=$2
    local role=$3
    local source=$4
    
    log "Testing message delivery to $topic via $api_url..."
    
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
    
    local response=$(curl -s -w "\n%{http_code}" -X POST "$api_url/$topic" \
      -H "Content-Type: application/json" \
      -H "X-API-Key: $API_KEY" \
      -H "X-SMTS-Message-ID: $message_id" \
      -H "X-SMTS-Timestamp: $timestamp" \
      -H "X-SMTS-Source: $source" \
      -H "smts-role: ${role}_writer" \
      -d "$payload")
    
    local http_code=$(echo "$response" | tail -n1)
    
    if [ "$http_code" -eq 200 ] || [ "$http_code" -eq 201 ]; then
        pass "Message delivery to $topic via $api_url"
    else
        fail "Message delivery to $topic via $api_url (HTTP $http_code)"
    fi
}

# Main test execution
echo "Starting SMTS REST API Tests"
echo "============================"
echo "EXT API Base URL: $API_BASE_URL_EXT"
echo "INT API Base URL: $API_BASE_URL_INT"
echo "Timeout: ${TEST_TIMEOUT}s"
echo ""

# Run tests
test_health
test_message_delivery "$API_BASE_URL_EXT" "${TOPIC_MONTERRA:-test.monterra.event}" "ext" "test-ext-client"
test_message_delivery "$API_BASE_URL_INT" "${TOPIC_PACT_UPDATE:-test.pact_update.event}" "int" "test-int-client"

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