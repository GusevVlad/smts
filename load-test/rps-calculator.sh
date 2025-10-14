#!/bin/bash
# rps-calculator.sh - Advanced RPS (Requests Per Second) calculator for SMTS load testing

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

# Function to calculate basic RPS
calculate_basic_rps() {
    local metrics_file="$1"
    
    if [ ! -f "$metrics_file" ]; then
        print_error "Metrics file not found: $metrics_file"
        return 1
    fi
    
    print_info "Calculating basic RPS from: $metrics_file"
    
    local total_requests=$(tail -n +2 "$metrics_file" | wc -l)
    local first_timestamp=$(tail -n +2 "$metrics_file" | head -1 | cut -d',' -f1)
    local last_timestamp=$(tail -n +2 "$metrics_file" | tail -1 | cut -d',' -f1)
    
    # Convert timestamps to epoch seconds
    local first_epoch=$(date -j -f "%Y-%m-%dT%H:%M:%S" "$first_timestamp" "+%s" 2>/dev/null || echo "0")
    local last_epoch=$(date -j -f "%Y-%m-%dT%H:%M:%S" "$last_timestamp" "+%s" 2>/dev/null || echo "0")
    
    local test_duration=0
    if [ "$first_epoch" -gt 0 ] && [ "$last_epoch" -gt 0 ]; then
        test_duration=$((last_epoch - first_epoch))
    fi
    
    local rps=0
    if [ "$test_duration" -gt 0 ]; then
        rps=$(echo "scale=2; $total_requests / $test_duration" | bc -l)
    fi
    
    echo "=== Basic RPS Calculation ==="
    echo "Total Requests: $total_requests"
    echo "Test Duration: ${test_duration}s"
    echo "Overall RPS: ${rps}"
    echo
    
    return 0
}

# Function to calculate RPS by time intervals
calculate_rps_intervals() {
    local metrics_file="$1"
    local interval="${2:-60}"  # Default 60-second intervals
    
    if [ ! -f "$metrics_file" ]; then
        print_error "Metrics file not found: $metrics_file"
        return 1
    fi
    
    print_info "Calculating RPS with ${interval}s intervals from: $metrics_file"
    
    # Convert first and last timestamps to epoch
    local first_timestamp=$(tail -n +2 "$metrics_file" | head -1 | cut -d',' -f1)
    local last_timestamp=$(tail -n +2 "$metrics_file" | tail -1 | cut -d',' -f1)
    
    local first_epoch=$(date -j -f "%Y-%m-%dT%H:%M:%S" "$first_timestamp" "+%s" 2>/dev/null || echo "0")
    local last_epoch=$(date -j -f "%Y-%m-%dT%H:%M:%S" "$last_timestamp" "+%s" 2>/dev/null || echo "0")
    
    if [ "$first_epoch" -eq 0 ] || [ "$last_epoch" -eq 0 ]; then
        print_error "Could not parse timestamps for interval analysis"
        return 1
    fi
    
    local total_duration=$((last_epoch - first_epoch))
    local intervals=$((total_duration / interval))
    
    echo "=== RPS by ${interval}s Intervals ==="
    echo "Total duration: ${total_duration}s"
    echo "Number of intervals: $intervals"
    echo
    
    local max_rps=0
    local min_rps=999999
    local total_rps=0
    local valid_intervals=0
    
    for ((i=0; i<=intervals; i++)); do
        local interval_start=$((first_epoch + (i * interval)))
        local interval_end=$((interval_start + interval))
        
        local interval_requests=$(tail -n +2 "$metrics_file" | while IFS=, read -r timestamp operation http_code response_time time_total time_connect time_starttransfer; do
            local request_epoch=$(date -j -f "%Y-%m-%dT%H:%M:%S" "$timestamp" "+%s" 2>/dev/null || echo "0")
            if [ "$request_epoch" -ge "$interval_start" ] && [ "$request_epoch" -lt "$interval_end" ]; then
                echo "1"
            fi
        done | wc -l)
        
        local interval_rps=$(echo "scale=2; $interval_requests / $interval" | bc -l)
        local interval_time=$(date -j -f "%s" "$interval_start" "+%H:%M:%S" 2>/dev/null || echo "N/A")
        
        echo "Interval $((i+1)) ($interval_time): ${interval_rps} RPS ($interval_requests requests)"
        
        # Track min/max RPS
        if (( $(echo "$interval_rps > $max_rps" | bc -l) )); then
            max_rps=$interval_rps
        fi
        if (( $(echo "$interval_rps < $min_rps" | bc -l) )); then
            min_rps=$interval_rps
        fi
        total_rps=$(echo "$total_rps + $interval_rps" | bc -l)
        ((valid_intervals++))
    done
    
    if [ "$valid_intervals" -gt 0 ]; then
        local avg_interval_rps=$(echo "scale=2; $total_rps / $valid_intervals" | bc -l)
        echo
        echo "=== Interval RPS Summary ==="
        echo "Average RPS per interval: ${avg_interval_rps}"
        echo "Maximum RPS: ${max_rps}"
        echo "Minimum RPS: ${min_rps}"
        echo "RPS Range: ${min_rps} - ${max_rps}"
    fi
}

