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
	"kraftkit.sh/unikraft/lib"
	kraftfilev07 "unikraft.com/x/kraftfile"
)

func Test_parseKraftfileSpecVersion(t *testing.T) {
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
			name:  "missing spec attribute defaults to legacy dispatch",
			input: "runtime: base:latest\n",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseKraftfileSpecVersion([]byte(tt.input))
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseKraftfileSpecVersion() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr {
				return
			}

			if got != tt.want {
				t.Errorf("parseKraftfileSpecVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}

func Test_NewProjectFromOptions_SpecVersionDispatch(t *testing.T) {
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
			wantLoader: ProjectLoaderV06,
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
		{
			name: "missing spec uses legacy loader",
			content: `
runtime: base:latest
rootfs: ./Dockerfile
cmd: ["/app"]
`,
			wantLoader: ProjectLoaderV06,
			wantSpec:   "",
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

func Test_NewProjectFromOptionsV07_ProjectNameSemantics(t *testing.T) {
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

func Test_NewProjectFromOptionsV07_TargetArtifactNames(t *testing.T) {
	project := mustProjectFromBytes(t, t.TempDir(), `
spec: v0.7
name: My App_01
unikraft: stable
targets:
  - plat: qemu
    arch: x86_64
`)

	targets := project.Targets()
	if len(targets) != 1 {
		t.Fatalf("len(Targets()) = %d, want 1", len(targets))
	}

	wantKernel := filepath.Join(project.OutDir(), "myapp_01_qemu-x86_64")
	if targets[0].Kernel() != wantKernel {
		t.Errorf("targets[0].Kernel() = %q, want %q", targets[0].Kernel(), wantKernel)
	}

	if targets[0].ConfigFilename() != ".config.myapp_01_qemu-x86_64" {
		t.Errorf("targets[0].ConfigFilename() = %q, want %q", targets[0].ConfigFilename(), ".config.myapp_01_qemu-x86_64")
	}
}

func Test_NewProjectFromOptionsV07_CustomOutDir(t *testing.T) {
	workdir := t.TempDir()
	outdir := filepath.Join(workdir, "artifacts")

	project, err := NewProjectFromOptions(
		context.Background(),
		WithProjectWorkdir(workdir),
		WithProjectOutDir(outdir),
		WithProjectKraftfileFromBytes([]byte(`
spec: v0.7
name: demo
unikraft: stable
targets:
  - plat: qemu
    arch: x86_64
`)),
	)
	if err != nil {
		t.Fatalf("NewProjectFromOptions() error = %v", err)
	}

	if project.OutDir() != outdir {
		t.Errorf("OutDir() = %q, want %q", project.OutDir(), outdir)
	}

	targets := project.Targets()
	if len(targets) != 1 {
		t.Fatalf("len(Targets()) = %d, want 1", len(targets))
	}

	wantKernel := filepath.Join(outdir, "demo_qemu-x86_64")
	if targets[0].Kernel() != wantKernel {
		t.Errorf("targets[0].Kernel() = %q, want %q", targets[0].Kernel(), wantKernel)
	}
}

func Test_NewProjectFromOptionsV07_ComponentStrings(t *testing.T) {
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

func Test_NewProjectFromOptionsV07_RuntimeReference(t *testing.T) {
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

func Test_NewProjectFromOptionsV07_RuntimeShorthandTag(t *testing.T) {
	project := mustProjectFromBytes(t, t.TempDir(), `
spec: v0.7
runtime: base:latest
`)

	if project.Runtime() == nil {
		t.Fatal("expected runtime to be present")
	}

	if project.Runtime().Name() != "base" {
		t.Errorf("Runtime().Name() = %q, want %q", project.Runtime().Name(), "base")
	}

	if project.Runtime().Version() != "latest" {
		t.Errorf("Runtime().Version() = %q, want %q", project.Runtime().Version(), "latest")
	}

	if project.Runtime().Source() != "base:latest" {
		t.Errorf("Runtime().Source() = %q, want %q", project.Runtime().Source(), "base:latest")
	}

	if project.Runtime().Reference() != "base:latest" {
		t.Errorf("Runtime().Reference() = %q, want %q", project.Runtime().Reference(), "base:latest")
	}
}

func Test_v07TemplateFromDocument(t *testing.T) {
	// Test the v07 template converter directly since template resolution
	// now happens at project load time and requires a package manager.
	doc := &kraftfilev07.Template{
		Source:  "https://github.com/unikraft/catalog.git",
		Version: "stable",
	}

	templateConfig, err := v07TemplateFromDocument(context.Background(), doc)
	if err != nil {
		t.Fatalf("v07TemplateFromDocument() error = %v", err)
	}

	if templateConfig == nil {
		t.Fatal("expected template config to be present")
	}

	if templateConfig.Name() != "catalog" {
		t.Errorf("Name() = %q, want %q", templateConfig.Name(), "catalog")
	}

	if templateConfig.Source() != "https://github.com/unikraft/catalog.git" {
		t.Errorf("Source() = %q, want original source", templateConfig.Source())
	}

	if templateConfig.Version() != "stable" {
		t.Errorf("Version() = %q, want %q", templateConfig.Version(), "stable")
	}

	if len(templateConfig.KConfig()) != 0 {
		t.Errorf("KConfig() len = %d, want 0", len(templateConfig.KConfig()))
	}
}

func Test_NewProjectFromOptionsV07_TemplateProvidesName(t *testing.T) {
	workdir := t.TempDir()
	templateDir := filepath.Join(workdir, "template")
	if err := os.MkdirAll(templateDir, 0o755); err != nil {
		t.Fatalf("could not create template directory: %v", err)
	}

	if err := os.WriteFile(filepath.Join(templateDir, "kraft.yaml"), []byte(`
spec: v0.7
name: template-name
runtime: base:latest
`), 0o644); err != nil {
		t.Fatalf("could not write template kraftfile: %v", err)
	}

	project := mustProjectFromBytes(t, workdir, `
spec: v0.7
template:
  source: `+templateDir+`
`)

	if project.Name() != "template-name" {
		t.Errorf("Name() = %q, want %q", project.Name(), "template-name")
	}
}

func Test_NewProjectFromOptionsV07_TemplateProjectFieldsOverride(t *testing.T) {
	workdir := t.TempDir()
	templateDir := filepath.Join(workdir, "template")
	if err := os.MkdirAll(templateDir, 0o755); err != nil {
		t.Fatalf("could not create template directory: %v", err)
	}

	if err := os.WriteFile(filepath.Join(templateDir, "kraft.yaml"), []byte(`
spec: v0.7
name: template-name
runtime: template-runtime:1.0
`), 0o644); err != nil {
		t.Fatalf("could not write template kraftfile: %v", err)
	}

	project := mustProjectFromBytes(t, workdir, `
spec: v0.7
name: project-name
template:
  source: `+templateDir+`
runtime: project-runtime:2.0
`)

	if project.Name() != "project-name" {
		t.Errorf("Name() = %q, want %q", project.Name(), "project-name")
	}

	if project.Runtime() == nil {
		t.Fatal("expected runtime to be present")
	}

	if project.Runtime().Name() != "project-runtime" {
		t.Errorf("Runtime().Name() = %q, want %q", project.Runtime().Name(), "project-runtime")
	}

	if project.Runtime().Version() != "2.0" {
		t.Errorf("Runtime().Version() = %q, want %q", project.Runtime().Version(), "2.0")
	}
}

func Test_NewProjectFromOptionsV07_RootfsFormat(t *testing.T) {
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

func Test_NewProjectFromOptionsV07_RomsMetadata(t *testing.T) {
	project := mustProjectFromBytes(t, t.TempDir(), `
spec: v0.7
runtime: base:latest
roms:
  - source: ./assets
    format: erofs
  - source: ./blob.bin
`)

	roms := project.Roms()
	if len(roms) != 2 {
		t.Fatalf("len(Roms()) = %d, want 2", len(roms))
	}

	if roms[0].Source != "./assets" {
		t.Errorf("roms[0].Source = %q, want %q", roms[0].Source, "./assets")
	}

	if roms[0].Format != kraftfilev07.FsTypeErofs {
		t.Errorf("roms[0].Format = %q, want %q", roms[0].Format, kraftfilev07.FsTypeErofs)
	}

	if roms[1].Source != "./blob.bin" {
		t.Errorf("roms[1].Source = %q, want %q", roms[1].Source, "./blob.bin")
	}

	if roms[1].Format != kraftfilev07.FsTypeCpio {
		t.Errorf("roms[1].Format = %q, want %q", roms[1].Format, kraftfilev07.FsTypeCpio)
	}
}

func Test_NewProjectFromOptionsV07_Volumes(t *testing.T) {
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

func Test_NewProjectFromOptionsV07_MutationsRejected(t *testing.T) {
	ctx := context.Background()
	project := mustProjectFromBytes(t, t.TempDir(), `
spec: v0.7
runtime: base:latest
`)

	if err := project.Save(ctx); !errors.Is(err, ErrProjectMutationNotSupported) {
		t.Fatalf("Save() error = %v, want ErrProjectMutationNotSupported", err)
	}

	if err := project.AddLibrary(ctx, lib.LibraryConfig{}); !errors.Is(err, ErrProjectMutationNotSupported) {
		t.Fatalf("AddLibrary() error = %v, want ErrProjectMutationNotSupported", err)
	}

	if err := project.RemoveLibrary(ctx, "libfoo"); !errors.Is(err, ErrProjectMutationNotSupported) {
		t.Fatalf("RemoveLibrary() error = %v, want ErrProjectMutationNotSupported", err)
	}
}

func mustProjectFromBytes(t *testing.T, workdir string, content string) Application {
	t.Helper()

	project, err := NewProjectFromOptions(
		context.Background(),
		WithProjectWorkdir(workdir),
		WithProjectKraftfileFromBytes([]byte(content)),
		WithProjectSkipValidation(true),
	)
	if err != nil {
		t.Fatalf("could not create project: %v", err)
	}

	return project
}
