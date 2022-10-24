// SPDX-License-Identifier: BSD-3-Clause
//
// Authors: Cezar Craciunoiu <cezar@unikraft.io>
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

package unsource

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/internal/cmdfactory"
	"kraftkit.sh/internal/cmdutil"

	"kraftkit.sh/packmanager"
)

type UnsourceOptions struct {
	PackageManager func(opts ...packmanager.PackageManagerOption) (packmanager.PackageManager, error)
}

func UnsourceCmd(f *cmdfactory.Factory) *cobra.Command {
	opts := &UnsourceOptions{
		PackageManager: f.PackageManager,
	}

	cmd, err := cmdutil.NewCmd(f, "unsource")
	if err != nil {
		panic("could not initialize 'kraft pkg unsource' commmand")
	}

	cmd.Short = "Remove Unikraft component manifests"
	cmd.Use = "unsource [FLAGS] [SOURCE]"
	cmd.Args = cmdutil.MinimumArgs(1, "must specify component or manifest")
	cmd.Aliases = []string{"a"}
	cmd.Long = heredoc.Docf(`
	`, "`")
	cmd.Example = heredoc.Docf(`
		# Remove a single component as a Git repository or manifest
		$ kraft pkg unsource https://github.com/unikraft/unikraft.git
		$ kraft pkg unsource https://raw.github.com/unikraft/index/stable/index.yaml
	`)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		source := ""
		if len(args) > 0 {
			source = args[0]
		}
		return unsourceRun(opts, source)
	}

	return cmd
}

func unsourceRun(opts *UnsourceOptions, source string) error {
	var err error

	pm, err := opts.PackageManager()
	if err != nil {
		return err
	}

	err = pm.RemoveSource(source)
	if err != nil {
		return err
	}

	return nil
}
