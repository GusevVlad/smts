#!/bin/bash
# int-client-send.sh - Internal client sending message to INT SMTS

# Load environment variables
if [ -f .env ]; then
    set -a
    source .env
    set +a
fi

# Configuration
API_BASE_URL="${API_BASE_URL_INT:-http://localhost:18094}"
API_KEY="${API_KEY:-test-api-key}"
TOPIC="${TOPIC_MONTERRA:-test.monterra.event}"

# Generate unique message ID
MESSAGE_ID=$(date +%Y%m%d%H%M%S)-$(openssl rand -hex 4)
TIMESTAMP=$(date -u +%Y-%m-%dT%H:%M:%SZ)

# Message payload - Internal system metrics
PAYLOAD=$(cat <<EOF
{
  "event_type": "dora_metrics_update",
  "system_id": "system-$(openssl rand -hex 4)",
  "system_name": "Internal Monitoring System",
  "timestamp": "$TIMESTAMP",
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

echo "Sending message via INT SMTS..."
echo "Message ID: $MESSAGE_ID"
echo "Topic: $TOPIC"

# Send the message
echo "Ready-to-use curl command:"
echo "curl -X POST \"$API_BASE_URL/$TOPIC\" \\"
echo "  -H \"Content-Type: application/json\" \\"
echo "  -H \"X-API-Key: $API_KEY\" \\"
echo "  -H \"X-SMTS-Message-ID: $MESSAGE_ID\" \\"
echo "  -H \"X-SMTS-Timestamp: $TIMESTAMP\" \\"
echo "  -H \"X-SMTS-Source: int-client\" \\"
echo "  -H \"smts-role: int_writer\" \\"
echo "  -d '$PAYLOAD'"
echo ""

RESPONSE=$(curl -s -w "\nHTTP_CODE:%{http_code}" -X POST "$API_BASE_URL/$TOPIC" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: $API_KEY" \
  -H "X-SMTS-Message-ID: $MESSAGE_ID" \
  -H "X-SMTS-Timestamp: $TIMESTAMP" \
  -H "X-SMTS-Source: int-client" \
  -H "smts-role: int_writer" \
  -d "$PAYLOAD")

# Extract HTTP status code
HTTP_CODE=$(echo "$RESPONSE" | grep "HTTP_CODE:" | cut -d':' -f2)
RESPONSE_BODY=$(echo "$RESPONSE" | grep -v "HTTP_CODE:")

echo "HTTP Status: $HTTP_CODE"
echo "Response: $RESPONSE_BODY"

if [ "$HTTP_CODE" -eq 200 ] || [ "$HTTP_CODE" -eq 201 ]; then
    echo "✅ Message sent successfully!"
else
    echo "❌ Failed to send message (HTTP $HTTP_CODE)"
    exit 1
fi