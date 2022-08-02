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

package make

import (
	"fmt"
	"reflect"
	"strings"

	"kraftkit.sh/exec"
)

const DefaultBinaryName = "make"

type export struct {
	export    string
	omitempty bool
	def       string
}

func parseExport(tag reflect.StructTag) (*export, error) {
	parts := strings.Split(tag.Get("export"), ",")
	if len(parts) == 0 {
		return nil, fmt.Errorf("could not identify export tag")
	}

	e := &export{
		export: parts[0],
	}

	def := tag.Get("default")
	if len(def) > 0 {
		e.def = def
	}

	for _, part := range parts[1:] {
		switch true {
		case part == "omitempty":
			e.omitempty = true
		}
	}

	return e, nil
}

type Make struct {
	opts *MakeOptions
	seq  *exec.SequentialProcesses
	cpw  *calculateProgressWriter
}

// NewFromInterface prepares a GNU Make command call by parsing the input
// interface searching for `export` annotations within each attribute's tag.
func NewFromInterface(args interface{}, mopts ...MakeOption) (*Make, error) {
	t := reflect.TypeOf(args)
	v := reflect.ValueOf(args)

	if v.Kind() == reflect.Ptr {
		return nil, fmt.Errorf("cannot derive interface arguments from pointer: passed by reference")
	}

	for i := 0; i < t.NumField(); i++ {
		e, err := parseExport(t.Field(i).Tag)
		if err != nil {
			return nil, fmt.Errorf("could not parse export tag: %s", err)
		}

		if len(e.export) > 0 {
			val := v.Field(i).String()
			if len(val) == 0 && len(e.def) > 0 {
				val = e.def
			}

			if e.omitempty && len(val) == 0 {
				continue
			}

			mopts = append(mopts,
				WithVar(e.export, val),
			)
		}
	}

	var err error
	make := &Make{}

	make.opts, err = NewMakeOptions(mopts...)
	if err != nil {
		return nil, err
	}

	if len(make.opts.bin) == 0 {
		make.opts.bin = DefaultBinaryName
	}

	var processes []*exec.Process
	var calcProgressExec *exec.Executable

	// The trick to determining the progress of the execution of the make
	// invocation is to first call `make -n` and read the number of lines.  Set up
	// a sequential call to first invoke this execution.  The exec library's
	// SequentialProcesses will handle correctly invoking the command under the
	// same conditions.
	if make.opts.onProgress != nil && !make.opts.justPrint {
		popts, err := NewMakeOptions(
			WithJustPrint(true),
			WithDirectory(make.opts.directory),
			WithBinPath(make.opts.bin),
			WithVars(make.opts.vars),
			WithTarget(make.opts.targets...),
		)
		if err != nil {
			return nil, err
		}

		calcProgressExec, err = exec.NewExecutable(make.opts.bin, *popts, popts.Vars()...)
		if err != nil {
			return nil, err
		}

		onProgressCallback := &onProgressWriter{
			onProgress: make.opts.onProgress,
		}
		make.cpw = &calculateProgressWriter{}
		calcProgressProcess, err := exec.NewProcessFromExecutable(
			calcProgressExec,
			append(make.opts.eopts,
				exec.WithStdout(make.cpw),
				exec.WithOnExitCallback(func(exitCode int) {
					if exitCode != 0 {
						return
					}

					onProgressCallback.total = make.cpw.totalLines
				}),
			)...,
		)
		if err != nil {
			return nil, err
		}

		make.opts.eopts = append(make.opts.eopts,
			exec.WithStdoutCallback(onProgressCallback),
		)

		processes = append(processes, calcProgressProcess)
	}

	mainExec, err := exec.NewExecutable(make.opts.bin, *make.opts, make.opts.Vars()...)
	if err != nil {
		return nil, err
	}

	mainProcess, err := exec.NewProcessFromExecutable(mainExec, make.opts.eopts...)
	if err != nil {
		return nil, err
	}

	seq, err := exec.NewSequential(append(processes, mainProcess)...)
	if err != nil {
		return nil, err
	}

	make.seq = seq

	return make, nil
}

// Execute starts and waits on the prepared make invocation
func (m *Make) Execute() error {
	return m.seq.StartAndWait()
}
