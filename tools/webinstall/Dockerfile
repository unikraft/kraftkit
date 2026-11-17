# SPDX-License-Identifier: BSD-3-Clause
# Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
# Licensed under the BSD-3-Clause License (the "License").
# You may not use this file expect in compliance with the License.

FROM golang:1.27.4-trixie@sha256:f18c3d73a184fb0582828510722e723b63a93f7188d11dd223d9be6f6880e65d AS base

ARG ORG=unikraft
ARG BIN=webinstall

WORKDIR /go/src/webinstall

ENV GOROOT=/usr/local/go
ENV PATH=$PATH:/go/src/webinstall
ENV CGO_ENABLED=0

COPY . /go/src/webinstall

RUN set -xe; \
    go build -tags static -a -ldflags='-s -w' -ldflags '-extldflags "-static"' .

FROM gcr.io/distroless/static:nonroot@sha256:aafe993665bf6200db38a02c11a9351e782a84b6a45e7f33b6695638c88e3931 AS prod

COPY --from=base /go/src/webinstall/webinstall /webinstall

EXPOSE 8080

ENTRYPOINT ["/webinstall"]
