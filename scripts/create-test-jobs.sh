#!/bin/bash

# Script to create test jobs for scheduled-db
# Usage: ./create-test-jobs.sh -u <unique_jobs> -r <recurring_jobs> [options]

set -e

# Default values
UNIQUE_JOBS=0
RECURRING_JOBS=0
BASE_URL="http://127.0.0.1:80"
START_DELAY=30
TIME_SPREAD=120
VERBOSE=false
DRY_RUN=false

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# Usage function
usage() {
    echo "Usage: $0 -u <unique_jobs> -r <recurring_jobs> [options]"
    echo ""
    echo "Options:"
    echo "  -u, --unique <number>      Number of unique jobs to create (default: 0)"
    echo "  -r, --recurring <number>   Number of recurring jobs to create (default: 0)"
    echo "  -s, --server <url>         Base URL of the server (default: http://127.0.0.1:12080)"
    echo "  -d, --start-delay <sec>    Seconds to wait before first job (default: 10)"
    echo "  -t, --time-spread <sec>    Time spread for jobs in seconds (default: 300)"
    echo "  -v, --verbose              Verbose output"
    echo "  -n, --dry-run              Show what would be created without actually creating"
    echo "  -h, --help                 Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0 -u 10 -r 5              Create 10 unique jobs and 5 recurring jobs"
    echo "  $0 -u 50 -r 20 -d 30       Create jobs starting 30 seconds from now"
    echo "  $0 -u 5 -t 60 -v           Create 5 unique jobs spread over 1 minute, verbose"
    echo "  $0 -r 10 -s http://127.0.0.1:12081  Create recurring jobs via follower node"
    exit 1
}

# Logging functions
log() {
    echo -e "${BLUE}[$(date '+%H:%M:%S')]${NC} $1"
}

success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

info() {
    if [[ "$VERBOSE" == "true" ]]; then
        echo -e "${CYAN}[INFO]${NC} $1"
    fi
}

# Function to check if server is available
check_server() {
    local url=$1

    info "Testing connection to $url..."
    local health_response=$(curl -s -w "HTTP_CODE:%{http_code}" --connect-timeout 5 "$url/health" 2>/dev/null || echo "CURL_ERROR")
    local http_code=$(echo "$health_response" | grep -o "HTTP_CODE:[0-9]*" | cut -d':' -f2 2>/dev/null || echo "000")
    local body=$(echo "$health_response" | sed 's/HTTP_CODE:[0-9]*$//')

    if [[ "$health_response" == "CURL_ERROR" ]]; then
        error "Cannot connect to $url"
        return 1
    elif [[ "$http_code" != "200" ]]; then
        error "Server at $url returned HTTP $http_code"
        error "Response: $body"
        return 1
    else
        local role=$(echo "$body" | grep -o '"role":"[^"]*"' | cut -d'"' -f4 2>/dev/null || echo "unknown")
        success "Server is available at $url (role: $role)"
        info "Health response: $body"
        return 0
    fi
}

# Function to generate random timestamp within the time spread
generate_timestamp() {
    local base_time=$1
    local spread_seconds=$2
    local random_offset=$((RANDOM % spread_seconds))
    local target_time=$((base_time + random_offset))

    # Convert to ISO format
    if command -v gdate >/dev/null 2>&1; then
        gdate -u -d "@$target_time" +%Y-%m-%dT%H:%M:%SZ
    elif [[ "$OSTYPE" == "darwin"* ]]; then
        # macOS date command
        date -u -r $target_time +%Y-%m-%dT%H:%M:%SZ
    else
        # Linux date command
        date -u -d "@$target_time" +%Y-%m-%dT%H:%M:%SZ
    fi
}

