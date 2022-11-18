// SPDX-License-Identifier: BSD-3-Clause
//
// Authors: Cezar Craciunoiu <cezar.craciunoiu@gmail.com>
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

package unset

import (
	"fmt"
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
	"kraftkit.sh/unikraft/app"
)

type UnsetOptions struct {
	PackageManager func(opts ...packmanager.PackageManagerOption) (packmanager.PackageManager, error)
	Logger         func() (log.Logger, error)
	IO             *iostreams.IOStreams

	// Command-line arguments
	Workdir string
}

func UnsetCmd(f *cmdfactory.Factory) *cobra.Command {
	opts := &UnsetOptions{
		PackageManager: f.PackageManager,
		Logger:         f.Logger,
		IO:             f.IOStreams,
	}

	cmd, err := cmdutil.NewCmd(f, "unset")
	if err != nil {
		panic("could not initialize 'kraft build unset' command")
	}

	cmd.Short = "Unset a variable for a Unikraft project"
	cmd.Use = "unset [OPTIONS] [param ...]"
	cmd.Aliases = []string{"u"}
	cmd.Long = heredoc.Doc(`
		unset a variable for a Unikraft project`)
	cmd.Example = heredoc.Doc(`
		# Unset variables in the cwd project
		$ kraft build unset LIBDEVFS_DEV_STDOUT LWIP_TCP_SND_BUF

		# Unset variables in a project at a path
		$ kraft build unset -w path/to/app LIBDEVFS_DEV_STDOUT LWIP_TCP_SND_BUF
	`)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		workdir := ""
		confOpts := []string{}

		// Skip if nothing can be unset
		if len(args) == 0 {
			return fmt.Errorf("no options to unset")
		}

		// Set the working directory
		if opts.Workdir != "" {
			workdir = opts.Workdir
		} else {
			workdir, err = os.Getwd()
			if err != nil {
				return err
			}
		}

		for _, arg := range args {
			confOpts = append(confOpts, arg+"=n")
		}

		return unsetRun(opts, workdir, confOpts)
	}

	cmd.Flags().StringVarP(
		&opts.Workdir,
		"workdir", "w",
		"",
		"Work on a unikernel at a path",
	)

	return cmd
}

func unsetRun(copts *UnsetOptions, workdir string, confOpts []string) error {
	pm, err := copts.PackageManager()
	if err != nil {
		return err
	}

	plog, err := copts.Logger()
	if err != nil {
		return err
	}

	// Check if dotconfig exists in workdir
	dotconfig := fmt.Sprintf("%s/.config", workdir)

	// Check if the file exists
	// TODO: offer option to start in interactive mode
	if _, err := os.Stat(dotconfig); os.IsNotExist(err) {
		return fmt.Errorf("dotconfig file does not exist: %s", dotconfig)
	}

	// Initialize at least the configuration options for a project
	projectOpts, err := app.NewProjectOptions(
		nil,
		app.WithLogger(plog),
		app.WithWorkingDirectory(workdir),
		app.WithDefaultConfigPath(),
		app.WithPackageManager(&pm),
		app.WithResolvedPaths(true),
		app.WithDotConfig(true),
		app.WithConfig(confOpts),
	)
	if err != nil {
		return err
	}

	// Interpret the application
	project, err := app.NewApplicationFromOptions(projectOpts)
	if err != nil {
		return err
	}

	return project.Unset(
		make.WithExecOptions(
			exec.WithStdin(copts.IO.In),
		),
	)
}
