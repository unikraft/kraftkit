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
	elfloader.name, elfloader.version = splitRuntimeNameVersion(name)
}

// String implements fmt.Stringer
func (ocipack *Runtime) String() string {
	return ocipack.pack.Name()
}

// Version implements kraftkit.sh/unikraft.Nameable
func (elfloader *Runtime) Version() string {
	return elfloader.version
}

// QueryName is the catalog key used to resolve the runtime package.
func (elfloader *Runtime) QueryName() string {
	if elfloader.name != "" {
		return elfloader.name
	}

	return elfloader.source
}

// QueryVersion is the catalog tag used to resolve the runtime package.
func (elfloader *Runtime) QueryVersion() string {
	return elfloader.version
}

// Reference returns the full runtime reference as supplied by the user or
// reconstructed from split name/version fields.
func (elfloader *Runtime) Reference() string {
	name := elfloader.QueryName()
	if name == "" {
		return ""
	}

	if elfloader.version != "" {
		return name + ":" + elfloader.version
	}

	return name
}

// Source of the ELF Loader runtime.
func (elfloader *Runtime) Source() string {
	return elfloader.source
}

func splitRuntimeNameVersion(name string) (string, string) {
	if name == "" {
		return "", ""
	}

	if strings.Contains(name, "@") {
		return name, ""
	}

	slash := strings.LastIndex(name, "/")
	colon := strings.LastIndex(name, ":")
	if colon <= slash {
		return name, ""
	}

	if firstSegment, _, ok := strings.Cut(name, "/"); ok {
		if strings.Contains(firstSegment, ".") || strings.Contains(firstSegment, ":") || firstSegment == "localhost" {
			return name, ""
		}
	}

	return name[:colon], name[colon+1:]
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
