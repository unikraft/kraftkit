# SPDX-License-Identifier: BSD-3-Clause
# Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
# Licensed under the BSD-3-Clause License (the "License").
# You may not use this file except in compliance with the License.
FROM kraftkit.sh/base-golang:latest

RUN set -xe; \
    apt-get update; \
    apt-get install -y --no-install-recommends \
      cmake \
      libssh2-1-dev \
      libssl-dev \
      pkg-config; \
    apt-get clean; \
    git config --global --add safe.directory /go/src/kraftkit.sh;

WORKDIR /go/src/kraftkit.sh

COPY . .

ENV DOCKER=
ENV GOROOT=/usr/local/go
ENV KRAFTKIT_LOG_LEVEL=debug
ENV KRAFTKIT_LOG_TYPE=basic
ENV PAGER=cat
ENV PATH=$PATH:/go/src/kraftkit.sh/dist
