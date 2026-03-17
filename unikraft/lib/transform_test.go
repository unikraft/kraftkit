// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package lib

import (
	"context"
	"testing"

	"kraftkit.sh/unikraft"
)

func TestTransformFromSchema(t *testing.T) {
	tests := []struct {
		name        string
		libName     string
		props       interface{}
		ukCtx       *unikraft.Context
		wantErr     bool
		wantName    string
		wantSource  string
		wantVersion string
	}{
		{
			name:        "empty name and string version prop",
			libName:     "",
			props:       "0.13.0",
			wantVersion: "0.13.0",
		},
		{
			name:        "name set from argument",
			libName:     "libfoo",
			props:       "0.1.0",
			wantName:    "libfoo",
			wantVersion: "0.1.0",
		},
		{
			name:    "map with source string",
			libName: "libbar",
			props: map[string]interface{}{
				"source": "https://github.com/example/lib",
			},
			wantName:   "libbar",
			wantSource: "https://github.com/example/lib",
		},
		{
			name:    "map with version string",
			libName: "libbaz",
			props: map[string]interface{}{
				"version": "2.0.0",
			},
			wantName:    "libbaz",
			wantVersion: "2.0.0",
		},
		{
			name:    "map with kconfig as map",
			libName: "libkconf",
			props: map[string]interface{}{
				"kconfig": map[string]interface{}{
					"CONFIG_FOO": "y",
				},
			},
			wantName: "libkconf",
		},
		{
			name:    "map with kconfig as slice",
			libName: "libkslice",
			props: map[string]interface{}{
				"kconfig": []interface{}{"CONFIG_BAR=y"},
			},
			wantName: "libkslice",
		},
		{
			name:    "invalid source type returns error",
			libName: "liberr",
			props: map[string]interface{}{
				"source": 42,
			},
			wantErr: true,
		},
		{
			name:    "unknown version type gets sprint-converted to string without error",
			libName: "liberr2",
			props: map[string]interface{}{
				"version": []string{"bad"},
			},
			wantErr: false,
			// TranslateFromSchema uses fmt.Sprint for unknown types
			wantName: "liberr2",
		},
		{
			name:    "unrecognized kconfig type is silently ignored without error",
			libName: "liberr3",
			props: map[string]interface{}{
				"kconfig": "invalid_string_kconfig",
			},
			wantErr:  false,
			wantName: "liberr3",
		},
		{
			name:    "with uk context uk_base set",
			libName: "libwithctx",
			props:   "1.0.0",
			ukCtx: &unikraft.Context{
				UK_BASE: "/tmp/unikraft",
			},
			wantName:    "libwithctx",
			wantVersion: "1.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.ukCtx != nil {
				ctx = unikraft.WithContext(ctx, tt.ukCtx)
			}

			lib, err := TransformFromSchema(ctx, tt.libName, tt.props)
			if (err != nil) != tt.wantErr {
				t.Errorf("TransformFromSchema() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			if tt.wantName != "" && lib.name != tt.wantName {
				t.Errorf("TransformFromSchema() name = %q, want %q", lib.name, tt.wantName)
			}
			if tt.wantVersion != "" && lib.version != tt.wantVersion {
				t.Errorf("TransformFromSchema() version = %q, want %q", lib.version, tt.wantVersion)
			}
			if tt.wantSource != "" && lib.source != tt.wantSource {
				t.Errorf("TransformFromSchema() source = %q, want %q", lib.source, tt.wantSource)
			}
		})
	}
}

func TestTransformFromSchema_LocalDir(t *testing.T) {
	// Test case where source is a local directory
	ctx := context.Background()

	// t.TempDir() is a valid directory on disk
	dir := t.TempDir()

	lib, err := TransformFromSchema(ctx, "liblocal", map[string]interface{}{
		"source": dir,
	})
	if err != nil {
		t.Fatalf("TransformFromSchema() unexpected error: %v", err)
	}
	// When source is a local dir (no UK_BASE), lib.path == lib.source == dir
	if lib.path != dir {
		t.Errorf("TransformFromSchema() path = %q, want %q", lib.path, dir)
	}
}

func TestTransformFromSchema_LocalDirWithUKBase(t *testing.T) {
	// Test case where source is local dir AND uk context is set
	dir := t.TempDir()

	ukCtx := &unikraft.Context{UK_BASE: t.TempDir()}
	ctx := unikraft.WithContext(context.Background(), ukCtx)

	lib, err := TransformFromSchema(ctx, "liblocal", map[string]interface{}{
		"source": dir,
	})
	if err != nil {
		t.Fatalf("TransformFromSchema() unexpected error: %v", err)
	}
	// path should be set to relative path from UK_BASE
	if lib.path == "" {
		t.Errorf("TransformFromSchema() path should be set when UK_BASE provided")
	}
}

func TestTransformMapFromSchema(t *testing.T) {
	tests := []struct {
		name    string
		data    interface{}
		wantErr bool
	}{
		{
			name: "valid map with string value",
			data: map[string]interface{}{
				"libfoo": "0.1.0",
			},
			wantErr: false,
		},
		{
			name: "valid map with map value",
			data: map[string]interface{}{
				"libbar": map[string]interface{}{
					"version": "1.0.0",
				},
			},
			wantErr: false,
		},
		{
			name: "map with invalid value type",
			data: map[string]interface{}{
				"libbad": 12345,
			},
			wantErr: true,
		},
		{
			name:    "non-map input returns error",
			data:    "not a map",
			wantErr: true,
		},
		{
			name:    "slice input returns error",
			data:    []string{"libfoo"},
			wantErr: true,
		},
		{
			name:    "nil input returns error",
			data:    nil,
			wantErr: true,
		},
		{
			name:    "empty map succeeds",
			data:    map[string]interface{}{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result, err := TransformMapFromSchema(ctx, tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("TransformMapFromSchema() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && result == nil {
				t.Errorf("TransformMapFromSchema() returned nil result without error")
			}
		})
	}
}
