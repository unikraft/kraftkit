// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package runtime

import (
	"context"
	"errors"
	"testing"
	"time"

	"kraftkit.sh/initrd"
	"kraftkit.sh/internal/tableprinter"
	"kraftkit.sh/kconfig"
	"kraftkit.sh/pack"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/arch"
	"kraftkit.sh/unikraft/plat"
)

// mockPackage implements pack.Package for testing delegation methods.
type mockPackage struct {
	id        string
	name      string
	version   string
	size      int64
	format    pack.PackageFormat
	pullErr   error
	pushErr   error
	saveErr   error
	unpackErr error
	exportErr error
	deleteErr error
}

func (m *mockPackage) ID() string                                         { return m.id }
func (m *mockPackage) Name() string                                       { return m.name }
func (m *mockPackage) Version() string                                    { return m.version }
func (m *mockPackage) Type() unikraft.ComponentType                       { return unikraft.ComponentTypeApp }
func (m *mockPackage) Size() int64                                        { return m.size }
func (m *mockPackage) Format() pack.PackageFormat                         { return m.format }
func (m *mockPackage) Metadata() interface{}                              { return nil }
func (m *mockPackage) Columns() []tableprinter.Column                     { return nil }
func (m *mockPackage) Push(_ context.Context, _ ...pack.PushOption) error { return m.pushErr }
func (m *mockPackage) Pull(_ context.Context, _ ...pack.PullOption) error { return m.pullErr }
func (m *mockPackage) Unpack(_ context.Context, _ string) error           { return m.unpackErr }
func (m *mockPackage) Save(_ context.Context) error                       { return m.saveErr }
func (m *mockPackage) Export(_ context.Context, _ string) error           { return m.exportErr }
func (m *mockPackage) Delete(_ context.Context) error                     { return m.deleteErr }
func (m *mockPackage) PulledAt(_ context.Context) (bool, time.Time, error) {
	return true, time.Time{}, nil
}
func (m *mockPackage) CreatedAt(_ context.Context) (time.Time, error) { return time.Time{}, nil }
func (m *mockPackage) UpdatedAt(_ context.Context) (time.Time, error) { return time.Time{}, nil }
func (m *mockPackage) String() string                                 { return m.name }

// mockTargetPackage implements both pack.Package and target.Target.
type mockTargetPackage struct {
	mockPackage
	kernel         string
	kernelDbg      string
	arch           arch.Architecture
	plat           plat.Platform
	command        []string
	configFilename string
}

func (m *mockTargetPackage) MarshalYAML() (interface{}, error) { return nil, nil }
func (m *mockTargetPackage) Source() string                    { return "" }
func (m *mockTargetPackage) Path() string                      { return "" }
func (m *mockTargetPackage) KConfigTree(_ context.Context, _ ...*kconfig.KeyValue) (*kconfig.KConfigFile, error) {
	return nil, nil
}
func (m *mockTargetPackage) KConfig() kconfig.KeyValueMap       { return nil }
func (m *mockTargetPackage) PrintInfo(_ context.Context) string { return "" }
func (m *mockTargetPackage) Architecture() arch.Architecture    { return m.arch }
func (m *mockTargetPackage) Platform() plat.Platform            { return m.plat }
func (m *mockTargetPackage) Kernel() string                     { return m.kernel }
func (m *mockTargetPackage) KernelDbg() string                  { return m.kernelDbg }
func (m *mockTargetPackage) Initrd() initrd.Initrd              { return nil }
func (m *mockTargetPackage) Roms() []string                     { return nil }
func (m *mockTargetPackage) Command() []string                  { return m.command }
func (m *mockTargetPackage) ConfigFilename() string             { return m.configFilename }
func (m *mockTargetPackage) SetKernelPath(_ string)             {}

func TestRuntime_Type(t *testing.T) {
	r := &Runtime{}
	if got := r.Type(); got != unikraft.ComponentTypeApp {
		t.Errorf("Type() = %v, want %v", got, unikraft.ComponentTypeApp)
	}
}

