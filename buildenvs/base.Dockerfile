# SPDX-License-Identifier: BSD-3-Clause
# Copyright (c) 2022, NEC Europe Ltd., Unikraft GmbH, and The KraftKit Authors.
# Licensed under the BSD-3-Clause License (the "License").
# You may not use this file except in compliance with the License.

ARG DEBIAN_VERSION=bullseye-20221114
ARG GCC_SUFFIX=
ARG GCC_VERSION=12.2.0
ARG KRAFTKIT_VERSION=latest
ARG QEMU_VERSION=7.2.4
ARG REGISTRY=kraftkit.sh
ARG UK_ARCH=x86_64

FROM ${REGISTRY}/gcc:${GCC_VERSION}-x86_64${GCC_SUFFIX} AS gcc-x86_64
# FROM ${REGISTRY}/gcc:${GCC_VERSION}-arm${GCC_SUFFIX} AS gcc-arm
# FROM ${REGISTRY}/gcc:${GCC_VERSION}-arm64${GCC_SUFFIX} AS gcc-arm64
FROM ${REGISTRY}/qemu:${QEMU_VERSION} AS qemu
FROM ${REGISTRY}/myself:${KRAFTKIT_VERSION} AS kraftkit
FROM debian:${DEBIAN_VERSION} AS base

ARG GCC_VERSION=12.2.0

COPY --from=gcc-x86_64 /bin/ /bin
COPY --from=gcc-x86_64 /lib/gcc/ /lib/gcc
COPY --from=gcc-x86_64 /include/ /include
COPY --from=gcc-x86_64 /x86_64-linux-gnu /x86_64-linux-gnu
COPY --from=gcc-x86_64 /libexec/gcc/x86_64-linux-gnu/${GCC_VERSION}/cc1 /libexec/gcc/x86_64-linux-gnu/${GCC_VERSION}/cc1
COPY --from=gcc-x86_64 /libexec/gcc/x86_64-linux-gnu/${GCC_VERSION}/collect2 /libexec/gcc/x86_64-linux-gnu/${GCC_VERSION}/collect2
# COPY --from=gcc-arm /bin/ /bin
# COPY --from=gcc-arm /lib/gcc/ /lib/gcc
# COPY --from=gcc-arm /include/ /include
# COPY --from=gcc-arm /lib/gcc/ /lib/gcc
# COPY --from=gcc-arm /arm-linux-gnueabihf /arm-linux-gnueabihf
# COPY --from=gcc-arm /libexec/gcc/arm-linux-gnueabihf/${GCC_VERSION}/cc1 /libexec/gcc/arm-linux-gnueabihf/${GCC_VERSION}/cc1
# COPY --from=gcc-arm /libexec/gcc/arm-linux-gnueabihf/${GCC_VERSION}/collect2 /libexec/gcc/arm-linux-gnueabihf/${GCC_VERSION}/collect2
# COPY --from=gcc-arm64 /bin/ /bin
# COPY --from=gcc-arm64 /lib/gcc/ /lib/gcc
# COPY --from=gcc-arm64 /include/ /include
# COPY --from=gcc-arm64 /lib/gcc/ /lib/gcc
# COPY --from=gcc-arm64 /aarch64-linux-gnu/ /aarch64-linux-gnu
# COPY --from=gcc-arm64 /libexec/gcc/aarch64-linux-gnu/${GCC_VERSION}/cc1 /libexec/gcc/aarch64-linux-gnu/${GCC_VERSION}/cc1
# COPY --from=gcc-arm64 /libexec/gcc/aarch64-linux-gnu/${GCC_VERSION}/collect2 /libexec/gcc/aarch64-linux-gnu/${GCC_VERSION}/collect2
COPY --from=qemu /bin/ /usr/local/bin
COPY --from=qemu /share/qemu/ /share/qemu
COPY --from=qemu /lib/x86_64-linux-gnu/ /lib/x86_64-linux-gnu
# COPY --from=qemu /lib/arm-linux-gnueabihf/ /lib/arm-linux-gnueabihf
# COPY --from=qemu /lib/aarch64-linux-gnu/ /lib/aarch64-linux-gnu
COPY --from=kraftkit /kraft /usr/local/bin

