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

package build

import (
	"fmt"
	"os"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/internal/cmdfactory"
	"kraftkit.sh/internal/cmdutil"

	"kraftkit.sh/unikraft/app"

	// Subcommands
	"kraftkit.sh/cmd/kraft/build/clean"
	"kraftkit.sh/cmd/kraft/build/configure"
	"kraftkit.sh/cmd/kraft/build/fetch"
	"kraftkit.sh/cmd/kraft/build/menuconfig"
	"kraftkit.sh/cmd/kraft/build/prepare"
	"kraftkit.sh/cmd/kraft/build/properclean"
	"kraftkit.sh/cmd/kraft/build/set"
	"kraftkit.sh/cmd/kraft/build/unset"
)

func BuildCmd(f *cmdfactory.Factory) *cobra.Command {
	cmd, err := cmdutil.NewCmd(f, "build",
		cmdutil.WithSubcmds(
			configure.ConfigureCmd(f),
			fetch.FetchCmd(f),
			menuconfig.MenuConfigCmd(f),
			prepare.PrepareCmd(f),
			set.SetCmd(f),
			unset.UnsetCmd(f),
			clean.CleanCmd(f),
			properclean.PropercleanCmd(f),
		),
	)
	if err != nil {
		panic("could not initialize 'kraft build' command")
	}

	application := &app.CommandOptions{
		PackageManager: f.PackageManager,
		ConfigManager:  f.ConfigManager,
		Logger:         f.Logger,
		IO:             f.IOStreams,
	}

	args := &app.CommandBuildArgs{}

	cmd.Short = "Configure and build Unikraft unikernels "
	cmd.Use = "build [FLAGS] [SUBCOMMAND|DIR]"
	cmd.Args = cmdutil.MaxDirArgs(1)
	cmd.Long = heredoc.Docf(`
		Configure and build Unikraft unikernels.

		The default behaviour of %[1]skraft build%[1]s is to build a project.  Given no
		arguments, you will be guided through interactive mode.
	`, "`")
	cmd.Example = heredoc.Doc(`
		# Build the current project (cwd)
		$ kraft build

		# Build path to a Unikraft project
		$ kraft build path/to/app
	`)
	cmd.RunE = func(cmd *cobra.Command, extraArgs []string) error {
		if (len(args.Architecture) > 0 || len(args.Platform) > 0) && len(args.Target) > 0 {
			return fmt.Errorf("the `--arch` and `--plat` options are not supported in addition to `--target`")
		}

		var err error
		if len(extraArgs) == 0 {
			application.Workdir, err = os.Getwd()
			if err != nil {
				return err
			}
		} else {
			application.Workdir = extraArgs[0]
		}

		return application.Build(args)
	}

	cmd.Flags().BoolVarP(
		&args.NoCache,
		"no-cache", "F",
		false,
		"Force a rebuild even if existing intermediate artifacts already exist",
	)

	cmd.Flags().StringVarP(
		&args.Architecture,
		"arch", "m",
		"",
		"Filter the creation of the build by architecture of known targets",
	)

	cmd.Flags().StringVarP(
		&args.Platform,
		"plat", "p",
		"",
		"Filter the creation of the build by platform of known targets",
	)

	cmd.Flags().StringVarP(
		&args.DotConfig,
		"config", "c",
		"",
		"Override the path to the KConfig `.config` file",
	)

	cmd.Flags().BoolVar(
		&args.KernelDbg,
		"dbg",
		false,
		"Build the debuggable (symbolic) kernel image instead of the stripped image",
	)

	cmd.Flags().StringVarP(
		&args.Target,
		"target", "t",
		"",
		"Build a particular known target",
	)

	cmd.Flags().BoolVar(
		&args.Fast,
		"fast",
		false,
		"Use maximum parallization when performing the build",
	)

	cmd.Flags().IntVarP(
		&args.Jobs,
		"jobs", "j",
		0,
		"Allow N jobs at once",
	)

	cmd.Flags().StringVar(
		&args.SaveBuildLog,
		"build-log",
		"",
		"Use the specified file to save the output from the build",
	)

	cmd.Flags().BoolVar(
		&args.NoSyncConfig,
		"no-sync-config",
		false,
		"Do not synchronize Unikraft's configuration before building",
	)

	cmd.Flags().BoolVar(
		&args.NoConfigure,
		"no-configure",
		false,
		"Do not run Unikraft's configure step before building",
	)

	cmd.Flags().BoolVar(
		&args.NoPrepare,
		"no-prepare",
		false,
		"Do not run Unikraft's prepare step before building",
	)

	return cmd
}
