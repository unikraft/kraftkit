// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package unset

import (
	"context"
	"errors"
	"testing"
)

func TestUnsetValidation(t *testing.T) {
	for _, tc := range []struct {
		name      string
		args      []string
		expectErr error
	}{
		{
			name:      "no options to unset",
			args:      []string{},
			expectErr: ErrNoOptionsToUnset,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := UnsetOptions{}
			err := opts.Run(context.Background(), tc.args)
			if err == nil || !errors.Is(err, tc.expectErr) {
				t.Fatalf("expected error wrapping %v, got: %v", tc.expectErr, err)
			}
		})
	}
}
