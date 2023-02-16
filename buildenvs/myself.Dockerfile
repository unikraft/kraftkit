# SPDX-License-Identifier: BSD-3-Clause
# Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
# Licensed under the BSD-3-Clause License (the "License").
# You may not use this file except in compliance with the License.

ARG GO_VERSION=1.20

FROM golang:${GO_VERSION}-bullseye AS base

ARG ORG=unikraft
ARG BIN=kraft
ARG GO_VERSION=${GO_VERSION}

RUN set -xe; \
    apt-get update; \
    apt-get install -y --no-install-recommends \
      build-essential \
      cmake \
      make \
      git; \
    go install mvdan.cc/gofumpt@latest; \
    git config --global --add safe.directory /go/src/kraftkit.sh;

WORKDIR /go/src/kraftkit.sh

ENV GOROOT=/usr/local/go
ENV PATH=$PATH:/go/src/kraftkit.sh/dist
