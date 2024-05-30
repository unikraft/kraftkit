# SPDX-License-Identifier: BSD-3-Clause
# Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
# Licensed under the BSD-3-Clause License (the "License").
# You may not use this file except in compliance with the License.

# Directories
WORKDIR     ?= $(CURDIR)
TESTDIR     ?= $(WORKDIR)/tests
DISTDIR     ?= $(WORKDIR)/dist
INSTALLDIR  ?= /usr/local/bin/
VENDORDIR   ?= $(WORKDIR)/third_party

# Arguments
REGISTRY    ?= kraftkit.sh
ORG         ?= unikraft
REPO        ?= kraftkit
BIN         ?= kraft \
               runu
TOOLS       ?= github-action \
               go-generate-qemu-devices \
               protoc-gen-go-netconn \
               webinstall
GOMOD       ?= kraftkit.sh
IMAGE_TAG   ?= latest
GO_VERSION  ?= 1.22

# Add a special version tag for pull requests
ifneq ($(shell grep 'refs/pull' $(WORKDIR)/.git/FETCH_HEAD),)
HASH_COMMIT ?= HEAD
HASH        += pr-$(shell cat $(WORKDIR)/.git/FETCH_HEAD | awk -F/ '{print $$3}')
endif

# Calculate the project version based on git history
ifeq ($(HASH),)
HASH_COMMIT ?= HEAD
HASH        ?= $(shell git update-index -q --refresh && \
                       git describe --tags)
# Others can't be dirty by definition
ifneq ($(HASH_COMMIT),HEAD)
HASH_COMMIT ?= HEAD
endif
DIRTY       ?= $(shell git update-index -q --refresh && \
                       git diff-index --quiet HEAD -- $(WORKDIR) || \
                       echo "-dirty")
endif
VERSION     ?= $(HASH)$(DIRTY)
GIT_SHA     ?= $(shell git update-index -q --refresh && \
                       git rev-parse --short HEAD)


# Tools
DOCKER      ?= docker
DOCKER_RUN  ?= $(DOCKER) run --rm $(1) \
               -e DOCKER= \
               -e GOOS=$(GOOS) \
               -e GOARCH=$(GOARCH) \
               -w /go/src/$(GOMOD) \
               --platform linux/amd64 \
               -v $(WORKDIR):/go/src/$(GOMOD) \
               $(REGISTRY)/$(2):$(IMAGE_TAG) \
                 $(3)
GO          ?= go
MKDIR       ?= mkdir
GIT         ?= git
CURL        ?= curl
CMAKE       ?= cmake

# Go tools
GOFUMPT_VERSION    ?= v0.6.0
GOFUMPT            ?= $(GO) run mvdan.cc/gofumpt@$(GOFUMPT_VERSION)
GOCILINT_VERSION   ?= v1.58.1
GOCILINT           ?= $(GO) run github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOCILINT_VERSION)
YTT_VERSION        ?= v0.49.0
YTT                ?= $(GO) run carvel.dev/ytt/cmd/ytt@$(YTT_VERSION)
GORELEASER_VERSION ?= v1.25.1
GORELEASER         ?= $(GO) run github.com/goreleaser/goreleaser@$(GORELEASER_VERSION)
GINKGO_VERSION     ?= v2.17.3
GINKGO             ?= $(GO) run github.com/onsi/ginkgo/v2/ginkgo@$(GINKGO_VERSION)

# Misc
Q           ?= @

UNAME_OS    ?= $(shell uname -s)
UNAME_ARCH  ?= $(shell uname -m)
GOOS        ?= linux
GOARCH      ?= amd64

# Don't try to pass the path to Darwin host's make into the container
ifeq ($(UNAME_OS),Darwin)
	MAKE_COMMAND = make
endif

# If on Darwin, we want to build a runnable binary.
# Check the OS and set GOOS/GOARCH flags accordingly.
# Note that we are still running a linux/amd64 container.
# TODO: For better performance, build an image for darwin/arm64 and darwin/amd64
ifeq ($(UNAME_OS),Darwin)
	GOOS = darwin
ifeq ($(UNAME_ARCH),arm64)
	GOARCH = arm64
