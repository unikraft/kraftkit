# SPDX-License-Identifier: BSD-3-Clause
# Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
# Licensed under the BSD-3-Clause License (the "License").
# You may not use this file except in compliance with the License.

FROM ubuntu:latest AS build

RUN mkdir -p /tmp/build && \
    echo "<p>Hello World!</p>" > /tmp/build/hello.txt

FROM scratch AS import

COPY --from=build /tmp/build/hello.txt /index.html
