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

package app

import (
	"fmt"
	"os"

	"kraftkit.sh/exec"
	"kraftkit.sh/log"
	"kraftkit.sh/make"
	"kraftkit.sh/unikraft/target"
)

type BuildOptions struct {
	log          log.Logger
	target       []target.TargetConfig
	mopts        []make.MakeOption
	onProgress   func(progress float64)
	noSyncConfig bool
	noFetch      bool
}

type BuildOption func(opts *BuildOptions) error

// WithBuildLogger sets the logger which can be used by the underlying build
// mechanisms
func WithBuildLogger(l log.Logger) BuildOption {
	return func(bo *BuildOptions) error {
		bo.log = l
		bo.mopts = append(bo.mopts,
			make.WithLogger(l),
		)
		return nil
	}
}

// WithBuildTarget specifies one or many of the listed targets defined by the
// application's Kraftfile
func WithBuildTarget(targets ...target.TargetConfig) BuildOption {
	return func(bo *BuildOptions) error {
		bo.target = append(bo.target, targets...)
		return nil
	}
}

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
			return fmt.Errorf("could not create or open build log file: %v", err)
		}

		bo.mopts = append(bo.mopts, make.WithExecOptions(
			exec.WithStdoutCallback(&saveBuildLog{file}),
		))

		return nil
	}
}

// WithBuildNoSyncConfig disables calling `make syncconfig` befere invoking the
// main Unikraft's build invocation.
func WithBuildNoSyncConfig(noSyncConfig bool) BuildOption {
	return func(bo *BuildOptions) error {
		bo.noSyncConfig = noSyncConfig
		return nil
	}
}

// WithBuildNoFetch disables calling `make fetch` befere invoking the
// main Unikraft's build invocation.
func WithBuildNoFetch(noFetch bool) BuildOption {
	return func(bo *BuildOptions) error {
		bo.noFetch = noFetch
		return nil
	}
}
