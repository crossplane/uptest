# Uptest Execution Modes

Uptest supports two execution modes for running end-to-end tests: **CLI Fork Mode** and **Library Mode**. This document explains both modes, their differences, and when to use each one.

## Overview

Starting with the introduction of dual execution modes, uptest provides flexibility in how it interacts with Chainsaw and Crossplane CLI tools:

- **CLI Fork Mode** (default): Uses external CLI binaries via shell commands
- **Library Mode** (opt-in): Uses Go libraries directly for better integration

## CLI Fork Mode (Default)

### Description
CLI Fork Mode is the traditional execution method where uptest spawns external processes to run Chainsaw and Crossplane CLI commands. This mode maintains backward compatibility with existing deployments and tooling.

### How it works
- Executes `"${CHAINSAW}" test` commands via shell
- Runs `"${CROSSPLANE_CLI}" beta trace` for resource monitoring
- Requires external binaries to be available in the environment
- Uses environment variables `CHAINSAW` and `CROSSPLANE_CLI` to locate binaries

### Dependencies
- Chainsaw CLI binary must be available in PATH or via `CHAINSAW` environment variable
- Crossplane CLI binary must be available in PATH or via `CROSSPLANE_CLI` environment variable

### Advantages
- **Backward compatibility**: Works with existing CI/CD pipelines and scripts
- **Tool flexibility**: Can use different versions of CLI tools without recompiling uptest
- **Environment control**: Respects system PATH and environment configuration
- **Isolation**: Each test run uses fresh CLI processes

### Usage
CLI Fork Mode is the default behavior. No additional flags are required:

```bash
# Default CLI fork mode
uptest e2e examples/s3/bucket.yaml

# With multiple manifests
uptest e2e examples/s3/bucket.yaml,examples/ec2/instance.yaml

# With environment variables set
export CHAINSAW=/usr/local/bin/chainsaw
export CROSSPLANE_CLI=/usr/local/bin/crossplane
uptest e2e examples/s3/bucket.yaml
```

## Library Mode

### Description
Library Mode integrates Chainsaw and Crossplane functionality directly as Go libraries, providing a more seamless and potentially performant execution experience.

### How it works
- Imports `github.com/kyverno/chainsaw/pkg/runner` for test execution
- Uses `github.com/crossplane/crossplane/cmd/crank/beta/trace` for resource monitoring
- No external binary dependencies required
- Direct Go API calls instead of shell command execution

### Dependencies
- Go modules automatically handle dependencies
- No external CLI binaries required

### Advantages
- **Better integration**: Direct API usage provides better error handling and control
- **No external dependencies**: Self-contained execution without requiring CLI binaries
- **Performance**: Potentially faster execution due to elimination of process spawning overhead
- **Consistency**: Ensures consistent versions of Chainsaw and Crossplane libraries

### Usage
Enable Library Mode using the `--use-library-mode` flag:

```bash
# Library mode execution
uptest e2e --use-library-mode examples/s3/bucket.yaml

# Library mode with additional flags
uptest e2e --use-library-mode --skip-delete examples/s3/bucket.yaml

# Library mode with timeout
uptest e2e --use-library-mode --default-timeout=1800s examples/s3/bucket.yaml
```