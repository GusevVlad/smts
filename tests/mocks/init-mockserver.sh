#!/bin/bash

# Initialize MockServer expectations for SMTS test environment

set -e

echo "Initializing MockServer expectations..."

# Determine which MockServer we're initializing based on environment
if [ "$1" = "api" ]; then
    echo "Initializing API MockServer..."
    MOCK_URL="http://api-mock:1081"
    
    # Configure API Mock expectations for EXT SMTS (port 1080)
    echo "Configuring API Mock expectations for EXT SMTS..."

    # Health check endpoint
    curl -X PUT -s "$MOCK_URL/expectation" -d '{
      "httpRequest": {
        "method": "GET",
        "path": "/health"
      },
      "httpResponse": {
        "statusCode": 200,
        "body": "OK"
      }
    }' > /dev/null

    # Message delivery endpoint for EXT SMTS
    curl -X PUT -s "$MOCK_URL/expectation" -d '{
      "httpRequest": {
        "method": "POST",
        "path": "/test.monterra.event",
        "headers": {
          "X-API-Key": ["test-api-key"]
        }
      },
      "httpResponse": {
        "statusCode": 200,
        "body": "{\"status\": \"delivered\"}"
      }
    }' > /dev/null

    # Configure API Mock expectations for INT SMTS (port 1081)
    echo "Configuring API Mock expectations for INT SMTS..."

    # DLP validation endpoint for INT SMTS
    curl -X PUT -s "$MOCK_URL/expectation" -d '{
      "httpRequest": {
        "method": "POST",
        "path": "/validate",
        "headers": {
          "X-API-Key": ["test-api-key"]
        }
      },
      "httpResponse": {
        "statusCode": 200,
        "body": "{\"approved\": true, \"reasons\": []}"
      }
    }' > /dev/null

    echo "API MockServer expectations configured successfully!"
    
elif [ "$1" = "dlp" ]; then
    echo "Initializing DLP MockServer..."
    MOCK_URL="http://dlp-mock:1080"
    
    # Configure DLP Mock expectations
    echo "Configuring DLP Mock expectations..."

    # Health check endpoint
    curl -X PUT -s "$MOCK_URL/expectation" -d '{
      "httpRequest": {
        "method": "GET",
        "path": "/health"
      },
      "httpResponse": {
        "statusCode": 200,
        "body": "OK"
      }
    }' > /dev/null

    # DLP validation endpoint
    curl -X PUT -s "$MOCK_URL/expectation" -d '{
      "httpRequest": {
        "method": "POST",
        "path": "/validate",
        "headers": {
          "X-API-Key": ["test-api-key"]
        }
      },
      "httpResponse": {
        "statusCode": 200,
        "body": "{\"approved\": true, \"reasons\": []}"
      }
    }' > /dev/null

    echo "DLP MockServer expectations configured successfully!"
else
    echo "Usage: $0 {api|dlp}"
    exit 1
fi

echo ""
echo "Available endpoints:"
echo "- EXT SMTS API: POST http://api-mock:1080/test.monterra.event"
echo "- INT SMTS DLP: POST http://api-mock:1081/validate"
echo "- DLP Service: POST http://dlp-mock:1080/validate"
echo "- Health checks: GET /health on all services"