else ifeq ($(UNAME_ARCH),x86_64)
	GOARCH = amd64
endif
endif

# If run with DOCKER= or within a container, unset DOCKER_RUN so all commands
# are not proxied via docker container.
ifeq ($(DOCKER),)
DOCKER_RUN  :=
else ifneq ($(wildcard /.dockerenv),)
DOCKER_RUN  :=
endif
.PROXY      :=
ifneq ($(DOCKER_RUN),)
.PROXY      := docker-proxy-
$(BIN): ENVIRONMENT ?= myself-full
$(BIN):
	$(info Running target via $(DOCKER)...)
	$(Q)$(call DOCKER_RUN,,$(ENVIRONMENT),$(MAKE) GOOS=$(GOOS) GOARCH=$(GOARCH) -e $@)
	$(Q)exit 0
endif

# Targets
.PHONY: all
.DEFAULT: all
all: help

.PHONY: build
build: CHANNEL ?= staging
build: $(WORKDIR)/goreleaser-$(CHANNEL).yaml
build: ## Build all KraftKit binary artifacts.
	$(GORELEASER) build --config $(WORKDIR)/goreleaser-$(CHANNEL).yaml --clean --skip-validate

$(WORKDIR)/goreleaser-$(CHANNEL).yaml: CHANNEL ?= staging
$(WORKDIR)/goreleaser-$(CHANNEL).yaml:
	$(YTT) -f .goreleaser-$(CHANNEL).yaml > goreleaser-$(CHANNEL).yaml

ifeq ($(DEBUG),y)
$(addprefix $(.PROXY), $(BIN)): GO_GCFLAGS ?= -N -l
else
$(addprefix $(.PROXY), $(BIN)): GO_LDFLAGS ?= -s -w
endif
$(addprefix $(.PROXY), $(BIN)): GO_LDFLAGS += -X "$(GOMOD)/internal/version.version=$(VERSION)"
$(addprefix $(.PROXY), $(BIN)): GO_LDFLAGS += -X "$(GOMOD)/internal/version.commit=$(GIT_SHA)"
$(addprefix $(.PROXY), $(BIN)): GO_LDFLAGS += -X "$(GOMOD)/internal/version.buildTime=$(shell date)"
$(addprefix $(.PROXY), $(BIN)): tidy
$(addprefix $(.PROXY), $(BIN)):
	GOOS=$(GOOS) \
	GOARCH=$(GOARCH) \
	$(GO) build \
		-v \
		-tags "containers_image_storage_stub,containers_image_openpgp" \
		-buildmode=pie \
		-gcflags=all='$(GO_GCFLAGS)' \
		-ldflags='$(GO_LDFLAGS)' \
		-o $(DISTDIR)/$@ \
		$(WORKDIR)/cmd/$@

.PHONY: tools
tools: $(TOOLS)

ifeq ($(DEBUG),y)
$(addprefix $(.PROXY), $(TOOLS)): GO_GCFLAGS ?= -N -l
else
$(addprefix $(.PROXY), $(TOOLS)): GO_LDFLAGS ?= -s -w
endif
$(addprefix $(.PROXY), $(TOOLS)): GO_LDFLAGS += -X "$(GOMOD)/internal/version.version=$(VERSION)"
$(addprefix $(.PROXY), $(TOOLS)): GO_LDFLAGS += -X "$(GOMOD)/internal/version.commit=$(GIT_SHA)"
$(addprefix $(.PROXY), $(TOOLS)): GO_LDFLAGS += -X "$(GOMOD)/internal/version.buildTime=$(shell date)"
$(addprefix $(.PROXY), $(TOOLS)):
	(cd $(WORKDIR)/tools/$@ && \
		$(GO) build -v \
		-tags "containers_image_storage_stub,containers_image_openpgp" \
		-o $(DISTDIR)/$@ \
		-gcflags=all='$(GO_GCFLAGS)' \
		-ldflags='$(GO_LDFLAGS)' \
		./...)

# Proxy all "build environment" (buildenvs) targets
buildenv-%:
	$(MAKE) -C $(WORKDIR)/buildenvs $*

