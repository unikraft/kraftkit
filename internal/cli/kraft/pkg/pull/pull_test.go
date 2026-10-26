// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package pull

import (
	"context"
	"strings"
	"testing"

	"kraftkit.sh/config"
	"kraftkit.sh/packmanager"
)

func TestPullPreFlagsValidation(t *testing.T) {
	origUmbrella := packmanager.UmbrellaInstance
	packmanager.UmbrellaInstance = &packmanager.UmbrellaManager{}
	defer func() {
		packmanager.UmbrellaInstance = origUmbrella
	}()

	for _, tc := range []struct {
		name         string
		platform     string
		architecture string
		expectPlat   string
		expectArch   string
		expectErr    string
	}{
		{
			name:         "valid plat and arch split",
			platform:     "qemu/x86_64",
			architecture: "",
			expectPlat:   "qemu",
			expectArch:   "x86_64",
		},
		{
			name:         "valid plat and matching arch",
			platform:     "qemu/arm64",
			architecture: "arm64",
			expectPlat:   "qemu",
			expectArch:   "arm64",
		},
		{
			name:         "invalid trailing slash in platform",
			platform:     "qemu/",
			architecture: "",
			expectErr:    "expected the flag in the form --plat=<plat>/<arch>",
		},
		{
			name:         "invalid leading slash in platform",
			platform:     "/x86_64",
			architecture: "",
			expectErr:    "expected the flag in the form --plat=<plat>/<arch>",
		},
		{
			name:         "conflicting platform and architecture flags",
			platform:     "qemu/arm64",
			architecture: "x86_64",
			expectErr:    "conflicting architecture specified in --plat (arm64) and --arch (x86_64)",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := PullOptions{
				Platform:     tc.platform,
				Architecture: tc.architecture,
			}

			cmd := NewCmd()
			ctx := config.WithConfigManager(context.Background(), &config.ConfigManager[config.KraftKit]{
				Config: &config.KraftKit{},
			})
			cmd.SetContext(ctx)

			err := opts.Pre(cmd, nil)
			if tc.expectErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.expectErr) {
					t.Fatalf("expected error containing %q, got: %v", tc.expectErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if opts.Platform != tc.expectPlat {
				t.Errorf("expected platform %q, got: %q", tc.expectPlat, opts.Platform)
			}
			if opts.Architecture != tc.expectArch {
				t.Errorf("expected architecture %q, got: %q", tc.expectArch, opts.Architecture)
			}
		})
	}
}
