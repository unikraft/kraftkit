// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strconv"

	cliconfig "github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/configfile"
	"github.com/mitchellh/go-homedir"
)

const (
	DefaultManifestIndex = "https://manifests.kraftkit.sh/index.yaml"
)

func NewDefaultKraftKitConfig() (*KraftKit, error) {
	var err error
	c := &KraftKit{}

	if err := setDefaults(c); err != nil {
		return nil, fmt.Errorf("could not set defaults for config: %s", err)
	}

	c.Auth, err = defaultAuths()
	if err != nil {
		return nil, fmt.Errorf("could not get default auths: %s", err)
	}

	// Add default path for plugins..
	if len(c.Paths.Plugins) == 0 {
		c.Paths.Plugins = filepath.Join(DataDir(), "plugins")
	}

	// ..for configuration files..
	if len(c.Paths.Config) == 0 {
		c.Paths.Config = filepath.Join(ConfigDir())
	}

	// ..for manifest files..
	if len(c.Paths.Manifests) == 0 {
		c.Paths.Manifests = filepath.Join(DataDir(), "manifests")
	}

	// ..for runtime files..
	if len(c.RuntimeDir) == 0 {
		c.RuntimeDir = filepath.Join(DataDir(), "runtime")
	}

	// ..for events files..
	if len(c.EventsPidFile) == 0 {
		c.EventsPidFile = filepath.Join(c.RuntimeDir, "events.pid")
	}

	// ..and for cached source files
	if len(c.Paths.Sources) == 0 {
		c.Paths.Sources = filepath.Join(DataDir(), "sources")
	}

	if len(c.Unikraft.Manifests) == 0 {
		c.Unikraft.Manifests = append(c.Unikraft.Manifests, DefaultManifestIndex)
	}

	return c, nil
}

func setDefaults(s interface{}) error {
	return setDefaultValue(reflect.ValueOf(s), "")
}

func setDefaultValue(v reflect.Value, def string) error {
	if v.Kind() != reflect.Ptr {
		return fmt.Errorf("not a pointer value")
	}

	v = reflect.Indirect(v)

	switch v.Kind() {
	case reflect.Int:
		if len(def) > 0 {
			i, err := strconv.ParseInt(def, 10, 64)
			if err != nil {
				return fmt.Errorf("could not parse default integer value: %s", err)
			}
			v.SetInt(i)
		}

	case reflect.String:
		if len(def) > 0 {
			v.SetString(def)
		}

	case reflect.Bool:
		if len(def) > 0 {
			b, err := strconv.ParseBool(def)
			if err != nil {
				return fmt.Errorf("could not parse default boolean value: %s", err)
			}
			v.SetBool(b)
		} else {
			// Assume false by default
			v.SetBool(false)
		}

	case reflect.Struct:
		// Iterate over the struct fields
		for i := 0; i < v.NumField(); i++ {
			// Use the `env` tag to look up the default value
			def = v.Type().Field(i).Tag.Get("default")
			if err := setDefaultValue(
				v.Field(i).Addr(),
				def,
			); err != nil {
				return err
			}
		}

	// TODO: Arrays? Maps?

	default:
		// Ignore this value and property entirely
		return nil
	}

	return nil
}

// defaultAuths uses the provided context to locate possible authentication
// values which can be used when speaking with remote registries.
func defaultAuths() (map[string]AuthConfig, error) {
	auths := make(map[string]AuthConfig)

	// Podman users may have their container registry auth configured in a
	// different location, that Docker packages aren't aware of.
	// If the Docker config file isn't found, we'll fallback to look where
	// Podman configures it, and parse that as a Docker auth config instead.

	// First, check $HOME/.docker/
	var home string
	var err error
	var configPath string
	foundDockerConfig := false

	// If this is run in the context of GitHub actions, use an alternative path
	// for the $HOME.
	if os.Getenv("GITUB_ACTION") == "yes" {
		home = "/github/home"
	} else {
		home, err = homedir.Dir()
	}
	if err == nil {
		foundDockerConfig = fileExists(filepath.Join(home, ".docker", "config.json"))

		if foundDockerConfig {
			configPath = filepath.Join(home, ".docker")
		}
	}

	// If $HOME/.docker/config.json isn't found, check $DOCKER_CONFIG (if set)
	if !foundDockerConfig && os.Getenv("DOCKER_CONFIG") != "" {
		foundDockerConfig = fileExists(filepath.Join(os.Getenv("DOCKER_CONFIG"), "config.json"))

		if foundDockerConfig {
			configPath = os.Getenv("DOCKER_CONFIG")
		}
	}

	// If either of those locations are found, load it using Docker's
	// config.Load, which may fail if the config can't be parsed.
	//
	// If neither was found, look for Podman's auth at
	// $XDG_RUNTIME_DIR/containers/auth.json and attempt to load it as a
	// Docker config.
	var cf *configfile.ConfigFile
	if foundDockerConfig {
		cf, err = cliconfig.Load(configPath)
		if err != nil {
			return nil, err
		}
	} else if f, err := os.Open(filepath.Join(os.Getenv("XDG_RUNTIME_DIR"), "containers", "auth.json")); err == nil {
		defer f.Close()

		cf, err = cliconfig.LoadFromReader(f)
		if err != nil {
			return nil, err
		}
	}

	if cf != nil {
		a, err := cf.GetAllCredentials()
		if err != nil {
			return nil, err
		}

		for domain, cfg := range a {
			if cfg.Username == "" && cfg.Password == "" {
				continue
			}

			purl, err := url.Parse(domain)
			if err != nil {
				return nil, err
			}

			u := purl.Host
			if u == "" {
				domain = purl.Path // Sometimes occurs with ghcr.io
			}
			if u == "" {
				u = cfg.ServerAddress
			}
			if u == "" {
				u = domain
			}

			auths[u] = AuthConfig{
				Endpoint: u,
				User:     cfg.Username,
				Token:    cfg.Password,
			}
		}
	}

	return auths, nil
}
