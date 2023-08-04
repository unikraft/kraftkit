# SPDX-License-Identifier: BSD-3-Clause
# Copyright (c) 2022, NEC Europe Ltd., Unikraft GmbH, and The KraftKit Authors.
# Licensed under the BSD-3-Clause License (the "License").
# You may not use this file except in compliance with the License.

FROM golang:1.20 AS builder

WORKDIR /src
COPY . ./
RUN CGO_ENABLED=0 go build -o /kraft-llb-plugin --ldflags "-s -w"

FROM scratch
COPY --from=builder /kraft-llb-plugin /bin/kraft-llb-plugin
ENTRYPOINT ["/bin/kraft-llb-plugin"]
