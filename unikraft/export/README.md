# Unikraft C-Go bindings

This package contains C-bindings for Unikraft to be used within the context of
Go applications which are built on top of Unikraft.  The purpose of this package
is twofold:

1. to allow KraftKit internals to reference Unikraft internal structures during
   the manifestation of a unikernel (either during compile-time or runtime);
   and, 
2. to enable developers programming in Go to directly reference Unikraft
   internals and bypass general-purpose syscall boundaries.

This package is work-in-progress, and as such the enumerated libraries exposed
(or "exported") by this package are delivered through the `v0` suffix.  This
serves to indicate that the exported constants, variables, methods and utilities
offered by KraftKit representing Unikraft internals are incomplete, subject to
change and therefore considered unstable.  Thus, usage of the exported methods
are delivered via:

```go
import "kraftkit.sh/unikraft/export/v0"
```

Towards [the release of Unikraft v1.0
itself](https://github.com/orgs/unikraft/projects/24/views/38), the exported
libraries, symbols, methods and utility methods will reflect both stable APIs
both in terms of Unikraft's internals but also with regard to how this package
can be used.
