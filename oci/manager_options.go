// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package oci

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"kraftkit.sh/config"
	"kraftkit.sh/log"
	"kraftkit.sh/oci/handler"

	cliconfig "github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/configfile"
	regtypes "github.com/docker/docker/api/types/registry"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/mitchellh/go-homedir"
)

type OCIManagerOption func(context.Context, *ociManager) error

// WithDetectHandler uses internal KraftKit configuration to determine which
// underlying OCI handler implementation should be used. Ultimately, this is
// done by checking whether set configuration can ultimately invoke a relative
// client to enable the handler.
func WithDetectHandler() OCIManagerOption {
	return func(ctx context.Context, manager *ociManager) error {
		if contAddr := config.G[config.KraftKit](ctx).ContainerdAddr; len(contAddr) > 0 {
			namespace := DefaultNamespace
			if n := os.Getenv("CONTAINERD_NAMESPACE"); n != "" {
				namespace = n
			}

			log.G(ctx).
				WithField("addr", contAddr).
				WithField("namespace", namespace).
				Trace("using containerd handler")

			manager.handle = func(ctx context.Context) (context.Context, handler.Handler, error) {
				return handler.NewContainerdHandler(ctx, contAddr, namespace, manager.auths)
			}

			return nil
		}

		// Fall-back to using a simpler directory/tarball-based OCI handler
		ociDir := filepath.Join(config.G[config.KraftKit](ctx).RuntimeDir, "oci")

		log.G(ctx).
			WithField("path", ociDir).
			Trace("using directory handler")

		manager.handle = func(ctx context.Context) (context.Context, handler.Handler, error) {
			handle, err := handler.NewDirectoryHandler(ociDir, manager.auths)
			if err != nil {
				return nil, nil, err
			}

			return ctx, handle, nil
		}

		return nil
	}
}

// WithContainerd forces the use of a containerd handler by providing an address
// to the containerd daemon (whether UNIX socket or TCP socket) as well as the
// default namespace to operate within.
func WithContainerd(ctx context.Context, addr, namespace string) OCIManagerOption {
	return func(ctx context.Context, manager *ociManager) error {
		if n := os.Getenv("CONTAINERD_NAMESPACE"); n != "" {
			namespace = n
		} else if namespace == "" {
			namespace = DefaultNamespace
		}

		log.G(ctx).
			WithField("addr", addr).
			WithField("namespace", namespace).
			Trace("using containerd handler")

		manager.handle = func(ctx context.Context) (context.Context, handler.Handler, error) {
			return handler.NewContainerdHandler(ctx, addr, namespace, manager.auths)
		}

		return nil
	}
}

// WithDirectory forces the use of a directory handler by providing a path to
// the directory to use as the OCI root.
func WithDirectory(ctx context.Context, path string) OCIManagerOption {
	return func(ctx context.Context, manager *ociManager) error {
		log.G(ctx).
			WithField("path", path).
			Trace("using directory handler")

		manager.handle = func(ctx context.Context) (context.Context, handler.Handler, error) {
			handle, err := handler.NewDirectoryHandler(path, manager.auths)
			if err != nil {
				return nil, nil, err
			}

			return ctx, handle, nil
		}

		return nil
	}
}

// WithDefaultRegistries sets the list of KraftKit-set registries which is
// defined through its configuration.
func WithDefaultRegistries() OCIManagerOption {
	return func(ctx context.Context, manager *ociManager) error {
		manager.registries = []string{DefaultRegistry}

		for _, manifest := range config.G[config.KraftKit](ctx).Unikraft.Manifests {
			// Use internal KraftKit knowledge of the fact that the config often lists
			// the well-known path of the Manifest package manager's remote index.
			// This is obviously not an OCI image registry so we can safely skip it.
			// Doing this speeds up the kraft CLI and the instantiation of the OCI
			// Package Manager in general by a noticeable amount, especially with
			// limited internet connectivity (as none is subsequently required).
			if manifest == config.DefaultManifestIndex {
				continue
			}

			regName, err := name.NewRegistry(manifest)
			if err != nil {
				continue
			}

			if _, err := transport.Ping(ctx, regName, http.DefaultTransport.(*http.Transport).Clone()); err == nil {
				manager.registries = append(manager.registries, manifest)
			}
		}

		return nil
	}
}

