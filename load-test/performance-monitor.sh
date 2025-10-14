#!/bin/bash
# performance-monitor.sh - Performance monitoring and reporting utilities for SMTS load testing

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

# Function to analyze performance metrics
analyze_performance_metrics() {
    local metrics_file="${1:-performance-metrics.csv}"
    
    if [ ! -f "$metrics_file" ]; then
        print_error "Metrics file not found: $metrics_file"
        return 1
    fi
    
    print_info "Analyzing performance metrics from: $metrics_file"
    
    # Basic statistics
    local total_requests=$(tail -n +2 "$metrics_file" | wc -l)
    local successful_requests=$(tail -n +2 "$metrics_file" | cut -d',' -f3 | grep -E "^(200|201)$" | wc -l)
    local failed_requests=$((total_requests - successful_requests))
    local success_rate=0
    if [ "$total_requests" -gt 0 ]; then
        success_rate=$((successful_requests * 100 / total_requests))
    fi
    
    echo "=== Performance Summary ==="
    echo "Total Requests: $total_requests"
    echo "Successful: $successful_requests"
    echo "Failed: $failed_requests"
    echo "Success Rate: ${success_rate}%"
    echo
    
    # Response time analysis
    local response_times=$(tail -n +2 "$metrics_file" | cut -d',' -f4)
    if [ -n "$response_times" ]; then
        local min_time=$(echo "$response_times" | sort -n | head -1)
        local max_time=$(echo "$response_times" | sort -n | tail -1)
        local avg_time=$(echo "$response_times" | awk '{sum+=$1} END {print sum/NR}')
        local p95_time=$(echo "$response_times" | sort -n | awk 'BEGIN{i=0} {s[i]=$1; i++;} END {print s[int(NR*0.95)]}')
        local p99_time=$(echo "$response_times" | sort -n | awk 'BEGIN{i=0} {s[i]=$1; i++;} END {print s[int(NR*0.99)]}')
        
        echo "=== Response Time Statistics (seconds) ==="
        echo "Minimum: $min_time"
        echo "Maximum: $max_time"
        echo "Average: $avg_time"
        echo "95th Percentile: $p95_time"
        echo "99th Percentile: $p99_time"
        echo
    fi
    
    # Operation-specific analysis
    echo "=== Operation Statistics ==="
    for operation in EXT_SEND INT_SEND EXT_RECEIVE INT_RECEIVE; do
        local op_data=$(tail -n +2 "$metrics_file" | grep ",$operation,")
        if [ -n "$op_data" ]; then
            local op_count=$(echo "$op_data" | wc -l)
            local op_success=$(echo "$op_data" | cut -d',' -f3 | grep -E "^(200|201)$" | wc -l)
            local op_success_rate=0
            if [ "$op_count" -gt 0 ]; then
                op_success_rate=$((op_success * 100 / op_count))
            fi
            
            # Calculate operation-specific response times
            local op_response_times=$(echo "$op_data" | cut -d',' -f4)
            local op_avg_time="N/A"
            if [ -n "$op_response_times" ]; then
                op_avg_time=$(echo "$op_response_times" | awk '{sum+=$1} END {print sum/NR}')
            fi
            
            echo "$operation:"
            echo "  Requests: $op_count"
            echo "  Success Rate: ${op_success_rate}%"
            echo "  Avg Response Time: ${op_avg_time}s"
        fi
    done
    echo
    
    # Error analysis
    if [ "$failed_requests" -gt 0 ]; then
        echo "=== Error Analysis ==="
        tail -n +2 "$metrics_file" | cut -d',' -f3 | grep -vE "^(200|201)$" | sort | uniq -c | sort -nr | while read count code; do
            echo "HTTP $code: $count occurrences"
        done
        echo
    fi
    
    # Performance against thresholds
    echo "=== Performance Thresholds ==="
    local max_avg_response_time=2.0
    local min_success_rate=95
    
    if (( $(echo "$avg_time > $max_avg_response_time" | bc -l) )); then
        print_warning "Average response time ($avg_time) exceeds threshold ($max_avg_response_time)"
    else
        print_success "Average response time ($avg_time) within threshold ($max_avg_response_time)"
    fi
    
    if [ "$success_rate" -lt "$min_success_rate" ]; then
        print_warning "Success rate (${success_rate}%) below threshold (${min_success_rate}%)"
    else
        print_success "Success rate (${success_rate}%) meets threshold (${min_success_rate}%)"
    fi
}

