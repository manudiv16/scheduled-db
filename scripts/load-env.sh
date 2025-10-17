#!/bin/bash

# Environment loader script for scheduled-db
# Usage: source ./load-env.sh [env_file]

set -a  # Automatically export all variables

# Default environment file
ENV_FILE=${1:-.env}

# Function to load environment file
load_env_file() {
    local file=$1

    if [ -f "$file" ]; then
        echo "🌍 Loading environment from: $file"

        # Load the file, filtering out comments and empty lines
        while IFS= read -r line; do
            # Skip empty lines and comments
            if [[ -n "$line" && ! "$line" =~ ^[[:space:]]*# ]]; then
                # Handle lines with export prefix
                if [[ "$line" =~ ^export[[:space:]]+ ]]; then
                    line=${line#export }
                fi

                # Only process lines with = assignment
                if [[ "$line" =~ ^[A-Za-z_][A-Za-z0-9_]*= ]]; then
                    echo "   $line"
                    eval "$line"
                fi
            fi
        done < "$file"

        echo "✅ Environment loaded successfully"
    else
        echo "⚠️  Environment file not found: $file"
        echo "   Creating from example..."

        if [ -f ".env.example" ]; then
            cp .env.example "$file"
            echo "✅ Created $file from .env.example"
            echo "💡 Edit $file to customize your configuration"
        else
            echo "❌ .env.example not found"
            return 1
        fi
    fi
}

# Function to show current environment
show_env() {
    echo ""
    echo "📋 Current scheduled-db environment:"
    echo "===================================="

    # Core cluster settings
    echo "Cluster Configuration:"
    echo "  CLUSTER_SIZE=${CLUSTER_SIZE:-"(not set)"}"
    echo "  CLUSTER_NAME=${CLUSTER_NAME:-"(not set)"}"
    echo ""

    # Network settings
    echo "Network Configuration:"
    echo "  RAFT_HOST=${RAFT_HOST:-"(not set)"}"
    echo "  HTTP_HOST=${HTTP_HOST:-"(not set)"}"
    echo "  BASE_RAFT_PORT=${BASE_RAFT_PORT:-"(not set)"}"
    echo "  BASE_HTTP_PORT=${BASE_HTTP_PORT:-"(not set)"}"
    echo "  HTTP_PORT_OFFSET=${HTTP_PORT_OFFSET:-"(not set)"}"
    echo ""

    # Storage settings
    echo "Storage Configuration:"
    echo "  DATA_BASE_DIR=${DATA_BASE_DIR:-"(not set)"}"
    echo "  LOG_BASE_DIR=${LOG_BASE_DIR:-"(not set)"}"
    echo ""

    # Node mappings
    echo "Node Mappings:"
    for i in {1..10}; do
        var_name="CLUSTER_NODE_$i"
        var_value="${!var_name}"
        if [ -n "$var_value" ]; then
            echo "  $var_name=$var_value"
        fi
    done

    # Application settings
    echo ""
    echo "Application Configuration:"
    echo "  SLOT_GAP=${SLOT_GAP:-"(not set)"}"
    echo "  DISCOVERY_STRATEGY=${DISCOVERY_STRATEGY:-"(not set)"}"
    echo "  TEST_HOST=${TEST_HOST:-"(not set)"}"
}

# Function to validate environment
validate_env() {
    echo ""
    echo "🔍 Validating environment..."

    local errors=0

    # Check required variables
    if [ -z "$CLUSTER_SIZE" ]; then
        echo "❌ CLUSTER_SIZE is not set"
        errors=$((errors + 1))
    elif ! [[ "$CLUSTER_SIZE" =~ ^[0-9]+$ ]] || [ "$CLUSTER_SIZE" -lt 1 ] || [ "$CLUSTER_SIZE" -gt 10 ]; then
        echo "❌ CLUSTER_SIZE must be a number between 1 and 10, got: $CLUSTER_SIZE"
        errors=$((errors + 1))
    fi

    if [ -z "$BASE_RAFT_PORT" ]; then
        echo "❌ BASE_RAFT_PORT is not set"
        errors=$((errors + 1))
    elif ! [[ "$BASE_RAFT_PORT" =~ ^[0-9]+$ ]] || [ "$BASE_RAFT_PORT" -lt 1024 ] || [ "$BASE_RAFT_PORT" -gt 65535 ]; then
        echo "❌ BASE_RAFT_PORT must be a valid port number (1024-65535), got: $BASE_RAFT_PORT"
        errors=$((errors + 1))
    fi

    if [ -z "$BASE_HTTP_PORT" ]; then
        echo "❌ BASE_HTTP_PORT is not set"
        errors=$((errors + 1))
    elif ! [[ "$BASE_HTTP_PORT" =~ ^[0-9]+$ ]] || [ "$BASE_HTTP_PORT" -lt 1024 ] || [ "$BASE_HTTP_PORT" -gt 65535 ]; then
        echo "❌ BASE_HTTP_PORT must be a valid port number (1024-65535), got: $BASE_HTTP_PORT"
        errors=$((errors + 1))
    fi

    # Check port conflicts
    if [ -n "$BASE_RAFT_PORT" ] && [ -n "$BASE_HTTP_PORT" ] && [ -n "$CLUSTER_SIZE" ]; then
        local max_raft_port=$((BASE_RAFT_PORT + CLUSTER_SIZE - 1))
        local max_http_port=$((BASE_HTTP_PORT + CLUSTER_SIZE - 1))

        # Check if Raft and HTTP port ranges overlap
        if [ "$BASE_RAFT_PORT" -le "$max_http_port" ] && [ "$BASE_HTTP_PORT" -le "$max_raft_port" ]; then
            echo "❌ Raft and HTTP port ranges overlap:"
            echo "   Raft ports: $BASE_RAFT_PORT-$max_raft_port"
            echo "   HTTP ports: $BASE_HTTP_PORT-$max_http_port"
            errors=$((errors + 1))
        fi
    fi

    # Check SLOT_GAP format if set
    if [ -n "$SLOT_GAP" ] && ! [[ "$SLOT_GAP" =~ ^[0-9]+(ns|us|µs|ms|s|m|h)$ ]]; then
        echo "❌ SLOT_GAP must be a valid duration (e.g., 10s, 5m, 1h), got: $SLOT_GAP"
        errors=$((errors + 1))
    fi

    if [ $errors -eq 0 ]; then
        echo "✅ Environment validation passed"
        return 0
    else
        echo "❌ Environment validation failed with $errors errors"
        return 1
    fi
}

# Function to generate cluster info
generate_cluster_info() {
    if [ -z "$CLUSTER_SIZE" ] || [ -z "$BASE_RAFT_PORT" ] || [ -z "$BASE_HTTP_PORT" ]; then
        echo "❌ Cannot generate cluster info: required variables not set"
        return 1
    fi

    echo ""
    echo "🏗️  Generated Cluster Configuration:"
    echo "====================================="

    local raft_host=${RAFT_HOST:-"127.0.0.1"}
    local http_host=${HTTP_HOST:-""}

    for ((i=1; i<=CLUSTER_SIZE; i++)); do
        local raft_port=$((BASE_RAFT_PORT + i - 1))
        local http_port=$((BASE_HTTP_PORT + i - 1))

        echo "Node $i:"
        echo "  Raft: ${raft_host}:${raft_port}"
        echo "  HTTP: ${http_host:-"(all interfaces)"}:${http_port}"
        echo "  Data: ${DATA_BASE_DIR:-"./data"}${i}"
        echo "  Logs: ${LOG_BASE_DIR:-"./logs"}/node-${i}.log"
        echo ""
    done
}

# Function to set development presets
set_preset() {
    local preset=$1

    case "$preset" in
        "dev"|"development")
            export CLUSTER_SIZE=3
            export RAFT_HOST=127.0.0.1
            export HTTP_HOST=""
            export BASE_RAFT_PORT=12000
            export BASE_HTTP_PORT=12080
            export DATA_BASE_DIR="./data"
            export LOG_BASE_DIR="./logs"
            echo "✅ Development preset applied"
            ;;
        "docker")
            export CLUSTER_SIZE=3
            export RAFT_HOST=0.0.0.0
            export HTTP_HOST=0.0.0.0
            export BASE_RAFT_PORT=7000
            export BASE_HTTP_PORT=8000
            export DATA_BASE_DIR="./data"
            export LOG_BASE_DIR="./logs"
            echo "✅ Docker preset applied"
            ;;
        "prod"|"production")
            export CLUSTER_SIZE=5
            export RAFT_HOST=0.0.0.0
            export HTTP_HOST=0.0.0.0
            export BASE_RAFT_PORT=9000
            export BASE_HTTP_PORT=9080
            export DATA_BASE_DIR="./data"
            export LOG_BASE_DIR="./logs"
            echo "✅ Production preset applied"
            ;;
        *)
            echo "❌ Unknown preset: $preset"
            echo "Available presets: dev, docker, prod"
            return 1
            ;;
    esac
}

