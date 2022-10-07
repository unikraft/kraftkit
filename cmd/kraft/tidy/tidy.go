// SPDX-License-Identifier: BSD-3-Clause
//
// Authors: Cezar Craciunoiu <cezar.craciunoiu@unikraft.io>
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

package tidy

import (
	"fmt"
	"os"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cmdfactory"
	"kraftkit.sh/internal/cmdutil"
	"kraftkit.sh/schema"
)

type tidyOptions struct {
	ConfigManager func() (*config.ConfigManager, error)

	TidyConfig bool
}

func TidyCmd(f *cmdfactory.Factory) *cobra.Command {
	cmd, err := cmdutil.NewCmd(f, "tidy")
	if err != nil {
		panic("could not initialize 'tidy' commmand")
	}

	opts := &tidyOptions{
		ConfigManager: f.ConfigManager,
	}

	cmd.Short = "Clean up kraftkit and unikraft configuration files"
	cmd.Use = "tidy [FLAGS] [DIR]"
	cmd.Args = cmdutil.MaxDirArgs(1)
	cmd.Long = heredoc.Docf(`
		Clean up kraftkit and unikraft configuration files.

		The default behaviour of %[1]skraft tidy%[1]s is to tidy a kraftfile. Given no
		arguments, it will clean up in the current directory. %[1]skraft tidy%[1]s can
		also be used to tidy kraftkit configuration files.
	`, "`")
	cmd.Example = heredoc.Doc(`
		# Clean the current unikraft project kraftfile (cwd)
		$ kraft tidy

		# Clean a unikraft project kraftfile at a given path
		$ kraft tidy path/to/app

		# Clean the kraftkit configuration files (default path)
		$ kraft tidy --config

		# Clean the kraftkit configuration files at a given path
		$ kraft tidy --config path/to/config
	`)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		var err error
		var workdir string
		if len(args) == 0 {
			workdir, err = os.Getwd()
			if err != nil {
				return err
			}
		} else {
			workdir = args[0]
		}

		return tidyRun(opts, workdir)
	}

	cmd.Flags().BoolVar(
		&opts.TidyConfig,
		"config",
		false,
		"Clean the kraftkit configuration files instead",
	)

	return cmd
}

func tidyRun(opts *tidyOptions, workdir string) error {
	var err error

	cfgm, err := opts.ConfigManager()
	if err != nil {
		return err
	}

	// TODO: tidy kraftkit configuration files
	if opts.TidyConfig {
		return fmt.Errorf("not implemented yet")
	}

	// Initialize at least the configuration options for a project
	projectOpts, err := schema.NewProjectOptions(
		nil,
		schema.WithWorkingDirectory(workdir),
		schema.WithDefaultConfigPath(),
		schema.WithResolvedPaths(true),
	)
	if err != nil {
		return err
	}

	if !schema.IsWorkdirInitialized(workdir) {
		return fmt.Errorf("cannot build uninitialized project! start with: kraft build init")
	}

	// Interpret the application
	project, err := schema.NewApplicationFromOptions(projectOpts)
	if err != nil {
		return err
	}

	if err := schema.SaveApplicationConfig(project,
		schema.WithSaveOldConfig(cfgm.Config.SaveOldKraftfile),
	); err != nil {
		return err
	}

	return nil
}
