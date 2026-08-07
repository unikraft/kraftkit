// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package build

import (
	"context"
	"reflect"
	"slices"
	"testing"

	"kraftkit.sh/config"
	"kraftkit.sh/make"
)

func TestMergeToolchain(t *testing.T) {
	for _, tc := range []struct {
		name        string
		global      map[string]string
		profiles    map[string]map[string]string
		profileName string
		flags       []string
		expect      map[string]string
	}{
		{
			name:   "flags only",
			global: nil,
			flags:  []string{"CC=clang", "LD=ld.lld"},
			expect: map[string]string{"CC": "clang", "LD": "ld.lld"},
		},
		{
			name:   "global only",
			global: map[string]string{"CC": "gcc", "UK_CFLAGS": "-O2"},
			flags:  nil,
			expect: map[string]string{"CC": "gcc", "UK_CFLAGS": "-O2"},
		},
		{
			name:   "flags and global disjoint keys",
			global: map[string]string{"UK_CFLAGS": "-O2"},
			flags:  []string{"CC=clang"},
			expect: map[string]string{"CC": "clang", "UK_CFLAGS": "-O2"},
		},
		{
			name:   "flag overrides global on same key",
			global: map[string]string{"CC": "gcc"},
			flags:  []string{"CC=clang"},
			expect: map[string]string{"CC": "clang"},
		},
		{
			name:   "malformed flag without equals is ignored",
			global: nil,
			flags:  []string{"BADENTRY", "CC=clang"},
			expect: map[string]string{"CC": "clang"},
		},
		{
			name:   "both empty returns empty map",
			global: nil,
			flags:  nil,
			expect: map[string]string{},
		},
		{
			name:   "value containing equals sign is preserved",
			global: nil,
			flags:  []string{"UK_CFLAGS=-O2 -DFOO=1"},
			expect: map[string]string{"UK_CFLAGS": "-O2 -DFOO=1"},
		},
		{
			name: "profile only",
			profiles: map[string]map[string]string{
				"myprofile": {"CC": "clang", "UK_CFLAGS": "-O3"},
			},
			profileName: "myprofile",
			expect:      map[string]string{"CC": "clang", "UK_CFLAGS": "-O3"},
		},
		{
			name:   "profile overrides global but flag overrides profile",
			global: map[string]string{"CC": "gcc", "LD": "ld.bfd"},
			profiles: map[string]map[string]string{
				"myprofile": {"CC": "clang", "UK_CFLAGS": "-O3"},
			},
			profileName: "myprofile",
			flags:       []string{"UK_CFLAGS=-O0"},
			expect:      map[string]string{"CC": "clang", "LD": "ld.bfd", "UK_CFLAGS": "-O0"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := config.WithConfigManager(context.Background(), &config.ConfigManager[config.KraftKit]{
				Config: &config.KraftKit{
					Toolchain:         tc.global,
					ToolchainProfiles: tc.profiles,
				},
			})
			got := mergeToolchain(ctx, tc.global, tc.profileName, tc.flags)
			if !reflect.DeepEqual(got, tc.expect) {
				t.Errorf("mergeToolchain() = %v, want %v", got, tc.expect)
			}
		})
	}
}

func TestValidateToolchainProfile(t *testing.T) {
	for _, tc := range []struct {
		name        string
		profiles    map[string]map[string]string
		profileName string
		wantErr     string
	}{
		{
			name: "no profile selected",
		},
		{
			name: "configured profile",
			profiles: map[string]map[string]string{
				"clang": {"CC": "clang"},
			},
			profileName: "clang",
		},
		{
			name:        "unknown profile",
			profileName: "clang",
			wantErr:     `toolchain profile "clang" does not exist`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := config.WithConfigManager(context.Background(), &config.ConfigManager[config.KraftKit]{
				Config: &config.KraftKit{ToolchainProfiles: tc.profiles},
			})

			err := validateToolchainProfile(ctx, tc.profileName)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("validateToolchainProfile() returned unexpected error: %v", err)
				}
				return
			}
			if err == nil || err.Error() != tc.wantErr {
				t.Fatalf("validateToolchainProfile() error = %v, want %q", err, tc.wantErr)
			}
		})
	}
}

func TestMakeOptionsForBuildIncludesResolvedToolchain(t *testing.T) {
	for _, tc := range []struct {
		name        string
		global      map[string]string
		profiles    map[string]map[string]string
		profileName string
		flags       []string
		expect      []string
	}{
		{
			name:   "global values are forwarded to make",
			global: map[string]string{"CC": "gcc", "UK_CFLAGS": "-O2"},
			expect: []string{"CC=gcc", "UK_CFLAGS=-O2"},
		},
		{
			name:   "cli overrides win over global config",
			global: map[string]string{"CC": "gcc", "LD": "ld"},
			flags:  []string{"CC=clang", "UK_CFLAGS=-O0 -g"},
			expect: []string{"CC=clang", "LD=ld", "UK_CFLAGS=-O0 -g"},
		},
		{
			name:        "profile overrides win over global, and cli overrides win over profile",
			global:      map[string]string{"CC": "gcc", "LD": "ld"},
			profileName: "myprofile",
			profiles: map[string]map[string]string{
				"myprofile": {"CC": "clang", "EXTRA_CFLAGS": "-O3"},
			},
			flags:  []string{"CC=gcc-12"},
			expect: []string{"CC=gcc-12", "LD=ld", "EXTRA_CFLAGS=-O3"},
		},
		{
			name:   "malformed flags are ignored",
			global: map[string]string{"CC": "gcc"},
			flags:  []string{"BADENTRY"},
			expect: []string{"CC=gcc"},
		},
		{
			name:   "empty config and flags produce no vars",
			expect: nil,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := config.WithConfigManager(context.Background(), &config.ConfigManager[config.KraftKit]{
				Config: &config.KraftKit{
					Toolchain:         tc.global,
					ToolchainProfiles: tc.profiles,
				},
			})

			mo, err := make.NewMakeOptions(makeOptionsForBuild(ctx, &BuildOptions{
				Toolchain:        tc.flags,
				ToolchainProfile: tc.profileName,
			})...)
			if err != nil {
				t.Fatalf("NewMakeOptions() returned unexpected error: %v", err)
			}

			got := mo.Vars()
			slices.Sort(got)
			slices.Sort(tc.expect)

			if !reflect.DeepEqual(got, tc.expect) {
				t.Fatalf("makeOptionsForBuild() vars = %v, want %v", got, tc.expect)
			}
		})
	}
}
