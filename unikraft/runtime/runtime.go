// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package runtime

import (
	"strings"

	"kraftkit.sh/kconfig"
	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/unikraft"
)

type Runtime struct {
	registry packmanager.PackageManager

	// Path to the kernel of the ELF loader.
	kernel string

	// The package representing the ELF Loader.
	pack pack.Package

	// Platform specifies the platform of the loader.
	platform string

	// Architecture specifies the architecture of the loader.
	architecture string

	// The name of the elfloader.
	name string

	// The version of the elfloader.
	version string

	// The source of the elfloader (can be either remote or local, this attribute
	// is ultimately handled by the packmanager).
	source string

	// List of kconfig key-values specific to this core.
	kconfig kconfig.KeyValueMap

	// The rootfs (initramfs) of the ELF loader.
	rootfs string
}

var _ unikraft.Nameable = (*Runtime)(nil)

// Type implements kraftkit.sh/unikraft.Nameable
func (elfloader *Runtime) Type() unikraft.ComponentType {
	return unikraft.ComponentTypeApp
}

// Name implements kraftkit.sh/unikraft.Nameable
func (elfloader *Runtime) Name() string {
	return elfloader.name
}

// SetName overwrites the name of the runtime.
func (elfloader *Runtime) SetName(name string) {
	if runtime := strings.Split(name, ":"); len(runtime) == 2 {
		elfloader.name = runtime[0]
		elfloader.version = runtime[1]
		return
	}

	elfloader.name = name
}

// String implements fmt.Stringer
func (ocipack *Runtime) String() string {
	return ocipack.pack.Name()
}

// Version implements kraftkit.sh/unikraft.Nameable
func (elfloader *Runtime) Version() string {
	return elfloader.version
}

// Source of the ELF Loader runtime.
func (elfloader *Runtime) Source() string {
	return elfloader.source
}

func (elfloader *Runtime) MarshalYAML() (interface{}, error) {
	ret := map[string]interface{}{}
	if len(elfloader.name) > 0 {
		ret["name"] = elfloader.name
	}
	if len(elfloader.version) > 0 {
		ret["version"] = elfloader.version
	}
	if len(elfloader.source) > 0 {
		ret["source"] = elfloader.source
	}
	if len(ret) == 0 {
		return nil, nil
	}
	return ret, nil
}
