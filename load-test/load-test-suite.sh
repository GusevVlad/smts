#!/bin/bash
# load-test-suite.sh - Comprehensive load testing suite for SMTS with LDAP authentication
# This script provides various load testing scenarios for the SMTS system

# Load configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONFIG_FILE="$SCRIPT_DIR/load-test-config.sh"

if [ -f "$CONFIG_FILE" ]; then
    source "$CONFIG_FILE"
    echo "Loaded configuration from: $CONFIG_FILE"
else
    echo "Warning: Configuration file not found at $CONFIG_FILE"
    echo "Using default values..."
fi

# Default configuration
API_BASE_URL_EXT="${API_BASE_URL_EXT:-http://localhost:18092}"
API_BASE_URL_INT="${API_BASE_URL_INT:-http://localhost:18094}"
MESSAGE_API_EXT="${MESSAGE_API_EXT:-http://localhost:19081}"
MESSAGE_API_INT="${MESSAGE_API_INT:-http://localhost:19083}"
API_KEY="${API_KEY:-test-api-key}"
TOPIC_MONTERRA="${TOPIC_MONTERRA:-test.monterra.event}"
TOPIC_PACT_UPDATE="${TOPIC_PACT_UPDATE:-test.pact_update.event}"

# LDAP credentials for testing
LDAP_USERNAME="${LDAP_USERNAME:-testuser}"
LDAP_PASSWORD="${LDAP_PASSWORD:-testpass}"
LDAP_AUTH_HEADER="Basic $(printf "%s" "$LDAP_USERNAME:$LDAP_PASSWORD" | base64)"

# Load test parameters
CONCURRENT_USERS="${CONCURRENT_USERS:-10}"
REQUESTS_PER_USER="${REQUESTS_PER_USER:-100}"
TEST_DURATION="${TEST_DURATION:-300}"  # 5 minutes in seconds
RAMP_UP_TIME="${RAMP_UP_TIME:-60}"     # 1 minute ramp-up
THINK_TIME="${THINK_TIME:-1}"          # 1 second between requests

# Test result tracking
TEST_START_TIME=""
TEST_END_TIME=""
TOTAL_REQUESTS=0
SUCCESSFUL_REQUESTS=0
FAILED_REQUESTS=0
RESPONSE_TIMES=()
REPORT_DIR="${REPORT_DIR:-$SCRIPT_DIR/report}"
PERFORMANCE_METRICS_FILE="${PERFORMANCE_METRICS_FILE:-$REPORT_DIR/performance-metrics.csv}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to generate unique message ID
generate_message_id() {
    echo "$(date +%Y%m%d%H%M%S)-$(openssl rand -hex 4)"
}

# Function to send message to EXT SMTS and measure performance
send_to_ext_smts_with_metrics() {
    local message_id=$(generate_message_id)
    local timestamp=$(date -u +%Y-%m-%dT%H:%M:%SZ)
    
    local payload=$(cat <<EOF
{  
  "env": "INT",
  "app_name": "zp-eco",
  "app_version": "1.0.1",
  "auto_system": "OMNIP",
  "metrics": [
    {
      "metric": "coverage",
      "value": "$(shuf -i 70-95 -n 1).$(shuf -i 1-9 -n 1)"
    },
    {
      "metric": "uncovered conditions",
      "value": "$(shuf -i 10-100 -n 1)"
    },
    {
      "metric": "specification exist",
      "value": "$([ $((RANDOM % 2)) -eq 0 ] && echo "true" || echo "false")"
    },
    {
      "metric": "specification correct",
      "value": "$([ $((RANDOM % 2)) -eq 0 ] && echo "true" || echo "false")"
    }
  ],
  "report_date": "$timestamp",
  "report_type": "unit coverage"
}
EOF
)

    local start_time=$(date +%s.%N)
    
    RESPONSE=$(curl -s -w "\nHTTP_CODE:%{http_code}\nTIME_TOTAL:%{time_total}\nTIME_CONNECT:%{time_connect}\nTIME_STARTTRANSFER:%{time_starttransfer}" \
      -X POST "$API_BASE_URL_EXT/send/$TOPIC_MONTERRA" \
      -H "Content-Type: application/json" \
      -H "Authorization: $LDAP_AUTH_HEADER" \
      -H "X-API-Key: $API_KEY" \
      -H "X-SMTS-Message-ID: $message_id" \
      -H "X-SMTS-Timestamp: $timestamp" \
      -H "X-SMTS-Source: ext-client" \
      -H "smts-role: ext_writer" \
      -d "$payload")
    
    local end_time=$(date +%s.%N)
    local response_time=$(echo "$end_time - $start_time" | bc)
    
    # Extract metrics
    local http_code=$(echo "$RESPONSE" | grep "HTTP_CODE:" | cut -d':' -f2)
    local time_total=$(echo "$RESPONSE" | grep "TIME_TOTAL:" | cut -d':' -f2)
    local time_connect=$(echo "$RESPONSE" | grep "TIME_CONNECT:" | cut -d':' -f2)
    local time_starttransfer=$(echo "$RESPONSE" | grep "TIME_STARTTRANSFER:" | cut -d':' -f2)
    
    echo "$(date +%Y-%m-%dT%H:%M:%S),EXT_SEND,$http_code,$response_time,$time_total,$time_connect,$time_starttransfer" >> "$PERFORMANCE_METRICS_FILE"
    
    if [ "$http_code" -eq 200 ] || [ "$http_code" -eq 201 ]; then
        return 0
    else
        return 1
    fi
}