# Run an environment where we can build
.PHONY: devenv
devenv: DOCKER_RUN_EXTRA ?= -it --name $(REPO)-devenv
devenv: WITH_KVM         ?= n
devenv: ## Start the development environment container.
ifeq ($(WITH_KVM),y)
	$(Q)$(call DOCKER_RUN,--device /dev/kvm $(DOCKER_RUN_EXTRA),myself-full,bash)
else
	$(Q)$(call DOCKER_RUN,$(DOCKER_RUN_EXTRA),myself-full,bash)
endif

.PHONY: tidy
tidy: ## Tidy import Go modules.
	$(GO) mod tidy -compat=$(GO_VERSION)

.PHONY: fmt
fmt: ## Format all files according to linting preferences.
	$(GOFUMPT) -e -l -w $(WORKDIR)

.PHONY: cicheck
cicheck: ## Run CI checks.
	$(GOCILINT) run --build-tags "containers_image_storage_stub,containers_image_openpgp"

.PHONY: test
test: test-unit test-framework test-e2e test-cloud-e2e ## Run all tests.

.PHONY: test-unit
test-unit: GOTEST_EXCLUDE := third_party/ test/ hack/ buildenvs/ dist/ docs/ tools/
test-unit: GOTEST_PKGS := $(foreach pkg,$(filter-out $(GOTEST_EXCLUDE),$(wildcard */)),$(pkg)...)
test-unit: ## Run unit tests.
	$(GINKGO)  -v -p -randomize-all -tags "containers_image_storage_stub,containers_image_openpgp" $(GOTEST_PKGS)

.PHONY: test-e2e
test-e2e: kraft ## Run CLI end-to-end tests.
	$(GINKGO) -v -p -randomize-all test/e2e/cli/...

.PHONY: test-framework
test-framework: kraft ## Run framework tests.
	$(GINKGO) -v -p -randomize-all ./test/e2e/framework/...

.PHONY: test-cloud-e2e
test-cloud-e2e: ## Run cloud end-to-end tests.
	$(GINKGO) -v -randomize-all --flake-attempts 2 --nodes 8 ./test/e2e/cloud/...

.PHONY: clean
clean:
	$(GO) clean -modcache -cache -i -r

.PHONY: properclean
properclean: ENVIRONMENT ?= myself-full
properclean: IMAGE       ?= $(REGISTRY)/$(ENVIRONMENT):$(IMAGE_TAG)
properclean: ## Completely clean the repository's build artifacts.
	rm -rf $(DISTDIR) $(TESTDIR)
	$(DOCKER) rmi $(IMAGE)

.PHONY: docs
docs: OUTDIR ?= $(WORKDIR)/docs/
docs: ## Generate Markdown documentation.
	$(GO) run $(WORKDIR)/tools/gendocs $(OUTDIR)

.PHONY: help
help: ## Show this help menu and exit.
	@awk 'BEGIN { \
		FS = ":.*##"; \
		printf "KraftKit developer build targets.\n\n"; \
		printf "\033[1mUSAGE\033[0m\n"; \
		printf "  make [VAR=... [VAR=...]] \033[36mTARGET\033[0m\n\n"; \
		printf "\033[1mTARGETS\033[0m\n"; \
	} \
	/^[a-zA-Z0-9_-]+:.*?##/ { \
		printf "  \033[36m%-23s\033[0m %s\n", $$1, $$2 \
	} \
	/^##@/ { \
		printf "\n\033[1m%s\033[0m\n", substr($$0, 5) \
	} ' $(MAKEFILE_LIST)

# Additional help entries
buildenv-base: ## OCI image used for building Unikraft unikernels with kraft.
buildenv-base-golang: ## OCI image used for building Unikraft unikernels with kraft and golang included.
buildenv-gcc: ## OCI image containing a Unikraft-centric build of gcc.
buildenv-myself-full: ## OCI image containing the build environment for KraftKit.
buildenv-myself: ## OCI image containing KraftKit binaries.
buildenv-qemu: ## OCI image containing a Unikraft-centric build of QEMU.
buildenv-github-action: ## OCI image used when building Unikraft unikernels in GitHub Actions.
tools: ## Build all tools.
kraft: ## The kraft binary.
runu: ## The runu binary.
