// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package hyperlight

import (
	"reflect"
	"testing"
)

func TestMarshalArgs(t *testing.T) {
	tests := []struct {
		name    string
		cfg     HyperlightConfig
		appArgs []string
		want    []string
	}{
		{
			name: "defaults memory stack and kernel only",
			cfg: HyperlightConfig{
				Memory:     "32Mi",
				Stack:      "8Mi",
				KernelPath: "/path/to/kernel",
			},
			want: []string{
				"--memory", "32Mi",
				"--stack", "8Mi",
				"/path/to/kernel",
			},
		},
		{
			name: "boolean flags",
			cfg: HyperlightConfig{
				Memory:      "64Mi",
				Stack:       "8Mi",
				KernelPath:  "/kernel",
				Quiet:       true,
				EnableTools: true,
				Net:         true,
			},
			want: []string{
				"--memory", "64Mi",
				"--stack", "8Mi",
				"--quiet",
				"--enable-tools",
				"--net",
				"/kernel",
			},
		},
		{
			name: "network policy ports repeat initrd mounts",
			cfg: HyperlightConfig{
				Memory:     "128Mi",
				Stack:      "16Mi",
				KernelPath: "/kernel",
				NetAllow:   []string{"10.0.0.1", "example.com"},
				Ports:      []int32{8080, 443},
				Repeat:     2,
				InitRd:     "/initrd.cpio",
				Mounts:     []string{"/host/src:/mnt/src"},
			},
			want: []string{
				"--memory", "128Mi",
				"--stack", "16Mi",
				"--net-allow", "10.0.0.1",
				"--net-allow", "example.com",
				"--port", "8080",
				"--port", "443",
				"--repeat", "2",
				"--initrd", "/initrd.cpio",
				"--mount", "/host/src:/mnt/src",
				"/kernel",
			},
		},
		{
			name: "exec mode without app arg separator",
			cfg: HyperlightConfig{
				Memory:     "32Mi",
				Stack:      "8Mi",
				KernelPath: "/kernel",
				Exec:       "print('hello')",
			},
			want: []string{
				"--memory", "32Mi",
				"--stack", "8Mi",
				"--exec", "print('hello')",
				"/kernel",
			},
		},
		{
			name: "application args after kernel separator",
			cfg: HyperlightConfig{
				Memory:     "32Mi",
				Stack:      "8Mi",
				KernelPath: "/kernel",
			},
			appArgs: []string{"--verbose", "arg"},
			want: []string{
				"--memory", "32Mi",
				"--stack", "8Mi",
				"/kernel",
				"--",
				"--verbose", "arg",
			},
		},
		{
			name: "repeat zero is omitted",
			cfg: HyperlightConfig{
				Memory:     "32Mi",
				Stack:      "8Mi",
				KernelPath: "/kernel",
				Repeat:     0,
			},
			want: []string{
				"--memory", "32Mi",
				"--stack", "8Mi",
				"/kernel",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.MarshalArgs(tt.appArgs)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MarshalArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}
