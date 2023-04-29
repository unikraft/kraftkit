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
               -w /go/src/$(GOMOD) \
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
	$(Q)$(call DOCKER_RUN,,$(ENVIRONMENT),$(MAKE) -e $@)
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
$(addprefix $(.PROXY), $(BIN)): git2go tidy
$(addprefix $(.PROXY), $(BIN)):
	$(GO) build \
		-tags static \
		-gcflags=all='$(GO_GCFLAGS)' \
		-ldflags='$(GO_LDFLAGS)' \
		-o $(DISTDIR)/$@ \
		$(WORKDIR)/cmd/$@

# Proxy all "build environment" (buildenvs) targets
buildenv-%:
		$(MAKE) -C $(WORKDIR)/buildenvs $*

# Run an environment where we can build
.PHONY: devenv
devenv: DOCKER_RUN_EXTRA ?= -it --name $(REPO)-devenv
devenv: WITH_KVM         ?= n
devenv: $(VENDORDIR)/libgit2/git2go/vendor/libgit2
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
test: GOTEST_EXCLUDE := third_party/ test/ hack/ buildenvs/ dist/ docs/
test: GOTEST_PKGS := $(foreach pkg,$(filter-out $(GOTEST_EXCLUDE),$(wildcard */)),$(pkg)...)
test: ## Run unit tests.
	$(GO) run github.com/onsi/ginkgo/v2/ginkgo -v -p -randomize-all --tags static $(GOTEST_PKGS)

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

.PHONY: git2go
git2go: $(VENDORDIR)/libgit2/git2go/static-build/install/lib/pkgconfig/libgit2.pc
	$(GO) install -tags static github.com/libgit2/git2go/v31/...

$(VENDORDIR)/libgit2/git2go/static-build/install/lib/pkgconfig/libgit2.pc: $(VENDORDIR)/libgit2/git2go/vendor/libgit2
	$(MKDIR) -p $(VENDORDIR)/libgit2/git2go/static-build/build
	$(MKDIR) -p $(VENDORDIR)/libgit2/git2go/static-build/install
	(cd $(VENDORDIR)/libgit2/git2go/static-build/build && $(CMAKE) \
		-DTHREADSAFE=ON \
		-DBUILD_CLAR=OFF \
		-DBUILD_SHARED_LIBS=OFF \
		-DREGEX_BACKEND=builtin \
		-DUSE_BUNDLED_ZLIB=ON \
		-DUSE_HTTPS=ON \
		-DUSE_SSH=ON \
		-DCMAKE_C_FLAGS=-fPIC \
		-DCMAKE_BUILD_TYPE="RelWithDebInfo" \
		-DCMAKE_INSTALL_PREFIX=$(VENDORDIR)/libgit2/git2go/static-build/install \
		-DCMAKE_INSTALL_LIBDIR="lib" \
		-DDEPRECATE_HARD="${BUILD_DEPRECATE_HARD}" \
		$(VENDORDIR)/libgit2/git2go/vendor/libgit2)
	$(MAKE) -C $(VENDORDIR)/libgit2/git2go/static-build/build install

$(VENDORDIR)/libgit2/git2go/vendor/libgit2: $(VENDORDIR)/libgit2/git2go
	$(GIT) -C $(VENDORDIR)/libgit2/git2go submodule update --init --recursive

$(VENDORDIR)/libgit2/git2go:
	$(GIT) clone --branch v31.7.9 --recurse-submodules https://github.com/libgit2/git2go.git $@

.PHONY: help
help: ## Show this help menu and exit.
	@awk 'BEGIN { \
		FS = ":.*##"; \
		printf "KraftKit developer build targets.\n\n"; \
		printf "\033[1mUSAGE\033[0m\n"; \
		printf "  make [VAR=... [VAR=...]] \033[36mTARGET\033[0m\n\n"; \
		printf "\033[1mTARGETS\033[0m\n"; \
	} \
	/^[a-zA-Z_-]+:.*?##/ { \
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
kraft: ## The kraft binary.