func TestRuntime_Name(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{
			name: "empty name",
			want: "",
		},
		{
			name: "named runtime",
			want: "base",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Runtime{name: tt.want}
			if got := r.Name(); got != tt.want {
				t.Errorf("Name() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRuntime_SetName(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantName    string
		wantVersion string
	}{
		{
			name:        "plain name without colon",
			input:       "base",
			wantName:    "base",
			wantVersion: "",
		},
		{
			name:        "name with version separated by colon",
			input:       "base:latest",
			wantName:    "base",
			wantVersion: "latest",
		},
		{
			name:        "full registry path without version",
			input:       "unikraft.org/base",
			wantName:    "unikraft.org/base",
			wantVersion: "",
		},
		{
			name:        "full registry path with version",
			input:       "unikraft.org/base:v0.14.0",
			wantName:    "unikraft.org/base",
			wantVersion: "v0.14.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Runtime{}
			r.SetName(tt.input)
			if r.name != tt.wantName {
				t.Errorf("SetName() name = %q, want %q", r.name, tt.wantName)
			}
			if r.version != tt.wantVersion {
				t.Errorf("SetName() version = %q, want %q", r.version, tt.wantVersion)
			}
		})
	}
}

func TestRuntime_Version(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    string
	}{
		{
			name:    "empty version",
			version: "",
			want:    "",
		},
		{
			name:    "semver version",
			version: "v0.14.0",
			want:    "v0.14.0",
		},
		{
			name:    "latest tag",
			version: "latest",
			want:    "latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Runtime{version: tt.version}
			if got := r.Version(); got != tt.want {
				t.Errorf("Version() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRuntime_Source(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "empty source",
			source: "",
			want:   "",
		},
		{
			name:   "oci source",
			source: "unikraft.org/base",
			want:   "unikraft.org/base",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Runtime{source: tt.source}
			if got := r.Source(); got != tt.want {
				t.Errorf("Source() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRuntime_MarshalYAML(t *testing.T) {
	tests := []struct {
		name        string
		runtime     Runtime
		wantNil     bool
		wantKeys    []string
		wantAbsKeys []string
	}{
		{
			name:    "empty runtime returns nil",
			runtime: Runtime{},
			wantNil: true,
		},
		{
			name:        "only name set",
			runtime:     Runtime{name: "base"},
			wantKeys:    []string{"name"},
			wantAbsKeys: []string{"version", "source"},
		},
		{
			name:        "only version set",
			runtime:     Runtime{version: "latest"},
			wantKeys:    []string{"version"},
			wantAbsKeys: []string{"name", "source"},
		},
		{
			name:        "only source set",
			runtime:     Runtime{source: "unikraft.org/base"},
			wantKeys:    []string{"source"},
			wantAbsKeys: []string{"name", "version"},
		},
		{
			name: "all fields set",
			runtime: Runtime{
				name:    "base",
				version: "latest",
				source:  "unikraft.org/base",
			},
			wantKeys: []string{"name", "version", "source"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.runtime.MarshalYAML()
			if err != nil {
				t.Fatalf("MarshalYAML() unexpected error: %v", err)
			}

			if tt.wantNil {
				if got != nil {
					t.Errorf("MarshalYAML() = %v, want nil", got)
				}
				return
			}

			m, ok := got.(map[string]interface{})
			if !ok {
				t.Fatalf("MarshalYAML() returned %T, want map[string]interface{}", got)
			}

			for _, key := range tt.wantKeys {
				if _, exists := m[key]; !exists {
					t.Errorf("MarshalYAML() missing key %q in result", key)
				}
			}

			for _, key := range tt.wantAbsKeys {
				if _, exists := m[key]; exists {
					t.Errorf("MarshalYAML() unexpected key %q in result", key)
				}
			}
		})
	}
}

func TestRuntime_String(t *testing.T) {
	mp := &mockPackage{name: "base"}
	r := &Runtime{pack: mp}
	if got := r.String(); got != "base" {
		t.Errorf("String() = %q, want %q", got, "base")
	}
}

func TestRuntime_PackDelegation(t *testing.T) {
	ctx := context.Background()
	mp := &mockPackage{
		id:      "test-id",
		name:    "base",
		version: "latest",
		size:    1024,
		format:  pack.PackageFormat("oci"),
	}
	r := &Runtime{pack: mp}

	t.Run("ID delegates to pack", func(t *testing.T) {
		if got := r.ID(); got != "test-id" {
			t.Errorf("ID() = %q, want %q", got, "test-id")
		}
	})

	t.Run("Columns delegates to pack", func(t *testing.T) {
		if got := r.Columns(); got != nil {
			t.Errorf("Columns() = %v, want nil", got)
		}
	})

	t.Run("Metadata delegates to pack", func(t *testing.T) {
		if got := r.Metadata(); got != nil {
			t.Errorf("Metadata() = %v, want nil", got)
		}
	})

	t.Run("Size delegates to pack", func(t *testing.T) {
		if got := r.Size(); got != 1024 {
			t.Errorf("Size() = %d, want 1024", got)
		}
	})

	t.Run("Format delegates to pack", func(t *testing.T) {
		if got := r.Format(); got != "oci" {
			t.Errorf("Format() = %q, want %q", got, "oci")
		}
	})

	t.Run("Unpack delegates to pack", func(t *testing.T) {
		if err := r.Unpack(ctx, "/tmp/dir"); err != nil {
			t.Errorf("Unpack() unexpected error: %v", err)
		}
		mp.unpackErr = errors.New("unpack failed")
		if err := r.Unpack(ctx, "/tmp/dir"); err == nil {
			t.Errorf("Unpack() expected error, got nil")
		}
		mp.unpackErr = nil
	})

	t.Run("Pull delegates to pack", func(t *testing.T) {
		if err := r.Pull(ctx); err != nil {
			t.Errorf("Pull() unexpected error: %v", err)
		}
	})

	t.Run("PulledAt delegates to pack", func(t *testing.T) {
		pulled, _, err := r.PulledAt(ctx)
		if err != nil {
			t.Errorf("PulledAt() unexpected error: %v", err)
		}
		if !pulled {
			t.Errorf("PulledAt() pulled = false, want true")
		}
	})

	t.Run("CreatedAt delegates to pack", func(t *testing.T) {
		if _, err := r.CreatedAt(ctx); err != nil {
			t.Errorf("CreatedAt() unexpected error: %v", err)
		}
	})

	t.Run("UpdatedAt delegates to pack", func(t *testing.T) {
		if _, err := r.UpdatedAt(ctx); err != nil {
			t.Errorf("UpdatedAt() unexpected error: %v", err)
		}
	})

	t.Run("Delete delegates to pack", func(t *testing.T) {
		if err := r.Delete(ctx); err != nil {
			t.Errorf("Delete() unexpected error: %v", err)
		}
	})

	t.Run("Save delegates to pack", func(t *testing.T) {
		if err := r.Save(ctx); err != nil {
			t.Errorf("Save() unexpected error: %v", err)
		}
	})

	t.Run("Export delegates to pack", func(t *testing.T) {
		if err := r.Export(ctx, "/tmp/export"); err != nil {
			t.Errorf("Export() unexpected error: %v", err)
		}
	})

	t.Run("Export returns error when pack is nil", func(t *testing.T) {
		rNil := &Runtime{}
		if err := rNil.Export(ctx, "/tmp/export"); err == nil {
			t.Errorf("Export() expected error for nil pack, got nil")
		}
	})
}

func TestRuntime_Push_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Push() expected panic, got none")
		}
	}()
	rt := &Runtime{}
	_ = rt.Push(context.Background())
}

func TestRuntime_TargetDelegation(t *testing.T) {
	mtp := &mockTargetPackage{
		mockPackage:    mockPackage{name: "base", version: "latest"},
		kernel:         "/boot/kernel",
		kernelDbg:      "/boot/kernel.dbg",
		command:        []string{"nginx", "-g", "daemon off;"},
		configFilename: ".config.x86_64-qemu-base",
	}
	r := &Runtime{pack: mtp}

	t.Run("Architecture delegates to target pack", func(t *testing.T) {
		if got := r.Architecture(); got != nil {
			t.Errorf("Architecture() = %v, want nil (mock returns nil)", got)
		}
	})

	t.Run("Platform delegates to target pack", func(t *testing.T) {
		if got := r.Platform(); got != nil {
			t.Errorf("Platform() = %v, want nil (mock returns nil)", got)
		}
	})

	t.Run("Kernel delegates to target pack", func(t *testing.T) {
		if got := r.Kernel(); got != "/boot/kernel" {
			t.Errorf("Kernel() = %q, want %q", got, "/boot/kernel")
		}
	})

	t.Run("Kernel returns runtime.kernel when set", func(t *testing.T) {
		rKernel := &Runtime{kernel: "/custom/kernel", pack: mtp}
		if got := rKernel.Kernel(); got != "/custom/kernel" {
			t.Errorf("Kernel() = %q, want %q", got, "/custom/kernel")
		}
	})

	t.Run("KernelDbg delegates to target pack", func(t *testing.T) {
		if got := r.KernelDbg(); got != "/boot/kernel.dbg" {
			t.Errorf("KernelDbg() = %q, want %q", got, "/boot/kernel.dbg")
		}
	})

	t.Run("Initrd delegates to target pack", func(t *testing.T) {
		if got := r.Initrd(); got != nil {
			t.Errorf("Initrd() = %v, want nil", got)
		}
	})

	t.Run("Command delegates to target pack", func(t *testing.T) {
		got := r.Command()
		if len(got) != 3 || got[0] != "nginx" {
			t.Errorf("Command() = %v, want [nginx -g daemon off;]", got)
		}
	})

	t.Run("ConfigFilename delegates to target pack", func(t *testing.T) {
		if got := r.ConfigFilename(); got != ".config.x86_64-qemu-base" {
			t.Errorf("ConfigFilename() = %q, want %q", got, ".config.x86_64-qemu-base")
		}
	})
}

func TestRuntime_NonTargetPackFallbacks(t *testing.T) {
	// When pack does not implement target.Target, methods return zero values.
	mp := &mockPackage{name: "base"}
	r := &Runtime{pack: mp}

	if got := r.Architecture(); got != nil {
		t.Errorf("Architecture() = %v, want nil", got)
	}
	if got := r.Platform(); got != nil {
		t.Errorf("Platform() = %v, want nil", got)
	}
	if got := r.Kernel(); got != "" {
		t.Errorf("Kernel() = %q, want empty", got)
	}
	if got := r.KernelDbg(); got != "" {
		t.Errorf("KernelDbg() = %q, want empty", got)
	}
	if got := r.Initrd(); got != nil {
		t.Errorf("Initrd() = %v, want nil", got)
	}
	if got := r.Command(); got != nil {
		t.Errorf("Command() = %v, want nil", got)
	}
	if got := r.ConfigFilename(); got != "" {
		t.Errorf("ConfigFilename() = %q, want empty", got)
	}
}

func TestRuntime_AddRootfs(t *testing.T) {
	r := &Runtime{}
	if err := r.AddRootfs("/path/to/rootfs"); err != nil {
		t.Errorf("AddRootfs() unexpected error: %v", err)
	}
}

func TestTransformFromSchema_String(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantName    string
		wantVersion string
		wantSource  string
		wantKernel  string
		wantErr     bool
	}{
		{
			name:     "plain name only",
			input:    "base",
			wantName: "base",
		},
		{
			name:        "name with tag",
			input:       "base:latest",
			wantName:    "base",
			wantVersion: "latest",
		},
		{
			name:    "too many colons errors",
			input:   "a:b:c",
			wantErr: true,
		},
		{
			name:        "oci scheme with tag",
			input:       "oci://unikraft.org/base:v0.14.0",
			wantSource:  "unikraft.org/base",
			wantVersion: "v0.14.0",
		},
		{
			name:       "oci scheme without tag",
			input:      "oci://unikraft.org/base",
			wantSource: "unikraft.org/base",
		},
		{
			name:       "kernel scheme",
			input:      "kernel:///path/to/kernel",
			wantKernel: "/path/to/kernel",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := TransformFromSchema(context.Background(), tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("TransformFromSchema() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("TransformFromSchema() unexpected error: %v", err)
			}

			r, ok := got.(Runtime)
			if !ok {
				t.Fatalf("TransformFromSchema() returned %T, want Runtime", got)
			}

			if r.name != tt.wantName {
				t.Errorf("name = %q, want %q", r.name, tt.wantName)
			}
			if r.version != tt.wantVersion {
				t.Errorf("version = %q, want %q", r.version, tt.wantVersion)
			}
			if r.source != tt.wantSource {
				t.Errorf("source = %q, want %q", r.source, tt.wantSource)
			}
			if r.kernel != tt.wantKernel {
				t.Errorf("kernel = %q, want %q", r.kernel, tt.wantKernel)
			}
		})
	}
}

func TestTransformFromSchema_Map(t *testing.T) {
	tests := []struct {
		name        string
		input       map[string]interface{}
		wantSource  string
		wantVersion string
		wantKconfig bool
		wantErr     bool
	}{
		{
			name: "map with full url source",
			input: map[string]interface{}{
				"source": "https://github.com/unikraft/app-elfloader",
			},
			wantSource: "https://github.com/unikraft/app-elfloader",
		},
		{
			name: "map with version string",
			input: map[string]interface{}{
				"version": "latest",
			},
			wantVersion: "latest",
		},
		{
			name: "map with version integer converted to string",
			input: map[string]interface{}{
				"version": 15,
			},
			wantVersion: "15",
		},
		{
			name: "map with kconfig as map",
			input: map[string]interface{}{
				"kconfig": map[string]interface{}{
					"CONFIG_X": "y",
				},
			},
			wantKconfig: true,
		},
		{
			name: "map with kconfig as slice",
			input: map[string]interface{}{
				"kconfig": []interface{}{"CONFIG_X=y"},
			},
			wantKconfig: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := TransformFromSchema(context.Background(), tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("TransformFromSchema() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("TransformFromSchema() unexpected error: %v", err)
			}

			r, ok := got.(Runtime)
			if !ok {
				t.Fatalf("TransformFromSchema() returned %T, want Runtime", got)
			}

			if r.source != tt.wantSource {
				t.Errorf("source = %q, want %q", r.source, tt.wantSource)
			}
			if r.version != tt.wantVersion {
				t.Errorf("version = %q, want %q", r.version, tt.wantVersion)
			}
			if tt.wantKconfig && r.kconfig == nil {
				t.Errorf("kconfig = nil, want non-nil")
			}
		})
	}
}

func TestRuntimeOptions(t *testing.T) {
	t.Run("WithName", func(t *testing.T) {
		r := &Runtime{}
		if err := WithName("base")(r); err != nil {
			t.Fatalf("WithName() error: %v", err)
		}
		if r.name != "base" {
			t.Errorf("name = %q, want %q", r.name, "base")
		}
	})

	t.Run("WithSource", func(t *testing.T) {
		r := &Runtime{}
		if err := WithSource("unikraft.org/base")(r); err != nil {
			t.Fatalf("WithSource() error: %v", err)
		}
		if r.source != "unikraft.org/base" {
			t.Errorf("source = %q, want %q", r.source, "unikraft.org/base")
		}
	})

	t.Run("WithRootfs", func(t *testing.T) {
		r := &Runtime{}
		if err := WithRootfs("/tmp/rootfs")(r); err != nil {
			t.Fatalf("WithRootfs() error: %v", err)
		}
		if r.rootfs != "/tmp/rootfs" {
			t.Errorf("rootfs = %q, want %q", r.rootfs, "/tmp/rootfs")
		}
	})

	t.Run("WithKernel", func(t *testing.T) {
		r := &Runtime{}
		if err := WithKernel("/path/to/kernel")(r); err != nil {
			t.Fatalf("WithKernel() error: %v", err)
		}
		if r.kernel != "/path/to/kernel" {
			t.Errorf("kernel = %q, want %q", r.kernel, "/path/to/kernel")
		}
	})

	t.Run("WithPlatform", func(t *testing.T) {
		r := &Runtime{}
		if err := WithPlatform("qemu")(r); err != nil {
			t.Fatalf("WithPlatform() error: %v", err)
		}
		if r.platform != "qemu" {
			t.Errorf("platform = %q, want %q", r.platform, "qemu")
		}
	})

	t.Run("WithArchitecture", func(t *testing.T) {
		r := &Runtime{}
		if err := WithArchitecture("x86_64")(r); err != nil {
			t.Fatalf("WithArchitecture() error: %v", err)
		}
		if r.architecture != "x86_64" {
			t.Errorf("architecture = %q, want %q", r.architecture, "x86_64")
		}
	})
}

func TestConstants(t *testing.T) {
	if PrebuiltRegistry != "unikraft.org" {
		t.Errorf("PrebuiltRegistry = %q, want %q", PrebuiltRegistry, "unikraft.org")
	}
	if DefaultRuntime != "unikraft.org/base:latest" {
		t.Errorf("DefaultRuntime = %q, want %q", DefaultRuntime, "unikraft.org/base:latest")
	}
	if DefaultKraftCloudRuntime != "index.unikraft.io/official/base:latest" {
		t.Errorf("DefaultKraftCloudRuntime = %q, want %q", DefaultKraftCloudRuntime, "index.unikraft.io/official/base:latest")
	}
}