# Function to generate detailed HTML report
generate_html_report() {
    local metrics_file="${1:-performance-metrics.csv}"
    local report_file="${2:-performance-report-$(date +%Y%m%d-%H%M%S).html}"
    
    if [ ! -f "$metrics_file" ]; then
        print_error "Metrics file not found: $metrics_file"
        return 1
    fi
    
    print_info "Generating HTML report: $report_file"
    
    cat > "$report_file" << EOF
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>SMTS Load Test Performance Report</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        .header { background: #f5f5f5; padding: 20px; border-radius: 5px; }
        .section { margin: 20px 0; padding: 15px; border: 1px solid #ddd; border-radius: 5px; }
        .success { color: green; }
        .warning { color: orange; }
        .error { color: red; }
        table { width: 100%; border-collapse: collapse; margin: 10px 0; }
        th, td { border: 1px solid #ddd; padding: 8px; text-align: left; }
        th { background-color: #f2f2f2; }
        .metric { font-weight: bold; }
    </style>
</head>
<body>
    <div class="header">
        <h1>SMTS Load Test Performance Report</h1>
        <p>Generated: $(date)</p>
    </div>
EOF

    # Basic statistics
    local total_requests=$(tail -n +2 "$metrics_file" | wc -l)
    local successful_requests=$(tail -n +2 "$metrics_file" | cut -d',' -f3 | grep -E "^(200|201)$" | wc -l)
    local failed_requests=$((total_requests - successful_requests))
    local success_rate=0
    if [ "$total_requests" -gt 0 ]; then
        success_rate=$((successful_requests * 100 / total_requests))
    fi
    
    cat >> "$report_file" << EOF
    <div class="section">
        <h2>Performance Summary</h2>
        <table>
            <tr><td class="metric">Total Requests</td><td>$total_requests</td></tr>
            <tr><td class="metric">Successful Requests</td><td>$successful_requests</td></tr>
            <tr><td class="metric">Failed Requests</td><td>$failed_requests</td></tr>
            <tr><td class="metric">Success Rate</td><td>${success_rate}%</td></tr>
        </table>
    </div>
EOF

    # Response time statistics
    local response_times=$(tail -n +2 "$metrics_file" | cut -d',' -f4)
    if [ -n "$response_times" ]; then
        local min_time=$(echo "$response_times" | sort -n | head -1)
        local max_time=$(echo "$response_times" | sort -n | tail -1)
        local avg_time=$(echo "$response_times" | awk '{sum+=$1} END {print sum/NR}')
        local p95_time=$(echo "$response_times" | sort -n | awk 'BEGIN{i=0} {s[i]=$1; i++;} END {print s[int(NR*0.95)]}')
        local p99_time=$(echo "$response_times" | sort -n | awk 'BEGIN{i=0} {s[i]=$1; i++;} END {print s[int(NR*0.99)]}')
        
        cat >> "$report_file" << EOF
    <div class="section">
        <h2>Response Time Statistics (seconds)</h2>
        <table>
            <tr><td class="metric">Minimum</td><td>$min_time</td></tr>
            <tr><td class="metric">Maximum</td><td>$max_time</td></tr>
            <tr><td class="metric">Average</td><td>$avg_time</td></tr>
            <tr><td class="metric">95th Percentile</td><td>$p95_time</td></tr>
            <tr><td class="metric">99th Percentile</td><td>$p99_time</td></tr>
        </table>
    </div>
EOF
    fi

    # Operation statistics
    cat >> "$report_file" << EOF
    <div class="section">
        <h2>Operation Statistics</h2>
        <table>
            <tr>
                <th>Operation</th>
                <th>Requests</th>
                <th>Success Rate</th>
                <th>Avg Response Time</th>
            </tr>
EOF

    for operation in EXT_SEND INT_SEND EXT_RECEIVE INT_RECEIVE; do
        local op_data=$(tail -n +2 "$metrics_file" | grep ",$operation,")
        if [ -n "$op_data" ]; then
            local op_count=$(echo "$op_data" | wc -l)
            local op_success=$(echo "$op_data" | cut -d',' -f3 | grep -E "^(200|201)$" | wc -l)
            local op_success_rate=0
            if [ "$op_count" -gt 0 ]; then
                op_success_rate=$((op_success * 100 / op_count))
            fi
            
            local op_response_times=$(echo "$op_data" | cut -d',' -f4)
            local op_avg_time="N/A"
            if [ -n "$op_response_times" ]; then
                op_avg_time=$(echo "$op_response_times" | awk '{sum+=$1} END {print sum/NR}')
            fi
            
            cat >> "$report_file" << EOF
            <tr>
                <td>$operation</td>
                <td>$op_count</td>
                <td>${op_success_rate}%</td>
                <td>${op_avg_time}s</td>
            </tr>
EOF
        fi
    done

    cat >> "$report_file" << EOF
        </table>
    </div>
EOF

    # Error analysis
    if [ "$failed_requests" -gt 0 ]; then
        cat >> "$report_file" << EOF
    <div class="section">
        <h2>Error Analysis</h2>
        <table>
            <tr>
                <th>HTTP Status Code</th>
                <th>Count</th>
            </tr>
EOF

        tail -n +2 "$metrics_file" | cut -d',' -f3 | grep -vE "^(200|201)$" | sort | uniq -c | sort -nr | while read count code; do
            cat >> "$report_file" << EOF
            <tr>
                <td>$code</td>
                <td>$count</td>
            </tr>
EOF
        done

        cat >> "$report_file" << EOF
        </table>
    </div>
EOF
    fi

    cat >> "$report_file" << EOF
</body>
</html>
EOF

    print_success "HTML report generated: $report_file"
}

# Function to monitor system resources during tests
monitor_system_resources() {
    local duration="${1:-300}"  # 5 minutes default
    local interval="${2:-10}"   # 10 seconds interval
    local output_file="${3:-system-metrics-$(date +%Y%m%d-%H%M%S).csv}"
    
    print_info "Starting system resource monitoring for ${duration}s (interval: ${interval}s)"
    print_info "Output file: $output_file"
    
    # Create header
    echo "timestamp,cpu_percent,memory_percent,disk_io,network_io" > "$output_file"
    
    local start_time=$(date +%s)
    local end_time=$((start_time + duration))
    
    while [ $(date +%s) -lt $end_time ]; do
        local timestamp=$(date +%Y-%m-%dT%H:%M:%S)
        local cpu_percent=$(top -l 1 | grep "CPU usage" | awk '{print $3}' | sed 's/%//')
        local memory_percent=$(top -l 1 | grep "PhysMem" | awk '{print $2}' | sed 's/M//')
        local disk_io=$(iostat -d -c 2 disk0 | tail -1 | awk '{print $2}')
        local network_io=$(netstat -ib | grep -e "en0" -e "en1" | head -1 | awk '{print $7}')
        
        echo "$timestamp,$cpu_percent,$memory_percent,$disk_io,$network_io" >> "$output_file"
        
        sleep "$interval"
    done
    
    print_success "System resource monitoring completed"
}

# Function to create performance comparison report
compare_performance_runs() {
    local baseline_file="$1"
    local current_file="$2"
    local comparison_file="${3:-performance-comparison-$(date +%Y%m%d-%H%M%S).txt}"
    
    if [ ! -f "$baseline_file" ] || [ ! -f "$current_file" ]; then
        print_error "Both baseline and current metrics files are required"
        return 1
    fi
    
    print_info "Creating performance comparison report"
    
    {
        echo "=== Performance Comparison Report ==="
        echo "Baseline: $baseline_file"
        echo "Current: $current_file"
        echo "Generated: $(date)"
        echo
        
        # Compare basic statistics
        echo "=== Basic Statistics Comparison ==="
        
        local baseline_total=$(tail -n +2 "$baseline_file" | wc -l)
        local current_total=$(tail -n +2 "$current_file" | wc -l)
        
        local baseline_success=$(tail -n +2 "$baseline_file" | cut -d',' -f3 | grep -E "^(200|201)$" | wc -l)
        local current_success=$(tail -n +2 "$current_file" | cut -d',' -f3 | grep -E "^(200|201)$" | wc -l)
        
        local baseline_rate=0
        local current_rate=0
        if [ "$baseline_total" -gt 0 ]; then
            baseline_rate=$((baseline_success * 100 / baseline_total))
        fi
        if [ "$current_total" -gt 0 ]; then
            current_rate=$((current_success * 100 / current_total))
        fi
        
        echo "Total Requests: $baseline_total (baseline) vs $current_total (current)"
        echo "Success Rate: ${baseline_rate}% (baseline) vs ${current_rate}% (current)"
        
        local rate_diff=$((current_rate - baseline_rate))
        if [ "$rate_diff" -lt 0 ]; then
            print_warning "Success rate decreased by ${rate_diff#-}%"
        elif [ "$rate_diff" -gt 0 ]; then
            print_success "Success rate improved by ${rate_diff}%"
        else
            echo "Success rate unchanged"
        fi
        echo
        
        # Compare response times
        echo "=== Response Time Comparison (seconds) ==="
        
        local baseline_times=$(tail -n +2 "$baseline_file" | cut -d',' -f4)
        local current_times=$(tail -n +2 "$current_file" | cut -d',' -f4)
        
        if [ -n "$baseline_times" ] && [ -n "$current_times" ]; then
            local baseline_avg=$(echo "$baseline_times" | awk '{sum+=$1} END {print sum/NR}')
            local current_avg=$(echo "$current_times" | awk '{sum+=$1} END {print sum/NR}')
            
            echo "Average Response Time: $baseline_avg (baseline) vs $current_avg (current)"
            
            local time_diff=$(echo "$current_avg - $baseline_avg" | bc -l)
            if (( $(echo "$time_diff > 0.1" | bc -l) )); then
                print_warning "Average response time increased by ${time_diff}s"
            elif (( $(echo "$time_diff < -0.1" | bc -l) )); then
                print_success "Average response time improved by ${time_diff#-}s"
            else
                echo "Average response time unchanged"
            fi
        fi
        
    } > "$comparison_file"
    
    print_success "Performance comparison report generated: $comparison_file"
}

# Function to create performance dashboard
create_performance_dashboard() {
    local metrics_file="${1:-performance-metrics.csv}"
    local dashboard_file="${2:-performance-dashboard-$(date +%Y%m%d-%H%M%S).html}"
    
    if [ ! -f "$metrics_file" ]; then
        print_error "Metrics file not found: $metrics_file"
        return 1
    fi
    
    print_info "Creating performance dashboard: $dashboard_file"
    
    # This would typically integrate with visualization libraries
    # For now, create a simple dashboard with key metrics
    
    cat > "$dashboard_file" << EOF
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>SMTS Performance Dashboard</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        .dashboard { display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: 20px; }
        .card { background: #f9f9f9; padding: 20px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        .metric { font-size: 24px; font-weight: bold; margin: 10px 0; }
        .label { color: #666; font-size: 14px; }
        .good { color: green; }
        .warning { color: orange; }
        .critical { color: red; }
    </style>
</head>
<body>
    <h1>SMTS Performance Dashboard</h1>
    <p>Last updated: $(date)</p>
    
    <div class="dashboard">
EOF

    # Calculate key metrics for dashboard
    local total_requests=$(tail -n +2 "$metrics_file" | wc -l)
    local successful_requests=$(tail -n +2 "$metrics_file" | cut -d',' -f3 | grep -E "^(200|201)$" | wc -l)
    local success_rate=0
    if [ "$total_requests" -gt 0 ]; then
        success_rate=$((successful_requests * 100 / total_requests))
    fi
    
    local response_times=$(tail -n +2 "$metrics_file" | cut -d',' -f4)
    local avg_response_time="N/A"
    if [ -n "$response_times" ]; then
        avg_response_time=$(echo "$response_times" | awk '{sum+=$1} END {printf "%.3f", sum/NR}')
    fi
    
    # Determine status classes
    local success_class="good"
    if [ "$success_rate" -lt 95 ]; then
        success_class="warning"
    fi
    if [ "$success_rate" -lt 80 ]; then
        success_class="critical"
    fi
    
    local response_class="good"
    if (( $(echo "$avg_response_time > 2.0" | bc -l 2>/dev/null || echo "0") )); then
        response_class="warning"
    fi
    if (( $(echo "$avg_response_time > 5.0" | bc -l 2>/dev/null || echo "0") )); then
        response_class="critical"
    fi
    
    cat >> "$dashboard_file" << EOF
        <div class="card">
            <div class="label">Total Requests</div>
            <div class="metric">$total_requests</div>
        </div>
        
        <div class="card">
            <div class="label">Success Rate</div>
            <div class="metric $success_class">${success_rate}%</div>
        </div>
        
        <div class="card">
            <div class="label">Avg Response Time</div>
            <div class="metric $response_class">${avg_response_time}s</div>
        </div>
        
        <div class="card">
            <div class="label">Operations</div>
            <div class="metric">
EOF

    for operation in EXT_SEND INT_SEND EXT_RECEIVE INT_RECEIVE; do
        local op_count=$(tail -n +2 "$metrics_file" | grep ",$operation," | wc -l)
        if [ "$op_count" -gt 0 ]; then
            cat >> "$dashboard_file" << EOF
                <div style="font-size: 14px; margin: 5px 0;">$operation: $op_count</div>
EOF
        fi
    done

    cat >> "$dashboard_file" << EOF
            </div>
        </div>
    </div>
    
    <div style="margin-top: 30px;">
        <h3>Recent Activity</h3>
        <table style="width: 100%; border-collapse: collapse;">
            <tr style="background: #f2f2f2;">
                <th style="padding: 8px; border: 1px solid #ddd;">Timestamp</th>
                <th style="padding: 8px; border: 1px solid #ddd;">Operation</th>
                <th style="padding: 8px; border: 1px solid #ddd;">Status</th>
                <th style="padding: 8px; border: 1px solid #ddd;">Response Time</th>
            </tr>
EOF

    # Show last 10 activities
    tail -n 10 "$metrics_file" | while IFS=, read -r timestamp operation http_code response_time time_total time_connect time_starttransfer; do
        local status_class="good"
        if [ "$http_code" != "200" ] && [ "$http_code" != "201" ]; then
            status_class="critical"
        fi
        cat >> "$dashboard_file" << EOF
            <tr>
                <td style="padding: 8px; border: 1px solid #ddd;">$timestamp</td>
                <td style="padding: 8px; border: 1px solid #ddd;">$operation</td>
                <td style="padding: 8px; border: 1px solid #ddd;" class="$status_class">$http_code</td>
                <td style="padding: 8px; border: 1px solid #ddd;">${response_time}s</td>
            </tr>
EOF
    done

    cat >> "$dashboard_file" << EOF
        </table>
    </div>
</body>
</html>
EOF

    print_success "Performance dashboard created: $dashboard_file"
}

# Function to calculate RPS (Requests Per Second) from metrics
calculate_rps() {
    local metrics_file="${1:-performance-metrics.csv}"
    
    if [ ! -f "$metrics_file" ]; then
        print_error "Metrics file not found: $metrics_file"
        return 1
    fi
    
    print_info "Calculating RPS from: $metrics_file"
    
    # Extract timestamps and calculate time range
    local timestamps=$(tail -n +2 "$metrics_file" | cut -d',' -f1)
    local first_timestamp=$(echo "$timestamps" | head -1)
    local last_timestamp=$(echo "$timestamps" | tail -1)
    
    # Convert timestamps to epoch seconds for calculation
    local first_epoch=$(date -j -f "%Y-%m-%dT%H:%M:%S" "$first_timestamp" "+%s" 2>/dev/null || echo "0")
    local last_epoch=$(date -j -f "%Y-%m-%dT%H:%M:%S" "$last_timestamp" "+%s" 2>/dev/null || echo "0")
    
    local total_requests=$(tail -n +2 "$metrics_file" | wc -l)
    local test_duration=0
    
    if [ "$first_epoch" -gt 0 ] && [ "$last_epoch" -gt 0 ]; then
        test_duration=$((last_epoch - first_epoch))
    fi
    
    local rps=0
    if [ "$test_duration" -gt 0 ]; then
        rps=$(echo "scale=2; $total_requests / $test_duration" | bc -l)
    fi
    
    echo "=== RPS (Requests Per Second) Analysis ==="
    echo "Total Requests: $total_requests"
    echo "Test Duration: ${test_duration}s"
    echo "Overall RPS: ${rps}"
    echo
    
    # Calculate RPS by minute intervals
    if [ "$test_duration" -gt 60 ]; then
        echo "=== RPS by Minute Intervals ==="
        
        # Group requests by minute
        local current_minute=""
        local minute_count=0
        
        while IFS=, read -r timestamp operation http_code response_time time_total time_connect time_starttransfer; do
            local minute=$(echo "$timestamp" | cut -d':' -f1-2)  # Get YYYY-MM-DDTHH:MM
            
            if [ "$minute" != "$current_minute" ]; then
                if [ -n "$current_minute" ] && [ "$minute_count" -gt 0 ]; then
                    echo "$current_minute: $minute_count requests"
                fi
                current_minute="$minute"
                minute_count=1
            else
                ((minute_count++))
            fi
        done < <(tail -n +2 "$metrics_file")
        
        # Print the last minute
        if [ -n "$current_minute" ] && [ "$minute_count" -gt 0 ]; then
            echo "$current_minute: $minute_count requests"
        fi
        echo
    fi
    
    # Calculate RPS by operation type
    echo "=== RPS by Operation Type ==="
    for operation in EXT_SEND INT_SEND EXT_RECEIVE INT_RECEIVE; do
        local op_requests=$(tail -n +2 "$metrics_file" | grep ",$operation," | wc -l)
        local op_rps=0
        if [ "$test_duration" -gt 0 ] && [ "$op_requests" -gt 0 ]; then
            op_rps=$(echo "scale=2; $op_requests / $test_duration" | bc -l)
        fi
        echo "$operation: ${op_rps} RPS ($op_requests requests)"
    done
}

# Function to generate RPS trend analysis
analyze_rps_trend() {
    local metrics_file="${1:-performance-metrics.csv}"
    local interval="${2:-60}"  # Default 60-second intervals
    
    if [ ! -f "$metrics_file" ]; then
        print_error "Metrics file not found: $metrics_file"
        return 1
    fi
    
    print_info "Analyzing RPS trend with ${interval}s intervals"
    
    # Convert first and last timestamps to epoch
    local first_timestamp=$(tail -n +2 "$metrics_file" | head -1 | cut -d',' -f1)
    local last_timestamp=$(tail -n +2 "$metrics_file" | tail -1 | cut -d',' -f1)
    
    local first_epoch=$(date -j -f "%Y-%m-%dT%H:%M:%S" "$first_timestamp" "+%s" 2>/dev/null || echo "0")
    local last_epoch=$(date -j -f "%Y-%m-%dT%H:%M:%S" "$last_timestamp" "+%s" 2>/dev/null || echo "0")
    
    if [ "$first_epoch" -eq 0 ] || [ "$last_epoch" -eq 0 ]; then
        print_error "Could not parse timestamps for trend analysis"
        return 1
    fi
    
    local total_duration=$((last_epoch - first_epoch))
    local intervals=$((total_duration / interval))
    
    echo "=== RPS Trend Analysis (${interval}s intervals) ==="
    echo "Total duration: ${total_duration}s"
    echo "Number of intervals: $intervals"
    echo
    
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
    done
}

# Main function
main() {
    local command="$1"
    local param1="$2"
    local param2="$3"
    local param3="$4"
    
    case "$command" in
        "analyze")
            analyze_performance_metrics "$param1"
            ;;
        "html-report")
            generate_html_report "$param1" "$param2"
            ;;
        "monitor-system")
            monitor_system_resources "$param1" "$param2" "$param3"
            ;;
        "compare")
            compare_performance_runs "$param1" "$param2" "$param3"
            ;;
        "dashboard")
            create_performance_dashboard "$param1" "$param2"
            ;;
        "rps")
            calculate_rps "$param1"
            ;;
        "rps-trend")
            analyze_rps_trend "$param1" "$param2"
            ;;
        *)
            echo "Usage: $0 {analyze|html-report|monitor-system|compare|dashboard|rps|rps-trend} [parameters]"
            echo
            echo "Commands:"
            echo "  analyze [metrics_file]              - Analyze performance metrics"
            echo "  html-report [metrics_file] [output] - Generate HTML performance report"
            echo "  monitor-system [duration] [interval] [output] - Monitor system resources"
            echo "  compare [baseline] [current] [output] - Compare performance runs"
            echo "  dashboard [metrics_file] [output]   - Create performance dashboard"
            echo "  rps [metrics_file]                  - Calculate RPS (Requests Per Second)"
            echo "  rps-trend [metrics_file] [interval] - Analyze RPS trend over time"
            echo
            echo "Examples:"
            echo "  $0 analyze performance-metrics.csv"
            echo "  $0 html-report metrics.csv report.html"
            echo "  $0 monitor-system 300 10 system.csv"
            echo "  $0 compare baseline.csv current.csv"
            echo "  $0 dashboard metrics.csv dashboard.html"
            echo "  $0 rps performance-metrics.csv"
            echo "  $0 rps-trend metrics.csv 30"
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