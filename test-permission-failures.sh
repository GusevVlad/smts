#!/bin/bash
# test-permission-failures.sh - Test LDAP authorization failures for users without proper roles
# This script tests that users without required LDAP groups get 403 Forbidden responses

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
TOPIC_MONTERRA="${TOPIC_MONTERRA:-test.monterra.event}"

# LDAP credentials for testing - using a user WITHOUT proper roles
# This user should exist in LDAP but NOT be in any of the required groups
LDAP_USERNAME="${LDAP_USERNAME_NO_ROLES:-norolesuser}"
LDAP_PASSWORD="${LDAP_PASSWORD_NO_ROLES:-norolespass}"
LDAP_AUTH_HEADER="Basic $(printf "%s" "$LDAP_USERNAME:$LDAP_PASSWORD" | base64)"

echo "=== Testing LDAP Authorization Failures ==="
echo "Script location: $SCRIPT_DIR"
echo "LDAP Username (no roles): $LDAP_USERNAME"
echo "Testing with user that has NO required LDAP groups"
echo

# Test result tracking variables
EXT_SEND_FAILED=false
INT_SEND_FAILED=false
EXT_RECEIVE_FAILED=false
INT_RECEIVE_FAILED=false
EXT_PROCESSED_FAILED=false
INT_PROCESSED_FAILED=false
SERVICES_HEALTHY=false

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

# Function to test endpoint with expected failure (403 Forbidden)
test_endpoint_failure() {
    local method=$1
    local url=$2
    local description=$3
    local payload=$4
    
    echo "=== Testing $description ==="
    echo "Expected: 403 Forbidden (Authorization failed - insufficient permissions)"
    echo "URL: $url"
    echo "Method: $method"
    
    if [ "$method" = "POST" ] && [ -n "$payload" ]; then
        echo "Payload: $payload"
        RESPONSE=$(curl -s -w "\nHTTP_CODE:%{http_code}" -X "$method" "$url" \
          -H "Content-Type: application/json" \
          -H "Authorization: $LDAP_AUTH_HEADER" \
          -d "$payload")
    else
        RESPONSE=$(curl -s -w "\nHTTP_CODE:%{http_code}" -X "$method" "$url" \
          -H "Content-Type: application/json" \
          -H "Authorization: $LDAP_AUTH_HEADER")
    fi
    
    # Extract HTTP status code
    HTTP_CODE=$(echo "$RESPONSE" | grep "HTTP_CODE:" | cut -d':' -f2)
    RESPONSE_BODY=$(echo "$RESPONSE" | grep -v "HTTP_CODE:")
    
    echo "HTTP Status: $HTTP_CODE"
    echo "Response: $RESPONSE_BODY"
    
    if [ "$HTTP_CODE" -eq 403 ]; then
        echo "✅ SUCCESS: Correctly received 403 Forbidden (authorization denied)"
        return 0
    elif [ "$HTTP_CODE" -eq 401 ]; then
        echo "⚠️  Authentication failed (401) - user may not exist in LDAP"
        return 1
    else
        echo "❌ FAILED: Expected 403 Forbidden but got $HTTP_CODE"
        echo "   This might indicate the user has permissions they shouldn't have"
        return 1
    fi
    echo
}

# Main execution
echo "1. Checking if services are running..."
echo "   EXT SMTS Health: http://localhost:18091/health"
echo "   INT SMTS Health: http://localhost:18093/health"
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

echo
echo "=== Testing Authorization Failures for User Without Roles ==="
echo "User: $LDAP_USERNAME"
echo "This user should exist in LDAP but NOT be in any required groups"
echo

# Test EXT SMTS endpoints
echo "2. Testing EXT SMTS endpoints with user without roles..."

test_endpoint_failure "POST" "$API_BASE_URL_EXT/send?topic=$TOPIC_MONTERRA" \
  "EXT SMTS Send endpoint" \
  '{"env":"INT","app_name":"test","app_version":"1.0.0","auto_system":"TEST","metrics":[],"report_date":"2024-01-01T00:00:00Z","report_type":"test"}' && {
    EXT_SEND_FAILED=true
}

test_endpoint_failure "GET" "$API_BASE_URL_EXT/receive?topic=$TOPIC_MONTERRA&count=1" \
  "EXT SMTS Receive endpoint" && {
    EXT_RECEIVE_FAILED=true
}

test_endpoint_failure "POST" "$API_BASE_URL_EXT/processed?topic=$TOPIC_MONTERRA" \
  "EXT SMTS Processed endpoint" \
  '{"processed":"test-message-id"}' && {
    EXT_PROCESSED_FAILED=true
}

echo
echo "3. Testing INT SMTS endpoints with user without roles..."

test_endpoint_failure "POST" "$API_BASE_URL_INT/send?topic=$TOPIC_MONTERRA" \
  "INT SMTS Send endpoint" \
  '{"env":"EXT","app_version":"1.0.0","auto_system":"TEST","metrics":[],"report_date":"2024-01-01T00:00:00Z","report_type":"test"}' && {
    INT_SEND_FAILED=true
}

