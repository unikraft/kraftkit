// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package logs

import (
	"testing"
)

func TestLogOptionsPrefixFor(t *testing.T) {
	tests := []struct {
		name        string
		opts        LogOptions
		machineName string
		longestName int
		want        string
	}{
		{
			name:        "explicit prefix wins over machine name",
			opts:        LogOptions{Prefix: "app", PrefixName: true},
			machineName: "unikernel",
			longestName: 9,
			want:        "app",
		},
		{
			name:        "explicit prefix wins over no prefix",
			opts:        LogOptions{Prefix: "app", NoPrefix: true},
			machineName: "unikernel",
			longestName: 9,
			want:        "app",
		},
		{
			name:        "prefix name pads a single machine",
			opts:        LogOptions{PrefixName: true},
			machineName: "uni",
			longestName: 9,
			want:        "uni      ",
		},
		{
			name:        "prefix name wins over no prefix",
			opts:        LogOptions{PrefixName: true, NoPrefix: true},
			machineName: "uni",
			longestName: 9,
			want:        "uni      ",
		},
		{
			name:        "multi-machine prefixes with the name",
			opts:        LogOptions{},
			machineName: "uni",
			longestName: 9,
			want:        "uni      ",
		},
		{
			name:        "no prefix returns empty",
			opts:        LogOptions{NoPrefix: true},
			machineName: "uni",
			longestName: 9,
			want:        "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.opts.prefixFor(tt.machineName, tt.longestName); got != tt.want {
				t.Errorf("prefixFor(%q, %d) = %q, want %q", tt.machineName, tt.longestName, got, tt.want)
			}
		})
	}
}
