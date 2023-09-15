# libmocktainer

A stripped down version of [libcontainer][1], with the bare minimum
functionalities preserved for running Linux VMMs (QEMU, Firecracker) while
remaining compliant with the [OCI runtime][2] flow.

Its main purpose is to back Unikraft's [`runu`](../cmd/runu) OCI runtime CLI.

## Maintenance

To facilitate the maintenance of this library, please be mindful and keep it as
close as possible to the upstream libcontainer code.

## Copyright and license

The source code of libcontainer is distributed under the terms of the [Apache
2.0 license][3], copyright 2014 Docker, inc.

[1]: https://github.com/opencontainers/runc/tree/1f25724a/libcontainer#readme
[2]: https://github.com/opencontainers/runtime-spec/blob/v1.1.0/runtime.md
[3]: https://github.com/opencontainers/runc/blob/1f25724a/LICENSE
