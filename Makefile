# SPDX-License-Identifier: BSD-3-Clause
# Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
# Licensed under the BSD-3-Clause License (the "License").
# You may not use this file except in compliance with the License.

# Directories
WORKDIR     ?= $(CURDIR)
TESTDIR     ?= $(WORKDIR)/tests
DISTDIR     ?= $(WORKDIR)/dist
INSTALLDIR  ?= /usr/local/bin/
VENDORDIR   ?= $(WORKDIR)/vendor

# Arguments
REGISTRY    ?= ghcr.io
ORG         ?= unikraft
REPO        ?= kraftkit
BIN         ?= kraft
GOMOD       ?= kraftkit.sh
IMAGE_TAG   ?= latest
GO_VERSION  ?= 1.18

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
               $(REGISTRY)/$(ORG)/$(REPO)/$(2):$(IMAGE_TAG) \
                 $(3)
GO          ?= go
GOFUMPT     ?= gofumpt
GOCILINT    ?= golangci-lint
MKDIR       ?= mkdir
GIT         ?= git

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
$(BIN):
	$(info Running target via Docker...)
	$(Q)$(call DOCKER_RUN,,$(MAKE) -e $@)
	$(Q)exit 0
endif

# Targets
.PHONY: all
$(.PROXY)all: $(BIN)

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
		-mod=readonly \
		-gcflags=all='$(GO_GCFLAGS)' \
		-ldflags='$(GO_LDFLAGS)' \
		-o $(DISTDIR)/$@ \
		$(WORKDIR)/cmd/$@

# Create an environment where we can build
.PHONY: container
container: GO_VERSION         ?= 1.18
container: DOCKER_BUILD_EXTRA ?=
container: ENVIRONMENT        ?= myself
container: IMAGE              ?= $(REGISTRY)/$(ORG)/$(REPO)/$(ENVIRONMENT):$(IMAGE_TAG)
container: TARGET             ?= base
container:
	$(DOCKER) build \
		--build-arg ORG=$(ORG) \
		--build-arg REPO=$(REPO) \
		--build-arg GO_VERSION=$(GO_VERSION) \
		--tag $(IMAGE) \
		--target $(TARGET) \
		--file $(WORKDIR)/buildenvs/$(ENVIRONMENT).Dockerfile \
		$(DOCKER_BUILD_EXTRA) $(WORKDIR)

# Run an environment where we can build
.PHONY: devenv
devenv: DOCKER_RUN_EXTRA ?= -it --name $(REPO)-devenv
devenv: WITH_KVM         ?= n
devenv: $(VENDORDIR)/github.com/libgit2/git2go/v31/vendor/libgit2
devenv:
ifeq ($(WITH_KVM),y)
	$(Q)$(call DOCKER_RUN,--device /dev/kvm $(DOCKER_RUN_EXTRA),myself,bash)
else
	$(Q)$(call DOCKER_RUN,$(DOCKER_RUN_EXTRA),myself,bash)
endif

.PHONY: tidy
tidy:
	$(GO) mod tidy -compat=$(GO_VERSION)

.PHONY: fmt
fmt:
	$(GOFUMPT) -e -l -w $(WORKDIR)

.PHONY: cicheck
cicheck:
	$(GOCILINT) run

.PHONY: clean
clean:
	$(GO) clean -cache -i -r

.PHONY: properclean
properclean: ENVIRONMENT ?= myself
properclean: IMAGE       ?= $(REGISTRY)/$(ORG)/$(REPO)/$(ENVIRONMENT):$(IMAGE_TAG)
properclean:
	rm -rf $(DISTDIR) $(TESTDIR)
	$(DOCKER) rmi $(IMAGE)

.PHONY: git2go
git2go: $(VENDORDIR)/github.com/libgit2/git2go/v31/static-build/install/lib/pkgconfig/libgit2.pc

$(VENDORDIR)/github.com/libgit2/git2go/v31/static-build/install/lib/pkgconfig/libgit2.pc: $(VENDORDIR)/github.com/libgit2/git2go/v31/vendor/libgit2
	$(MAKE) -C $(VENDORDIR)/github.com/libgit2/git2go/v31 install-static

$(VENDORDIR)/github.com/libgit2/git2go/v31/vendor/libgit2: $(VENDORDIR)/github.com/libgit2/git2go
	$(GIT) -C $(VENDORDIR)/github.com/libgit2/git2go/v31 submodule update --init --recursive

$(VENDORDIR)/github.com/libgit2/git2go:
	$(GIT) clone --branch v31.7.9 --recurse-submodules https://github.com/libgit2/git2go.git $@/v31
