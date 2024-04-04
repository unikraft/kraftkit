// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package runtime

import (
	"context"
	"fmt"
	"time"

	"kraftkit.sh/initrd"
	"kraftkit.sh/internal/tableprinter"
	"kraftkit.sh/oci"
	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/arch"
	"kraftkit.sh/unikraft/plat"
	"kraftkit.sh/unikraft/target"
)

var _ pack.Package = (*Runtime)(nil)

const (
	PrebuiltRegistry = "unikraft.org"
	DefaultPrebuilt  = "unikraft.org/base:latest"
)

// NewELFLoaderRuntime prepares a "ELF loader" application that has been
// pre-built and is accessible from a remote registry.
func NewELFLoaderRuntime(ctx context.Context, pbopts ...RuntimeOption) (*Runtime, error) {
	elfloader := Runtime{}

	for _, opt := range pbopts {
		if err := opt(&elfloader); err != nil {
			return nil, err
		}
	}

	if defaultRuntime != "" {
		// Return early if the user provided a custom elfloader unikernel
		// application.
		if ok, _ := unikraft.IsFileUnikraftUnikernel(defaultRuntime); ok {
			elfloader.kernel = defaultRuntime
			return &elfloader, nil
		}

		elfloader.source = defaultRuntime
	} else if len(elfloader.kernel) > 0 {
		return &elfloader, nil
	} else if len(elfloader.name) == 0 {
		elfloader.name = DefaultPrebuilt
	}

	var err error
	elfloader.registry = packmanager.G(ctx)
	if elfloader.registry == nil {
		elfloader.registry, err = oci.NewOCIManager(ctx,
			oci.WithDetectHandler(),
		)
	} else {
		elfloader.registry, err = elfloader.registry.From(oci.OCIFormat)
	}
	if err != nil {
		return nil, err
	}

	if err := elfloader.registry.SetSources(ctx, PrebuiltRegistry); err != nil {
		return nil, err
	}

	// First try locally
	results, err := elfloader.registry.Catalog(ctx,
		packmanager.WithName(elfloader.name),
		packmanager.WithTypes(unikraft.ComponentTypeApp),
	)
	if err != nil {
		return nil, err
	}

	if len(results) == 0 {
		results, err = elfloader.registry.Catalog(ctx,
			packmanager.WithName(elfloader.name),
			packmanager.WithTypes(unikraft.ComponentTypeApp),
			packmanager.WithRemote(true),
		)
		if err != nil {
			return nil, err
		}
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("could not find elfloader")
	} else if len(results) > 1 {
		options := make([]string, len(results))
		for i, result := range results {
			options[i] = result.Name()
		}
		return nil, fmt.Errorf("too many options: %v", options)
	}

	elfloader.pack = results[0]
	elfloader.name = results[0].Name()
	elfloader.version = results[0].Version()
	elfloader.source = results[0].Name()

	return &elfloader, nil
}

// ID implements kraftkit.sh/pack.Package
func (runtime *Runtime) ID() string {
	return runtime.pack.ID()
}

// Columns implements kraftkit.sh/pack.Package
func (elfloader *Runtime) Columns() []tableprinter.Column {
	return elfloader.pack.Columns()
}

// Metadata implements kraftkit.sh/pack.Package
func (elfloader *Runtime) Metadata() interface{} {
	return elfloader.pack.Metadata()
}

// Push implements kraftkit.sh/pack.Package
func (elfloader *Runtime) Push(ctx context.Context, opts ...pack.PushOption) error {
	panic("not implemented: kraftkit.sh/unikraft/elfloader.Runtime.Push")
}

// Unpack implements kraftkit.sh/pack.Package
func (elfloader *Runtime) Unpack(ctx context.Context, dir string) error {
	return elfloader.pack.Unpack(ctx, dir)
}

// Pull implements kraftkit.sh/pack.Package
func (elfloader *Runtime) Pull(ctx context.Context, opts ...pack.PullOption) error {
	return elfloader.pack.Pull(ctx, opts...)
}

// Pull implements kraftkit.sh/pack.Package
func (elfloader *Runtime) PulledAt(ctx context.Context) (bool, time.Time, error) {
	return elfloader.pack.PulledAt(ctx)
}

func (elfloader *Runtime) Delete(ctx context.Context) error {
	return elfloader.pack.Delete(ctx)
}

// Save implements kraftkit.sh/pack.Package
func (elfloader *Runtime) Save(ctx context.Context) error {
	return elfloader.pack.Save(ctx)
}

// Format implements kraftkit.sh/unikraft.component.Component
func (elfloader *Runtime) Format() pack.PackageFormat {
	return elfloader.pack.Format()
}

// Architecture implements kraftkit.sh/unikraft.target.Target
func (elfloader *Runtime) Architecture() arch.Architecture {
	return elfloader.pack.(target.Target).Architecture()
}

// Platform implements kraftkit.sh/unikraft.target.Target
func (elfloader *Runtime) Platform() plat.Platform {
	return elfloader.pack.(target.Target).Platform()
}

// Kernel implements kraftkit.sh/unikraft.target.Target
func (elfloader *Runtime) Kernel() string {
	if len(elfloader.kernel) > 0 {
		return elfloader.kernel
	}

	if t, ok := elfloader.pack.(target.Target); ok {
		return t.Kernel()
	}

	return ""
}

// KernelDbg implements kraftkit.sh/unikraft.target.Target
func (elfloader *Runtime) KernelDbg() string {
	return elfloader.pack.(target.Target).KernelDbg()
}

// Initrd implements kraftkit.sh/unikraft.target.Target
func (elfloader *Runtime) Initrd() initrd.Initrd {
	return elfloader.pack.(target.Target).Initrd()
}

// Command implements kraftkit.sh/unikraft.target.Target
func (elfloader *Runtime) Command() []string {
	return elfloader.pack.(target.Target).Command()
}

// ConfigFilename implements kraftkit.sh/unikraft.target.Target
func (elfloader *Runtime) ConfigFilename() string {
	return elfloader.pack.(target.Target).ConfigFilename()
}
