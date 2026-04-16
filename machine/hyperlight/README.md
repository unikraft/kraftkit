## Hyperlight Machine Driver

A kraftkit machine driver that runs Unikraft unikernels on
[Hyperlight](https://github.com/hyperlight-dev/hyperlight) micro-VMs. Each
machine created through this driver is a child `hyperlight-unikraft` process,
so `kraft ps`, `kraft stop`, `kraft rm`, and `kraft logs` all work across
separate kraft invocations via standard PID tracking.

### Requirements

- Linux host with `/dev/kvm` read/write access.
- The `hyperlight-unikraft` binary (from
  [hyperlight-unikraft](https://github.com/hyperlight-dev/hyperlight-unikraft))
  installed on `$PATH`:

  ```bash
  cd hyperlight-unikraft/host
  cargo build --release
  sudo cp target/release/hyperlight-unikraft /usr/local/bin/
  ```

- Unikraft kernels built for the `hyperlight` platform target.

No cgo, no linker flags, no shared libraries — kraftkit just needs to find
`hyperlight-unikraft` in `$PATH` at runtime.

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

- `DefaultMemory`: `16Mi`. Override with `--memory` for heavier guests.
- `DefaultStack`: `8Mi`. Not yet exposed through the CLI or Kraftfile.

### Limitations

- `kraft pause` is not supported; Hyperlight has no pause semantics.
- The child process is terminated on `kraft stop` via SIGTERM; there is no
  in-VM quiesce step.
