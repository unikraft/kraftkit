// SPDX-License-Identifier: Apache-2.0
// Copyright 2014 Docker, Inc.
// Copyright 2023 Unikraft GmbH and The KraftKit Authors

package libmocktainer

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/selinux/go-selinux"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"

	"github.com/opencontainers/runc/libcontainer/apparmor"
	"github.com/opencontainers/runc/libcontainer/keys"
	"github.com/opencontainers/runc/libcontainer/system"

	"kraftkit.sh/libmocktainer/configs"
	"kraftkit.sh/libmocktainer/unikraft"
)

type linuxStandardInit struct {
	pipe      *os.File
	parentPid int
	fifoFd    int
	logFd     int
	config    *initConfig
}

func (l *linuxStandardInit) getSessionRingParams() (string, uint32, uint32) {
	var newperms uint32 //nolint:gosimple

	// Without user ns we need 'UID' search permissions.
	newperms = 0x80000

	// Create a unique per session container name that we can join in setns;
	// However, other containers can also join it.
	return "_ses." + l.config.ContainerID, 0xffffffff, newperms
}

func (l *linuxStandardInit) Init() error {
	if err := selinux.SetKeyLabel(l.config.ProcessLabel); err != nil {
		return err
	}
	defer selinux.SetKeyLabel("") //nolint: errcheck
	ringname, keepperms, newperms := l.getSessionRingParams()

	// Do not inherit the parent's session keyring.
	if sessKeyId, err := keys.JoinSessionKeyring(ringname); err != nil {
		// If keyrings aren't supported then it is likely we are on an
		// older kernel (or inside an LXC container). While we could bail,
		// the security feature we are using here is best-effort (it only
		// really provides marginal protection since VFS credentials are
		// the only significant protection of keyrings).
		//
		// TODO(cyphar): Log this so people know what's going on, once we
		//               have proper logging in 'runc init'.
		if !errors.Is(err, unix.ENOSYS) {
			return fmt.Errorf("unable to join session keyring: %w", err)
		}
	} else {
		// Make session keyring searchable. If we've gotten this far we
		// bail on any error -- we don't want to have a keyring with bad
		// permissions.
		if err := keys.ModKeyringPerm(sessKeyId, keepperms, newperms); err != nil {
			return fmt.Errorf("unable to mod keyring permissions: %w", err)
		}
	}

	if err := setupNetwork(l.config); err != nil {
		return err
	}
	if err := setupRoute(l.config.Config); err != nil {
		return err
	}

	// initialises the labeling system
	selinux.GetEnabled()

	// We don't need the mount nor idmap fds after prepareRootfs() nor if it fails.
	err := prepareRootfs(l.pipe, l.config, mountFds{})
	if err != nil {
		return err
	}

	if err := apparmor.ApplyProfile(l.config.AppArmorProfile); err != nil {
		return fmt.Errorf("unable to apply apparmor profile: %w", err)
	}

	pdeath, err := system.GetParentDeathSignal()
	if err != nil {
		return fmt.Errorf("can't get pdeath signal: %w", err)
	}
	// Tell our parent that we're ready to Execv. This must be done before the
	// Seccomp rules have been applied, because we need to be able to read and
	// write to a socket.
	if err := syncParentReady(l.pipe); err != nil {
		return fmt.Errorf("sync ready: %w", err)
	}
	if err := selinux.SetExecLabel(l.config.ProcessLabel); err != nil {
		return fmt.Errorf("can't set process label: %w", err)
	}
	defer selinux.SetExecLabel("") //nolint: errcheck
	if err := finalizeNamespace(l.config); err != nil {
		return err
	}
	// finalizeNamespace can change user/group which clears the parent death
	// signal, so we restore it here.
	if err := pdeath.Restore(); err != nil {
		return fmt.Errorf("can't restore pdeath signal: %w", err)
	}
	// Compare the parent from the initial start of the init process and make
	// sure that it did not change.  if the parent changes that means it died
	// and we were reparented to something else so we should just kill ourself
	// and not cause problems for someone else.
	if unix.Getppid() != l.parentPid {
		return unix.Kill(unix.Getpid(), unix.SIGKILL)
	}
	// Check for the arg before waiting to make sure it exists and it is
	// returned as a create time error.
	name, err := exec.LookPath(l.config.Args[0])
	if err != nil {
		return err
	}

	// Close the pipe to signal that we have completed our init.
	logrus.Debugf("init: closing the pipe to signal completion")
	_ = l.pipe.Close()

	// Close the log pipe fd so the parent's ForwardLogs can exit.
	if err := unix.Close(l.logFd); err != nil {
		return &os.PathError{Op: "close log pipe", Path: "fd " + strconv.Itoa(l.logFd), Err: err}
	}

	// Wait for the FIFO to be opened on the other side before exec-ing the
	// user process. We open it through /proc/self/fd/$fd, because the fd that
	// was given to us was an O_PATH fd to the fifo itself. Linux allows us to
	// re-open an O_PATH fd through /proc.
	fifoPath := "/proc/self/fd/" + strconv.Itoa(l.fifoFd)
	fd, err := unix.Open(fifoPath, unix.O_WRONLY|unix.O_CLOEXEC, 0)
	if err != nil {
		return &os.PathError{Op: "open exec fifo", Path: fifoPath, Err: err}
	}
	if _, err := unix.Write(fd, []byte("0")); err != nil {
		return &os.PathError{Op: "write exec fifo", Path: fifoPath, Err: err}
	}

	// -- BEGIN Unikraft

	var isUnikernel bool
	for _, lbl := range l.config.Config.Labels {
		if lbl == "org.unikraft.kernel=" { // injected by `runu create`
			isUnikernel = true
			break
		}
	}

	if isUnikernel {
		// This must happen in the Start phase of the OCI startup flow, right
		// before exec(), because the setup of the container's network interfaces
		// typically happens between the Create and the Start phases (e.g. CNI).
		qemuNetArgs, err := unikraft.SetupQemuNet()
		if err != nil {
			return fmt.Errorf("setting up qemu network: %w", err)
		}
		l.config.Args = append(l.config.Args, qemuNetArgs...)
	}

	// -- END Unikraft

	// Close the O_PATH fifofd fd before exec because the kernel resets
	// dumpable in the wrong order. This has been fixed in newer kernels, but
	// we keep this to ensure CVE-2016-9962 doesn't re-emerge on older kernels.
	// N.B. the core issue itself (passing dirfds to the host filesystem) has
	// since been resolved.
	// https://github.com/torvalds/linux/blob/v4.9/fs/exec.c#L1290-L1318
	_ = unix.Close(l.fifoFd)

	s := l.config.SpecState
	s.Pid = unix.Getpid()
	s.Status = specs.StateCreated
	if err := l.config.Config.Hooks[configs.StartContainer].RunHooks(s); err != nil {
		return err
	}

	return system.Exec(name, l.config.Args[0:], os.Environ())
}
