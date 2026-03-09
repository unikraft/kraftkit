// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package volume

import (
	"context"
	"testing"
)

func Test_TransformFromSchema(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name         string
		input        interface{}
		wantSource   string
		wantDest     string
		wantDriver   string
		wantReadOnly bool
		wantErr      bool
	}{
		{
			name:       "string with colon sets source and destination",
			input:      "src:/mnt",
			wantSource: "src",
			wantDest:   "/mnt",
		},
		{
			name:       "string without colon defaults destination to slash",
			input:      "src",
			wantSource: "src",
			wantDest:   "/",
		},
		{
			name:    "string with more than one colon returns error",
			input:   "src:/mnt:/extra",
			wantErr: true,
		},
		{
			name: "map with all fields",
			input: map[string]interface{}{
				"driver":      "9pfs",
				"source":      "/host/path",
				"destination": "/guest/path",
				"readonly":    true,
			},
			wantDriver:   "9pfs",
			wantSource:   "/host/path",
			wantDest:     "/guest/path",
			wantReadOnly: true,
		},
		{
			name: "map with only source",
			input: map[string]interface{}{
				"source": "/host/path",
			},
			wantSource: "/host/path",
		},
		{
			name: "map with readonly false",
			input: map[string]interface{}{
				"source":   "/host/path",
				"readonly": false,
			},
			wantSource:   "/host/path",
			wantReadOnly: false,
		},
		{
			name:    "map with invalid driver type errors",
			input:   map[string]interface{}{"driver": 123},
			wantErr: true,
		},
		{
			name:    "map with invalid source type errors",
			input:   map[string]interface{}{"source": 123},
			wantErr: true,
		},
		{
			name:    "map with invalid destination type errors",
			input:   map[string]interface{}{"destination": 123},
			wantErr: true,
		},
		{
			name:    "map with invalid readonly type errors",
			input:   map[string]interface{}{"readonly": "yes"},
			wantErr: true,
		},
		{
			name:       "empty map returns empty volume",
			input:      map[string]interface{}{},
			wantSource: "",
			wantDest:   "",
		},
		{
			name:       "empty string sets empty source and slash destination",
			input:      "",
			wantSource: "",
			wantDest:   "/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := TransformFromSchema(ctx, tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("TransformFromSchema() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			vol, ok := got.(VolumeConfig)
			if !ok {
				t.Fatalf("TransformFromSchema() result type = %T, want VolumeConfig", got)
			}

			if vol.Source() != tt.wantSource {
				t.Errorf("Source() = %q, want %q", vol.Source(), tt.wantSource)
			}
			if vol.Destination() != tt.wantDest {
				t.Errorf("Destination() = %q, want %q", vol.Destination(), tt.wantDest)
			}
			if vol.Driver() != tt.wantDriver {
				t.Errorf("Driver() = %q, want %q", vol.Driver(), tt.wantDriver)
			}
			if vol.ReadOnly() != tt.wantReadOnly {
				t.Errorf("ReadOnly() = %v, want %v", vol.ReadOnly(), tt.wantReadOnly)
			}
		})
	}
}
