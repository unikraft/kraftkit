// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package app

import (
	"context"
	"fmt"
	"maps"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	kraftfilev07 "unikraft.com/x/kraftfile"

	"kraftkit.sh/kconfig"
	"kraftkit.sh/log"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/app/volume"
	"kraftkit.sh/unikraft/core"
	"kraftkit.sh/unikraft/lib"
	"kraftkit.sh/unikraft/runtime"
	"kraftkit.sh/unikraft/target"
	"kraftkit.sh/unikraft/template"
)

func newProjectFromOptionsV07(ctx context.Context, popts *ProjectOptions) (Application, error) {
	doc, err := kraftfilev07.ParseBytes(popts.kraftfile.content)
	if err != nil {
		return nil, err
	}

	name, _ := popts.GetProjectName()
	if doc.Name != "" {
		name = doc.Name
	}

	ukContext := &unikraft.Context{
		UK_NAME: name,
		UK_BASE: popts.workdir,
	}
	ctx = unikraft.WithContext(ctx, ukContext)

	if doc.Template != nil {
		opts := []TemplateResolutionOption{}
		if popts.workdir != "" {
			opts = append(opts, WithTemplateResolutionWorkdir(popts.workdir))
		}
		doc, err = ResolveTemplate(ctx, doc, opts...)
		if err != nil {
			return nil, err
		}

		// Update name if the template resolution changed it or populated it.
		if doc.Name != "" {
			name = doc.Name
			ukContext.UK_NAME = doc.Name
		}
	}

	popts.kraftfile.config = map[string]any{
		"spec": doc.Spec,
	}

	outdir, err := v07OutDirFromProjectOptions(popts)
	if err != nil {
		return nil, err
	}

	ukContext.BUILD_DIR = outdir

	if _, err := os.Stat(ukContext.BUILD_DIR); err != nil && os.IsNotExist(err) {
		if err := os.MkdirAll(ukContext.BUILD_DIR, 0o755); err != nil {
			return nil, fmt.Errorf("creating build directory: %w", err)
		}
	}

	ctx = unikraft.WithContext(ctx, ukContext)

	unikraftConfig, err := v07CoreFromDocument(ctx, doc.Unikraft)
	if err != nil {
		return nil, err
	}

	runtimeConfig, err := v07RuntimeFromDocument(doc.Runtime)
	if err != nil {
		return nil, err
	}

	libraries, err := v07LibrariesFromDocument(ctx, doc.Libraries)
	if err != nil {
		return nil, err
	}

	if unikraftConfig != nil {
		popts.kconfig.OverrideBy(unikraftConfig.KConfig())
	}

	for _, library := range libraries {
		popts.kconfig.OverrideBy(library.KConfig())
	}

	targets, err := v07TargetsFromDocument(ctx, doc.Targets)
	if err != nil {
		return nil, err
	}

	volumes, err := v07VolumesFromDocument(ctx, doc.Volumes)
	if err != nil {
		return nil, err
	}

	rootfs, fsType, roms := v07RootfsAndRomsFromDocument(doc.Rootfs, doc.Roms)

	project, err := NewApplicationFromOptions(
		WithName(name),
		WithWorkingDir(popts.workdir),
		WithFilename(popts.kraftfile.path),
		WithOutDir(outdir),
		WithUnikraft(unikraftConfig),
		WithRuntime(runtimeConfig),
		WithRootfs(rootfs),
		WithFsType(fsType),
		WithRoms(roms...),
		WithCommand(doc.Cmd...),
		WithLabels(maps.Clone(doc.Labels)),
		WithLibraries(libraries),
		WithTargets(targets),
		WithConfiguration(popts.kconfig.Slice()...),
		WithKraftfile(popts.kraftfile),
		WithVolumes(volumes...),
		WithEnv(doc.Env.AsStringMap()),
		WithLoaderKind(ProjectLoaderV07),
		WithSpecVersion(doc.Spec),
	)
	if err != nil {
		return nil, err
	}

	for _, targ := range project.Targets() {
		kvmap, err := kconfig.NewKeyValueMapFromFile(filepath.Join(popts.workdir, targ.ConfigFilename()))
		if err != nil {
			log.G(ctx).Tracef("skipping uninitialized target config file: %v", err)
			continue
		}

		targ.KConfig().OverrideBy(kvmap)
	}

	return project, nil
}

func v07OutDirFromProjectOptions(popts *ProjectOptions) (string, error) {
	if popts.outDir == "" {
		return popts.RelativePath(unikraft.BuildDir), nil
	}

	return filepath.Abs(popts.outDir)
}

func v07CoreFromDocument(ctx context.Context, doc *kraftfilev07.Unikraft) (*core.UnikraftConfig, error) {
	if doc == nil {
		return nil, nil
	}

	value := map[string]any{}
	if doc.Source != "" {
		value["source"] = doc.Source
	}
	if doc.Version != "" {
		value["version"] = doc.Version
	}
	if kconf := doc.KConfig.AsMap(); len(kconf) > 0 {
		value["kconfig"] = kconf
	}

	transformed, err := core.TransformFromSchema(ctx, value)
	if err != nil {
		return nil, err
	}

	config := transformed.(core.UnikraftConfig)
	return &config, nil
}