# Function to calculate RPS by operation type
calculate_rps_by_operation() {
    local metrics_file="$1"
    
    if [ ! -f "$metrics_file" ]; then
        print_error "Metrics file not found: $metrics_file"
        return 1
    fi
    
    print_info "Calculating RPS by operation type from: $metrics_file"
    
    # Get overall test duration
    local first_timestamp=$(tail -n +2 "$metrics_file" | head -1 | cut -d',' -f1)
    local last_timestamp=$(tail -n +2 "$metrics_file" | tail -1 | cut -d',' -f1)
    
    local first_epoch=$(date -j -f "%Y-%m-%dT%H:%M:%S" "$first_timestamp" "+%s" 2>/dev/null || echo "0")
    local last_epoch=$(date -j -f "%Y-%m-%dT%H:%M:%S" "$last_timestamp" "+%s" 2>/dev/null || echo "0")
    
    local test_duration=0
    if [ "$first_epoch" -gt 0 ] && [ "$last_epoch" -gt 0 ]; then
        test_duration=$((last_epoch - first_epoch))
    fi
    
    echo "=== RPS by Operation Type ==="
    echo "Test Duration: ${test_duration}s"
    echo
    
    for operation in EXT_SEND INT_SEND EXT_RECEIVE INT_RECEIVE; do
        local op_requests=$(tail -n +2 "$metrics_file" | grep ",$operation," | wc -l)
        local op_rps=0
        if [ "$test_duration" -gt 0 ] && [ "$op_requests" -gt 0 ]; then
            op_rps=$(echo "scale=2; $op_requests / $test_duration" | bc -l)
        fi
        
        local op_success=$(tail -n +2 "$metrics_file" | grep ",$operation," | cut -d',' -f3 | grep -E "^(200|201)$" | wc -l)
        local op_success_rate=0
        if [ "$op_requests" -gt 0 ]; then
            op_success_rate=$((op_success * 100 / op_requests))
        fi
        
        echo "$operation:"
        echo "  RPS: ${op_rps}"
        echo "  Total Requests: $op_requests"
        echo "  Success Rate: ${op_success_rate}%"
        echo
    done
}

