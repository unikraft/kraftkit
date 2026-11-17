# SPDX-License-Identifier: BSD-3-Clause
# Copyright (c) 2022, NEC Europe Ltd., Unikraft GmbH, and The KraftKit Authors.
# Licensed under the BSD-3-Clause License (the "License").
# You may not use this file except in compliance with the License.

FROM golang:1.27.4@sha256:205c7636d74e0ffbe3d007162df7d74f859f5a8c8322dfb9f907977686c126cf AS builder

WORKDIR /src
COPY . ./
RUN CGO_ENABLED=0 go build -o /kraft-llb-plugin -tags "containers_image_storage_stub,containers_image_openpgp" --ldflags "-s -w"

FROM scratch
COPY --from=builder /kraft-llb-plugin /bin/kraft-llb-plugin
ENTRYPOINT ["/bin/kraft-llb-plugin"]
