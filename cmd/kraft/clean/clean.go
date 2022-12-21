// SPDX-License-Identifier: BSD-3-Clause
//
// Authors: Cezar Craciunoiu <cezar.craciunoiu@gmail.com>
//
// Copyright (c) 2022, Universitatea Politehnica Bucharest.  All rights reserved.
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

package clean

import (
	"os"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/unikraft/app"
)

type Clean struct{}

func New() *cobra.Command {
	return cmdfactory.New(&Clean{}, cobra.Command{
		Short: "Remove the build object files of a Unikraft project",
		Use:   "clean [DIR]",
		Args:  cmdfactory.MaxDirArgs(1),
		Long: heredoc.Doc(`
			Remove the build object files of a Unikraft project`),
		Example: heredoc.Doc(`
			# Clean the cwd build directory
			$ kraft build clean

			# clean a project at a path
			$ kraft build clean path/to/app`),
		Annotations: map[string]string{
			"help:group": "build",
		},
	})
}

func (opts *Clean) Run(cmd *cobra.Command, args []string) error {
	var err error

	ctx := cmd.Context()
	workdir := ""

	if len(args) == 0 {
		workdir, err = os.Getwd()
		if err != nil {
			return err
		}
	} else {
		workdir = args[0]
	}

	// Initialize at least the configuration options for a project
	project, err := app.NewProjectFromOptions(
		app.WithProjectWorkdir(workdir),
		app.WithProjectDefaultKraftfiles(),
	)
	if err != nil {
		return err
	}

	return project.Clean(ctx, nil)
}
