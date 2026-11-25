// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package volume

import (
	"context"
	"testing"
)

func TestTransformFromSchema_ShortHand(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		wantSource      string
		wantDestination string
	}{
		{
			name:            "source and destination",
			input:           "./src:/dest",
			wantSource:      "./src",
			wantDestination: "/dest",
		},
		{
			name:            "source only mounts at root",
			input:           "./rootfs",
			wantSource:      "./rootfs",
			wantDestination: "/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := TransformFromSchema(context.Background(), tt.input)
			if err != nil {
				t.Fatalf("TransformFromSchema(%q) error = %v", tt.input, err)
			}

			volume, ok := got.(VolumeConfig)
			if !ok {
				t.Fatalf("TransformFromSchema(%q) = %T, want VolumeConfig", tt.input, got)
			}

			if volume.Source() != tt.wantSource {
				t.Errorf("Source() = %q, want %q", volume.Source(), tt.wantSource)
			}

			if volume.Destination() != tt.wantDestination {
				t.Errorf("Destination() = %q, want %q", volume.Destination(), tt.wantDestination)
			}
		})
	}
}