# Function to create a unique job
create_unique_job() {
    local job_number=$1
    local timestamp=$2
    local url=$3

    local job_data="{\"type\":\"unico\",\"timestamp\":\"$timestamp\"}"

    if [[ "$DRY_RUN" == "true" ]]; then
        info "Would create unique job #$job_number at $timestamp"
        return 0
    fi

    local response=$(curl -s -w "HTTP_CODE:%{http_code}" -X POST \
        -H "Content-Type: application/json" \
        -d "$job_data" \
        "$url/jobs" 2>/dev/null || echo "CURL_ERROR")

    local http_code=$(echo "$response" | grep -o "HTTP_CODE:[0-9]*" | cut -d':' -f2 2>/dev/null || echo "000")
    local body=$(echo "$response" | sed 's/HTTP_CODE:[0-9]*$//')

    if [[ "$response" == "CURL_ERROR" ]]; then
        error "Failed to connect for unique job #$job_number"
        return 1
    elif [[ "$http_code" != "200" && "$http_code" != "201" ]]; then
        error "Unique job #$job_number failed with HTTP $http_code"
        error "Response: $body"
        return 1
    elif echo "$body" | grep -q "error"; then
        error "Unique job #$job_number failed: $(echo "$body" | grep -o '"error":"[^"]*"' | cut -d'"' -f4)"
        return 1
    else
        local job_id=$(echo "$body" | grep -o '"id":"[^"]*"' | cut -d'"' -f4 2>/dev/null || echo "unknown")
        success "Created unique job #$job_number (ID: $job_id) scheduled for $timestamp"
        info "HTTP $http_code - Response: $body"
        return 0
    fi
}

# Function to create a recurring job
create_recurring_job() {
    local job_number=$1
    local timestamp=$2
    local url=$3

    # Generate random cron expressions (5 fields: minute hour day month weekday)
    local cron_expressions=(
        "* * * * *"         # Every minute
        "*/2 * * * *"       # Every 2 minutes
        "*/3 * * * *"       # Every 3 minutes
        "*/5 * * * *"       # Every 5 minutes
        "*/10 * * * *"      # Every 10 minutes
        "0 * * * *"         # Every hour
        "0 */2 * * *"       # Every 2 hours
        "0 0 * * *"         # Every day at midnight
    )
    local cron_expression=${cron_expressions[$((RANDOM % ${#cron_expressions[@]}))]}

    local job_data="{\"type\":\"recurrente\",\"timestamp\":\"$timestamp\",\"cron_expression\":\"$cron_expression\"}"

    if [[ "$DRY_RUN" == "true" ]]; then
        info "Would create recurring job #$job_number at $timestamp with cron: $cron_expression"
        return 0
    fi

    local response=$(curl -s -w "HTTP_CODE:%{http_code}" -X POST \
        -H "Content-Type: application/json" \
        -d "$job_data" \
        "$url/jobs" 2>/dev/null || echo "CURL_ERROR")

    local http_code=$(echo "$response" | grep -o "HTTP_CODE:[0-9]*" | cut -d':' -f2 2>/dev/null || echo "000")
    local body=$(echo "$response" | sed 's/HTTP_CODE:[0-9]*$//')

    if [[ "$response" == "CURL_ERROR" ]]; then
        error "Failed to connect for recurring job #$job_number"
        return 1
    elif [[ "$http_code" != "200" && "$http_code" != "201" ]]; then
        error "Recurring job #$job_number failed with HTTP $http_code"
        error "Response: $body"
        return 1
    elif echo "$body" | grep -q "error"; then
        error "Recurring job #$job_number failed: $(echo "$body" | grep -o '"error":"[^"]*"' | cut -d'"' -f4)"
        return 1
    else
        local job_id=$(echo "$body" | grep -o '"id":"[^"]*"' | cut -d'"' -f4 2>/dev/null || echo "unknown")
        success "Created recurring job #$job_number (ID: $job_id) scheduled for $timestamp, cron: $cron_expression"
        info "HTTP $http_code - Response: $body"
        return 0
    fi
}

# Function to show cluster status
show_cluster_status() {
    local base_url=$1
    local base_port=$(echo "$base_url" | grep -o ':[0-9]*' | cut -d':' -f2)

    log "Cluster Status:"

    for port_offset in 0 1 2; do
        local port=$((base_port + port_offset))
        local url="http://127.0.0.1:$port"

        if curl -s --connect-timeout 2 "$url/health" >/dev/null 2>&1; then
            local health=$(curl -s "$url/health" 2>/dev/null)
            local role=$(echo "$health" | grep -o '"role":"[^"]*"' | cut -d'"' -f4 2>/dev/null || echo "unknown")
            echo "  Port $port: ✅ $role"
        else
            echo "  Port $port: ❌ Not responding"
        fi
    done
    echo
}

# Function to show final statistics
show_statistics() {
    local unique_created=$1
    local recurring_created=$2
    local unique_failed=$3
    local recurring_failed=$4

    log "=== Job Creation Summary ==="
    echo "  Unique jobs:"
    echo "    ✅ Created: $unique_created"
    if [[ $unique_failed -gt 0 ]]; then
        echo "    ❌ Failed:  $unique_failed"
    fi
    echo "  Recurring jobs:"
    echo "    ✅ Created: $recurring_created"
    if [[ $recurring_failed -gt 0 ]]; then
        echo "    ❌ Failed:  $recurring_failed"
    fi
    echo "  Total successful: $((unique_created + recurring_created))"

    if [[ $((unique_failed + recurring_failed)) -gt 0 ]]; then
        echo "  Total failed: $((unique_failed + recurring_failed))"
    fi
    echo
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -u|--unique)
            UNIQUE_JOBS="$2"
            shift 2
            ;;
        -r|--recurring)
            RECURRING_JOBS="$2"
            shift 2
            ;;
        -s|--server)
            BASE_URL="$2"
            shift 2
            ;;
        -d|--start-delay)
            START_DELAY="$2"
            shift 2
            ;;
        -t|--time-spread)
            TIME_SPREAD="$2"
            shift 2
            ;;
        -v|--verbose)
            VERBOSE=true
            shift
            ;;
        -n|--dry-run)
            DRY_RUN=true
            shift
            ;;
        -h|--help)
            usage
            ;;
        *)
            error "Unknown option: $1"
            usage
            ;;
    esac
