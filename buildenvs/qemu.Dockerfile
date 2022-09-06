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

ARG DEBIAN_VERSION=stretch-20191224

FROM debian:${DEBIAN_VERSION} AS qemu-build

ARG QEMU_VERSION=4.2.0
WORKDIR /out

RUN set -ex; \
    apt-get -y update; \
    apt-get -y upgrade; \
    apt-get install -y \
        build-essential \
        curl \
        libaio-dev \
        libcap-dev \
        libcap-ng-dev \
        libglib2.0-dev \
        liblzo2-dev \
        libpixman-1-dev \
        pkg-config \
        flex \
        bison \
        python \
        texinfo \
        vde2 \
        zlib1g-dev \
        xz-utils; \
    curl -O https://download.qemu.org/qemu-${QEMU_VERSION}.tar.xz; \
    tar xf qemu-${QEMU_VERSION}.tar.xz; \
    apt-get install -y; \
    cd qemu-${QEMU_VERSION}; \
    ./configure \
        --prefix=/ \
        --static \
        --python=/usr/bin/python2 \
        --audio-drv-list="" \
        --disable-docs \
        --disable-debug-info \
        --disable-opengl \
        --disable-virglrenderer \
        --disable-vte \
        --disable-gtk \
        --disable-sdl \
        --disable-bluez \
        --disable-spice \
        --disable-vnc \
        --disable-curses \
        --disable-smartcard \
        --disable-libnfs \
        --disable-libusb \
        --disable-glusterfs \
        --disable-werror \
        --target-list="x86_64-softmmu,i386-softmmu,aarch64-softmmu,arm-softmmu"; \
    make; \
    make install

FROM scratch AS qemu

COPY --from=qemu-build /bin/qemu-ga /bin/
COPY --from=qemu-build /bin/qemu-img /bin/
COPY --from=qemu-build /bin/qemu-io /bin/
COPY --from=qemu-build /bin/qemu-nbd /bin/
COPY --from=qemu-build /bin/qemu-pr-helper /bin/
COPY --from=qemu-build /bin/qemu-system-aarch64 /bin/
COPY --from=qemu-build /bin/qemu-system-arm /bin/
COPY --from=qemu-build /bin/qemu-system-i386 /bin/
COPY --from=qemu-build /bin/qemu-system-x86_64 /bin/
COPY --from=qemu-build /share/qemu/ /share/qemu/
COPY --from=qemu-build /lib/x86_64-linux-gnu/ /lib/x86_64-linux-gnu/
