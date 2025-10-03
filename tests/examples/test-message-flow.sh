#!/bin/bash
# test-message-flow.sh - Test complete bidirectional message flows between EXT and INT SMTS

# Load environment variables
if [ -f .env ]; then
    set -a
    source .env
    set +a
fi

echo "=== Testing SMTS Bidirectional Message Flows ==="
echo

# Check if services are running
echo "1. Checking if services are running..."
echo "   EXT SMTS Health: http://localhost:18091/health"
echo "   INT SMTS Health: http://localhost:18093/health"
echo "   Corporate API Health: http://localhost:18080/health"
echo

# Check EXT SMTS health
EXT_HEALTH=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:18091/health)
if [ "$EXT_HEALTH" -eq 200 ]; then
    echo "✅ EXT SMTS is healthy"
else
    echo "❌ EXT SMTS is not responding (HTTP $EXT_HEALTH)"
    echo "   Please make sure the test containers are running:"
    echo "   docker-compose -f docker-compose.test.yml up -d"
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

# Check Corporate API health
CORP_API_HEALTH=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:18080/health)
if [ "$CORP_API_HEALTH" -eq 200 ]; then
    echo "✅ Corporate API is healthy"
else
    echo "❌ Corporate API is not responding (HTTP $CORP_API_HEALTH)"
    exit 1
fi

echo
echo "=== Flow 1: EXT → INT Message Flow ==="
echo "Path: EXT Client → EXT-SMTS → Corporate API → ArtemisMQ → INT-SMTS → INT Client"
echo

echo "2.1. Sending message to EXT SMTS (Flow 1)..."
./ext-client-send.sh
if [ $? -ne 0 ]; then
    echo "❌ Failed to send message to EXT SMTS"
    exit 1
fi

echo "2.2. Checking if message was transported to INT SMTS via Corporate API and ArtemisMQ..."
echo "   Checking INT SMTS message API..."

# Wait a moment for message transportation through Corporate API and ArtemisMQ
echo "   Waiting for message to flow through Corporate API and ArtemisMQ..."
sleep 5

RESPONSE=$(curl -s -X GET "$MESSAGE_API_INT/messages?topic=$TOPIC_MONTERRA&count=1" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: $API_KEY" \
  -H "smts-role: int_reader")

echo "   Response:"
echo "$RESPONSE" | jq . 2>/dev/null || echo "$RESPONSE"

# Extract message count from response
RECEIVED_COUNT=$(echo "$RESPONSE" | jq -r '.received // 0' 2>/dev/null || echo "0")
if [ "$RECEIVED_COUNT" -gt 0 ]; then
    echo "✅ Message successfully transported to INT SMTS via Corporate API and ArtemisMQ!"
else
    echo "⚠️  No messages found in INT SMTS"
    echo "   This might be because:"
    echo "   - The Corporate API is not forwarding messages to ArtemisMQ"
    echo "   - INT-SMTS is not consuming from ArtemisMQ"
    echo "   - The message is still being processed"
fi

echo
echo
echo "=== Flow 2: INT → EXT Message Flow ==="
echo "Path: INT Client → INT-SMTS → DLP → ArtemisMQ → Corporate API → EXT-SMTS → EXT Client"
echo

echo "2.1. Sending message to INT SMTS (Flow 2)..."
./int-client-send.sh
if [ $? -ne 0 ]; then
    echo "❌ Failed to send message to INT SMTS"
    exit 1
fi


echo
echo "2.2. Checking if message was transported to EXT SMTS via ArtemisMQ and Corporate API..."
echo "   Checking EXT SMTS message API..."

# Wait a moment for message transportation through ArtemisMQ and Corporate API
echo "   Waiting for message to flow through ArtemisMQ and Corporate API..."
sleep 5

RESPONSE=$(curl -s -X GET "$MESSAGE_API_EXT/messages?topic=$TOPIC_MONTERRA&count=1" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: $API_KEY" \
  -H "smts-role: ext_reader")

echo "   Response:"
echo "$RESPONSE" | jq . 2>/dev/null || echo "$RESPONSE"

# Extract message count from response
RECEIVED_COUNT=$(echo "$RESPONSE" | jq -r '.received // 0' 2>/dev/null || echo "0")
if [ "$RECEIVED_COUNT" -gt 0 ]; then
    echo "✅ Message successfully transported to EXT SMTS via ArtemisMQ and Corporate API!"
else
    echo "⚠️  No messages found in EXT SMTS"
    echo "   This might be because:"
    echo "   - INT-SMTS is not pushing messages to ArtemisMQ"
    echo "   - Corporate API is not consuming from ArtemisMQ"
    echo "   - The message is still being processed"
fi

echo "=== Test Complete ==="