# Function to send message to INT SMTS and measure performance
send_to_int_smts_with_metrics() {
    local message_id=$(generate_message_id)
    local timestamp=$(date -u +%Y-%m-%dT%H:%M:%SZ)
    
    local payload=$(cat <<EOF
{
  "env": "EXT",
  "app_version": "1.0.1",
  "auto_system": "OMNIP",
  "metrics": [
    {
      "metric": "coverage",
      "value": "$(shuf -i 70-95 -n 1).$(shuf -i 1-9 -n 1)"
    },
    {
      "metric": "uncovered conditions",
      "value": "$(shuf -i 10-100 -n 1)"
    },
    {
      "metric": "specification exist",
      "value": "$([ $((RANDOM % 2)) -eq 0 ] && echo "true" || echo "false")"
    },
    {
      "metric": "specification correct",
      "value": "$([ $((RANDOM % 2)) -eq 0 ] && echo "true" || echo "false")"
    }
  ],
  "report_date": "$timestamp",
  "report_type": "unit coverage"
}
EOF
)

    local start_time=$(date +%s.%N)
    
    RESPONSE=$(curl -s -w "\nHTTP_CODE:%{http_code}\nTIME_TOTAL:%{time_total}\nTIME_CONNECT:%{time_connect}\nTIME_STARTTRANSFER:%{time_starttransfer}" \
      -X POST "$API_BASE_URL_INT/send/$TOPIC_MONTERRA" \
      -H "Content-Type: application/json" \
      -H "Authorization: $LDAP_AUTH_HEADER" \
      -H "X-API-Key: $API_KEY" \
      -H "X-SMTS-Message-ID: $message_id" \
      -H "X-SMTS-Timestamp: $timestamp" \
      -H "X-SMTS-Source: int-client" \
      -H "smts-role: int_writer" \
      -d "$payload")
    
    local end_time=$(date +%s.%N)
    local response_time=$(echo "$end_time - $start_time" | bc)
    
    # Extract metrics
    local http_code=$(echo "$RESPONSE" | grep "HTTP_CODE:" | cut -d':' -f2)
    local time_total=$(echo "$RESPONSE" | grep "TIME_TOTAL:" | cut -d':' -f2)
    local time_connect=$(echo "$RESPONSE" | grep "TIME_CONNECT:" | cut -d':' -f2)
    local time_starttransfer=$(echo "$RESPONSE" | grep "TIME_STARTTRANSFER:" | cut -d':' -f2)
    
    echo "$(date +%Y-%m-%dT%H:%M:%S),INT_SEND,$http_code,$response_time,$time_total,$time_connect,$time_starttransfer" >> "$PERFORMANCE_METRICS_FILE"
    
    if [ "$http_code" -eq 200 ] || [ "$http_code" -eq 201 ]; then
        return 0
    else
        return 1
    fi
}

