// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package set

import (
	"context"
	"errors"
	"testing"
)

func TestSetValidation(t *testing.T) {
	for _, tc := range []struct {
		name      string
		args      []string
		expectErr error
	}{
		{
			name:      "no options to set",
			args:      []string{},
			expectErr: ErrNoOptionsToSet,
		},
		{
			name:      "malformed option without equals sign",
			args:      []string{"CONFIG_LIBUKDEBUG"},
			expectErr: ErrInvalidOption,
		},
		{
			name:      "malformed option with trailing equals sign",
			args:      []string{"CONFIG_LIBUKDEBUG="},
			expectErr: ErrInvalidOption,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := SetOptions{}
			err := opts.Run(context.Background(), tc.args)
			if err == nil || !errors.Is(err, tc.expectErr) {
				t.Fatalf("expected error wrapping %v, got: %v", tc.expectErr, err)
			}
		})
	}
}
