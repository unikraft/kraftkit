// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
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
type ConfigManager[C any] struct {
	Config     *C
	ConfigFile string
	Feeders    []Feeder
}

type ConfigManagerOption[C any] func(cm *ConfigManager[C]) error

func WithFeeder[C any](feeder Feeder) ConfigManagerOption[C] {
	return func(cm *ConfigManager[C]) error {
		cm.AddFeeder(feeder)
		return nil
	}
}

func WithFile[C any](file string, forceCreate bool) ConfigManagerOption[C] {
	return func(cm *ConfigManager[C]) error {
		ext := strings.Split(file, ".")
		if len(ext) == 1 {
			return fmt.Errorf("unknown file extension for config file: %s", file)
		}

		_, err := os.Stat(file)

		switch ext[len(ext)-1] {
		case "yaml", "yml":
			yml := YamlFeeder{
				File: file,
			}
			if os.IsNotExist(err) {
				err := yml.Write(cm.Config, forceCreate)
				if err != nil {
					return fmt.Errorf("could not write initial config: %v", err)
				}
			}
			return WithFeeder[C](yml)(cm)
		default:
			return fmt.Errorf("unsupported file extension: %s", file)
		}
	}
}

func WithDefaultConfigFile[C any]() ConfigManagerOption[C] {
	return func(cm *ConfigManager[C]) error {
		return WithFile[C](DefaultConfigFile(), true)(cm)
	}
}

func NewConfigManager[C any](c *C, opts ...ConfigManagerOption[C]) (*ConfigManager[C], error) {
	if c == nil {
		return nil, fmt.Errorf("cannot instantiate ConfigManager without Config")
	}

	cm := &ConfigManager[C]{
		Config: c,
	}

	for _, o := range opts {
		if err := o(cm); err != nil {
			return nil, fmt.Errorf("could not apply config manager option: %v", err)
		}
	}

	// Feed the config, pass the manager anyway if this fails, we still have
	// defaults
	if err := cm.Feed(); err != nil {
		return cm, fmt.Errorf("could not feed config: %v", err)
	}

	return cm, nil
}

// AddFeeder adds a feeder that provides configuration data.
func (cm *ConfigManager[C]) AddFeeder(f Feeder) *ConfigManager[C] {
	if f == nil {
		return cm
	}

	cm.Feeders = append(cm.Feeders, f)
	return cm
}

// Feed binds configuration data from added feeders to the added structs.
func (cm *ConfigManager[C]) Feed() error {
	for _, f := range cm.Feeders {
		if err := cm.feedStruct(f, cm.Config); err != nil {
			return err
		}
	}

	return nil
}

func (cm *ConfigManager[C]) Write(merge bool) error {
	for _, f := range cm.Feeders {
		if err := f.Write(cm.Config, merge); err != nil {
			return err
		}
	}

	return nil
}

// SetupListener adds an OS signal listener to the Config instance. The listener
// listens to the `SIGHUP` signal and refreshes the Config instance. It would
// call the provided fallback if the refresh process failed.
func (cm *ConfigManager[C]) SetupListener(fallback func(err error)) *ConfigManager[C] {
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
func (cm *ConfigManager[C]) feedStruct(f Feeder, s interface{}) error {
	if err := f.Feed(s); err != nil {
		return fmt.Errorf("failed to feed config: %v", err)
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

// FetchConfigDirFromArgs returns the path to the alternate config directory
// that can be set via the --config-dir flag. This needs to be fetched before flags
// are populated with AttributeFlags to ensure that the function is called only once.
func FetchConfigDirFromArgs(args []string) (path string) {
	for idx, arg := range args {
		if !strings.HasPrefix(arg, "--config-dir") {
			continue
		}
		if strings.Contains(arg, "=") {
			if split := strings.Split(arg, "="); len(split) == 2 {
				path = split[1]
			}
		} else {
			if !strings.HasPrefix(args[idx+1], "-") {
				path = args[idx+1]
			}
		}
		break
	}
	return
}

func Default[C any](key string) string {
	found, _, def, _, err := findConfigDefault[C](key, "", "", reflect.ValueOf(new([0]C)))
	if err != nil || found != key {
		return def
	}

	return ""
}

func findConfigDefault[C any](needle, offset, def string, v reflect.Value) (string, string, string, reflect.Value, error) {
	if v.Kind() != reflect.Ptr {
		return needle, offset, def, v, fmt.Errorf("not a pointer value")
	}

	if needle == offset {
		return needle, offset, def, v, nil
	}

	v = reflect.Indirect(v)
	switch v.Kind() {
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			name := v.Type().Field(i).Tag.Get("json")
			if len(name) == 0 {
				continue
			}

			check := name
			if len(offset) > 0 {
				check = offset + "." + name
			}

			dNeedle, dOffset, dDef, dv, dErr := findConfigDefault[C](
				needle,
				check,
				v.Type().Field(i).Tag.Get("default"),
				v.Field(i).Addr(),
			)

			if dOffset == needle {
				return dNeedle, dOffset, dDef, dv, dErr
			}
		}
	}

	return needle, offset, def, v, fmt.Errorf("could not find default for: %s", needle)
}
