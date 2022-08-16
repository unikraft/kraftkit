# SPDX-License-Identifier: BSD-3-Clause
#
# Authors: Alexander Jung <alex@unikraft.io>
#
# Copyright (c) 2022, Unikraft GmbH.  All rights reserved.
#
# Redistribution and use in source and binary forms, with or without
# modification, are permitted provided that the following conditions
# are met:
#
# 1. Redistributions of source code must retain the above copyright
#    notice, this list of conditions and the following disclaimer.
# 2. Redistributions in binary form must reproduce the above copyright
#    notice, this list of conditions and the following disclaimer in the
#    documentation and/or other materials provided with the distribution.
# 3. Neither the name of the copyright holder nor the names of its
#    contributors may be used to endorse or promote products derived from
#    this software without specific prior written permission.
#
# THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
# AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
# IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
# ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE
# LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR
# CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF
# SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
# INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN
# CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
# ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
# POSSIBILITY OF SUCH DAMAGE.

# Directories
WORKDIR     ?= $(CURDIR)
TESTDIR     ?= $(WORKDIR)/tests
DISTDIR     ?= $(WORKDIR)/dist
INSTALLDIR  ?= /usr/local/bin/

# Arguments
REGISTRY    ?= ghcr.io
ORG         ?= unikraft
REPO        ?= kraftkit
BIN         ?= kraftkit \
               ukbuild \
               ukpkg
GOMOD       ?= kraftkit.sh
IMAGE_TAG   ?= latest
GO_VERSION  ?= 1.17

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
$(addprefix $(.PROXY), $(BIN)): deps
$(addprefix $(.PROXY), $(BIN)):
	$(GO) build \
		-gcflags=all='$(GO_GCFLAGS)' \
		-ldflags='$(GO_LDFLAGS)' \
		-o $(DISTDIR)/$@ \
		$(WORKDIR)/cmd/$@

# Create an environment where we can build
.PHONY: container
container: GO_VERSION         ?= 1.17
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
devenv:
	$(Q)$(call DOCKER_RUN,$(DOCKER_RUN_EXTRA),myself,bash)

.PHONY: deps
deps:
	$(GO) mod tidy -compat=$(GO_VERSION)

.PHONY: fmt
fmt:
	$(GOFUMPT) -e -l -w $(WORKDIR)

.PHONY: clean
clean:
	$(GO) clean -cache -i -r

