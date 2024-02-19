// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package xen

import (
	"xenbits.xenproject.org/git-http/xen.git/tools/golang/xenlight"
)

const (
	XenMemoryScale   = 1024
	XenMemoryDefault = 64
	XenCPUsDefault   = 1
)

type XenConfig struct {
	DomID xenlight.Domid
}

type XenOption func(*xenlight.DomainConfig) error

func NewXenConfig(xopts ...XenOption) (*xenlight.DomainConfig, error) {
	xcfg, err := xenlight.NewDomainConfig()
	if err != nil {
		return nil, err
	}

	binfoPtr, err := xenlight.NewDomainBuildInfo(xenlight.DomainTypePv)
	if err != nil {
		return nil, err
	}

	xcfg.BInfo = *binfoPtr

	cinfoPtr, err := xenlight.NewDomainCreateInfo()
	if err != nil {
		return nil, err
	}

	xcfg.CInfo = *cinfoPtr

	xcfg.CInfo.Type = xenlight.DomainTypePv

	for _, xopt := range xopts {
		if err := xopt(xcfg); err != nil {
			return nil, err
		}
	}
	return xcfg, nil
}

func WithCpu(cpu int) XenOption {
	return func(cfg *xenlight.DomainConfig) error {
		cfg.BInfo.MaxVcpus = cpu
		return nil
	}
}

func WithMemoryKb(memory uint64) XenOption {
	return func(cfg *xenlight.DomainConfig) error {
		cfg.BInfo.MaxMemkb = memory
		return nil
	}
}

func WithName(name string) XenOption {
	return func(cfg *xenlight.DomainConfig) error {
		cfg.CInfo.Name = name
		return nil
	}
}

func WithP9(p9 xenlight.DeviceP9) XenOption {
	return func(cfg *xenlight.DomainConfig) error {
		cfg.P9S = append(cfg.P9S, p9)
		return nil
	}
}

func WithNetwork(network xenlight.DeviceNic) XenOption {
	return func(cfg *xenlight.DomainConfig) error {
		cfg.Nics = append(cfg.Nics, network)
		return nil
	}
}

func WithKernel(kernel string) XenOption {
	return func(cfg *xenlight.DomainConfig) error {
		cfg.BInfo.Kernel = kernel
		return nil
	}
}

func WithRamdisk(ramdisk string) XenOption {
	return func(cfg *xenlight.DomainConfig) error {
		cfg.BInfo.Ramdisk = ramdisk
		return nil
	}
}

func WithUuid(uuid string) XenOption {
	return func(cfg *xenlight.DomainConfig) error {
		cfg.CInfo.Uuid = xenlight.Uuid([]byte(uuid))
		return nil
	}
}

func WithArgs(args []string) XenOption {
	return func(cfg *xenlight.DomainConfig) error {
		cfg.BInfo.Extra = xenlight.StringList(args)
		return nil
	}
}

func WithType(xtype xenlight.DomainType) XenOption {
	return func(cfg *xenlight.DomainConfig) error {
		cfg.BInfo.Type = xtype
		cfg.CInfo.Type = xtype
		return nil
	}
}