# Function to check received messages with performance metrics
check_received_messages_with_metrics() {
    local deployment=$1
    local topic=$2
    
    if [ "$deployment" = "ext" ]; then
        endpoint="$MESSAGE_API_EXT/messages?topic=$topic&count=1"
        role="ext_reader"
        operation="EXT_RECEIVE"
    else
        endpoint="$MESSAGE_API_INT/messages?topic=$topic&count=1"
        role="int_reader"
        operation="INT_RECEIVE"
    fi
    
    local start_time=$(date +%s.%N)
    
    RESPONSE=$(curl -s -w "\nHTTP_CODE:%{http_code}\nTIME_TOTAL:%{time_total}\nTIME_CONNECT:%{time_connect}\nTIME_STARTTRANSFER:%{time_starttransfer}" \
      -X GET "$endpoint" \
      -H "Content-Type: application/json" \
      -H "Authorization: $LDAP_AUTH_HEADER" \
      -H "X-API-Key: $API_KEY" \
      -H "smts-role: $role")
    
    local end_time=$(date +%s.%N)
    local response_time=$(echo "$end_time - $start_time" | bc)
    
    # Extract metrics
    local http_code=$(echo "$RESPONSE" | grep "HTTP_CODE:" | cut -d':' -f2)
    local time_total=$(echo "$RESPONSE" | grep "TIME_TOTAL:" | cut -d':' -f2)
    local time_connect=$(echo "$RESPONSE" | grep "TIME_CONNECT:" | cut -d':' -f2)
    local time_starttransfer=$(echo "$RESPONSE" | grep "TIME_STARTTRANSFER:" | cut -d':' -f2)
    
    echo "$(date +%Y-%m-%dT%H:%M:%S),$operation,$http_code,$response_time,$time_total,$time_connect,$time_starttransfer" >> "$PERFORMANCE_METRICS_FILE"
    
    # Extract message count from response (filter out curl metrics lines first)
    local json_response=$(echo "$RESPONSE" | grep -v "HTTP_CODE\|TIME_TOTAL\|TIME_CONNECT\|TIME_STARTTRANSFER")
    local received_count=$(echo "$json_response" | jq -r '.received // 0' 2>/dev/null || echo "0")
    if [ "$received_count" -gt 0 ]; then
        return 0
    else
        return 1
    fi
}

# Function to check service health
check_service_health() {
    local service_name=$1
    local health_url=$2
    
    print_info "Checking $service_name health..."
    local status_code=$(curl -s -o /dev/null -w "%{http_code}" "$health_url")
    
    if [ "$status_code" -eq 200 ]; then
        print_success "$service_name is healthy"
        return 0
    else
        print_error "$service_name is not responding (HTTP $status_code)"
        return 1
    fi
}

# Function to initialize performance metrics file
init_performance_metrics() {
    # Ensure report directory exists
    mkdir -p "$REPORT_DIR"
    echo "timestamp,operation,http_code,response_time,time_total,time_connect,time_starttransfer" > "$PERFORMANCE_METRICS_FILE"
    print_success "Performance metrics file initialized: $PERFORMANCE_METRICS_FILE"
}

# Function to run a single user load test with message verification
run_single_user_test() {
    local user_id=$1
    local requests=$2
    local think_time=$3
    
    print_info "Starting user $user_id with $requests requests (think time: ${think_time}s)"
    
    local user_success=0
    local user_failures=0
    
    for ((i=1; i<=requests; i++)); do
        # Randomly choose between EXT and INT SMTS
        local target=$((RANDOM % 2))
        
        if [ $target -eq 0 ]; then
            if send_to_ext_smts_with_metrics; then
                print_success "Message sent to EXT SMTS"
                sleep 2  # Wait for message to flow
                
                # Check if message was received by INT SMTS
                if check_received_messages_with_metrics "int" "$TOPIC_MONTERRA"; then
                    print_success "Message successfully flowed from EXT to INT"
                    ((user_success++))
                else
                    print_error "Message not found in INT SMTS (EXT→INT flow)"
                    ((user_failures++))
                fi
            else
                print_error "Failed to send message to EXT SMTS"
                ((user_failures++))
            fi
        else
            if send_to_int_smts_with_metrics; then
                print_success "Message sent to INT SMTS"
                sleep 2  # Wait for message to flow
                
                # Check if message was received by EXT SMTS
                if check_received_messages_with_metrics "ext" "$TOPIC_MONTERRA"; then
                    print_success "Message successfully flowed from INT to EXT"
                    ((user_success++))
                else
                    print_error "Message not found in EXT SMTS (INT→EXT flow)"
                    ((user_failures++))
                fi
            else
                print_error "Failed to send message to INT SMTS"
                ((user_failures++))
            fi
        fi
        
        # Add think time between requests
        sleep "$think_time"
    done
    
    echo "User $user_id completed: $user_success successful, $user_failures failed"
    return $user_failures
}

