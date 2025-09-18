#!/bin/bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
E2E_DIR="$(dirname "$SCRIPT_DIR")"
CLUSTER_NAME="${CLUSTER_NAME:-uptest-e2e}"
PROVIDER="${PROVIDER:-all}"

echo "Running uptest e2e tests..."
echo "Provider: $PROVIDER"
echo "Cluster: $CLUSTER_NAME"

# Setup kind cluster
echo "=== Setting up kind cluster ==="
"$SCRIPT_DIR/kind-cluster.sh"

# Build uptest
echo "=== Building uptest ==="
ROOT_DIR="$E2E_DIR/../../"
cd "$ROOT_DIR"
make build
UPTEST_BIN="$ROOT_DIR/_output/bin/${PLATFORM}/uptest"
export UPTEST_BIN

# Function to run tests for a provider
run_provider_tests() {
    local provider_name=$1
    local provider_dir="$E2E_DIR/provider-$provider_name"
    
    if [ ! -d "$provider_dir" ]; then
        echo "Provider directory not found: $provider_dir"
        return 1
    fi
    
    echo "=== Running $provider_name tests ==="
    cd "$provider_dir"
    ./run-tests.sh
}

# Function to cleanup cluster
cleanup_cluster() {
    echo "=== Cleaning up cluster ==="
    if ${KIND} get clusters | grep -q "^${CLUSTER_NAME}$"; then
        ${KIND} delete cluster --name ${CLUSTER_NAME}
        echo "Cluster ${CLUSTER_NAME} deleted"
    fi
}

# Set trap to cleanup on exit
trap cleanup_cluster EXIT

# Run tests based on provider selection
case "$PROVIDER" in
    "nop")
        run_provider_tests "nop"
        ;;
    "kubernetes")
        run_provider_tests "kubernetes"
        ;;
    "all")
        run_provider_tests "nop"
        run_provider_tests "kubernetes"
        ;;
    *)
        echo "Unknown provider: $PROVIDER"
        echo "Available providers: nop, kubernetes, all"
        exit 1
        ;;
esac

echo "All e2e tests completed successfully!"