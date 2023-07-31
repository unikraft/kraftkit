# SPDX-License-Identifier: BSD-3-Clause
# Copyright (c) 2022, NEC Europe Ltd., and The KraftKit Authors.
# Licensed under the BSD-3-Clause License (the "License").
# You may not use this file except in compliance with the License.

ARG DEBIAN_VERSION=bullseye-20221114

FROM debian:${DEBIAN_VERSION} AS qemu-build

ARG QEMU_VERSION=7.2.4
ARG WITH_XEN=disable
ARG WITH_KVM=enable

ARG WITH_x86_64=enable
ARG WITH_aarch64=enable
ARG WITH_arm=enable

ARG MAKE_NPROC=1

WORKDIR /out

# Install dependencies
RUN set -ex; \
    apt-get -y update; \
    apt-get install -y \
        bison \
        build-essential \
        curl \
        flex \
        libaio-dev \
        libattr1-dev \
        libcap-dev \
        libcap-ng-dev \
        libglib2.0-dev \
        liblzo2-dev \
        libpixman-1-dev \
        ninja-build \
        pkg-config \
        python \
        texinfo \
        vde2 \
        xz-utils \
        zlib1g-dev; \
    apt-get clean;

# Download and extract QEMU
RUN set -ex; \
    curl -O https://download.qemu.org/qemu-${QEMU_VERSION}.tar.xz; \
    tar xf qemu-${QEMU_VERSION}.tar.xz; \
    apt-get install -y;

# Configure and build QEMU
RUN set -ex; \
    cd qemu-${QEMU_VERSION}; \
    tlist=""; \
    if [ "${WITH_x86_64}" = "enable" ]; then \
        tlist=",x86_64-softmmu"; \
    fi; \
    if [ "${WITH_aarch64}" = "enable" ]; then \
        tlist="${tlist},aarch64-softmmu"; \
    fi; \
    if [ "${WITH_arm}" = "enable" ]; then \
        tlist="${tlist},arm-softmmu"; \
    fi; \
    ./configure \
        --target-list=$(echo ${tlist} | tail -c +2) \
        --static \
        --prefix=/ \
        --audio-drv-list="" \
        --enable-attr \
        --disable-auth-pam \
        --disable-avx2 \
        --disable-avx512f \
        --disable-bochs \
        --disable-bpf \
        --disable-brlapi \
        --disable-bsd-user \
        --disable-bzip2 \
        --disable-canokey \
        --disable-capstone \
        --disable-cfi \
        --disable-cfi-debug \
        --disable-cloop \
        --disable-cocoa \
        --disable-coreaudio \
        --disable-crypto-afalg \
        --disable-curl \
        --disable-curses \
        --disable-dbus-display \
        --disable-dmg \
        --disable-docs \
        --disable-dsound \
        --disable-fuse \
        --disable-fuse-lseek \
        --disable-gcov \
        --disable-gcrypt \
        --disable-gettext \
        --disable-gio \
        --disable-glusterfs \
        --disable-gnutls \
        --disable-gprof \
        --disable-gtk \
        --disable-guest-agent \
        --disable-guest-agent-msi \
        --disable-hax \
        --disable-hvf \
        --disable-iconv \
        --disable-jack \
        --disable-keyring \
        --${WITH_KVM}-kvm \
        --disable-l2tpv3 \
        --disable-libdaxctl \
        --disable-libiscsi \
        --disable-libnfs \
        --disable-libpmem \
        --disable-libssh \
        --disable-libudev \
        --disable-libusb \
        --disable-libvduse \
        --disable-linux-aio \
        --disable-linux-io-uring \
        --enable-linux-user \
        --disable-live-block-migration \
        --disable-lzfse \
        --enable-lzo \
        --disable-malloc-trim \
        --disable-membarrier \
        --disable-modules \
        --disable-mpath \
        --disable-multiprocess \
        --disable-netmap \
        --disable-nettle \
        --disable-numa \
        --disable-nvmm \
        --disable-opengl \
        --disable-oss \
        --disable-pa \
        --disable-parallels \
        --disable-pie \
        --disable-png \
        --disable-profiler \
        --disable-pvrdma \
        --disable-qcow1 \
        --disable-qed \
        --disable-qga-vss \
        --disable-rbd \
        --disable-rdma \
        --disable-replication \
        --disable-safe-stack \
        --disable-sdl \
        --disable-sdl-image \
        --disable-seccomp \
        --disable-selinux \
        --disable-slirp-smbd \
        --disable-smartcard \
        --disable-snappy \
        --disable-sparse \
        --disable-spice \
        --disable-spice-protocol \
        --enable-system \
        --enable-tcg \
        --enable-tools \
        --disable-tpm \
        --disable-u2f \
        --disable-usb-redir \
        --enable-user \
        --disable-vde \
        --disable-vdi \
        --disable-vduse-blk-export \
        --disable-vfio-user-server \
        --enable-vhost-crypto \
        --enable-vhost-kernel \
        --enable-vhost-net \
        --enable-vhost-user \
        --enable-vhost-user-blk-server \
        --enable-vhost-vdpa \
        --disable-virglrenderer \
        --disable-virtiofsd \
        --disable-vmnet \
        --disable-vnc \
        --disable-vnc-jpeg \
        --disable-vnc-sasl \
        --disable-vte \
        --disable-vvfat \
        --disable-werror \
        --disable-whpx \
        --${WITH_XEN}-xen \
        --${WITH_XEN}-xen-pci-passthrough \
        --disable-xkbcommon \
        --disable-zstd \
        --enable-virtfs \
        ; \
        make -j${MAKE_NPROC}; \
        make install;

FROM scratch AS qemu
COPY --from=qemu-build /bin/qemu-img \
                       /bin/qemu-io \
                       /bin/qemu-nbd \
                       /bin/qemu-pr-helper \
                       /bin/qemu-system-aarch64 \
                       /bin/qemu-system-arm \
                       /bin/qemu-system-x86_64 \
                       /bin/

COPY --from=qemu-build /share/qemu/ /share/qemu/
COPY --from=qemu-build /lib/x86_64-linux-gnu/ /lib/x86_64-linux-gnu/
