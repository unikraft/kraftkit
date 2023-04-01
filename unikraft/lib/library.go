// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package lib

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	"kraftkit.sh/kconfig"
	"kraftkit.sh/log"
	"kraftkit.sh/make"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/component"
)

type Library interface {
	component.Component

	// Exportsyms contains the list of exported symbols the library makes
	// available via the standard `exportsyms.uk` file.
	Exportsyms() []string

	// Syscalls contains the list of provided syscalls by the library.
	Syscalls() []*unikraft.ProvidedSyscall

	// IsInternal dictates whether the library comes from the Unikraft core
	// repository.
	IsInternal() bool

	// Platform is a pointer to a string, which indicates whether the library is a
	// "platform library" and therefore specific to a platform.  When nil, it is
	// not specific to a platform.
	Platform() *string

	// ASFlags
	ASFlags() []*make.ConditionalValue

	// ASIncludes
	ASIncludes() []*make.ConditionalValue

	// CFlags
	CFlags() []*make.ConditionalValue

	// CIncludes
	CIncludes() []*make.ConditionalValue

	// CXXFlags
	CXXFlags() []*make.ConditionalValue

	// CXXIncludes
	CXXIncludes() []*make.ConditionalValue

	// Objs
	Objs() []*make.ConditionalValue

	// ObjCFlags
	ObjCFlags() []*make.ConditionalValue

	// Srcs
	Srcs() []*make.ConditionalValue
}

type LibraryConfig struct {
	// name of the library.
	name string

	// version of the library.
	version string

	// source of the library (can be either remote or local, this attribute is
	// ultimately handled by the packmanager).
	source string

	// TODO(nderjung): future implementation
	// origin contains the URL of the remote source code if this library wraps an
	// existing library.
	//nolint:gofumpt
	//origin string

	// list of kconfig values specific to this library.
	kconfig kconfig.KeyValueMap

	// kname the kconfig name which enables this library.
	kname string

	// TODO(nderjung): future implementation
	// projectdir is the root location of the project that this library is a
	// member of.
	//nolint:gofumpt
	//projectdir string

	// path is the location to this library within the context of a project.
	path string

	// TODO(nderjung): future implementation
	// patchdir is the directory where patches to the origin library are kept.
	//nolint:gofumpt
	//patchdir string

	// exportsyms contains the list of exported symbols the library makes
	// available via the standard `exportsyms.uk` file.
	exportsyms []string

	// syscalls contains the list of provided syscalls by the library.
	syscalls []*unikraft.ProvidedSyscall

	// internal dictates whether the library comes from the Unikraft core
	// repository.
	internal bool

	// platform is a pointer to a string, which indicates whether the library is a
	// "platform library" and therefore specific to a platform.  When nil, it is
	// not specific to a platform.
	platform *string

	// asflags contains the list of AS flags used during the compilation of the
	// library.
	asflags []*make.ConditionalValue

	// asincludes contains the list of AS include paths used during the
	// compilation of the library.
	asincludes []*make.ConditionalValue

	// cflags contains the list of CC flags used during the compilation of the
	// library.
	cflags []*make.ConditionalValue

	// cincludes contains the list of include directories used during the
	// compilation of the library.
	cincludes []*make.ConditionalValue

	// cxxflags contains the list of CXX flags used during the compilation of the
	// library.
	cxxflags []*make.ConditionalValue

	// cxxincludes contains the list of CXX include directories used during the
	// compilation of the library.
	cxxincludes []*make.ConditionalValue

	// objs contains the list of object targets for this library.
	objs []*make.ConditionalValue

	// objcflags contains the list of object CC flags used during the compilation
	// of objects.
	objcflags []*make.ConditionalValue

	// srcs contains the list of source files of this library.
	srcs []*make.ConditionalValue
}

type Libraries map[string]*LibraryConfig

func (lc LibraryConfig) Name() string {
	return lc.name
}

func (lc LibraryConfig) Source() string {
	return lc.source
}

func (lc LibraryConfig) Version() string {
	return lc.version
}

func (lc LibraryConfig) Type() unikraft.ComponentType {
	return unikraft.ComponentTypeLib
}

func (lc LibraryConfig) Path() string {
	return lc.path
}

func (lc LibraryConfig) KConfigTree(_ context.Context, env ...*kconfig.KeyValue) (*kconfig.KConfigFile, error) {
	config_uk := filepath.Join(lc.Path(), unikraft.Config_uk)
	if _, err := os.Stat(config_uk); err != nil {
		return nil, fmt.Errorf("could not read component Config.uk: %v", err)
	}

	return kconfig.Parse(config_uk, lc.kconfig.Override(env...).Slice()...)
}

