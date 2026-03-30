// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"kraftkit.sh/initrd"
	uklib "kraftkit.sh/unikraft/lib"
)

func Test_sniffKraftfileSpecVersion(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "spec field with v prefix",
			input: "spec: v0.7\nruntime: base:latest\n",
			want:  "v0.7",
		},
		{
			name:  "spec field without v prefix",
			input: "spec: 0.7\nruntime: base:latest\n",
			want:  "v0.7",
		},
		{
			name:  "specification field",
			input: "specification: v0.6\nruntime: base:latest\n",
			want:  "v0.6",
		},
		{
			name:    "missing spec attribute",
			input:   "runtime: base:latest\n",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := sniffKraftfileSpecVersion([]byte(tt.input))
			if (err != nil) != tt.wantErr {
				t.Fatalf("sniffKraftfileSpecVersion() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr {
				return
			}

			if got != tt.want {
				t.Errorf("sniffKraftfileSpecVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNewProjectFromOptionsDispatchesBySpecVersion(t *testing.T) {
	tests := []struct {
		name       string
		content    string
		wantLoader ProjectLoader
		wantSpec   string
	}{
		{
			name: "legacy spec uses legacy loader",
			content: `
spec: v0.6
runtime: base:latest
rootfs: ./Dockerfile
cmd: ["/app"]
`,
			wantLoader: ProjectLoaderLegacy,
			wantSpec:   "v0.6",
		},
		{
			name: "v0.7 spec uses v0.7 loader",
			content: `
spec: v0.7
runtime: base:latest
`,
			wantLoader: ProjectLoaderV07,
			wantSpec:   "v0.7",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			project := mustProjectFromBytes(t, t.TempDir(), tt.content)

			if project.LoaderKind() != tt.wantLoader {
				t.Errorf("LoaderKind() = %q, want %q", project.LoaderKind(), tt.wantLoader)
			}

			if project.SpecVersion() != tt.wantSpec {
				t.Errorf("SpecVersion() = %q, want %q", project.SpecVersion(), tt.wantSpec)
			}
		})
	}
}

func TestV07ProjectNameSemantics(t *testing.T) {
	tests := []struct {
		name    string
		content string
		envKey  string
		envVal  string
		want    string
	}{
		{
			name: "does not interpolate compose style variables",
			content: `
spec: v0.7
name: ${APP_NAME}
runtime: base:latest
`,
			envKey: "APP_NAME",
			envVal: "expanded",
			want:   "${APP_NAME}",
		},
		{
			name: "does not normalize project name at load time",
			content: `
spec: v0.7
name: My App_01
runtime: base:latest
`,
			want: "My App_01",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envKey != "" {
				t.Setenv(tt.envKey, tt.envVal)
			}

			project := mustProjectFromBytes(t, t.TempDir(), tt.content)
			if project.Name() != tt.want {
				t.Errorf("Name() = %q, want %q", project.Name(), tt.want)
			}
		})
	}
}

func TestV07ComponentStringsDoNotUseFilesystemHeuristics(t *testing.T) {
	workdir := t.TempDir()
	existingPath := filepath.Join(workdir, "stable")
	if err := os.Mkdir(existingPath, 0o755); err != nil {
		t.Fatalf("could not create sentinel directory: %v", err)
	}

	project := mustProjectFromBytes(t, workdir, `
spec: v0.7
name: demo
unikraft: `+existingPath+`
libraries:
  libfoo: `+existingPath+`
`)

	unikraftConfig := project.Unikraft(context.Background())
	if unikraftConfig == nil {
		t.Fatal("expected unikraft config to be present")
	}

	if unikraftConfig.Version() != existingPath {
		t.Errorf("Unikraft().Version() = %q, want %q", unikraftConfig.Version(), existingPath)
	}

	if unikraftConfig.Source() != "" {
		t.Errorf("Unikraft().Source() = %q, want empty", unikraftConfig.Source())
	}

	libraries, err := project.Libraries(context.Background())
	if err != nil {
		t.Fatalf("Libraries() error = %v", err)
	}

	library, ok := libraries["libfoo"]
	if !ok {
		t.Fatal("expected libfoo to be present")
	}

	if library.Version() != existingPath {
		t.Errorf("library.Version() = %q, want %q", library.Version(), existingPath)
	}

	if library.Source() != "" {
		t.Errorf("library.Source() = %q, want empty", library.Source())
	}
}

func TestV07RuntimeReferenceIsPreserved(t *testing.T) {
	project := mustProjectFromBytes(t, t.TempDir(), `
spec: v0.7
runtime: index.unikraft.io/official/base:latest
`)

	if project.Runtime() == nil {
		t.Fatal("expected runtime to be present")
	}

	if project.Runtime().Name() != "index.unikraft.io/official/base:latest" {
		t.Errorf("Runtime().Name() = %q, want full OCI ref", project.Runtime().Name())
	}

	if project.Runtime().Version() != "" {
		t.Errorf("Runtime().Version() = %q, want empty", project.Runtime().Version())
	}
}

func TestV07TemplateSourceMapsToTemplateFields(t *testing.T) {
	project := mustProjectFromBytes(t, t.TempDir(), `
spec: v0.7
template:
  source: https://github.com/unikraft/catalog.git
  version: stable
unikraft: stable
`)

	if project.Template() == nil {
		t.Fatal("expected template to be present")
	}

	if project.Template().Name() != "catalog" {
		t.Errorf("Template().Name() = %q, want %q", project.Template().Name(), "catalog")
	}

	if project.Template().Source() != "https://github.com/unikraft/catalog.git" {
		t.Errorf("Template().Source() = %q, want original source", project.Template().Source())
	}

	if project.Template().Version() != "stable" {
		t.Errorf("Template().Version() = %q, want %q", project.Template().Version(), "stable")
	}

	if len(project.Template().KConfig()) != 0 {
		t.Errorf("Template().KConfig() len = %d, want 0", len(project.Template().KConfig()))
	}
}

func TestV07RootfsFormatMapsToInitrdFsType(t *testing.T) {
	project := mustProjectFromBytes(t, t.TempDir(), `
spec: v0.7
runtime: base:latest
rootfs:
  source: ./rootfs
  format: erofs
`)

	if project.Rootfs() != "./rootfs" {
		t.Errorf("Rootfs() = %q, want %q", project.Rootfs(), "./rootfs")
	}

	if project.InitrdFsType() != initrd.FsTypeErofs {
		t.Errorf("InitrdFsType() = %q, want %q", project.InitrdFsType(), initrd.FsTypeErofs)
	}
}

func TestV07VolumesMapModeAndReadonly(t *testing.T) {
	project := mustProjectFromBytes(t, t.TempDir(), `
spec: v0.7
runtime: base:latest
volumes:
  - source: ./data
    destination: /mnt/data
    mode: ro
    readonly: true
`)

	volumes := project.Volumes()
	if len(volumes) != 1 {
		t.Fatalf("len(Volumes()) = %d, want 1", len(volumes))
	}

	if volumes[0].Source() != "./data" {
		t.Errorf("volumes[0].Source() = %q, want %q", volumes[0].Source(), "./data")
	}

	if volumes[0].Destination() != "/mnt/data" {
		t.Errorf("volumes[0].Destination() = %q, want %q", volumes[0].Destination(), "/mnt/data")
	}

	if volumes[0].Mode() != "ro" {
		t.Errorf("volumes[0].Mode() = %q, want %q", volumes[0].Mode(), "ro")
	}

	if !volumes[0].ReadOnly() {
		t.Error("volumes[0].ReadOnly() = false, want true")
	}
}

func TestV07MutationOperationsAreRejected(t *testing.T) {
	project := mustProjectFromBytes(t, t.TempDir(), `
spec: v0.7
runtime: base:latest
`)

	tests := []struct {
		name string
		run  func(Application) error
	}{
		{
			name: "save",
			run: func(project Application) error {
				return project.Save(context.Background())
			},
		},
		{
			name: "add library",
			run: func(project Application) error {
				return project.AddLibrary(context.Background(), uklib.LibraryConfig{})
			},
		},
		{
			name: "remove library",
			run: func(project Application) error {
				return project.RemoveLibrary(context.Background(), "libfoo")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run(project)
			if !errors.Is(err, ErrProjectMutationNotSupported) {
				t.Errorf("operation error = %v, want %v", err, ErrProjectMutationNotSupported)
			}
		})
	}
}

func mustProjectFromBytes(t *testing.T, workdir string, content string) Application {
	t.Helper()

	project, err := NewProjectFromOptions(
		context.Background(),
		WithProjectWorkdir(workdir),
		WithProjectKraftfileFromBytes([]byte(content)),
	)
	if err != nil {
		t.Fatalf("could not create project: %v", err)
	}

	return project
}
