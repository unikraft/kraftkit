// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package app

import (
	"context"
	"fmt"

	"unikraft.com/x/kraftfile"

	"kraftkit.sh/initrd"
	"kraftkit.sh/kconfig"
	umake "kraftkit.sh/make" // make package to ukmake so it doesn't collide with Go's built in make()
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/app/volume"
	"kraftkit.sh/unikraft/component"
	"kraftkit.sh/unikraft/core"
	"kraftkit.sh/unikraft/lib"
	"kraftkit.sh/unikraft/runtime"
	"kraftkit.sh/unikraft/target"
	"kraftkit.sh/unikraft/template"
)

// When a kraft.yaml file has "spec: v0.7", we skip the old parser and use
// unikraft.com/x/kraftfile to read it instead. applicationV07 holds the
// result and makes it work with the rest of KraftKit.

// Type layout from unikraft.com/x/kraftfile (spec.go):
//
//	Kraftfile { Name, Targets []Target, Cmd Command, Env Map,
//	            Labels, Runtime *Runtime, Unikraft *Unikraft,
//	            Libraries map[string]Library, Rootfs *FS, Roms []FS,
//	            Volumes Volumes, Template *Template }

type applicationV07 struct {
	kf         *kraftfile.Kraftfile // parsed by unikraft.com/x/kraftfile
	workingDir string
	outDir     string
	kraftfile  *Kraftfile // kraftkit's own file metadata struct
}

/*
newApplicationV07 is the constructor called from project.go when
spec: v0.7 is detected in the raw file bytes.
*/
func newApplicationV07(_ context.Context, popts *ProjectOptions) (Application, error) {
	// ParseBytes is the entry point from unikraft.com/x/kraftfile/parse.go
	// It validates the spec version and unmarshals every field.
	kf, err := kraftfile.ParseBytes(popts.kraftfile.content)
	if err != nil {
		return nil, fmt.Errorf("parsing v0.7 Kraftfile: %w", err)
	}

	outDir := popts.outDir
	if outDir == "" {
		outDir = popts.RelativePath(unikraft.BuildDir)
	}

	return &applicationV07{
		kf:         kf,
		workingDir: popts.workdir,
		outDir:     outDir,
		kraftfile:  popts.kraftfile,
	}, nil
}

//─── unikraft.Nameable

func (a *applicationV07) Type() unikraft.ComponentType {
	return unikraft.ComponentTypeApp
}

func (a *applicationV07) Name() string {
	return a.kf.Name
}

func (a *applicationV07) Version() string {
	return ""
}

func (a *applicationV07) String() string {
	return a.kf.Name
}

//─── component.Component

func (a *applicationV07) Source() string {
	return ""
}

func (a *applicationV07) Path() string {
	return a.workingDir
}

func (a *applicationV07) KConfigTree(_ context.Context, _ ...*kconfig.KeyValue) (*kconfig.KConfigFile, error) {
	return nil, fmt.Errorf("not yet implemented for v0.7 schema")
}

func (a *applicationV07) KConfig() kconfig.KeyValueMap {
	return kconfig.KeyValueMap{}
}

func (a *applicationV07) PrintInfo(_ context.Context) string {
	return fmt.Sprintf("%s (spec: v0.7)", a.kf.Name)
}

func (a *applicationV07) MarshalYAML() (interface{}, error) {
	return nil, fmt.Errorf("not yet implemented for v0.7 schema")
}

//─── Implementing Application: simple field reads

func (a *applicationV07) WorkingDir() string {
	return a.workingDir
}

func (a *applicationV07) OutDir() string {
	return a.outDir
}

func (a *applicationV07) Kraftfile() *Kraftfile {
	return a.kraftfile
}

func (a *applicationV07) Labels() map[string]string {
	return a.kf.Labels
}

func (a *applicationV07) Command() []string {
	return []string(a.kf.Cmd)
}

// Env reads from kraftfile.Map (kraftfile/spec.go)
// converts []MapPair{Key, Value any} → map[string]string.
func (a *applicationV07) Env() map[string]string {
	return a.kf.Env.AsStringMap()
}

// Rootfs reads from kraftfile.FS.Source
func (a *applicationV07) Rootfs() string {
	if a.kf.Rootfs != nil {
		return a.kf.Rootfs.Source
	}
	return ""
}

func (a *applicationV07) Roms() []string {
	roms := make([]string, 0, len(a.kf.Roms))
	for _, r := range a.kf.Roms {
		roms = append(roms, r.Source)
	}
	return roms
}

// InitrdFsType reads from kraftfile.FS.Format which is FsType
// FsType.String() returns the string value
func (a *applicationV07) InitrdFsType() initrd.FsType {
	if a.kf.Rootfs != nil {
		return initrd.FsType(a.kf.Rootfs.Format.String())
	}
	return initrd.FsTypeCpio
}

