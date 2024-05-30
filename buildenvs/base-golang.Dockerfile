# SPDX-License-Identifier: BSD-3-Clause
# Copyright (c) 2022, NEC Europe Ltd., Unikraft GmbH, and The KraftKit Authors.
# Licensed under the BSD-3-Clause License (the "License").
# You may not use this file except in compliance with the License.
ARG REGISTRY=kraftkit.sh
ARG GO_VERSION=1.22.3

FROM golang:${GO_VERSION}-bookworm AS golang
FROM ${REGISTRY}/base:latest

COPY --from=golang /usr/local/go /usr/local/go

ENV PATH=$PATH:/usr/local/go/bin
