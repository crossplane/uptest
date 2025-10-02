#!/bin/bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$SCRIPT_DIR/../../../"
UPTEST_BIN="$ROOT_DIR/_output/bin/${PLATFORM}/uptest"

echo "Running provider-nop e2e tests with uptest..."

# Build uptest
echo "Building uptest..."
cd "$ROOT_DIR"
make build

echo "Using uptest binary: $UPTEST_BIN"

# Test 1: Basic resource test
echo "=== Test 1: Basic resource test ==="
cd "$SCRIPT_DIR" && $UPTEST_BIN e2e \
    --setup-script="setup.sh" \
    --default-timeout=240s \
    "manifests/basic-resource.yaml"

# Test 2: Timeout test
echo "=== Test 2: Timeout test ==="
cd "$SCRIPT_DIR" && $UPTEST_BIN e2e \
    --setup-script="setup.sh" \
    --default-timeout=60s \
    "manifests/timeout-test.yaml"

# Test 3: Custom conditions test
echo "=== Test 3: Custom conditions test ==="
cd "$SCRIPT_DIR" && $UPTEST_BIN e2e \
    --setup-script="setup.sh" \
    --default-timeout=240s \
    "manifests/conditions-test.yaml"

# Test 4: Multi-resource test
echo "=== Test 4: Multi-resource test ==="
cd "$SCRIPT_DIR" && $UPTEST_BIN e2e \
    --setup-script="setup.sh" \
    --default-timeout=240s \
    "manifests/multi-resource.yaml"

# Test 5: Skip delete test
echo "=== Test 5: Skip delete test ==="
cd "$SCRIPT_DIR" && $UPTEST_BIN e2e \
    --setup-script="setup.sh" \
    --skip-delete \
    --default-timeout=240s \
    "manifests/basic-resource.yaml"

# Test 6: Render only test
echo "=== Test 6: Render only test ==="
cd "$SCRIPT_DIR" && $UPTEST_BIN e2e \
    --setup-script="setup.sh" \
    --render-only \
    --default-timeout=240s \
    "manifests/basic-resource.yaml"

# Test 7: Skip import and update test
echo "=== Test 7: Skip import and update test ==="
cd "$SCRIPT_DIR" && $UPTEST_BIN e2e \
    --setup-script="setup.sh" \
    --skip-import \
    --skip-update \
    --default-timeout=240s \
    "manifests/basic-resource.yaml"

echo "All provider-nop e2e tests completed successfully!"