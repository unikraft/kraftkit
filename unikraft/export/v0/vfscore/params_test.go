// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package vfscore

import (
	"testing"
)

func TestExportedParams(t *testing.T) {
	params := ExportedParams()
	if len(params) != 1 {
		t.Fatalf("expected 1 param, got %d", len(params))
	}
	if got := params[0].Name(); got != "vfs.fstab" {
		t.Errorf("got %q, want %q", got, "vfs.fstab")
	}
}

func TestNewFstabEntry_String(t *testing.T) {
	tests := []struct {
		name         string
		sourceDevice string
		mountTarget  string
		fsDriver     string
		flags        string
		opts         string
		ukopts       string
		want         string
	}{
		{
			name:         "full entry",
			sourceDevice: "fs0",
			mountTarget:  "/",
			fsDriver:     "9pfs",
			flags:        "",
			opts:         "trans=virtio,version=9p2000.L",
			ukopts:       "",
			want:         "fs0:/:9pfs::trans=virtio,version=9p2000.L:",
		},
		{
			name:         "empty entry",
			sourceDevice: "",
			mountTarget:  "",
			fsDriver:     "",
			flags:        "",
			opts:         "",
			ukopts:       "",
			want:         ":::::",
		},
		{
			name:         "all fields set",
			sourceDevice: "dev",
			mountTarget:  "/mnt",
			fsDriver:     "ext4",
			flags:        "ro",
			opts:         "noatime",
			ukopts:       "custom",
			want:         "dev:/mnt:ext4:ro:noatime:custom",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			entry := NewFstabEntry(tc.sourceDevice, tc.mountTarget, tc.fsDriver, tc.flags, tc.opts, tc.ukopts)
			if got := entry.String(); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
