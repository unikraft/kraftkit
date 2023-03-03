// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package setup

import (
	"context"

	"github.com/shirou/gopsutil/v3/host"
)

type SetupOption func(*Setup) error

type HostPackageManager interface {
	Install(context.Context, ...string) error
}

// Check if the provided `os` variable is valid in format and supported.
func WithOS(os string) SetupOption {
	switch os {
	case "ubuntu", "linux/ubuntu":
		return NotSupported
	case "windows":
		return NotSupported
	}

	return NotSupported
}

// Check if the provided `arch` variable is valid in format and supported.
func WithArch(arch string) SetupOption {
	switch arch {
	case "x86_64":
		return NotSupported
	case "arm64":
		return NotSupported
	}

	return NotSupported
}

// Check if the provided `vmm` variable is valid in format and supported.
func WithVMM(vmm string) SetupOption {
	switch vmm {
	case "kvm":
		return NotSupported
	case "xen":
		return NotSupported
	}

	return NotSupported
}

// Check if the provided `pm` variable is valid in format and supported.
func WithPM(pm string) SetupOption {
	return NotSupported
}

func WithDetectHostOS() SetupOption {
	os_name := detect_os()
	return WithOS(os_name)
}

func WithDetectArch() SetupOption {
	arch := detect_arch()
	return WithArch(arch)
}

func WithDetectVMM() SetupOption {
	vmm := detect_vmm()
	return WithVMM(vmm)
}

func WithDetectPM() SetupOption {
	return NotSupported
}

func detect_os() string {
	os_info, _ := host.Info()
	os_name := os_info.OS + "/" + os_info.Platform

	return os_name
}

func detect_arch() string {
	os_info, _ := host.Info()
	arch := os_info.KernelArch

	return arch
}

func detect_vmm() string {
	os_virt, role, _ := host.Virtualization()
	vmm := os_virt + "/" + role

	return vmm
}

func NotSupported(*Setup) error {
	return nil
}
