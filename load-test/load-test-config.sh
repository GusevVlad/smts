#!/bin/bash
# load-test-config.sh - Configuration file for SMTS load testing suite

# API Endpoints
API_BASE_URL_EXT="http://localhost:18092"
API_BASE_URL_INT="http://localhost:18094"
MESSAGE_API_EXT="http://localhost:19081"
MESSAGE_API_INT="http://localhost:19083"

# Authentication
API_KEY="test-api-key"
LDAP_USERNAME="testuser"
LDAP_PASSWORD="testpass"

# Topics
TOPIC_MONTERRA="test.monterra.event"
TOPIC_PACT_UPDATE="test.pact_update.event"

# Load Test Parameters
CONCURRENT_USERS=10
REQUESTS_PER_USER=100
TEST_DURATION=300  # 5 minutes in seconds
RAMP_UP_TIME=60    # 1 minute ramp-up
THINK_TIME=1       # 1 second between requests

# RPS Calculation Settings
RPS_INTERVAL=60                    # Default interval for RPS calculation (seconds)
RPS_TREND_INTERVAL=30              # Interval for trend analysis (seconds)
ENABLE_RPS_ANALYSIS=true           # Enable RPS analysis in reports
RPS_THRESHOLD_WARNING=50           # Warning threshold for RPS
RPS_THRESHOLD_CRITICAL=100         # Critical threshold for RPS

# Performance Thresholds
MAX_AVG_RESPONSE_TIME=2.0
MIN_SUCCESS_RATE=95
MAX_95TH_PERCENTILE=3.0
MAX_99TH_PERCENTILE=5.0

# Logging and Output
LOG_LEVEL="INFO"                   # DEBUG, INFO, WARNING, ERROR
ENABLE_DETAILED_METRICS=true
REPORT_DIR="./report"
PERFORMANCE_METRICS_FILE="$REPORT_DIR/performance-metrics.csv"

# Stress Test Configuration
STRESS_LEVELS=("10" "50" "100" "200" "500")  # Concurrent users per stress level
STRESS_DURATION=60                           # Duration per stress level (seconds)

# Soak Test Configuration
SOAK_DURATION_HOURS=1
SOAK_CONCURRENT_USERS=5
SOAK_THINK_TIME=10

# Environment-specific overrides
# Uncomment and modify for different environments

# Production Environment
# API_BASE_URL_EXT="https://ext-smts.prod.example.com"
# API_BASE_URL_INT="https://int-smts.prod.example.com"
# API_KEY="prod-api-key"
# LDAP_USERNAME="prod-user"
# LDAP_PASSWORD="prod-password"

# Staging Environment
# API_BASE_URL_EXT="https://ext-smts.staging.example.com"
# API_BASE_URL_INT="https://int-smts.staging.example.com"
# API_KEY="staging-api-key"
# LDAP_USERNAME="staging-user"
# LDAP_PASSWORD="staging-password"