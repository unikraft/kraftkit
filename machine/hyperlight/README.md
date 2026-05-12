## Hyperlight Machine Driver

A kraftkit machine driver that runs Unikraft unikernels on
[Hyperlight](https://github.com/hyperlight-dev/hyperlight) micro-VMs. Each
machine created through this driver is a detached `hyperlight-unikraft` child
process, so `kraft ps`, `kraft stop`, `kraft rm`, and `kraft logs` all work
across separate kraft invocations via standard PID tracking.

### Requirements

- Linux host with `/dev/kvm` read/write access, or Windows host with the
  Windows Hypervisor Platform (WHP) enabled.
- The `hyperlight-unikraft` binary (from
  [danbugs/hyperlight-unikraft](https://github.com/danbugs/hyperlight-unikraft))
  installed on `$PATH`:

  ```bash
  cargo install --git https://github.com/danbugs/hyperlight-unikraft \
      --branch main hyperlight-unikraft-host --bin hyperlight-unikraft
  ```

- Unikraft kernels built for the `hyperlight` platform target.

No cgo, no linker flags, no shared libraries — kraftkit just needs to find
`hyperlight-unikraft` on `$PATH` at runtime. A future iteration may link the
Hyperlight host library in-process via cgo for tighter lifecycle control and
lower per-call overhead; the subprocess model is a deliberate first step.

### Platform selection

The driver registers two names:

- `hyperlight` (canonical)
- `hl` (shorthand)

Either can be used with `--plat`:

```bash
kraft build --plat hyperlight --arch x86_64
kraft run   --plat hl ...
```

### Defaults

- `DefaultMemory`: `32Mi`, matching the `hyperlight-unikraft` default.
  Override with `--memory` for guests that need a different allocation.
- `DefaultStack`: `8Mi`. Override with `--hyperlight-stack`.

### Supported run options

The driver maps the common KraftKit run surface that Hyperlight can execute:

- `--memory` is passed to `hyperlight-unikraft --memory`.
- `--rootfs`/`--initrd` CPIO archives are passed as
  `hyperlight-unikraft --initrd`.
- Application arguments after `--` are passed after the kernel path.
- Writable `9pfs` directory volumes are passed as repeatable
  `hyperlight-unikraft --mount HOST:GUEST` entries.

Hyperlight-specific host options are exposed with a `--hyperlight-*` prefix on
`kraft run` and are ignored by other platform drivers:

- `--hyperlight-stack`
- `--hyperlight-quiet`
- `--hyperlight-enable-tools`
- `--hyperlight-repeat`
- `--hyperlight-mount`
- `--hyperlight-exec`

Dockerfile and OCI rootfs metadata can contain default environment variables.
`hyperlight-unikraft` does not currently expose a runtime environment-injection
interface. If the guest needs environment variables, compile them into the
unikernel configuration or application.

### Limitations

- `kraft pause` is not supported; Hyperlight has no pause semantics.
- The child process is terminated on `kraft stop` via SIGTERM; there is no
  in-VM quiesce step.
- Runtime environment injection is not supported. Explicit `kraft run --env`
  and Kraftfile `env:` entries fail with a clear error.
- Network attachments, port publishing, emulation mode, and kernel arguments
  are rejected because `hyperlight-unikraft` does not support those KraftKit
  interfaces.
- Read-only volumes and non-`9pfs` volume drivers are rejected. Rootfs/initrd
  volumes are supported only when they represent the main initrd at `/`.
