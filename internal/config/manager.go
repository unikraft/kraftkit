// SPDX-License-Identifier: BSD-3-Clause
//
// Authors: Alexander Jung <alex@unikraft.io>
//
// Copyright (c) 2022, Unikraft UG.  All rights reserved.
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
	"os"
	"os/signal"
	"reflect"

	"strings"
	"syscall"
)

// ConfigManager uses the package facilities, there should be at least one
// instance of it. It holds the configuration feeders and structs.
type ConfigManager struct {
	Config     *Config
	ConfigFile string
	Feeders    []Feeder
}

type ConfigManagerOption func(cm *ConfigManager) error

func WithFeeder(feeder Feeder) ConfigManagerOption {
	return func(cm *ConfigManager) error {
		cm.AddFeeder(feeder)
		return nil
	}
}

func WithFile(file string) ConfigManagerOption {
	return func(cm *ConfigManager) error {
		ext := strings.Split(file, ".")
		if len(ext) == 1 {
			return fmt.Errorf("unknown file extension for config file: %s", file)
		}

		switch ext[len(ext)-1] {
		case "yaml", "yml":
			return WithFeeder(YamlFeeder{
				File: file,
			})(cm)
		default:
			return fmt.Errorf("unsupported file extension: %s", file)
		}
	}
}

func WithDefaultConfigFile() ConfigManagerOption {
	return func(cm *ConfigManager) error {
		return WithFile(ConfigFile())(cm)
	}
}

func NewConfigManager(opts ...ConfigManagerOption) (*ConfigManager, error) {
	cm := &ConfigManager{}

	c, err := NewDefaultConfig()
	if err != nil {
		return nil, fmt.Errorf("could not seed default values for config: %s", err)
	}

	cm.Config = c

	for _, o := range opts {
		if err := o(cm); err != nil {
			return nil, err
		}
	}

	// Feed the config, pass the manager anyway if this fails, we still have
	// defaults
	if err := cm.Feed(); err != nil {
		return cm, err
	}

	return cm, nil
}

// AddFeeder adds a feeder that provides configuration data.
func (cm *ConfigManager) AddFeeder(f Feeder) *ConfigManager {
	cm.Feeders = append(cm.Feeders, f)
	return cm
}

// Feed binds configuration data from added feeders to the added structs.
func (cm *ConfigManager) Feed() error {
	for _, f := range cm.Feeders {
		if err := cm.feedStruct(f, &cm.Config); err != nil {
			return err
		}
	}

	return nil
}

func (cm *ConfigManager) Write() error {
	for _, f := range cm.Feeders {
		if err := f.Write(cm.Config); err != nil {
			return err
		}
	}

	return nil
}

// SetupListener adds an OS signal listener to the Config instance. The listener
// listens to the `SIGHUP` signal and refreshes the Config instance. It would
// call the provided fallback if the refresh process failed.
func (cm *ConfigManager) SetupListener(fallback func(err error)) *ConfigManager {
	s := make(chan os.Signal, 1)

	signal.Notify(s, syscall.SIGHUP)

	go func() {
		for {
			<-s
			if err := cm.Feed(); err != nil {
				fallback(err)
			}
		}
	}()

	return cm
}

// feedStruct feeds a struct using given feeder.
func (cm *ConfigManager) feedStruct(f Feeder, s interface{}) error {
	if err := f.Feed(s); err != nil {
		return fmt.Errorf("config: failed to feed struct; err %v", err)
	}

	return nil
}

func AllowedValues(key string) []string {
	for _, details := range ConfigDetails() {
		if details.Key == key {
			return details.AllowedValues
		}
	}

	return []string{}
}

func Default(key string) string {
	found, _, def, _, err := findConfigDefault(key, "", "", reflect.ValueOf(&Config{}))
	if err != nil || found != key {
		return def
	}

	return ""
}
