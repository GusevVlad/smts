#!/bin/bash
# run-smts-examples.sh - Start test environment and run SMTS examples

set -e

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${BLUE}🚀 SMTS Examples Runner${NC}"
echo "=========================="

# Check if docker-compose is available
if ! command -v docker-compose &> /dev/null; then
    echo -e "${RED}❌ docker-compose is not installed${NC}"
    exit 1
fi

# Check if test environment is running
check_services() {
    echo -e "${YELLOW}Checking if test services are running...${NC}"
    
    # Check if containers are running
    if docker-compose -f docker-compose.test.yml ps | grep -q "Up"; then
        echo -e "${GREEN}✅ Test services are running${NC}"
        return 0
    else
        echo -e "${YELLOW}⚠️ Test services are not running${NC}"
        return 1
    fi
}

# Start test environment
start_services() {
    echo -e "${YELLOW}Starting test environment...${NC}"
    
    # Build and start services
    docker-compose -f docker-compose.test.yml up -d --build
    
    # Wait for services to be ready
    echo -e "${YELLOW}Waiting for services to be ready...${NC}"
    sleep 10
    
    # Check if services are healthy
    if check_services; then
        echo -e "${GREEN}✅ Test environment started successfully${NC}"
    else
        echo -e "${RED}❌ Failed to start test environment${NC}"
        exit 1
    fi
}

# Run examples
run_examples() {
    echo ""
    echo -e "${BLUE}Running SMTS Examples${NC}"
    echo "======================"
    
    # Make sure scripts are executable
    chmod +x *.sh
    
    # Run health checks first
    echo -e "${YELLOW}1. Testing health endpoints...${NC}"
    if curl -s -f "$HEALTH_EXT" > /dev/null; then
        echo -e "${GREEN}✅ EXT Health: OK${NC}"
    else
        echo -e "${RED}❌ EXT Health: FAILED${NC}"
    fi
    
    if curl -s -f "$HEALTH_INT" > /dev/null; then
        echo -e "${GREEN}✅ INT Health: OK${NC}"
    else
        echo -e "${RED}❌ INT Health: FAILED${NC}"
    fi
    
    # Run examples
    echo ""
    echo -e "${YELLOW}2. Running external client send example...${NC}"
    echo -e "${BLUE}--- Sending Message Content ---${NC}"
    # Capture and display the exact message being sent
    MESSAGE_ID=$(date +%Y%m%d%H%M%S)-$(openssl rand -hex 4)
    TIMESTAMP=$(date -u +%Y-%m-%dT%H:%M:%SZ)
    SENT_PAYLOAD=$(cat <<EOF
{
  "event_type": "dora_metrics_update",
  "project_id": "project-$(openssl rand -hex 4)",
  "project_name": "Monterra Platform",
  "timestamp": "$TIMESTAMP",
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
    "end": "$TIMESTAMP"
  },
  "team_size": 15,
  "environment": "production"
}
EOF
)
    echo "$SENT_PAYLOAD" | jq '.' 2>/dev/null || echo "$SENT_PAYLOAD"
    echo -e "${BLUE}--- End Sending Message ---${NC}"
    ./ext-client-send.sh
    
    echo ""
    echo -e "${YELLOW}3. Testing EXT → INT communication...${NC}"
    echo -e "${BLUE}--- Step 1: EXT client sends message ---${NC}"
    ./ext-client-send.sh
    echo -e "${BLUE}--- Step 2: INT client receives message ---${NC}"
    ./int-client-receive.sh "${TOPIC_MONTERRA:-test.monterra.event}" 1
    echo -e "${BLUE}--- End EXT → INT communication ---${NC}"
    
    echo ""
    echo -e "${YELLOW}4. Testing INT → EXT communication...${NC}"
    echo -e "${BLUE}--- Step 1: INT client sends message ---${NC}"
    ./int-client-send.sh
    echo -e "${BLUE}--- Step 2: EXT client receives message ---${NC}"
    ./ext-client-receive.sh "${TOPIC_MONTERRA:-test.monterra.event}" 1
    echo -e "${BLUE}--- End INT → EXT communication ---${NC}"
    
    echo ""
    echo -e "${YELLOW}5. Running bidirectional communication demo...${NC}"
    ./smts-bidirectional-demo.sh
    
    echo ""
    echo -e "${YELLOW}6. Running complete test suite...${NC}"
    ./smts-complete-test.sh
}

# Stop test environment
stop_services() {
    echo ""
    echo -e "${YELLOW}Stopping test environment...${NC}"
    docker-compose -f docker-compose.test.yml down
    echo -e "${GREEN}✅ Test environment stopped${NC}"
}

# Main execution
main() {
    # Load environment variables
    set -a
    source .env
    set +a
    
    # Default behavior: check if services are running, if not start them, then run examples
    if ! check_services; then
        echo -e "${YELLOW}Test environment not running. Starting services...${NC}"
        start_services
    fi
    
    run_examples
}

# Execute main function
main