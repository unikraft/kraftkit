// SPDX-License-Identifier: BSD-3-Clause
//
// Authors: Alexander Jung <alex@unikraft.io>
//
// Copyright (c) 2022, Unikraft GmbH.  All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions
// are met:
//
// 1. Redistributions of source code must retain the above copyright
//    notice, this list of conditions and the following disclaimer.
// 2. Redistributions in binary form must reproduce the above copyright
//    notice, this list of conditions and the following disclaimer in the
//    documentation and/or other materials provided with the distribution.
// 3. Neither the name of the copyright holder nor the names of its
//    contributors may be used to endorse or promote products derived from
//    this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
// AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
// IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
// ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE
// LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR
// CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF
// SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
// INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN
// CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
// ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
// POSSIBILITY OF SUCH DAMAGE.

package config

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strconv"
)

const (
	DefaultNoPrompt      = "false"
	DefaultNoParallel    = "true"
	DefaultNoEmojis      = "false"
	DefaultGitProtocol   = "https"
	DefaultLogLevel      = "info"
	DefaultLogTimestamps = "false"
	DefaultLogType       = "fancy"
	DefaultRuntimeDir    = "/var/kraftkit"
)

func Defaults() map[string]string {
	return map[string]string{
		"KRAFTKIT_NO_PROMPT":      DefaultNoPrompt,
		"KRAFTKIT_NO_PARALLEL":    DefaultNoParallel,
		"KRAFTKIT_NO_EMOJIS":      DefaultNoEmojis,
		"KRAFTKIT_GIT_PROTOCOL":   DefaultGitProtocol,
		"KRAFTKIT_LOG_LEVEL":      DefaultLogLevel,
		"KRAFTKIT_LOG_TIMESTAMPS": DefaultLogTimestamps,
		"KRAFTKIT_LOG_TYPE":       DefaultLogType,
		"KRAFTKIT_RUNTIME_DIR":    DefaultRuntimeDir,
	}
}

func NewDefaultConfig() (*Config, error) {
	c := &Config{}

	if err := setDefaults(c); err != nil {
		return nil, fmt.Errorf("could not set defaults for config: %s", err)
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
			def = Defaults()[v.Type().Field(i).Tag.Get("env")]
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
