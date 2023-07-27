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
	"kraftkit.sh/machine/platform"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/unikraft/app"
	"kraftkit.sh/unikraft/target"
)

type Clean struct {
	Architecture string `long:"arch" short:"m" usage:"Filter prepare based on a target's architecture"`
	Kraftfile    string `long:"kraftfile" usage:"Set an alternative path of the Kraftfile"`
	Platform     string `long:"plat" short:"p" usage:"Filter prepare based on a target's platform"`
	Target       string `long:"target" short:"t" usage:"Filter prepare based on a specific target"`
}

func New() *cobra.Command {
	cmd, err := cmdfactory.New(&Clean{}, cobra.Command{
		Short: "Remove the build object files of a Unikraft project",
		Use:   "clean [DIR]",
		Args:  cmdfactory.MaxDirArgs(1),
		Long: heredoc.Doc(`
			Remove the build object files of a Unikraft project`),
		Example: heredoc.Doc(`
			# Clean the cwd build directory
			$ kraft clean

			# clean a project at a path
			$ kraft clean path/to/app`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "build",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *Clean) Pre(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	pm, err := packmanager.NewUmbrellaManager(ctx)
	if err != nil {
		return err
	}

	cmd.SetContext(packmanager.WithPackageManager(ctx, pm))

	opts.Platform = platform.PlatformByName(opts.Platform).String()

	return nil
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

	popts := []app.ProjectOption{
		app.WithProjectWorkdir(workdir),
	}

	if len(opts.Kraftfile) > 0 {
		popts = append(popts, app.WithProjectKraftfile(opts.Kraftfile))
	} else {
		popts = append(popts, app.WithProjectDefaultKraftfiles())
	}

	// Initialize at least the configuration options for a project
	project, err := app.NewProjectFromOptions(ctx, popts...)
	if err != nil {
		return err
	}

	// Filter project targets by any provided CLI options
	targets := target.Filter(
		project.Targets(),
		opts.Architecture,
		opts.Platform,
		opts.Target,
	)

	t, err := target.Select(targets)
	if err != nil {
		return err
	}

	return project.Clean(ctx, t)
}
