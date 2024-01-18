// SPDX-License-Identifier: Apache-2.0
// Copyright 2020 syzkaller project authors. All rights reserved.
// Copyright 2020 The Compose Specification Authors.
// Copyright 2022 Unikraft GmbH. All rights reserved.

package kconfig

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

const DotConfigFileName = ".config"

// KeyValueMap is a map of KeyValues
type KeyValueMap map[string]*KeyValue

// NewKeyValueMapFromSlice build a new Mapping from a set of KEY=VALUE strings
func NewKeyValueMapFromSlice(values ...interface{}) (KeyValueMap, error) {
	mapping := KeyValueMap{}

	for _, value := range values {
		var str string
		switch t := value.(type) {
		case string:
			str = t
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			str = fmt.Sprintf("%d", t)
		}
		tokens := strings.SplitN(str, "=", 2)
		if len(tokens) > 1 && tokens[1] != "" {
			mapping[tokens[0]] = &KeyValue{
				Key:   tokens[0],
				Value: tokens[1],
			}
		} else {
			return nil, fmt.Errorf("kconfig option must be a key-value pair(key=value), found: %v", str)
		}
	}

	return mapping, nil
}

// NewKeyValueMapFromMap build a new Mapping from a set of KEY=VALUE strings
func NewKeyValueMapFromMap(values map[string]interface{}) (KeyValueMap, error) {
	mapping := KeyValueMap{}
	for key, value := range values {
		mapping[key] = &KeyValue{
			Key: key,
		}

		if value == nil {
			return nil, fmt.Errorf("kconfig option must have a value, on key: %v", key)
		}

		switch casting := value.(type) {
		case string:
			mapping[key].Value = casting
		case bool:
			v := "n"
			if casting {
				v = "y"
			}
			mapping[key].Value = v
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			mapping[key].Value = fmt.Sprintf("%d", casting)
		default:
			mapping[key].Value = fmt.Sprintf("%s", casting)
		}
	}

	return mapping, nil
}

// Override accepts a list of key value pairs and overrides the key in the map
func (kvm KeyValueMap) Override(extra ...*KeyValue) KeyValueMap {
	for _, kv := range extra {
		kvm[kv.Key] = kv
	}

	return kvm
}

// OverrideBy update KeyValueMap with values from another KeyValueMap
func (kvm KeyValueMap) OverrideBy(other KeyValueMap) KeyValueMap {
	for k, v := range other {
		kvm[k] = v
	}
	return kvm
}

// NewKConfigValuesFromFile build a KConfigValues from a provided file path
func NewKeyValueMapFromFile(file string) (KeyValueMap, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, fmt.Errorf("could not open file: %v", err)
	}

	defer f.Close()

	ret := KeyValueMap{}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		k, v := NewKeyValue(scanner.Text())
		if v == nil {
			continue
		}

		ret[k] = v
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return ret, nil
}

// Slice returns the map as a slice
func (kvm KeyValueMap) Slice() []*KeyValue {
	var slice []*KeyValue
	for _, kv := range kvm {
		slice = append(slice, kv)
	}
	return slice
}

// Set a new key with specified value
func (kvm KeyValueMap) Set(key, value string) KeyValueMap {
	kvm[key] = &KeyValue{
		Key:   key,
		Value: value,
	}

	return kvm
}

// Unset a specific key
func (kvm KeyValueMap) Unset(key string) KeyValueMap {
	delete(kvm, key)

	return kvm
}

// Resolve update a KConfig for keys without value (`key`, but not `key=`)
func (kvm KeyValueMap) Resolve(lookupFn func(string) (string, bool)) KeyValueMap {
	for k, v := range kvm {
		if v != nil {
			continue
		}

		value, ok := lookupFn(k)
		if !ok {
			continue
		}

		kvm[k] = &KeyValue{
			Key:   k,
			Value: value,
		}
	}

	return kvm
}

// RemoveEmpty excludes keys that are not associated with a value
func (kvm KeyValueMap) RemoveEmpty() KeyValueMap {
	for k, v := range kvm {
		if v == nil || v.Value == "" {
			delete(kvm, k)
		}
	}

	return kvm
}

// Get returns a KeyValue based on a key and a boolean result value if the
// entries was resolvable.
func (kvm KeyValueMap) Get(key string) (*KeyValue, bool) {
	if val, ok := kvm[key]; ok {
		return val, true
	}

	// Attempt with a `CONFIG_` prefix
	if val, ok := kvm[fmt.Sprintf("%s%s", Prefix, key)]; ok {
		return val, true
	}

	return nil, false
}

// AnyYes accepts an input list of keys which are all checked against the
// KConfig value for "y" (meaning "yes" or "true").  If any of the keys are set
// to this value, this method returns true.
func (kvm KeyValueMap) AnyYes(keys ...string) bool {
	for _, key := range keys {
		if val, ok := kvm[key]; ok && val.Value == Yes {
			return true
		}
	}

	return false
}

// AllNoOrUnset accepts an input list of keys which are all checked against not
// having the value for "n" (meaning "no" or "false") or whether they are unset.
// If any of the keys are set to this value, this method returns false.
func (kvm KeyValueMap) AllNoOrUnset(keys ...string) bool {
	for _, key := range keys {
		if val, ok := kvm[key]; ok && val.Value != No {
			return false
		}
	}

	return true
}