done

# Validate arguments
if [[ ! "$UNIQUE_JOBS" =~ ^[0-9]+$ ]] || [[ ! "$RECURRING_JOBS" =~ ^[0-9]+$ ]]; then
    error "Job counts must be positive integers"
    usage
fi

if [[ $UNIQUE_JOBS -eq 0 ]] && [[ $RECURRING_JOBS -eq 0 ]]; then
    error "At least one of --unique or --recurring must be greater than 0"
    usage
fi

# Main execution
main() {
    log "=== Scheduled-DB Job Creator ==="
    echo "Configuration:"
    echo "  Unique jobs: $UNIQUE_JOBS"
    echo "  Recurring jobs: $RECURRING_JOBS"
    echo "  Server: $BASE_URL"
    echo "  Start delay: ${START_DELAY}s"
    echo "  Time spread: ${TIME_SPREAD}s"
    echo "  Dry run: $DRY_RUN"
    echo

    # Check server availability
    echo "Testing server connection..."
    if ! check_server "$BASE_URL"; then
        error "Cannot connect to server. Is the cluster running?"
        echo
        echo "Debug information:"
        echo "  URL: $BASE_URL"
        echo "  Curl test: curl -v $BASE_URL/health"
        echo
        echo "Try starting the cluster first:"
        echo "  ./start-traditional-cluster-fixed.sh  # Local cluster"
        echo "  make dev-up                           # Docker compose"
        echo "  make k8s-deploy                       # Kubernetes"
        echo
        echo "Or check if cluster is on different port:"
        echo "  curl http://127.0.0.1:8080/health    # Docker/direct binary"
        echo "  curl http://127.0.0.1:12080/health   # Traditional script"
        exit 1
    fi

    # Show cluster status
    show_cluster_status "$BASE_URL"

    if [[ "$DRY_RUN" == "true" ]]; then
        warn "DRY RUN MODE - No jobs will actually be created"
        echo
    fi

    # Calculate base time for jobs
    local base_time=$(($(date +%s) + START_DELAY))

    if command -v gdate >/dev/null 2>&1; then
        log "Jobs will be scheduled starting $(gdate -d "@$base_time" '+%Y-%m-%d %H:%M:%S UTC') with $TIME_SPREAD second spread"
    elif [[ "$OSTYPE" == "darwin"* ]]; then
        # macOS date command
        log "Jobs will be scheduled starting $(date -r $base_time '+%Y-%m-%d %H:%M:%S UTC') with $TIME_SPREAD second spread"
    else
        # Linux date command
        log "Jobs will be scheduled starting $(date -d "@$base_time" '+%Y-%m-%d %H:%M:%S UTC') with $TIME_SPREAD second spread"
    fi
    echo

    # Counters
    local unique_created=0
    local recurring_created=0
    local unique_failed=0
    local recurring_failed=0

    # Create unique jobs
    if [[ $UNIQUE_JOBS -gt 0 ]]; then
        log "Creating $UNIQUE_JOBS unique jobs..."

        for ((i=1; i<=UNIQUE_JOBS; i++)); do
            local timestamp=$(generate_timestamp $base_time $TIME_SPREAD)

            if create_unique_job $i "$timestamp" "$BASE_URL"; then
                ((unique_created++))
            else
                ((unique_failed++))
            fi

            # Small delay to avoid overwhelming the server
            sleep 0.1
        done
        echo
    fi

    # Create recurring jobs
    if [[ $RECURRING_JOBS -gt 0 ]]; then
        log "Creating $RECURRING_JOBS recurring jobs..."

        for ((i=1; i<=RECURRING_JOBS; i++)); do
            local timestamp=$(generate_timestamp $base_time $TIME_SPREAD)

            if create_recurring_job $i "$timestamp" "$BASE_URL"; then
                ((recurring_created++))
            else
                ((recurring_failed++))
            fi

            # Small delay to avoid overwhelming the server
            sleep 0.1
        done
        echo
    fi

    # Show final statistics
    show_statistics $unique_created $recurring_created $unique_failed $recurring_failed

    if [[ "$DRY_RUN" == "false" ]]; then
        log "Job creation complete!"
        echo "Monitor job execution with:"
        echo "  tail -f logs_node-*.log | grep -E '(Executing job|Job executed)'"
        echo
        echo "Check cluster status:"
        echo "  curl $BASE_URL/debug/cluster | jq"
    else
        log "Dry run complete! Use without -n/--dry-run to actually create jobs."
    fi
}

# Run main function
main "$@"
