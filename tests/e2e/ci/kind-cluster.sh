#!/bin/bash

set -euo pipefail

CLUSTER_NAME="${CLUSTER_NAME:-uptest-e2e}"
K8S_VERSION="${K8S_VERSION:-v1.28.0}"

echo "Setting up kind cluster for uptest e2e tests..."

# Create kind cluster if it doesn't exist
if ! ${KIND} get clusters | grep -q "^${CLUSTER_NAME}$"; then
    echo "Creating kind cluster: ${CLUSTER_NAME}"
    cat <<EOF | ${KIND} create cluster --name ${CLUSTER_NAME} --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  image: kindest/node:${K8S_VERSION}
  extraPortMappings:
  - containerPort: 80
    hostPort: 8080
  - containerPort: 443
    hostPort: 8443
EOF
else
    echo "Kind cluster ${CLUSTER_NAME} already exists"
fi

# Set kubectl context
${KUBECTL} config use-context kind-${CLUSTER_NAME}

# Wait for cluster to be ready
echo "Waiting for cluster to be ready..."
${KUBECTL} wait --for=condition=Ready nodes --all --timeout=300s

echo "Kind cluster ${CLUSTER_NAME} is ready!"
echo "Cluster info:"
${KUBECTL} cluster-info