// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package hyperlight

import "strconv"

// HyperlightConfig represents configuration for a Hyperlight micro-VM.
type HyperlightConfig struct {
	// KernelPath is the path to the unikernel binary.
	KernelPath string `json:"kernelPath,omitempty"`

	// Memory is the amount of memory allocated to the VM (e.g. "512Mi").
	Memory string `json:"memory,omitempty"`

	// Stack is the stack size (e.g. "8Mi").
	Stack string `json:"stack,omitempty"`

	// Quiet suppresses hyperlight-unikraft host-side status messages.
	Quiet bool `json:"quiet,omitempty"`

	// EnableTools enables tool dispatch via the __dispatch host function.
	EnableTools bool `json:"enableTools,omitempty"`

	// Repeat runs the application N additional times.
	Repeat int `json:"repeat,omitempty"`

	// Exec is an inline code snippet for the guest interpreter.
	Exec string `json:"exec,omitempty"`

	// InitRd is the path to the initramfs/rootfs CPIO archive.
	InitRd string `json:"initrd,omitempty"`

	// Mounts are host directory preopens in HOST:GUEST form.
	Mounts []string `json:"mounts,omitempty"`

	// LogPath is the path to the log file.
	LogPath string `json:"logPath,omitempty"`
}

func (hlcfg *HyperlightConfig) MarshalArgs(appArgs []string) []string {
	args := []string{
		"--memory", hlcfg.Memory,
		"--stack", hlcfg.Stack,
	}
	if hlcfg.Quiet {
		args = append(args, "--quiet")
	}
	if hlcfg.EnableTools {
		args = append(args, "--enable-tools")
	}
	if hlcfg.Repeat > 0 {
		args = append(args, "--repeat", strconv.Itoa(hlcfg.Repeat))
	}
	if hlcfg.InitRd != "" {
		args = append(args, "--initrd", hlcfg.InitRd)
	}
	for _, mount := range hlcfg.Mounts {
		args = append(args, "--mount", mount)
	}
	if hlcfg.Exec != "" {
		args = append(args, "--exec", hlcfg.Exec)
	}
	args = append(args, hlcfg.KernelPath)

	if hlcfg.Exec == "" && len(appArgs) > 0 {
		args = append(args, "--")
		args = append(args, appArgs...)
	}

	return args
}

type HyperlightOption func(*HyperlightConfig) error

func NewHyperlightConfig(opts ...HyperlightOption) (*HyperlightConfig, error) {
	cfg := &HyperlightConfig{
		Memory: DefaultMemory,
		Stack:  DefaultStack,
	}

	for _, o := range opts {
		if err := o(cfg); err != nil {
			return nil, err
		}
	}

	return cfg, nil
}

func WithKernel(kernel string) HyperlightOption {
	return func(c *HyperlightConfig) error {
		c.KernelPath = kernel
		return nil
	}
}

func WithMemory(memory string) HyperlightOption {
	return func(c *HyperlightConfig) error {
		c.Memory = memory
		return nil
	}
}

func WithInitRd(initrd string) HyperlightOption {
	return func(c *HyperlightConfig) error {
		c.InitRd = initrd
		return nil
	}
}

func WithStack(stack string) HyperlightOption {
	return func(c *HyperlightConfig) error {
		c.Stack = stack
		return nil
	}
}

func WithMounts(mounts ...string) HyperlightOption {
	return func(c *HyperlightConfig) error {
		c.Mounts = mounts
		return nil
	}
}

func WithQuiet(quiet bool) HyperlightOption {
	return func(c *HyperlightConfig) error {
		c.Quiet = quiet
		return nil
	}
}

func WithEnableTools(enableTools bool) HyperlightOption {
	return func(c *HyperlightConfig) error {
		c.EnableTools = enableTools
		return nil
	}
}

func WithRepeat(repeat int) HyperlightOption {
	return func(c *HyperlightConfig) error {
		c.Repeat = repeat
		return nil
	}
}

func WithExec(code string) HyperlightOption {
	return func(c *HyperlightConfig) error {
		c.Exec = code
		return nil
	}
}
