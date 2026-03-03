# SPDX-License-Identifier: BSD-3-Clause
# Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
# Licensed under the BSD-3-Clause License (the "License").
# You may not use this file expect in compliance with the License.

ARG GO_VERSION=1.25

FROM golang:${GO_VERSION}-bullseye AS base

ARG ORG=unikraft
ARG BIN=webinstall
ARG GO_VERSION=${GO_VERSION}

WORKDIR /go/src/webinstall

ENV GOROOT=/usr/local/go
ENV PATH=$PATH:/go/src/webinstall
ENV CGO_ENABLED=0

COPY . /go/src/webinstall

RUN set -xe; \
    go build -tags static -a -ldflags='-s -w' -ldflags '-extldflags "-static"' .

FROM gcr.io/distroless/static:nonroot AS prod

COPY --from=base /go/src/webinstall/webinstall /webinstall

EXPOSE 8080

ENTRYPOINT ["/webinstall"]