// String returns the serialized string representing a .config file
func (kvm KeyValueMap) String() string {
	var ret strings.Builder

	for _, v := range kvm {
		if v.Value == "n" {
			ret.WriteString("# ")
			ret.WriteString(v.Key)
			ret.WriteString(" is not set")
		} else {
			ret.WriteString(v.Key)
			ret.WriteString("=")
			if v.Value == "y" {
				ret.WriteString(v.Value)
			} else if _, err := strconv.Atoi(v.Value); err == nil {
				ret.WriteString(v.Value)
			} else {
				ret.WriteString("\"")
				ret.WriteString(strings.ReplaceAll(v.Value, "\"", "\\\""))
				ret.WriteString("\"")
			}
		}
		ret.WriteString("\n")
	}

	return ret.String()
}

// MarshalYAML makes KeyValueMap implement yaml.Marshaller
func (kvm KeyValueMap) MarshalYAML() (interface{}, error) {
	return kvm.Slice(), nil
}

// DotConfigFile represents a parsed .config file. It should not be modified
// directly, only by means of calling methods. The only exception is
// Config.Value which may be modified directly. Note: config names don't include
// CONFIG_ prefix, here and in other public interfaces, users of this package
// should never mention CONFIG_. Use Yes/Mod/No consts to check for/set config
// to particular values.
type DotConfigFile struct {
	Slice    []*KeyValue
	Map      KeyValueMap // duplicates Configs for convenience
	comments []string
}

// KeyValue represents a KConfig option with its name and its value.
type KeyValue struct {
	Key      string
	Value    string
	comments []string
}

// NewKeyValue returns a populated KeyValue by parsing the input line
func NewKeyValue(line string) (string, *KeyValue) {
	line = strings.TrimSpace(line)

	// Skip blank lines
	if line == "" {
		return "", nil
	}

	// Skip commented-out lines
	if strings.HasPrefix(line, "#") {
		return "", nil
	}

	tokens := strings.SplitN(line, "=", 2)
	if len(tokens) <= 1 {
		return "", nil
	}

	k := tokens[0]
	v := strings.Join(tokens[1:], "=")
	if strings.HasPrefix(v, "\"") && strings.HasSuffix(v, "\"") {
		v = strings.TrimSuffix(v[1:], "\"")
	}

	return k, &KeyValue{
		Key:   k,
		Value: v,
	}
}

// String implements fmt.Stringer
func (kv KeyValue) String() string {
	return fmt.Sprintf("%s=%s", kv.Key, kv.Value)
}

// MarshalYAML makes KeyValue implement yaml.Marshaller
func (kv *KeyValue) MarshalYAML() (interface{}, error) {
	return fmt.Sprint(kv.Key, "=", kv.Value), nil
}

const (
	Yes    = "y"
	Mod    = "m"
	No     = "n"
	Prefix = "CONFIG_"
)

// Value returns config value, or No if it's not present at all.
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
		cfg = &KeyValue{
			Key:   name,
			Value: val,
		}

		cf.Map[name] = cfg
		cf.Slice = append(cf.Slice, cfg)
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
	for _, cfg := range cf.Slice {
		if cfg.Value == Mod {
			cfg.Value = Yes
		}
	}
}

func (cf *DotConfigFile) ModToNo() {
	for _, cfg := range cf.Slice {
		if cfg.Value == Mod {
			cfg.Value = No
		}
	}
}

func (cf *DotConfigFile) Serialize() []byte {
	buf := new(bytes.Buffer)
	for _, cfg := range cf.Slice {
		for _, comment := range cfg.comments {
			fmt.Fprintf(buf, "%v\n", comment)
		}

		if cfg.Value == No {
			fmt.Fprintf(buf, "# %v%v is not set\n", Prefix, cfg.Key)
		} else {
			fmt.Fprintf(buf, "%v%v=%v\n", Prefix, cfg.Key, cfg.Value)
		}
	}

	for _, comment := range cf.comments {
		fmt.Fprintf(buf, "%v\n", comment)
	}

	return buf.Bytes()
}

func ParseConfig(file string) (*DotConfigFile, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("failed to open .config file %v: %v", file, err)
	}

	return ParseConfigData(data)
}

func ParseConfigData(data []byte) (*DotConfigFile, error) {
	cf := &DotConfigFile{
		Map: make(map[string]*KeyValue),
	}

	s := bufio.NewScanner(bytes.NewReader(data))
	for s.Scan() {
		cf.parseLine(s.Text())
	}

	return cf, nil
}

func (cf *DotConfigFile) Clone() *DotConfigFile {
	cf1 := &DotConfigFile{
		Map:      make(map[string]*KeyValue),
		comments: cf.comments,
	}

	for _, cfg := range cf.Slice {
		cfg1 := new(KeyValue)
		*cfg1 = *cfg
		cf1.Slice = append(cf1.Slice, cfg1)
		cf1.Map[cfg1.Key] = cfg1
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
	reConfigY = regexp.MustCompile(`^` + Prefix + `([A-Za-z0-9_]+)=(y|m|(?:-?[0-9]+)|(?:0x[0-9a-fA-F]+)|(?:".*?"))$`)
	reConfigN = regexp.MustCompile(`^# ` + Prefix + `([A-Za-z0-9_]+) is not set$`)
)