# Function to generate RPS trend analysis
generate_rps_trend() {
    local metrics_file="$1"
    local output_file="${2:-rps-trend-$(date +%Y%m%d-%H%M%S).csv}"
    
    if [ ! -f "$metrics_file" ]; then
        print_error "Metrics file not found: $metrics_file"
        return 1
    fi
    
    print_info "Generating RPS trend analysis: $output_file"
    
    # Create CSV header
    echo "timestamp,requests,rps" > "$output_file"
    
    # Group by minute and calculate RPS
    local current_minute=""
    local minute_requests=0
    
    while IFS=, read -r timestamp operation http_code response_time time_total time_connect time_starttransfer; do
        local minute=$(echo "$timestamp" | cut -d':' -f1-2)  # Get YYYY-MM-DDTHH:MM
        
        if [ "$minute" != "$current_minute" ]; then
            # Output previous minute's data
            if [ -n "$current_minute" ] && [ "$minute_requests" -gt 0 ]; then
                local minute_rps=$(echo "scale=2; $minute_requests / 60" | bc -l)
                echo "${current_minute}:00,$minute_requests,$minute_rps" >> "$output_file"
            fi
            
            current_minute="$minute"
            minute_requests=1
        else
            ((minute_requests++))
        fi
    done < <(tail -n +2 "$metrics_file")
    
    # Output the last minute
    if [ -n "$current_minute" ] && [ "$minute_requests" -gt 0 ]; then
        local minute_rps=$(echo "scale=2; $minute_requests / 60" | bc -l)
        echo "${current_minute}:00,$minute_requests,$minute_rps" >> "$output_file"
    fi
    
    print_success "RPS trend analysis generated: $output_file"
}

# Function to compare RPS between two test runs
compare_rps_runs() {
    local baseline_file="$1"
    local current_file="$2"
    
    if [ ! -f "$baseline_file" ] || [ ! -f "$current_file" ]; then
        print_error "Both baseline and current metrics files are required"
        return 1
    fi
    
    print_info "Comparing RPS between baseline and current runs"
    
    local baseline_rps=$(calculate_basic_rps "$baseline_file" | grep "Overall RPS:" | awk '{print $3}')
    local current_rps=$(calculate_basic_rps "$current_file" | grep "Overall RPS:" | awk '{print $3}')
    
    echo "=== RPS Comparison ==="
    echo "Baseline RPS: $baseline_rps"
    echo "Current RPS: $current_rps"
    echo
    
    local rps_diff=$(echo "$current_rps - $baseline_rps" | bc -l)
    local percent_change=$(echo "scale=2; ($rps_diff / $baseline_rps) * 100" | bc -l)
    
    if (( $(echo "$rps_diff > 0" | bc -l) )); then
        print_success "RPS improved by ${rps_diff} (${percent_change}%)"
    elif (( $(echo "$rps_diff < 0" | bc -l) )); then
        print_warning "RPS decreased by ${rps_diff#-} (${percent_change#-}%)"
    else
        echo "RPS unchanged"
    fi
}

# Main function
main() {
    local command="$1"
    local param1="$2"
    local param2="$3"
    
    case "$command" in
        "basic")
            calculate_basic_rps "$param1"
            ;;
        "intervals")
            calculate_rps_intervals "$param1" "$param2"
            ;;
        "operations")
            calculate_rps_by_operation "$param1"
            ;;
        "trend")
            generate_rps_trend "$param1" "$param2"
            ;;
        "compare")
            compare_rps_runs "$param1" "$param2"
            ;;
        "all")
            calculate_basic_rps "$param1"
            echo
            calculate_rps_intervals "$param1" "${param2:-60}"
            echo
            calculate_rps_by_operation "$param1"
            ;;
        *)
            echo "Usage: $0 {basic|intervals|operations|trend|compare|all} [parameters]"
            echo
            echo "Commands:"
            echo "  basic [metrics_file]              - Calculate basic RPS"
            echo "  intervals [metrics_file] [seconds] - Calculate RPS by time intervals"
            echo "  operations [metrics_file]         - Calculate RPS by operation type"
            echo "  trend [metrics_file] [output]     - Generate RPS trend CSV"
            echo "  compare [baseline] [current]      - Compare RPS between test runs"
            echo "  all [metrics_file] [interval]     - Run all RPS analyses"
            echo
            echo "Examples:"
            echo "  $0 basic performance-metrics.csv"
            echo "  $0 intervals metrics.csv 30"
            echo "  $0 operations metrics.csv"
            echo "  $0 trend metrics.csv rps-trend.csv"
            echo "  $0 compare baseline.csv current.csv"
            echo "  $0 all metrics.csv 60"
            ;;
    esac
}

# Check dependencies
if ! command -v bc &> /dev/null; then
    print_error "bc command is required but not installed"
    exit 1
fi

# Execute main function
main "$@"