# Main execution
main() {
    # Handle command line arguments
    case "${1:-}" in
        "--help"|"-h")
            cat << EOF
Usage: source ./load-env.sh [options] [env_file]

Options:
    --help, -h          Show this help message
    --show, -s          Show current environment
    --validate, -v      Validate environment
    --generate, -g      Generate cluster configuration
    --preset <name>     Apply preset configuration

Presets:
    dev, development    Local development (3 nodes, localhost)
    docker              Docker setup (3 nodes, all interfaces)
    prod, production    Production-like (5 nodes, all interfaces)

Examples:
    source ./load-env.sh                    # Load .env file
    source ./load-env.sh .env.production    # Load specific file
    source ./load-env.sh --preset dev       # Apply development preset
    source ./load-env.sh --show             # Show current environment

EOF
            return 0
            ;;
        "--show"|"-s")
            show_env
            return 0
            ;;
        "--validate"|"-v")
            validate_env
            return $?
            ;;
        "--generate"|"-g")
            generate_cluster_info
            return 0
            ;;
        "--preset")
            if [ -z "$2" ]; then
                echo "❌ Preset name required"
                return 1
            fi
            set_preset "$2"
            show_env
            return 0
            ;;
    esac

    # Load environment file
    load_env_file "$ENV_FILE"

    # Auto-generate cluster node mappings if not already set
    if [ -n "$CLUSTER_SIZE" ] && [ -n "$BASE_RAFT_PORT" ] && [ -n "$BASE_HTTP_PORT" ]; then
        local raft_host=${RAFT_HOST:-"127.0.0.1"}

        for ((i=1; i<=CLUSTER_SIZE; i++)); do
            local var_name="CLUSTER_NODE_$i"
            if [ -z "${!var_name}" ]; then
                local raft_port=$((BASE_RAFT_PORT + i - 1))
                local http_port=$((BASE_HTTP_PORT + i - 1))
                local mapping="${raft_host}:${raft_port},${raft_host}:${http_port}"
                export $var_name="$mapping"
            fi
        done
    fi

    # Show loaded configuration
    show_env

    # Validate if requested
    if [ "${VALIDATE_ENV:-}" = "true" ]; then
        validate_env
    fi
}

# Disable automatic export after we're done
set +a

# Only run main if the script is being sourced with arguments
if [ "${BASH_SOURCE[0]}" != "${0}" ]; then
    # Being sourced
    main "$@"
else
    # Being executed directly
    echo "❌ This script should be sourced, not executed directly"
    echo "Usage: source ./load-env.sh"
    exit 1
fi
