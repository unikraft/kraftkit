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

package menuconfig

import (
	"os"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/exec"
	"kraftkit.sh/internal/cmdfactory"
	"kraftkit.sh/internal/cmdutil"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
	"kraftkit.sh/make"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/schema"
)

type MenuConfigOptions struct {
	PackageManager func(opts ...packmanager.PackageManagerOption) (packmanager.PackageManager, error)
	Logger         func() (log.Logger, error)
	IO             *iostreams.IOStreams
}

func MenuConfigCmd(f *cmdfactory.Factory) *cobra.Command {
	opts := &MenuConfigOptions{
		PackageManager: f.PackageManager,
		Logger:         f.Logger,
		IO:             f.IOStreams,
	}

	cmd, err := cmdutil.NewCmd(f, "menuconfig")
	if err != nil {
		panic("could not initialize 'ukbuild menuconfig' commmand")
	}

	cmd.Short = "menuconfig open's Unikraft configuration editor TUI"
	cmd.Use = "menuconfig [DIR]"
	cmd.Aliases = []string{"m", "menu"}
	cmd.Args = cmdutil.MaxDirArgs(1)
	cmd.Long = heredoc.Doc(`
		Open Unikraft's configuration editor TUI`)
	cmd.Example = heredoc.Doc(`
		# Open the menuconfig in the cwd project
		$ ukbuild menuconfig
		
		# Open the menuconfig for a project at a path
		$ ukbuild menu path/to/app
	`)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		workdir := ""

		if len(args) == 0 {
			workdir, err = os.Getwd()
			if err != nil {
				return err
			}
		} else {
			workdir = args[0]
		}

		return menuConfigRun(opts, workdir)
	}

	return cmd
}

func menuConfigRun(mcopts *MenuConfigOptions, workdir string) error {
	pm, err := mcopts.PackageManager()
	if err != nil {
		return err
	}

	plog, err := mcopts.Logger()
	if err != nil {
		return err
	}

	// Initialize at least the configuration options for a project
	projectOpts, err := schema.NewProjectOptions(
		nil,
		schema.WithLogger(plog),
		schema.WithWorkingDirectory(workdir),
		schema.WithDefaultConfigPath(),
		schema.WithPackageManager(&pm),
		schema.WithResolvedPaths(true),
		schema.WithDotConfig(false),
	)
	if err != nil {
		return err
	}

	// Interpret the application
	project, err := schema.NewApplicationFromOptions(projectOpts)
	if err != nil {
		return err
	}

	return project.Make(
		make.WithExecOptions(
			exec.WithStdin(mcopts.IO.In),
			exec.WithStdout(mcopts.IO.Out),
		),
		make.WithTarget("menuconfig"),
	)
}
