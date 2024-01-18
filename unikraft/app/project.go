// SPDX-License-Identifier: Apache-2.0
//
// Copyright 2020 The Compose Specification Authors.
// Copyright 2022 Unikraft GmbH. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"kraftkit.sh/kconfig"
	"kraftkit.sh/log"
	"kraftkit.sh/schema"
	"kraftkit.sh/unikraft"
)

// ErrNoKraftfile is thrown when a project is instantiated at a directory
// without a recognizable Kraftfile.
var ErrNoKraftfile = fmt.Errorf("no Kraftfile specified")

// DefaultFileNames defines the kraft file names for auto-discovery (in order
// of preference)
var DefaultFileNames = []string{
	"kraft.yaml",
	"kraft.yml",
	"Kraftfile.yml",
	"Kraftfile.yaml",
	"Kraftfile",
}

// IsWorkdirInitialized provides a quick check to determine if whether one of
// the supported project files (Kraftfiles) is present within a provided working
// directory.
func IsWorkdirInitialized(dir string) bool {
	return len(findFiles(DefaultFileNames, dir)) > 0
}

// NewProjectFromOptions load a kraft project based on command line options
func NewProjectFromOptions(ctx context.Context, opts ...ProjectOption) (Application, error) {
	popts, err := NewProjectOptions(opts...)
	if err != nil {
		return nil, fmt.Errorf("could not apply project options: %v", err)
	}

	workdir, err := popts.Workdir()
	if err != nil {
		return nil, err
	}

	absWorkdir, err := filepath.Abs(workdir)
	if err != nil {
		return nil, err
	}

	if popts.name != "" {
		popts.SetProjectName(popts.name, false)
	} else {
		popts.SetProjectName(filepath.Base(absWorkdir), true)
	}

	if popts.kraftfile == nil {
		return nil, ErrNoKraftfile
	}

	name, _ := popts.GetProjectName()
	outdir := unikraft.BuildDir

	iface := popts.kraftfile.config
	if iface == nil {
		dict, err := parseConfig(popts.kraftfile.content, popts)
		if err != nil {
			return nil, err
		}

		iface = dict
		popts.kraftfile.config = dict
	}

	if !popts.skipValidation {
		if err := schema.Validate(ctx, iface); err != nil {
			return nil, err
		}
	}

	iface = groupXFieldsIntoExtensions(iface)
	if n, ok := iface["name"]; ok {
		name = n.(string)
	}

	if n, ok := iface["outdir"]; ok {
		name = n.(string)
	}

	popts.kraftfile.config = iface

	uk := &unikraft.Context{
		UK_NAME:   name,
		UK_BASE:   popts.RelativePath(workdir),
		BUILD_DIR: popts.RelativePath(outdir),
	}

	if _, err := os.Stat(uk.BUILD_DIR); err != nil && os.IsNotExist(err) {
		if err := os.MkdirAll(uk.BUILD_DIR, 0o755); err != nil {
			return nil, fmt.Errorf("creating build directory: %w", err)
		}
	}

	ctx = unikraft.WithContext(ctx, uk)

	appl, err := NewApplicationFromInterface(ctx, popts.kraftfile.config, popts)
	if err != nil {
		return nil, err
	}

	app := appl.(*application)
	if err != nil {
		return nil, err
	}

	projectName, _ := popts.GetProjectName()
	if app.name != "" {
		projectName = app.name
	}

	if !popts.skipNormalization {
		projectName = normalizeProjectName(projectName)
	}

	if app.unikraft != nil {
		popts.kconfig.OverrideBy(app.unikraft.KConfig())
	}

	for _, library := range app.libraries {
		popts.kconfig.OverrideBy(library.KConfig())
	}

	// Post-process each target by parsing any available .config file
	for _, target := range app.targets {
		target.KraftfileConfig(popts.kraftfile.config)
		kvmap, err := kconfig.NewKeyValueMapFromFile(
			filepath.Join(popts.workdir, target.ConfigFilename()),
		)
		if err != nil {
			log.G(ctx).Tracef("skipping uninitialized target config file: %v", err)
			continue
		}

		target.KConfig().OverrideBy(kvmap)
	}

	project, err := NewApplicationFromOptions(
		WithName(projectName),
		WithWorkingDir(popts.workdir),
		WithFilename(app.filename),
		WithOutDir(app.outDir),
		WithUnikraft(app.unikraft),
		WithRuntime(app.runtime),
		WithRootfs(app.rootfs),
		WithTemplate(app.template),
		WithCommand(app.command...),
		WithLibraries(app.libraries),
		WithTargets(app.targets),
		WithConfiguration(popts.kconfig.Slice()...),
		WithExtensions(app.extensions),
		WithKraftfile(popts.kraftfile),
		WithVolumes(app.volumes...),
	)
	if err != nil {
		return nil, err
	}

	if !popts.skipNormalization {
		err = normalize(project.(*application))
		if err != nil {
			return nil, err
		}
	}

	return project, nil
}
