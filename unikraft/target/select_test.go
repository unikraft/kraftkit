// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package target

import (
	"testing"

	"kraftkit.sh/unikraft/arch"
	"kraftkit.sh/unikraft/plat"
)

// makeTargets builds a slice of Target for filter tests.
func makeTargets() []Target {
	return []Target{
		NewTargetFromOptions(
			WithName("app"),
			WithPlatform(plat.NewPlatformFromOptions(plat.WithName("kvm"))),
			WithArchitecture(arch.NewArchitectureFromOptions(arch.WithName("x86_64"))),
		),
		NewTargetFromOptions(
			WithName("app"),
			WithPlatform(plat.NewPlatformFromOptions(plat.WithName("xen"))),
			WithArchitecture(arch.NewArchitectureFromOptions(arch.WithName("arm64"))),
		),
		NewTargetFromOptions(
			WithName("srv"),
			WithPlatform(plat.NewPlatformFromOptions(plat.WithName("kvm"))),
			WithArchitecture(arch.NewArchitectureFromOptions(arch.WithName("arm64"))),
		),
	}
}

func TestFilter(t *testing.T) {
	targets := makeTargets()

	t.Run("no filters returns all targets", func(t *testing.T) {
		got := Filter(targets, "", "", "")
		if len(got) != len(targets) {
			t.Errorf("Filter() returned %d targets, want %d", len(got), len(targets))
		}
	})

	t.Run("filter by arch x86_64", func(t *testing.T) {
		got := Filter(targets, "x86_64", "", "")
		if len(got) != 1 {
			t.Fatalf("Filter(arch=x86_64) returned %d targets, want 1", len(got))
		}
		if got[0].Architecture().Name() != "x86_64" {
			t.Errorf("unexpected architecture: %q", got[0].Architecture().Name())
		}
	})

	t.Run("filter by arch arm64", func(t *testing.T) {
		got := Filter(targets, "arm64", "", "")
		if len(got) != 2 {
			t.Errorf("Filter(arch=arm64) returned %d targets, want 2", len(got))
		}
	})

	t.Run("filter by plat kvm", func(t *testing.T) {
		got := Filter(targets, "", "kvm", "")
		if len(got) != 2 {
			t.Errorf("Filter(plat=kvm) returned %d targets, want 2", len(got))
		}
		for _, tgt := range got {
			if tgt.Platform().Name() != "kvm" {
				t.Errorf("unexpected platform: %q", tgt.Platform().Name())
			}
		}
	})

	t.Run("filter by plat and arch", func(t *testing.T) {
		got := Filter(targets, "arm64", "kvm", "")
		if len(got) != 1 {
			t.Fatalf("Filter(arch=arm64,plat=kvm) returned %d targets, want 1", len(got))
		}
		if got[0].Name() != "srv" {
			t.Errorf("Name = %q, want srv", got[0].Name())
		}
	})

	t.Run("filter by targ name only", func(t *testing.T) {
		got := Filter(targets, "", "", "srv")
		if len(got) != 1 {
			t.Fatalf("Filter(targ=srv) returned %d targets, want 1", len(got))
		}
		if got[0].Name() != "srv" {
			t.Errorf("Name = %q, want srv", got[0].Name())
		}
	})

	t.Run("filter by targ as plat/arch string", func(t *testing.T) {
		got := Filter(targets, "", "", "kvm/x86_64")
		if len(got) != 1 {
			t.Fatalf("Filter(targ=kvm/x86_64) returned %d targets, want 1", len(got))
		}
		if got[0].Platform().Name() != "kvm" || got[0].Architecture().Name() != "x86_64" {
			t.Errorf("unexpected target: %s/%s", got[0].Platform().Name(), got[0].Architecture().Name())
		}
	})

	t.Run("filter by plat and targ name", func(t *testing.T) {
		got := Filter(targets, "", "xen", "app")
		if len(got) != 1 {
			t.Fatalf("Filter(plat=xen,targ=app) returned %d targets, want 1", len(got))
		}
		if got[0].Platform().Name() != "xen" {
			t.Errorf("Platform = %q, want xen", got[0].Platform().Name())
		}
	})

	t.Run("filter by all three — exact match", func(t *testing.T) {
		got := Filter(targets, "x86_64", "kvm", "app")
		if len(got) != 1 {
			t.Fatalf("Filter(arch=x86_64,plat=kvm,targ=app) returned %d targets, want 1", len(got))
		}
	})

	t.Run("no match returns empty slice", func(t *testing.T) {
		got := Filter(targets, "arm", "", "")
		if len(got) != 0 {
			t.Errorf("Filter(arch=arm) returned %d targets, want 0", len(got))
		}
	})
}
