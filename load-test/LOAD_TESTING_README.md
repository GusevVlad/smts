# SMTS Load Testing Suite

A comprehensive load testing suite for the SMTS (Secure Message Transfer System) with LDAP authentication support. This suite provides various testing scenarios to evaluate system performance, reliability, and scalability.

## Overview

The load testing suite consists of multiple components:

- **Main Load Test Script** (`load-test-suite.sh`) - Primary test execution engine
- **Configuration File** (`load-test-config.sh`) - Test parameters and environment settings
- **Performance Monitor** (`performance-monitor.sh`) - Performance analysis and reporting utilities
- **Documentation** (this file) - Usage instructions and best practices

## Prerequisites

### System Requirements
- Bash shell (version 4.0+)
- `curl` command-line tool
- `jq` for JSON parsing
- `bc` for floating-point calculations
- `openssl` for generating unique message IDs


### Environment Setup
1. Ensure the SMTS services are running:
   ```bash
   docker-compose -f docker-compose.test.yml up -d
   ```

2. Verify services are healthy:
   ```bash
   curl http://localhost:18091/health  # EXT SMTS
   curl http://localhost:18093/health  # INT SMTS
   curl http://localhost:18080/health  # Corporate API
   ```

## Quick Start

### Basic Load Test
```bash
# Make scripts executable
chmod +x load-test-suite.sh performance-monitor.sh

# Run a simple concurrent load test
./load-test-suite.sh concurrent 10
```

### Test Scenarios

#### 1. Concurrent Load Test
Tests with multiple concurrent users sending requests:
```bash
# 20 concurrent users, 100 requests each
./load-test-suite.sh concurrent 20

# With custom configuration
CONCURRENT_USERS=50 REQUESTS_PER_USER=200 ./load-test-suite.sh concurrent
```

#### 2. Duration-Based Load Test
Runs load test for a specified duration:
```bash
# 10-minute test with 10 concurrent users
./load-test-suite.sh duration 600

# 1-hour test with custom users
./load-test-suite.sh duration 3600 20
```

#### 3. Message Flow Test
Tests complete message flow between EXT and INT SMTS:
```bash
# 50 message flow iterations
./load-test-suite.sh flow 50
```

#### 4. Stress Test
Gradually increases load to identify breaking points:
```bash
# Run stress test with increasing load levels
./load-test-suite.sh stress
```

#### 5. Soak Test
Long-running test to identify memory leaks and stability issues:
```bash
# 2-hour soak test
./load-test-suite.sh soak 2

# 8-hour soak test with 5 concurrent users
./load-test-suite.sh soak 8 5
```

#### 6. Single User Test
Tests single user performance:
```bash
# Single user with 1000 requests
./load-test-suite.sh single 1000
```

## Configuration

### Environment Configuration
Edit `load-test-config.sh` to customize test parameters:

```bash
# API Endpoints
API_BASE_URL_EXT="http://localhost:18092"
API_BASE_URL_INT="http://localhost:18094"

# Authentication
API_KEY="test-api-key"
LDAP_USERNAME="testuser"
LDAP_PASSWORD="testpass"

# Load Test Parameters
CONCURRENT_USERS=10
REQUESTS_PER_USER=100
TEST_DURATION=300
THINK_TIME=1

# Performance Thresholds
MAX_AVG_RESPONSE_TIME=2.0
MIN_SUCCESS_RATE=95
```

### Environment-Specific Configurations

#### Production Environment
```bash
# Uncomment and modify in load-test-config.sh
API_BASE_URL_EXT="https://ext-smts.prod.example.com"
API_BASE_URL_INT="https://int-smts.prod.example.com"
API_KEY="prod-api-key"
LDAP_USERNAME="prod-user"
LDAP_PASSWORD="prod-password"
```

#### Staging Environment
```bash
API_BASE_URL_EXT="https://ext-smts.staging.example.com"
API_BASE_URL_INT="https://int-smts.staging.example.com"
API_KEY="staging-api-key"
LDAP_USERNAME="staging-user"
LDAP_PASSWORD="staging-password"
```

## Performance Monitoring

### Real-time Monitoring
```bash
# Monitor system resources during test
./performance-monitor.sh monitor-system 300 10 system-metrics.csv
```

### Performance Analysis
```bash
# Analyze performance metrics
./performance-monitor.sh analyze performance-metrics.csv

# Generate HTML report
./performance-monitor.sh html-report performance-metrics.csv report.html

# Create performance dashboard
./performance-monitor.sh dashboard performance-metrics.csv dashboard.html
```

### Performance Comparison
```bash
# Compare two test runs
./performance-monitor.sh compare baseline.csv current.csv comparison.txt
```

