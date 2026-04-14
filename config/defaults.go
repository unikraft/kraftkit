// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package config

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	cliconfig "github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/credentials"
	"github.com/docker/cli/cli/config/types"
	"kraftkit.sh/log"
)

const (
	DefaultManifestIndex = "https://manifests.kraftkit.sh/index.yaml"
)

func NewDefaultKraftKitConfig(ctx context.Context) (*KraftKit, error) {
	var err error
	c := &KraftKit{}

	if err := setDefaults(c); err != nil {
		return nil, fmt.Errorf("could not set defaults for config: %s", err)
	}

	c.Auth, err = defaultAuths(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not get default auths: %s", err)
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
func defaultAuths(ctx context.Context) (map[string]AuthConfig, error) {
	auths := make(map[string]AuthConfig)

	cf := cliconfig.LoadDefaultConfigFile(os.Stderr)
	if cf == nil {
		return nil, fmt.Errorf("could not load default config file")
	}

	a, err := credentials.NewFileStore(cf).GetAll()
	if err != nil {
		return nil, err
	}

	if cf.CredentialsStore != "" {
		storeOutput, storeErr := captureStderr(func() error {
			globalAuths, innerErr := cf.GetCredentialsStore("").GetAll()
			if innerErr != nil {
				return innerErr
			}
			for k, v := range globalAuths {
				if _, already := a[k]; !already {
					a[k] = v
				}
			}
			return nil
		})
		if len(storeOutput) > 0 {
			log.G(ctx).Debugf("global credential store output (%s): %s", cf.CredentialsStore, storeOutput)
		}
		if storeErr != nil {
			log.G(ctx).Debugf("external credential store %q failed: %v", cf.CredentialsStore, storeErr)
		}
	}

	for registryHostname := range cf.CredentialHelpers {
		var newAuth types.AuthConfig

		helperOutput, helperErr := captureStderr(func() error {
			var innerErr error
			newAuth, innerErr = cf.GetAuthConfig(registryHostname)
			return innerErr
		})

		if len(helperOutput) > 0 {
			log.G(ctx).Debugf("credential helper output (%s): %s", registryHostname, helperOutput)
		}
		if helperErr != nil {
			log.G(ctx).Debugf("failed to get credentials for registry %q: %v", registryHostname, helperErr)
			continue
		}

		a[registryHostname] = newAuth
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

	return auths, nil
}

// captureStderr temporarily redirects os.Stderr to an internal pipe, runs fn,
// then returns any text the helper wrote to stderr (trimmed). os.Stderr is
// restored before this function returns.
// NOTE(craciunoiuc): Workaround for subprocesses that write to stderr
func captureStderr(fn func() error) (output string, err error) {
	r, w, pipeErr := os.Pipe()
	if pipeErr != nil {
		return "", fn()
	}

	orig := os.Stderr
	os.Stderr = w

	var buf bytes.Buffer
	done := make(chan struct{})
	go func() {
		defer close(done)
		io.Copy(&buf, r) //nolint:errcheck
	}()

	err = fn()

	os.Stderr = orig
	w.Close()
	<-done
	r.Close()

	return strings.TrimSpace(buf.String()), err
}
