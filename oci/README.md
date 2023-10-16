# Unikraft OCI Images 

This package contains the implementation for handling both the packaging and distribution of Unikraft unikernels via the [Open Container Image (OCI)'s Image Specification](https://github.com/opencontainers/image-spec/blob/main/spec.md).

At a high-level, this is done in order to ease distribution of pre-built unikernel images using existing infrastructure.
More precisely, the specification is adopted with several well-known annotations and practices to ensure consistency and compatibility with Unikraft unikernel images which are packed into an OCI image format.

## Overview

The OCI Image Specification allows for the declaration of artifacts in a hierarchical fashion.
The top-most element is an index which contains a list of manifests.
Each manifest in KraftKit's case is a pre-built unikernel image along with accompanying root filesystem.

### Supported backends

There are currently two supported backends for using and manipulating OCI images in KraftKit:

- `directory` (default) handles the representation of the OCI Image in directory format;
- `containerd` which requires configuration in the [KraftKit config file](https://unikraft.org/docs/cli/options) and uses containerd's content storage system.

### General usage

```
kraft pkg --name helloworld:latest --with-kconfig --plat qemu
```

In the directory implementation, the representation results in the following artifacts on the host system:

```
/root/.local/share/kraftkit/runtime/oci
├── configs
│   └── sha256
│       ├── 9530779d911fc36a50e156d36379d2e4d59eb259f28fba2311a80845a11988ec.json
│       └── fe5f6abe40068df5d5e98a08dc2f697faa5d22c064ac890e4f1a160d2999c6c4.json
├── indexes
│   └── helloworld
│       └── latest.json
├── layers
│   └── sha256
│       ├── 784e290aec5e97bf1e9ba17759fb0df3188a7285b08fccc84f882be4f2f65064
│       └── 989eddd15537fc00d20d560ae35c435b59b96c421412d75a85f22b7331a78aff
└── manifests
    └── sha256
        ├── 656072e6e2bb60be3bd439460058c3240e29aa83e718e42fd213789d2bbfa0f0.json
        └── 865cdfc985c9818b31b3b8094e405cd7e2d96fca4c19f07f38ef75a9077679d7.json
```

For containerd it results the following objects within the content store:

```
ctr content ls
DIGEST                                                                  SIZE    AGE             LABELS
sha256:089d3e4f1c08951c5bc7b0a41ab9fca5445ac06dea642679d243db00a1948f0d 2.627kB 1 second        containerd.io/distribution.source.index.docker.io=library/helloworld,containerd.io/gc.ref.content.l.0=sha256:989eddd15537fc00d20d560ae35c435b59b96c421412d75a85f22b7331a78aff,containerd.io/gc.ref.content.l.1=sha256:9530779d911fc36a50e156d36379d2e4d59eb259f28fba2311a80845a11988ec,containerd.io/gc.root=true,kraftkit.sh/oci.mediaType=application/vnd.oci.image.manifest.v1+json
sha256:784e290aec5e97bf1e9ba17759fb0df3188a7285b08fccc84f882be4f2f65064 217.1kB 1 second        containerd.io/gc.root=true,kraftkit.sh/oci.mediaType=application/vnd.oci.image.layer.v1.tar
sha256:9530779d911fc36a50e156d36379d2e4d59eb259f28fba2311a80845a11988ec 1.96kB  1 second        containerd.io/gc.root=true,kraftkit.sh/oci.mediaType=application/vnd.oci.image.config.v1+json
sha256:989eddd15537fc00d20d560ae35c435b59b96c421412d75a85f22b7331a78aff 241.7kB 1 second        containerd.io/gc.root=true,kraftkit.sh/oci.mediaType=application/vnd.oci.image.layer.v1.tar
sha256:ce3ff4f9a957ab52f8187d03d8e45db4cbb35101ea4e65ff0f3f08b0ebbda111 4.773kB 1 second        containerd.io/distribution.source.index.docker.io=library/helloworld,containerd.io/gc.ref.content.m.0=sha256:f63c483c81fd38dfc3c9088f7decaaec488cea3ff09648a5373ce6efb0c0234c,containerd.io/gc.ref.content.m.1=sha256:089d3e4f1c08951c5bc7b0a41ab9fca5445ac06dea642679d243db00a1948f0d,containerd.io/gc.root=true,kraftkit.sh/oci.mediaType=application/vnd.oci.image.index.v1+json
sha256:f63c483c81fd38dfc3c9088f7decaaec488cea3ff09648a5373ce6efb0c0234c 2.455kB 1 second        containerd.io/distribution.source.index.docker.io=library/helloworld,containerd.io/gc.ref.content.l.0=sha256:784e290aec5e97bf1e9ba17759fb0df3188a7285b08fccc84f882be4f2f65064,containerd.io/gc.ref.content.l.1=sha256:fe5f6abe40068df5d5e98a08dc2f697faa5d22c064ac890e4f1a160d2999c6c4,containerd.io/gc.root=true,kraftkit.sh/oci.mediaType=application/vnd.oci.image.manifest.v1+json
sha256:fe5f6abe40068df5d5e98a08dc2f697faa5d22c064ac890e4f1a160d2999c6c4 1.788kB 1 second        containerd.io/gc.root=true,kraftkit.sh/oci.mediaType=application/vnd.oci.image.config.v1+json
```