func (lc LibraryConfig) KConfig() kconfig.KeyValueMap {
	values := kconfig.KeyValueMap{}
	values.OverrideBy(lc.kconfig)

	// TODO(craciunoiuc): Temporary check as not all libraries follow the same
	// naming convention. Will be replaced by the kconfig parser.
	// See: https://github.com/unikraft/kraftkit/issue/653
	if strings.HasPrefix(strings.ToUpper(lc.name), "LIB") {
		values.Set(kconfig.Prefix+strings.ToUpper(lc.name), kconfig.Yes)
	} else {
		values.Set(kconfig.Prefix+"LIB"+strings.ToUpper(lc.name), kconfig.Yes)
	}

	return values
}

func (lc LibraryConfig) IsUnpacked() bool {
	if f, err := os.Stat(lc.Path()); err == nil && f.IsDir() {
		return true
	}

	return false
}

func (lc LibraryConfig) PrintInfo(ctx context.Context) string {
	return "not implemented: unikraft.lib.LibraryConfig.PrintInfo"
}

// NewFromDir returns all the registering libraries from a given directory.
// The method can return multiple libraries if they are detected from a
// top-level Makefile.uk/Config.uk pair which can arbitrarily register any
// number of libraries.
func NewFromDir(ctx context.Context, dir string, opts ...LibraryOption) (Libraries, error) {
	libs := Libraries{}

	makefile_uk := filepath.Join(dir, unikraft.Makefile_uk)
	if _, err := os.Stat(makefile_uk); err != nil {
		return nil, fmt.Errorf("cannot parse library from directory without Makefile.uk")
	}

	// TODO: Parse the Config.uk file to grab even more information, e.g.
	// dependencies and description.
	// config_uk := filepath.Join(dir, unikraft.Config_uk)
	// if _, err := os.Stat(config_uk); err != nil {
	// 	return nil, fmt.Errorf("cannot parse library from directory without Config.uk")
	// }

	log.G(ctx).WithFields(logrus.Fields{
		"file": makefile_uk,
	}).Trace("reading")

	fm, err := os.Open(makefile_uk)
	if err != nil {
		return nil, fmt.Errorf("could not open Makefile.uk: %v", err)
	}

	defer fm.Close()

	// Start by parsing each line for match additions, this can help us sort
	// information later on when we seek for registrations.
	const NO_COMPOSITE = ""
	type addition struct {
		composite string
		flag      string
		condition string
		matches   []string
	}
	additions := map[string][]addition{}

	scanner := bufio.NewScanner(fm)
	for scanner.Scan() {
		line := scanner.Text()

		if !strings.Contains(line, "+=") {
			continue
		}

		for strings.Contains(scanner.Text(), "\\") {
			scanner.Scan()
			line += " " + scanner.Text()
		}

		match, composite, flag, condition, matches := MatchAddition(line)

		if !match {
			continue
		}

		if _, ok := additions[composite]; !ok {
			additions[composite] = []addition{}
		}

		additions[composite] = append(additions[composite], addition{
			composite: composite,
			flag:      flag,
			condition: condition,
			matches:   matches,
		})
	}

	// Reset the scanner and search line-by-line so we can contextualize how many
	// libraries we are about to parse.
	if _, err = fm.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	scanner = bufio.NewScanner(fm)
	for scanner.Scan() {
		match, plat, libname, _ := MatchRegistrationLine(scanner.Text())
		if !match {
			continue
		}

		lib := LibraryConfig{
			name:     strings.TrimPrefix(libname, "lib"),
			platform: plat,
			path:     dir,
		}

		kname := strings.ToUpper(libname)
		lib.kname = kconfig.Prefix + kname

		for _, extra := range additions[kname] {
			cvs := []*make.ConditionalValue{}
			for _, m := range extra.matches {
				cv := &make.ConditionalValue{
					Value: m,
				}
				if extra.condition != "y" {
					cv.DependsOn = &extra.condition
				}
				cvs = append(cvs, cv)
			}

			switch extra.flag {
			case ASFLAGS:
				lib.asflags = append(lib.asflags, cvs...)
			case ASINCLUDES:
				lib.asincludes = append(lib.asincludes, cvs...)
			case CFLAGS:
				lib.cflags = append(lib.cflags, cvs...)
			case CINCLUDES:
				lib.cincludes = append(lib.cincludes, cvs...)
			case CXXFLAGS:
				lib.cxxflags = append(lib.cxxflags, cvs...)
			case CXXINCLUDES:
				lib.cxxincludes = append(lib.cxxincludes, cvs...)
			case OBJS:
				lib.objs = append(lib.objs, cvs...)
			case OBJCFLAGS:
				lib.objcflags = append(lib.objcflags, cvs...)
			case SRCS:
				lib.srcs = append(lib.srcs, cvs...)
			}
		}

		libs[libname] = &lib
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// If there is only one library that has been registered, all global flags and
	// includes can be associated.
	onlylib := ""
	if len(libs) == 1 {
		for lib := range libs {
			onlylib = lib
			break
		}

		// Match exported syscalls
		for _, syscalls := range additions[unikraft.UK_PROVIDED_SYSCALLS] {
			for _, syscall := range syscalls.matches {
				provided := unikraft.NewProvidedSyscall(syscall)
				if provided == nil {
					continue // TODO: Warn, error?
				}

				libs[onlylib].syscalls = append(libs[onlylib].syscalls, provided)
			}
		}

		for _, extra := range additions[NO_COMPOSITE] {
			cvs := []*make.ConditionalValue{}
			for _, m := range extra.matches {
				cv := &make.ConditionalValue{
					Value: m,
				}
				if extra.condition != "y" {
					cv.DependsOn = &extra.condition
				}
				cvs = append(cvs, cv)
			}

			switch extra.flag {
			case ASFLAGS:
				libs[onlylib].asflags = append(libs[onlylib].asflags, cvs...)
			case ASINCLUDES:
				libs[onlylib].asincludes = append(libs[onlylib].asincludes, cvs...)
			case CFLAGS:
				libs[onlylib].cflags = append(libs[onlylib].cflags, cvs...)
			case CINCLUDES:
				libs[onlylib].cincludes = append(libs[onlylib].cincludes, cvs...)
			case CXXFLAGS:
				libs[onlylib].cxxflags = append(libs[onlylib].cxxflags, cvs...)
			case CXXINCLUDES:
				libs[onlylib].cxxincludes = append(libs[onlylib].cxxincludes, cvs...)
			case OBJS:
				libs[onlylib].objs = append(libs[onlylib].objs, cvs...)
			case OBJCFLAGS:
				libs[onlylib].objcflags = append(libs[onlylib].objcflags, cvs...)
			case SRCS:
				libs[onlylib].srcs = append(libs[onlylib].srcs, cvs...)
			}
		}

		exportsyms_uk := filepath.Join(dir, unikraft.Exportsyms_uk)
		if _, err := os.Stat(exportsyms_uk); err == nil {
			log.G(ctx).WithFields(logrus.Fields{
				"file": exportsyms_uk,
			}).Trace("reading")
			fe, err := os.Open(exportsyms_uk)
			if err != nil {
				return nil, fmt.Errorf("could not open exportsyms.uk: %v", err)
			}

			defer fe.Close()

			scanner = bufio.NewScanner(fe)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if len(line) == 0 || line[0] == '#' {
					continue
				}

				libs[onlylib].exportsyms = append(libs[onlylib].exportsyms, line)
			}
		}
	}

	for k, lib := range libs {
		for _, opt := range opts {
			if err := opt(lib); err != nil {
				return nil, err
			}
		}

		libs[k] = lib
	}

	return libs, nil
}

