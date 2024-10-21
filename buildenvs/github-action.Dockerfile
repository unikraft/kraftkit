# SPDX-License-Identifier: BSD-3-Clause
# Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
# Licensed under the BSD-3-Clause License (the "License").
# You may not use this file except in compliance with the License.
ARG GO_VERSION=1.22.3
ARG DEBIAN_VERSION=bookworm-20240513
ARG KRAFTKIT_VERSION=latest
ARG QEMU_VERSION=8.2.4
ARG REGISTRY=kraftkit.sh

FROM golang:${GO_VERSION}-bullseye AS build

COPY . /go/src/kraftkit.sh

WORKDIR /go/src/kraftkit.sh

ENV GOROOT=/usr/local/go

RUN set -xe; \
    git config \
        --global \
        --add safe.directory /go/src/kraftkit.sh; \
    DOCKER= DISTDIR=/ make github-action;

FROM ${REGISTRY}/qemu:${QEMU_VERSION} AS qemu
FROM ${REGISTRY}/myself:${KRAFTKIT_VERSION} AS kraftkit
FROM debian:${DEBIAN_VERSION}         AS base

COPY --from=qemu  /bin/          /usr/local/bin
COPY --from=qemu  /share/qemu/   /share/qemu
COPY --from=qemu  /lib/x86_64-linux-gnu/ /lib/x86_64-linux-gnu
COPY --from=build /github-action /usr/local/bin/github-action
COPY --from=kraftkit /kraft       /usr/local/bin/kraft
COPY --from=hairyhenderson/gomplate:stable /gomplate /bin/gomplate

