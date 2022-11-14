# KraftKit 🚀🐒🧰

[![](https://pkg.go.dev/badge/kraftkit.sh.svg)](https://pkg.go.dev/kraftkit.sh)
![](https://img.shields.io/static/v1?label=license&message=BSD-3&color=%23385177)
[![](https://img.shields.io/discord/762976922531528725.svg?label=discord&logo=discord&logoColor=ffffff&color=7389D8&labelColor=6A7EC2)][unikraft-discord]
[![Go Report Card](https://goreportcard.com/badge/kraftkit.sh)](https://goreportcard.com/report/kraftkit.sh)

Kraftkit is a suite of tools and framework for building custom, minimal, immutable lightweight virtual machines based on [Unikraft](https://unikraft.org).
With Kraftkitt, you can use unikernels at every of their lifecycle: from construction to production.

You can quickly and easily install KraftKit using the interactive installer.  Simply run the following command to get started: 

```shell
curl --proto '=https' --tlsv1.2 -sSf https://get.kraftkit.sh | sh
```

Alternatively you can download the binaries from the [releases pages](https://github.com/unikraft/kraftkit/releases).

## Quick-start

Building a unikernel with KraftKit is designed to be simple, simply add a `Kraftfile` to your project directory, which specifies the libraries needed for your unikernel:

```yaml
specification: v0.5

unikraft: stable

libraries:
  newlib: stable

targets:
  - name: default
    architecture: x86_64
    platform: kvm
```

You can also add an additional `Makefile.uk` which specifies any source files:

```Makefile
$(eval $(call addlib,apphelloworld))

APPHELLOWORLD_SRCS-y += $(APPHELLOWORLD_BASE)/main.c
```

Then it is a case of running:

```shell
cd path/to/workdir

kraft pkg update
kraft build
```

You can run your unikernel using:

```shell
kraft run
```

## License

KraftKit is part of the [Unikraft OSS Project][unikraft-website] and licensed under `BSD-3-Clause`.

[unikraft-website]: https://unikraft.org
[unikraft-docs]: https://unikraft.org/docs
[unikraft-discord]: https://bit.ly/UnikraftDiscord
