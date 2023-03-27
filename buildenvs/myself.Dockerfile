# SPDX-License-Identifier: BSD-3-Clause
# Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
# Licensed under the BSD-3-Clause License (the "License").
# You may not use this file except in compliance with the License.

ARG GO_VERSION=1.20.2

FROM golang:${GO_VERSION}-bullseye AS kraftkit-full

# Install build dependencies
RUN set -xe; \
    apt-get update; \
    apt-get install -y --no-install-recommends \
      build-essential=12.9 \
      cmake==3.18.4-2+deb11u1 \
      make=4.3-4.1 \
      pkg-config=0.29.2-1 \
      git=1:2.30.2-1+deb11u2; \
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