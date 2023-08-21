// SPDX-License-Identifier: Apache-2.0
// Copyright 2014 Docker, Inc.
// Copyright 2023 Unikraft GmbH and The KraftKit Authors

// Package specconv implements conversion of specifications to libcontainer
// configurations
package specconv

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/opencontainers/runtime-spec/specs-go"

	"kraftkit.sh/libmocktainer/configs"
)

var (
	initMapsOnce     sync.Once
	namespaceMapping map[specs.LinuxNamespaceType]configs.NamespaceType
)

func initMaps() {
	initMapsOnce.Do(func() {
		namespaceMapping = map[specs.LinuxNamespaceType]configs.NamespaceType{
			specs.NetworkNamespace: configs.NEWNET,
		}
	})
}

// KnownNamespaces returns the list of the known namespaces.
// Used by `runc features`.
func KnownNamespaces() []string {
	initMaps()
	var res []string
	for k := range namespaceMapping {
		res = append(res, string(k))
	}
	sort.Strings(res)
	return res
}

type CreateOpts struct {
	Spec *specs.Spec
}

// CreateLibcontainerConfig creates a new libcontainer configuration from a
// given specification and a cgroup name
func CreateLibcontainerConfig(opts *CreateOpts) (*configs.Config, error) {
	// runc's cwd will always be the bundle path
	rcwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	cwd, err := filepath.Abs(rcwd)
	if err != nil {
		return nil, err
	}
	spec := opts.Spec
	if spec.Root == nil {
		return nil, errors.New("root must be specified")
	}
	rootfsPath := spec.Root.Path
	if !filepath.IsAbs(rootfsPath) {
		rootfsPath = filepath.Join(cwd, rootfsPath)
	}
	labels := []string{}
	for k, v := range spec.Annotations {
		labels = append(labels, k+"="+v)
	}
	config := &configs.Config{
		Rootfs: rootfsPath,
		Labels: append(labels, "bundle="+cwd),
	}

	// set linux-specific config
	if spec.Linux != nil {
		initMaps()

		for _, ns := range spec.Linux.Namespaces {
			t, exists := namespaceMapping[ns.Type]
			if !exists {
				return nil, fmt.Errorf("namespace %q does not exist", ns)
			}
			if config.Namespaces.Contains(t) {
				return nil, fmt.Errorf("malformed spec file: duplicated ns %q", ns)
			}
			config.Namespaces.Add(t, ns.Path)
		}
		if config.Namespaces.Contains(configs.NEWNET) && config.Namespaces.PathOf(configs.NEWNET) == "" {
			config.Networks = []*configs.Network{
				{
					Type: "loopback",
				},
			}
		}
	}

	createHooks(spec, config)
	config.Version = specs.Version
	return config, nil
}

func createHooks(rspec *specs.Spec, config *configs.Config) {
	config.Hooks = configs.Hooks{}
	if rspec.Hooks != nil {
		for _, h := range rspec.Hooks.Prestart {
			cmd := createCommandHook(h)
			config.Hooks[configs.Prestart] = append(config.Hooks[configs.Prestart], configs.NewCommandHook(cmd))
		}
		for _, h := range rspec.Hooks.CreateRuntime {
			cmd := createCommandHook(h)
			config.Hooks[configs.CreateRuntime] = append(config.Hooks[configs.CreateRuntime], configs.NewCommandHook(cmd))
		}
		for _, h := range rspec.Hooks.CreateContainer {
			cmd := createCommandHook(h)
			config.Hooks[configs.CreateContainer] = append(config.Hooks[configs.CreateContainer], configs.NewCommandHook(cmd))
		}
		for _, h := range rspec.Hooks.StartContainer {
			cmd := createCommandHook(h)
			config.Hooks[configs.StartContainer] = append(config.Hooks[configs.StartContainer], configs.NewCommandHook(cmd))
		}
		for _, h := range rspec.Hooks.Poststart {
			cmd := createCommandHook(h)
			config.Hooks[configs.Poststart] = append(config.Hooks[configs.Poststart], configs.NewCommandHook(cmd))
		}
		for _, h := range rspec.Hooks.Poststop {
			cmd := createCommandHook(h)
			config.Hooks[configs.Poststop] = append(config.Hooks[configs.Poststop], configs.NewCommandHook(cmd))
		}
	}
}

func createCommandHook(h specs.Hook) configs.Command {
	cmd := configs.Command{
		Path: h.Path,
		Args: h.Args,
		Env:  h.Env,
	}
	if h.Timeout != nil {
		d := time.Duration(*h.Timeout) * time.Second
		cmd.Timeout = &d
	}
	return cmd
}
