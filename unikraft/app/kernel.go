// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package app

import (
	"bytes"
	"compress/gzip"
	"context"
	"debug/elf"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strings"

	"kraftkit.sh/kconfig"
	makefile "kraftkit.sh/make"
	"kraftkit.sh/unikraft/core"
	"kraftkit.sh/unikraft/lib"
)

var uk_ibinfo_section_name = ".uk_libinfo"

// This type contains informations about how the components of a unikernel were built.
// Those can be either the core(unikraft) or the libraries, the difference being that
// in the case of the core, the name of the component is empty.
type ComponentInfoRecord struct {
	LibName         string   `yaml:"name,omitempty" json:"name,omitempty"`
	Comment         string   `yaml:"comment,omitempty" json:"comment,omitempty"`
	Version         string   `yaml:"version,omitempty" json:"version,omitempty"`
	License         string   `yaml:"license,omitempty" json:"license,omitempty"`
	GitDesc         string   `yaml:"git_description,omitempty" json:"git_description,omitempty"`
	UkVersion       string   `yaml:"uk_version,omitempty" json:"uk_version,omitempty"`
	UkFullVersion   string   `yaml:"uk_full_version,omitempty" json:"uk_full_version,omitempty"`
	UkCodeName      string   `yaml:"uk_code_name,omitempty" json:"uk_code_name,omitempty"`
	UkConfig        []byte   `yaml:"uk_config,omitempty" json:"uk_config,omitempty"`
	UkConfiggz      []byte   `yaml:"-" json:"-"`
	Compiler        string   `yaml:"compiler,omitempty" json:"compiler,omitempty"`
	CompileDate     string   `yaml:"compile_date,omitempty" json:"compile_date,omitempty"`
	CompiledBy      string   `yaml:"compiled_by,omitempty" json:"compiled_by,omitempty"`
	CompiledByAssoc string   `yaml:"compiled_by_association,omitempty" json:"compiled_by_association,omitempty"`
	CompileFlags    []string `yaml:"compile_flags,omitempty" json:"compile_flags,omitempty"`
}

func libraryFromRecord(libRecord ComponentInfoRecord) (*lib.LibraryConfig, error) {
	version := "unknown"
	if libRecord.Version != "" {
		version = libRecord.Version
	}

	if libRecord.UkVersion != "" {
		version = libRecord.UkVersion
	}

	if libRecord.UkFullVersion != "" {
		version = libRecord.UkFullVersion
	}

	// map the compile flags slice to a make.ConditionalValue slice
	cFlags := make([]*makefile.ConditionalValue, 0)

	for _, flag := range libRecord.CompileFlags {
		cFlags = append(cFlags, &makefile.ConditionalValue{
			DependsOn: nil,
			Value:     flag,
		})
	}

	return lib.NewLibraryConfigFromOptions(lib.WithName(libRecord.LibName),
		lib.WithVersion(version),
		lib.WithLicense(libRecord.License),
		lib.WithCompiler(libRecord.Compiler),
		lib.WithCompileDate(libRecord.CompileDate),
		lib.WithCompiledBy(libRecord.CompiledBy),
		lib.WithCompiledByAssoc(libRecord.CompiledByAssoc),
		lib.WithCFlags(cFlags),
	)
}

func gunzipConfig(configgz []byte) ([]byte, error) {
	buf := bytes.NewBuffer(configgz)
	gz, err := gzip.NewReader(buf)
	if err != nil {
		return nil, err
	}
	defer gz.Close()

	configData, err := io.ReadAll(gz)
	if err != nil {
		return nil, err
	}

	return configData, nil
}

func unikraftFromRecord(ctx context.Context, unikraftRecord ComponentInfoRecord) (*core.UnikraftConfig, error) {
	var configData []byte
	var err error

	if unikraftRecord.UkConfig != nil {
		configData = unikraftRecord.UkConfig
	} else if unikraftRecord.UkConfiggz != nil {
		configData, err = gunzipConfig(unikraftRecord.UkConfiggz)
		if err != nil {
			return nil, err
		}
	}

	config, err := kconfig.ParseConfigData(configData)
	if err != nil {
		return nil, err
	}

	configMap := kconfig.KeyValueMap{}

	for _, cfg := range config.Slice {
		if cfg.Value == kconfig.No {
			continue
		}
		configMap[cfg.Key] = &kconfig.KeyValue{
			Key:   cfg.Key,
			Value: cfg.Value,
		}
	}

	version := "Unknown"
	if unikraftRecord.UkVersion != "" {
		version = unikraftRecord.UkVersion
	}

	if unikraftRecord.UkFullVersion != "" {
		version = unikraftRecord.UkFullVersion
	}

	return core.NewUnikraftFromOptions(ctx, core.WithVersion(version), core.WithKConfig(configMap))
}

