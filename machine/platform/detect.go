// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package platform

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"kraftkit.sh/internal/set"
	"kraftkit.sh/machine/qemu"
)

type SystemMode string

const (
	SystemUnknown = SystemMode("unknown")
	SystemGuest   = SystemMode("guest")
	SystemHost    = SystemMode("host")
)

// getenv retrieves the environment variable key. If it does not exist it
// returns the default.
func getenv(key string, dfault string, combineWith ...string) string {
	value := os.Getenv(key)
	if value == "" {
		value = dfault
	}

	switch len(combineWith) {
	case 0:
		return value
	case 1:
		return filepath.Join(value, combineWith[0])
	default:
		all := make([]string, len(combineWith)+1)
		all[0] = value
		copy(all[1:], combineWith)
		return filepath.Join(all...)
	}
}

// hostProc returns the provided procfs path, using environmental variable to
// allow base path configuration.
func hostProc(path ...string) string {
	return getenv("HOST_PROC", "/proc", path...)
}

// hostProc returns the provided dev path, using environmental variable to
// allow base path configuration.
func hostDev(path ...string) string {
	return getenv("HOST_DEV", "/dev", path...)
}

// pathExists simply returns whether the provided file path exists.
func pathExists(file string) bool {
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		return true
	}
	return false
}

// readLines reads contents from a file and splits them by new lines.
// A convenience wrapper to readLinesOffsetN(file, 0, -1).
func readLines(file string) ([]string, error) {
	return readLinesOffsetN(file, 0, -1)
}

// readLinesOffsetN reads contents from file and splits them by new line.
// The offset tells at which line number to start.
// The count determines the number of lines to read (starting from offset):
// n >= 0: at most n lines
// n < 0: whole file
func readLinesOffsetN(file string, offset uint, n int) ([]string, error) {
	f, err := os.Open(file)
	if err != nil {
		return []string{""}, err
	}
	defer f.Close()

	var ret []string

	r := bufio.NewReader(f)
	for i := 0; i < n+int(offset) || n < 0; i++ {
		line, err := r.ReadString('\n')
		if err != nil {
			if err == io.EOF && len(line) > 0 {
				ret = append(ret, strings.Trim(line, "\n"))
			}
			break
		}
		if i < int(offset) {
			continue
		}
		ret = append(ret, strings.Trim(line, "\n"))
	}

	return ret, nil
}

// Detect returns the hypervisor and system mode in the context to the
// determined hypervisor or an error if not detectable.
func Detect(ctx context.Context) (Platform, SystemMode, error) {
	file := hostProc("xen")
	if pathExists(file) {
		system := PlatformXen
		role := SystemGuest // assume guest

		if pathExists(filepath.Join(file, "capabilities")) {
			contents, err := readLines(filepath.Join(file, "capabilities"))
			if err == nil {
				if set.NewStringSet(contents...).Contains("control_d") {
					role = SystemHost
				}
			}
		}

		return system, role, nil
	}

	file = hostProc("modules")
	if pathExists(file) {
		contents, err := readLines(file)
		if err == nil {
			if set.NewStringSet(contents...).Contains("kvm") {
				return PlatformKVM, SystemHost, nil
				// } else if set.NewStringSet(contents...).Contains("hv_util") {
				// 	return HypervisorHyperV, SystemGuest, nil
				// } else if set.NewStringSet(contents...).Contains("vboxdrv") {
				// 	return HypervisorVirtualBox, SystemHost, nil
				// } else if set.NewStringSet(contents...).Contains("vboxguest") {
				// 	return HypervisorVirtualBox, SystemGuest, nil
				// } else if set.NewStringSet(contents...).Contains("vmware") {
				// 	return HypervisorVMwareESX, SystemGuest, nil
			}
		}
	}

	file = hostProc("cpuinfo")
	if pathExists(file) {
		contents, err := readLines(file)
		if err == nil {
			if set.NewStringSet(contents...).Contains("QEMU Virtual CPU") {
				return PlatformQEMU, SystemGuest, nil
			} else if set.NewStringSet(contents...).Contains("Common KVM processor") {
				return PlatformKVM, SystemGuest, nil
			} else if set.NewStringSet(contents...).Contains("Common 32-bit KVM processor") {
				return PlatformKVM, SystemGuest, nil
			}
		}
	}

	file = hostDev("kvm")
	if pathExists(file) {
		if kvmFile, err := os.Stat(file); err == nil &&
			kvmFile.Mode()&os.ModeCharDevice != 0 {
			// Send ioctl for KVM_GET_API_VERSION
			file, err := os.Open(file)
			if err != nil {
				return PlatformUnknown, SystemUnknown, fmt.Errorf("could not open kvm device: %w", err)
			}
			defer file.Close()

			// 0xAE is the magic number for KVM
			kvmctl := uintptr(0xAE << 8)
			version, _, err := syscall.Syscall(syscall.SYS_IOCTL, file.Fd(), kvmctl, 0)
			if err != syscall.Errno(0) {
				return PlatformUnknown, SystemUnknown, fmt.Errorf("could not send ioctl to kvm device: %w", err)
			}

			// 12 is the current version of KVM_GET_API_VERSION
			// specification says to error if the version is not 12
			if version != 12 {
				return PlatformUnknown, SystemUnknown, fmt.Errorf("kvm version too old, or malformed, should be 12, but is %d", version)
			}

			return PlatformKVM, SystemHost, nil
		} else {
			return PlatformUnknown, SystemUnknown, fmt.Errorf("kvm exists but is not a character device")
		}
	}

	// Check if any QEMU binaries are installed on the host.  Since we could not
	// determine if virtualization extensions are possible at this point, at least
	// guest emulation is possible with QEMU.
	for _, bin := range []string{
		qemu.QemuSystemX86,
		qemu.QemuSystemArm,
		qemu.QemuSystemAarch64,
	} {
		if _, err := exec.LookPath(bin); err != nil {
			continue
		}

		return PlatformQEMU, SystemGuest, nil
	}

	return PlatformUnknown, SystemUnknown, fmt.Errorf("could not determine hypervisor and system mode")
}
