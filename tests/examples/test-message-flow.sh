#!/bin/bash
# test-message-flow.sh - Test complete message flow from EXT to INT SMTS

# Load environment variables
if [ -f .env ]; then
    set -a
    source .env
    set +a
fi

echo "=== Testing SMTS Message Flow ==="
echo

# Check if services are running
echo "1. Checking if services are running..."
echo "   EXT SMTS Health: http://localhost:18091/health"
echo "   INT SMTS Health: http://localhost:18093/health"
echo

# Check EXT SMTS health
EXT_HEALTH=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:18091/health)
if [ "$EXT_HEALTH" -eq 200 ]; then
    echo "✅ EXT SMTS is healthy"
else
    echo "❌ EXT SMTS is not responding (HTTP $EXT_HEALTH)"
    echo "   Please make sure the test containers are running:"
    echo "   docker-compose -f docker-compose.test.yml up -d smts-ext-test smts-int-test"
    exit 1
fi

# Check INT SMTS health
INT_HEALTH=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:18093/health)
if [ "$INT_HEALTH" -eq 200 ]; then
    echo "✅ INT SMTS is healthy"
else
    echo "❌ INT SMTS is not responding (HTTP $INT_HEALTH)"
    exit 1
fi

echo
echo "2. Sending message to EXT SMTS..."
./ext-client-send.sh
if [ $? -ne 0 ]; then
    echo "❌ Failed to send message to EXT SMTS"
    exit 1
fi

echo
echo "3. Checking if message was received by EXT SMTS..."
echo "   Checking EXT SMTS message API..."
RESPONSE=$(curl -s -X GET "$MESSAGE_API_EXT/messages?topic=$TOPIC_MONTERRA&count=1" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: $API_KEY" \
  -H "smts-role: ext_reader")

echo "   Response:"
echo "$RESPONSE" | jq . 2>/dev/null || echo "$RESPONSE"

# Extract message count from response
RECEIVED_COUNT=$(echo "$RESPONSE" | jq -r '.received // 0' 2>/dev/null || echo "0")
if [ "$RECEIVED_COUNT" -gt 0 ]; then
    echo "✅ Message received by EXT SMTS"
else
    echo "❌ No messages found in EXT SMTS"
    echo "   This might be because:"
    echo "   - The message wasn't published to NATS"
    echo "   - The consumer already processed and acknowledged the message"
    echo "   - There's an issue with the message API"
fi

echo
echo "4. Checking if message was transported to INT SMTS..."
echo "   This requires the corporate API to transport messages from EXT to INT"
echo "   Checking INT SMTS message API..."

# Wait a moment for potential message transportation
sleep 2

RESPONSE=$(curl -s -X GET "$MESSAGE_API_INT/messages?topic=$TOPIC_MONTERRA&count=1" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: $API_KEY" \
  -H "smts-role: int_reader")

echo "   Response:"
echo "$RESPONSE" | jq . 2>/dev/null || echo "$RESPONSE"

# Extract message count from response
RECEIVED_COUNT=$(echo "$RESPONSE" | jq -r '.received // 0' 2>/dev/null || echo "0")
if [ "$RECEIVED_COUNT" -gt 0 ]; then
    echo "✅ Message transported to INT SMTS!"
else
    echo "⚠️  No messages found in INT SMTS"
    echo "   This is expected if:"
    echo "   - The corporate API transportation is not configured"
    echo "   - The message is still being processed"
    echo "   - The test environment doesn't have message transportation enabled"
fi

echo
echo "=== Test Complete ==="
echo
echo "For bidirectional testing, you can also:"
echo "1. Send message to INT SMTS: ./int-client-send.sh"
echo "2. Receive from EXT SMTS: ./ext-client-receive.sh"
echo
echo "To see detailed logs, check the container logs:"
echo "docker logs smts-ext-test"
echo "docker logs smts-int-test"