func (a *applicationV07) SetInitrdFsType(_ initrd.FsType) {}

func (a *applicationV07) SetRootfs(rootfs string) {
	if a.kf.Rootfs == nil {
		a.kf.Rootfs = &kraftfile.FS{}
	}
	a.kf.Rootfs.Source = rootfs
}

// LibraryNames iterates the libraries map
func (a *applicationV07) LibraryNames() []string {
	names := make([]string, 0, len(a.kf.Libraries))
	for name := range a.kf.Libraries {
		names = append(names, name)
	}
	return names
}

// TargetNames iterates []Target
func (a *applicationV07) TargetNames() []string {
	names := make([]string, 0, len(a.kf.Targets))
	for _, t := range a.kf.Targets {
		names = append(names, fmt.Sprintf("%s/%s", t.Plat, t.Arch))
	}
	return names
}

func (a *applicationV07) Extensions() component.Extensions {
	return nil
}

// ─── Application: stubs (require kraftkit type conversion, TODO in separate PR)

func (a *applicationV07) Unikraft(_ context.Context) *core.UnikraftConfig {
	return nil
}

func (a *applicationV07) Template() *template.TemplateConfig {
	return nil
}

func (a *applicationV07) Runtime() *runtime.Runtime {
	return nil
}

func (a *applicationV07) Libraries(_ context.Context) (map[string]*lib.LibraryConfig, error) {
	return nil, fmt.Errorf("not yet implemented for v0.7 schema")
}

func (a *applicationV07) Targets() []target.Target {
	return nil
}

func (a *applicationV07) Volumes() []*volume.VolumeConfig {
	return nil
}

func (a *applicationV07) IsConfigured(_ target.Target) bool {
	return false
}

func (a *applicationV07) MakeArgs(_ context.Context, _ target.Target) (*core.MakeArgs, error) {
	return nil, fmt.Errorf("not yet implemented for v0.7 schema")
}

func (a *applicationV07) Make(_ context.Context, _ target.Target, _ ...umake.MakeOption) error {
	return fmt.Errorf("not yet implemented for v0.7 schema")
}

func (a *applicationV07) SyncConfig(_ context.Context, _ target.Target, _ ...umake.MakeOption) error {
	return fmt.Errorf("not yet implemented for v0.7 schema")
}

func (a *applicationV07) Configure(_ context.Context, _ target.Target, _ kconfig.KeyValueMap, _ ...umake.MakeOption) error {
	return fmt.Errorf("not yet implemented for v0.7 schema")
}

func (a *applicationV07) Prepare(_ context.Context, _ target.Target, _ ...umake.MakeOption) error {
	return fmt.Errorf("not yet implemented for v0.7 schema")
}

func (a *applicationV07) Clean(_ context.Context, _ target.Target, _ ...umake.MakeOption) error {
	return fmt.Errorf("not yet implemented for v0.7 schema")
}

func (a *applicationV07) Properclean(_ context.Context, _ target.Target, _ ...umake.MakeOption) error {
	return fmt.Errorf("not yet implemented for v0.7 schema")
}

func (a *applicationV07) Fetch(_ context.Context, _ target.Target, _ ...umake.MakeOption) error {
	return fmt.Errorf("not yet implemented for v0.7 schema")
}

func (a *applicationV07) Set(_ context.Context, _ target.Target, _ kconfig.KeyValueMap, _ ...umake.MakeOption) error {
	return fmt.Errorf("not yet implemented for v0.7 schema")
}

func (a *applicationV07) Unset(_ context.Context, _ target.Target, _ kconfig.KeyValueMap, _ ...umake.MakeOption) error {
	return fmt.Errorf("not yet implemented for v0.7 schema")
}

func (a *applicationV07) Build(_ context.Context, _ target.Target, _ ...BuildOption) error {
	return fmt.Errorf("not yet implemented for v0.7 schema")
}

func (a *applicationV07) MergeTemplate(_ context.Context, _ Application) (Application, error) {
	return nil, fmt.Errorf("not yet implemented for v0.7 schema")
}

func (a *applicationV07) Components(_ context.Context, _ ...target.Target) ([]component.Component, error) {
	return nil, fmt.Errorf("not yet implemented for v0.7 schema")
}

func (a *applicationV07) WithTarget(_ target.Target) (Application, error) {
	return nil, fmt.Errorf("not yet implemented for v0.7 schema")
}

func (a *applicationV07) Save(_ context.Context) error {
	return fmt.Errorf("not yet implemented for v0.7 schema")
}

func (a *applicationV07) RemoveLibrary(_ context.Context, _ string) error {
	return fmt.Errorf("not yet implemented for v0.7 schema")
}

func (a *applicationV07) AddLibrary(_ context.Context, _ lib.LibraryConfig) error {
	return fmt.Errorf("not yet implemented for v0.7 schema")
}