test_endpoint_failure "GET" "$API_BASE_URL_INT/receive?topic=$TOPIC_MONTERRA&count=1" \
  "INT SMTS Receive endpoint" && {
    INT_RECEIVE_FAILED=true
}

test_endpoint_failure "POST" "$API_BASE_URL_INT/processed?topic=$TOPIC_MONTERRA" \
  "INT SMTS Processed endpoint" \
  '{"processed":"test-message-id"}' && {
    INT_PROCESSED_FAILED=true
}

echo
echo "=== Test Complete ==="
echo
echo "=== Summary of Ready-to-use Curl Commands for Permission Testing ==="
echo
echo "1. Test EXT SMTS Send endpoint failure:"
echo "   curl -X POST \"$API_BASE_URL_EXT/send?topic=$TOPIC_MONTERRA\" \\"
echo "     -H \"Content-Type: application/json\" \\"
echo "     -H \"Authorization: $LDAP_AUTH_HEADER\" \\"
echo "     -d '{\"env\":\"INT\",\"app_name\":\"test\",\"app_version\":\"1.0.0\",\"auto_system\":\"TEST\",\"metrics\":[],\"report_date\":\"2024-01-01T00:00:00Z\",\"report_type\":\"test\"}'"
echo
echo "2. Test EXT SMTS Receive endpoint failure:"
echo "   curl -X GET \"$API_BASE_URL_EXT/receive?topic=$TOPIC_MONTERRA&count=1\" \\"
echo "     -H \"Content-Type: application/json\" \\"
echo "     -H \"Authorization: $LDAP_AUTH_HEADER\""
echo
echo "3. Test INT SMTS Send endpoint failure:"
echo "   curl -X POST \"$API_BASE_URL_INT/send?topic=$TOPIC_MONTERRA\" \\"
echo "     -H \"Content-Type: application/json\" \\"
echo "     -H \"Authorization: $LDAP_AUTH_HEADER\" \\"
echo "     -d '{\"env\":\"EXT\",\"app_version\":\"1.0.0\",\"auto_system\":\"TEST\",\"metrics\":[],\"report_date\":\"2024-01-01T00:00:00Z\",\"report_type\":\"test\"}'"
echo
echo "4. Test INT SMTS Receive endpoint failure:"
echo "   curl -X GET \"$API_BASE_URL_INT/receive?topic=$TOPIC_MONTERRA&count=1\" \\"
echo "     -H \"Content-Type: application/json\" \\"
echo "     -H \"Authorization: $LDAP_AUTH_HEADER\""
echo
echo "=== Important Notes ==="
echo "- This script tests users WITHOUT proper LDAP group memberships"
echo "- The test user '$LDAP_USERNAME' should exist in LDAP but NOT be in any required groups"
echo "- Expected behavior: All requests should return 403 Forbidden"
echo "- If you get 401 Unauthorized, the user may not exist in LDAP"
echo "- If you get 200 OK, the user may have permissions they shouldn't have"
echo "- Configure test user credentials in tests/examples/.env:"
echo "   LDAP_USERNAME_NO_ROLES=norolesuser"
echo "   LDAP_PASSWORD_NO_ROLES=norolespass"
echo
echo "=== Test Summary ==="
echo "Test Results:"
echo "  ✅ Services Health Check: $([ "$SERVICES_HEALTHY" = true ] && echo "PASS" || echo "FAIL")"
echo "  ✅ EXT SMTS Send Failed: $([ "$EXT_SEND_FAILED" = true ] && echo "PASS" || echo "FAIL")"
echo "  ✅ EXT SMTS Receive Failed: $([ "$EXT_RECEIVE_FAILED" = true ] && echo "PASS" || echo "FAIL")"
echo "  ✅ EXT SMTS Processed Failed: $([ "$EXT_PROCESSED_FAILED" = true ] && echo "PASS" || echo "FAIL")"
echo "  ✅ INT SMTS Send Failed: $([ "$INT_SEND_FAILED" = true ] && echo "PASS" || echo "FAIL")"
echo "  ✅ INT SMTS Receive Failed: $([ "$INT_RECEIVE_FAILED" = true ] && echo "PASS" || echo "FAIL")"
echo "  ✅ INT SMTS Processed Failed: $([ "$INT_PROCESSED_FAILED" = true ] && echo "PASS" || echo "FAIL")"
echo
echo "Overall Status: $([ "$EXT_SEND_FAILED" = true ] && [ "$EXT_RECEIVE_FAILED" = true ] && [ "$EXT_PROCESSED_FAILED" = true ] && [ "$INT_SEND_FAILED" = true ] && [ "$INT_RECEIVE_FAILED" = true ] && [ "$INT_PROCESSED_FAILED" = true ] && echo "✅ ALL PERMISSION TESTS PASSED" || echo "❌ SOME PERMISSION TESTS FAILED")"
echo
echo "=== Next Steps ==="
echo "1. Run 'test-message-flow-ldap.sh' to test with a user WITH proper roles"
echo "2. Compare results to verify authorization is working correctly"
echo "3. Check LDAP server logs to confirm group membership checks"