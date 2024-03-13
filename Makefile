# Project Setup
PROJECT_NAME := uptest
PROJECT_REPO := github.com/upbound/$(PROJECT_NAME)

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
# GOLANGCILINT_VERSION ?= 1.54.0

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
