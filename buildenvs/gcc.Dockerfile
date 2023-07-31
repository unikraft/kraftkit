# SPDX-License-Identifier: BSD-3-Clause
# Copyright (c) 2022, NEC Europe Ltd., Unikraft GmbH, and The KraftKit Authors.
# Licensed under the BSD-3-Clause License (the "License").
# You may not use this file except in compliance with the License.

ARG DEBIAN_VERSION=bullseye-20221114

FROM debian:${DEBIAN_VERSION} AS gcc-build

ARG BINUTILS_VERSION=2.39
ARG GCC_VERSION=12.2.0
ARG UK_ARCH=x86_64
ARG GLIB_VERSION=2.31
ARG MAKE_NPROC=1
ENV PREFIX=/out

RUN set -ex; \
    apt-get update; \
    apt-get install -y \
        bison \
        build-essential \
        bzip2 \
        curl \
        file \
        flex \
        gcc \
        libc6-dev \
        libgmp3-dev \
        libgnutls28-dev \
        libisl-dev \
        libmpc-dev \
        libmpfr-dev \
        make \
        patch \
        texinfo \
        wget \
        ; \
    apt-get clean; \
    mkdir -p ${PREFIX}/src; \
    cd ${PREFIX}/src; \
    curl -O https://ftp.fu-berlin.de/unix/gnu/binutils//binutils-${BINUTILS_VERSION}.tar.gz; \
    tar zxf binutils-${BINUTILS_VERSION}.tar.gz; \
    rm binutils-${BINUTILS_VERSION}.tar.gz; \
    chown -R root:root binutils-${BINUTILS_VERSION}; \
    chmod -R o-w,g+w binutils-${BINUTILS_VERSION}; \
    mkdir -p ${PREFIX}/src/build-binutils; \
    cd ${PREFIX}/src/build-binutils; \
    BINUTILS_CONFIGURE_ARGS="\
        --prefix=${PREFIX} \
        --with-sysroot \
        --disable-nls \
        --disable-werror"; \
    case ${UK_ARCH} in \
        x86_64) \
            BINUTILS_CONFIGURE_ARGS="$BINUTILS_CONFIGURE_ARGS \
                --target=x86_64-linux-gnu" \
            ;; \
        arm) \
            BINUTILS_CONFIGURE_ARGS="$BINUTILS_CONFIGURE_ARGS \
                --target=arm-linux-gnueabihf" \
            ;; \
        arm64) \
            BINUTILS_CONFIGURE_ARGS="$BINUTILS_CONFIGURE_ARGS \
                --target=aarch64-linux-gnu" \
            ;; \
    esac; \
    ../binutils-${BINUTILS_VERSION}/configure ${BINUTILS_CONFIGURE_ARGS}; \
    make; \
    make install; \
    cd ${PREFIX}/src; \
    curl -O https://ftp.fu-berlin.de/unix/languages/gcc/releases/gcc-${GCC_VERSION}/gcc-${GCC_VERSION}.tar.gz; \
    tar zxf gcc-${GCC_VERSION}.tar.gz; \
    rm gcc-${GCC_VERSION}.tar.gz; \
    chown -R root:root gcc-${GCC_VERSION}; \
    chmod -R o-w,g+w gcc-${GCC_VERSION}; \
    mkdir ${PREFIX}/src/build-gcc; \
    cd ${PREFIX}/src/build-gcc; \
    GCC_CONFIGURE_ARGS="\
        --prefix=${PREFIX} \
        --with-glibc-version=${GLIB_VERSION} \
        --without-headers \
        --disable-nls \
        --disable-shared \
        --disable-multilib \
        --disable-decimal-float \
        --disable-libgomp \
        --disable-libquadmath \
        --disable-libssp \
        --disable-libvtv \
        --disable-host-shared \
        --with-boot-ldflags=-static \
        --with-stage1-ldflags=-static \
        --enable-languages=c,c++,go"; \
    case ${UK_ARCH} in \
        x86_64) \
            GCC_PREFIX="x86_64-linux-gnu" \
            ;; \
        arm) \
            GCC_PREFIX="arm-linux-gnueabihf" \
            ;; \
        arm64) \
            GCC_PREFIX="aarch64-linux-gnu" \
            ;; \
    esac; \
    GCC_CONFIGURE_ARGS="$GCC_CONFIGURE_ARGS --target=${GCC_PREFIX}"; \
    ../gcc-${GCC_VERSION}/configure ${GCC_CONFIGURE_ARGS}; \
    make -j${MAKE_NPROC} all-gcc; \
    make install-gcc; \
    make all-target-libgcc; \
    make install-target-libgcc; \
    rm -rf ${PREFIX}/src;

FROM scratch AS gcc

COPY --from=gcc-build /out/bin/ /bin/
COPY --from=gcc-build /out/include/ /include/
COPY --from=gcc-build /out/lib/ /lib/
COPY --from=gcc-build /out/libexec/ /libexec/
COPY --from=gcc-build /out/${GCC_PREFIX} /${GCC_PREFIX}