# Function to run concurrent load test
run_concurrent_load_test() {
    local concurrent_users=$1
    local requests_per_user=$2
    local think_time=$3
    
    print_info "Starting concurrent load test with $concurrent_users users, $requests_per_user requests per user"
    
    local pids=()
    local user_results=()
    
    # Start all users
    for ((user=1; user<=concurrent_users; user++)); do
        run_single_user_test "$user" "$requests_per_user" "$think_time" &
        pids+=($!)
    done
    
    # Wait for all users to complete
    for pid in "${pids[@]}"; do
        wait "$pid"
        user_results+=($?)
    done
    
    # Calculate total results
    local total_failures=0
    for result in "${user_results[@]}"; do
        total_failures=$((total_failures + result))
    done
    
    local total_requests=$((concurrent_users * requests_per_user))
    local successful_requests=$((total_requests - total_failures))
    
    echo "Concurrent load test completed:"
    echo "  Total requests: $total_requests"
    echo "  Successful: $successful_requests"
    echo "  Failed: $total_failures"
    echo "  Success rate: $((successful_requests * 100 / total_requests))%"
    
    return $total_failures
}

# Function to run duration-based load test with message verification
run_duration_load_test() {
    local duration=$1
    local concurrent_users=$2
    local think_time=$3
    
    print_info "Starting duration-based load test for ${duration}s with $concurrent_users users"
    
    local start_time=$(date +%s)
    local end_time=$((start_time + duration))
    local pids=()
    
    # Start all users
    for ((user=1; user<=concurrent_users; user++)); do
        (
            local user_requests=0
            local user_success=0
            local user_failures=0
            
            while [ $(date +%s) -lt $end_time ]; do
                # Randomly choose between EXT and INT SMTS
                local target=$((RANDOM % 2))
                
                if [ $target -eq 0 ]; then
                    if send_to_ext_smts_with_metrics; then
                        sleep 2  # Wait for message to flow
                        
                        # Check if message was received by INT SMTS
                        if check_received_messages_with_metrics "int" "$TOPIC_MONTERRA"; then
                            ((user_success++))
                        else
                            ((user_failures++))
                        fi
                    else
                        ((user_failures++))
                    fi
                else
                    if send_to_int_smts_with_metrics; then
                        sleep 2  # Wait for message to flow
                        
                        # Check if message was received by EXT SMTS
                        if check_received_messages_with_metrics "ext" "$TOPIC_MONTERRA"; then
                            ((user_success++))
                        else
                            ((user_failures++))
                        fi
                    else
                        ((user_failures++))
                    fi
                fi
                
                ((user_requests++))
                sleep "$think_time"
            done
            
            echo "User $user completed: $user_requests requests ($user_success successful, $user_failures failed)"
        ) &
        pids+=($!)
    done
    
    # Wait for all users to complete
    for pid in "${pids[@]}"; do
        wait "$pid"
    done
    
    print_success "Duration-based load test completed"
}

# Function to run message flow test
run_message_flow_test() {
    local iterations=$1
    
    print_info "Running message flow test with $iterations iterations"
    
    local flow_success=0
    local flow_failures=0
    
    for ((i=1; i<=iterations; i++)); do
        print_info "Message flow iteration $i"
        
        # Send message to EXT SMTS
        if send_to_ext_smts_with_metrics; then
            print_success "Message sent to EXT SMTS"
            sleep 2  # Wait for message to flow
            
            # Check if message was received by INT SMTS
            if check_received_messages_with_metrics "int" "$TOPIC_MONTERRA"; then
                print_success "Message successfully flowed from EXT to INT"
                ((flow_success++))
            else
                print_error "Message not found in INT SMTS (EXT→INT flow)"
                ((flow_failures++))
            fi
        else
            print_error "Failed to send message to EXT SMTS"
            ((flow_failures++))
        fi
        
        # Send message to INT SMTS
        if send_to_int_smts_with_metrics; then
            print_success "Message sent to INT SMTS"
            sleep 2  # Wait for message to flow
            
            # Check if message was received by EXT SMTS
            if check_received_messages_with_metrics "ext" "$TOPIC_MONTERRA"; then
                print_success "Message successfully flowed from INT to EXT"
                ((flow_success++))
            else
                print_error "Message not found in EXT SMTS (INT→EXT flow)"
                ((flow_failures++))
            fi
        else
            print_error "Failed to send message to INT SMTS"
            ((flow_failures++))
        fi
        
        sleep 1  # Brief pause between iterations
    done
    
    echo "Message flow test completed:"
    echo "  Successful flows: $flow_success"
    echo "  Failed flows: $flow_failures"
    echo "  Success rate: $((flow_success * 100 / (flow_success + flow_failures)))%"
    
    return $flow_failures
}

