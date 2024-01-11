// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

// Package compose provides primitives for running Unikraft applications
// via the Compose specification.
package compose

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/compose-spec/compose-go/loader"
	"github.com/compose-spec/compose-go/types"

	"kraftkit.sh/log"
	mplatform "kraftkit.sh/machine/platform"
	ukarch "kraftkit.sh/unikraft/arch"
)

type Project struct {
	*types.Project `json:"project"` // The underlying compose-go project
}

// DefaultFileNames is a list of default compose file names to look for
var DefaultFileNames = []string{
	"docker-compose.yml",
	"docker-compose.yaml",
	"compose.yml",
	"compose.yaml",
	"Composefile",
}

// NewProjectFromComposeFile loads a compose file and returns a project. If no
// compose file is specified, it will look for one in the current directory.
func NewProjectFromComposeFile(ctx context.Context, workdir, composefile string) (*Project, error) {
	if composefile == "" {
		for _, file := range DefaultFileNames {
			fullpath := filepath.Join(workdir, file)
			if _, err := os.Stat(fullpath); err == nil {
				log.G(ctx).Debugf("Found compose file: %s", file)
				composefile = file
				break
			}
		}
	}

	if composefile == "" {
		return nil, fmt.Errorf("no compose file found")
	}

	fullpath := filepath.Join(workdir, composefile)

	config := types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{
			{
				Filename: fullpath,
			},
		},
	}

	project, err := loader.Load(config)
	if err != nil {
		return nil, err
	}

	project.ComposeFiles = []string{composefile}
	project.WorkingDir = workdir

	return &Project{project}, err
}

// Validate performs some early checks on the project to ensure it is valid,
// as well as fill in some unspecified fields.
func (project *Project) Validate(ctx context.Context) error {
	// Check that each service has at least an image name or a build context
	for _, service := range project.Services {
		if service.Image == "" && service.Build == nil {
			return fmt.Errorf("service %s has neither an image nor a build context", service.Name)
		}
	}

	// If the project has no name, use the directory name
	if project.Name == "" {
		// Take the last part of the working directory
		parts := strings.Split(project.WorkingDir, "/")
		project.Name = parts[len(parts)-1]
	}

	// Fill in any missing image names and prepend the project name
	for i, service := range project.Services {
		project.Services[i].Name = fmt.Sprint(project.Name, "-", service.Name)
	}

	// Fill in any missing platforms
	for i, service := range project.Services {
		if service.Platform == "" {
			hostPlatform, _, err := mplatform.Detect(ctx)
			if err != nil {
				return err
			}

			hostArch, err := ukarch.HostArchitecture()
			if err != nil {
				return err
			}

			project.Services[i].Platform = fmt.Sprint(hostPlatform, "/", hostArch)

		}
	}

	return nil
}