# Install unikraft dependencies
RUN set -xe; \
    apt-get update; \
    apt-get install -y --no-install-recommends \
      autoconf \
      bindgen \
      bison \
      bzip2 \
      ca-certificates \
      cmake \
      curl \
      flex \
      g++-12 \
      gawk \
      gcc-12 \
      gcc-12-aarch64-linux-gnu \
      gcc-12-arm-linux-gnueabihf \
      git \
      jq \
      libncursesw5 \
      libncursesw5-dev \
      make \
      openssh-client \
      patch \
      python3 \
      socat \
      unzip \
      uuid-runtime \
      wget \
      xz-utils \
      zip \
      sudo \
      ; \
    apt-get clean; \
    rm -Rf /var/cache/apt/*; \
    rm -Rf /var/lib/apt/lists/*

RUN ln -s /usr/bin/cpp-12                                   /usr/bin/cc; \
    ln -s /usr/bin/cpp-12                                   /usr/bin/cpp; \
    ln -s /usr/bin/gcc-12                                   /usr/bin/gcc; \
    ln -s /usr/bin/gcc-ar-12                                /usr/bin/gcc-ar; \
    ln -s /usr/bin/gcc-nm-12                                /usr/bin/gcc-nm; \
    ln -s /usr/bin/gcc-ranlib-12                            /usr/bin/gcc-ranlib; \
    ln -s /usr/bin/gcov-12                                  /usr/bin/gcov; \
    ln -s /usr/bin/gcov-dump-12                             /usr/bin/gcov-dump; \
    ln -s /usr/bin/gcov-tool-12                             /usr/bin/gcov-tool; \
    ln -s /usr/bin/lto-tool-12                              /usr/bin/lto-tool; \
    ln -s /usr/bin/aarch64-linux-gnu-cpp-12                 /usr/bin/aarch64-linux-gnu-cpp; \
    ln -s /usr/bin/aarch64-linux-gnu-gcc-12                 /usr/bin/aarch64-linux-gnu-gcc; \
    ln -s /usr/bin/aarch64-linux-gnu-gcc-ar-12              /usr/bin/aarch64-linux-gnu-gcc-ar; \
    ln -s /usr/bin/aarch64-linux-gnu-gcc-nm-12              /usr/bin/aarch64-linux-gnu-gcc-nm; \
    ln -s /usr/bin/aarch64-linux-gnu-gcc-ranlib-12          /usr/bin/aarch64-linux-gnu-gcc-ranlib; \
    ln -s /usr/bin/aarch64-linux-gnu-gcov-12                /usr/bin/aarch64-linux-gnu-gcov; \
    ln -s /usr/bin/aarch64-linux-gnu-gcov-dump-12           /usr/bin/aarch64-linux-gnu-gcov-dump; \
    ln -s /usr/bin/aarch64-linux-gnu-gcov-tool-12           /usr/bin/aarch64-linux-gnu-gcov-tool; \
    ln -s /usr/bin/aarch64-linux-gnu-lto-tool-12            /usr/bin/aarch64-linux-gnu-lto-tool; \
    ln -s /usr/bin/gcc-12-arm-linux-gnueabihf-cpp-12        /usr/bin/gcc-12-arm-linux-gnueabihf-cpp; \
    ln -s /usr/bin/gcc-12-arm-linux-gnueabihf-gcc-12        /usr/bin/gcc-12-arm-linux-gnueabihf-gcc; \
    ln -s /usr/bin/gcc-12-arm-linux-gnueabihf-gcc-ar-12     /usr/bin/gcc-12-arm-linux-gnueabihf-gcc-ar; \
    ln -s /usr/bin/gcc-12-arm-linux-gnueabihf-gcc-nm-12     /usr/bin/gcc-12-arm-linux-gnueabihf-gcc-nm; \
    ln -s /usr/bin/gcc-12-arm-linux-gnueabihf-gcc-ranlib-12 /usr/bin/gcc-12-arm-linux-gnueabihf-gcc-ranlib; \
    ln -s /usr/bin/gcc-12-arm-linux-gnueabihf-gcov-12       /usr/bin/gcc-12-arm-linux-gnueabihf-gcov; \
    ln -s /usr/bin/gcc-12-arm-linux-gnueabihf-gcov-dump-12  /usr/bin/gcc-12-arm-linux-gnueabihf-gcov-dump; \
    ln -s /usr/bin/gcc-12-arm-linux-gnueabihf-gcov-tool-12  /usr/bin/gcc-12-arm-linux-gnueabihf-gcov-tool; \
    ln -s /usr/bin/gcc-12-arm-linux-gnueabihf-lto-tool-12   /usr/bin/gcc-12-arm-linux-gnueabihf-lto-tool;

WORKDIR /github/workspace

RUN set -xe; \
    groupadd -g 127 runner; \
    groupadd -g 1000 arunner; \
    groupadd -g 1001 brunner; \
    groupadd -g 1002 crunner; \
    useradd -rm -d /home/runner -s /bin/bash -g runner -G sudo -u 1001 runner; \
    sed -i /etc/sudoers -re 's/^%sudo.*/%sudo ALL=(ALL:ALL) NOPASSWD: ALL/g'; \
    sed -i /etc/sudoers -re 's/^root.*/root ALL=(ALL:ALL) NOPASSWD: ALL/g'; \
    sed -i /etc/sudoers -re 's/^#includedir.*/## **Removed the include directive** ##"/g'; \
    echo "runner ALL=(ALL) NOPASSWD: ALL" >> /etc/sudoers; \
    chown -R runner:runner /github/workspace; \
    usermod -aG 1000 runner; \
    usermod -aG 1001 runner; \
    usermod -aG 1002 runner; \
    mkdir -p /run/user/1000; \
    mkdir -p /run/user/1001; \
    mkdir -p /run/user/1002; \
    chown -R runner:arunner /run/user/1000; \
    chown -R runner:brunner /run/user/1001; \
    chown -R runner:crunner /run/user/1002; \
    chmod 0700 /run/user/1000; \
    chmod 0700 /run/user/1001; \
    chmod 0700 /run/user/1002;
USER runner

ENV CLICOLOR_FORCE=1

ENTRYPOINT [ "github-action" ]