# Function to generate performance report
generate_performance_report() {
    print_info "Generating performance report..."
    
    if [ ! -f "$PERFORMANCE_METRICS_FILE" ]; then
        print_error "Performance metrics file not found: $PERFORMANCE_METRICS_FILE"
        return 1
    fi
    
    local report_file="$REPORT_DIR/performance-report-$(date +%Y%m%d-%H%M%S).txt"
    
    {
        echo "=== SMTS Load Test Performance Report ==="
        echo "Generated: $(date)"
        echo "Test Duration: $TEST_DURATION seconds"
        echo "Concurrent Users: $CONCURRENT_USERS"
        echo "Requests per User: $REQUESTS_PER_USER"
        echo
        echo "--- Performance Summary ---"
        
        # Calculate basic statistics
        local total_requests=$(tail -n +2 "$PERFORMANCE_METRICS_FILE" | wc -l)
        local successful_requests=$(tail -n +2 "$PERFORMANCE_METRICS_FILE" | cut -d',' -f3 | grep -E "^(200|201)$" | wc -l)
        local failed_requests=$((total_requests - successful_requests))
        local success_rate=$((successful_requests * 100 / total_requests))
        
        echo "Total Requests: $total_requests"
        echo "Successful: $successful_requests"
        echo "Failed: $failed_requests"
        echo "Success Rate: ${success_rate}%"
        echo
        
        # Calculate response time statistics
        echo "--- Response Time Statistics (seconds) ---"
        local response_times=$(tail -n +2 "$PERFORMANCE_METRICS_FILE" | cut -d',' -f4)
        local min_time=$(echo "$response_times" | sort -n | head -1)
        local max_time=$(echo "$response_times" | sort -n | tail -1)
        local avg_time=$(echo "$response_times" | awk '{sum+=$1} END {print sum/NR}')
        local p95_time=$(echo "$response_times" | sort -n | awk 'BEGIN{i=0} {s[i]=$1; i++;} END {print s[int(NR*0.95)]}')
        local p99_time=$(echo "$response_times" | sort -n | awk 'BEGIN{i=0} {s[i]=$1; i++;} END {print s[int(NR*0.99)]}')
        
        echo "Minimum: $min_time"
        echo "Maximum: $max_time"
        echo "Average: $avg_time"
        echo "95th Percentile: $p95_time"
        echo "99th Percentile: $p99_time"
        echo
        
        # Operation-specific statistics
        echo "--- Operation Statistics ---"
        for operation in EXT_SEND INT_SEND EXT_RECEIVE INT_RECEIVE; do
            local op_count=$(tail -n +2 "$PERFORMANCE_METRICS_FILE" | grep ",$operation," | wc -l)
            local op_success=$(tail -n +2 "$PERFORMANCE_METRICS_FILE" | grep ",$operation," | cut -d',' -f3 | grep -E "^(200|201)$" | wc -l)
            if [ "$op_count" -gt 0 ]; then
                local op_success_rate=$((op_success * 100 / op_count))
                echo "$operation: $op_count requests, ${op_success_rate}% success rate"
            fi
        done
        
    } > "$report_file"
    
    print_success "Performance report generated: $report_file"
}

# Function to clean up old reports
cleanup_reports() {
    print_info "Cleaning up old reports..."
    
    if [ -d "$REPORT_DIR" ]; then
        local files_removed=$(find "$REPORT_DIR" -name "*.csv" -o -name "*.txt" | wc -l)
        find "$REPORT_DIR" -name "*.csv" -delete
        find "$REPORT_DIR" -name "*.txt" -delete
        print_success "Cleaned up $files_removed old report files"
    else
        print_warning "Report directory not found: $REPORT_DIR"
    fi
}

# Function to run stress test
run_stress_test() {
    local stress_levels=("10" "50" "100" "200" "500")
    local stress_duration=60  # 1 minute per stress level
    
    print_info "Starting stress test with increasing load levels"
    
    for level in "${stress_levels[@]}"; do
        print_info "Stress level: $level concurrent users for ${stress_duration}s"
        
        local start_time=$(date +%s)
        local end_time=$((start_time + stress_duration))
        local pids=()
        
        # Start stress users
        for ((user=1; user<=level; user++)); do
            (
                while [ $(date +%s) -lt $end_time ]; do
                    # Alternate between EXT and INT SMTS
                    if [ $((user % 2)) -eq 0 ]; then
                        send_to_ext_smts_with_metrics > /dev/null 2>&1
                    else
                        send_to_int_smts_with_metrics > /dev/null 2>&1
                    fi
                    sleep 0.1  # Very short think time for stress test
                done
            ) &
            pids+=($!)
        done
        
        # Wait for stress level to complete
        for pid in "${pids[@]}"; do
            wait "$pid"
        done
        
        print_success "Stress level $level completed"
        sleep 5  # Brief cooldown between levels
    done
    
    print_success "Stress test completed"
}

