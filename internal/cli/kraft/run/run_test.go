// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package run

import (
	"testing"

	mplatform "kraftkit.sh/machine/platform"
)

func TestRunOptionsStartOptions(t *testing.T) {
	opts := RunOptions{
		Detach:     true,
		Remove:     true,
		Prefix:     "app",
		PrefixName: true,
		platform:   mplatform.Platform("qemu"),
	}

	startOpts := opts.startOptions()

	if !startOpts.Detach {
		t.Error("startOptions() did not carry Detach")
	}
	if startOpts.Platform != "qemu" {
		t.Errorf("startOptions().Platform = %q, want %q", startOpts.Platform, "qemu")
	}
	if !startOpts.Remove {
		t.Error("startOptions() did not carry Remove")
	}
	if startOpts.Prefix != "app" {
		t.Errorf("startOptions().Prefix = %q, want %q", startOpts.Prefix, "app")
	}
	if !startOpts.PrefixName {
		t.Error("startOptions() did not carry PrefixName")
	}
}
