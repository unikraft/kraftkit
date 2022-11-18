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

package fetch

import (
	"os"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/internal/cmdfactory"
	"kraftkit.sh/internal/cmdutil"
	"kraftkit.sh/unikraft/app"
)

func FetchCmd(f *cmdfactory.Factory) *cobra.Command {
	application := &app.ApplicationOptions{
		PackageManager: f.PackageManager,
		Logger:         f.Logger,
		IO:             f.IOStreams,
	}

	cmd, err := cmdutil.NewCmd(f, "fetch")
	if err != nil {
		panic("could not initialize 'kraft build fetch' command")
	}

	cmd.Short = "Fetch a Unikraft unikernel's dependencies"
	cmd.Use = "fetch [DIR]"
	cmd.Aliases = []string{"f"}
	cmd.Args = cmdutil.MaxDirArgs(1)
	cmd.Long = heredoc.Doc(`
		Fetch a Unikraft unikernel's dependencies`)
	cmd.Example = heredoc.Doc(`
		# Fetch the cwd project
		$ kraft build fetch

		# Fetch a project at a path
		$ kraft build fetch path/to/app
	`)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			application.Workdir, err = os.Getwd()
			if err != nil {
				return err
			}
		} else {
			application.Workdir = args[0]
		}

		return application.Fetch()
	}

	return cmd
}