# Function to run soak test
run_soak_test() {
    local duration_hours=${1:-1}  # Default 1 hour
    local concurrent_users=${2:-5}  # Default 5 users
    local think_time=${3:-10}  # Default 10 seconds between requests
    
    local duration_seconds=$((duration_hours * 3600))
    
    print_info "Starting soak test for ${duration_hours} hours with $concurrent_users users"
    print_info "Think time: ${think_time}s between requests"
    
    run_duration_load_test "$duration_seconds" "$concurrent_users" "$think_time"
    
    print_success "Soak test completed"
}

# Main function to handle different test scenarios
main() {
    local test_type=$1
    local test_param=$2
    
    # Handle cleanup option
    if [ "$test_type" = "cleanup" ]; then
        cleanup_reports
        exit 0
    fi

    # Initialize performance metrics
    init_performance_metrics
    
    # Check if services are healthy
    print_info "Checking service health..."
    check_service_health "EXT SMTS" "http://localhost:18091/health" || exit 1
    check_service_health "INT SMTS" "http://localhost:18093/health" || exit 1
    check_service_health "Corporate API" "http://localhost:18080/health" || exit 1

    TEST_START_TIME=$(date +%s)
    
    case "$test_type" in
        "concurrent")
            run_concurrent_load_test "${test_param:-$CONCURRENT_USERS}" "$REQUESTS_PER_USER" "$THINK_TIME"
            ;;
        "duration")
            run_duration_load_test "${test_param:-$TEST_DURATION}" "$CONCURRENT_USERS" "$THINK_TIME"
            ;;
        "flow")
            run_message_flow_test "${test_param:-10}"
            ;;
        "stress")
            run_stress_test
            ;;
        "soak")
            run_soak_test "${test_param:-1}" "$CONCURRENT_USERS" "$THINK_TIME"
            ;;
        "single")
            run_single_user_test "1" "${test_param:-$REQUESTS_PER_USER}" "$THINK_TIME"
            ;;
        *)
            echo "Usage: $0 {concurrent|duration|flow|stress|soak|single|cleanup} [parameter]"
            echo
            echo "Test types:"
            echo "  concurrent [users]    - Run concurrent load test with specified users"
            echo "  duration [seconds]    - Run duration-based load test"
            echo "  flow [iterations]     - Run message flow test"
            echo "  stress                - Run stress test with increasing load"
            echo "  soak [hours]          - Run soak test for specified hours"
            echo "  single [requests]     - Run single user test"
            echo "  cleanup               - Remove all old report files"
            echo
            echo "Examples:"
            echo "  $0 concurrent 20      - Run concurrent test with 20 users"
            echo "  $0 duration 600       - Run 10-minute duration test"
            echo "  $0 flow 50            - Run 50 message flow iterations"
            echo "  $0 stress             - Run stress test"
            echo "  $0 soak 2             - Run 2-hour soak test"
            echo "  $0 single 100         - Run single user with 100 requests"
            echo "  $0 cleanup            - Clean up old report files"
            exit 1
            ;;
    esac
    
    TEST_END_TIME=$(date +%s)
    local total_duration=$((TEST_END_TIME - TEST_START_TIME))
    
    print_info "Test completed in ${total_duration} seconds"
    
    # Generate performance report
    generate_performance_report
    
    print_success "Load test suite execution completed"
}

# Check if bc is available for floating point calculations
if ! command -v bc &> /dev/null; then
    print_error "bc command is required but not installed. Please install it:"
    echo "  Ubuntu/Debian: sudo apt-get install bc"
    echo "  CentOS/RHEL: sudo yum install bc"
    echo "  macOS: brew install bc"
    exit 1
fi

# Check if jq is available for JSON parsing
if ! command -v jq &> /dev/null; then
    print_error "jq command is required but not installed. Please install it:"
    echo "  Ubuntu/Debian: sudo apt-get install jq"
    echo "  CentOS/RHEL: sudo yum install jq"
    echo "  macOS: brew install jq"
    exit 1
fi

# Execute main function with provided arguments
main "$@"