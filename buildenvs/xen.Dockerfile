# SPDX-License-Identifier: BSD-3-Clause
# Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
# Licensed under the BSD-3-Clause License (the "License").
# You may not use this file except in compliance with the License.

ARG DEBIAN_VERSION=bookworm-20240513

FROM debian:${DEBIAN_VERSION} AS xenbuild

ARG XEN_VERSION=4.18
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
        libglib2.0-dev \
		liblzo2-dev \
		liblz-dev \
		liblzma-dev \
		libnl-3-dev \
		libnl-route-3-dev \
        libncurses5-dev \
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
        uuid-dev; \
	pip3 install python-config --break-system-packages; \
    git clone -b stable-${XEN_VERSION} https://xenbits.xen.org/git-http/xen.git /xen; \
	sed '/xs.opic: CFLAGS += -DUSE_PTHREAD/a xs.o: CFLAGS += -DUSE_PTHREAD' /xen/tools/libs/store/Makefile; \
    cd /xen; \
    ./configure --enable-virtfs; \
    make -j ${MAKE_NPROC} build-tools; \
    make -j ${MAKE_NPROC} install-tools; \
	cp  /usr/lib/x86_64-linux-gnu/libyajl_s.a /usr/lib/x86_64-linux-gnu/libyajl.a

FROM scratch AS xen

COPY --from=xenbuild /usr/lib/x86_64-linux-gnu/liblzma.a \
				/usr/lib/x86_64-linux-gnu/libbz2.a \
				/usr/lib/x86_64-linux-gnu/libzstd.a \
				/usr/lib/x86_64-linux-gnu/liblzo2.a \
				/usr/lib/x86_64-linux-gnu/libyajl.a \
				/usr/lib/x86_64-linux-gnu/libz.a \
				/usr/lib/x86_64-linux-gnu/libnl-route-3.a \
				/usr/lib/x86_64-linux-gnu/libnl-3.a \
				/usr/lib/x86_64-linux-gnu/libuuid.a \
				/usr/lib/x86_64-linux-gnu/libutil.a \
				/usr/lib/x86_64-linux-gnu/
COPY --from=xenbuild /usr/local/lib/libxen*.a /usr/local/lib/
COPY --from=xenbuild /usr/local/lib/libxen*.so* /usr/local/lib/
COPY --from=xenbuild /usr/local/include/*.h /usr/local/include/
