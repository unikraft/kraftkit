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
- `DefaultStack`: `8Mi`. Not yet exposed through the CLI or Kraftfile.

### Limitations

- `kraft pause` is not supported; Hyperlight has no pause semantics.
- The child process is terminated on `kraft stop` via SIGTERM; there is no
  in-VM quiesce step.
