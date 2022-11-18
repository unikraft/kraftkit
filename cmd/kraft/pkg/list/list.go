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

package list

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/unikraft/app"

	"kraftkit.sh/internal/cmdfactory"
	"kraftkit.sh/internal/cmdutil"
)

func ListCmd(f *cmdfactory.Factory) *cobra.Command {
	application := &app.ApplicationOptions{
		PackageManager: f.PackageManager,
		ConfigManager:  f.ConfigManager,
		Logger:         f.Logger,
		IO:             f.IOStreams,
	}

	args := &app.CommandListArgs{}

	cmd, err := cmdutil.NewCmd(f, "list")
	if err != nil {
		panic("could not initialize 'kraft pkg list' command")
	}

	cmd.Short = "List installed Unikraft component packages"
	cmd.Use = "list [FLAGS] [DIR]"
	cmd.Aliases = []string{"l", "ls"}
	cmd.Args = cmdutil.MaxDirArgs(1)
	cmd.Long = heredoc.Doc(`
		List installed Unikraft component packages.
	`)
	cmd.Example = heredoc.Doc(`
		$ kraft pkg list
	`)
	cmd.RunE = func(cmd *cobra.Command, extraArgs []string) error {
		if len(extraArgs) > 0 {
			application.Workdir = extraArgs[0]
		}
		return application.List(args)
	}

	cmd.Flags().IntVarP(
		&args.LimitResults,
		"limit", "l",
		30,
		"Maximum number of items to print (-1 returns all)",
	)

	noLimitResults := false
	cmd.Flags().BoolVarP(
		&noLimitResults,
		"no-limit", "T",
		false,
		"Do not limit the number of items to print",
	)
	if noLimitResults {
		args.LimitResults = -1
	}

	cmd.Flags().BoolVarP(
		&args.Update,
		"update", "U",
		false,
		"Get latest information about components before listing results",
	)

	cmd.Flags().BoolVarP(
		&args.ShowCore,
		"core", "C",
		false,
		"Show Unikraft core versions",
	)

	cmd.Flags().BoolVarP(
		&args.ShowArchs,
		"arch", "M",
		false,
		"Show architectures",
	)

	cmd.Flags().BoolVarP(
		&args.ShowPlats,
		"plats", "P",
		false,
		"Show platforms",
	)

	cmd.Flags().BoolVarP(
		&args.ShowLibs,
		"libs", "L",
		false,
		"Show libraries",
	)

	cmd.Flags().BoolVarP(
		&args.ShowApps,
		"apps", "A",
		false,
		"Show applications",
	)

	return cmd
}
