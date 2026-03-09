// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package target

import (
	"context"
	"testing"

	"kraftkit.sh/unikraft"
)

// ---------------------------------------------------------------------------
// TransformFromSchema — string input
// ---------------------------------------------------------------------------

func TestTransformFromSchema_String(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantPlat string
		wantArch string
		wantErr  bool
	}{
		{
			// "kvm" is aliased to "qemu" by mplatform.PlatformsByName()
			name:     "kvm/x86_64 is aliased to qemu",
			input:    "kvm/x86_64",
			wantPlat: "qemu",
			wantArch: "x86_64",
		},
		{
			name:     "qemu/arm64",
			input:    "qemu/arm64",
			wantPlat: "qemu",
			wantArch: "arm64",
		},
		{
			name:     "linuxu/x86_64 stays as linuxu",
			input:    "linuxu/x86_64",
			wantPlat: "linuxu",
			wantArch: "x86_64",
		},
		{
			name:     "xen/arm64",
			input:    "xen/arm64",
			wantPlat: "xen",
			wantArch: "arm64",
		},
		{
			name:    "missing slash returns error",
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
			got, err := TransformFromSchema(context.Background(), tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("TransformFromSchema(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if err != nil {
				return
			}
			tc, ok := got.(TargetConfig)
			if !ok {
				t.Fatalf("TransformFromSchema returned %T, want TargetConfig", got)
			}
			if tc.Platform().Name() != tt.wantPlat {
				t.Errorf("Platform().Name() = %q, want %q", tc.Platform().Name(), tt.wantPlat)
			}
			if tc.Architecture().Name() != tt.wantArch {
				t.Errorf("Architecture().Name() = %q, want %q", tc.Architecture().Name(), tt.wantArch)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TransformFromSchema — map input
// ---------------------------------------------------------------------------

func TestTransformFromSchema_Map(t *testing.T) {
	t.Run("architecture and platform keys", func(t *testing.T) {
		input := map[string]interface{}{
			"architecture": "x86_64",
			"platform":     "linuxu",
		}
		got, err := TransformFromSchema(context.Background(), input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		tc := got.(TargetConfig)
		if tc.Architecture().Name() != "x86_64" {
			t.Errorf("Architecture = %q, want x86_64", tc.Architecture().Name())
		}
		if tc.Platform().Name() != "linuxu" {
			t.Errorf("Platform = %q, want linuxu", tc.Platform().Name())
		}
	})

	t.Run("arch and plat alias keys", func(t *testing.T) {
		input := map[string]interface{}{
			"arch": "arm64",
			"plat": "xen",
		}
		got, err := TransformFromSchema(context.Background(), input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		tc := got.(TargetConfig)
		if tc.Architecture().Name() != "arm64" {
			t.Errorf("Architecture = %q, want arm64", tc.Architecture().Name())
		}
		if tc.Platform().Name() != "xen" {
			t.Errorf("Platform = %q, want xen", tc.Platform().Name())
		}
	})

	t.Run("platform with inline arch override", func(t *testing.T) {
		input := map[string]interface{}{
			"platform": "linuxu/x86_64",
		}
		got, err := TransformFromSchema(context.Background(), input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		tc := got.(TargetConfig)
		if tc.Platform().Name() != "linuxu" {
			t.Errorf("Platform = %q, want linuxu", tc.Platform().Name())
		}
		if tc.Architecture().Name() != "x86_64" {
			t.Errorf("Architecture = %q, want x86_64", tc.Architecture().Name())
		}
	})

	t.Run("name key sets target name", func(t *testing.T) {
		input := map[string]interface{}{
			"name":         "myapp",
			"architecture": "x86_64",
			"platform":     "linuxu",
		}
		got, err := TransformFromSchema(context.Background(), input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		tc := got.(TargetConfig)
		if tc.Name() != "myapp" {
			t.Errorf("Name = %q, want myapp", tc.Name())
		}
	})

	t.Run("output key sets kernel path", func(t *testing.T) {
		input := map[string]interface{}{
			"architecture": "x86_64",
			"platform":     "linuxu",
			"output":       "/build/myapp_linuxu-x86_64",
		}
		got, err := TransformFromSchema(context.Background(), input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		tc := got.(TargetConfig)
		if tc.Kernel() != "/build/myapp_linuxu-x86_64" {
			t.Errorf("Kernel = %q, want /build/myapp_linuxu-x86_64", tc.Kernel())
		}
	})

	t.Run("kconfig as map", func(t *testing.T) {
		input := map[string]interface{}{
			"architecture": "x86_64",
			"platform":     "linuxu",
			"kconfig": map[string]interface{}{
				"CONFIG_EXAMPLE": "y",
			},
		}
		got, err := TransformFromSchema(context.Background(), input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		tc := got.(TargetConfig)
		if tc.kconfig == nil {
			t.Error("kconfig should not be nil after map input")
		}
	})

	t.Run("kconfig as slice", func(t *testing.T) {
		input := map[string]interface{}{
			"architecture": "x86_64",
			"platform":     "linuxu",
			"kconfig":      []interface{}{"CONFIG_EXAMPLE=y"},
		}
		got, err := TransformFromSchema(context.Background(), input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		tc := got.(TargetConfig)
		if tc.kconfig == nil {
			t.Error("kconfig should not be nil after slice input")
		}
	})

	t.Run("invalid type for name returns error", func(t *testing.T) {
		input := map[string]interface{}{
			"name":         123,
			"architecture": "x86_64",
			"platform":     "linuxu",
		}
		_, err := TransformFromSchema(context.Background(), input)
		if err == nil {
			t.Error("expected error for non-string name, got nil")
		}
	})

	t.Run("invalid type for platform returns error", func(t *testing.T) {
		input := map[string]interface{}{
			"architecture": "x86_64",
			"platform":     123,
		}
		_, err := TransformFromSchema(context.Background(), input)
		if err == nil {
			t.Error("expected error for non-string platform, got nil")
		}
	})
}

// ---------------------------------------------------------------------------
// TransformFromSchema — context-aware behaviour
// ---------------------------------------------------------------------------

func TestTransformFromSchema_WithContext(t *testing.T) {
	t.Run("UK_NAME from context sets target name", func(t *testing.T) {
		ctx := unikraft.WithContext(context.Background(), &unikraft.Context{
			UK_NAME: "ctxapp",
		})
		input := map[string]interface{}{
			"architecture": "x86_64",
			"platform":     "linuxu",
		}
		got, err := TransformFromSchema(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		tc := got.(TargetConfig)
		if tc.Name() != "ctxapp" {
			t.Errorf("Name = %q, want ctxapp", tc.Name())
		}
	})

	t.Run("BUILD_DIR from context populates kernel and kernelDbg paths", func(t *testing.T) {
		ctx := unikraft.WithContext(context.Background(), &unikraft.Context{
			UK_NAME:   "myapp",
			BUILD_DIR: "/tmp/build",
		})
		input := map[string]interface{}{
			"architecture": "x86_64",
			"platform":     "linuxu",
		}
		got, err := TransformFromSchema(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		tc := got.(TargetConfig)
		if tc.Kernel() == "" {
			t.Error("Kernel path should be set when BUILD_DIR is present in context")
		}
		if tc.KernelDbg() == "" {
			t.Error("KernelDbg path should be set when BUILD_DIR is present in context")
		}
	})
}

// ---------------------------------------------------------------------------
// TransformFromSchema — unsupported type
// ---------------------------------------------------------------------------

func TestTransformFromSchema_InvalidType(t *testing.T) {
	_, err := TransformFromSchema(context.Background(), 42)
	if err == nil {
		t.Error("expected error for unsupported input type int, got nil")
	}
}
