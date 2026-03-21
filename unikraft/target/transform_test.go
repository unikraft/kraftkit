// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package target

import (
	"context"
	"strings"
	"testing"

	"kraftkit.sh/unikraft"
)

func TestTransformFromSchema_string(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name         string
		input        string
		wantPlatName string
		wantArchName string
		wantErr      bool
	}{
		{
			name:         "valid platform/arch string",
			input:        "kvm/x86_64",
			wantPlatName: "qemu",
			wantArchName: "x86_64",
		},
		{
			name:         "xen/arm64 string",
			input:        "xen/arm64",
			wantPlatName: "xen",
			wantArchName: "arm64",
		},
		{
			name:    "string without slash returns error",
			input:   "kvm",
			wantErr: true,
		},
		{
			name:    "empty string returns error",
			input:   "",
			wantErr: true,
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

			tc, ok := got.(TargetConfig)
			if !ok {
				t.Fatalf("result type = %T, want TargetConfig", got)
			}
			if tc.Platform().Name() != tt.wantPlatName {
				t.Errorf("platform = %q, want %q", tc.Platform().Name(), tt.wantPlatName)
			}
			if tc.Architecture().Name() != tt.wantArchName {
				t.Errorf("architecture = %q, want %q", tc.Architecture().Name(), tt.wantArchName)
			}
		})
	}
}

func TestTransformFromSchema_map(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name         string
		input        map[string]interface{}
		wantName     string
		wantPlatName string
		wantArchName string
		wantKernel   string
		wantErr      bool
	}{
		{
			name: "map with platform and architecture keys",
			input: map[string]interface{}{
				"platform":     "qemu",
				"architecture": "x86_64",
			},
			wantPlatName: "qemu",
			wantArchName: "x86_64",
		},
		{
			name: "map with plat and arch aliases",
			input: map[string]interface{}{
				"plat": "xen",
				"arch": "arm64",
			},
			wantPlatName: "xen",
			wantArchName: "arm64",
		},
		{
			name: "map with name key",
			input: map[string]interface{}{
				"name":         "myapp",
				"platform":     "qemu",
				"architecture": "x86_64",
			},
			wantName:     "myapp",
			wantPlatName: "qemu",
			wantArchName: "x86_64",
		},
		{
			name: "map with kernel key sets name",
			input: map[string]interface{}{
				"kernel":       "mykernel",
				"platform":     "qemu",
				"architecture": "x86_64",
			},
			wantName:     "mykernel",
			wantPlatName: "qemu",
			wantArchName: "x86_64",
		},
		{
			name: "map with output key sets kernel path",
			input: map[string]interface{}{
				"platform":     "qemu",
				"architecture": "x86_64",
				"output":       "/build/myapp",
			},
			wantPlatName: "qemu",
			wantArchName: "x86_64",
			wantKernel:   "/build/myapp",
		},
		{
			name: "map with kconfig as map",
			input: map[string]interface{}{
				"platform":     "qemu",
				"architecture": "x86_64",
				"kconfig": map[string]interface{}{
					"CONFIG_EXAMPLE": "y",
				},
			},
			wantPlatName: "qemu",
			wantArchName: "x86_64",
		},
		{
			name: "map with kconfig as slice",
			input: map[string]interface{}{
				"platform":     "qemu",
				"architecture": "x86_64",
				"kconfig":      []interface{}{"CONFIG_EXAMPLE=y"},
			},
			wantPlatName: "qemu",
			wantArchName: "x86_64",
		},
		{
			name: "platform with slash resolves arch",
			input: map[string]interface{}{
				"platform": "qemu/x86_64",
			},
			wantPlatName: "qemu",
			wantArchName: "x86_64",
		},
		{
			name: "name must be string",
			input: map[string]interface{}{
				"name":         123,
				"platform":     "qemu",
				"architecture": "x86_64",
			},
			wantErr: true,
		},
		{
			name: "platform must be string",
			input: map[string]interface{}{
				"platform":     123,
				"architecture": "x86_64",
			},
			wantErr: true,
		},
		{
			name: "output must be string",
			input: map[string]interface{}{
				"platform":     "qemu",
				"architecture": "x86_64",
				"output":       123,
			},
			wantErr: true,
		},
		{
			name: "kernel must be string",
			input: map[string]interface{}{
				"platform":     "qemu",
				"architecture": "x86_64",
				"kernel":       123,
			},
			wantErr: true,
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

			tc, ok := got.(TargetConfig)
			if !ok {
				t.Fatalf("result type = %T, want TargetConfig", got)
			}
			if tt.wantName != "" && tc.Name() != tt.wantName {
				t.Errorf("name = %q, want %q", tc.Name(), tt.wantName)
			}
			if tt.wantPlatName != "" && tc.Platform().Name() != tt.wantPlatName {
				t.Errorf("platform = %q, want %q", tc.Platform().Name(), tt.wantPlatName)
			}
			if tt.wantArchName != "" && tc.Architecture().Name() != tt.wantArchName {
				t.Errorf("architecture = %q, want %q", tc.Architecture().Name(), tt.wantArchName)
			}
			if tt.wantKernel != "" && tc.Kernel() != tt.wantKernel {
				t.Errorf("kernel = %q, want %q", tc.Kernel(), tt.wantKernel)
			}
		})
	}
}

func TestTransformFromSchema_invalidType(t *testing.T) {
	ctx := context.Background()

	_, err := TransformFromSchema(ctx, 42)
	if err == nil {
		t.Error("expected error for invalid type, got nil")
	}
}

func TestTransformFromSchema_withUKContext(t *testing.T) {
	buildDir := t.TempDir()
	ctx := unikraft.WithContext(context.Background(), &unikraft.Context{
		UK_NAME:   "myapp",
		BUILD_DIR: buildDir,
	})

	got, err := TransformFromSchema(ctx, "qemu/x86_64")
	if err != nil {
		t.Fatalf("TransformFromSchema() error = %v", err)
	}

	tc, ok := got.(TargetConfig)
	if !ok {
		t.Fatalf("result type = %T, want TargetConfig", got)
	}

	if tc.Name() != "myapp" {
		t.Errorf("name = %q, want %q", tc.Name(), "myapp")
	}
	if !strings.HasPrefix(tc.Kernel(), buildDir) {
		t.Errorf("kernel path %q should be under buildDir %q", tc.Kernel(), buildDir)
	}
	if !strings.HasPrefix(tc.KernelDbg(), buildDir) {
		t.Errorf("kernelDbg path %q should be under buildDir %q", tc.KernelDbg(), buildDir)
	}
	if !strings.HasSuffix(tc.KernelDbg(), ".dbg") {
		t.Errorf("kernelDbg path %q should end with .dbg", tc.KernelDbg())
	}
}

func TestTransformFromSchema_withPresetKernel(t *testing.T) {
	buildDir := t.TempDir()
	ctx := unikraft.WithContext(context.Background(), &unikraft.Context{
		UK_NAME:   "myapp",
		BUILD_DIR: buildDir,
	})

	input := map[string]interface{}{
		"platform":     "qemu",
		"architecture": "x86_64",
		"output":       "/custom/kernel/path",
	}

	got, err := TransformFromSchema(ctx, input)
	if err != nil {
		t.Fatalf("TransformFromSchema() error = %v", err)
	}

	tc, ok := got.(TargetConfig)
	if !ok {
		t.Fatalf("result type = %T, want TargetConfig", got)
	}

	// When output is preset, BUILD_DIR should not override it
	if tc.Kernel() != "/custom/kernel/path" {
		t.Errorf("kernel = %q, want %q", tc.Kernel(), "/custom/kernel/path")
	}
}
