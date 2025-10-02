#!/bin/bash
# int-client-receive.sh - Internal client receiving messages from INT SMTS

# Load environment variables
if [ -f .env ]; then
    set -a
    source .env
    set +a
fi

# Configuration
MESSAGE_API_URL="${MESSAGE_API_INT:-http://localhost:19083}"
TOPIC="${TOPIC_MONTERRA:-test.monterra.event}"
COUNT="${1:-1}"  # Default to 1 message if not specified

echo "Receiving messages from INT SMTS..."
echo "Message API: $MESSAGE_API_URL"
echo "Topic: $TOPIC"
echo "Count: $COUNT"

# Receive messages
echo "Ready-to-use curl command:"
echo "curl -X GET \"$MESSAGE_API_URL/messages?topic=$TOPIC&count=$COUNT\" \\"
echo "  -H \"Content-Type: application/json\" \\"
echo "  -H \"X-API-Key: $API_KEY\" \\"
echo "  -H \"smts-role: int_reader\""
echo ""

RESPONSE=$(curl -s -w "\nHTTP_CODE:%{http_code}" -X GET "$MESSAGE_API_URL/messages?topic=$TOPIC&count=$COUNT" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: $API_KEY" \
  -H "smts-role: int_reader")

# Extract HTTP status code
HTTP_CODE=$(echo "$RESPONSE" | grep "HTTP_CODE:" | cut -d':' -f2)
RESPONSE_BODY=$(echo "$RESPONSE" | grep -v "HTTP_CODE:")

echo "HTTP Status: $HTTP_CODE"
echo "Response:"
echo "$RESPONSE_BODY" | jq . 2>/dev/null || echo "$RESPONSE_BODY"

if [ "$HTTP_CODE" -eq 200 ]; then
    echo "✅ Messages retrieved successfully!"
else
    echo "❌ Failed to retrieve messages (HTTP $HTTP_CODE)"
    exit 1
fi