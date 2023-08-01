# SPDX-License-Identifier: BSD-3-Clause
# Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
# Licensed under the BSD-3-Clause License (the "License")
# You may not use this file except in compliance with the License.

# Meta directory association
_MKFILE_PATH := $(abspath $(lastword $(MAKEFILE_LIST)))
BUILDENVSDIR := $(patsubst %/,%,$(dir $(_MKFILE_PATH)))
WORKDIR      ?= $(BUILDENVSDIR)/..

REGISTRY     ?= kraftkit.sh
IMAGE_TAG    ?= latest

BUILDENVS    ?= gcc \
                qemu \
                myself \
                base

DOCKER       ?= docker

WITH_CACHE   ?= y

.PHONY: qemu
qemu: QEMU_VERSION ?= 7.2.4
qemu: MAKE_NPROC   ?= $(shell nproc)
qemu: ENVIRONMENT  ?= qemu
qemu: IMAGE        ?= $(REGISTRY)/qemu:$(QEMU_VERSION)
ifeq ($(WITH_CACHE),y)
qemu: _WITH_CACHE  := --cache-from $(IMAGE)
else
qemu: _WITH_CACHE  := --no-cache
endif
qemu:
	$(DOCKER) build \
		--build-arg QEMU_VERSION=$(QEMU_VERSION) \
		--build-arg MAKE_NPROC=$(MAKE_NPROC) \
		--tag $(IMAGE) \
		$(_WITH_CACHE) \
		--file $(BUILDENVSDIR)/qemu.Dockerfile \
		$(DOCKER_BUILD_EXTRA) $(WORKDIR)

.PHONY: myself
myself: GO_VERSION  ?= 1.20.6
myself: ENVIRONMENT ?= myself
myself: IMAGE       ?= $(REGISTRY)/myself:$(IMAGE_TAG)
myself: TARGET      ?= kraftkit
ifeq ($(WITH_CACHE),y)
myself: _WITH_CACHE := --cache-from $(IMAGE)
else
myself: _WITH_CACHE := --no-cache
endif
myself:
	$(DOCKER) build \
		--build-arg GO_VERSION=$(GO_VERSION) \
		--tag $(IMAGE) \
		--target $(TARGET) \
		$(_WITH_CACHE) \
		--file $(BUILDENVSDIR)/$(ENVIRONMENT).Dockerfile \
		$(DOCKER_BUILD_EXTRA) $(WORKDIR)

.PHONY: myself-full
myself-full:
	$(MAKE) TARGET=kraftkit-full IMAGE=$(REGISTRY)/myself-full:$(IMAGE_TAG) myself

.PHONY: base
base: ENVIRONMENT      ?= base
base: GO_VERSION       ?= 1.20.6
base: IMAGE            ?= $(REGISTRY)/base:$(IMAGE_TAG)
base: KRAFTKIT_VERSION ?= latest
base: QEMU_VERSION     ?= 7.2.4
ifeq ($(WITH_CACHE),y)
base: _WITH_CACHE      := --cache-from $(IMAGE)
else
base: _WITH_CACHE      := --no-cache
endif
base:
	$(DOCKER) build \
		--build-arg GO_VERSION=$(GO_VERSION) \
		--build-arg KRAFTKIT_VERSION=$(KRAFTKIT_VERSION) \
		--build-arg QEMU_VERSION=$(QEMU_VERSION) \
		--build-arg REGISTRY=$(REGISTRY) \
		--tag $(IMAGE) \
		$(_WITH_CACHE) \
		--file $(BUILDENVSDIR)/base.Dockerfile \
		$(DOCKER_BUILD_EXTRA) $(WORKDIR)

.PHONY: github-action
github-action: ENVIRONMENT      ?= github-action
github-action: GO_VERSION       ?= 1.20.6
github-action: IMAGE            ?= $(REGISTRY)/github-action:$(IMAGE_TAG)
ifeq ($(WITH_CACHE),y)
github-action: _WITH_CACHE      := --cache-from $(IMAGE)
else
github-action: _WITH_CACHE      := --no-cache
endif
github-action:
	$(DOCKER) build \
		--build-arg GO_VERSION=$(GO_VERSION) \
		--tag $(IMAGE) \
		$(_WITH_CACHE) \
		--file $(BUILDENVSDIR)/github-action.Dockerfile \
		$(DOCKER_BUILD_EXTRA) $(WORKDIR)