// fileExists returns true if the given path exists and is not a directory.
func fileExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir()
}

// defaultAuths uses the provided context to locate possible authentication
// values which can be used when speaking with remote registries.
func defaultAuths(ctx context.Context) (map[string]config.AuthConfig, error) {
	auths := make(map[string]config.AuthConfig)

	// Podman users may have their container registry auth configured in a
	// different location, that Docker packages aren't aware of.
	// If the Docker config file isn't found, we'll fallback to look where
	// Podman configures it, and parse that as a Docker auth config instead.

	// First, check $HOME/.docker/
	var home string
	var err error
	foundDockerConfig := false

	// If this is run in the context of GitHub actions, use an alternative path
	// for the $HOME.
	if os.Getenv("GITUB_ACTION") == "yes" {
		home = "/github/home"
	} else {
		home, err = homedir.Dir()
	}
	if err == nil {
		foundDockerConfig = fileExists(filepath.Join(home, ".docker/config.json"))
	}

	// If $HOME/.docker/config.json isn't found, check $DOCKER_CONFIG (if set)
	if !foundDockerConfig && os.Getenv("DOCKER_CONFIG") != "" {
		foundDockerConfig = fileExists(filepath.Join(os.Getenv("DOCKER_CONFIG"), "config.json"))
	}

	// If either of those locations are found, load it using Docker's
	// config.Load, which may fail if the config can't be parsed.
	//
	// If neither was found, look for Podman's auth at
	// $XDG_RUNTIME_DIR/containers/auth.json and attempt to load it as a
	// Docker config.
	var cf *configfile.ConfigFile
	if foundDockerConfig {
		cf, err = cliconfig.Load(os.Getenv("DOCKER_CONFIG"))
		if err != nil {
			return nil, err
		}
	} else if f, err := os.Open(filepath.Join(os.Getenv("XDG_RUNTIME_DIR"), "containers/auth.json")); err == nil {
		defer f.Close()

		cf, err = cliconfig.LoadFromReader(f)
		if err != nil {
			return nil, err
		}
	}

	if cf != nil {
		for domain, cfg := range cf.AuthConfigs {
			if cfg.Username == "" && cfg.Password == "" {
				continue
			}
			auths[domain] = config.AuthConfig{
				Endpoint: cfg.ServerAddress,
				User:     cfg.Username,
				Token:    cfg.Password,
			}
		}
	}

	for domain, auth := range config.G[config.KraftKit](ctx).Auth {
		auths[domain] = auth
	}

	return auths, nil
}

// WithDefaultAuth uses the KraftKit-set configuration for authentication
// against remote registries.
func WithDefaultAuth() OCIManagerOption {
	return func(ctx context.Context, manager *ociManager) error {
		var err error

		manager.auths, err = defaultAuths(ctx)
		if err != nil {
			return err
		}

		return nil
	}
}

// WithRegistries sets the list of registries to use when making calls to
// non-canonically named OCI references.
func WithRegistries(registries ...string) OCIManagerOption {
	return func(ctx context.Context, manager *ociManager) error {
		manager.registries = registries
		return nil
	}
}

// WithDockerConfig sets the authentication configuration to use when making
// calls to authenticated registries.
func WithDockerConfig(auth regtypes.AuthConfig) OCIManagerOption {
	return func(ctx context.Context, manager *ociManager) error {
		if auth.ServerAddress == "" {
			return fmt.Errorf("cannot use auth config without server address")
		}

		if manager.auths == nil {
			manager.auths = make(map[string]config.AuthConfig, 1)
		}

		manager.auths[auth.ServerAddress] = config.AuthConfig{
			Endpoint: auth.ServerAddress,
			User:     auth.Username,
			Token:    auth.Password,
		}
		return nil
	}
}
