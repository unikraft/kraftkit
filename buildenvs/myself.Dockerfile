# SPDX-License-Identifier: BSD-3-Clause
# Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
# Licensed under the BSD-3-Clause License (the "License").
# You may not use this file except in compliance with the License.

ARG DEBIAN_VERSION=trixie
ARG XEN_VERSION=4.19
ARG REGISTRY=kraftkit.sh

FROM ${REGISTRY}/xen:${XEN_VERSION} AS xen
FROM debian:${DEBIAN_VERSION}       AS kraftkit-full

# Install build dependencies
RUN set -xe; \
    apt-get update; \
    apt-get install -y --no-install-recommends \
      build-essential \
      ca-certificates \
      clang \
      cmake \
      curl \
      git \
      liblzo2-dev \
      libnl-3-dev \
      libnl-genl-3-dev \
      libnl-route-3-dev \
      libssh2-1-dev \
      libssl-dev \
      libyajl-dev \
      make \
      pkg-config \
    ; \
    apt-get clean;

ARG GO_VERSION=1.25.7

# Install Go
RUN set -xe; \
    curl -Lo /tmp/go.tar.gz https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz; \
    rm -rf /usr/local/go && tar -C /usr/local -xzf /tmp/go.tar.gz

ENV PATH="${PATH}:/usr/local/go/bin"

# Install YTT and Cosign
RUN set -xe; \
    curl -s -L https://github.com/vmware-tanzu/carvel-ytt/releases/download/v0.48.0/ytt-linux-amd64 > /tmp/ytt; \
    echo "090dc914c87e5ba5861e37f885f12bac3b15559c183c30d4af2e63ccab03d5f9  /tmp/ytt" | sha256sum -c -; \
    mv /tmp/ytt /usr/local/bin/ytt; \
    chmod +x /usr/local/bin/ytt; \
    curl -s -O -L "https://github.com/sigstore/cosign/releases/latest/download/cosign-linux-amd64"; \
    mv cosign-linux-amd64 /usr/local/bin/cosign; \
    chmod +x /usr/local/bin/cosign;

COPY --from=xen /usr/local/lib/libxen*.a /usr/local/lib/libxen*.so* /usr/local/lib/
COPY --from=xen /usr/local/include/* /usr/local/include/
COPY --from=xen /usr/lib/x86_64-linux-gnu/liblzma.a \
                /usr/lib/x86_64-linux-gnu/libbz2.a \
                /usr/lib/x86_64-linux-gnu/libzstd.a \
                /usr/lib/x86_64-linux-gnu/liblzo2.a \
                /usr/lib/x86_64-linux-gnu/libyajl.a \
                /usr/lib/x86_64-linux-gnu/libz.a \
                /usr/lib/x86_64-linux-gnu/libnl-route-3.a \
                /usr/lib/x86_64-linux-gnu/libnl-3.a \
                /usr/lib/x86_64-linux-gnu/libuuid.a \
                /usr/lib/x86_64-linux-gnu/libutil.a \
                /usr/lib/x86_64-linux-gnu/

WORKDIR /go/src/kraftkit.sh

COPY --from=ghcr.io/goreleaser/goreleaser-cross:v1.25.7-v2.13.3 /usr/bin/goreleaser /usr/bin/

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
    git config --global --add safe.directory /go/src/kraftkit.sh; \
    make kraft; \
    kraft -h;

FROM scratch AS kraftkit

COPY --from=kraftkit-build /go/src/kraftkit.sh/dist/kraft /kraft

ENTRYPOINT [ "/kraft" ]
