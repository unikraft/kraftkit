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

package pkg

import (
	"fmt"
	"os"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/internal/cmdfactory"
	"kraftkit.sh/internal/cmdutil"

	"kraftkit.sh/unikraft/app"

	"kraftkit.sh/cmd/kraft/pkg/list"
	"kraftkit.sh/cmd/kraft/pkg/pull"
	"kraftkit.sh/cmd/kraft/pkg/source"
	"kraftkit.sh/cmd/kraft/pkg/update"
)

func PkgCmd(f *cmdfactory.Factory) *cobra.Command {
	cmd, err := cmdutil.NewCmd(f, "pkg",
		cmdutil.WithSubcmds(
			list.ListCmd(f),
			pull.PullCmd(f),
			source.SourceCmd(f),
			update.UpdateCmd(f),
		),
	)
	if err != nil {
		panic("could not initialize 'kraft pkg' command")
	}

	application := &app.ApplicationOptions{
		PackageManager: f.PackageManager,
		ConfigManager:  f.ConfigManager,
		Logger:         f.Logger,
		IO:             f.IOStreams,
	}

	args := &app.CommandPkgArgs{}

	cmd.Short = "Package and distribute Unikraft unikernels and their dependencies"
	cmd.Use = "pkg [FLAGS] [SUBCOMMAND|DIR]"
	cmd.Args = cmdutil.MaxDirArgs(1)
	cmd.Long = heredoc.Docf(`
		Package and distribute Unikraft unikernels and their dependencies.

		With %[1]skraft pkg%[1]s you are able to turn output artifacts from %[1]skraft build%[1]s
		into a distributable archive ready for deployment.  At the same time,
		%[1]skraft pkg%[1]s allows you to manage these archives: pulling, pushing, or
		adding them to a project.

		The default behaviour of %[1]skraft pkg%[1]s is to package a project.  Given no
		arguments, you will be guided through interactive mode.

		For initram and disk images, passing in a directory as the argument will
		result automatically packaging that directory into the requested format.
		Separating the input with a %[1]s:%[1]s delimeter allows you to set the
		output that of the artifact.
	`, "`")
	cmd.Example = heredoc.Doc(`
		# Package the current Unikraft project (cwd)
		$ kraft pkg

		# Package path to a Unikraft project
		$ kraft pkg path/to/application

		# Package with an additional initramfs
		$ kraft pkg --initrd ./root-fs .

		# Same as above but also save the resulting CPIO artifact locally
		$ kraft pkg --initrd ./root-fs:./root-fs.cpio .
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

		return application.Pkg(args)
	}

	// TODO: Enable flag if multiple managers are detected?
	cmd.Flags().StringVarP(
		&args.Format,
		"as", "M",
		"auto",
		"Force the packaging despite possible conflicts",
	)

	cmd.Flags().BoolVar(
		&args.Force,
		"force-format",
		false,
		"Force the use of a packaging handler format",
	)

	cmd.Flags().StringVarP(
		&args.Architecture,
		"arch", "m",
		"",
		"Filter the creation of the package by architecture of known targets",
	)

	cmd.Flags().StringVarP(
		&args.Platform,
		"plat", "p",
		"",
		"Filter the creation of the package by platform of known targets",
	)

	cmd.Flags().StringVar(
		&args.Name,
		"name",
		"",
		"Specify the name of the package.",
	)

	cmd.Flags().StringVarP(
		&args.Kernel,
		"kernel", "k",
		"",
		"Override the path to the unikernel image",
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
		"Package the debuggable (symbolic) kernel image instead of the stripped image",
	)

	cmd.Flags().BoolVar(
		&args.WithDbg,
		"with-dbg",
		false,
		"In addition to the stripped kernel, include the debug image",
	)

	cmd.Flags().StringVarP(
		&args.Target,
		"target", "t",
		"",
		"Package a particular known target",
	)

	cmd.Flags().StringVarP(
		&args.Initrd,
		"initrd", "i",
		"",
		"Path to init ramdisk to bundle within the package (passing a path will "+
			"automatically generate a CPIO image)",
	)

	cmd.Flags().StringSliceVarP(
		&args.Volumes,
		"volumes", "v",
		[]string{},
		"Additional volumes to bundle within the package",
	)

	cmd.Flags().StringVarP(
		&args.Output,
		"output", "o",
		"",
		"Save the package at the following output.",
	)

	return cmd
}
