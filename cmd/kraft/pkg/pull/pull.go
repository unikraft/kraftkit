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

package pull

import (
	"os"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/internal/cmdfactory"
	"kraftkit.sh/internal/cmdutil"
	"kraftkit.sh/unikraft/app"
)

func PullCmd(f *cmdfactory.Factory) *cobra.Command {
	application := &app.CommandOptions{
		PackageManager: f.PackageManager,
		ConfigManager:  f.ConfigManager,
		Logger:         f.Logger,
		IO:             f.IOStreams,
	}

	args := &app.CommandPullArgs{}

	cmd, err := cmdutil.NewCmd(f, "pull")
	if err != nil {
		panic("could not initialize root command")
	}

	cmd.Short = "Pull a Unikraft unikernel and/or its dependencies"
	cmd.Use = "pull [FLAGS] [PACKAGE|DIR]"
	cmd.Aliases = []string{"p"}
	// cmd.Args = cmdutil(1)
	cmd.Long = heredoc.Doc(`
		Pull a Unikraft unikernel, component microlibrary from a remote location`)
	cmd.Example = heredoc.Doc(`
		# Pull the dependencies for a project in the current working directory
		$ kraft pkg pull
		
		# Pull dependencies for a project at a path
		$ kraft pkg pull path/to/app

		# Pull a source repository
		$ kraft pkg pull github.com/unikraft/app-nginx.git

		# Pull an OCI-packaged Unikraft unikernel
		$ kraft pkg pull unikraft.io/nginx:1.21.6

		# Pull from a manifest
		$ kraft pkg pull nginx@1.21.6
	`)
	cmd.RunE = func(cmd *cobra.Command, extraArgs []string) error {
		query := ""
		if len(extraArgs) > 0 {
			query = strings.Join(extraArgs, " ")
		}

		if len(query) == 0 {
			query, err = os.Getwd()
			if err != nil {
				return err
			}
		}

		if err := cmdutil.MutuallyExclusive(
			"the `--with-deps` option is not supported with `--no-deps`",
			args.WithDeps,
			args.NoDeps,
		); err != nil {
			return err
		}

		return application.Pull(args, query)
	}

	// TODO: Enable flag if multiple managers are detected?
	cmd.Flags().StringVarP(
		&args.Manager,
		"manager", "M",
		"auto",
		"Force the handler type (Omittion will attempt auto-detect)",
	)

	cmd.Flags().BoolVarP(
		&args.WithDeps,
		"with-deps", "d",
		false,
		"Pull dependencies",
	)

	cmd.Flags().StringVarP(
		&application.Workdir,
		"workdir", "w",
		"",
		"Set a path to working directory to pull components to",
	)

	cmd.Flags().BoolVarP(
		&args.NoDeps,
		"no-deps", "D",
		true,
		"Do not pull dependencies",
	)

	cmd.Flags().StringVarP(
		&args.Architecture,
		"arch", "m",
		"",
		"Specify the desired architecture",
	)

	cmd.Flags().StringVarP(
		&args.Platform,
		"plat", "p",
		"",
		"Specify the desired architecture",
	)

	cmd.Flags().BoolVarP(
		&args.AllVersions,
		"all-versions", "a",
		false,
		"Pull all versions",
	)

	cmd.Flags().BoolVarP(
		&args.NoChecksum,
		"no-checksum", "C",
		false,
		"Do not verify package checksum (if available)",
	)

	cmd.Flags().BoolVarP(
		&args.NoChecksum,
		"no-cache", "Z",
		false,
		"Do not use cache and pull directly from source",
	)

	return cmd
}
