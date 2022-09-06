# SPDX-License-Identifier: BSD-3-Clause
#
# Authors: Alexander Jung <alexander.jung@neclab.eu>
#
# Copyright (c) 2020, NEC Europe Ltd., NEC Corporation. All rights reserved.
#
# Redistribution and use in source and binary forms, with or without
# modification, are permitted provided that the following conditions
# are met:
#
# 1. Redistributions of source code must retain the above copyright
#    notice, this list of conditions and the following disclaimer.
# 2. Redistributions in binary form must reproduce the above copyright
#    notice, this list of conditions and the following disclaimer in the
#    documentation and/or other materials provided with the distribution.
# 3. Neither the name of the copyright holder nor the names of its
#    contributors may be used to endorse or promote products derived from
#    this software without specific prior written permission.
#
# THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
# AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
# IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
# ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE
# LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR
# CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF
# SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
# INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN
# CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
# ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
# POSSIBILITY OF SUCH DAMAGE.

ARG DEBIAN_VERSION=stretch-20200224

FROM debian:${DEBIAN_VERSION} AS gcc-build

ARG BINUTILS_VERSION=2.31.1
ARG GCC_VERSION=9.2.0
ARG UK_ARCH=x86_64
ARG GLIB_VERSION=2.11
ENV PREFIX=/out

RUN set -ex; \
    apt-get update; \
    apt-get install -y \
        wget \
        curl \
        gcc \
        libgmp3-dev \
        libmpfr-dev \
        libisl-dev \
        libcloog-isl-dev \
        libmpc-dev \
        texinfo \
        bison \
        flex \
        make \
        bzip2 \
        patch \
        file \
        build-essential; \
    mkdir -p ${PREFIX}/src; \
    cd ${PREFIX}/src; \
    curl -O https://ftp.gnu.org/gnu/binutils/binutils-${BINUTILS_VERSION}.tar.gz; \
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
    curl -O https://ftp.gnu.org/gnu/gcc/gcc-${GCC_VERSION}/gcc-${GCC_VERSION}.tar.gz; \
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
        --disable-threads \
        --disable-libatomic \
        --disable-libgomp \
        --disable-libmpx \
        --disable-libquadmath \
        --disable-libssp \
        --disable-libvtv \
        --disable-libstdcxx \
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
    make all-gcc; \
    make install-gcc; \
    make all-target-libgcc; \
    make install-target-libgcc;

FROM scratch AS gcc

COPY --from=gcc-build /out/bin/ /bin/
COPY --from=gcc-build /out/include/ /include/
COPY --from=gcc-build /out/lib/ /lib/
COPY --from=gcc-build /out/libexec/ /libexec/
COPY --from=gcc-build /out/${GCC_PREFIX} /${GCC_PREFIX}