func (lib LibraryConfig) Exportsyms() []string {
	return lib.exportsyms
}

func (lib LibraryConfig) Syscalls() []*unikraft.ProvidedSyscall {
	return lib.syscalls
}

func (lib LibraryConfig) IsInternal() bool {
	return lib.internal
}

func (lib LibraryConfig) Platform() *string {
	return lib.platform
}

func (lib LibraryConfig) ASFlags() []*make.ConditionalValue {
	return lib.asflags
}

func (lib LibraryConfig) ASIncludes() []*make.ConditionalValue {
	return lib.asincludes
}

func (lib LibraryConfig) CFlags() []*make.ConditionalValue {
	return lib.cflags
}

func (lib LibraryConfig) CIncludes() []*make.ConditionalValue {
	return lib.cincludes
}

func (lib LibraryConfig) CXXFlags() []*make.ConditionalValue {
	return lib.cxxflags
}

func (lib LibraryConfig) CXXIncludes() []*make.ConditionalValue {
	return lib.cxxincludes
}

func (lib LibraryConfig) Objs() []*make.ConditionalValue {
	return lib.objs
}

func (lib LibraryConfig) ObjCFlags() []*make.ConditionalValue {
	return lib.objcflags
}

func (lib LibraryConfig) Srcs() []*make.ConditionalValue {
	return lib.srcs
}

// MarshalYAML makes LibraryConfig implement yaml.Marshaller
func (lc LibraryConfig) MarshalYAML() (interface{}, error) {
	ret := map[string]interface{}{
		"version": lc.version,
	}

	if lc.kconfig != nil && len(lc.kconfig) > 0 {
		ret["kconfig"] = lc.kconfig
	}

	return ret, nil
}

func LibraryWithVersion(name string, version string) LibraryConfig {
	return LibraryConfig{
		name:    name,
		version: version,
	}
}
