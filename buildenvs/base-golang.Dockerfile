# SPDX-License-Identifier: BSD-3-Clause
# Copyright (c) 2022, NEC Europe Ltd., Unikraft GmbH, and The KraftKit Authors.
# Licensed under the BSD-3-Clause License (the "License").
# You may not use this file except in compliance with the License.
ARG REGISTRY=kraftkit.sh

FROM golang:1.27.4-trixie@sha256:f18c3d73a184fb0582828510722e723b63a93f7188d11dd223d9be6f6880e65d AS golang
FROM ${REGISTRY}/base:latest

COPY --from=golang /usr/local/go /usr/local/go

ENV PATH=$PATH:/usr/local/go/bin