func addFieldToRecord(record *ComponentInfoRecord, tnum int, data []byte, byteorder binary.ByteOrder, configFile string) error {
	switch tnum {
	case 0x0001:
		record.LibName = string(data[:len(data)-1])
	case 0x0002:
		record.Comment = string(data[:len(data)-1])
	case 0x0003:
		record.Version = string(data[:len(data)-1])
	case 0x0004:
		record.License = string(data[:len(data)-1])
	case 0x0005:
		record.GitDesc = string(data[:len(data)-1])
	case 0x0006:
		record.UkVersion = string(data[:len(data)-1])
	case 0x0007:
		record.UkFullVersion = string(data[:len(data)-1])
	case 0x0008:
		record.UkCodeName = string(data[:len(data)-1])
	case 0x0009:
		record.UkConfig = data
		if configFile != "" {
			err := os.WriteFile(configFile, record.UkConfig, 0o644)
			if err != nil {
				return err
			}
		}
	case 0x000A:
		record.UkConfiggz = data
		if configFile != "" {
			config, err := gunzipConfig(record.UkConfiggz)
			if err != nil {
				return err
			}

			err = os.WriteFile(configFile, config, 0o644)
			if err != nil {
				return err
			}
		}
	case 0x000B:
		record.Compiler = string(data[:len(data)-1])
	case 0x000C:
		record.CompileDate = string(data[:len(data)-1])
	case 0x000D:
		record.CompiledBy = string(data[:len(data)-1])
	case 0x000E:
		record.CompiledByAssoc = string(data[:len(data)-1])
	case 0x00F:
		// convert the data to an integer based on the byterorder
		flags := byteorder.Uint32(data)

		if flags&0x01 == 0x01 {
			flags ^= 0x01
			record.CompileFlags = append(record.CompileFlags, "PIE")
		}

		if flags&0x02 == 0x02 {
			flags ^= 0x02
			record.CompileFlags = append(record.CompileFlags, "DCE")
		}

		if flags&0x04 == 0x04 {
			flags ^= 0x04
			record.CompileFlags = append(record.CompileFlags, "LTO")
		}

		if flags != 0 {
			record.CompileFlags = append(record.CompileFlags, fmt.Sprintf("UNKNOWN-0x%x", flags))
		}

	default:
	}
	return nil
}

func parseUKLibInfo(ctx context.Context, elfName, configFile, kraftFile string, elfData []byte, ushortSize, uintSize int, byteorder binary.ByteOrder) (*application, error) {
	hdrHdrLen := uintSize + ushortSize
	recHdrLen := ushortSize + uintSize
	seek := 0
	left := len(elfData)

	var err error
	var unikraft *core.UnikraftConfig
	var libraries []*lib.LibraryConfig

	for left >= hdrHdrLen {
		hdrLen := int(byteorder.Uint32(elfData[seek : seek+uintSize]))
		seek += uintSize
		if hdrLen > left {
			return nil, fmt.Errorf("invalid header size at byte position %d", seek)
		}
		hdrVersion := int(byteorder.Uint16(elfData[seek : seek+ushortSize]))
		seek += ushortSize
		left -= hdrLen
		recSeek := seek
		recLeft := hdrLen - hdrHdrLen
		seek += recLeft

		if hdrVersion != 1 {
			continue
		}

		record := ComponentInfoRecord{}

		for recLeft >= recHdrLen {
			recType := int(byteorder.Uint16(elfData[recSeek : recSeek+ushortSize]))
			recSeek += ushortSize
			recLen := int(byteorder.Uint32(elfData[recSeek : recSeek+uintSize]))
			recSeek += uintSize
			if recLen > recLeft {
				return nil, fmt.Errorf("invalid record size at byte position %d", recSeek)
			}
			recData := elfData[recSeek : recSeek+(recLen-recHdrLen)]
			recSeek += len(recData)
			recLeft -= recLen

			err := addFieldToRecord(&record, recType, recData, byteorder, configFile)
			if err != nil {
				return nil, err
			}
		}

		if record.LibName == "" {
			unikraft, err = unikraftFromRecord(ctx, record)
			if err != nil {
				return nil, err
			}
		} else {
			lib, err := libraryFromRecord(record)
			if err != nil {
				return nil, err
			}
			libraries = append(libraries, lib)
		}
	}

	libs := map[string]*lib.LibraryConfig{}

	for _, lib := range libraries {
		libs[lib.Name()] = lib
	}

	name := elfName

	if defname, have := unikraft.KConfig().Get("UK_DEFNAME"); have {
		name = defname.Value
	}

	if ukname, have := unikraft.KConfig().Get("UK_NAME"); have {
		name = ukname.Value
	}

	// Go through all libraries and try to build their own kconfig
	for i, lib := range libs {
		libConfig := kconfig.KeyValueMap{}

		// Iterate through all the configs in core
		for _, cfg := range unikraft.KConfig().Slice() {
			// If the prefix of the config is the upper case version of the library name
			if strings.HasPrefix(strings.ToUpper(cfg.Key), strings.ToUpper(lib.Name())) {
				// Add the config to the library config
				libConfig[cfg.Key] = cfg
				// Remove the config from the core config
				delete(unikraft.KConfig(), cfg.Key)
			}
		}

		libs[i].SetKConfig(libConfig)
	}

	return &application{
		name:      name,
		unikraft:  unikraft,
		libraries: libs,
		kraftfile: &Kraftfile{path: kraftFile},
	}, nil
}

// This function attempts to read the uk_libinfo section of an ELF binary and fill in the fields of an application on a
// best-effort basis.
func NewApplicationFromKernel(ctx context.Context, elfPath, configFile, kraftFile string) (Application, error) {
	fe, err := elf.Open(elfPath)
	if err != nil {
		return nil, err
	}

	var libinfo_section *elf.Section

	for _, section := range fe.Sections {
		if section.Name == uk_ibinfo_section_name {
			libinfo_section = section
		}
	}

	if libinfo_section == nil {
		return nil, fmt.Errorf("no %s section found", uk_ibinfo_section_name)
	}

	elfData, err := libinfo_section.Data()
	if err != nil {
		return nil, err
	}

	elfName := strings.Split(elfPath, "/")[len(strings.Split(elfPath, "/"))-1]
	// TODO: potentially look at the architecture to figure out those sizes
	app, err := parseUKLibInfo(ctx, elfName, configFile, kraftFile, elfData, 2, 4, fe.ByteOrder)

	return app, err
}
