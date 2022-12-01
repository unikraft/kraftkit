// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package make

import (
	"context"
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
func (m *Make) Execute(ctx context.Context) error {
	return m.seq.StartAndWait(ctx)
}
