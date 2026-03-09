// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package volume

import (
	"testing"
)

func TestVolumeConfig_Accessors(t *testing.T) {
	vol := &VolumeConfig{
		driver:      "9pfs",
		source:      "/host/path",
		destination: "/guest/path",
		mode:        "0755",
		readOnly:    true,
	}

	if got := vol.Driver(); got != "9pfs" {
		t.Errorf("Driver() = %q, want %q", got, "9pfs")
	}
	if got := vol.Source(); got != "/host/path" {
		t.Errorf("Source() = %q, want %q", got, "/host/path")
	}
	if got := vol.Destination(); got != "/guest/path" {
		t.Errorf("Destination() = %q, want %q", got, "/guest/path")
	}
	if got := vol.Mode(); got != "0755" {
		t.Errorf("Mode() = %q, want %q", got, "0755")
	}
	if got := vol.ReadOnly(); got != true {
		t.Errorf("ReadOnly() = %v, want %v", got, true)
	}
}

func TestVolumeConfig_Accessors_ZeroValues(t *testing.T) {
	vol := &VolumeConfig{}

	if got := vol.Driver(); got != "" {
		t.Errorf("Driver() = %q, want empty string", got)
	}
	if got := vol.Source(); got != "" {
		t.Errorf("Source() = %q, want empty string", got)
	}
	if got := vol.Destination(); got != "" {
		t.Errorf("Destination() = %q, want empty string", got)
	}
	if got := vol.Mode(); got != "" {
		t.Errorf("Mode() = %q, want empty string", got)
	}
	if got := vol.ReadOnly(); got != false {
		t.Errorf("ReadOnly() = %v, want false", got)
	}
}

func TestVolumeConfig_MarshalYAML(t *testing.T) {
	tests := []struct {
		name     string
		vol      *VolumeConfig
		wantNil  bool
		wantKeys map[string]interface{}
	}{
		{
			name:    "fully empty config returns nil",
			vol:     &VolumeConfig{},
			wantNil: true,
		},
		{
			name: "all fields populated",
			vol: &VolumeConfig{
				driver:      "9pfs",
				source:      "/host/path",
				destination: "/guest/path",
				readOnly:    true,
			},
			wantKeys: map[string]interface{}{
				"driver":      "9pfs",
				"source":      "/host/path",
				"destination": "/guest/path",
				"readOnly":    true,
			},
		},
		{
			name: "only source set includes source and readOnly",
			vol: &VolumeConfig{
				source: "/host/path",
			},
			wantKeys: map[string]interface{}{
				"source":   "/host/path",
				"readOnly": false,
			},
		},
		{
			name: "only destination set",
			vol: &VolumeConfig{
				destination: "/guest/path",
			},
			wantKeys: map[string]interface{}{
				"destination": "/guest/path",
			},
		},
		{
			name: "only driver set",
			vol: &VolumeConfig{
				driver: "9pfs",
			},
			wantKeys: map[string]interface{}{
				"driver": "9pfs",
			},
		},
		{
			name: "source with readonly true",
			vol: &VolumeConfig{
				source:   "/host/path",
				readOnly: true,
			},
			wantKeys: map[string]interface{}{
				"source":   "/host/path",
				"readOnly": true,
			},
		},
		{
			name: "source and destination without driver",
			vol: &VolumeConfig{
				source:      "/host/path",
				destination: "/guest/path",
			},
			wantKeys: map[string]interface{}{
				"source":      "/host/path",
				"destination": "/guest/path",
				"readOnly":    false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.vol.MarshalYAML()
			if err != nil {
				t.Fatalf("MarshalYAML() unexpected error = %v", err)
			}

			if tt.wantNil {
				if got != nil {
					t.Errorf("MarshalYAML() = %v, want nil", got)
				}
				return
			}

			result, ok := got.(map[string]interface{})
			if !ok {
				t.Fatalf("MarshalYAML() result type = %T, want map[string]interface{}", got)
			}

			if len(result) != len(tt.wantKeys) {
				t.Errorf("MarshalYAML() has %d keys, want %d — got: %v", len(result), len(tt.wantKeys), result)
			}

			for k, wantVal := range tt.wantKeys {
				gotVal, exists := result[k]
				if !exists {
					t.Errorf("MarshalYAML() missing key %q", k)
					continue
				}
				if gotVal != wantVal {
					t.Errorf("MarshalYAML()[%q] = %v, want %v", k, gotVal, wantVal)
				}
			}
		})
	}
}
