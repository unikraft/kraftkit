// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package app

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"sigs.k8s.io/yaml"
)

// NewProjectFromOptions load a kraft project based on command line options.
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

	popts.workdir = absWorkdir

	if popts.name != "" {
		popts.SetProjectName(popts.name, false)
	} else {
		popts.SetProjectName(filepath.Base(absWorkdir), true)
	}

	if popts.kraftfile == nil {
		return nil, ErrNoKraftfile
	}

	spec, err := sniffKraftfileSpecVersion(popts.kraftfile.content)
	if err != nil {
		return nil, err
	}

	if isV07SpecVersion(spec) {
		return newProjectFromOptionsV07(ctx, popts)
	}

	return newLegacyProjectFromOptions(ctx, popts)
}

func sniffKraftfileSpecVersion(content []byte) (string, error) {
	var header struct {
		Spec          string `json:"spec,omitempty"`
		Specification string `json:"specification,omitempty"`
	}

	if err := yaml.Unmarshal(content, &header); err != nil {
		return "", err
	}

	spec := firstNonEmpty(header.Spec, header.Specification)
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return "", nil
	}

	if !strings.HasPrefix(spec, "v") {
		spec = "v" + spec
	}

	parts := strings.Split(strings.TrimPrefix(spec, "v"), ".")
	if len(parts) < 2 {
		return spec, nil
	}

	return "v" + parts[0] + "." + parts[1], nil
}

func isV07SpecVersion(spec string) bool {
	return spec == ProjectLoaderV07.String()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}

	return ""
}

func (loader ProjectLoader) String() string {
	return string(loader)
}
