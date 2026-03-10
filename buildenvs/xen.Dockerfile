# SPDX-License-Identifier: BSD-3-Clause
# Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
# Licensed under the BSD-3-Clause License (the "License").
# You may not use this file except in compliance with the License.

ARG DEBIAN_VERSION=bookworm-20240513

FROM debian:${DEBIAN_VERSION} AS xenbuild

ARG XEN_VERSION=4.19
ARG MAKE_NPROC=1

# The sed line should stay here until [1] is merged or forever if it's not
# [1]: https://lists.xenproject.org/archives/html/xen-devel/2024-07/msg00295.html

RUN set -xe; \
    apt-get update; \
    apt-get install -y \
        binutils \
        bison \
        build-essential \
        cmake \
        flex \
        gcc \
        git \
        iasl \
        libbz2-dev \
        libfdt-dev \
        libglib2.0-dev \
        liblz-dev \
        liblzma-dev \
        liblzo2-dev \
        libncurses5-dev \
        libnl-3-dev \
        libnl-route-3-dev \
        libpixman-1-dev \
        libslirp-dev \
        libssh2-1-dev \
        libssl-dev \
        libuuid1 \
        libyajl-dev \
        libz3-dev \
        libzstd-dev \
        make \
        ninja-build \
        perl \
        pkg-config \
        python3 \
        python3-pip \
        python3-setuptools \
        python3-wheel \
        uuid-dev \
    ; \
    pip3 install python-config --break-system-packages; \
    git clone -b stable-${XEN_VERSION} https://xenbits.xen.org/git-http/xen.git /xen; \
    sed -i '/xs.opic: CFLAGS += -DUSE_PTHREAD/a xs.o: CFLAGS += -DUSE_PTHREAD' /xen/tools/libs/store/Makefile; \
    cd /xen; \
    ./configure \
        --enable-virtfs \
    ; \
    make -j ${MAKE_NPROC} build-tools; \
    make -j ${MAKE_NPROC} install-tools; \
    ARCH_TRIPLET=$(dpkg-architecture -q DEB_HOST_MULTIARCH); \
    cp /usr/lib/${ARCH_TRIPLET}/libyajl_s.a /usr/lib/${ARCH_TRIPLET}/libyajl.a; \
    mkdir -p /out/libs/${ARCH_TRIPLET}; \
    cp /usr/lib/${ARCH_TRIPLET}/liblzma.a \
       /usr/lib/${ARCH_TRIPLET}/libbz2.a \
       /usr/lib/${ARCH_TRIPLET}/libzstd.a \
       /usr/lib/${ARCH_TRIPLET}/liblzo2.a \
       /usr/lib/${ARCH_TRIPLET}/libyajl.a \
       /usr/lib/${ARCH_TRIPLET}/libz.a \
       /usr/lib/${ARCH_TRIPLET}/libnl-route-3.a \
       /usr/lib/${ARCH_TRIPLET}/libnl-3.a \
       /usr/lib/${ARCH_TRIPLET}/libuuid.a \
       /usr/lib/${ARCH_TRIPLET}/libutil.a \
       /out/libs/${ARCH_TRIPLET}/

FROM scratch AS xen

COPY --from=xenbuild /out/libs/ /usr/lib/
COPY --from=xenbuild /usr/local/lib/libxen*.a /usr/local/lib/
COPY --from=xenbuild /usr/local/lib/libxen*.so* /usr/local/lib/
COPY --from=xenbuild /usr/local/include/*.h /usr/local/include/
