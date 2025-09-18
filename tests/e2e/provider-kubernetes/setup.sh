#!/bin/bash

set -euo pipefail

echo "Setting up provider-kubernetes for uptest e2e tests..."

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

# Install provider-kubernetes
echo "Installing provider-kubernetes..."
cat <<EOF | ${KUBECTL} apply -f -
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-kubernetes
spec:
  package: xpkg.upbound.io/crossplane-contrib/provider-kubernetes:v1.0.0
EOF

# Wait for provider to be healthy
echo "Waiting for provider-kubernetes to be ready..."
${KUBECTL} wait --for=condition=healthy provider/provider-kubernetes --timeout=300s

echo "Creating ProviderConfig for provider-kubernetes..."
cat <<EOF | ${KUBECTL} apply -f -
apiVersion: kubernetes.crossplane.io/v1alpha1
kind: ProviderConfig
metadata:
  name: default
spec:
  credentials:
    source: InjectedIdentity
EOF

echo "Creating ClusterProviderConfig for provider-kubernetes..."
cat <<EOF | ${KUBECTL} apply -f -
apiVersion: kubernetes.m.crossplane.io/v1alpha1
kind: ClusterProviderConfig
metadata:
  name: cluster
spec:
  credentials:
    source: InjectedIdentity
EOF

# Create crossplane-system namespace for namespaced resources
${KUBECTL} create namespace crossplane-system || true

echo "Provider-kubernetes setup completed successfully!"