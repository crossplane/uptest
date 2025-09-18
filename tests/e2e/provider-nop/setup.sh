#!/bin/bash

set -euo pipefail

echo "Setting up provider-nop for uptest e2e tests..."

# Install Crossplane if not already installed
if ! ${KUBECTL} get crd compositeresourcedefinitions.apiextensions.crossplane.io &> /dev/null; then
    echo "Installing Crossplane..."
    ${KUBECTL} create namespace crossplane-system || true
    helm repo add crossplane-stable https://charts.crossplane.io/stable
    helm repo update
    helm install crossplane crossplane-stable/crossplane \
        --namespace crossplane-system \
        --create-namespace \
        --wait
    
    echo "Waiting for Crossplane to be ready..."
    ${KUBECTL} wait --for=condition=ready pod -l app=crossplane --namespace=crossplane-system --timeout=300s
fi

# Install provider-nop
echo "Installing provider-nop..."
cat <<EOF | ${KUBECTL} apply -f -
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-nop
spec:
  package: xpkg.upbound.io/crossplane-contrib/provider-nop:v0.5.0
EOF

# Wait for provider to be healthy
echo "Waiting for provider-nop to be ready..."
${KUBECTL} wait --for=condition=healthy provider/provider-nop --timeout=300s

# Create crossplane-system namespace for connection secrets
${KUBECTL} create namespace crossplane-system || true

echo "Provider-nop setup completed successfully!"