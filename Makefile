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
BIN         ?= kraft
TOOLS       ?= github-action \
               go-generate-qemu-devices \
               protoc-gen-go-netconn \
               webinstall
GOMOD       ?= kraftkit.sh
IMAGE_TAG   ?= latest
GO_VERSION  ?= 1.20

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
GOFUMPT     ?= gofumpt
GOCILINT    ?= golangci-lint
MKDIR       ?= mkdir
GIT         ?= git
CURL        ?= curl
CMAKE       ?= cmake

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
	$(info Running target via Docker...)
	$(Q)$(call DOCKER_RUN,,$(ENVIRONMENT),$(MAKE) GOOS=$(GOOS) GOARCH=$(GOARCH) -e $@)
	$(Q)exit 0
endif

# Targets
.PHONY: all
.DEFAULT: all
all: help

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
		-gcflags=all='$(GO_GCFLAGS)' \
		-ldflags='$(GO_LDFLAGS)' \
		-o $(DISTDIR)/$@ \
		$(WORKDIR)/cmd/$@

.PHONY: tools
tools: $(TOOLS)

$(addprefix $(.PROXY), $(TOOLS)):
	cd $(WORKDIR)/tools/$@ && $(GO) build -o $(DISTDIR)/$@ . && cd $(WORKDIR)

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
	$(GOCILINT) run

.PHONY: test
test: test-unit test-e2e ## Run all tests.

.PHONY: test-unit
test-unit: GOTEST_EXCLUDE := third_party/ test/ hack/ buildenvs/ dist/ docs/
test-unit: GOTEST_PKGS := $(foreach pkg,$(filter-out $(GOTEST_EXCLUDE),$(wildcard */)),$(pkg)...)
test-unit: ## Run unit tests.
	$(GO) run github.com/onsi/ginkgo/v2/ginkgo -v -p -randomize-all $(GOTEST_PKGS)

.PHONY: test-e2e
test-e2e: $(BIN) ## Run CLI end-to-end tests.
	$(GO) run github.com/onsi/ginkgo/v2/ginkgo -v -p -randomize-all test/e2e/cli/...

.PHONY: install-golangci-lint
install-golangci-lint: GOLANGCI_LINT_VERSION     ?= 1.51.2
install-golangci-lint: GOLANGCI_LINT_INSTALL_DIR ?= $$($(GO) env GOPATH)/bin
install-golangci-lint: ## Install the Golang CI lint tool
	$(CURL) -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(GOLANGCI_LINT_INSTALL_DIR) v$(GOLANGCI_LINT_VERSION)

.PHONY: clean
clean:
	$(GO) clean -modcache -cache -i -r

.PHONY: properclean
properclean: ENVIRONMENT ?= myself-full
properclean: IMAGE       ?= $(REGISTRY)/$(ENVIRONMENT):$(IMAGE_TAG)
properclean: ## Completely clean the repository's build artifacts.
	rm -rf $(DISTDIR) $(TESTDIR)
	$(DOCKER) rmi $(IMAGE)

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
buildenv-gcc: ## OCI image containing a Unikraft-centric build of gcc.
buildenv-myself-full: ## OCI image containing the build environment for KraftKit.
buildenv-myself: ## OCI image containing KraftKit binaries.
buildenv-qemu: ## OCI image containing a Unikraft-centric build of QEMU.
buildenv-github-action: ## OCI image used when building Unikraft unikernels in GitHub Actions.
tools: ## Build all tools.
kraft: ## The kraft binary.
