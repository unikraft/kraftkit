// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package app

import (
	"os"

	"github.com/juju/errors"
	"kraftkit.sh/exec"
	"kraftkit.sh/make"
)

type BuildOptions struct {
	mopts      []make.MakeOption
	onProgress func(progress float64)
	noPrepare  bool
}

type BuildOption func(opts *BuildOptions) error

// WithBuildMakeOptions allows customization of the invocation of the GNU make
// tool
func WithBuildMakeOptions(mopts ...make.MakeOption) BuildOption {
	return func(bo *BuildOptions) error {
		bo.mopts = append(bo.mopts, mopts...)
		return nil
	}
}

// WithBuildProgressFunc sets an optional progress function which is used as a
// callback during the ultimate invocation of make within Unikraft's build
// system
func WithBuildProgressFunc(onProgress func(progress float64)) BuildOption {
	return func(bo *BuildOptions) error {
		bo.onProgress = onProgress
		return nil
	}
}

type saveBuildLog struct {
	file *os.File
}

func (sbl *saveBuildLog) Write(b []byte) (int, error) {
	return sbl.file.Write(b)
}

// WithBuildLogFile specifies a path to a file which will be used to save the
// output from Unikraft's build invocation
func WithBuildLogFile(path string) BuildOption {
	return func(bo *BuildOptions) error {
		if len(path) == 0 {
			return nil
		}

		file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o666)
		if err != nil {
			return errors.Annotate(err, "could not create or open build log file")
		}

		bo.mopts = append(bo.mopts, make.WithExecOptions(
			exec.WithStdoutCallback(&saveBuildLog{file}),
		))

		return nil
	}
}

// WithBuildNoPrepare disables calling `make prepare` befere invoking the
// main Unikraft's build invocation.
func WithBuildNoPrepare(noPrepare bool) BuildOption {
	return func(bo *BuildOptions) error {
		bo.noPrepare = noPrepare
		return nil
	}
}
