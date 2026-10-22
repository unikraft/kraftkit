// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package start

import (
	"testing"
)

func TestStartOptionsLogOptions(t *testing.T) {
	opts := StartOptions{
		NoPrefix:   true,
		Platform:   "qemu",
		Prefix:     "app",
		PrefixName: true,
	}

	logOpts := opts.logOptions()

	if !logOpts.Follow {
		t.Error("logOptions() should always follow")
	}
	if !logOpts.NoPrefix {
		t.Error("logOptions() did not carry NoPrefix")
	}
	if logOpts.Platform != "qemu" {
		t.Errorf("logOptions().Platform = %q, want %q", logOpts.Platform, "qemu")
	}
	if logOpts.Prefix != "app" {
		t.Errorf("logOptions().Prefix = %q, want %q", logOpts.Prefix, "app")
	}
	if !logOpts.PrefixName {
		t.Error("logOptions() did not carry PrefixName")
	}
}
