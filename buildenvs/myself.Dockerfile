# SPDX-License-Identifier: BSD-3-Clause
# Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
# Licensed under the BSD-3-Clause License (the "License").
# You may not use this file except in compliance with the License.

ARG GO_VERSION=1.20.2

FROM golang:${GO_VERSION}-bullseye AS golang

FROM ubuntu:22.04 AS kraftkit-full

# https://github.com/docker-library/golang/blob/5c6fa890/1.20/bullseye/Dockerfile
ENV PATH /usr/local/go/bin:$PATH
ENV GOLANG_VERSION ${GO_VERSION}
COPY --from=golang /usr/local/go /usr/local/go
ENV GOPATH /go
ENV PATH $GOPATH/bin:$PATH
RUN mkdir -p "$GOPATH/src" "$GOPATH/bin" && chmod -R 1777 "$GOPATH"

# Install build dependencies
RUN set -xe; \
    apt-get update; \
    apt-get install -y --no-install-recommends \
      ca-certificates=20211016ubuntu0.22.04.1 \
      curl=7.81.0-1ubuntu1.10 \
      build-essential=12.9ubuntu3 \
      cmake=3.22.1-1ubuntu1.22.04.1 \
      libssh2-1-dev=1.10.0-3 \
      libssl-dev=3.0.2-0ubuntu1.9 \
      pkg-config=0.29.2-1ubuntu3 \
      openssh-client=1:8.9p1-3ubuntu0.1 \
      git=1:2.34.1-1ubuntu1.9; \
    apt-get clean; \
    go install mvdan.cc/gofumpt@v0.4.0; \
    git config --global --add safe.directory /go/src/kraftkit.sh;

# Install YTT
RUN set -xe; \
    curl -s -L https://github.com/vmware-tanzu/carvel-ytt/releases/download/v0.41.1/ytt-linux-amd64 > /tmp/ytt; \
    echo "65dbc4f3a4a2ed84296dd1b323e8e7bd77e488fa7540d12dd36cf7fb2fc77c03  /tmp/ytt" | sha256sum -c -; \
    mv /tmp/ytt /usr/local/bin/ytt; \
    chmod +x /usr/local/bin/ytt;

WORKDIR /go/src/kraftkit.sh

COPY --from=ghcr.io/goreleaser/goreleaser-cross:v1.20.2 /usr/bin/goreleaser /usr/bin/

ENV DOCKER=
ENV GOROOT=/usr/local/go
ENV KRAFTKIT_LOG_LEVEL=debug
ENV KRAFTKIT_LOG_TYPE=basic
ENV PAGER=cat
ENV PATH=$PATH:/go/src/kraftkit.sh/dist

FROM kraftkit-full AS kraftkit-build

COPY . .

# Build the binary
RUN set -xe; \
    make kraft; \
    kraft -h;

FROM scratch AS kraftkit

COPY --from=kraftkit-build /go/src/kraftkit.sh/dist/kraft /kraft

ENTRYPOINT [ "/kraft" ]
