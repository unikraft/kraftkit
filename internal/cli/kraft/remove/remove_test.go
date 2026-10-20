// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package remove

import (
	"context"
	"strings"
	"testing"
)

func TestRemoveValidation(t *testing.T) {
	for _, tc := range []struct {
		name      string
		opts      RemoveOptions
		args      []string
		expectErr string
	}{
		{
			name:      "no machine specified without all",
			opts:      RemoveOptions{All: false},
			args:      []string{},
			expectErr: "no machine(s) specified",
		},
		{
			name:      "conflicting machine and all flag",
			opts:      RemoveOptions{All: true},
			args:      []string{"test-machine"},
			expectErr: "cannot specify machines and --all at the same time",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.opts.Run(context.Background(), tc.args)
			if err == nil || !strings.Contains(err.Error(), tc.expectErr) {
				t.Fatalf("expected error containing %q, got: %v", tc.expectErr, err)
			}
		})
	}
}
