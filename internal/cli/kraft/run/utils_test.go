// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package run

import (
	"context"
	"strings"
	"testing"

	machineapi "kraftkit.sh/api/machine/v1alpha1"
)

func TestParseNetworksMacAddress(t *testing.T) {
	for _, tc := range []struct {
		name      string
		mac       string
		networks  []string
		expectErr string
	}{
		{
			name:      "no network",
			mac:       "02:00:00:00:00:01",
			networks:  nil,
			expectErr: "the --mac flag only works when providing exactly one network",
		},
		{
			name:      "two networks",
			mac:       "02:00:00:00:00:01",
			networks:  []string{"kraft0", "kraft1"},
			expectErr: "the --mac flag only works when providing exactly one network",
		},
		{
			name:      "invalid mac",
			mac:       "not-a-mac",
			networks:  []string{"kraft0"},
			expectErr: "could not parse MAC address",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := RunOptions{
				MacAddress: tc.mac,
				Networks:   tc.networks,
			}

			err := opts.parseNetworks(context.Background(), &machineapi.Machine{})
			if err == nil || !strings.Contains(err.Error(), tc.expectErr) {
				t.Fatalf("expected error containing %q, got: %v", tc.expectErr, err)
			}
		})
	}
}
