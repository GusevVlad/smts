#!/bin/bash
# test-signature-topic.sh - Test message flow with signature topic mapping

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
LDAP_USERNAME="${LDAP_USERNAME:-testuser}"
LDAP_PASSWORD="${LDAP_PASSWORD:-testpass}"
LDAP_AUTH_HEADER="Basic $(printf "%s" "$LDAP_USERNAME:$LDAP_PASSWORD" | base64)"

TOPIC="signature"

echo "=== Testing signature topic mapping ==="
echo "Script location: $SCRIPT_DIR"
echo "LDAP Username: $LDAP_USERNAME"
echo "Topic: $TOPIC"
echo

# Function to generate unique message ID
generate_message_id() {
    echo "$(date +%Y%m%d%H%M%S)-$(openssl rand -hex 4)"
}

# Send message to EXT SMTS with LDAP auth
send_to_ext_smts() {
    local message_id=$(generate_message_id)
    local timestamp=$(date -u +%Y-%m-%dT%H:%M:%SZ)
    
    local payload=$(cat <<EOF
{
"content": "MEUCIQCUw46tibIDsqMFF7qwjl+3tJUId1/DOjFWbhNmbmsk/wIgNfOu+y/c6KQ+VKHBP395EE1UQbL4PNfcwpvKaPZAn8s=", "publicKey.content": "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0k109Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K"
}
EOF
)
    echo "=== Sending message to EXT SMTS with topic $TOPIC ==="
    echo "curl -X POST \"$API_BASE_URL_EXT/send?topic=$TOPIC\" \\"
    echo "  -H \"Content-Type: application/json\" \\"
    echo "  -H \"Authorization: $LDAP_AUTH_HEADER\" \\"
    echo "  -d '$payload'"
    echo

    echo "Executing the request..."
    RESPONSE=$(curl -s -w "\nHTTP_CODE:%{http_code}" -X POST "$API_BASE_URL_EXT/send?topic=$TOPIC" \
      -H "Content-Type: application/json" \
      -H "Authorization: $LDAP_AUTH_HEADER" \
      -d "$payload")

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

# Check corporate API logs for endpoint path
check_corporate_api_logs() {
    echo "=== Checking corporate API logs for endpoint path ==="
    docker logs corporate-api 2>&1 | grep -E "Message received via|Message forwarded to ArtemisMQ" | tail -5
    echo
}

# Main execution
send_to_ext_smts
check_corporate_api_logs

echo "=== Test complete ==="