// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

// Package config provides facilities for manipulating kraftkit configuration
// files on the local filesystem.
package config

import (
	"os"
	"path/filepath"

	"github.com/onsi/ginkgo/v2"

	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// Config is a YAML-serializable kraftkit configuration.
// It provides facilities for reading and writing individual configuration
// attributes from/to a file.
//
// This type is purposely decoupled from kraftkit's internal configuration
// facilities to allow testing the CLI from an outside perspective,
// independently from kraftkit's internal implementation.
type Config struct {
	path string
}

// NewTempConfig creates a temporary kraftkit configuration file on the local
// filesystem, pre-populated with a "paths" configuration section referencing
// temporary directories.
//
// A ginkgo cleanup node is automatically created to handle the removal of the
// (temporary) parent directory of this configuration file.
func NewTempConfig() *Config {
	const offset = 1

	tmpDir, err := os.MkdirTemp("", "kraftkit-e2e-*")
	if err != nil {
		ginkgo.Fail("Error creating temporary directory for configuration: "+err.Error(), offset)
	}
	ginkgo.DeferCleanup(
		func() error {
			return os.RemoveAll(tmpDir)
		},
		ginkgo.Offset(offset),
	)

	configDir := filepath.Join(tmpDir, "config")
	if err := os.Mkdir(configDir, 0o755); err != nil {
		ginkgo.Fail("Error creating temporary subdirectory "+configDir+": "+err.Error(), offset)
	}

	configDoc := yaml.NewMapRNode(nil)
	err = configDoc.PipeE(
		yaml.SetField("paths", yaml.NewMapRNode(nil)),
		yaml.Tee(yaml.SetField("manifests", yaml.NewStringRNode(filepath.Join(tmpDir, "manifests")))),
		yaml.Tee(yaml.SetField("plugins", yaml.NewStringRNode(filepath.Join(tmpDir, "plugins")))),
		yaml.SetField("sources", yaml.NewStringRNode(filepath.Join(tmpDir, "sources"))),
	)
	if err != nil {
		ginkgo.Fail("Error creating initial configuration YAML: "+err.Error(), offset)
	}

	c := &Config{
		path: filepath.Join(configDir, "config.yaml"),
	}

	if err := yaml.WriteFile(configDoc, c.path); err != nil {
		ginkgo.Fail("Error creating configuration file: "+err.Error(), offset)
	}

	return c
}

// Path returns the path to the kraftkit configuration.
func (c *Config) Path() string {
	return c.path
}

// Read returns the YAML node at the given YAML path from the configuration.
func (c *Config) Read(yamlPath ...string) *yaml.RNode {
	const offset = 1

	configDoc, err := yaml.ReadFile(c.path)
	if err != nil {
		ginkgo.Fail("Error reading configuration file: "+err.Error(), offset)
	}

	v, err := configDoc.Pipe(yaml.Lookup(yamlPath...))
	if err != nil {
		ginkgo.Fail("Error invoking YAML filter(s) on configuration: "+err.Error(), offset)
	}

	return v
}

// Write writes configuration changes by executing the given YAML filters.
func (c *Config) Write(filters ...yaml.Filter) {
	const offset = 1

	if len(filters) == 0 {
		return
	}

	var f yaml.Filter = filters[0]
	if len(filters) > 1 {
		f = yaml.TeePiper{
			Filters: filters,
		}
	}

	if err := yaml.UpdateFile(f, c.path); err != nil {
		ginkgo.Fail("Error updating configuration file: "+err.Error(), offset)
	}
}