## Test Data

### Message Payloads
The test suite generates realistic message payloads with:
- Random metric values within specified ranges
- Unique message IDs and timestamps
- Varied application environments (EXT/INT)
- Different metric combinations

### Custom Payloads
To customize message payloads, modify the payload generation functions in `load-test-suite.sh`:

```bash
# Example custom payload in send_to_ext_smts_with_metrics()
local payload=$(cat <<EOF
{
  "env": "INT",
  "app_name": "custom-app",
  "app_version": "2.0.0",
  "auto_system": "CUSTOM",
  "metrics": [
    {
      "metric": "custom_metric",
      "value": "custom_value"
    }
  ],
  "report_date": "$timestamp",
  "report_type": "custom_report"
}
EOF
)
```

## Performance Metrics

### Collected Metrics
- **Response Time**: Total time from request to response
- **HTTP Status Codes**: Success/failure rates
- **Connection Time**: Time to establish connection
- **Start Transfer Time**: Time to first byte
- **System Resources**: CPU, memory, disk I/O, network I/O

### Performance Thresholds
- **Success Rate**: Minimum 95% successful requests
- **Average Response Time**: Maximum 2.0 seconds
- **95th Percentile**: Maximum 3.0 seconds
- **99th Percentile**: Maximum 5.0 seconds

## Best Practices

### Test Planning
1. **Start Small**: Begin with low concurrency and gradually increase
2. **Establish Baseline**: Run single-user tests to establish performance baseline
3. **Incremental Testing**: Increase load gradually to identify breaking points
4. **Monitor Resources**: Track system resources during tests
5. **Compare Results**: Compare against baseline performance

### Test Execution
1. **Warm-up Period**: Allow system to stabilize before measurements
2. **Consistent Environment**: Use identical environments for comparable results
3. **Network Considerations**: Account for network latency in test environment
4. **Data Cleanup**: Clear test data between runs for consistent results

### Analysis
1. **Look for Patterns**: Identify performance degradation patterns
2. **Correlate Metrics**: Correlate response times with system resources
3. **Identify Bottlenecks**: Use percentile analysis to identify bottlenecks
4. **Document Findings**: Maintain detailed test reports and findings

## Troubleshooting

### Common Issues

#### Service Unavailable
```bash
# Check service health
curl http://localhost:18091/health
curl http://localhost:18093/health
curl http://localhost:18080/health

# Restart services if needed
docker-compose -f docker-compose.test.yml restart
```

#### LDAP Authentication Failures
- Verify LDAP configuration in test configuration files
- Check LDAP mock server is running
- Validate LDAP credentials in configuration

#### Performance Degradation
- Monitor system resources during tests
- Check for network latency
- Verify backend services are not overloaded

#### Test Script Errors
- Ensure all dependencies are installed
- Verify script permissions (`chmod +x`)
- Check configuration file syntax

### Debug Mode
Enable detailed logging by modifying the configuration:
```bash
LOG_LEVEL="DEBUG"
ENABLE_DETAILED_METRICS=true
```

## Advanced Usage

### Custom Test Scenarios
Create custom test scenarios by modifying the main test functions in `load-test-suite.sh`:

```bash
# Example custom test scenario
run_custom_scenario() {
    local scenario_name=$1
    local iterations=$2
    
    print_info "Running custom scenario: $scenario_name"
    
    for ((i=1; i<=iterations; i++)); do
        # Custom test logic here
        send_to_ext_smts_with_metrics
        sleep 2
        send_to_int_smts_with_metrics
        sleep 1
    done
}
```

### Integration with CI/CD
```yaml
# Example GitHub Actions workflow
name: Load Test
on:
  push:
    branches: [ main ]
  schedule:
    - cron: '0 2 * * *'  # Daily at 2 AM

jobs:
  load-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      - name: Setup environment
        run: |
          sudo apt-get install curl jq bc openssl
          docker-compose -f docker-compose.test.yml up -d
      - name: Run load tests
        run: |
          chmod +x load-test-suite.sh
          ./load-test-suite.sh concurrent 20
      - name: Generate report
        run: |
          ./performance-monitor.sh html-report performance-metrics.csv report.html
      - name: Upload report
        uses: actions/upload-artifact@v2
        with:
          name: load-test-report
          path: report.html
```

# Calculate basic RPS
./performance-monitor.sh rps report/performance-metrics.csv

# Analyze RPS trends with 30-second intervals
./performance-monitor.sh rps-trend metrics.csv 30

# Use dedicated RPS calculator
./rps-calculator.sh all report/performance-metrics.csv 60

# Compare RPS between test runs
./rps-calculator.sh compare baseline.csv current.csv
