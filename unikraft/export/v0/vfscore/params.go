// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package vfscore

import (
	"strings"

	"kraftkit.sh/unikraft/export/v0/ukargparse"
)

var ParamVfsFstab = ukargparse.NewParamStrSlice("vfs", "fstab", nil)

// ExportedParams returns the parameters available by this exported library.
func ExportedParams() []ukargparse.Param {
	return []ukargparse.Param{
		ParamVfsFstab,
	}
}

// FstabEntry is a vfscore mount entry.
type FstabEntry struct {
	sourceDevice string
	mountTarget  string
	fsDriver     string
	flags        string
	opts         string
	ukopts       string
}

// NewFstabEntry generates a structure that is representative of one of
// Unikraft's vfscore automounts.
func NewFstabEntry(sourceDevice, mountTarget, fsDriver, flags, opts, ukopts string) FstabEntry {
	return FstabEntry{
		sourceDevice,
		mountTarget,
		fsDriver,
		flags,
		opts,
		ukopts,
	}
}

// String implements fmt.Stringer and returns a valid vfs.automount-formatted
// entry.
func (entry FstabEntry) String() string {
	return strings.Join([]string{
		entry.sourceDevice,
		entry.mountTarget,
		entry.fsDriver,
		entry.flags,
		entry.opts,
		entry.ukopts,
	}, ":")
}
