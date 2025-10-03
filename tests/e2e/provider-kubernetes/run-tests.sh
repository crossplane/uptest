#!/bin/bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$SCRIPT_DIR/../../../"
UPTEST_BIN="$ROOT_DIR/_output/bin/${PLATFORM}/uptest"

echo "Running provider-kubernetes e2e tests with uptest..."

# Build uptest
echo "Building uptest..."
cd "$ROOT_DIR"
make build

echo "Using uptest binary: $UPTEST_BIN"

# Test 1: Simple ConfigMap test
echo "=== Test 1: Simple ConfigMap test ==="
cd "$SCRIPT_DIR" && $UPTEST_BIN e2e \
    --setup-script="setup.sh" \
    --default-timeout=240s \
    "manifests/simple-configmap.yaml"

# Test 2: Multi-objects test
echo "=== Test 2: Multi-objects test ==="
cd "$SCRIPT_DIR" && $UPTEST_BIN e2e \
    --setup-script="setup.sh" \
    --default-timeout=240s \
    "manifests/multi-objects.yaml"

# Test 3: Skip delete test
echo "=== Test 3: Skip delete test ==="
cd "$SCRIPT_DIR" && $UPTEST_BIN e2e \
    --setup-script="setup.sh" \
    --skip-delete \
    --default-timeout=240s \
    "manifests/simple-configmap.yaml"

# Test 4: Skip import and update test
echo "=== Test 4: Skip import and update test ==="
cd "$SCRIPT_DIR" && $UPTEST_BIN e2e \
    --setup-script="setup.sh" \
    --skip-import \
    --skip-update \
    --default-timeout=240s \
    "manifests/simple-configmap.yaml"

echo "All provider-kubernetes e2e tests completed successfully!"