func v07TemplateFromDocument(ctx context.Context, doc *kraftfilev07.Template) (*template.TemplateConfig, error) {
	if doc == nil {
		return nil, nil
	}

	value := map[string]any{}
	if name := guessNameFromSource(doc.Source); name != "" {
		value["name"] = name
	}
	if doc.Source != "" {
		value["source"] = doc.Source
	}
	if doc.Version != "" {
		value["version"] = doc.Version
	}

	transformed, err := template.TransformFromSchema(ctx, value)
	if err != nil {
		return nil, err
	}

	config := transformed.(template.TemplateConfig)
	return &config, nil
}

func v07RuntimeFromDocument(doc *kraftfilev07.Runtime) (*runtime.Runtime, error) {
	if doc == nil {
		return nil, nil
	}

	ref := string(*doc)
	if ref == "" {
		return nil, nil
	}

	config := &runtime.Runtime{}
	config.SetName(ref)
	if err := runtime.WithSource(ref)(config); err != nil {
		return nil, err
	}

	return config, nil
}

func v07LibrariesFromDocument(ctx context.Context, docs map[string]kraftfilev07.Library) (map[string]*lib.LibraryConfig, error) {
	if len(docs) == 0 {
		return nil, nil
	}

	libraries := make(map[string]*lib.LibraryConfig, len(docs))
	for name, doc := range docs {
		value := map[string]any{}
		if doc.Source != "" {
			value["source"] = doc.Source
		}
		if doc.Version != "" {
			value["version"] = doc.Version
		}
		if kconf := doc.KConfig.AsMap(); len(kconf) > 0 {
			value["kconfig"] = kconf
		}

		config, err := lib.TransformFromSchema(ctx, name, value)
		if err != nil {
			return nil, err
		}

		library := config
		libraries[name] = &library
	}

	return libraries, nil
}

func v07TargetsFromDocument(ctx context.Context, docs []kraftfilev07.Target) ([]*target.TargetConfig, error) {
	if len(docs) == 0 {
		return nil, nil
	}

	targets := make([]*target.TargetConfig, 0, len(docs))
	for _, doc := range docs {
		value := map[string]any{}
		if doc.Arch != "" {
			value["arch"] = doc.Arch
		}
		if doc.Plat != "" {
			value["plat"] = doc.Plat
		}
		if kconf := doc.KConfig.AsMap(); len(kconf) > 0 {
			value["kconfig"] = kconf
		}

		transformed, err := target.TransformFromSchema(ctx, value)
		if err != nil {
			return nil, err
		}

		config := transformed.(target.TargetConfig)
		targets = append(targets, &config)
	}

	return targets, nil
}

func v07VolumesFromDocument(ctx context.Context, docs kraftfilev07.Volumes) ([]*volume.VolumeConfig, error) {
	if len(docs) == 0 {
		return nil, nil
	}

	volumes := make([]*volume.VolumeConfig, 0, len(docs))
	for _, doc := range docs {
		value := map[string]any{}
		if doc.Driver != "" {
			value["driver"] = doc.Driver
		}
		if doc.Source != "" {
			value["source"] = doc.Source
		}
		if doc.Destination != "" {
			value["destination"] = doc.Destination
		}
		if doc.Mode != nil {
			value["mode"] = doc.Mode
		}
		if doc.ReadOnly {
			value["readonly"] = doc.ReadOnly
		}

		transformed, err := volume.TransformFromSchema(ctx, value)
		if err != nil {
			return nil, err
		}

		config := transformed.(volume.VolumeConfig)
		volumes = append(volumes, &config)
	}

	return volumes, nil
}

func v07RootfsAndRomsFromDocument(rootfs *kraftfilev07.FS, roms []kraftfilev07.FS) (string, kraftfilev07.FsType, []kraftfilev07.FS) {
	var (
		rootfsPath string
		fsType     kraftfilev07.FsType
		rawRoms    []kraftfilev07.FS
	)

	if rootfs != nil {
		rootfsPath = rootfs.Source
		if rootfs.Format != "" {
			fsType = kraftfilev07.FsType(rootfs.Format.String())
		}
	}

	for _, rom := range roms {
		rawRoms = append(rawRoms, kraftfilev07.FS{
			Source: rom.Source,
			Format: kraftfilev07.FsType(rom.Format.String()),
		})
	}

	return rootfsPath, fsType, rawRoms
}

func guessNameFromSource(source string) string {
	if source == "" {
		return ""
	}

	candidate := source
	if parsed, err := url.Parse(source); err == nil && parsed.Host != "" {
		candidate = parsed.Path
	}

	candidate = strings.TrimSuffix(candidate, ".git")
	candidate = strings.TrimSuffix(candidate, ".tar.gz")
	candidate = strings.TrimSuffix(candidate, "/")

	name := path.Base(candidate)
	switch name {
	case ".", "/", "":
		return ""
	default:
		return name
	}
}