ARG GCC_PREFIX=x86_64-linux-gnu

# Link the GCC toolchain
RUN set -xe; \
    ln -s /bin/${GCC_PREFIX}-as         /bin/as; \
    ln -s /bin/${GCC_PREFIX}-ar         /bin/ar; \
    ln -s /bin/${GCC_PREFIX}-c++        /bin/c++; \
    ln -s /bin/${GCC_PREFIX}-c++filt    /bin/c++filt; \
    ln -s /bin/${GCC_PREFIX}-elfedit    /bin/elfedit; \
    ln -s /bin/${GCC_PREFIX}-gcc        /bin/cc; \
    ln -s /bin/${GCC_PREFIX}-gcc        /bin/gcc; \
    ln -s /bin/${GCC_PREFIX}-gcc-ar     /bin/gcc-ar; \
    ln -s /bin/${GCC_PREFIX}-gcc-nm     /bin/gcc-nm; \
    ln -s /bin/${GCC_PREFIX}-gcc-ranlib /bin/gcc-ranlib; \
    ln -s /bin/${GCC_PREFIX}-gccgo      /bin/gccgo; \
    ln -s /bin/${GCC_PREFIX}-gcov       /bin/gcov; \
    ln -s /bin/${GCC_PREFIX}-gcov-dump  /bin/gcov-dump; \
    ln -s /bin/${GCC_PREFIX}-gcov-tool  /bin/gcov-tool; \
    ln -s /bin/${GCC_PREFIX}-gprof      /bin/gprof; \
    ln -s /bin/${GCC_PREFIX}-ld         /bin/ld; \
    ln -s /bin/${GCC_PREFIX}-nm         /bin/nm; \
    ln -s /bin/${GCC_PREFIX}-objcopy    /bin/objcopy; \
    ln -s /bin/${GCC_PREFIX}-objdump    /bin/objdump; \
    ln -s /bin/${GCC_PREFIX}-ranlib     /bin/ranlib; \
    ln -s /bin/${GCC_PREFIX}-readelf    /bin/readelf; \
    ln -s /bin/${GCC_PREFIX}-size       /bin/size; \
    ln -s /bin/${GCC_PREFIX}-strings    /bin/strings; \
    ln -s /bin/${GCC_PREFIX}-strip      /bin/strip;

ENV LC_ALL=C.UTF-8
ENV LANG=C.UTF-8
ENV KRAFTKIT_LOG_TYPE=basic
ENV DOCKER=

# Install unikraft dependencies
RUN set -xe; \
    apt-get update; \
    apt-get install -y --no-install-recommends \
      make=4.3-4.1 \
      libncursesw5-dev=6.2+20201114-2+deb11u1 \
      libncursesw5=6.2+20201114-2+deb11u1 \
      libyaml-dev=0.2.2-1 \
      flex=2.6.4-8 \
      git=1:2.30.2-1+deb11u2 \
      wget=1.21-1+deb11u1 \
      patch=2.7.6-7 \
      gawk=1:5.1.0-1 \
      socat=1.7.4.1-3 \
      bison=2:3.7.5+dfsg-1 \
      bindgen=0.55.1-3+b1 \
      bzip2=1.0.8-4 \
      unzip=6.0-26+deb11u1 \
      uuid-runtime=2.36.1-8+deb11u1 \
      openssh-client=1:8.4p1-5+deb11u1 \
      autoconf=2.69-14 \
      xz-utils=5.2.5-2.1~deb11u1 \
      python3=3.9.2-3 \
      ca-certificates=20210119; \
    apt-get clean; \
    rm -Rf /var/cache/apt/*; \
    rm -Rf /var/lib/apt/lists/*; \
    kraft --log-type basic --log-level debug pkg update;

WORKDIR /workspace

ENTRYPOINT [ "kraft" ]