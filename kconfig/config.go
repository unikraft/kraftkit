// SPDX-License-Identifier: Apache-2.0
// Copyright 2020 syzkaller project authors. All rights reserved.
// Copyright 2020 The Compose Specification Authors.
// Copyright 2022 Unikraft GmbH. All rights reserved.

package kconfig

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"
)

const DotConfigFileName = ".config"

// KConfigValues is a map of KConfigValue
type KConfigValues map[string]*KConfigValue

// NewKConfigValues build a new Mapping from a set of KEY=VALUE strings
func NewKConfigValues(values ...string) KConfigValues {
	mapping := KConfigValues{}

	for _, env := range values {
		tokens := strings.SplitN(env, "=", 2)
		if len(tokens) > 1 {
			mapping[tokens[0]] = &KConfigValue{
				Name:  tokens[0],
				Value: tokens[1],
			}
		} else {
			mapping[env] = nil
		}
	}

	return mapping
}

// OverrideBy update KConfigValues with values from another KConfigValues
func (kco KConfigValues) OverrideBy(other KConfigValues) KConfigValues {
	for k, v := range other {
		kco[k] = v
	}
	return kco
}

// Set a new key with specified value
func (kco KConfigValues) Set(key, value string) KConfigValues {
	kco[key] = &KConfigValue{
		Name:  key,
		Value: value,
	}

	return kco
}

// Unset a specific key
func (kco KConfigValues) Unset(key string) KConfigValues {
	delete(kco, key)

	return kco
}

// Resolve update a KConfig for keys without value (`key`, but not `key=`)
func (kco KConfigValues) Resolve(lookupFn func(string) (string, bool)) KConfigValues {
	for k, v := range kco {
		if v == nil {
			if value, ok := lookupFn(k); ok {
				kco[k] = &KConfigValue{
					Name:  k,
					Value: value,
				}
			}
		}
	}

	return kco
}

// RemoveEmpty excludes keys that are not associated with a value
func (kco KConfigValues) RemoveEmpty() KConfigValues {
	for k, v := range kco {
		if v == nil || v.Value == "" {
			delete(kco, k)
		}
	}

	return kco
}

// DotConfigFile represents a parsed .config file. It should not be modified
// directly, only by means of calling methods. The only exception is
// Config.Value which may be modified directly. Note: config names don't include
// CONFIG_ prefix, here and in other public interfaces, users of this package
// should never mention CONFIG_. Use Yes/Mod/No consts to check for/set config
// to particular values.
type DotConfigFile struct {
	Configs  []*KConfigValue
	Map      map[string]*KConfigValue // duplicates Configs for convenience
	comments []string
}

type KConfigValue struct {
	Name     string
	Value    string
	comments []string
}

const (
	Yes    = "y"
	Mod    = "m"
	No     = "---===[[[is not set]]]===---" // to make it more obvious when some code writes it directly
	prefix = "CONFIG_"
)

//  Value returns config value, or No if it's not present at all.
func (cf *DotConfigFile) Value(name string) string {
	cfg := cf.Map[name]
	if cfg == nil {
		return No
	}

	return cfg.Value
}

// Set changes config value, or adds it if it's not yet present.
func (cf *DotConfigFile) Set(name, val string) {
	cfg := cf.Map[name]
	if cfg == nil {
		cfg = &KConfigValue{
			Name:  name,
			Value: val,
		}

		cf.Map[name] = cfg
		cf.Configs = append(cf.Configs, cfg)
	}

	cfg.Value = val
	cfg.comments = append(cfg.comments, cf.comments...)
	cf.comments = nil
}

// Unset sets config value to No, if it's present in the config.
func (cf *DotConfigFile) Unset(name string) {
	cfg := cf.Map[name]
	if cfg == nil {
		return
	}

	cfg.Value = No
}

func (cf *DotConfigFile) ModToYes() {
	for _, cfg := range cf.Configs {
		if cfg.Value == Mod {
			cfg.Value = Yes
		}
	}
}

func (cf *DotConfigFile) ModToNo() {
	for _, cfg := range cf.Configs {
		if cfg.Value == Mod {
			cfg.Value = No
		}
	}
}

func (cf *DotConfigFile) Serialize() []byte {
	buf := new(bytes.Buffer)
	for _, cfg := range cf.Configs {
		for _, comment := range cfg.comments {
			fmt.Fprintf(buf, "%v\n", comment)
		}

		if cfg.Value == No {
			fmt.Fprintf(buf, "# %v%v is not set\n", prefix, cfg.Name)
		} else {
			fmt.Fprintf(buf, "%v%v=%v\n", prefix, cfg.Name, cfg.Value)
		}
	}

	for _, comment := range cf.comments {
		fmt.Fprintf(buf, "%v\n", comment)
	}

	return buf.Bytes()
}

func ParseConfig(file string) (*DotConfigFile, error) {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("failed to open .config file %v: %v", file, err)
	}

	return ParseConfigData(data, file)
}

func ParseConfigData(data []byte, file string) (*DotConfigFile, error) {
	cf := &DotConfigFile{
		Map: make(map[string]*KConfigValue),
	}

	s := bufio.NewScanner(bytes.NewReader(data))
	for s.Scan() {
		cf.parseLine(s.Text())
	}

	return cf, nil
}

func (cf *DotConfigFile) clone() *DotConfigFile {
	cf1 := &DotConfigFile{
		Map:      make(map[string]*KConfigValue),
		comments: cf.comments,
	}

	for _, cfg := range cf.Configs {
		cfg1 := new(KConfigValue)
		*cfg1 = *cfg
		cf1.Configs = append(cf1.Configs, cfg1)
		cf1.Map[cfg1.Name] = cfg1
	}

	return cf1
}

func (cf *DotConfigFile) parseLine(text string) {
	if match := reConfigY.FindStringSubmatch(text); match != nil {
		cf.Set(match[1], match[2])
	} else if match := reConfigN.FindStringSubmatch(text); match != nil {
		cf.Set(match[1], No)
	} else {
		cf.comments = append(cf.comments, text)
	}
}

var (
	reConfigY = regexp.MustCompile(`^` + prefix + `([A-Za-z0-9_]+)=(y|m|(?:-?[0-9]+)|(?:0x[0-9a-fA-F]+)|(?:".*?"))$`)
	reConfigN = regexp.MustCompile(`^# ` + prefix + `([A-Za-z0-9_]+) is not set$`)
)
