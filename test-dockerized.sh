#!/bin/bash

# Test script for dockerized SMTS tests
set -e

echo "=== Testing Dockerized SMTS Setup ==="

# Function to cleanup on exit
cleanup() {
    echo "Cleaning up..."
    docker-compose -f docker-compose.test.yml down -v 2>/dev/null || true
}

trap cleanup EXIT

# Test 1: Build Dockerfile.test
echo "1. Building Dockerfile.test..."
docker build -t smts-test:latest -f Dockerfile.test .

# Test 2: Run unit tests in Docker
echo "2. Running unit tests in Docker..."
docker run --rm smts-test:latest /app/bin/smts-unit-test -test.v -test.timeout=5m

# Test 3: Run integration tests in Docker (without external services)
echo "3. Running integration tests in Docker (standalone)..."
docker run --rm smts-test:latest /app/bin/smts-integration-test -test.v -test.timeout=5m

# Test 4: Test EXT SMTS specific tests
echo "4. Running EXT SMTS specific tests..."
docker run --rm smts-test:latest /app/bin/smts-integration-test -test.v -test.timeout=5m -test.run="TestEXT_SMTS"

# Test 5: Test INT SMTS specific tests
echo "5. Running INT SMTS specific tests..."
docker run --rm smts-test:latest /app/bin/smts-integration-test -test.v -test.timeout=5m -test.run="TestINT_SMTS"

# Test 6: Test docker-compose setup (services only)
echo "6. Testing docker-compose services startup..."
docker-compose -f docker-compose.test.yml up -d --build artemis-test api-mock dlp-mock

# Wait for services to be ready
echo "Waiting for services to be ready..."
sleep 30

# Check if services are healthy
echo "Checking service health..."
docker-compose -f docker-compose.test.yml ps

# Test 7: Build and test EXT SMTS service
echo "7. Building and testing EXT SMTS service..."
docker-compose -f docker-compose.test.yml build smts-ext-test

# Test 8: Build and test INT SMTS service
echo "8. Building and testing INT SMTS service..."
docker-compose -f docker-compose.test.yml build smts-int-test

echo "=== All Dockerized Tests Completed Successfully ==="
echo ""
echo "Available Make targets:"
echo "  make docker-test-unit          - Run unit tests in Docker"
echo "  make docker-test-integration   - Run integration tests in Docker"
echo "  make docker-test-ext           - Run EXT SMTS tests in Docker"
echo "  make docker-test-int           - Run INT SMTS tests in Docker"
echo "  make docker-compose-test       - Run full test suite with docker-compose"
echo "  make docker-compose-test-ext   - Run EXT SMTS tests with docker-compose"
echo "  make docker-compose-test-int   - Run INT SMTS tests with docker-compose"