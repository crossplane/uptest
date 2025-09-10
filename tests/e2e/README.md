# Uptest End-to-End Tests

This directory contains end-to-end tests for the uptest tool itself. These tests validate uptest's functionality by running it against test providers in a real Kubernetes environment.

## Overview

The E2E tests use two test providers to validate different uptest capabilities:

- **provider-nop**: A minimal provider that simulates resource lifecycle without external dependencies
- **provider-kubernetes**: A provider that manages native Kubernetes resources

## Test Structure

```
tests/e2e/
├── provider-nop/           # Tests using provider-nop
│   ├── manifests/          # Test manifests
│   ├── setup.sh           # Provider setup script
│   └── run-tests.sh       # Test runner
├── provider-kubernetes/    # Tests using provider-kubernetes  
│   ├── manifests/          # Test manifests
│   ├── setup.sh           # Provider setup script
│   └── run-tests.sh       # Test runner
├── ci/                    # CI integration scripts
│   ├── kind-cluster.sh    # Kind cluster setup
│   └── test-runner.sh     # Main test orchestrator
└── README.md              # This file
```

## Running Tests

### Prerequisites

- Docker
- kubectl
- helm
- kind
- chainsaw
- crossplane-cli

### Quick Start

Run all E2E tests:
```bash
make uptest-e2e
```

Run tests for specific provider:
```bash
make uptest-e2e.nop           # Provider-nop only
make uptest-e2e.kubernetes    # Provider-kubernetes only
```

### Manual Testing

1. Setup kind cluster:
```bash
make uptest-e2e.setup
```

2. Run specific provider tests:
```bash
cd tests/e2e/provider-nop
./run-tests.sh
```

3. Cleanup:
```bash
make uptest-e2e.cleanup
```

### Environment Variables

- `CLUSTER_NAME`: Kind cluster name (default: `uptest-e2e`)
- `PROVIDER`: Provider to test (`nop`, `kubernetes`, or `all`)
- `PLATFORM`: Target platform for uptest binary (used in build path)
- `K8S_VERSION`: Kubernetes version to use (default: `v1.28.0`)