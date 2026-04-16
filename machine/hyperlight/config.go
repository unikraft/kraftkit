// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package hyperlight

// HyperlightConfig represents configuration for a Hyperlight micro-VM.
type HyperlightConfig struct {
	// KernelPath is the path to the unikernel binary.
	KernelPath string `json:"kernelPath,omitempty"`

	// Memory is the amount of memory allocated to the VM (e.g. "512Mi").
	Memory string `json:"memory,omitempty"`

	// Stack is the stack size (e.g. "8Mi").
	Stack string `json:"stack,omitempty"`

	// InitRd is the path to the initramfs/rootfs CPIO archive.
	InitRd string `json:"initrd,omitempty"`

	// LogPath is the path to the log file.
	LogPath string `json:"logPath,omitempty"`
}

type HyperlightOption func(*HyperlightConfig) error

func NewHyperlightConfig(opts ...HyperlightOption) (*HyperlightConfig, error) {
	cfg := &HyperlightConfig{
		Memory: "16Mi",
		Stack:  "8Mi",
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
