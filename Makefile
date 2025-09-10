# SPDX-FileCopyrightText: 2023 The Crossplane Authors <https://crossplane.io>
#
# SPDX-License-Identifier: Apache-2.0

# Project Setup
PROJECT_NAME := uptest
PROJECT_REPO := github.com/crossplane/$(PROJECT_NAME)

PLATFORMS ?= linux_amd64 linux_arm64 darwin_amd64 darwin_arm64

# -include will silently skip missing files, which allows us
# to load those files with a target in the Makefile. If only
# "include" was used, the make command would fail and refuse
# to run a target until the include commands succeeded.
-include build/makelib/common.mk

# ====================================================================================
# Setup Output
S3_BUCKET ?= crossplane.uptest.releases
-include build/makelib/output.mk

# ====================================================================================
# Setup Go
GO_REQUIRED_VERSION = 1.21
# GOLANGCILINT_VERSION is inherited from build submodule by default.
# Uncomment below if you need to override the version.
GOLANGCILINT_VERSION ?= 1.61.0

GO_STATIC_PACKAGES = $(GO_PROJECT)/cmd/uptest
GO_LDFLAGS += -X $(GO_PROJECT)/internal/version.Version=$(VERSION)
GO_SUBDIRS += cmd internal
GO111MODULE = on
-include build/makelib/golang.mk

# ====================================================================================
# Targets

# run `make help` to see the targets and options

# We want submodules to be set up the first time `make` is run.
# We manage the build/ folder and its Makefiles as a submodule.
# The first time `make` is run, the includes of build/*.mk files will
# all fail, and this target will be run. The next time, the default as defined
# by the includes will be run instead.
fallthrough: submodules
	@echo Initial setup complete. Running make again . . .
	@make

# Update the submodules, such as the common build scripts.
submodules:
	@git submodule sync
	@git submodule update --init --recursive

.PHONY: submodules fallthrough

-include build/makelib/k8s_tools.mk
-include build/makelib/controlplane.mk

uptest:
	@echo "Running uptest"
	@printenv

# NOTE(hasheddan): the build submodule currently overrides XDG_CACHE_HOME in
# order to force the Helm 3 to use the .work/helm directory. This causes Go on
# Linux machines to use that directory as the build cache as well. We should
# adjust this behavior in the build submodule because it is also causing Linux
# users to duplicate their build cache, but for now we just make it easier to
# identify its location in CI so that we cache between builds.
go.cachedir:
	@go env GOCACHE

go.mod.cachedir:
	@go env GOMODCACHE

# ====================================================================================
# E2E Testing

CROSSPLANE_VERSION = 1.20.0
CROSSPLANE_NAMESPACE = crossplane-system
-include build/makelib/local.xpkg.mk

# Run all e2e tests
uptest-e2e: $(KIND) $(KUBECTL) $(CHAINSAW) $(CROSSPLANE_CLI) build
	@echo "Running all e2e tests..."
	@KIND=$(KIND) KUBECTL=$(KUBECTL) CHAINSAW=$(CHAINSAW) CROSSPLANE_CLI=$(CROSSPLANE_CLI) CROSSPLANE_NAMESPACE=$(CROSSPLANE_NAMESPACE) PLATFORM=$(PLATFORM) ./tests/e2e/ci/test-runner.sh

# Run e2e tests for provider-nop only
uptest-e2e.nop: $(KIND) $(KUBECTL) $(CHAINSAW) $(CROSSPLANE_CLI) build
	@echo "Running provider-nop e2e tests..."
	@KIND=$(KIND) KUBECTL=$(KUBECTL) CHAINSAW=$(CHAINSAW) CROSSPLANE_CLI=$(CROSSPLANE_CLI) CROSSPLANE_NAMESPACE=$(CROSSPLANE_NAMESPACE) PLATFORM=$(PLATFORM) PROVIDER=nop ./tests/e2e/ci/test-runner.sh

# Run e2e tests for provider-kubernetes only
uptest-e2e.kubernetes: $(KIND) $(KUBECTL) $(CHAINSAW) $(CROSSPLANE_CLI) build
	@echo "Running provider-kubernetes e2e tests..."
	@KIND=$(KIND) KUBECTL=$(KUBECTL) CHAINSAW=$(CHAINSAW) CROSSPLANE_CLI=$(CROSSPLANE_CLI) CROSSPLANE_NAMESPACE=$(CROSSPLANE_NAMESPACE) PLATFORM=$(PLATFORM) PROVIDER=kubernetes ./tests/e2e/ci/test-runner.sh

# Setup kind cluster for e2e testing
uptest-e2e.setup:
	@echo "Setting up kind cluster for e2e testing..."
	@KIND=$(KIND) KUBECTL=$(KUBECTL) CHAINSAW=$(CHAINSAW) CROSSPLANE_CLI=$(CROSSPLANE_CLI) CROSSPLANE_NAMESPACE=$(CROSSPLANE_NAMESPACE) PLATFORM=$(PLATFORM) ./tests/e2e/ci/kind-cluster.sh

# Cleanup kind cluster after e2e testing
uptest-e2e.cleanup:
	@echo "Cleaning up kind cluster..."
	@KIND=$(KIND) KUBECTL=$(KUBECTL) CHAINSAW=$(CHAINSAW) CROSSPLANE_CLI=$(CROSSPLANE_CLI) CROSSPLANE_NAMESPACE=$(CROSSPLANE_NAMESPACE) PLATFORM=$(PLATFORM) kind delete cluster --name uptest-e2e || true

.PHONY: e2e e2e.nop e2e.kubernetes e2e.setup e2e.cleanup
